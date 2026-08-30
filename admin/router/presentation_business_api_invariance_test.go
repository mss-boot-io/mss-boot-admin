package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/admin/apis"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	adminpkg "github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/admin/presentation"
	presentationcore "github.com/mss-boot-io/mss-boot-admin/admin/presentation/core"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	responsegorm "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions/gorm"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// presentationBusinessEndpoint is the server-side projection of every core
// page-presentation data source. Keeping this matrix beside the production
// router makes the security invariant executable: a presentation profile may
// select only already-projected fields; it never grants or widens the backing
// business API.
type presentationBusinessEndpoint struct {
	pageKey          string
	path             string
	wantMarker       string
	forbiddenMarkers []string
	rootOnly         bool
}

func TestCorePresentationProfilesDoNotChangeBusinessAPIAuthorityOrData(t *testing.T) {
	fixture := newPresentationBusinessAPIFixture(t)

	endpoints := []presentationBusinessEndpoint{
		{pageKey: "user.list", path: "/admin/api/users", wantMarker: "fixture-user-row", forbiddenMarkers: []string{"user-password-hash-secret", "user-salt-secret"}},
		{pageKey: "role.list", path: "/admin/api/roles", wantMarker: "fixture-role-row"},
		{pageKey: "menu.list", path: "/admin/api/menus", wantMarker: "fixture-menu-row"},
		{pageKey: "department.list", path: "/admin/api/departments", wantMarker: "fixture-department-row"},
		{pageKey: "post.list", path: "/admin/api/posts", wantMarker: "fixture-post-row"},
		{pageKey: "task.list", path: "/admin/api/tasks", wantMarker: "fixture-task-row", forbiddenMarkers: []string{"task-payload-secret", "task-metadata-secret", "task-script-secret"}},
		{pageKey: "notice.list", path: "/admin/api/notices", wantMarker: "fixture-notice-own-row", forbiddenMarkers: []string{"fixture-notice-other-row"}},
		{pageKey: "language.list", path: "/admin/api/languages?view=summary", wantMarker: "fixture-language-row", forbiddenMarkers: []string{"language-definition-secret"}},
		{pageKey: "option.list", path: "/admin/api/options", wantMarker: "fixture-option-row", forbiddenMarkers: []string{"option-item-secret"}},
		{pageKey: "system-config.list", path: "/admin/api/system-configs", wantMarker: "fixture-system-config-row", forbiddenMarkers: []string{"system-config-content-secret"}, rootOnly: true},
		{pageKey: "online-session.list", path: "/admin/api/online-sessions", wantMarker: "fixture-online-session-row", forbiddenMarkers: []string{"reauth-sensitive-state"}, rootOnly: true},
		{pageKey: "log.login", path: "/admin/api/audit-logs/login", wantMarker: "fixture-login-log-row", forbiddenMarkers: []string{"login-log-secret"}},
		{pageKey: "log.audit", path: "/admin/api/audit-logs/operation", wantMarker: "fixture-audit-log-row", forbiddenMarkers: []string{"audit-request-secret", "audit-response-secret", "audit-message-secret", "audit-path-secret"}},
		{pageKey: "log.runtime", path: "/admin/api/logs", wantMarker: "fixture-runtime-log-row", forbiddenMarkers: []string{"runtime-log-secret"}},
	}

	assertPresentationEndpointMatrix(t, fixture.router, endpoints)

	baseline := make(map[string]any, len(endpoints))
	for _, endpoint := range endpoints {
		endpoint := endpoint
		t.Run(endpoint.pageKey+"/authority-and-projection", func(t *testing.T) {
			profileOnly := fixture.get(t, endpoint.path, fixture.tokens[profileOnlyRole])
			require.Equal(t, http.StatusForbidden, profileOnly.Code, profileOnly.Body.String())

			businessOnly := fixture.get(t, endpoint.path, fixture.tokens[businessOnlyRole])
			businessAndPresentation := fixture.get(t, endpoint.path, fixture.tokens[businessAndPresentationRole])
			root := fixture.get(t, endpoint.path, fixture.tokens[rootRole])
			require.Equal(t, http.StatusOK, root.Code, root.Body.String())

			if endpoint.rootOnly {
				require.Equal(t, http.StatusForbidden, businessOnly.Code, businessOnly.Body.String())
				require.Equal(t, normalizeBusinessResponse(t, businessOnly.Body.Bytes()), normalizeBusinessResponse(t, businessAndPresentation.Body.Bytes()),
					"presentation governance grants must not bypass the handler-adjacent root boundary")
				baseline[endpoint.pageKey] = normalizeBusinessResponse(t, root.Body.Bytes())
				require.Contains(t, root.Body.String(), endpoint.wantMarker)
				assertForbiddenPresentationMarkers(t, endpoint, root.Body.String())
				return
			}

			require.Equal(t, http.StatusOK, businessOnly.Code, businessOnly.Body.String())
			require.Equal(t, http.StatusOK, businessAndPresentation.Code, businessAndPresentation.Body.String())
			require.Equal(t, normalizeBusinessResponse(t, businessOnly.Body.Bytes()), normalizeBusinessResponse(t, businessAndPresentation.Body.Bytes()),
				"adding presentation governance authority changed the business API projection")
			require.Contains(t, businessOnly.Body.String(), endpoint.wantMarker)
			assertForbiddenPresentationMarkers(t, endpoint, businessOnly.Body.String())
			baseline[endpoint.pageKey] = normalizeBusinessResponse(t, businessOnly.Body.Bytes())
		})
	}

	fixture.insertPublishedProfiles(t, endpoints)

	for _, endpoint := range endpoints {
		endpoint := endpoint
		t.Run(endpoint.pageKey+"/published-profile-invariance", func(t *testing.T) {
			token := fixture.tokens[businessOnlyRole]
			if endpoint.rootOnly {
				token = fixture.tokens[rootRole]
			}
			afterProfile := fixture.get(t, endpoint.path, token)
			require.Equal(t, http.StatusOK, afterProfile.Code, afterProfile.Body.String())
			require.Equal(t, baseline[endpoint.pageKey], normalizeBusinessResponse(t, afterProfile.Body.Bytes()),
				"published presentation profile changed row identity, row scope, or the server projection")
			assertForbiddenPresentationMarkers(t, endpoint, afterProfile.Body.String())

			profileOnly := fixture.get(t, endpoint.path, fixture.tokens[profileOnlyRole])
			require.Equal(t, http.StatusForbidden, profileOnly.Code,
				"profile existence unexpectedly granted direct business API authority")
		})
	}

	// A GET policy remains read-only after presentation governance is added.
	// These representative generic and custom mutation routes cover the shared
	// authorization boundary used by the 14 list pages.
	for _, path := range []string{
		"/admin/api/users",
		"/admin/api/roles",
		"/admin/api/departments",
		"/admin/api/tasks",
		"/admin/api/languages",
		"/admin/api/options",
	} {
		denied := fixture.request(t, http.MethodPost, path, fixture.tokens[businessAndPresentationRole], `{}`)
		require.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())
	}
}

