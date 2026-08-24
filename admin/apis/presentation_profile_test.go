package apis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/presentation"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPresentationProfileAPIPreconditionsConflictsIdempotencyAndHistory(t *testing.T) {
	db, router, capability := setupPresentationProfileAPITest(t)
	document := apiPresentationDocument(t, capability, presentation.ScopeApplication, "", 20)
	createBody := apiPresentationCreateBody(t, presentation.ScopeApplication, "", capability.PageKey, document)

	missingCreatePrecondition := presentationAPIRequest(router, http.MethodPost, "/admin/api/presentation-profiles", createBody, nil)
	require.Equal(t, http.StatusPreconditionRequired, missingCreatePrecondition.Code, missingCreatePrecondition.Body.String())

	unknownOuterField := strings.TrimSuffix(createBody, "}") + `,"unexpected":true}`
	invalidCreate := presentationAPIRequest(router, http.MethodPost, "/admin/api/presentation-profiles", unknownOuterField, map[string]string{
		"If-None-Match": "*",
	})
	require.Equal(t, http.StatusUnprocessableEntity, invalidCreate.Code, invalidCreate.Body.String())

	createdResponse := presentationAPIRequest(router, http.MethodPost, "/admin/api/presentation-profiles", createBody, map[string]string{
		"If-None-Match": "*",
	})
	require.Equal(t, http.StatusCreated, createdResponse.Code, createdResponse.Body.String())
	created := decodePresentationProfileResource(t, createdResponse)
	require.Equal(t, int64(1), created.Version)
	require.Equal(t, presentationProfileETag(created.ID, created.Version), createdResponse.Header().Get("ETag"))

	detail := presentationAPIRequest(router, http.MethodGet, "/admin/api/presentation-profiles/"+created.ID, "", nil)
	require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())
	require.Equal(t, presentationProfileETag(created.ID, 1), detail.Header().Get("ETag"))

	updatedDocument := apiPresentationDocument(t, capability, presentation.ScopeApplication, "", 50)
	replaceBody := apiPresentationReplaceBody(t, updatedDocument)
	missingReplacePrecondition := presentationAPIRequest(
		router, http.MethodPut, "/admin/api/presentation-profiles/"+created.ID+"/draft", replaceBody, nil,
	)
	require.Equal(t, http.StatusPreconditionRequired, missingReplacePrecondition.Code, missingReplacePrecondition.Body.String())

	updatedResponse := presentationAPIRequest(
		router,
		http.MethodPut,
		"/admin/api/presentation-profiles/"+created.ID+"/draft",
		replaceBody,
		map[string]string{"If-Match": presentationProfileETag(created.ID, 1)},
	)
	require.Equal(t, http.StatusOK, updatedResponse.Code, updatedResponse.Body.String())
	updated := decodePresentationProfileResource(t, updatedResponse)
	require.Equal(t, int64(2), updated.Version)

	staleResponse := presentationAPIRequest(
		router,
		http.MethodPut,
		"/admin/api/presentation-profiles/"+created.ID+"/draft",
		apiPresentationReplaceBody(t, apiPresentationDocument(t, capability, presentation.ScopeApplication, "", 100)),
		map[string]string{"If-Match": presentationProfileETag(created.ID, 1)},
	)
	require.Equal(t, http.StatusPreconditionFailed, staleResponse.Code, staleResponse.Body.String())
	require.Equal(t, presentationProfileETag(created.ID, 2), staleResponse.Header().Get("ETag"))
	var conflict struct {
		ErrorCode string `json:"errorCode"`
		Data      struct {
			Current map[string]any `json:"current"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(staleResponse.Body.Bytes(), &conflict), staleResponse.Body.String())
	require.Equal(t, "PRESENTATION_REVISION_CONFLICT", conflict.ErrorCode)
	require.Equal(t, map[string]any{"id": created.ID, "version": float64(2)}, conflict.Data.Current)

	missingIdempotency := presentationAPIRequest(
		router,
		http.MethodPost,
		"/admin/api/presentation-profiles/"+created.ID+"/publish",
		"",
		map[string]string{"If-Match": presentationProfileETag(created.ID, 2)},
	)
	require.Equal(t, http.StatusBadRequest, missingIdempotency.Code, missingIdempotency.Body.String())

	firstPublishHeaders := map[string]string{
		"If-Match":        presentationProfileETag(created.ID, 2),
		"Idempotency-Key": "publish-api-first",
	}
	publishedResponse := presentationAPIRequest(
		router, http.MethodPost, "/admin/api/presentation-profiles/"+created.ID+"/publish", "", firstPublishHeaders,
	)
	require.Equal(t, http.StatusOK, publishedResponse.Code, publishedResponse.Body.String())
	firstPublish := decodePresentationTransition(t, publishedResponse)
	require.False(t, firstPublish.Replayed)
	require.Equal(t, int64(3), firstPublish.Profile.Version)
	require.Equal(t, int64(1), firstPublish.Revision.Revision)

	replayResponse := presentationAPIRequest(
		router, http.MethodPost, "/admin/api/presentation-profiles/"+created.ID+"/publish", "", firstPublishHeaders,
	)
	require.Equal(t, http.StatusOK, replayResponse.Code, replayResponse.Body.String())
	replay := decodePresentationTransition(t, replayResponse)
	require.True(t, replay.Replayed)
	require.Equal(t, firstPublish.Revision.ContentDigest, replay.Revision.ContentDigest)

	conflictingReuse := presentationAPIRequest(
		router,
		http.MethodPost,
		"/admin/api/presentation-profiles/"+created.ID+"/publish",
		"",
		map[string]string{
			"If-Match":        presentationProfileETag(created.ID, 3),
			"Idempotency-Key": "publish-api-first",
		},
	)
	require.Equal(t, http.StatusConflict, conflictingReuse.Code, conflictingReuse.Body.String())

	secondDraftResponse := presentationAPIRequest(
		router,
		http.MethodPut,
		"/admin/api/presentation-profiles/"+created.ID+"/draft",
		apiPresentationReplaceBody(t, apiPresentationDocument(t, capability, presentation.ScopeApplication, "", 100)),
		map[string]string{"If-Match": presentationProfileETag(created.ID, 3)},
	)
	require.Equal(t, http.StatusOK, secondDraftResponse.Code, secondDraftResponse.Body.String())
	secondDraft := decodePresentationProfileResource(t, secondDraftResponse)

	secondPublishResponse := presentationAPIRequest(
		router,
		http.MethodPost,
		"/admin/api/presentation-profiles/"+created.ID+"/publish",
		"",
		map[string]string{
			"If-Match":        presentationProfileETag(created.ID, secondDraft.Version),
			"Idempotency-Key": "publish-api-second",
		},
	)
	require.Equal(t, http.StatusOK, secondPublishResponse.Code, secondPublishResponse.Body.String())
	secondPublish := decodePresentationTransition(t, secondPublishResponse)

	rollbackResponse := presentationAPIRequest(
		router,
		http.MethodPost,
		"/admin/api/presentation-profiles/"+created.ID+"/rollback",
		`{"revision":1}`,
		map[string]string{
			"If-Match":        presentationProfileETag(created.ID, secondPublish.Profile.Version),
			"Idempotency-Key": "rollback-api-first",
		},
	)
	require.Equal(t, http.StatusOK, rollbackResponse.Code, rollbackResponse.Body.String())
	rollback := decodePresentationTransition(t, rollbackResponse)
	require.Equal(t, models.PresentationTransitionRollback, rollback.Revision.Transition)
	require.NotNil(t, rollback.Revision.SourceRevision)
	require.Equal(t, int64(1), *rollback.Revision.SourceRevision)

	history := presentationAPIRequest(
		router, http.MethodGet, "/admin/api/presentation-profiles/"+created.ID+"/revisions?page=1&pageSize=100", "", nil,
	)
	require.Equal(t, http.StatusOK, history.Code, history.Body.String())
	require.NotContains(t, history.Body.String(), "publish-api-first")
	require.NotContains(t, history.Body.String(), "rollback-api-first")
	var page dto.PresentationRevisionListResponse
	require.NoError(t, json.Unmarshal(history.Body.Bytes(), &page), history.Body.String())
	require.Equal(t, int64(3), page.Total)
	require.Equal(t, []int64{3, 2, 1}, []int64{page.Items[0].Revision, page.Items[1].Revision, page.Items[2].Revision})

	var revisionCount int64
	require.NoError(t, db.Model(&models.PresentationRevision{}).Where("profile_id = ?", created.ID).Count(&revisionCount).Error)
	require.Equal(t, int64(3), revisionCount)
}

func TestPresentationProfileAPIConcurrentConditionalWritesCommitOnce(t *testing.T) {
	_, router, capability := setupPresentationProfileAPITest(t)
	createBody := apiPresentationCreateBody(
		t,
		presentation.ScopeApplication,
		"",
		capability.PageKey,
		apiPresentationDocument(t, capability, presentation.ScopeApplication, "", 20),
	)
	createdResponse := presentationAPIRequest(
		router,
		http.MethodPost,
		"/admin/api/presentation-profiles",
		createBody,
		map[string]string{"If-None-Match": "*"},
	)
	require.Equal(t, http.StatusCreated, createdResponse.Code, createdResponse.Body.String())
	created := decodePresentationProfileResource(t, createdResponse)

	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, 2)
	var writers sync.WaitGroup
	bodies := []string{
		apiPresentationReplaceBody(
			t,
			apiPresentationDocument(t, capability, presentation.ScopeApplication, "", 50),
		),
		apiPresentationReplaceBody(
			t,
			apiPresentationDocument(t, capability, presentation.ScopeApplication, "", 100),
		),
	}
	for _, body := range bodies {
		body := body
		writers.Add(1)
		go func() {
			defer writers.Done()
			<-start
			responses <- presentationAPIRequest(
				router,
				http.MethodPut,
				"/admin/api/presentation-profiles/"+created.ID+"/draft",
				body,
				map[string]string{"If-Match": presentationProfileETag(created.ID, created.Version)},
			)
		}()
	}
	close(start)
	writers.Wait()
	close(responses)

	statusCounts := map[int]int{}
	for response := range responses {
		statusCounts[response.Code]++
		if response.Code == http.StatusOK || response.Code == http.StatusPreconditionFailed {
			require.Equal(t, presentationProfileETag(created.ID, 2), response.Header().Get("ETag"))
		}
	}
	require.Equal(t, 1, statusCounts[http.StatusOK])
	require.Equal(t, 1, statusCounts[http.StatusPreconditionFailed])

	detail := presentationAPIRequest(
		router,
		http.MethodGet,
		"/admin/api/presentation-profiles/"+created.ID,
		"",
		nil,
	)
	require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())
	current := decodePresentationProfileResource(t, detail)
	require.Equal(t, int64(2), current.Version)
}

func TestPresentationProfileAPIEffectiveReadUsesOnlyAuthenticatedSubjects(t *testing.T) {
	_, router, capability := setupPresentationProfileAPITest(t)

	profiles := []struct {
		scope     presentation.ScopeKind
		subjectID string
		pageSize  int
		key       string
	}{
		{scope: presentation.ScopeApplication, pageSize: 20, key: "effective-app"},
		{scope: presentation.ScopeRole, subjectID: "role-current", pageSize: 30, key: "effective-role"},
		{scope: presentation.ScopeUser, subjectID: "user-current", pageSize: 40, key: "effective-user"},
	}
	for _, profile := range profiles {
		body := apiPresentationCreateBody(
			t,
			profile.scope,
			profile.subjectID,
			capability.PageKey,
			apiPresentationDocument(t, capability, profile.scope, profile.subjectID, profile.pageSize),
		)
		createdResponse := presentationAPIRequest(router, http.MethodPost, "/admin/api/presentation-profiles", body, map[string]string{
			"If-None-Match": "*",
		})
		require.Equal(t, http.StatusCreated, createdResponse.Code, createdResponse.Body.String())
		created := decodePresentationProfileResource(t, createdResponse)
		published := presentationAPIRequest(
			router,
			http.MethodPost,
			"/admin/api/presentation-profiles/"+created.ID+"/publish",
			"",
			map[string]string{
				"If-Match":        presentationProfileETag(created.ID, created.Version),
				"Idempotency-Key": profile.key,
			},
		)
		require.Equal(t, http.StatusOK, published.Code, published.Body.String())
	}

	effectiveResponse := presentationAPIRequest(
		router,
		http.MethodGet,
		"/admin/api/presentation/effective/"+capability.PageKey+"?userID=attacker&roleID=attacker",
		"",
		nil,
	)
	require.Equal(t, http.StatusOK, effectiveResponse.Code, effectiveResponse.Body.String())
	require.Equal(t, "no-store", effectiveResponse.Header().Get("Cache-Control"))
	var effective dto.EffectivePresentationResponse
	require.NoError(t, json.Unmarshal(effectiveResponse.Body.Bytes(), &effective), effectiveResponse.Body.String())
	require.Equal(t, 20, apiPresentationPageSize(t, effective.Layers.Application))
	require.Equal(t, 30, apiPresentationPageSize(t, effective.Layers.Role))
	require.Equal(t, 40, apiPresentationPageSize(t, effective.Layers.User))
}

func TestPresentationProfileAPIRequiresCanonicalMutationETag(t *testing.T) {
	profileID := "profile-1"
	canonical := presentationProfileETag(profileID, 7)
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "canonical", value: canonical, valid: true},
		{name: "missing", value: ""},
		{name: "weak", value: "W/" + canonical},
		{name: "wildcard", value: "*"},
		{name: "list", value: canonical + ", " + presentationProfileETag(profileID, 8)},
		{name: "wrong identity", value: presentationProfileETag("profile-2", 7)},
		{name: "unquoted", value: "presentation-profile-profile-1-7"},
		{name: "zero", value: `"presentation-profile-profile-1-0"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPut, "/", nil)
			if test.value != "" {
				ctx.Request.Header.Set("If-Match", test.value)
			}
			version, err := parsePresentationIfMatch(ctx, profileID)
			if test.valid {
				require.NoError(t, err)
				require.Equal(t, int64(7), version)
				return
			}
			require.Error(t, err)
		})
	}
}

