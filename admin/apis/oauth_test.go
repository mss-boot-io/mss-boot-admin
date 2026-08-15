package apis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/oauthstate"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"github.com/redis/go-redis/v9"
)

const testOAuthCredential = "interactive-session-token"

func TestOAuthCallbackValidatesStateBeforeCodeExchange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	installOAuthTestVerifier(t)

	t.Run("successful login is one time and returns server intent", func(t *testing.T) {
		exchanges := 0
		user := newOAuthTestUser(&exchanges)
		state, cookie := issueOAuthState(t, user, pkg.GithubLoginProvider, oauthstate.IntentLogin, "", nil)

		first := executeOAuthCallback(user, pkg.GithubLoginProvider, state, "", nil, cookie)
		if first.Code != http.StatusCreated {
			t.Fatalf("first callback status = %d, body=%s", first.Code, first.Body.String())
		}
		var token dto.OAuthCallbackResponse
		if err := json.Unmarshal(first.Body.Bytes(), &token); err != nil {
			t.Fatalf("decode callback response: %v", err)
		}
		if token.Code != http.StatusOK {
			t.Fatalf("callback business code = %d, want %d", token.Code, http.StatusOK)
		}
		if token.Intent != string(oauthstate.IntentLogin) || token.Provider != pkg.GithubLoginProvider ||
			token.Token != "admin-session-token" || token.Expire == nil {
			t.Fatalf("server callback metadata = %#v", token)
		}
		var payload map[string]any
		if err := json.Unmarshal(first.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode callback payload: %v", err)
		}
		if payload["provider"] != string(pkg.GithubLoginProvider) {
			t.Fatalf("provider JSON field = %#v", payload["provider"])
		}
		if _, exists := payload["accessToken"]; exists {
			t.Fatalf("callback exposed provider access token: %s", first.Body.String())
		}
		if _, exists := payload["refreshToken"]; exists {
			t.Fatalf("callback exposed provider refresh token: %s", first.Body.String())
		}
		if first.Header().Get("Cache-Control") != "no-store" ||
			first.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Fatalf("callback cache headers = %#v", first.Header())
		}
		if exchanges != 1 {
			t.Fatalf("exchange calls = %d, want 1", exchanges)
		}

		replay := executeOAuthCallback(user, pkg.GithubLoginProvider, state, "", nil, cookie)
		if replay.Code != http.StatusUnauthorized || exchanges != 1 {
			t.Fatalf("replay status=%d exchanges=%d, want 401/1", replay.Code, exchanges)
		}
	})

	t.Run("successful binding requires the issuing session", func(t *testing.T) {
		exchanges := 0
		user := newOAuthTestUser(&exchanges)
		verifier := oauthTestUser("user-1", false)
		state, cookie := issueOAuthState(t, user, pkg.LarkLoginProvider, oauthstate.IntentBinding, testOAuthCredential, verifier)
		callback := executeOAuthCallback(user, pkg.LarkLoginProvider, state, testOAuthCredential, verifier, cookie)
		if callback.Code != http.StatusCreated || exchanges != 1 {
			t.Fatalf("binding callback status=%d exchanges=%d body=%s", callback.Code, exchanges, callback.Body.String())
		}
		var token dto.OAuthCallbackResponse
		if err := json.Unmarshal(callback.Body.Bytes(), &token); err != nil {
			t.Fatalf("decode callback response: %v", err)
		}
		if token.Code != http.StatusOK {
			t.Fatalf("callback business code = %d, want %d", token.Code, http.StatusOK)
		}
		if token.Intent != string(oauthstate.IntentBinding) || token.Provider != pkg.LarkLoginProvider ||
			token.Token != "" || token.Expire != nil {
			t.Fatalf("binding callback metadata = %#v", token)
		}
	})

	t.Run("binding rejects an identity owned by another user", func(t *testing.T) {
		exchanges := 0
		user := newOAuthTestUser(&exchanges)
		user.oauthBindingComplete = func(*gin.Context, string, pkg.LoginProvider, string) error {
			return errOAuthIdentityAlreadyBound
		}
		verifier := oauthTestUser("user-1", false)
		state, cookie := issueOAuthState(t, user, pkg.GithubLoginProvider, oauthstate.IntentBinding, testOAuthCredential, verifier)
		callback := executeOAuthCallback(user, pkg.GithubLoginProvider, state, testOAuthCredential, verifier, cookie)
		if callback.Code != http.StatusConflict || exchanges != 1 {
			t.Fatalf("binding conflict status=%d exchanges=%d body=%s", callback.Code, exchanges, callback.Body.String())
		}
		if bytes.Contains(callback.Body.Bytes(), []byte("provider-token")) {
			t.Fatalf("binding conflict leaked provider credential: %s", callback.Body.String())
		}
	})

	negativeTests := []struct {
		name       string
		provider   pkg.LoginProvider
		state      string
		cookie     *http.Cookie
		credential string
		verifier   security.Verifier
		wantStatus int
	}{
		{name: "missing state", provider: pkg.GithubLoginProvider, wantStatus: http.StatusUnprocessableEntity},
		{name: "unknown state", provider: pkg.GithubLoginProvider, state: "not-issued", wantStatus: http.StatusUnauthorized},
	}
	for _, test := range negativeTests {
		t.Run(test.name, func(t *testing.T) {
			exchanges := 0
			user := newOAuthTestUser(&exchanges)
			callback := executeOAuthCallback(user, test.provider, test.state, test.credential, test.verifier, test.cookie)
			if callback.Code != test.wantStatus || exchanges != 0 {
				t.Fatalf("callback status=%d exchanges=%d body=%s", callback.Code, exchanges, callback.Body.String())
			}
		})
	}

	t.Run("expired state", func(t *testing.T) {
		exchanges := 0
		user := newOAuthTestUser(&exchanges)
		user.oauthStates = expiredOAuthStateStore{}
		callback := executeOAuthCallback(user, pkg.GithubLoginProvider, "expired", "", nil, nil)
		if callback.Code != http.StatusUnauthorized || exchanges != 0 {
			t.Fatalf("expired callback status=%d exchanges=%d", callback.Code, exchanges)
		}
	})

	t.Run("provider failure is generic and state remains one time", func(t *testing.T) {
		exchanges := 0
		user := newOAuthTestUser(&exchanges)
		state, cookie := issueOAuthState(t, user, pkg.GithubLoginProvider, oauthstate.IntentLogin, "", nil)
		user.oauthCodeExchange = func(*gin.Context, pkg.LoginProvider, string) (*dto.OauthToken, error) {
			exchanges++
			return nil, errors.New("provider response contained provider-secret-error")
		}
		callback := executeOAuthCallback(user, pkg.GithubLoginProvider, state, "", nil, cookie)
		if callback.Code != http.StatusUnauthorized || exchanges != 1 {
			t.Fatalf("provider failure status=%d exchanges=%d body=%s", callback.Code, exchanges, callback.Body.String())
		}
		if bytes.Contains(callback.Body.Bytes(), []byte("provider-secret-error")) {
			t.Fatalf("provider failure leaked upstream detail: %s", callback.Body.String())
		}
		replay := executeOAuthCallback(user, pkg.GithubLoginProvider, state, "", nil, cookie)
		if replay.Code != http.StatusUnauthorized || exchanges != 1 {
			t.Fatalf("provider failure replay status=%d exchanges=%d", replay.Code, exchanges)
		}
	})

	t.Run("callback code and state are accepted only from POST JSON", func(t *testing.T) {
		exchanges := 0
		user := newOAuthTestUser(&exchanges)
		state, cookie := issueOAuthState(t, user, pkg.GithubLoginProvider, oauthstate.IntentLogin, "", nil)
		request := httptest.NewRequest(
			http.MethodPost,
			"/callback/github?code=query-code&state="+url.QueryEscape(state),
			bytes.NewReader([]byte(`{}`)),
		)
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(cookie)
		queryOnly := executeOAuthRequest(request, nil, user.Callback)
		if queryOnly.Code != http.StatusUnprocessableEntity || exchanges != 0 {
			t.Fatalf("query callback status=%d exchanges=%d body=%s", queryOnly.Code, exchanges, queryOnly.Body.String())
		}
		proper := executeOAuthCallback(user, pkg.GithubLoginProvider, state, "", nil, cookie)
		if proper.Code != http.StatusCreated || exchanges != 1 {
			t.Fatalf("body callback status=%d exchanges=%d body=%s", proper.Code, exchanges, proper.Body.String())
		}
	})
}

