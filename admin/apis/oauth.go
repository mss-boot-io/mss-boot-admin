package apis

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkauthen "github.com/larksuite/oapi-sdk-go/v3/service/authen/v1"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/oauthstate"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

const oauthBrowserCookiePrefix = "mss_oauth_"

var defaultOAuthStateStore = oauthstate.New()

type oauthStateStore interface {
	Issue(context.Context, redis.UniversalClient, oauthstate.Record, time.Duration) (string, string, oauthstate.Record, error)
	Consume(context.Context, redis.UniversalClient, string) (oauthstate.Record, error)
}

type oauthAuthorizeURLBuilder func(*gin.Context, pkg.LoginProvider, string) (string, error)
type oauthCodeExchange func(*gin.Context, pkg.LoginProvider, string) (string, error)
type oauthLoginCompleter func(*gin.Context, pkg.LoginProvider, string) (string, time.Time, error)
type oauthBindingCompleter func(*gin.Context, string, pkg.LoginProvider, string) error
type oauthReauthenticationCompleter func(*gin.Context, string, string, pkg.LoginProvider, string) error

var errOAuthIdentityAlreadyBound = errors.New("oauth identity is already bound")
var errOAuthReauthenticationIdentityMismatch = errors.New("oauth reauthentication identity does not match")

func (e *User) stateStore() oauthStateStore {
	if e.oauthStates != nil {
		return e.oauthStates
	}
	return defaultOAuthStateStore
}

func (e *User) authorizeURLBuilder() oauthAuthorizeURLBuilder {
	if e.oauthURLBuilder != nil {
		return e.oauthURLBuilder
	}
	return buildOAuthAuthorizeURL
}

func (e *User) codeExchange() oauthCodeExchange {
	if e.oauthCodeExchange != nil {
		return e.oauthCodeExchange
	}
	return exchangeOAuthCode
}

func (e *User) loginCompleter() oauthLoginCompleter {
	if e.oauthLoginComplete != nil {
		return e.oauthLoginComplete
	}
	return completeOAuthLogin
}

func (e *User) bindingCompleter() oauthBindingCompleter {
	if e.oauthBindingComplete != nil {
		return e.oauthBindingComplete
	}
	return completeOAuthBinding
}

func (e *User) reauthenticationCompleter() oauthReauthenticationCompleter {
	if e.oauthReauthComplete != nil {
		return e.oauthReauthComplete
	}
	return completeOAuthReauthentication
}

// SessionOAuthAuthorize starts the only supported browser OAuth flow.
// @Summary Start browser-session OAuth2 authorization
// @Tags user
// @Accept json
// @Produce json
// @Param data body dto.OAuthAuthorizeRequest true "data"
// @Success 201 {object} dto.OAuthAuthorizeResponse
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 503 {object} response.Response
// @Router /admin/api/user/session/oauth2/authorize [post]
func (e *User) SessionOAuthAuthorize(c *gin.Context) {
	if !middleware.BrowserSessionAvailable() {
		response.Make(c).Err(http.StatusServiceUnavailable)
		return
	}
	e.oauthAuthorize(c)
}

// SessionCallback completes OAuth login into an HttpOnly browser cookie and
// deliberately omits the Admin JWT from the response body.
// @Summary Complete browser-session OAuth2 callback
// @Tags user
// @Accept json
// @Produce json
// @Param provider path string true "provider"
// @Param data body dto.OAuthCallbackRequest true "data"
// @Success 201 {object} dto.OAuthCallbackResponse
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 503 {object} response.Response
// @Router /admin/api/user/session/{provider}/callback [post]
func (e *User) SessionCallback(c *gin.Context) {
	if !middleware.BrowserSessionAvailable() {
		response.Make(c).Err(http.StatusServiceUnavailable)
		return
	}
	e.oauthCallback(c)
}