func normalizeBusinessResponse(t *testing.T, raw []byte) any {
	t.Helper()
	var value any
	require.NoError(t, json.Unmarshal(raw, &value), string(raw))
	var dropEphemeral func(any)
	dropEphemeral = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			delete(typed, "traceId")
			for _, child := range typed {
				dropEphemeral(child)
			}
		case []any:
			for _, child := range typed {
				dropEphemeral(child)
			}
		}
	}
	dropEphemeral(value)
	return value
}

func assertForbiddenPresentationMarkers(t *testing.T, endpoint presentationBusinessEndpoint, body string) {
	t.Helper()
	for _, marker := range endpoint.forbiddenMarkers {
		require.NotContains(t, body, marker, "%s leaked a sensitive or out-of-scope field", endpoint.pageKey)
	}
}

func assertPresentationEndpointMatrix(t *testing.T, engine *gin.Engine, endpoints []presentationBusinessEndpoint) {
	t.Helper()
	definitions := presentationcore.Definitions()
	definitionKeys := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		definitionKeys = append(definitionKeys, definition.PageKey)
	}
	endpointKeys := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		endpointKeys = append(endpointKeys, endpoint.pageKey)
	}
	sort.Strings(definitionKeys)
	sort.Strings(endpointKeys)
	require.Equal(t, definitionKeys, endpointKeys, "business API invariance matrix drifted from core presentation inventory")

	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	contracts := CustomRouteContracts()
	for _, endpoint := range endpoints {
		path := endpoint.path
		if index := strings.IndexByte(path, '?'); index >= 0 {
			path = path[:index]
		}
		_, registered := routes[http.MethodGet+" "+path]
		require.True(t, registered, "%s does not resolve to a real production Gin route", endpoint.pageKey)

		for _, contract := range contracts {
			if contract.Method == http.MethodGet && contract.Path == path {
				require.Equal(t, RouteAuthorized, contract.Class, endpoint.pageKey)
				require.Equal(t, endpoint.rootOnly, contract.RootOnly, endpoint.pageKey)
			}
		}
	}
}