func TestOAuthCallbackBurnsStateOnProviderBrowserOrSessionMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	installOAuthTestVerifier(t)

	tests := []struct {
		name       string
		issue      pkg.LoginProvider
		callback   pkg.LoginProvider
		intent     oauthstate.Intent
		issueToken string
		callToken  string
		issueUser  security.Verifier
		callUser   security.Verifier
		mutate     func(*http.Cookie)
	}{
		{
			name:     "provider mismatch",
			issue:    pkg.GithubLoginProvider,
			callback: pkg.LarkLoginProvider,
			intent:   oauthstate.IntentLogin,
		},
		{
			name:     "browser nonce mismatch",
			issue:    pkg.GithubLoginProvider,
			callback: pkg.GithubLoginProvider,
			intent:   oauthstate.IntentLogin,
			mutate:   func(cookie *http.Cookie) { cookie.Value = "attacker-browser" },
		},
		{
			name:       "binding credential mismatch",
			issue:      pkg.LarkLoginProvider,
			callback:   pkg.LarkLoginProvider,
			intent:     oauthstate.IntentBinding,
			issueToken: "session-a",
			callToken:  "session-b",
			issueUser:  oauthTestUser("user-1", false),
			callUser:   oauthTestUser("user-1", false),
		},
		{
			name:       "binding user mismatch",
			issue:      pkg.LarkLoginProvider,
			callback:   pkg.LarkLoginProvider,
			intent:     oauthstate.IntentBinding,
			issueToken: "session-a",
			callToken:  "session-a",
			issueUser:  oauthTestUser("user-1", false),
			callUser:   oauthTestUser("user-2", false),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exchanges := 0
			user := newOAuthTestUser(&exchanges)
			state, cookie := issueOAuthState(t, user, test.issue, test.intent, test.issueToken, test.issueUser)
			originalCookie := *cookie
			if test.mutate != nil {
				test.mutate(cookie)
			}
			callback := executeOAuthCallback(user, test.callback, state, test.callToken, test.callUser, cookie)
			if callback.Code != http.StatusUnauthorized || exchanges != 0 {
				t.Fatalf("mismatch status=%d exchanges=%d body=%s", callback.Code, exchanges, callback.Body.String())
			}

			// Every validation failure after lookup consumes the state. Retrying
			// with the original provider/browser/session must still fail.
			replay := executeOAuthCallback(user, test.issue, state, test.issueToken, test.issueUser, &originalCookie)
			if replay.Code != http.StatusUnauthorized || exchanges != 0 {
				t.Fatalf("mismatch replay status=%d exchanges=%d", replay.Code, exchanges)
			}
		})
	}
}