// oauthAuthorize issues a server-owned, browser-bound authorization attempt.
func (e *User) oauthAuthorize(c *gin.Context) {
	setOAuthNoStoreHeaders(c)
	api := response.Make(c)
	req := &dto.OAuthAuthorizeRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if req.Provider != pkg.GithubLoginProvider && req.Provider != pkg.LarkLoginProvider {
		api.Err(http.StatusUnprocessableEntity)
		return
	}

	intent := oauthstate.Intent(req.Intent)
	verify := currentVerifier(c)
	record := oauthstate.Record{Provider: string(req.Provider), Intent: intent}
	switch intent {
	case oauthstate.IntentLogin:
		// Login initiation must be anonymous. A stale or residual identity is
		// rejected instead of silently becoming part of a different flow.
		if verify != nil || middleware.RequestUsesBrowserSession(c) ||
			strings.TrimSpace(c.GetHeader("Authorization")) != "" {
			api.Err(http.StatusConflict)
			return
		}
	case oauthstate.IntentBinding:
		if verify == nil {
			api.Err(http.StatusUnauthorized)
			return
		}
		if middleware.IsPersonalAccessTokenVerifier(verify) {
			api.Err(http.StatusForbidden)
			return
		}
		sid := middleware.CurrentSessionID(c)
		if !middleware.RequestUsesBrowserSession(c) || sid == "" {
			api.Err(http.StatusForbidden)
			return
		}
		record.UserID = verify.GetUserID()
		record.SessionID = sid
	case oauthstate.IntentReauthentication:
		sid := middleware.CurrentSessionID(c)
		if verify == nil {
			api.Err(http.StatusUnauthorized)
			return
		}
		if middleware.IsPersonalAccessTokenVerifier(verify) {
			api.Err(http.StatusForbidden)
			return
		}
		if !middleware.RequestUsesBrowserSession(c) || sid == "" {
			api.Err(http.StatusForbidden)
			return
		}
		record.UserID = verify.GetUserID()
		record.SessionID = sid
	default:
		api.Err(http.StatusUnprocessableEntity)
		return
	}

	state, browserNonce, issued, err := e.stateStore().Issue(
		c,
		center.GetCache(),
		record,
		oauthstate.DefaultTTL,
	)
	if err != nil {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	authorizeURL, err := e.authorizeURLBuilder()(c, req.Provider, state)
	if err != nil {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	setOAuthBrowserCookie(c, state, browserNonce, oauthstate.DefaultTTL)
	api.OK(&dto.OAuthAuthorizeResponse{
		AuthorizeURL: authorizeURL,
		AttemptID:    oauthstate.Digest(state),
		ExpiresAt:    issued.ExpiresAt,
	})
}

// oauthCallback exchanges an authorization code only after consuming and
// validating the server-issued state and its browser/session bindings.
func (e *User) oauthCallback(c *gin.Context) {
	setOAuthNoStoreHeaders(c)
	api := response.Make(c)
	req := &dto.OAuthCallbackRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}

	record, err := e.stateStore().Consume(c, center.GetCache(), req.State)
	clearOAuthBrowserCookie(c, req.State)
	if err != nil {
		if errors.Is(err, oauthstate.ErrNotFound) || errors.Is(err, oauthstate.ErrExpired) {
			api.Err(http.StatusUnauthorized)
			return
		}
		api.Err(http.StatusServiceUnavailable)
		return
	}
	if record.Provider != string(req.Provider) || !record.Intent.Valid() {
		api.Err(http.StatusUnauthorized)
		return
	}
	browserNonce, cookieErr := c.Cookie(oauthBrowserCookieName(req.State))
	if cookieErr != nil || !constantTimeEqual(oauthstate.Digest(browserNonce), record.BrowserHash) {
		api.Err(http.StatusUnauthorized)
		return
	}

	verify := currentVerifier(c)
	switch record.Intent {
	case oauthstate.IntentLogin:
		if verify != nil || middleware.RequestUsesBrowserSession(c) ||
			strings.TrimSpace(c.GetHeader("Authorization")) != "" ||
			record.UserID != "" || record.SessionID != "" {
			api.Err(http.StatusUnauthorized)
			return
		}
	case oauthstate.IntentBinding:
		if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) ||
			!middleware.RequestUsesBrowserSession(c) ||
			record.UserID == "" || record.UserID != verify.GetUserID() ||
			record.SessionID == "" || record.SessionID != middleware.CurrentSessionID(c) {
			api.Err(http.StatusUnauthorized)
			return
		}
	case oauthstate.IntentReauthentication:
		if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) ||
			!middleware.RequestUsesBrowserSession(c) ||
			record.UserID == "" || record.UserID != verify.GetUserID() ||
			record.SessionID == "" || record.SessionID != middleware.CurrentSessionID(c) {
			api.Err(http.StatusUnauthorized)
			return
		}
	}

	providerAccessToken, err := e.codeExchange()(c, req.Provider, req.Code)
	if err != nil || strings.TrimSpace(providerAccessToken) == "" {
		// Provider failures may contain credentials in response details. Do not
		// attach or log the raw error from this public callback.
		api.Err(http.StatusUnauthorized)
		return
	}

	result := &dto.OAuthCallbackResponse{
		// The callback uses HTTP 201 because it consumes a one-time attempt,
		// while the response body keeps the established API success code used
		// by the login persistence path.
		Code:      http.StatusOK,
		Provider:  req.Provider,
		Intent:    string(record.Intent),
		AttemptID: oauthstate.Digest(req.State),
	}
	switch record.Intent {
	case oauthstate.IntentLogin:
		token, expiresAt, completeErr := e.loginCompleter()(c, req.Provider, providerAccessToken)
		if completeErr != nil || token == "" || expiresAt.IsZero() {
			api.Err(http.StatusUnauthorized)
			return
		}
		if cookieErr := middleware.SetBrowserSessionCookies(c, token, expiresAt); cookieErr != nil {
			api.Err(http.StatusInternalServerError)
			return
		}
		result.Expire = &expiresAt
	case oauthstate.IntentBinding:
		completeErr := e.bindingCompleter()(c, record.UserID, req.Provider, providerAccessToken)
		if errors.Is(completeErr, errOAuthIdentityAlreadyBound) {
			api.Err(http.StatusConflict)
			return
		}
		if completeErr != nil {
			api.Log.Error("oauth binding completion failed")
			api.Err(http.StatusInternalServerError)
			return
		}
	case oauthstate.IntentReauthentication:
		completeErr := e.reauthenticationCompleter()(
			c,
			record.UserID,
			record.SessionID,
			req.Provider,
			providerAccessToken,
		)
		if errors.Is(completeErr, errOAuthReauthenticationIdentityMismatch) ||
			errors.Is(completeErr, service.ErrSecuritySessionUnavailable) {
			api.Err(http.StatusUnauthorized)
			return
		}
		if completeErr != nil {
			api.Log.Error("oauth reauthentication completion failed")
			api.Err(http.StatusInternalServerError)
			return
		}
	default:
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	api.OK(result)
}

