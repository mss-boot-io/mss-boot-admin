package apis

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type passwordState struct {
	PasswordHash          string
	Salt                  string
	LocalPasswordDisabled bool
}

func TestResetPasswordRejectsPersonalAccessTokensWithoutChangingPassword(t *testing.T) {
	db := preparePasswordResetTestDB(t)
	before := loadPasswordState(t, db, "user-1")

	tests := []struct {
		name      string
		principal *models.User
	}{
		{
			name: "personal access token value",
			principal: &models.User{UserLogin: models.UserLogin{
				PersonAccessToken: "pat-id",
			}},
		},
		{
			name: "non-refreshable token marker",
			principal: &models.User{UserLogin: models.UserLogin{
				RefreshTokenDisable: true,
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.principal.ID = "user-1"
			recorder := executePasswordReset(t, test.principal, `{"password":"replacement-password1"}`)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, body = %s, want 403", recorder.Code, recorder.Body.String())
			}
			if after := loadPasswordState(t, db, "user-1"); after != before {
				t.Fatalf("password state changed for rejected token: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestResetPasswordRequiresRecentDurableSessionAndKeepsAnonymousRecoveryBehavior(t *testing.T) {
	db := preparePasswordResetTestDB(t)
	before := loadPasswordState(t, db, "user-1")

	principal := &models.User{}
	principal.ID = "user-1"
	sessionResponse := executePasswordReset(t, principal, `{"password":"replacement-password1"}`)
	if sessionResponse.Code != http.StatusCreated {
		t.Fatalf("ordinary session status = %d, body = %s, want 201", sessionResponse.Code, sessionResponse.Body.String())
	}
	if sessionResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("password change Cache-Control = %q, want no-store", sessionResponse.Header().Get("Cache-Control"))
	}
	afterSession := loadPasswordState(t, db, "user-1")
	if afterSession == before {
		t.Fatal("ordinary session did not update the password")
	}
	wantHash, err := security.SetPassword("replacement-password1", afterSession.Salt)
	if err != nil {
		t.Fatalf("derive expected password: %v", err)
	}
	if afterSession.PasswordHash != wantHash {
		t.Fatalf("ordinary session password hash = %q, want %q", afterSession.PasswordHash, wantHash)
	}
	if afterSession.LocalPasswordDisabled {
		t.Fatal("password reset did not re-enable the local credential")
	}

	// Anonymous recovery still reaches the existing request-body validation and
	// email/captcha checks rather than being classified as a PAT request.
	anonymousMalformed := executePasswordReset(t, nil, `{}`)
	if anonymousMalformed.Code != http.StatusUnprocessableEntity {
		t.Fatalf("anonymous malformed recovery status = %d, body = %s, want existing 422", anonymousMalformed.Code, anonymousMalformed.Body.String())
	}
	anonymousResponse := executePasswordReset(t, nil, `{"password":"another-password1"}`)
	if anonymousResponse.Code != http.StatusForbidden {
		t.Fatalf("anonymous recovery status = %d, body = %s, want existing 403", anonymousResponse.Code, anonymousResponse.Body.String())
	}
	if afterAnonymous := loadPasswordState(t, db, "user-1"); afterAnonymous != afterSession {
		t.Fatalf("incomplete anonymous recovery changed password: before=%+v after=%+v", afterSession, afterAnonymous)
	}
}

func TestOAuthBindingMutationsRejectPersonalAccessTokensWithoutDatabaseSideEffects(t *testing.T) {
	db := prepareOAuthBindingTestDB(t)
	before := countOAuthBindings(t, db)

	principals := []struct {
		name      string
		principal *models.User
	}{
		{
			name: "personal access token value",
			principal: &models.User{UserLogin: models.UserLogin{
				PersonAccessToken: "pat-id",
			}},
		},
		{
			name: "non-refreshable token marker",
			principal: &models.User{UserLogin: models.UserLogin{
				RefreshTokenDisable: true,
			}},
		},
	}
	routes := []struct {
		name    string
		method  string
		path    string
		handler gin.HandlerFunc
	}{
		{name: "binding", method: http.MethodPost, path: "/admin/api/user/binding", handler: (&User{}).Binding},
		{name: "unbinding", method: http.MethodDelete, path: "/admin/api/user/unbinding", handler: (&User{}).Unbinding},
	}

	for _, principal := range principals {
		t.Run(principal.name, func(t *testing.T) {
			principal.principal.ID = "user-1"
			for _, route := range routes {
				t.Run(route.name, func(t *testing.T) {
					recorder := executeUserOAuthMutation(
						t,
						principal.principal,
						route.method,
						route.path,
						route.handler,
					)
					if recorder.Code != http.StatusForbidden {
						t.Fatalf("status = %d, body = %s, want 403", recorder.Code, recorder.Body.String())
					}
					if after := countOAuthBindings(t, db); after != before {
						t.Fatalf("OAuth binding rows changed for rejected token: before=%d after=%d", before, after)
					}
				})
			}
		})
	}
}

func preparePasswordResetTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open password reset database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get password reset database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.Exec(`CREATE TABLE mss_boot_users (
		id TEXT PRIMARY KEY,
		password_hash TEXT NOT NULL,
		salt TEXT NOT NULL,
		local_password_disabled BOOLEAN NOT NULL DEFAULT TRUE,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create users table: %v", err)
	}
	if err = db.Exec(
		"INSERT INTO mss_boot_users (id, password_hash, salt) VALUES (?, ?, ?)",
		"user-1", "original-password-hash", "original-salt",
	).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err = db.AutoMigrate(&models.UserSession{}); err != nil {
		t.Fatalf("create user sessions table: %v", err)
	}
	now := time.Now()
	session := &models.UserSession{
		UserID:            "user-1",
		LoginAt:           now,
		LastSeenAt:        now,
		ReauthenticatedAt: &now,
		ExpiredAt:         now.Add(time.Hour),
	}
	session.ID = "session-1"
	if err = db.Create(session).Error; err != nil {
		t.Fatalf("seed user session: %v", err)
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })
	return db
}

func prepareOAuthBindingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open OAuth binding database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get OAuth binding database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.Exec(`CREATE TABLE mss_boot_user_oauth2 (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		provider TEXT NOT NULL,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create OAuth binding table: %v", err)
	}
	if err = db.Exec(
		"INSERT INTO mss_boot_user_oauth2 (id, user_id, provider) VALUES (?, ?, ?)",
		"binding-1", "user-1", "github",
	).Error; err != nil {
		t.Fatalf("seed OAuth binding: %v", err)
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })
	return db
}

func executePasswordReset(t *testing.T, verifier security.Verifier, body string) *httptest.ResponseRecorder {
	t.Helper()
	previousVerifyHandler := response.VerifyHandler
	response.VerifyHandler = func(*gin.Context) security.Verifier { return verifier }
	t.Cleanup(func() { response.VerifyHandler = previousVerifyHandler })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if verifier != nil {
			c.Set("JWT_PAYLOAD", jwt.MapClaims{"sid": "session-1"})
		}
		c.Next()
	})
	router.POST("/admin/api/user/reset-password", (&User{}).ResetPassword)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/api/user/reset-password", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

func executeUserOAuthMutation(
	t *testing.T,
	verifier security.Verifier,
	method string,
	path string,
	handler gin.HandlerFunc,
) *httptest.ResponseRecorder {
	t.Helper()
	previousVerifyHandler := response.VerifyHandler
	response.VerifyHandler = func(*gin.Context) security.Verifier { return verifier }
	t.Cleanup(func() { response.VerifyHandler = previousVerifyHandler })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Handle(method, path, handler)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(`{"provider":"github"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

func loadPasswordState(t *testing.T, db *gorm.DB, userID string) passwordState {
	t.Helper()
	var state passwordState
	if err := db.Table("mss_boot_users").
		Select("password_hash", "salt", "local_password_disabled").
		Where("id = ?", userID).
		Take(&state).Error; err != nil {
		t.Fatalf("load password state: %v", err)
	}
	return state
}

func countOAuthBindings(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Table("mss_boot_user_oauth2").Count(&count).Error; err != nil {
		t.Fatalf("count OAuth bindings: %v", err)
	}
	return count
}
