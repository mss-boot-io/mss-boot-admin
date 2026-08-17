package middleware

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ginjwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	signedjwt "github.com/golang-jwt/jwt/v4"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type challengeAuthAppConfig map[string]string

func (c challengeAuthAppConfig) SetAppConfig(*gin.Context, string, bool, string) error { return nil }

func (c challengeAuthAppConfig) GetAppConfig(_ *gin.Context, key string) (string, bool) {
	value, ok := c[key]
	return value, ok
}

func TestAdminJWTVerifierDoesNotContainProviderCredential(t *testing.T) {
	const providerToken = "provider-access-token-must-not-enter-admin-jwt"
	user := &models.User{UserLogin: models.UserLogin{
		Username: "oauth-user",
		RoleID:   "root-role",
		Role:     &models.Role{Name: "root", Root: true},
		Password: providerToken,
		Captcha:  "captcha-secret",
		Provider: pkg.GithubLoginProvider,
	}}
	user.ID = "user-1"
	claims := principalClaims(user)
	token, err := signedjwt.NewWithClaims(signedjwt.SigningMethodHS256, signedjwt.MapClaims{
		"uid": claims["uid"],
		"rid": claims["rid"],
	}).SignedString([]byte("test-only-signing-key"))
	if err != nil {
		t.Fatalf("sign Admin JWT: %v", err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT segments = %d, want 3", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode JWT payload: %v", err)
	}
	if strings.Contains(string(payload), providerToken) ||
		strings.Contains(string(payload), "captcha-secret") ||
		strings.Contains(string(payload), `\"password\"`) ||
		strings.Contains(string(payload), `\"captcha\"`) ||
		strings.Contains(string(payload), `\"role\"`) ||
		strings.Contains(string(payload), `\"root\"`) ||
		strings.Contains(string(payload), `\"username\"`) {
		t.Fatalf("JWT payload leaked transient login secrets: %s", payload)
	}

	var decodedClaims map[string]any
	if err = json.Unmarshal(payload, &decodedClaims); err != nil {
		t.Fatalf("decode JWT claims: %v", err)
	}
	if decodedClaims["uid"] != "user-1" || decodedClaims["rid"] != "root-role" {
		t.Fatalf("JWT identity claims = %#v, want uid/rid only", decodedClaims)
	}
	if _, exists := decodedClaims["verifier"]; exists {
		t.Fatalf("JWT retained the legacy verifier claim: %#v", decodedClaims)
	}
	if user.Password != "" || user.Captcha != "" {
		t.Fatal("JWT marshalling did not clear transient secrets from the in-memory principal")
	}
}

func TestPublicLoginRejectsRawOAuthProviderVerifiers(t *testing.T) {
	tests := []struct {
		provider pkg.LoginProvider
		want     bool
	}{
		{provider: pkg.GithubLoginProvider, want: true},
		{provider: pkg.LarkLoginProvider, want: true},
		{provider: pkg.EmailLoginProvider, want: false},
		{provider: pkg.LoginProvider(""), want: false},
	}
	for _, test := range tests {
		user := &models.User{UserLogin: models.UserLogin{Provider: test.provider}}
		if got := isDisallowedPublicOAuthVerifier(user); got != test.want {
			t.Fatalf("provider %q disallowed = %v, want %v", test.provider, got, test.want)
		}
	}
}