func completeOAuthLogin(
	c *gin.Context,
	provider pkg.LoginProvider,
	providerAccessToken string,
) (string, time.Time, error) {
	login := &models.User{UserLogin: models.UserLogin{
		Provider: provider,
		Password: providerAccessToken,
	}}
	return middleware.AuthenticateAndGenerateToken(c, login)
}

func completeOAuthBinding(
	c *gin.Context,
	userID string,
	provider pkg.LoginProvider,
	providerAccessToken string,
) error {
	identity, err := resolveOAuthIdentity(c, userID, provider, providerAccessToken)
	if err != nil {
		return err
	}
	return persistOAuthBinding(c, userID, identity)
}

func resolveOAuthIdentity(
	c *gin.Context,
	userID string,
	provider pkg.LoginProvider,
	providerAccessToken string,
) (*models.UserOAuth2, error) {
	user := &models.User{}
	if err := center.GetDB(c, user).Where("id = ?", userID).First(user).Error; err != nil {
		return nil, err
	}
	previousPassword := user.Password
	user.Password = providerAccessToken
	defer func() { user.Password = previousPassword }()

	var (
		identity *models.UserOAuth2
		err      error
	)
	switch provider {
	case pkg.GithubLoginProvider:
		identity, err = user.GetUserGithubOAuth2(c)
	case pkg.LarkLoginProvider:
		identity, err = user.GetUserLarkOAuth2(c)
	default:
		return nil, errors.New("oauth provider is unsupported")
	}
	if err != nil {
		return nil, err
	}
	if identity == nil {
		return nil, errors.New("oauth identity is missing")
	}
	return identity, nil
}

func completeOAuthReauthentication(
	c *gin.Context,
	userID, sid string,
	provider pkg.LoginProvider,
	providerAccessToken string,
) error {
	identity, err := resolveOAuthIdentity(c, userID, provider, providerAccessToken)
	if err != nil {
		return err
	}
	if identity.ID == "" || identity.UserID != userID || identity.Provider != provider {
		return errOAuthReauthenticationIdentityMismatch
	}
	return service.Session.MarkRecentlyAuthenticated(
		c,
		center.Default.GetDB(c, &models.UserSession{}),
		sid,
		userID,
	)
}