func setupPresentationProfileAPITest(t *testing.T) (*gorm.DB, *gin.Engine, presentation.CapabilityDefinition) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "-"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&models.PresentationProfile{}, &models.PresentationRevision{}, &models.Role{}, &models.User{},
	))

	role := &models.Role{Name: "Current role", Status: enum.Enabled}
	role.ID = "role-current"
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(role).Error)
	user := &models.User{UserLogin: models.UserLogin{
		Username: "current-user", RoleID: role.ID, Role: role, Status: enum.Enabled,
	}}
	user.ID = "user-current"
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Omit("Role", "Post", "Department", "OAuth2").Create(user).Error)

	capability := apiPresentationCapability(t)
	api := newPresentationProfileController()
	api.service = &service.PresentationProfileService{
		Database: db,
		Registry: presentation.MustNewRegistry(capability),
	}
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	previousAuthHandler := response.AuthHandler
	config.Cfg.Auth.IdentityKey = "presentation-api-test-identity"
	response.AuthHandler = func(ctx *gin.Context) { ctx.Next() }
	t.Cleanup(func() {
		config.Cfg.Auth.IdentityKey = previousIdentityKey
		response.AuthHandler = previousAuthHandler
		_ = sqlDB.Close()
	})

	principal := &models.User{UserLogin: models.UserLogin{
		Username: user.Username,
		RoleID:   role.ID,
		Role:     role,
	}}
	principal.ID = user.ID
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(config.Cfg.Auth.IdentityKey, principal)
		ctx.Next()
	})
	api.Other(router.Group("/admin/api"))
	return db, router, capability
}