func TestOAuthAuthorizeEnforcesIntentAuthenticationBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	installOAuthTestVerifier(t)

	tests := []struct {
		name       string
		intent     oauthstate.Intent
		credential string
		verifier   security.Verifier
		wantStatus int
	}{
		{name: "anonymous binding", intent: oauthstate.IntentBinding, wantStatus: http.StatusUnauthorized},
		{name: "personal access token binding", intent: oauthstate.IntentBinding, credential: "pat", verifier: oauthTestUser("user-1", true), wantStatus: http.StatusForbidden},
		{name: "residual authenticated login", intent: oauthstate.IntentLogin, credential: "session", verifier: oauthTestUser("user-1", false), wantStatus: http.StatusConflict},
		{name: "retired integration", intent: oauthstate.Intent("integration"), credential: "session", verifier: oauthTestUser("user-1", false), wantStatus: http.StatusUnprocessableEntity},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exchanges := 0
			user := newOAuthTestUser(&exchanges)
			recorder := executeOAuthAuthorize(user, pkg.GithubLoginProvider, test.intent, test.credential, test.verifier)
			if recorder.Code != test.wantStatus {
				t.Fatalf("authorize status=%d want=%d body=%s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
		})
	}
}

func TestOAuthBrowserTransportNeverExposesAdminToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	installOAuthTestVerifier(t)
	configureOAuthBrowserSession(t)

	exchanges := 0
	user := newOAuthTestUser(&exchanges)
	state, cookie := issueOAuthStateWithHandler(
		t,
		user,
		pkg.GithubLoginProvider,
		oauthstate.IntentLogin,
		"",
		nil,
		user.SessionOAuthAuthorize,
	)
	callback := executeOAuthCallbackWithHandler(
		user,
		pkg.GithubLoginProvider,
		state,
		"",
		nil,
		cookie,
		user.SessionCallback,
	)
	if callback.Code != http.StatusCreated || exchanges != 1 {
		t.Fatalf("browser callback status=%d exchanges=%d body=%s", callback.Code, exchanges, callback.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(callback.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode browser callback: %v", err)
	}
	for _, forbidden := range []string{"token", "accessToken", "refreshToken"} {
		if _, exists := payload[forbidden]; exists {
			t.Fatalf("browser callback exposed %s: %s", forbidden, callback.Body.String())
		}
	}
	cookies := callback.Result().Cookies()
	byName := make(map[string]*http.Cookie, len(cookies))
	for _, responseCookie := range cookies {
		byName[responseCookie.Name] = responseCookie
	}
	if byName[middleware.BrowserSessionCookieName] == nil ||
		byName[middleware.BrowserCSRFCookieName] == nil {
		t.Fatalf("browser callback cookies = %#v", cookies)
	}
}

func TestOAuthStateBurnsOnResponseTransportSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	installOAuthTestVerifier(t)
	configureOAuthBrowserSession(t)

	tests := []struct {
		name             string
		authorizeHandler gin.HandlerFunc
		callbackHandler  gin.HandlerFunc
	}{
		{name: "browser state on bearer callback"},
		{name: "bearer state on browser callback"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exchanges := 0
			user := newOAuthTestUser(&exchanges)
			authorizeHandler := user.SessionOAuthAuthorize
			callbackHandler := user.Callback
			replayHandler := user.SessionCallback
			if test.name == "bearer state on browser callback" {
				authorizeHandler = user.OAuthAuthorize
				callbackHandler = user.SessionCallback
				replayHandler = user.Callback
			}
			state, cookie := issueOAuthStateWithHandler(
				t,
				user,
				pkg.GithubLoginProvider,
				oauthstate.IntentLogin,
				"",
				nil,
				authorizeHandler,
			)
			callback := executeOAuthCallbackWithHandler(
				user,
				pkg.GithubLoginProvider,
				state,
				"",
				nil,
				cookie,
				callbackHandler,
			)
			if callback.Code != http.StatusUnauthorized || exchanges != 0 {
				t.Fatalf("transport switch status=%d exchanges=%d body=%s", callback.Code, exchanges, callback.Body.String())
			}
			replay := executeOAuthCallbackWithHandler(
				user,
				pkg.GithubLoginProvider,
				state,
				"",
				nil,
				cookie,
				replayHandler,
			)
			if replay.Code != http.StatusUnauthorized || exchanges != 0 {
				t.Fatalf("transport-switch replay status=%d exchanges=%d", replay.Code, exchanges)
			}
		})
	}
}