const (
	businessOnlyRole            = "presentation-business-only"
	businessAndPresentationRole = "presentation-business-and-governance"
	profileOnlyRole             = "presentation-governance-only"
	rootRole                    = "presentation-business-root"
)

type presentationBusinessAPIFixture struct {
	db                  *gorm.DB
	router              *gin.Engine
	tokens              map[string]string
	presentationService *service.PresentationProfileService
}

func newPresentationBusinessAPIFixture(t *testing.T) *presentationBusinessAPIFixture {
	t.Helper()
	previousGinMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousGinMode) })

	runtimeRoot := t.TempDir()
	previousWorkingDirectory, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(runtimeRoot))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousWorkingDirectory)) })
	require.NoError(t, os.MkdirAll("logs", 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join("logs", "runtime.log"),
		[]byte("2026/08/30 01:02:03 [INFO] fixture-runtime-log-row Authorization: Bearer runtime-log-secret\n"),
		0o600,
	))

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "presentation-business-api.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(
		&models.ConfigRevision{},
		&models.Role{},
		&models.Menu{},
		&models.Department{},
		&models.Post{},
		&models.User{},
		&models.Task{},
		&models.Notice{},
		&models.Language{},
		&models.Option{},
		&models.SystemConfig{},
		&models.UserSession{},
		&models.AuditLog{},
		&models.LoginLog{},
		&models.PresentationProfile{},
		&models.PresentationRevision{},
	))

	enforcer := newPresentationBusinessEnforcer(t)
	previousHandle := gormdb.InstallDefault(&gormdb.Handle{DB: db, Enforcer: enforcer})
	t.Cleanup(func() { gormdb.InstallDefault(previousHandle) })

	previousAuthorizationPolicies := service.AuthorizationPolicies
	service.AuthorizationPolicies = service.NewAuthorizationPolicyService()
	t.Cleanup(func() { service.AuthorizationPolicies = previousAuthorizationPolicies })

	previousAuthConfig := config.Cfg.Auth
	previousApplicationMode := config.Cfg.Application.Mode
	previousAuth := middleware.Auth
	previousVerifier := middleware.Verifier
	previousAuthHandler := response.AuthHandler
	previousVerifyHandler := response.VerifyHandler
	previousCleaner := responsegorm.CleanCacheFromTag
	previousAuthMiddleware, hadAuthMiddleware := middleware.Middlewares.Load("auth")
	config.Cfg.Application.Mode = config.ModeTest
	config.Cfg.Auth = config.Auth{
		Realm:       "presentation-business-api-test",
		Key:         "presentation-business-api-test-signing-key-20260830",
		IdentityKey: "presentation-business-api-principal",
		Timeout:     time.Hour,
		MaxRefresh:  time.Hour,
	}
	middleware.Verifier = &models.User{}
	responsegorm.CleanCacheFromTag = nil
	require.NoError(t, middleware.Init())
	t.Cleanup(func() {
		config.Cfg.Auth = previousAuthConfig
		config.Cfg.Application.Mode = previousApplicationMode
		middleware.Auth = previousAuth
		middleware.Verifier = previousVerifier
		response.AuthHandler = previousAuthHandler
		response.VerifyHandler = previousVerifyHandler
		responsegorm.CleanCacheFromTag = previousCleaner
		if hadAuthMiddleware {
			middleware.Middlewares.Store("auth", previousAuthMiddleware)
		} else {
			middleware.Middlewares.Delete("auth")
		}
	})

	principals := presentationBusinessPrincipals()
	middleware.Auth.PayloadFunc = func(data any) jwt.MapClaims {
		principal, ok := data.(security.Verifier)
		if !ok || principal == nil {
			return jwt.MapClaims{}
		}
		return jwt.MapClaims{"uid": principal.GetUserID(), "rid": principal.GetRoleID()}
	}
	middleware.Auth.IdentityHandler = func(ctx *gin.Context) any {
		roleID := cast.ToString(jwt.ExtractClaims(ctx)["rid"])
		return principals[roleID]
	}

	tokens := make(map[string]string, len(principals))
	for roleID, principal := range principals {
		token, _, tokenErr := middleware.Auth.TokenGenerator(principal)
		require.NoError(t, tokenErr, roleID)
		tokens[roleID] = token
	}

	seedPresentationBusinessRows(t, db)

	definitions := presentationcore.Definitions()
	registry := presentation.MustNewFrozenRegistry(definitions...)
	pageKeys := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		pageKeys = append(pageKeys, definition.PageKey)
	}
	policy := presentation.MustNewAdoptionPolicy(presentation.AdoptionActive, pageKeys, false, registry)
	presentationService, err := service.NewPresentationProfileService(registry, policy)
	require.NoError(t, err)
	presentationService.Database = db
	presentationController, err := apis.NewPresentationProfileController(presentationService)
	require.NoError(t, err)

	engine := gin.New()
	_, err = InitRouteGroupsWithDependencies(engine.Group("/admin"), Dependencies{
		PresentationProfiles: presentationController,
	})
	require.NoError(t, err)

	return &presentationBusinessAPIFixture{
		db: db, router: engine, tokens: tokens, presentationService: presentationService,
	}
}

