package apis

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type interactiveTokenState struct {
	Count   int64
	Token   string
	Revoked bool
}

func TestClearAuthCookieIsPublicAndReturnsNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/api/user/auth-cookie/clear", (&User{}).ClearAuthCookie)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/api/user/auth-cookie/clear", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %q, want 204", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", recorder.Body.String())
	}
}

func TestUpdateUserInfoRejectsPersonalAccessTokensWithoutChangingRecoveryFields(t *testing.T) {
	db := prepareInteractiveSecurityTestDB(t)
	beforeEmail, beforePhone := loadRecoveryFields(t, db)

	for _, test := range personalAccessTokenPrincipals() {
		t.Run(test.name, func(t *testing.T) {
			recorder := executeHandlerWithIdentity(
				t,
				test.principal,
				http.MethodPut,
				"/admin/api/user/userInfo",
				"/admin/api/user/userInfo",
				(&User{}).UpdateUserInfo,
				`{"email":"attacker@example.invalid","phone":"000000"}`,
			)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, body = %s, want 403", recorder.Code, recorder.Body.String())
			}
			afterEmail, afterPhone := loadRecoveryFields(t, db)
			if afterEmail != beforeEmail || afterPhone != beforePhone {
				t.Fatalf(
					"recovery fields changed for rejected token: before=(%q,%q) after=(%q,%q)",
					beforeEmail,
					beforePhone,
					afterEmail,
					afterPhone,
				)
			}
		})
	}
}

func TestUpdateUserInfoRejectsEmailChangeUntilCanonicalIdentityMigration(t *testing.T) {
	db := prepareInteractiveSecurityTestDB(t)
	beforeEmail, beforePhone := loadRecoveryFields(t, db)
	principal := &models.User{}
	principal.ID = "user-1"
	recorder := executeHandlerWithIdentity(
		t,
		principal,
		http.MethodPut,
		"/admin/api/user/userInfo",
		"/admin/api/user/userInfo",
		(&User{}).UpdateUserInfo,
		`{"email":"victim@example.test","name":"must-not-apply"}`,
	)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s, want 422", recorder.Code, recorder.Body.String())
	}
	afterEmail, afterPhone := loadRecoveryFields(t, db)
	if afterEmail != beforeEmail || afterPhone != beforePhone {
		t.Fatalf("recovery fields changed: before=(%q,%q) after=(%q,%q)", beforeEmail, beforePhone, afterEmail, afterPhone)
	}
}

func TestUserAuthTokenManagementRejectsPersonalAccessTokensWithoutDatabaseSideEffects(t *testing.T) {
	db := prepareInteractiveSecurityTestDB(t)
	routes := []struct {
		name    string
		method  string
		route   string
		path    string
		handler gin.HandlerFunc
		body    string
	}{
		{
			name:    "list",
			method:  http.MethodGet,
			route:   "/admin/api/user-auth-tokens",
			path:    "/admin/api/user-auth-tokens",
			handler: (&UserAuthToken{}).List,
		},
		{
			name:    "create",
			method:  http.MethodPost,
			route:   "/admin/api/user-auth-tokens",
			path:    "/admin/api/user-auth-tokens?validityPeriod=0",
			handler: (&UserAuthToken{}).Generate,
		},
		{
			name:    "refresh",
			method:  http.MethodPut,
			route:   "/admin/api/user-auth-token/:id/refresh",
			path:    "/admin/api/user-auth-token/token-1/refresh",
			handler: (&UserAuthToken{}).Refresh,
		},
		{
			name:    "revoke",
			method:  http.MethodPut,
			route:   "/admin/api/user-auth-token/:id/revoke",
			path:    "/admin/api/user-auth-token/token-1/revoke",
			handler: (&UserAuthToken{}).Revoked,
		},
	}

	for _, principal := range personalAccessTokenPrincipals() {
		t.Run(principal.name, func(t *testing.T) {
			for _, route := range routes {
				t.Run(route.name, func(t *testing.T) {
					before := loadInteractiveTokenState(t, db)
					recorder := executeHandlerWithIdentity(
						t,
						principal.principal,
						route.method,
						route.route,
						route.path,
						route.handler,
						route.body,
					)
					if recorder.Code != http.StatusForbidden {
						t.Fatalf("status = %d, body = %s, want 403", recorder.Code, recorder.Body.String())
					}
					if after := loadInteractiveTokenState(t, db); after != before {
						t.Fatalf("token state changed for rejected credential: before=%+v after=%+v", before, after)
					}
				})
			}
		})
	}
}

func personalAccessTokenPrincipals() []struct {
	name      string
	principal *models.User
} {
	byValue := &models.User{UserLogin: models.UserLogin{PersonAccessToken: "pat-id"}}
	byValue.ID = "user-1"
	byMarker := &models.User{UserLogin: models.UserLogin{RefreshTokenDisable: true}}
	byMarker.ID = "user-1"
	return []struct {
		name      string
		principal *models.User
	}{
		{name: "personal access token value", principal: byValue},
		{name: "non-refreshable token marker", principal: byMarker},
	}
}

func prepareInteractiveSecurityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open interactive security database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get interactive security database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	statements := []string{
		`CREATE TABLE mss_boot_users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			phone TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE mss_boot_user_auth_token (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token TEXT NOT NULL,
			expired_at DATETIME,
			revoked BOOLEAN NOT NULL,
			tenant_id TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
	}
	for _, statement := range statements {
		if err = db.Exec(statement).Error; err != nil {
			t.Fatalf("create interactive security table: %v", err)
		}
	}
	if err = db.Exec(
		"INSERT INTO mss_boot_users (id, email, phone) VALUES (?, ?, ?)",
		"user-1", "owner@example.test", "13800000000",
	).Error; err != nil {
		t.Fatalf("seed interactive security user: %v", err)
	}
	if err = db.Exec(
		"INSERT INTO mss_boot_user_auth_token (id, user_id, token, revoked) VALUES (?, ?, ?, ?)",
		"token-1", "user-1", "original-token", false,
	).Error; err != nil {
		t.Fatalf("seed interactive security token: %v", err)
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })
	return db
}

func executeHandlerWithIdentity(
	t *testing.T,
	verifier security.Verifier,
	method string,
	route string,
	path string,
	handler gin.HandlerFunc,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	identityKey := config.Cfg.Auth.IdentityKey
	if identityKey == "" {
		identityKey = "identity"
	}
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = identityKey
	defer func() { config.Cfg.Auth.IdentityKey = previousIdentityKey }()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Handle(method, route, func(ctx *gin.Context) {
		ctx.Set(identityKey, verifier)
		handler(ctx)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

func loadRecoveryFields(t *testing.T, db *gorm.DB) (string, string) {
	t.Helper()
	var result struct {
		Email string
		Phone string
	}
	if err := db.Table("mss_boot_users").Select("email", "phone").Where("id = ?", "user-1").Take(&result).Error; err != nil {
		t.Fatalf("load recovery fields: %v", err)
	}
	return result.Email, result.Phone
}

func loadInteractiveTokenState(t *testing.T, db *gorm.DB) interactiveTokenState {
	t.Helper()
	var state interactiveTokenState
	if err := db.Table("mss_boot_user_auth_token").Count(&state.Count).Error; err != nil {
		t.Fatalf("count interactive tokens: %v", err)
	}
	if err := db.Table("mss_boot_user_auth_token").Select("token", "revoked").Where("id = ?", "token-1").Take(&state).Error; err != nil {
		t.Fatalf("load interactive token state: %v", err)
	}
	return state
}