func TestOAuthProviderConfigurationIsTransportSpecific(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := center.GetAppConfig()
	settings := oauthRedirectAppConfig{
		"security:githubClientId":                   "v5-github-id",
		"security:githubClientSecret":               "v5-github-secret",
		"security:githubRedirectURI":                "https://v5.example/user/callback/github",
		"security:githubBrowserSessionClientId":     "v6-github-id",
		"security:githubBrowserSessionClientSecret": "v6-github-secret",
		"security:githubBrowserSessionRedirectURI":  "https://v6.example/user/callback/github",
		"security:larkAppId":                        "v5-lark-id",
		"security:larkAppSecret":                    "v5-lark-secret",
		"security:larkRedirectURI":                  "https://v5.example/user/callback/lark",
		"security:larkBrowserSessionAppId":          "v6-lark-id",
		"security:larkBrowserSessionAppSecret":      "v6-lark-secret",
		"security:larkBrowserSessionRedirectURI":    "https://v6.example/user/callback/lark",
	}
	center.SetAppConfig(settings)
	t.Cleanup(func() { center.SetAppConfig(previous) })

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	for _, test := range []struct {
		provider      pkg.LoginProvider
		legacy        string
		browser       string
		legacyID      string
		browserID     string
		legacySecret  string
		browserSecret string
	}{
		{provider: pkg.GithubLoginProvider, legacy: settings["security:githubRedirectURI"], browser: settings["security:githubBrowserSessionRedirectURI"], legacyID: settings["security:githubClientId"], browserID: settings["security:githubBrowserSessionClientId"], legacySecret: settings["security:githubClientSecret"], browserSecret: settings["security:githubBrowserSessionClientSecret"]},
		{provider: pkg.LarkLoginProvider, legacy: settings["security:larkRedirectURI"], browser: settings["security:larkBrowserSessionRedirectURI"], legacyID: settings["security:larkAppId"], browserID: settings["security:larkBrowserSessionAppId"], legacySecret: settings["security:larkAppSecret"], browserSecret: settings["security:larkBrowserSessionAppSecret"]},
	} {
		got, err := oauthRedirectURL(ctx, test.provider)
		if err != nil || got != test.legacy {
			t.Fatalf("legacy redirect = (%q, %v), want %q", got, err, test.legacy)
		}
		if got, err = oauthClientID(ctx, test.provider); err != nil || got != test.legacyID {
			t.Fatalf("legacy client ID = (%q, %v), want %q", got, err, test.legacyID)
		}
		if got, err = oauthClientSecret(ctx, test.provider); err != nil || got != test.legacySecret {
			t.Fatalf("legacy client secret selection failed: %v", err)
		}
		ctx.Set(oauthTransportContextKey, oauthstate.TransportBrowserCookie)
		got, err = oauthRedirectURL(ctx, test.provider)
		if err != nil || got != test.browser {
			t.Fatalf("browser redirect = (%q, %v), want %q", got, err, test.browser)
		}
		if got, err = oauthClientID(ctx, test.provider); err != nil || got != test.browserID {
			t.Fatalf("browser client ID = (%q, %v), want %q", got, err, test.browserID)
		}
		if got, err = oauthClientSecret(ctx, test.provider); err != nil || got != test.browserSecret {
			t.Fatalf("browser client secret selection failed: %v", err)
		}
		ctx.Set(oauthTransportContextKey, oauthstate.TransportBearer)
	}
	delete(settings, "security:githubBrowserSessionRedirectURI")
	ctx.Set(oauthTransportContextKey, oauthstate.TransportBrowserCookie)
	if _, err := oauthRedirectURL(ctx, pkg.GithubLoginProvider); err == nil {
		t.Fatal("browser OAuth silently fell back to the V5 redirect URI")
	}
}