func presentationBusinessPrincipals() map[string]*models.User {
	post := &models.Post{DataScope: models.DataScopeSelf}
	post.ID = "fixture-restricted-post"
	makePrincipal := func(roleID string, root bool) *models.User {
		principal := &models.User{UserLogin: models.UserLogin{
			RoleID:   roleID,
			Role:     &models.Role{Root: root, Status: enum.Enabled},
			PostID:   post.ID,
			Post:     post,
			Username: "fixture-principal",
			Status:   enum.Enabled,
		}}
		principal.ID = "fixture-principal-user"
		return principal
	}
	return map[string]*models.User{
		businessOnlyRole:            makePrincipal(businessOnlyRole, false),
		businessAndPresentationRole: makePrincipal(businessAndPresentationRole, false),
		profileOnlyRole:             makePrincipal(profileOnlyRole, false),
		rootRole:                    makePrincipal(rootRole, true),
	}
}

func newPresentationBusinessEnforcer(t *testing.T) casbin.IEnforcer {
	t.Helper()
	policyModel, err := casbinmodel.NewModelFromString(`[request_definition]
r = sub, tp, obj, act

[policy_definition]
p = sub, tp, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.tp == p.tp && keyMatch(r.obj, p.obj) && regexMatch(r.act, p.act)
`)
	require.NoError(t, err)

	businessPaths := []string{
		"/admin/api/users",
		"/admin/api/roles",
		"/admin/api/menus",
		"/admin/api/departments",
		"/admin/api/posts",
		"/admin/api/tasks",
		"/admin/api/notices",
		"/admin/api/languages",
		"/admin/api/options",
		"/admin/api/system-configs",
		"/admin/api/online-sessions",
		"/admin/api/audit-logs/login",
		"/admin/api/audit-logs/operation",
		"/admin/api/logs",
	}
	presentationPaths := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/admin/api/presentation-capabilities"},
		{method: http.MethodGet, path: "/admin/api/presentation-profiles"},
		{method: http.MethodPost, path: "/admin/api/presentation-profiles"},
		{method: http.MethodPost, path: "/admin/api/presentation-profiles/validate"},
	}

	var policy strings.Builder
	for _, roleID := range []string{businessOnlyRole, businessAndPresentationRole} {
		for _, path := range businessPaths {
			fmt.Fprintf(&policy, "p, %s, %s, %s, %s\n", roleID, adminpkg.APIAccessType.String(), path, http.MethodGet)
		}
	}
	for _, roleID := range []string{businessAndPresentationRole, profileOnlyRole} {
		for _, route := range presentationPaths {
			fmt.Fprintf(&policy, "p, %s, %s, %s, %s\n", roleID, adminpkg.APIAccessType.String(), route.path, route.method)
		}
	}
	policyPath := filepath.Join(t.TempDir(), "presentation-business-policy.csv")
	require.NoError(t, os.WriteFile(policyPath, []byte(policy.String()), 0o600))
	enforcer, err := casbin.NewEnforcer(policyModel, fileadapter.NewAdapter(policyPath))
	require.NoError(t, err)
	return enforcer
}

func seedPresentationBusinessRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Date(2026, time.August, 30, 1, 2, 3, 0, time.UTC)
	withoutHooks := db.Session(&gorm.Session{SkipHooks: true})

	role := &models.Role{Name: "fixture-role-row", Status: enum.Enabled, Remark: "fixture role"}
	role.ID = "fixture-role-row"
	require.NoError(t, withoutHooks.Create(role).Error)

	menu := &models.Menu{
		ParentID: "", Name: "fixture-menu-row", Path: "/fixture-menu-row", Component: "./Fixture",
		Type: adminpkg.MenuAccessType, Status: enum.Enabled,
	}
	menu.ID = "fixture-menu-row"
	require.NoError(t, withoutHooks.Create(menu).Error)

	department := &models.Department{ParentID: "", Name: "fixture-department-row", Code: "FIXTURE", Status: enum.Enabled}
	department.ID = "fixture-department-row"
	require.NoError(t, withoutHooks.Create(department).Error)

	post := &models.Post{
		ParentID: "", Name: "fixture-post-row", Code: "FIXTURE", Status: enum.Enabled,
		DataScope: models.DataScopeCustomDept, DeptIDS: "post-department-scope-secret",
	}
	post.ID = "fixture-post-row"
	require.NoError(t, withoutHooks.Create(post).Error)

	user := &models.User{UserLogin: models.UserLogin{
		RoleID: "fixture-role-row", PostID: "fixture-post-row", DepartmentID: "fixture-department-row",
		Username: "fixture-user-row", Email: "fixture-user@example.test", PasswordHash: "user-password-hash-secret",
		Salt: "user-salt-secret", Status: enum.Enabled,
	}, Name: "fixture user"}
	user.ID = "fixture-user-row"
	require.NoError(t, withoutHooks.Omit(clause.Associations).Create(user).Error)

	task := &models.Task{
		Name: "fixture-task-row", Provider: models.TaskProviderDefault, Spec: "0 * * * * *", Status: enum.Disabled,
		Body: "task-payload-secret", Metadata: "task-metadata-secret", Python: "task-script-secret",
		Command: "[]", Args: "[]",
	}
	task.ID = "fixture-task-row"
	task.CreatedAt, task.UpdatedAt = now, now
	require.NoError(t, withoutHooks.Create(task).Error)

	ownedNotice := &models.Notice{UserID: "fixture-principal-user", Title: "fixture-notice-own-row", Type: models.NoticeTypeMessage}
	ownedNotice.ID = "fixture-notice-own-row"
	ownedNotice.CreatedAt, ownedNotice.UpdatedAt = now, now
	otherNotice := &models.Notice{UserID: "fixture-other-user", Title: "fixture-notice-other-row", Type: models.NoticeTypeMessage}
	otherNotice.ID = "fixture-notice-other-row"
	otherNotice.CreatedAt, otherNotice.UpdatedAt = now, now
	require.NoError(t, withoutHooks.Create(&[]*models.Notice{ownedNotice, otherNotice}).Error)

	definitions := models.LanguageDefines{
		&models.LanguageDefine{ID: "fixture-language-definition", Group: "fixture", Key: "secret", Value: "language-definition-secret"},
	}
	language := &models.Language{Name: "en-US", Remark: "fixture-language-row", Status: enum.Enabled, Defines: &definitions}
	language.ID = "fixture-language-row"
	language.CreatedAt, language.UpdatedAt = now, now
	require.NoError(t, withoutHooks.Create(language).Error)

	items := models.OptionItems{
		&models.OptionItem{ID: "fixture-option-item", Key: "secret", Label: "secret", Value: "option-item-secret"},
	}
	option := &models.Option{
		Category: "fixture", DisplayName: "fixture-option-row", Name: "fixture.option", Remark: "fixture option",
		Items: &items, Status: enum.Enabled, Version: 1,
	}
	option.ID = "fixture-option-row"
	option.CreatedAt, option.UpdatedAt = now, now
	require.NoError(t, withoutHooks.Create(option).Error)

	systemConfig := &models.SystemConfig{
		Name: "fixture-system-config-row", Ext: source.Scheme("yaml"), Content: "system-config-content-secret", Remark: "fixture",
	}
	systemConfig.ID = "fixture-system-config-row"
	systemConfig.CreatedAt, systemConfig.UpdatedAt = now, now
	require.NoError(t, withoutHooks.Create(systemConfig).Error)

	reauthenticatedAt := now
	session := &models.UserSession{
		UserID: "fixture-session-user", Username: "fixture-online-session-row", RoleID: "fixture-role-row",
		LoginAt: now, LastSeenAt: now, ReauthenticatedAt: &reauthenticatedAt,
		ExpiredAt: time.Now().Add(time.Hour), IP: "127.0.0.1", UserAgent: "fixture-agent",
	}
	session.ID = "fixture-online-session-row"
	session.CreatedAt, session.UpdatedAt = now, now
	require.NoError(t, withoutHooks.Create(session).Error)

	loginLog := &models.LoginLog{
		UserID: "fixture-user-row", Username: "fixture-login-log-row", Status: enum.Enabled,
		Message: "Authorization: Bearer login-log-secret", LoginAt: now,
	}
	loginLog.ID = "fixture-login-log-row"
	loginLog.CreatedAt, loginLog.UpdatedAt = now, now
	require.NoError(t, withoutHooks.Create(loginLog).Error)

	auditLog := &models.AuditLog{
		UserID: "fixture-user-row", Username: "fixture-audit-log-row", Type: models.AuditLogTypeUpdate,
		Action: "update", Resource: "fixture", Method: http.MethodPut,
		Path: "/fixture?token=audit-path-secret", Status: enum.Enabled,
		Message: "Authorization: Bearer audit-message-secret", Request: `{"password":"audit-request-secret"}`,
		Response: `{"token":"audit-response-secret"}`, CreatedAt: now,
	}
	auditLog.ID = "fixture-audit-log-row"
	auditLog.UpdatedAt = now
	require.NoError(t, withoutHooks.Create(auditLog).Error)
}