func presentationAPIRequest(
	router http.Handler,
	method string,
	path string,
	body string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func apiPresentationCreateBody(
	t *testing.T,
	scope presentation.ScopeKind,
	subjectID string,
	pageKey string,
	document json.RawMessage,
) string {
	t.Helper()
	encoded, err := json.Marshal(dto.PresentationProfileCreateRequest{
		PresentationProfileIdentity: dto.PresentationProfileIdentity{
			Scope: scope, SubjectID: subjectID, PageKey: pageKey,
		},
		Document: document,
	})
	require.NoError(t, err)
	return string(encoded)
}

func apiPresentationReplaceBody(t *testing.T, document json.RawMessage) string {
	t.Helper()
	encoded, err := json.Marshal(dto.PresentationDraftReplaceRequest{Document: document})
	require.NoError(t, err)
	return string(encoded)
}

func decodePresentationProfileResource(t *testing.T, response *httptest.ResponseRecorder) *dto.PresentationProfileResource {
	t.Helper()
	resource := &dto.PresentationProfileResource{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), resource), response.Body.String())
	return resource
}

func decodePresentationTransition(t *testing.T, response *httptest.ResponseRecorder) *dto.PresentationTransitionResponse {
	t.Helper()
	transition := &dto.PresentationTransitionResponse{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), transition), response.Body.String())
	require.NotNil(t, transition.Profile)
	require.NotNil(t, transition.Revision)
	return transition
}