func newOAuthTestUser(exchanges *int) *User {
	return &User{
		oauthStates: oauthstate.New(),
		oauthURLBuilder: func(_ *gin.Context, provider pkg.LoginProvider, state string) (string, error) {
			return "https://" + string(provider) + ".example.test/authorize?state=" + url.QueryEscape(state), nil
		},
		oauthCodeExchange: func(_ *gin.Context, _ pkg.LoginProvider, code string) (*dto.OauthToken, error) {
			*exchanges++
			expiry := time.Now().Add(5 * time.Minute).UTC()
			return &dto.OauthToken{
				AccessToken:  "provider-token-" + code,
				RefreshToken: "provider-refresh-token",
				Expiry:       &expiry,
			}, nil
		},
		oauthLoginComplete: func(_ *gin.Context, _ pkg.LoginProvider, _ string) (string, time.Time, error) {
			return "admin-session-token", time.Now().Add(time.Hour).UTC(), nil
		},
		oauthBindingComplete: func(_ *gin.Context, _ string, _ pkg.LoginProvider, _ string) error {
			return nil
		},
	}
}

func issueOAuthState(
	t *testing.T,
	user *User,
	provider pkg.LoginProvider,
	intent oauthstate.Intent,
	credential string,
	verifier security.Verifier,
) (string, *http.Cookie) {
	return issueOAuthStateWithHandler(t, user, provider, intent, credential, verifier, user.OAuthAuthorize)
}

func issueOAuthStateWithHandler(
	t *testing.T,
	user *User,
	provider pkg.LoginProvider,
	intent oauthstate.Intent,
	credential string,
	verifier security.Verifier,
	handler gin.HandlerFunc,
) (string, *http.Cookie) {
	t.Helper()
	recorder := executeOAuthAuthorizeWithHandler(user, provider, intent, credential, verifier, handler)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("authorize status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var result dto.OAuthAuthorizeResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode authorize response: %v", err)
	}
	authorizeURL, err := url.Parse(result.AuthorizeURL)
	if err != nil {
		t.Fatalf("parse authorize URL: %v", err)
	}
	state := authorizeURL.Query().Get("state")
	if state == "" {
		t.Fatal("authorize URL did not contain server state")
	}
	if result.AttemptID != oauthstate.Digest(state) {
		t.Fatalf("authorize attempt ID = %q, want digest of server state", result.AttemptID)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("authorize Cache-Control = %q, want no-store", recorder.Header().Get("Cache-Control"))
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("authorize cookie = %#v", cookies)
	}
	return state, cookies[0]
}