func TestPublicLoginEndpointRejectsRawProviderTokens(t *testing.T) {
	configureBrowserSessionTest(t)
	previousAuth := Auth
	previousVerifier := Verifier
	t.Cleanup(func() {
		Auth = previousAuth
		Verifier = previousVerifier
	})
	Verifier = &models.User{}
	Auth = &ginjwt.GinJWTMiddleware{
		Realm:         "test",
		Key:           []byte("test-only-signing-key-with-sufficient-length"),
		Timeout:       time.Hour,
		MaxRefresh:    time.Hour,
		IdentityKey:   "identity",
		Authenticator: authenticateLoginRequest,
		PayloadFunc: func(any) ginjwt.MapClaims {
			return ginjwt.MapClaims{}
		},
		TokenLookup:   "header: Authorization",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,
	}
	if err := Auth.MiddlewareInit(); err != nil {
		t.Fatalf("MiddlewareInit() error = %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/user/session/login", BrowserSessionLoginHandler)
	for _, provider := range []pkg.LoginProvider{pkg.GithubLoginProvider, pkg.LarkLoginProvider} {
		body, err := json.Marshal(map[string]string{
			"type":     string(provider),
			"password": "raw-provider-token",
		})
		if err != nil {
			t.Fatalf("marshal login body: %v", err)
		}
		request := httptest.NewRequest(http.MethodPost, "/user/session/login", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://admin.example")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("provider %q status = %d, want 401; body=%s", provider, recorder.Code, recorder.Body.String())
		}
		if strings.Contains(recorder.Body.String(), "raw-provider-token") {
			t.Fatalf("provider %q response leaked raw token: %s", provider, recorder.Body.String())
		}
	}
}

func TestEmailChallengeLoginProviderOutageReturnsServiceUnavailable(t *testing.T) {
	configureBrowserSessionTest(t)
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open login audit database: %v", err)
	}
	if err = database.AutoMigrate(&models.LoginLog{}); err != nil {
		t.Fatalf("migrate login audit database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("open login audit SQL handle: %v", err)
	}
	previousDB := gormdb.DB
	previousAuth := Auth
	previousVerifier := Verifier
	previousChallenge := center.GetRuntimeChallenge()
	previousAppConfig := center.GetAppConfig()
	gormdb.DB = database
	Verifier = &models.User{}
	center.SetRuntimeChallenge(nil)
	center.SetAppConfig(challengeAuthAppConfig{"security:emailEnabled": "true"})
	t.Cleanup(func() {
		_ = sqlDB.Close()
		gormdb.DB = previousDB
		Auth = previousAuth
		Verifier = previousVerifier
		center.SetRuntimeChallenge(previousChallenge)
		center.SetAppConfig(previousAppConfig)
	})

	Auth = &ginjwt.GinJWTMiddleware{
		Realm:         "test",
		Key:           []byte("test-only-signing-key-with-sufficient-length"),
		Timeout:       time.Hour,
		MaxRefresh:    time.Hour,
		IdentityKey:   "identity",
		Authenticator: authenticateLoginRequest,
		Unauthorized:  writeUnauthorizedAuthResponse,
		TokenLookup:   "header: Authorization",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,
	}
	if err = Auth.MiddlewareInit(); err != nil {
		t.Fatalf("MiddlewareInit() error = %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	if err = router.SetTrustedProxies(nil); err != nil {
		t.Fatalf("disable trusted proxies: %v", err)
	}
	router.POST("/user/session/login", BrowserSessionLoginHandler)
	body := `{"type":"email","email":"person@example.com","captcha":"123456"}`
	request := httptest.NewRequest(http.MethodPost, "/user/session/login", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://admin.example")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("email challenge outage status = %d, body=%s, want 503", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "person@example.com") || strings.Contains(recorder.Body.String(), "123456") {
		t.Fatalf("email challenge outage response leaked credentials: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "authentication challenge is temporarily unavailable") {
		t.Fatalf("email challenge outage response is not the fixed contract: %s", recorder.Body.String())
	}
}

func TestClearAuthCookieExpiresOnlyV6BrowserCookies(t *testing.T) {
	previousAuth := Auth
	t.Cleanup(func() { Auth = previousAuth })
	Auth = &ginjwt.GinJWTMiddleware{
		SendCookie:     true,
		CookieName:     "jwt",
		CookieDomain:   "example.test",
		SecureCookie:   true,
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteLaxMode,
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ClearAuthCookie(ctx)

	response := recorder.Result()
	cookies := response.Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cleared cookies = %d, want 2", len(cookies))
	}
	byName := make(map[string]*http.Cookie, len(cookies))
	for _, cookie := range cookies {
		byName[cookie.Name] = cookie
	}
	if legacy := byName["jwt"]; legacy != nil {
		t.Fatalf("retired JWT cookie was emitted: %#v", legacy)
	}
	if browser := byName[BrowserSessionCookieName]; browser == nil || browser.MaxAge >= 0 ||
		!browser.HttpOnly || browser.Path != browserSessionCookiePath {
		t.Fatalf("cleared browser session cookie = %#v", browser)
	}
	if csrf := byName[BrowserCSRFCookieName]; csrf == nil || csrf.MaxAge >= 0 ||
		csrf.HttpOnly || csrf.Path != browserCSRFCookiePath {
		t.Fatalf("cleared browser CSRF cookie = %#v", csrf)
	}
}