func persistOAuthBinding(c *gin.Context, userID string, identity *models.UserOAuth2) error {
	if identity == nil {
		return errors.New("oauth identity is missing")
	}
	if identity.ID != "" {
		if identity.UserID == userID {
			return nil
		}
		return errOAuthIdentityAlreadyBound
	}
	identity.User = nil
	identity.UserID = userID
	db := center.GetDB(c, identity)
	if err := db.Create(identity).Error; err != nil {
		return normalizeOAuthIdentityCreateError(db, userID, identity, err)
	}
	return nil
}

// normalizeOAuthIdentityCreateError resolves the provider-scoped identity
// after a failed insert. This makes concurrent uniqueness conflicts stable
// even when GORM TranslateError is disabled (the repository default).
func normalizeOAuthIdentityCreateError(
	db *gorm.DB,
	userID string,
	identity *models.UserOAuth2,
	createErr error,
) error {
	if db != nil && identity != nil && identity.IdentityKey != nil {
		existing := &models.UserOAuth2{}
		lookupErr := db.Unscoped().
			Where("identity_key = ?", *identity.IdentityKey).
			Take(existing).Error
		if lookupErr == nil {
			if !existing.DeletedAt.Valid && existing.UserID == userID {
				return nil
			}
			return errOAuthIdentityAlreadyBound
		}
	}
	if isOAuthIdentityUniqueViolation(createErr) {
		return errOAuthIdentityAlreadyBound
	}
	return createErr
}

type sqlStateError interface {
	SQLState() string
}

func isOAuthIdentityUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var stateErr sqlStateError
	if errors.As(err, &stateErr) && stateErr.SQLState() == "23505" {
		return true
	}
	message := strings.ToLower(err.Error())
	identityConstraint := strings.Contains(message, "identity_key") ||
		strings.Contains(message, "ux_user_oauth2_identity_key")
	return identityConstraint && (strings.Contains(message, "duplicate entry") ||
		strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "violates unique"))
}

func setOAuthNoStoreHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "no-referrer")
}

func currentVerifier(c *gin.Context) security.Verifier {
	if response.VerifyHandler == nil {
		return nil
	}
	return response.VerifyHandler(c)
}

func oauthBrowserCookieName(state string) string {
	digest := oauthstate.Digest(state)
	return oauthBrowserCookiePrefix + digest[:24]
}

func setOAuthBrowserCookie(c *gin.Context, state, browserNonce string, ttl time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthBrowserCookieName(state),
		Value:    browserNonce,
		Path:     "/admin/api/user/",
		MaxAge:   int(ttl / time.Second),
		HttpOnly: true,
		Secure:   config.Cfg.Auth.BrowserSession.Secure || requestIsHTTPS(c),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOAuthBrowserCookie(c *gin.Context, state string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthBrowserCookieName(state),
		Path:     "/admin/api/user/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   config.Cfg.Auth.BrowserSession.Secure || requestIsHTTPS(c),
		SameSite: http.SameSiteLaxMode,
	})
}

