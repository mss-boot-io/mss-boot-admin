package apis

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	responsegorm "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions/gorm"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type operationalAPIFixture struct {
	db          *gorm.DB
	router      *gin.Engine
	principal   *models.User
	identityKey string
}

func setupOperationalAPITest(t *testing.T) *operationalAPIFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())+"?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Notice{},
		&models.Task{},
		&models.SystemConfig{},
		&models.AuditLog{},
		&models.LoginLog{},
	))

	previousDB := gormdb.DB
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	previousAuthHandler := response.AuthHandler
	previousVerifyHandler := response.VerifyHandler
	previousCleaner := responsegorm.CleanCacheFromTag
	identityKey := "operational-api-test-identity"
	config.Cfg.Auth.IdentityKey = identityKey
	gormdb.DB = db
	response.AuthHandler = func(c *gin.Context) { c.Next() }
	responsegorm.CleanCacheFromTag = nil
	principal := &models.User{UserLogin: models.UserLogin{
		RoleID: "delegated-role",
		Role:   &models.Role{Root: false},
	}}
	principal.ID = "owner"
	response.VerifyHandler = func(*gin.Context) security.Verifier { return principal }
	t.Cleanup(func() {
		gormdb.DB = previousDB
		config.Cfg.Auth.IdentityKey = previousIdentityKey
		response.AuthHandler = previousAuthHandler
		response.VerifyHandler = previousVerifyHandler
		responsegorm.CleanCacheFromTag = previousCleaner
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(identityKey, principal)
		c.Next()
	})
	return &operationalAPIFixture{
		db:          db,
		router:      router,
		principal:   principal,
		identityKey: identityKey,
	}
}

func TestNoticeGenericRoutesCannotCrossOwnerBoundary(t *testing.T) {
	fixture := setupOperationalAPITest(t)
	ownerNotice := &models.Notice{UserID: "owner", Title: "owner-only", Type: models.NoticeTypeMessage}
	otherNotice := &models.Notice{UserID: "other", Title: "other-secret", Type: models.NoticeTypeMessage}
	require.NoError(t, fixture.db.Create(ownerNotice).Error)
	require.NoError(t, fixture.db.Create(otherNotice).Error)

	controller := newNoticeController()
	fixture.router.GET("/notices", controller.GetAction(response.Search).Handler()...)
	fixture.router.GET("/notices/:id", controller.GetAction(response.Get).Handler()...)
	fixture.router.PUT("/notices/:id", controller.GetAction(response.Control).Handler()...)
	fixture.router.DELETE("/notices/:id", controller.GetAction(response.Delete).Handler()...)

	list := operationalAPIRequest(fixture.router, http.MethodGet, "/notices?userID=other", "")
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.NotContains(t, list.Body.String(), "other-secret")

	get := operationalAPIRequest(fixture.router, http.MethodGet, "/notices/"+otherNotice.ID, "")
	require.Equal(t, http.StatusNotFound, get.Code, get.Body.String())
	update := operationalAPIRequest(
		fixture.router,
		http.MethodPut,
		"/notices/"+otherNotice.ID,
		`{"title":"stolen","type":"message"}`,
	)
	require.Equal(t, http.StatusNotFound, update.Code, update.Body.String())
	deleted := operationalAPIRequest(fixture.router, http.MethodDelete, "/notices/"+otherNotice.ID, "")
	require.Equal(t, http.StatusNoContent, deleted.Code, deleted.Body.String())

	var preserved models.Notice
	require.NoError(t, fixture.db.First(&preserved, "id = ?", otherNotice.ID).Error)
	require.Equal(t, "other-secret", preserved.Title)
}

func TestTaskSummaryOmitsExecutablePayloadAndDetailRequiresRoot(t *testing.T) {
	fixture := setupOperationalAPITest(t)
	definition := &models.Task{
		Name:     "sensitive task",
		Provider: models.TaskProviderDefault,
		Spec:     "0 * * * * *",
		Protocol: "https",
		Endpoint: "internal.example.test/admin",
		Method:   "POST",
		Body:     `{"password":"payload-secret"}`,
		Metadata: `{"Authorization":"Bearer metadata-secret"}`,
		Python:   "script-secret",
		Command:  "[]",
		Args:     "[]",
		Status:   enum.Disabled,
	}
	require.NoError(t, fixture.db.Create(definition).Error)

	controller := newTaskController()
	fixture.router.GET("/tasks", controller.List)
	fixture.router.GET("/tasks/:id", controller.GetAction(response.Get).Handler()...)

	list := operationalAPIRequest(fixture.router, http.MethodGet, "/tasks", "")
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Contains(t, list.Body.String(), "sensitive task")
	for _, forbidden := range []string{"payload-secret", "metadata-secret", "script-secret", "internal.example.test", `"body"`, `"metadata"`, `"python"`, `"endpoint"`} {
		require.NotContains(t, list.Body.String(), forbidden)
	}

	detail := operationalAPIRequest(fixture.router, http.MethodGet, "/tasks/"+definition.ID, "")
	require.Equal(t, http.StatusForbidden, detail.Code, detail.Body.String())
}

func TestAuditProjectionNeverReturnsRawBodiesAndRedactsDiagnostics(t *testing.T) {
	fixture := setupOperationalAPITest(t)
	entry := &models.AuditLog{
		UserID:   "owner",
		Username: "operator",
		Type:     models.AuditLogTypeUpdate,
		Action:   "update",
		Resource: "system-config",
		Method:   http.MethodPut,
		Path:     "/system-configs/private?token=path-secret",
		Status:   enum.Enabled,
		Message:  "Authorization: Bearer message-secret",
		Request:  `{"password":"request-secret"}`,
		Response: `{"token":"response-secret"}`,
	}
	require.NoError(t, fixture.db.Create(entry).Error)
	fixture.router.GET(
		"/audit-logs/operation",
		protectOperationalResponse,
		(&AuditLogAPI{}).OperationLogs,
	)

	result := operationalAPIRequest(fixture.router, http.MethodGet, "/audit-logs/operation", "")
	require.Equal(t, http.StatusOK, result.Code, result.Body.String())
	require.Equal(t, "no-store", result.Header().Get("Cache-Control"))
	for _, forbidden := range []string{"request-secret", "response-secret", "message-secret", "path-secret", `"request"`, `"response"`} {
		require.NotContains(t, result.Body.String(), forbidden)
	}
	require.Contains(t, result.Body.String(), "[REDACTED]")
}

func operationalAPIRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