func (fixture *presentationBusinessAPIFixture) insertPublishedProfiles(t *testing.T, endpoints []presentationBusinessEndpoint) {
	t.Helper()
	definitions := make(map[string]presentation.CapabilityDefinition)
	for _, definition := range presentationcore.Definitions() {
		definitions[definition.PageKey] = definition
	}
	for _, endpoint := range endpoints {
		definition := definitions[endpoint.pageKey]
		configuredTitle := "configured-" + strings.ReplaceAll(endpoint.pageKey, ".", "-")
		pageSize := 50
		document := presentation.Profile{
			APIVersion: presentation.APIVersion,
			Kind:       presentation.Kind,
			Metadata: presentation.Metadata{
				Name:           "invariance-" + strings.ReplaceAll(endpoint.pageKey, ".", "-"),
				PageKey:        endpoint.pageKey,
				DefinitionHash: definition.DefinitionHash,
				Scope:          presentation.Scope{Kind: presentation.ScopeApplication},
			},
			Spec: presentation.ProfileSpec{
				Title: &presentation.LocalizedText{ZhCN: &configuredTitle, EnUS: &configuredTitle},
				List:  &presentation.ListPatch{PageSize: &pageSize},
			},
		}
		raw, err := json.Marshal(document)
		require.NoError(t, err)
		created, err := fixture.presentationService.CreateDraft(
			nil,
			dto.PresentationProfileIdentity{
				Scope: presentation.ScopeApplication, PageKey: endpoint.pageKey,
			},
			raw,
			"fixture-principal-user",
		)
		require.NoError(t, err, endpoint.pageKey)
		published, err := fixture.presentationService.Publish(
			nil,
			created.ID,
			created.Version,
			"publish-invariance-"+strings.ReplaceAll(endpoint.pageKey, ".", "-"),
			"fixture-principal-user",
		)
		require.NoError(t, err, endpoint.pageKey)
		require.EqualValues(t, 1, published.Profile.PublishedRevision, endpoint.pageKey)

		effective, err := fixture.presentationService.Effective(
			nil, endpoint.pageKey, "fixture-principal-user", businessOnlyRole,
		)
		require.NoError(t, err, endpoint.pageKey)
		require.False(t, effective.Fallback, endpoint.pageKey)
		require.Contains(t, string(effective.Layers.Application), configuredTitle, endpoint.pageKey)
	}
}

func (fixture *presentationBusinessAPIFixture) get(t *testing.T, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	return fixture.request(t, http.MethodGet, path, token, "")
}

func (fixture *presentationBusinessAPIFixture) request(
	t *testing.T,
	method string,
	path string,
	token string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	result := httptest.NewRecorder()
	fixture.router.ServeHTTP(result, request)
	return result
}