func requestIsHTTPS(c *gin.Context) bool {
	return c != nil && c.Request != nil &&
		(c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https"))
}

func constantTimeEqual(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func buildOAuthAuthorizeURL(c *gin.Context, provider pkg.LoginProvider, state string) (string, error) {
	switch provider {
	case pkg.GithubLoginProvider:
		clientID, err := oauthClientID(c, provider)
		if err != nil {
			return "", err
		}
		redirectURL, err := oauthRedirectURL(c, provider)
		if err != nil {
			return "", err
		}
		scope := oauthScope(c, provider)
		return (&oauth2.Config{
			ClientID:    clientID,
			RedirectURL: redirectURL,
			Scopes:      splitScopes(scope),
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://github.com/login/oauth/authorize",
				TokenURL: "https://github.com/login/oauth/access_token",
			},
		}).AuthCodeURL(state), nil
	case pkg.LarkLoginProvider:
		appID, err := oauthClientID(c, provider)
		if err != nil {
			return "", err
		}
		redirectURI, err := oauthRedirectURL(c, provider)
		if err != nil {
			return "", err
		}
		authorizeURL, _ := url.Parse("https://open.larksuite.com/open-apis/authen/v1/index")
		query := authorizeURL.Query()
		query.Set("redirect_uri", redirectURI)
		query.Set("app_id", appID)
		query.Set("state", state)
		authorizeURL.RawQuery = query.Encode()
		return authorizeURL.String(), nil
	default:
		return "", errors.New("unsupported oauth provider")
	}
}

func exchangeOAuthCode(c *gin.Context, provider pkg.LoginProvider, code string) (string, error) {
	switch provider {
	case pkg.GithubLoginProvider:
		clientID, err := oauthClientID(c, provider)
		if err != nil {
			return "", err
		}
		clientSecret, err := oauthClientSecret(c, provider)
		if err != nil {
			return "", err
		}
		redirectURL, err := oauthRedirectURL(c, provider)
		if err != nil {
			return "", err
		}
		scope := oauthScope(c, provider)
		token, err := (&oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       splitScopes(scope),
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://github.com/login/oauth/authorize",
				TokenURL: "https://github.com/login/oauth/access_token",
			},
		}).Exchange(c, code)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(token.AccessToken), nil
	case pkg.LarkLoginProvider:
		appID, err := oauthClientID(c, provider)
		if err != nil {
			return "", err
		}
		appSecret, err := oauthClientSecret(c, provider)
		if err != nil {
			return "", err
		}
		client := lark.NewClient(appID, appSecret)
		request := larkauthen.NewCreateAccessTokenReqBuilder().
			Body(larkauthen.NewCreateAccessTokenReqBodyBuilder().
				GrantType("authorization_code").
				Code(code).
				Build()).Build()
		providerResponse, err := client.Authen.AccessToken.Create(c, request)
		if err != nil || !providerResponse.Success() || providerResponse.Data == nil ||
			providerResponse.Data.AccessToken == nil || strings.TrimSpace(*providerResponse.Data.AccessToken) == "" {
			if err != nil {
				return "", err
			}
			return "", errors.New("lark oauth exchange failed")
		}
		return strings.TrimSpace(*providerResponse.Data.AccessToken), nil
	default:
		return "", errors.New("unsupported oauth provider")
	}
}

func requiredAppConfig(c *gin.Context, keys ...string) (string, error) {
	if value, ok := appConfigAny(c, keys...); ok && strings.TrimSpace(value) != "" {
		return value, nil
	}
	return "", fmt.Errorf("required oauth configuration is missing")
}

func oauthRedirectURL(c *gin.Context, provider pkg.LoginProvider) (string, error) {
	switch provider {
	case pkg.GithubLoginProvider:
		return requiredAppConfig(c,
			"security:githubBrowserSessionRedirectURI",
			"security:githubBrowserSessionRedirectURL",
		)
	case pkg.LarkLoginProvider:
		return requiredAppConfig(c,
			"security:larkBrowserSessionRedirectURI",
			"security:larkBrowserSessionRedirectURL",
		)
	default:
		return "", errors.New("unsupported oauth provider")
	}
}

func oauthClientID(c *gin.Context, provider pkg.LoginProvider) (string, error) {
	switch provider {
	case pkg.GithubLoginProvider:
		return requiredAppConfig(c, "security:githubBrowserSessionClientId")
	case pkg.LarkLoginProvider:
		return requiredAppConfig(c, "security:larkBrowserSessionAppId")
	default:
		return "", errors.New("unsupported oauth provider")
	}
}

func oauthClientSecret(c *gin.Context, provider pkg.LoginProvider) (string, error) {
	switch provider {
	case pkg.GithubLoginProvider:
		return requiredAppConfig(c, "security:githubBrowserSessionClientSecret")
	case pkg.LarkLoginProvider:
		return requiredAppConfig(c, "security:larkBrowserSessionAppSecret")
	default:
		return "", errors.New("unsupported oauth provider")
	}
}

func oauthScope(c *gin.Context, provider pkg.LoginProvider) string {
	if provider != pkg.GithubLoginProvider {
		return ""
	}
	scope, _ := appConfigAny(c, "security:githubBrowserSessionScope")
	return scope
}

func appConfigAny(c *gin.Context, keys ...string) (string, bool) {
	store := center.GetAppConfig()
	if store == nil {
		return "", false
	}
	for _, key := range keys {
		if value, ok := store.GetAppConfig(c, key); ok && strings.TrimSpace(value) != "" {
			return value, true
		}
	}
	return "", false
}

func splitScopes(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if scope := strings.TrimSpace(part); scope != "" {
			result = append(result, scope)
		}
	}
	return result
}
