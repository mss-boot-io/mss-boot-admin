package apis

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type appConfigSecretTestEnforcer struct {
	allowed bool
	err     error
	calls   [][]interface{}
}

func (e *appConfigSecretTestEnforcer) Enforce(rvals ...interface{}) (bool, error) {
	e.calls = append(e.calls, rvals)
	return e.allowed, e.err
}

func appConfigSecretHandlerResponse(
	t *testing.T,
	api *AppConfig,
	method string,
	group string,
	body string,
	principal *models.User,
) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "group", Value: group}}
	ctx.Request = httptest.NewRequest(
		method,
		"/admin/api/app-configs/"+group,
		bytes.NewBufferString(body),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")
	if principal != nil {
		ctx.Set(config.Cfg.Auth.IdentityKey, principal)
	}
	if method == http.MethodGet {
		api.Group(ctx)
	} else {
		api.Control(ctx)
	}
	return recorder
}

func appConfigSecretPrincipal(roleID string, root bool) *models.User {
	return &models.User{UserLogin: models.UserLogin{
		RoleID: roleID,
		Role:   &models.Role{Root: root},
	}}
}

func decodeAppConfigGroup(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	result := make(map[string]any)
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result), recorder.Body.String())
	return result
}

func configureAppConfigSecretIdentityKey(t *testing.T) {
	t.Helper()
	previous := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = "app-config-secret-test-identity"
	t.Cleanup(func() { config.Cfg.Auth.IdentityKey = previous })
}

func seedAppConfigSecretReadRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create([]*models.AppConfig{
		{Group: "security", Name: "githubClientId", Value: "client-id", Auth: true},
		{Group: "security", Name: "githubClientSecret", Value: "client-secret", Auth: false},
	}).Error)
}

func TestAppConfigSecretReadProjectionUsesExplicitComponentPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("non-root without read omits the secret", func(t *testing.T) {
		db := setupThemeConfigAPITest(t)
		configureAppConfigSecretIdentityKey(t)
		seedAppConfigSecretReadRows(t, db)
		enforcer := &appConfigSecretTestEnforcer{}
		response := appConfigSecretHandlerResponse(
			t,
			&AppConfig{enforcer: enforcer},
			http.MethodGet,
			"security",
			"",
			appConfigSecretPrincipal("viewer", false),
		)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		group := decodeAppConfigGroup(t, response)
		require.Equal(t, "client-id", group["githubClientId"])
		require.NotContains(t, group, "githubClientSecret")
		require.Equal(t, [][]interface{}{{
			"viewer", "COMPONENT", appConfigSecretReadPath, http.MethodGet,
		}}, enforcer.calls)
	})

	t.Run("non-root with read receives the secret", func(t *testing.T) {
		db := setupThemeConfigAPITest(t)
		configureAppConfigSecretIdentityKey(t)
		seedAppConfigSecretReadRows(t, db)
		enforcer := &appConfigSecretTestEnforcer{allowed: true}
		response := appConfigSecretHandlerResponse(
			t,
			&AppConfig{enforcer: enforcer},
			http.MethodGet,
			"security",
			"",
			appConfigSecretPrincipal("secret-reader", false),
		)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.Equal(t, "client-secret", decodeAppConfigGroup(t, response)["githubClientSecret"])
	})

	t.Run("root bypasses field policy", func(t *testing.T) {
		db := setupThemeConfigAPITest(t)
		configureAppConfigSecretIdentityKey(t)
		seedAppConfigSecretReadRows(t, db)
		enforcer := &appConfigSecretTestEnforcer{err: errors.New("must not be called")}
		response := appConfigSecretHandlerResponse(
			t,
			&AppConfig{enforcer: enforcer},
			http.MethodGet,
			"security",
			"",
			appConfigSecretPrincipal("root", true),
		)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.Equal(t, "client-secret", decodeAppConfigGroup(t, response)["githubClientSecret"])
		require.Empty(t, enforcer.calls)
	})

	t.Run("policy error returns no configuration", func(t *testing.T) {
		db := setupThemeConfigAPITest(t)
		configureAppConfigSecretIdentityKey(t)
		seedAppConfigSecretReadRows(t, db)
		enforcer := &appConfigSecretTestEnforcer{err: errors.New("policy backend unavailable")}
		response := appConfigSecretHandlerResponse(
			t,
			&AppConfig{enforcer: enforcer},
			http.MethodGet,
			"security",
			"",
			appConfigSecretPrincipal("viewer", false),
		)
		require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
		require.NotContains(t, response.Body.String(), "client-secret")
	})
}

