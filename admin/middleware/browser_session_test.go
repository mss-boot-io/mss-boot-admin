package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
)

func configureBrowserSessionTest(t *testing.T) {
	t.Helper()
	previousAuthConfig := config.Cfg.Auth
	previousApplication := config.Cfg.Application
	previousCORS := config.Cfg.CORS
	previousAuth := Auth
	t.Cleanup(func() {
		config.Cfg.Auth = previousAuthConfig
		config.Cfg.Application = previousApplication
		config.Cfg.CORS = previousCORS
		Auth = previousAuth
	})
	config.Cfg.Auth = config.Auth{
		Key:         "browser-session-test-signing-key-32-bytes",
		IdentityKey: "browser-session-test-identity",
		Timeout:     time.Hour,
		MaxRefresh:  24 * time.Hour,
		BrowserSession: config.BrowserSession{
			Secure:             true,
			SameSite:           "lax",
			WebSocketTicketTTL: 30 * time.Second,
		},
	}
	config.Cfg.Application.Origin = "https://api.example"
	config.Cfg.CORS.AllowOrigins = []string{"https://admin.example"}
}

func TestBrowserCSRFTokenIsBoundToSession(t *testing.T) {
	const key = "browser-session-test-signing-key-32-bytes"
	token, err := issueBrowserCSRFToken("jwt-one", []byte(key))
	if err != nil {
		t.Fatalf("issueBrowserCSRFToken() error = %v", err)
	}
	if err = validateBrowserCSRFToken(token, "jwt-one", []byte(key)); err != nil {
		t.Fatalf("validateBrowserCSRFToken() error = %v", err)
	}
	if err = validateBrowserCSRFToken(token, "jwt-two", []byte(key)); err == nil {
		t.Fatal("CSRF token validated for a different JWT")
	}
	if err = validateBrowserCSRFToken(token, "jwt-one", []byte("different-signing-key-32-bytes")); err == nil {
		t.Fatal("CSRF token validated with a different key")
	}
}

func TestQueryTokenCannotDisableBrowserSessionCSRFClassification(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/admin/api/options?token=ignored", nil)
	ctx.Request.AddCookie(&http.Cookie{Name: BrowserSessionCookieName, Value: "browser-session"})
	if !RequestUsesBrowserSession(ctx) {
		t.Fatal("query token disabled V6 browser-session CSRF classification")
	}
	ctx.Request.Header.Set("Authorization", "Bearer api-token")
	if RequestUsesBrowserSession(ctx) {
		t.Fatal("explicit API bearer was classified as browser-session authentication")
	}
}

func TestEnforceBrowserCSRFMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureBrowserSessionTest(t)
	csrf, err := issueBrowserCSRFToken("session-jwt", []byte(config.Cfg.Auth.Key))
	if err != nil {
		t.Fatalf("issueBrowserCSRFToken() error = %v", err)
	}

	tests := []struct {
		name       string
		method     string
		origin     string
		cookie     string
		csrfCookie string
		csrfHeader string
		bearer     string
		cookieName string
		want       int
	}{
		{name: "valid cookie mutation", method: http.MethodPost, origin: "https://admin.example", cookie: "session-jwt", csrfCookie: csrf, csrfHeader: csrf, want: http.StatusNoContent},
		{name: "retired jwt cookie is ignored", method: http.MethodPost, origin: "https://admin.example", cookie: "session-jwt", cookieName: "jwt", csrfCookie: csrf, csrfHeader: csrf, want: http.StatusNoContent},
		{name: "safe method", method: http.MethodGet, cookie: "session-jwt", want: http.StatusNoContent},
		{name: "anonymous mutation", method: http.MethodPost, origin: "https://attacker.example", want: http.StatusNoContent},
		{name: "API bearer remains supported", method: http.MethodPost, cookie: "session-jwt", bearer: "api-bearer", want: http.StatusNoContent},
		{name: "missing origin", method: http.MethodPost, cookie: "session-jwt", csrfCookie: csrf, csrfHeader: csrf, want: http.StatusForbidden},
		{name: "untrusted origin", method: http.MethodPost, origin: "https://admin.example.attacker.test", cookie: "session-jwt", csrfCookie: csrf, csrfHeader: csrf, want: http.StatusForbidden},
		{name: "missing header", method: http.MethodPost, origin: "https://admin.example", cookie: "session-jwt", csrfCookie: csrf, want: http.StatusForbidden},
		{name: "mismatched double submit", method: http.MethodPost, origin: "https://admin.example", cookie: "session-jwt", csrfCookie: csrf, csrfHeader: "other", want: http.StatusForbidden},
		{name: "token belongs to other JWT", method: http.MethodPost, origin: "https://admin.example", cookie: "different-jwt", csrfCookie: csrf, csrfHeader: csrf, want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(EnforceBrowserCSRF())
			router.Handle(test.method, "/admin/api/mutation", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(test.method, "/admin/api/mutation", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.bearer != "" {
				request.Header.Set("Authorization", "Bearer "+test.bearer)
			}
			if test.csrfHeader != "" {
				request.Header.Set(BrowserCSRFHeaderName, test.csrfHeader)
			}
			if test.cookie != "" {
				cookieName := test.cookieName
				if cookieName == "" {
					cookieName = BrowserSessionCookieName
				}
				request.AddCookie(&http.Cookie{Name: cookieName, Value: test.cookie})
			}
			if test.csrfCookie != "" {
				request.AddCookie(&http.Cookie{Name: BrowserCSRFCookieName, Value: test.csrfCookie})
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, body=%s, want %d", recorder.Code, recorder.Body.String(), test.want)
			}
		})
	}
}

func TestSetBrowserSessionCookiesUsesSecureSeparatedCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureBrowserSessionTest(t)
	Auth = &jwt.GinJWTMiddleware{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/api/user/session/login", nil)
	if err := SetBrowserSessionCookies(ctx, "signed-admin-jwt", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SetBrowserSessionCookies() error = %v", err)
	}
	cookies := recorder.Result().Cookies()
	byName := make(map[string]*http.Cookie, len(cookies))
	for _, cookie := range cookies {
		byName[cookie.Name] = cookie
	}
	session := byName[BrowserSessionCookieName]
	csrf := byName[BrowserCSRFCookieName]
	if session == nil || session.Value != "signed-admin-jwt" || !session.HttpOnly || !session.Secure ||
		session.Path != browserSessionCookiePath || session.Domain != "" {
		t.Fatalf("session cookie = %#v", session)
	}
	if csrf == nil || csrf.HttpOnly || !csrf.Secure || csrf.Path != browserCSRFCookiePath ||
		strings.Contains(csrf.Value, "signed-admin-jwt") {
		t.Fatalf("CSRF cookie = %#v", csrf)
	}
	if err := validateBrowserCSRFToken(csrf.Value, session.Value, []byte(config.Cfg.Auth.Key)); err != nil {
		t.Fatalf("validateBrowserCSRFToken() error = %v", err)
	}
}