func apiPresentationCapability(t *testing.T) presentation.CapabilityDefinition {
	t.Helper()
	capability := presentation.CapabilityDefinition{
		PageKey: "orders.list", DefinitionVersion: presentation.DefinitionVersion,
		Components: []presentation.CapabilityComponent{{ID: "text"}, {ID: "select"}},
		Fields: []presentation.CapabilityField{{
			ID: "status", Label: apiPresentationText("状态", "Status"), ValueType: "enum", Required: true,
			Sortable: true, Filterable: true,
			Surfaces: []presentation.Surface{
				presentation.SurfaceList, presentation.SurfaceSearch, presentation.SurfaceForm, presentation.SurfaceDetail,
			},
			Components: []string{"text", "select"},
		}},
		DataSources: []presentation.CapabilityDataSource{{
			ID: "orders.list", RequiredPermissions: []string{"/orders", "/orders/permissions/list"},
		}},
		Actions: []presentation.CapabilityAction{{
			ID: "orders.read", RequiredPermissions: []string{"/orders/permissions/read"},
			Placements: []presentation.ActionPlacement{presentation.PlacementRow},
		}},
		DefaultPresentation: presentation.CompletePresentation{
			Title: apiPresentationText("订单", "Orders"), DataSource: "orders.list",
			List: presentation.CompleteListPresentation{
				Columns: []presentation.CompleteField{{Field: "status", Component: "text", Order: 10}},
				Density: "middle", PageSize: 20,
				DefaultSort: []presentation.Sort{{Field: "status", Direction: "asc"}},
			},
			Search: presentation.CompleteSearchPresentation{
				Fields: []presentation.CompleteField{{Field: "status", Component: "select", Order: 10}},
			},
			Form: presentation.CompleteFormPresentation{
				Fields: []presentation.CompleteField{{Field: "status", Component: "select", Order: 10}}, Columns: 1,
			},
			Detail: presentation.CompleteDetailPresentation{
				Fields: []presentation.CompleteField{{Field: "status", Component: "text", Order: 10}}, Columns: 1,
			},
			Actions: []presentation.CompleteAction{{
				Action: "orders.read", Placement: presentation.PlacementRow, Order: 10,
			}},
		},
	}
	capability.DefinitionHash, _ = presentation.ComputeDefinitionHash(&capability)
	require.Empty(t, presentation.ValidateCapability(&capability))
	return capability
}

