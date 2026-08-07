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
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/oauthstate"
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
type oauthCodeExchange func(*gin.Context, pkg.LoginProvider, string) (*dto.OauthToken, error)
type oauthLoginCompleter func(*gin.Context, pkg.LoginProvider, string) (string, time.Time, error)
type oauthBindingCompleter func(*gin.Context, string, pkg.LoginProvider, string) error

var errOAuthIdentityAlreadyBound = errors.New("oauth identity is already bound")

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

// OAuthAuthorize issues a server-owned, browser-bound authorization attempt.
// @Summary Start an OAuth2 authorization attempt
// @Tags user
// @Accept json
// @Produce json
// @Param data body dto.OAuthAuthorizeRequest true "data"
// @Success 201 {object} dto.OAuthAuthorizeResponse
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 422 {object} response.Response
// @Router /admin/api/user/oauth2/authorize [post]
func (e *User) OAuthAuthorize(c *gin.Context) {
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
	credentialFingerprint := requestCredentialFingerprint(c)
	record := oauthstate.Record{Provider: string(req.Provider), Intent: intent}
	switch intent {
	case oauthstate.IntentLogin:
		// Login initiation must be anonymous. A stale or residual identity is
		// rejected instead of silently becoming part of a different flow.
		if verify != nil || credentialFingerprint != "" {
			api.Err(http.StatusConflict)
			return
		}
	case oauthstate.IntentBinding:
		if verify == nil || credentialFingerprint == "" {
			api.Err(http.StatusUnauthorized)
			return
		}
		if middleware.IsPersonalAccessTokenVerifier(verify) {
			api.Err(http.StatusForbidden)
			return
		}
		record.UserID = verify.GetUserID()
		record.CredentialFingerprint = credentialFingerprint
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
	req := &dto.OauthCallbackReq{}
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
	credentialFingerprint := requestCredentialFingerprint(c)
	switch record.Intent {
	case oauthstate.IntentLogin:
		if verify != nil || credentialFingerprint != "" || record.UserID != "" || record.CredentialFingerprint != "" {
			api.Err(http.StatusUnauthorized)
			return
		}
	case oauthstate.IntentBinding:
		if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) ||
			record.UserID == "" || record.UserID != verify.GetUserID() ||
			credentialFingerprint == "" ||
			!constantTimeEqual(credentialFingerprint, record.CredentialFingerprint) {
			api.Err(http.StatusUnauthorized)
			return
		}
	}

	providerToken, err := e.codeExchange()(c, req.Provider, req.Code)
	if err != nil || providerToken == nil || strings.TrimSpace(providerToken.AccessToken) == "" {
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
		token, expiresAt, completeErr := e.loginCompleter()(c, req.Provider, providerToken.AccessToken)
		if completeErr != nil || token == "" || expiresAt.IsZero() {
			api.Err(http.StatusUnauthorized)
			return
		}
		result.Token = token
		result.Expire = &expiresAt
	case oauthstate.IntentBinding:
		completeErr := e.bindingCompleter()(c, record.UserID, req.Provider, providerToken.AccessToken)
		if errors.Is(completeErr, errOAuthIdentityAlreadyBound) {
			api.Err(http.StatusConflict)
			return
		}
		if completeErr != nil {
			api.Log.Error("oauth binding completion failed")
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
	user := &models.User{}
	if err := center.GetDB(c, user).Where("id = ?", userID).First(user).Error; err != nil {
		return err
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
		return errors.New("oauth provider is unsupported")
	}
	if err != nil {
		return err
	}
	if identity == nil {
		return errors.New("oauth identity is missing")
	}
	return persistOAuthBinding(c, userID, identity)
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

func requestCredentialFingerprint(c *gin.Context) string {
	credential := requestCredential(c)
	if credential == "" {
		return ""
	}
	return oauthstate.Digest(credential)
}

func requestCredential(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	authorization := strings.TrimSpace(c.GetHeader("Authorization"))
	if authorization != "" {
		parts := strings.Fields(authorization)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
		return authorization
	}
	if token := strings.TrimSpace(c.Query("token")); token != "" {
		return token
	}
	if token, err := c.Cookie("jwt"); err == nil {
		return strings.TrimSpace(token)
	}
	return ""
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
		Secure:   requestIsHTTPS(c),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOAuthBrowserCookie(c *gin.Context, state string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthBrowserCookieName(state),
		Path:     "/admin/api/user/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   requestIsHTTPS(c),
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
		clientID, err := requiredAppConfig(c, "security:githubClientId")
		if err != nil {
			return "", err
		}
		redirectURL, err := requiredAppConfig(c,
			"security:githubRedirectURI",
			"security:githubRedirectUrl",
			"security:githubRedirectURL",
		)
		if err != nil {
			return "", err
		}
		scope, _ := appConfigAny(c, "security:githubScope")
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
		appID, err := requiredAppConfig(c, "security:larkAppId")
		if err != nil {
			return "", err
		}
		redirectURI, err := requiredAppConfig(c, "security:larkRedirectURI", "security:larkRedirectUrl")
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

func exchangeOAuthCode(c *gin.Context, provider pkg.LoginProvider, code string) (*dto.OauthToken, error) {
	switch provider {
	case pkg.GithubLoginProvider:
		clientID, err := requiredAppConfig(c, "security:githubClientId")
		if err != nil {
			return nil, err
		}
		clientSecret, err := requiredAppConfig(c, "security:githubClientSecret")
		if err != nil {
			return nil, err
		}
		redirectURL, err := requiredAppConfig(c,
			"security:githubRedirectURI",
			"security:githubRedirectUrl",
			"security:githubRedirectURL",
		)
		if err != nil {
			return nil, err
		}
		scope, _ := appConfigAny(c, "security:githubScope")
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
			return nil, err
		}
		result := &dto.OauthToken{AccessToken: token.AccessToken, TokenType: token.TokenType, RefreshToken: token.RefreshToken}
		if !token.Expiry.IsZero() {
			result.Expiry = &token.Expiry
		}
		return result, nil
	case pkg.LarkLoginProvider:
		appID, err := requiredAppConfig(c, "security:larkAppId")
		if err != nil {
			return nil, err
		}
		appSecret, err := requiredAppConfig(c, "security:larkAppSecret")
		if err != nil {
			return nil, err
		}
		client := lark.NewClient(appID, appSecret)
		request := larkauthen.NewCreateAccessTokenReqBuilder().
			Body(larkauthen.NewCreateAccessTokenReqBodyBuilder().
				GrantType("authorization_code").
				Code(code).
				Build()).Build()
		providerResponse, err := client.Authen.AccessToken.Create(c, request)
		if err != nil || !providerResponse.Success() || providerResponse.Data == nil ||
			providerResponse.Data.AccessToken == nil || providerResponse.Data.TokenType == nil ||
			providerResponse.Data.RefreshToken == nil || providerResponse.Data.ExpiresIn == nil ||
			providerResponse.Data.RefreshExpiresIn == nil {
			if err != nil {
				return nil, err
			}
			return nil, errors.New("lark oauth exchange failed")
		}
		expiry := time.Now().Add(time.Duration(*providerResponse.Data.ExpiresIn) * time.Second)
		refreshExpiry := time.Now().Add(time.Duration(*providerResponse.Data.RefreshExpiresIn) * time.Second)
		return &dto.OauthToken{
			AccessToken:   *providerResponse.Data.AccessToken,
			TokenType:     *providerResponse.Data.TokenType,
			RefreshToken:  *providerResponse.Data.RefreshToken,
			Expiry:        &expiry,
			RefreshExpiry: &refreshExpiry,
		}, nil
	default:
		return nil, errors.New("unsupported oauth provider")
	}
}

func requiredAppConfig(c *gin.Context, keys ...string) (string, error) {
	if value, ok := appConfigAny(c, keys...); ok && strings.TrimSpace(value) != "" {
		return value, nil
	}
	return "", fmt.Errorf("required oauth configuration is missing")
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
