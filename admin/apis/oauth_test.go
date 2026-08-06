package apis

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
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
		if first.Code != http.StatusOK {
			t.Fatalf("first callback status = %d, body=%s", first.Code, first.Body.String())
		}
		var token dto.OauthToken
		if err := json.Unmarshal(first.Body.Bytes(), &token); err != nil {
			t.Fatalf("decode callback response: %v", err)
		}
		if token.Intent != string(oauthstate.IntentLogin) || token.Provider != string(pkg.GithubLoginProvider) {
			t.Fatalf("server callback metadata = %#v", token)
		}
		var payload map[string]any
		if err := json.Unmarshal(first.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode callback payload: %v", err)
		}
		if payload["provider"] != string(pkg.GithubLoginProvider) {
			t.Fatalf("provider JSON field = %#v", payload["provider"])
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
		if callback.Code != http.StatusOK || exchanges != 1 {
			t.Fatalf("binding callback status=%d exchanges=%d body=%s", callback.Code, exchanges, callback.Body.String())
		}
		var token dto.OauthToken
		if err := json.Unmarshal(callback.Body.Bytes(), &token); err != nil {
			t.Fatalf("decode callback response: %v", err)
		}
		if token.Intent != string(oauthstate.IntentBinding) || token.Provider != string(pkg.LarkLoginProvider) {
			t.Fatalf("binding callback metadata = %#v", token)
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

func newOAuthTestUser(exchanges *int) *User {
	return &User{
		oauthStates: oauthstate.New(),
		oauthURLBuilder: func(_ *gin.Context, provider pkg.LoginProvider, state string) (string, error) {
			return "https://" + string(provider) + ".example.test/authorize?state=" + url.QueryEscape(state), nil
		},
		oauthCodeExchange: func(_ *gin.Context, _ pkg.LoginProvider, code string) (*dto.OauthToken, error) {
			*exchanges++
			return &dto.OauthToken{AccessToken: "provider-token-" + code}, nil
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
	t.Helper()
	recorder := executeOAuthAuthorize(user, provider, intent, credential, verifier)
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
	body, _ := json.Marshal(dto.OAuthAuthorizeRequest{Provider: provider, Intent: string(intent)})
	request := httptest.NewRequest(http.MethodPost, "/authorize", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if credential != "" {
		request.Header.Set("Authorization", "Bearer "+credential)
	}
	return executeOAuthRequest(request, verifier, user.OAuthAuthorize)
}

func executeOAuthCallback(
	user *User,
	provider pkg.LoginProvider,
	state string,
	credential string,
	verifier security.Verifier,
	cookie *http.Cookie,
) *httptest.ResponseRecorder {
	requestURL := "/callback/" + string(provider) + "?code=provider-code"
	if state != "" {
		requestURL += "&state=" + url.QueryEscape(state)
	}
	request := httptest.NewRequest(http.MethodGet, requestURL, nil)
	if credential != "" {
		request.Header.Set("Authorization", "Bearer "+credential)
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	return executeOAuthRequest(request, verifier, user.Callback)
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
	if request.Method == http.MethodPost {
		router.POST("/authorize", handler)
	} else {
		router.GET("/callback/:provider", handler)
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
