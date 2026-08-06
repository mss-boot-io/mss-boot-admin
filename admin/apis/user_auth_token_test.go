package apis

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	standardjwt "github.com/golang-jwt/jwt/v4"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserAuthTokenAPIShowsSecretsOnlyOnCreateAndRotate(t *testing.T) {
	db := prepareUserAuthTokenAPITestDB(t)
	auth := newUserAuthTokenAPITestMiddleware(t)
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = "identity"
	t.Cleanup(func() { config.Cfg.Auth.IdentityKey = previousIdentityKey })
	previousAuth := middleware.Auth
	middleware.Auth = auth
	t.Cleanup(func() { middleware.Auth = previousAuth })

	owner := &models.User{}
	owner.ID = "user-1"
	createdRecorder := executeHandlerWithIdentity(
		t,
		owner,
		http.MethodPost,
		"/admin/api/user-auth-tokens",
		"/admin/api/user-auth-tokens?validityPeriod=24h",
		(&UserAuthToken{}).Generate,
		"",
	)
	if createdRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createdRecorder.Code, createdRecorder.Body.String())
	}
	if got := createdRecorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("create Cache-Control = %q, want no-store", got)
	}
	var created dto.UserAuthTokenSecretResponse
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" || created.Token == "" || created.Fingerprint == "" {
		t.Fatalf("incomplete create response: %+v", created)
	}

	stored := &models.UserAuthToken{}
	if err := db.First(stored, "id = ?", created.ID).Error; err != nil {
		t.Fatalf("load created PAT: %v", err)
	}
	if stored.LegacyToken != "" || !models.VerifyUserAuthToken(created.Token, stored.TokenHash) {
		t.Fatal("create persisted recoverable plaintext or the wrong digest")
	}

	listRecorder := executeHandlerWithIdentity(
		t,
		owner,
		http.MethodGet,
		"/admin/api/user-auth-tokens",
		"/admin/api/user-auth-tokens",
		(&UserAuthToken{}).List,
		"",
	)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	var page struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(page.Data) != 1 {
		t.Fatalf("list size = %d, want 1", len(page.Data))
	}
	for _, forbidden := range []string{"token", "tokenHash", "legacyToken"} {
		if _, ok := page.Data[0][forbidden]; ok {
			t.Fatalf("list exposed forbidden field %q: %s", forbidden, listRecorder.Body.String())
		}
	}
	if page.Data[0]["fingerprint"] != created.Fingerprint {
		t.Fatalf("list fingerprint = %#v, want %q", page.Data[0]["fingerprint"], created.Fingerprint)
	}

	rotatedRecorder := executeHandlerWithIdentity(
		t,
		owner,
		http.MethodPut,
		"/admin/api/user-auth-token/:id/refresh",
		"/admin/api/user-auth-token/"+created.ID+"/refresh",
		(&UserAuthToken{}).Refresh,
		"",
	)
	if rotatedRecorder.Code != http.StatusOK {
		t.Fatalf("rotate status = %d, body = %s", rotatedRecorder.Code, rotatedRecorder.Body.String())
	}
	if got := rotatedRecorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("rotate Cache-Control = %q, want no-store", got)
	}
	var rotated dto.UserAuthTokenSecretResponse
	if err := json.Unmarshal(rotatedRecorder.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("decode rotate response: %v", err)
	}
	if rotated.ID != created.ID || rotated.Token == "" || rotated.Token == created.Token {
		t.Fatalf("rotation response does not contain one new token for the same record")
	}

	if err := db.First(stored, "id = ?", created.ID).Error; err != nil {
		t.Fatalf("reload rotated PAT: %v", err)
	}
	if models.VerifyUserAuthToken(created.Token, stored.TokenHash) {
		t.Fatal("old token remained valid after rotation")
	}
	if !models.VerifyUserAuthToken(rotated.Token, stored.TokenHash) {
		t.Fatal("rotated token does not match the replacement digest")
	}
	assertPersonalAccessTokenClaims(t, auth, rotated.Token, created.ID)

	other := &models.User{}
	other.ID = "user-2"
	hashBeforeIsolationCheck := stored.TokenHash
	otherRefreshRecorder := executeHandlerWithIdentity(
		t,
		other,
		http.MethodPut,
		"/admin/api/user-auth-token/:id/refresh",
		"/admin/api/user-auth-token/"+created.ID+"/refresh",
		(&UserAuthToken{}).Refresh,
		"",
	)
	if otherRefreshRecorder.Code != http.StatusNotFound {
		t.Fatalf("other-owner rotate status = %d, want 404", otherRefreshRecorder.Code)
	}
	otherRevokeRecorder := executeHandlerWithIdentity(
		t,
		other,
		http.MethodPut,
		"/admin/api/user-auth-token/:id/revoke",
		"/admin/api/user-auth-token/"+created.ID+"/revoke",
		(&UserAuthToken{}).Revoked,
		"",
	)
	if otherRevokeRecorder.Code != http.StatusOK {
		t.Fatalf("other-owner revoke status = %d, want idempotent 200", otherRevokeRecorder.Code)
	}
	if err := db.First(stored, "id = ?", created.ID).Error; err != nil {
		t.Fatalf("reload owner-isolated PAT: %v", err)
	}
	if stored.Revoked || stored.TokenHash != hashBeforeIsolationCheck {
		t.Fatal("other owner changed PAT state")
	}

	revokeRecorder := executeHandlerWithIdentity(
		t,
		owner,
		http.MethodPut,
		"/admin/api/user-auth-token/:id/revoke",
		"/admin/api/user-auth-token/"+created.ID+"/revoke",
		(&UserAuthToken{}).Revoked,
		"",
	)
	if revokeRecorder.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, body = %s", revokeRecorder.Code, revokeRecorder.Body.String())
	}
	if err := db.First(stored, "id = ?", created.ID).Error; err != nil {
		t.Fatalf("reload revoked PAT: %v", err)
	}
	if !stored.Revoked {
		t.Fatal("owner revoke did not persist")
	}
}

func prepareUserAuthTokenAPITestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.AutoMigrate(&models.UserAuthToken{}); err != nil {
		t.Fatalf("migrate PAT API test schema: %v", err)
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })
	return db
}

func newUserAuthTokenAPITestMiddleware(t *testing.T) *jwt.GinJWTMiddleware {
	t.Helper()
	auth := &jwt.GinJWTMiddleware{
		Key:     []byte("user-auth-token-api-test-signing-key"),
		Timeout: time.Hour,
		PayloadFunc: func(data any) jwt.MapClaims {
			verifier := data.(security.Verifier)
			return jwt.MapClaims{
				"refreshTokenDisabled": verifier.GetRefreshTokenDisable(),
				"personAccessToken":    verifier.GetPersonAccessToken(),
			}
		},
	}
	if err := auth.MiddlewareInit(); err != nil {
		t.Fatalf("initialize JWT middleware: %v", err)
	}
	return auth
}

func assertPersonalAccessTokenClaims(t *testing.T, auth *jwt.GinJWTMiddleware, rawToken, tokenID string) {
	t.Helper()
	parsed, err := auth.ParseTokenString(rawToken)
	if err != nil {
		t.Fatalf("parse PAT: %v", err)
	}
	claims, ok := parsed.Claims.(standardjwt.MapClaims)
	if !ok {
		t.Fatalf("claims type = %T", parsed.Claims)
	}
	if claims["refreshTokenDisabled"] != true || claims["personAccessToken"] != tokenID {
		t.Fatalf("rotated token is not a PAT: %#v", claims)
	}
}
