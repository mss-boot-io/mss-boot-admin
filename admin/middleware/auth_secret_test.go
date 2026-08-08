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
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
)

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
	router.POST("/user/login", PublicLoginHandler)
	for _, provider := range []pkg.LoginProvider{pkg.GithubLoginProvider, pkg.LarkLoginProvider} {
		body, err := json.Marshal(map[string]string{
			"type":     string(provider),
			"password": "raw-provider-token",
		})
		if err != nil {
			t.Fatalf("marshal login body: %v", err)
		}
		request := httptest.NewRequest(http.MethodPost, "/user/login", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
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

func TestClearAuthCookieExpiresHttpOnlyJWT(t *testing.T) {
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
	if len(cookies) != 1 {
		t.Fatalf("cleared cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "jwt" || cookie.Value != "" || cookie.MaxAge >= 0 ||
		!cookie.HttpOnly || !cookie.Secure || cookie.Domain != "example.test" ||
		cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cleared cookie = %#v", cookie)
	}
}