func TestBrowserSessionLoginResponseNeverContainsJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureBrowserSessionTest(t)
	Auth = &jwt.GinJWTMiddleware{
		Realm:       "browser-session-test",
		Key:         []byte(config.Cfg.Auth.Key),
		Timeout:     time.Hour,
		IdentityKey: "identity",
		Authenticator: func(*gin.Context) (any, error) {
			return jwt.MapClaims{"uid": "user-1", "rid": "role-1", "sid": "session-1"}, nil
		},
		PayloadFunc: func(data any) jwt.MapClaims {
			return data.(jwt.MapClaims)
		},
		TokenLookup: "header: Authorization",
	}
	if err := Auth.MiddlewareInit(); err != nil {
		t.Fatalf("MiddlewareInit() error = %v", err)
	}
	router := gin.New()
	router.POST("/admin/api/user/session/login", BrowserSessionLoginHandler)
	request := httptest.NewRequest(http.MethodPost, "/admin/api/user/session/login", nil)
	request.Header.Set("Origin", "https://admin.example")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "token") || strings.Contains(recorder.Body.String(), "eyJ") {
		t.Fatalf("login response leaked JWT: %s", recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", recorder.Header().Get("Cache-Control"))
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 || cookies[0].Value == "" {
		t.Fatalf("session cookies = %#v", cookies)
	}
}

func TestBrowserSessionRefreshRotatesCredentialWithoutReturningIt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureBrowserSessionTest(t)
	db, _ := setupAuthSessionTest(t)
	if err := db.AutoMigrate(&models.Role{}, &models.User{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	role := &models.Role{Name: "browser-user", Status: enum.Enabled}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	user := &models.User{UserLogin: models.UserLogin{
		Username: "browser-user",
		Password: "stored-password",
		RoleID:   role.ID,
		Status:   enum.Enabled,
	}}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	sessionID, err := service.Session.Create(context.Background(), db, service.CreateSessionInput{
		UserID: user.ID, Username: user.Username, RoleID: role.ID, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	now := time.Now().Add(-time.Minute).UTC()
	Auth = &jwt.GinJWTMiddleware{
		Realm:       "browser-session-test",
		Key:         []byte(config.Cfg.Auth.Key),
		Timeout:     time.Hour,
		MaxRefresh:  24 * time.Hour,
		IdentityKey: "identity",
		PayloadFunc: func(data any) jwt.MapClaims { return data.(jwt.MapClaims) },
		TokenLookup: "cookie: " + BrowserSessionCookieName,
		TimeFunc:    func() time.Time { return now },
	}
	if err = Auth.MiddlewareInit(); err != nil {
		t.Fatalf("MiddlewareInit() error = %v", err)
	}
	oldToken, _, err := Auth.TokenGenerator(jwt.MapClaims{
		"uid": user.ID,
		"rid": role.ID,
		"sid": sessionID,
	})
	if err != nil {
		t.Fatalf("TokenGenerator() error = %v", err)
	}
	csrf, err := issueBrowserCSRFToken(oldToken, []byte(config.Cfg.Auth.Key))
	if err != nil {
		t.Fatalf("issueBrowserCSRFToken() error = %v", err)
	}
	now = now.Add(time.Minute)

	router := gin.New()
	router.POST("/admin/api/user/session/refresh-token", BrowserSessionRefreshHandler)
	request := httptest.NewRequest(http.MethodPost, "/admin/api/user/session/refresh-token", nil)
	request.Header.Set("Origin", "https://admin.example")
	request.Header.Set(BrowserCSRFHeaderName, csrf)
	request.AddCookie(&http.Cookie{Name: BrowserSessionCookieName, Value: oldToken})
	request.AddCookie(&http.Cookie{Name: BrowserCSRFCookieName, Value: csrf})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "token") || strings.Contains(recorder.Body.String(), oldToken) {
		t.Fatalf("refresh response leaked a credential: %s", recorder.Body.String())
	}
	var response dto.BrowserSessionResponse
	if err = json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Expire.IsZero() {
		t.Fatalf("refresh response = %#v, error=%v", response, err)
	}
	byName := make(map[string]*http.Cookie)
	for _, cookie := range recorder.Result().Cookies() {
		byName[cookie.Name] = cookie
	}
	newSession := byName[BrowserSessionCookieName]
	newCSRF := byName[BrowserCSRFCookieName]
	if newSession == nil || newCSRF == nil || newSession.Value == oldToken {
		t.Fatalf("rotated cookies = %#v", recorder.Result().Cookies())
	}
	if err = validateBrowserCSRFToken(newCSRF.Value, newSession.Value, []byte(config.Cfg.Auth.Key)); err != nil {
		t.Fatalf("rotated CSRF token is not bound to the new session: %v", err)
	}
}