func TestAppConfigSecretWriteRequiresExplicitComponentPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("denied mixed mutation is atomic", func(t *testing.T) {
		db := setupThemeConfigAPITest(t)
		configureAppConfigSecretIdentityKey(t)
		enforcer := &appConfigSecretTestEnforcer{}
		response := appConfigSecretHandlerResponse(
			t,
			&AppConfig{enforcer: enforcer},
			http.MethodPut,
			"security",
			`{"data":{"githubClientSecret":"secret","githubClientId":"client-id"}}`,
			appConfigSecretPrincipal("config-editor", false),
		)
		require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
		var count int64
		require.NoError(t, db.Model(&models.AppConfig{}).Count(&count).Error)
		require.Zero(t, count)
		require.Len(t, enforcer.calls, 1)
		require.Equal(t, appConfigSecretWritePath, enforcer.calls[0][2])
	})

	t.Run("write grant permits the whole mutation", func(t *testing.T) {
		db := setupThemeConfigAPITest(t)
		configureAppConfigSecretIdentityKey(t)
		enforcer := &appConfigSecretTestEnforcer{allowed: true}
		response := appConfigSecretHandlerResponse(
			t,
			&AppConfig{enforcer: enforcer},
			http.MethodPut,
			"security",
			`{"data":{"githubClientSecret":"secret","githubClientId":"client-id"}}`,
			appConfigSecretPrincipal("secret-editor", false),
		)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var rows []models.AppConfig
		require.NoError(t, db.Order("name").Find(&rows).Error)
		require.Len(t, rows, 2)
	})

	t.Run("ordinary setting remains under the route permission", func(t *testing.T) {
		db := setupThemeConfigAPITest(t)
		configureAppConfigSecretIdentityKey(t)
		enforcer := &appConfigSecretTestEnforcer{}
		response := appConfigSecretHandlerResponse(
			t,
			&AppConfig{enforcer: enforcer},
			http.MethodPut,
			"security",
			`{"data":{"githubClientId":"client-id"}}`,
			appConfigSecretPrincipal("config-editor", false),
		)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.Empty(t, enforcer.calls)
		var count int64
		require.NoError(t, db.Model(&models.AppConfig{}).Count(&count).Error)
		require.EqualValues(t, 1, count)
	})

	t.Run("missing identity and policy errors fail closed", func(t *testing.T) {
		t.Run("missing identity", func(t *testing.T) {
			db := setupThemeConfigAPITest(t)
			configureAppConfigSecretIdentityKey(t)
			response := appConfigSecretHandlerResponse(
				t,
				&AppConfig{enforcer: &appConfigSecretTestEnforcer{allowed: true}},
				http.MethodPut,
				"email",
				`{"data":{"password":"smtp-secret"}}`,
				nil,
			)
			require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
			var count int64
			require.NoError(t, db.Model(&models.AppConfig{}).Count(&count).Error)
			require.Zero(t, count)
		})

		t.Run("policy error", func(t *testing.T) {
			db := setupThemeConfigAPITest(t)
			configureAppConfigSecretIdentityKey(t)
			response := appConfigSecretHandlerResponse(
				t,
				&AppConfig{enforcer: &appConfigSecretTestEnforcer{err: errors.New("policy backend unavailable")}},
				http.MethodPut,
				"security",
				`{"data":{"githubClientSecret":"provider-secret"}}`,
				appConfigSecretPrincipal("config-editor", false),
			)
			require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
			var count int64
			require.NoError(t, db.Model(&models.AppConfig{}).Count(&count).Error)
			require.Zero(t, count)
		})
	})
}

func TestStorageAppConfigSurfaceIsAdmissionPolicyOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("historical provider and credential rows are never projected", func(t *testing.T) {
		db := setupThemeConfigAPITest(t)
		configureAppConfigSecretIdentityKey(t)
		require.NoError(t, db.Create([]*models.AppConfig{
			{Group: "storage", Name: "maxSize", Value: "10485760", Auth: true},
			{Group: "storage", Name: "allowedTypes", Value: "image/png,image/*", Auth: true},
			{Group: "storage", Name: "type", Value: "s3", Auth: true},
			{Group: "storage", Name: "s3Endpoint", Value: "https://legacy.invalid", Auth: true},
			{Group: "storage", Name: "s3AccessKeyID", Value: "legacy-access", Auth: true},
			{Group: "storage", Name: "s3SecretAccessKey", Value: "legacy-secret", Auth: false},
		}).Error)
		enforcer := &appConfigSecretTestEnforcer{allowed: true}
		response := appConfigSecretHandlerResponse(
			t,
			&AppConfig{enforcer: enforcer},
			http.MethodGet,
			"storage",
			"",
			appConfigSecretPrincipal("config-editor", false),
		)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.Equal(t, map[string]any{
			"allowedTypes": "image/png,image/*",
			"maxSize":      "10485760",
		}, decodeAppConfigGroup(t, response))
		require.Empty(t, enforcer.calls)
	})

	t.Run("provider and credential writes return a stable 422 without partial policy writes", func(t *testing.T) {
		db := setupThemeConfigAPITest(t)
		configureAppConfigSecretIdentityKey(t)
		enforcer := &appConfigSecretTestEnforcer{allowed: true}
		response := appConfigSecretHandlerResponse(
			t,
			&AppConfig{enforcer: enforcer},
			http.MethodPut,
			"storage",
			`{"data":{"maxSize":"10485760","s3SecretAccessKey":"removed"}}`,
			appConfigSecretPrincipal("config-editor", false),
		)
		require.Equal(t, http.StatusUnprocessableEntity, response.Code, response.Body.String())
		body := decodeAppConfigGroup(t, response)
		require.Equal(t, "STORAGE_PROFILE_APP_CONFIG_FORBIDDEN", body["errorCode"])
		require.Equal(
			t,
			"storage provider and credential settings must come from the startup profile",
			body["errorMessage"],
		)
		require.Empty(t, enforcer.calls)
		var count int64
		require.NoError(t, db.Model(&models.AppConfig{}).Count(&count).Error)
		require.Zero(t, count)
	})
}