func apiPresentationDocument(
	t *testing.T,
	capability presentation.CapabilityDefinition,
	scope presentation.ScopeKind,
	subjectID string,
	pageSize int,
) json.RawMessage {
	t.Helper()
	presentationScope := presentation.Scope{Kind: scope}
	if scope != presentation.ScopeApplication {
		subject := subjectID
		presentationScope.Subject = &subject
	}
	profile := &presentation.Profile{
		APIVersion: presentation.APIVersion,
		Kind:       presentation.Kind,
		Metadata: presentation.Metadata{
			Name: "orders-" + string(scope), PageKey: capability.PageKey,
			DefinitionHash: capability.DefinitionHash, Scope: presentationScope,
		},
		Spec: presentation.ProfileSpec{
			Title: apiPresentationTextPointer("订单视图", "Order view"),
			List:  &presentation.ListPatch{PageSize: &pageSize},
		},
	}
	raw, err := json.Marshal(profile)
	require.NoError(t, err)
	return raw
}

func apiPresentationText(zhCN, enUS string) presentation.LocalizedText {
	return presentation.LocalizedText{ZhCN: &zhCN, EnUS: &enUS}
}

func apiPresentationTextPointer(zhCN, enUS string) *presentation.LocalizedText {
	value := apiPresentationText(zhCN, enUS)
	return &value
}

func apiPresentationPageSize(t *testing.T, raw json.RawMessage) int {
	t.Helper()
	document, issues := presentation.ParseDocument(raw)
	require.Empty(t, issues)
	require.NotNil(t, document.Profile.Spec.List)
	require.NotNil(t, document.Profile.Spec.List.PageSize)
	return *document.Profile.Spec.List.PageSize
}
