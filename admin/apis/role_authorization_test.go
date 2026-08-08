package apis

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	adminpkg "github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	bootenum "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
)

const roleAuthorizationAPITestModel = `
[request_definition]
r = sub, typ, obj, act

[policy_definition]
p = sub, typ, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.typ == p.typ && r.obj == p.obj && r.act == p.act
`

func setupRoleAuthorizationAPITest(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database := &gormdb.Database{
		Driver:      "sqlite",
		Source:      fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()),
		CasbinModel: roleAuthorizationAPITestModel,
	}
	handle, err := database.Open(t.Context())
	require.NoError(t, err)
	previousHandle := gormdb.InstallDefault(handle)
	previousPolicies := service.AuthorizationPolicies
	service.AuthorizationPolicies = service.NewAuthorizationPolicyService()
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = "role-authorization-test-identity"
	t.Cleanup(func() {
		config.Cfg.Auth.IdentityKey = previousIdentityKey
		service.AuthorizationPolicies = previousPolicies
		gormdb.ClearDefault(handle)
		if previousHandle != nil {
			gormdb.InstallDefault(previousHandle)
		}
		_ = handle.Close()
	})
	require.NoError(t, handle.DB.AutoMigrate(
		&models.Role{},
		&models.Menu{},
		&models.ConfigRevision{},
	))
	require.NoError(t, handle.DB.Exec(`CREATE TABLE IF NOT EXISTS mss_boot_users (
		id varchar(64) PRIMARY KEY,
		role_id varchar(64),
		deleted_at datetime
	)`).Error)
	return handle.DB
}

func insertRoleAuthorizationTestRole(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	role := models.Role{Name: id, Status: bootenum.Enabled}
	role.ID = id
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&role).Error)
}

func roleAuthorizationPrincipal(root bool) *models.User {
	return &models.User{UserLogin: models.UserLogin{
		RoleID: "actor-role",
		Role:   &models.Role{Root: root, Status: bootenum.Enabled},
	}}
}

func roleAuthorizationResponse(
	method, roleID, body, ifMatch string,
	principal *models.User,
) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "roleID", Value: roleID}}
	ctx.Request = httptest.NewRequest(method, "/admin/api/role/authorize/"+roleID, bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		ctx.Request.Header.Set("If-Match", ifMatch)
	}
	if principal != nil {
		ctx.Set(config.Cfg.Auth.IdentityKey, principal)
	}
	if method == http.MethodGet {
		(&Role{}).GetAuthorize(ctx)
	} else {
		(&Role{}).SetAuthorize(ctx)
	}
	return recorder
}

func authorityMutationResponse(kind string, principal *models.User) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/api/authority", bytes.NewBufferString(`{}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if principal != nil {
		ctx.Set(config.Cfg.Auth.IdentityKey, principal)
	}
	switch kind {
	case "role":
		ctx.Params = gin.Params{{Key: "roleID", Value: "target"}}
		(&Role{}).SetAuthorize(ctx)
	case "menu":
		ctx.Request.Method = http.MethodPut
		ctx.Params = gin.Params{{Key: "roleID", Value: "target"}}
		(&Menu{}).UpdateAuthorize(ctx)
	case "bind-api":
		(&Menu{}).BindAPI(ctx)
	}
	return recorder
}

func menuBindAPIResponse(body string, principal *models.User) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/api/menu/bind-api", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if principal != nil {
		ctx.Set(config.Cfg.Auth.IdentityKey, principal)
	}
	(&Menu{}).BindAPI(ctx)
	return recorder
}

func menuAuthorizeProjectionResponse(principal *models.User) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/api/menu/authorize", nil)
	if principal != nil {
		ctx.Set(config.Cfg.Auth.IdentityKey, principal)
	}
	(&Menu{}).GetAuthorize(ctx)
	return recorder
}