func executeOAuthAuthorize(
	user *User,
	provider pkg.LoginProvider,
	intent oauthstate.Intent,
	credential string,
	verifier security.Verifier,
) *httptest.ResponseRecorder {
	return executeOAuthAuthorizeWithHandler(user, provider, intent, credential, verifier, user.OAuthAuthorize)
}

func executeOAuthAuthorizeWithHandler(
	user *User,
	provider pkg.LoginProvider,
	intent oauthstate.Intent,
	credential string,
	verifier security.Verifier,
	handler gin.HandlerFunc,
) *httptest.ResponseRecorder {
	body, _ := json.Marshal(dto.OAuthAuthorizeRequest{Provider: provider, Intent: string(intent)})
	request := httptest.NewRequest(http.MethodPost, "/authorize", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if credential != "" {
		request.Header.Set("Authorization", "Bearer "+credential)
	}
	return executeOAuthRequest(request, verifier, handler)
}

func executeOAuthCallback(
	user *User,
	provider pkg.LoginProvider,
	state string,
	credential string,
	verifier security.Verifier,
	cookie *http.Cookie,
) *httptest.ResponseRecorder {
	return executeOAuthCallbackWithHandler(user, provider, state, credential, verifier, cookie, user.Callback)
}

func executeOAuthCallbackWithHandler(
	user *User,
	provider pkg.LoginProvider,
	state string,
	credential string,
	verifier security.Verifier,
	cookie *http.Cookie,
	handler gin.HandlerFunc,
) *httptest.ResponseRecorder {
	body, _ := json.Marshal(dto.OauthCallbackReq{Code: "provider-code", State: state})
	requestURL := "/callback/" + string(provider)
	request := httptest.NewRequest(http.MethodPost, requestURL, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if credential != "" {
		request.Header.Set("Authorization", "Bearer "+credential)
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	return executeOAuthRequest(request, verifier, handler)
}

func executeOAuthRequest(request *http.Request, verifier security.Verifier, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if verifier != nil {
			c.Set("oauth-test-verifier", verifier)
		}
		c.Next()
	})
	if request.URL.Path == "/authorize" {
		router.POST("/authorize", handler)
	} else {
		router.POST("/callback/:provider", handler)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func installOAuthTestVerifier(t *testing.T) {
	t.Helper()
	previous := response.VerifyHandler
	response.VerifyHandler = func(c *gin.Context) security.Verifier {
		value, exists := c.Get("oauth-test-verifier")
		if !exists {
			return nil
		}
		verifier, _ := value.(security.Verifier)
		return verifier
	}
	t.Cleanup(func() { response.VerifyHandler = previous })
}

func configureOAuthBrowserSession(t *testing.T) {
	t.Helper()
	previousConfig := config.Cfg.Auth
	previousAuth := middleware.Auth
	config.Cfg.Auth = config.Auth{
		Key:            "oauth-browser-session-test-key-32-bytes",
		IdentityKey:    "oauth-browser-session-test-identity",
		Timeout:        time.Hour,
		SessionEnabled: true,
		BrowserSession: config.BrowserSession{
			Enabled:  true,
			Secure:   true,
			SameSite: "lax",
		},
	}
	middleware.Auth = &jwt.GinJWTMiddleware{}
	t.Cleanup(func() {
		config.Cfg.Auth = previousConfig
		middleware.Auth = previousAuth
	})
}

func oauthTestUser(userID string, personalAccessToken bool) *models.User {
	user := &models.User{}
	user.ID = userID
	if personalAccessToken {
		user.RefreshTokenDisable = true
		user.PersonAccessToken = "pat"
	}
	return user
}

type expiredOAuthStateStore struct{}

type oauthRedirectAppConfig map[string]string

func (oauthRedirectAppConfig) SetAppConfig(*gin.Context, string, bool, string) error {
	return nil
}

func (settings oauthRedirectAppConfig) GetAppConfig(_ *gin.Context, key string) (string, bool) {
	value, ok := settings[key]
	return value, ok
}

func (expiredOAuthStateStore) Issue(
	context.Context,
	redis.UniversalClient,
	oauthstate.Record,
	time.Duration,
) (string, string, oauthstate.Record, error) {
	return "", "", oauthstate.Record{}, oauthstate.ErrExpired
}

func (expiredOAuthStateStore) Consume(
	context.Context,
	redis.UniversalClient,
	string,
) (oauthstate.Record, error) {
	return oauthstate.Record{}, oauthstate.ErrExpired
}