func TestAuthorityMutationRequiresCurrentRootEvenWithLegacyGrant(t *testing.T) {
	db := setupRoleAuthorizationAPITest(t)
	insertRoleAuthorizationTestRole(t, db, "target")
	require.NoError(t, db.Create(&models.CasbinRule{
		PType: "p", V0: "actor-role", V1: adminpkg.APIAccessType.String(),
		V2: "/admin/api/role/authorize/*", V3: http.MethodPost,
	}).Error)
	require.NoError(t, gormdb.Enforcer.LoadPolicy())

	for _, kind := range []string{"role", "menu", "bind-api"} {
		t.Run(kind+" missing principal", func(t *testing.T) {
			response := authorityMutationResponse(kind, nil)
			require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
		})
		t.Run(kind+" delegated principal", func(t *testing.T) {
			response := authorityMutationResponse(kind, roleAuthorizationPrincipal(false))
			require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
		})
	}
}

func TestMenuBindAPIMissingOrNullPathsRejectsAndExplicitEmptyRevokes(t *testing.T) {
	db := setupRoleAuthorizationAPITest(t)
	parent := models.Menu{
		Path: "/menu-a", Method: http.MethodGet, Type: adminpkg.MenuAccessType, Status: bootenum.Enabled,
	}
	parent.ID = "menu-a"
	child := models.Menu{
		ParentID: parent.ID, Path: "/admin/api/old", Method: http.MethodGet,
		Type: adminpkg.APIAccessType, Status: bootenum.Enabled,
	}
	child.ID = "api-old"
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&[]models.Menu{parent, child}).Error)
	require.NoError(t, db.Create(&[]models.CasbinRule{
		{PType: "p", V0: "role-a", V1: adminpkg.MenuAccessType.String(), V2: parent.Path, V3: parent.Method},
		{PType: "p", V0: "role-a", V1: adminpkg.APIAccessType.String(), V2: child.Path, V3: child.Method},
	}).Error)
	require.NoError(t, gormdb.Enforcer.LoadPolicy())
	root := roleAuthorizationPrincipal(true)

	for name, body := range map[string]string{
		"missing": `{"menuID":"menu-a"}`,
		"null":    `{"menuID":"menu-a","paths":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			response := menuBindAPIResponse(body, root)
			require.Equal(t, http.StatusUnprocessableEntity, response.Code, response.Body.String())
		})
	}
	var beforeCount int64
	require.NoError(t, db.Model(&models.CasbinRule{}).
		Where("v0 = ? AND v1 = ?", "role-a", adminpkg.APIAccessType.String()).Count(&beforeCount).Error)
	require.Equal(t, int64(1), beforeCount)

	cleared := menuBindAPIResponse(`{"menuID":"menu-a","paths":[]}`, root)
	require.Equal(t, http.StatusCreated, cleared.Code, cleared.Body.String())
	var childCount int64
	require.NoError(t, db.Unscoped().Model(&models.Menu{}).
		Where("parent_id = ? AND type = ?", parent.ID, adminpkg.APIAccessType).Count(&childCount).Error)
	require.Zero(t, childCount)
	var afterCount int64
	require.NoError(t, db.Model(&models.CasbinRule{}).
		Where("v0 = ? AND v1 = ?", "role-a", adminpkg.APIAccessType.String()).Count(&afterCount).Error)
	require.Zero(t, afterCount)
}

func TestMenuAuthorizeProjectionFailsClosedWhenDurableReconciliationFails(t *testing.T) {
	_ = setupRoleAuthorizationAPITest(t)
	previous := ensureCurrentAuthorizationPolicies
	reloadErr := errors.New("reload failed")
	ensureCurrentAuthorizationPolicies = func(*gin.Context, *gorm.DB) error { return reloadErr }
	t.Cleanup(func() { ensureCurrentAuthorizationPolicies = previous })

	response := menuAuthorizeProjectionResponse(roleAuthorizationPrincipal(false))
	require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
}

func TestMenuAuthorizeProjectionRootBypassesPolicyInfrastructure(t *testing.T) {
	db := setupRoleAuthorizationAPITest(t)
	previousEnsure := ensureCurrentAuthorizationPolicies
	previousEnforce := enforceMenuAuthorization
	ensureCalls := 0
	enforceCalls := 0
	ensureCurrentAuthorizationPolicies = func(*gin.Context, *gorm.DB) error {
		ensureCalls++
		return errors.New("reconcile unavailable")
	}
	enforceMenuAuthorization = func(string, string, string, string) (bool, error) {
		enforceCalls++
		return false, errors.New("enforcer unavailable")
	}
	t.Cleanup(func() {
		ensureCurrentAuthorizationPolicies = previousEnsure
		enforceMenuAuthorization = previousEnforce
	})
	menu := models.Menu{
		Path: "/system-config", Method: http.MethodGet, Type: adminpkg.MenuAccessType, Status: bootenum.Enabled,
	}
	menu.ID = "system-config"
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&menu).Error)

	response := menuAuthorizeProjectionResponse(roleAuthorizationPrincipal(true))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Zero(t, ensureCalls, "root must not reconcile Casbin policy")
	require.Zero(t, enforceCalls, "root must not call the Casbin enforcer")
	var menus []*models.Menu
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &menus))
	require.True(t, menuTreeContainsPath(menus, menu.Path))
}

func TestMenuAuthorizeProjectionHidesRootOnlyPagesFromDelegatedRole(t *testing.T) {
	db := setupRoleAuthorizationAPITest(t)
	previous := ensureCurrentAuthorizationPolicies
	ensureCurrentAuthorizationPolicies = func(*gin.Context, *gorm.DB) error { return nil }
	t.Cleanup(func() { ensureCurrentAuthorizationPolicies = previous })
	securityDirectory := models.Menu{
		Path: "/security", Type: adminpkg.DirectoryAccessType, Status: bootenum.Enabled,
	}
	securityDirectory.ID = "security-directory"
	menus := []models.Menu{
		securityDirectory,
		{ParentID: securityDirectory.ID, Path: "/security/online-sessions", Method: http.MethodGet, Type: adminpkg.MenuAccessType, Status: bootenum.Enabled},
		{Path: "/system-config", Method: http.MethodGet, Type: adminpkg.MenuAccessType, Status: bootenum.Enabled},
		{Path: "/workplace", Method: http.MethodGet, Type: adminpkg.MenuAccessType, Status: bootenum.Enabled},
		{Path: "/disabled", Method: http.MethodGet, Type: adminpkg.MenuAccessType, Status: bootenum.Disabled},
	}
	menus[1].ID = "online-sessions"
	menus[2].ID = "system-config"
	menus[3].ID = "workplace"
	menus[4].ID = "disabled"
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&menus).Error)
	for _, menu := range menus[1:] {
		require.NoError(t, db.Create(&models.CasbinRule{
			PType: "p", V0: "actor-role", V1: adminpkg.MenuAccessType.String(), V2: menu.Path, V3: menu.Method,
		}).Error)
	}
	require.NoError(t, gormdb.Enforcer.LoadPolicy())

	delegated := menuAuthorizeProjectionResponse(roleAuthorizationPrincipal(false))
	require.Equal(t, http.StatusOK, delegated.Code, delegated.Body.String())
	var delegatedMenus []*models.Menu
	require.NoError(t, json.Unmarshal(delegated.Body.Bytes(), &delegatedMenus))
	require.False(t, menuTreeContainsPath(delegatedMenus, "/system-config"))
	require.False(t, menuTreeContainsPath(delegatedMenus, "/security/online-sessions"))
	require.False(t, menuTreeContainsPath(delegatedMenus, "/security"), "empty protected directory must be pruned")
	require.True(t, menuTreeContainsPath(delegatedMenus, "/workplace"))
	require.False(t, menuTreeContainsPath(delegatedMenus, "/disabled"), "disabled legacy policy must not project a menu")

	root := menuAuthorizeProjectionResponse(roleAuthorizationPrincipal(true))
	require.Equal(t, http.StatusOK, root.Code, root.Body.String())
	var rootMenus []*models.Menu
	require.NoError(t, json.Unmarshal(root.Body.Bytes(), &rootMenus))
	require.True(t, menuTreeContainsPath(rootMenus, "/system-config"))
	require.True(t, menuTreeContainsPath(rootMenus, "/security/online-sessions"))
	require.True(t, menuTreeContainsPath(rootMenus, "/security"))
	require.False(t, menuTreeContainsPath(rootMenus, "/disabled"), "disabled menus remain unavailable to root")
}

func menuTreeContainsPath(menus []*models.Menu, path string) bool {
	for _, menu := range menus {
		if menu.Path == path || menuTreeContainsPath(menu.Children, path) {
			return true
		}
	}
	return false
}

func TestRoleAuthorizationMissingPathsRejectsAndExplicitEmptyClears(t *testing.T) {
	db := setupRoleAuthorizationAPITest(t)
	insertRoleAuthorizationTestRole(t, db, "role-a")
	require.NoError(t, db.Create(&models.CasbinRule{
		PType: "p", V0: "role-a", V1: adminpkg.MenuAccessType.String(), V2: "/menu", V3: "GET",
	}).Error)
	require.NoError(t, gormdb.Enforcer.LoadPolicy())
	root := roleAuthorizationPrincipal(true)

	missing := roleAuthorizationResponse(http.MethodPost, "role-a", `{}`, "", root)
	require.Equal(t, http.StatusUnprocessableEntity, missing.Code, missing.Body.String())
	var policyCount int64
	require.NoError(t, db.Model(&models.CasbinRule{}).Where("v0 = ?", "role-a").Count(&policyCount).Error)
	require.Equal(t, int64(1), policyCount)

	cleared := roleAuthorizationResponse(http.MethodPost, "role-a", `{"paths":[]}`, "", root)
	require.Equal(t, http.StatusCreated, cleared.Code, cleared.Body.String())
	require.Equal(t, `"role-authorization-role-a-1"`, cleared.Header().Get("ETag"))
	require.Equal(t, "no-store", cleared.Header().Get("Cache-Control"))
	require.Equal(t, "missing", cleared.Header().Get(roleAuthorizationPreconditionHeader))
	require.Contains(t, cleared.Header().Get("Warning"), "If-Match will be required")
	var resource dto.GetAuthorizeResponse
	require.NoError(t, json.Unmarshal(cleared.Body.Bytes(), &resource))
	require.Equal(t, "role-a", resource.RoleID)
	require.Equal(t, "1", resource.Revision)
	require.NotNil(t, resource.Paths)
	require.Empty(t, resource.Paths)
	require.NoError(t, db.Model(&models.CasbinRule{}).Where("v0 = ?", "role-a").Count(&policyCount).Error)
	require.Zero(t, policyCount)

	read := roleAuthorizationResponse(http.MethodGet, "role-a", "", "", roleAuthorizationPrincipal(false))
	require.Equal(t, http.StatusOK, read.Code, read.Body.String())
	require.Equal(t, cleared.Header().Get("ETag"), read.Header().Get("ETag"))
}

func TestRoleAuthorizationStaleIfMatchReturnsCanonicalCurrentResource(t *testing.T) {
	db := setupRoleAuthorizationAPITest(t)
	insertRoleAuthorizationTestRole(t, db, "role-a")
	menu := models.Menu{Path: "/menu-a", Method: "GET", Type: adminpkg.MenuAccessType, Status: bootenum.Enabled}
	menu.ID = "menu-a"
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&menu).Error)
	root := roleAuthorizationPrincipal(true)

	initial := roleAuthorizationResponse(http.MethodGet, "role-a", "", "", root)
	require.Equal(t, http.StatusOK, initial.Code, initial.Body.String())
	require.Equal(t, `"role-authorization-role-a-0"`, initial.Header().Get("ETag"))
	first := roleAuthorizationResponse(
		http.MethodPost, "role-a", `{"paths":["/menu-a"]}`, initial.Header().Get("ETag"), root,
	)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	require.Equal(t, `"role-authorization-role-a-1"`, first.Header().Get("ETag"))

	stale := roleAuthorizationResponse(http.MethodPost, "role-a", `{"paths":[]}`, initial.Header().Get("ETag"), root)
	require.Equal(t, http.StatusPreconditionFailed, stale.Code, stale.Body.String())
	require.Equal(t, first.Header().Get("ETag"), stale.Header().Get("ETag"))
	var conflict dto.AuthorizeRevisionConflictResponse
	require.NoError(t, json.Unmarshal(stale.Body.Bytes(), &conflict))
	require.Equal(t, roleAuthorizationRevisionConflictCode, conflict.ErrorCode)
	require.Equal(t, "1", conflict.Data.Current.Revision)
	require.Equal(t, []string{"/menu-a"}, conflict.Data.Current.Paths)

	bad := roleAuthorizationResponse(http.MethodPost, "role-a", `{"paths":[]}`, `W/"role-authorization-role-a-1"`, root)
	require.Equal(t, http.StatusBadRequest, bad.Code, bad.Body.String())
}

func TestRoleAuthorizationMissingAndInactiveTargetsFailClosed(t *testing.T) {
	db := setupRoleAuthorizationAPITest(t)
	insertRoleAuthorizationTestRole(t, db, "role-inactive")
	require.NoError(t, db.Model(&models.Role{}).
		Where("id = ?", "role-inactive").
		Update("status", bootenum.Disabled).Error)
	root := roleAuthorizationPrincipal(true)

	missingRead := roleAuthorizationResponse(http.MethodGet, "missing-role", "", "", root)
	require.Equal(t, http.StatusNotFound, missingRead.Code, missingRead.Body.String())
	missingWrite := roleAuthorizationResponse(http.MethodPost, "missing-role", `{"paths":[]}`, "", root)
	require.Equal(t, http.StatusNotFound, missingWrite.Code, missingWrite.Body.String())
	inactiveWrite := roleAuthorizationResponse(http.MethodPost, "role-inactive", `{"paths":[]}`, "", root)
	require.Equal(t, http.StatusConflict, inactiveWrite.Code, inactiveWrite.Body.String())
	require.Contains(t, inactiveWrite.Body.String(), roleAuthorizationInactiveCode)
}

func managedRoleLifecycleResponse(method, roleID string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: roleID}}
	ctx.Request = httptest.NewRequest(method, "/admin/api/roles/"+roleID, nil)
	protectManagedRoleLifecycle(ctx)
	return recorder
}

func TestManagedRoleLifecycleRejectsRootAndDefaultButAllowsOrdinaryRole(t *testing.T) {
	db := setupRoleAuthorizationAPITest(t)
	for _, roleID := range []string{"root-role", "default-role", "ordinary-role"} {
		insertRoleAuthorizationTestRole(t, db, roleID)
	}
	// Root and Default are intentionally GORM read-only, so test fixtures use
	// SQL to represent rows provisioned by migration/bootstrap code.
	require.NoError(t, db.Exec("UPDATE mss_boot_roles SET root = ? WHERE id = ?", true, "root-role").Error)
	require.NoError(t, db.Exec("UPDATE mss_boot_roles SET `default` = ? WHERE id = ?", true, "default-role").Error)

	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		require.Equal(t, http.StatusForbidden, managedRoleLifecycleResponse(method, "root-role").Code)
		require.Equal(t, http.StatusForbidden, managedRoleLifecycleResponse(method, "default-role").Code)
		ordinary := managedRoleLifecycleResponse(method, "ordinary-role")
		require.False(t, ordinary.Code == http.StatusForbidden, ordinary.Body.String())
	}
	// Create requests share the controller's control chain but must skip the
	// target-role lifecycle lookup; requireRootManagement still guards them.
	require.NotEqual(t, http.StatusUnprocessableEntity, managedRoleLifecycleResponse(http.MethodPost, "").Code)
	require.Equal(t, http.StatusNotFound, managedRoleLifecycleResponse(http.MethodDelete, "missing").Code)
}

func TestOrdinaryRoleDeleteRequiresClearedPoliciesAndUserAssignments(t *testing.T) {
	db := setupRoleAuthorizationAPITest(t)
	insertRoleAuthorizationTestRole(t, db, "policy-role")
	insertRoleAuthorizationTestRole(t, db, "user-role")
	require.NoError(t, db.Create(&models.CasbinRule{
		PType: "p", V0: "policy-role", V1: adminpkg.MenuAccessType.String(), V2: "/menu", V3: http.MethodGet,
	}).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO mss_boot_users (id, role_id) VALUES (?, ?)", "user-a", "user-role",
	).Error)

	policyConflict := managedRoleLifecycleResponse(http.MethodDelete, "policy-role")
	require.Equal(t, http.StatusConflict, policyConflict.Code, policyConflict.Body.String())
	require.Contains(t, policyConflict.Body.String(), roleDeletePoliciesConflictCode)
	userConflict := managedRoleLifecycleResponse(http.MethodDelete, "user-role")
	require.Equal(t, http.StatusConflict, userConflict.Code, userConflict.Body.String())
	require.Contains(t, userConflict.Body.String(), roleDeleteUsersConflictCode)
}
