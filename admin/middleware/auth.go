package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	bootpkg "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	storagecache "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	runtimechallenge "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/challenge"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/11 22:03:02
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/11 22:03:02
 */

var (
	Auth     *jwt.GinJWTMiddleware
	Verifier security.Verifier
)

const (
	authenticationFailureKey      = "mss.authentication.failure"
	authorizationPolicyFailureKey = "mss.authorization.policy.failure"
	challengeUnavailableKey       = "mss.authentication.challenge-unavailable"
	publicLoginDisallowOAuthKey   = "mss.authentication.public-login-disallow-oauth"
)

var ensureAuthorizationPolicyCurrent = func(c *gin.Context) error {
	return service.AuthorizationPolicies.EnsureCurrent(
		c,
		center.Default.GetDB(c, &models.ConfigRevision{}),
	)
}

func Init() error {
	Auth = &jwt.GinJWTMiddleware{
		Realm:       config.Cfg.Auth.Realm,
		Key:         []byte(config.Cfg.Auth.Key),
		Timeout:     config.Cfg.Auth.Timeout,
		MaxRefresh:  config.Cfg.Auth.MaxRefresh,
		IdentityKey: config.Cfg.Auth.IdentityKey,
		PayloadFunc: func(data any) jwt.MapClaims {
			if v, ok := data.(security.Verifier); ok {
				claims := principalClaims(v)
				if v.GetRefreshTokenDisable() {
					return claims
				}
				bag := loginContext.Load()
				loginContext.Clear()
				if bag.sid == "" {
					// AuthenticateVerifier always creates the durable V6 session.
					// A missing sid means the handoff into TokenGenerator failed.
					slog.Error("session sid missing in payload")
					return jwt.MapClaims{}
				}
				claims["sid"] = bag.sid
				return claims
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) any {
			claims := jwt.ExtractClaims(c)
			if personAccessToken, ok := claims["personAccessToken"]; ok && cast.ToString(personAccessToken) != "" {
				userID, roleID, err := principalSnapshotFromClaims(claims)
				if err != nil {
					markAuthenticationFailure(c)
					return nil
				}
				verifier := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
				return authenticatePersonalAccessToken(
					c,
					verifier,
					cast.ToString(personAccessToken),
					userID,
					roleID,
				)
			}
			principal, err := currentPrincipalFromClaims(c, claims)
			if err != nil {
				markAuthenticationFailure(c)
				return nil
			}
			if !validateSessionFromClaims(c, claims, principal) {
				markAuthenticationFailure(c)
				return nil
			}
			return principal
		},
		Authenticator:   authenticateLoginRequest,
		Authorizator:    authorizeCurrentPolicyRequest,
		LoginResponse:   rejectGenericAuthResponse,
		RefreshResponse: rejectGenericAuthResponse,
		Unauthorized:    writeUnauthorizedAuthResponse,
		// The application accepts only explicit API Authorization headers and,
		// when enabled, the fixed V6 browser-session cookie.
		TokenLookup: authenticationTokenLookup(),

		// TokenHeadName is a string in the header. Default value is "Bearer"
		TokenHeadName: "Bearer",

		// TimeFunc provides the current time. You can override it to use another time value.
		//This is useful for testing or if your server uses a different time zone than your tokens.
		TimeFunc: time.Now,
	}
	err := Auth.MiddlewareInit()
	if err != nil {
		Auth = nil
		return fmt.Errorf("initialize Admin authentication middleware: %w", err)
	}
	response.AuthHandler = Auth.MiddlewareFunc()
	response.VerifyHandler = GetVerify
	Middlewares.Store("auth", Auth.MiddlewareFunc())
	return nil
}

func writeUnauthorizedAuthResponse(c *gin.Context, code int, message string) {
	if c.GetBool(challengeUnavailableKey) {
		writeAuthErrorResponse(
			c,
			http.StatusServiceUnavailable,
			http.StatusServiceUnavailable,
			"authentication challenge is temporarily unavailable",
		)
		return
	}
	if c.GetBool(authorizationPolicyFailureKey) {
		writeAuthErrorResponse(
			c,
			http.StatusServiceUnavailable,
			http.StatusServiceUnavailable,
			"authorization policy is temporarily unavailable",
		)
		return
	}
	if code == http.StatusForbidden && c.GetBool(authenticationFailureKey) {
		code = http.StatusUnauthorized
	}
	writeAuthErrorResponse(c, code, code, message)
}

// rejectGenericAuthResponse prevents future route registrations from exposing
// gin-jwt's token-returning login or refresh response. Browser authentication
// is owned exclusively by BrowserSessionLoginHandler and
// BrowserSessionRefreshHandler; API automation uses PAT Bearer credentials.
func rejectGenericAuthResponse(c *gin.Context, _ int, _ string, _ time.Time) {
	writeAuthErrorResponse(c, http.StatusNotFound, http.StatusNotFound, "route not found")
}

func authenticateLoginRequest(c *gin.Context) (any, error) {
	loginVals := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
	if err := c.ShouldBind(&loginVals); err != nil {
		return "", jwt.ErrMissingLoginValues
	}
	if c.GetBool(publicLoginDisallowOAuthKey) && isDisallowedPublicOAuthVerifier(loginVals) {
		return "", jwt.ErrFailedAuthentication
	}
	return AuthenticateVerifier(c, loginVals)
}

// ClearAuthCookie expires the V6 browser session cookies without writing a
// response. Session-aware logout handlers use it after revoking the
// server-side session.
func ClearAuthCookie(c *gin.Context) {
	if c == nil {
		return
	}
	ClearBrowserSessionCookies(c)
}

// AuthenticateVerifier executes the canonical credential verification,
// session creation, and login-audit path for a caller-supplied login verifier.
// Callers must generate the JWT in the same goroutine so PayloadFunc can
// consume the newly-created V6 session identifier.
func AuthenticateVerifier(c *gin.Context, loginVals security.Verifier) (security.Verifier, error) {
	if c == nil || loginVals == nil {
		return nil, jwt.ErrMissingLoginValues
	}
	loginContext.Clear()
	api := response.Make(c)
	ok, user, err := loginVals.Verify(c)
	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	db := center.Default.GetDB(c, nil)

	if err != nil {
		if errors.Is(err, storagecache.ErrChallengeUnavailable) || errors.Is(err, runtimechallenge.ErrUnavailable) {
			c.Set(challengeUnavailableKey, true)
		}
		// Authentication providers may include credentials or upstream response
		// details in errors. Keep the durable audit outcome diagnosable without
		// persisting those details.
		if logErr := service.Audit.LogLogin(db, "", extractLoginUsername(loginVals), ip, userAgent, "authentication failed", false); logErr != nil {
			api.AddError(logErr).Log.Warn("write login log failed")
		}
		return nil, err
	}
	if !ok || user == nil {
		if logErr := service.Audit.LogLogin(db, "", extractLoginUsername(loginVals), ip, userAgent, "authentication failed", false); logErr != nil {
			api.AddError(logErr).Log.Warn("write login log failed")
		}
		return nil, jwt.ErrFailedAuthentication
	}
	currentUser, principalErr := loadCurrentPrincipal(c, user.GetUserID(), user.GetRoleID())
	if principalErr != nil {
		if logErr := service.Audit.LogLogin(
			db,
			user.GetUserID(),
			user.GetUsername(),
			ip,
			userAgent,
			"authentication identity unavailable",
			false,
		); logErr != nil {
			api.AddError(logErr).Log.Warn("write login log failed")
		}
		return nil, jwt.ErrFailedAuthentication
	}
	user = currentUser
	sid, sessionErr := service.Session.Create(c, center.Default.GetDB(c, &models.UserSession{}), service.CreateSessionInput{
		UserID:    user.GetUserID(),
		Username:  user.GetUsername(),
		RoleID:    user.GetRoleID(),
		IP:        ip,
		UserAgent: userAgent,
		TTL:       config.Cfg.Auth.Timeout,
	})
	if sessionErr != nil {
		api.AddError(sessionErr).Log.Error("session create failed")
		if logErr := service.Audit.LogLogin(db, user.GetUserID(), user.GetUsername(), ip, userAgent,
			"session creation failed", false); logErr != nil {
			api.AddError(logErr).Log.Warn("write login log failed")
		}
		return nil, jwt.ErrFailedAuthentication
	}
	loginContext.Store(sid)
	if logErr := service.Audit.LogLogin(db, user.GetUserID(), user.GetUsername(), ip, userAgent, "login success", true); logErr != nil {
		api.AddError(logErr).Log.Warn("write login log failed")
	}
	return user, nil
}

func clearVerifierSecrets(verifier security.Verifier) {
	if user, ok := verifier.(*models.User); ok && user != nil {
		user.Password = ""
		user.Captcha = ""
	}
}

func principalClaims(verifier security.Verifier) jwt.MapClaims {
	if verifier == nil {
		return jwt.MapClaims{}
	}
	clearVerifierSecrets(verifier)
	return jwt.MapClaims{
		"uid":                  verifier.GetUserID(),
		"rid":                  verifier.GetRoleID(),
		"refreshTokenDisabled": verifier.GetRefreshTokenDisable(),
		"personAccessToken":    verifier.GetPersonAccessToken(),
	}
}

func principalSnapshotFromClaims(claims jwt.MapClaims) (string, string, error) {
	userID := strings.TrimSpace(cast.ToString(claims["uid"]))
	roleID := strings.TrimSpace(cast.ToString(claims["rid"]))
	if userID == "" && roleID == "" {
		return "", "", errors.New("identity snapshot is missing")
	}
	if userID == "" || roleID == "" {
		return "", "", errors.New("identity snapshot is incomplete")
	}
	return userID, roleID, nil
}

func loadCurrentPrincipal(c *gin.Context, userID, expectedRoleID string) (security.Verifier, error) {
	if c == nil {
		return nil, errors.New("request context is missing")
	}
	user, err := models.LoadCurrentUserPrincipal(
		c,
		center.Default.GetDB(c, &models.User{}),
		userID,
	)
	if err != nil {
		return nil, errors.New("current identity is unavailable")
	}
	if expectedRoleID == "" || user.GetRoleID() != expectedRoleID {
		return nil, errors.New("current identity role changed")
	}
	return user, nil
}

func currentPrincipalFromClaims(c *gin.Context, claims jwt.MapClaims) (security.Verifier, error) {
	userID, roleID, err := principalSnapshotFromClaims(claims)
	if err != nil {
		return nil, err
	}
	return loadCurrentPrincipal(c, userID, roleID)
}

func isDisallowedPublicOAuthVerifier(verifier security.Verifier) bool {
	user, ok := verifier.(*models.User)
	return ok && user != nil &&
		(user.Provider == pkg.GithubLoginProvider || user.Provider == pkg.LarkLoginProvider)
}

// AuthenticateAndGenerateToken completes the canonical login path and returns
// the Admin JWT without invoking LoginResponse. It is used by server-completed
// OAuth callbacks, which have their own explicit response DTO.
func AuthenticateAndGenerateToken(
	c *gin.Context,
	loginVals security.Verifier,
) (string, time.Time, error) {
	if Auth == nil {
		return "", time.Time{}, errors.New("authentication middleware is not initialized")
	}
	principal, err := AuthenticateVerifier(c, loginVals)
	if err != nil {
		return "", time.Time{}, err
	}
	token, expiresAt, err := Auth.TokenGenerator(principal)
	if err != nil {
		loginContext.Clear()
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func authenticatePersonalAccessToken(
	c *gin.Context,
	verifier security.Verifier,
	tokenID string,
	expectedUserID string,
	expectedRoleID string,
) security.Verifier {
	verifier.SetRefreshTokenDisable(true)
	verifier.SetPersonAccessToken(tokenID)
	if err := verifier.CheckToken(c, jwt.GetToken(c)); err != nil {
		markAuthenticationFailure(c)
		return nil
	}
	if verifier.GetUserID() != expectedUserID || verifier.GetRoleID() != expectedRoleID {
		markAuthenticationFailure(c)
		return nil
	}
	return verifier
}

// GetVerify 获取当前登录用户
func GetVerify(ctx *gin.Context) security.Verifier {
	if ctx == nil {
		return nil
	}
	identity, ok := ctx.Get(config.Cfg.Auth.IdentityKey)
	if !ok {
		return nil
	}
	verifier, ok := identity.(security.Verifier)
	if !ok {
		return nil
	}
	return verifier
}

// OptionalAuth preserves anonymous access while validating any credential the
// client chooses to send. Invalid, expired, or revoked credentials fail closed
// instead of silently degrading to an anonymous request.
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requestHasCredential(c) {
			c.Next()
			return
		}
		if Auth == nil {
			writeAuthErrorResponse(c, http.StatusInternalServerError, http.StatusInternalServerError, "authentication middleware is not initialized")
			return
		}
		Auth.MiddlewareFunc()(c)
	}
}

func requestHasCredential(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if strings.TrimSpace(c.GetHeader("Authorization")) != "" {
		return true
	}
	if cookie, err := c.Cookie(BrowserSessionCookieName); err == nil && strings.TrimSpace(cookie) != "" {
		return true
	}
	return false
}

func authenticationTokenLookup() string {
	return "header: Authorization, cookie: " + BrowserSessionCookieName
}

func markAuthenticationFailure(c *gin.Context) {
	if c != nil {
		c.Set(authenticationFailureKey, true)
	}
}

func authorizationObject(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if route := c.FullPath(); route != "" {
		return route
	}
	if c.Request == nil || c.Request.URL == nil {
		return ""
	}
	return c.Request.URL.Path
}

func isSelfServiceRequest(method, path string) bool {
	_, ok := selfServiceRequests[method+" "+path]
	return ok
}

// IsPersonalAccessTokenVerifier reports whether the authenticated identity was
// established from a non-refreshable personal access token. Personal access
// tokens are suitable for API automation, but must not be accepted as proof of
// an interactive user session for sensitive account recovery operations.
func IsPersonalAccessTokenVerifier(verifier security.Verifier) bool {
	return verifier != nil &&
		(verifier.GetPersonAccessToken() != "" || verifier.GetRefreshTokenDisable())
}

// CurrentSessionID returns the durable server-session identifier carried by
// the authenticated JWT. Sensitive self-service fails closed when a credential
// has no sid instead of treating an API token as step-up proof.
func CurrentSessionID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(cast.ToString(jwt.ExtractClaims(c)["sid"]))
}

func isInteractiveSensitiveRequest(method, path string) bool {
	switch method + " " + path {
	case http.MethodPost + " /admin/api/user/reset-password",
		http.MethodGet + " /admin/api/user/security",
		http.MethodPost + " /admin/api/user/security/reauthenticate",
		http.MethodPut + " /admin/api/user/security/password",
		http.MethodDelete + " /admin/api/user/oauth2/:provider",
		http.MethodPut + " /admin/api/user/userInfo",
		http.MethodPost + " /admin/api/user/session/oauth2/authorize",
		http.MethodPost + " /admin/api/user/session/:provider/callback",
		http.MethodPost + " /admin/api/ws/tickets",
		http.MethodGet + " /admin/api/user-auth-tokens",
		http.MethodPost + " /admin/api/user-auth-tokens",
		http.MethodPut + " /admin/api/user-auth-token/:id/revoke",
		http.MethodPut + " /admin/api/user-auth-token/:id/refresh":
		return true
	default:
		return false
	}
}

var selfServiceRequests = map[string]struct{}{
	http.MethodGet + " /admin/api/menu/authorize":                   {},
	http.MethodGet + " /admin/api/notice/unread":                    {},
	http.MethodGet + " /admin/api/notice/read/:id":                  {},
	http.MethodPut + " /admin/api/notice/read/:id":                  {},
	http.MethodGet + " /admin/api/user-configs/:group":              {},
	http.MethodPut + " /admin/api/user-configs/:group":              {},
	http.MethodDelete + " /admin/api/user-configs/theme":            {},
	http.MethodGet + " /admin/api/user-configs/profile":             {},
	http.MethodGet + " /admin/api/app-configs/profile":              {},
	http.MethodGet + " /admin/api/user-auth-tokens":                 {},
	http.MethodPost + " /admin/api/user-auth-tokens":                {},
	http.MethodPut + " /admin/api/user-auth-token/:id/revoke":       {},
	http.MethodPut + " /admin/api/user-auth-token/:id/refresh":      {},
	http.MethodPost + " /admin/api/user/reset-password":             {},
	http.MethodGet + " /admin/api/user/security":                    {},
	http.MethodPost + " /admin/api/user/security/reauthenticate":    {},
	http.MethodPut + " /admin/api/user/security/password":           {},
	http.MethodDelete + " /admin/api/user/oauth2/:provider":         {},
	http.MethodGet + " /admin/api/user/userInfo":                    {},
	http.MethodPut + " /admin/api/user/userInfo":                    {},
	http.MethodPost + " /admin/api/user/avatar":                     {},
	http.MethodGet + " /admin/api/user/oauth2":                      {},
	http.MethodPost + " /admin/api/user/session/oauth2/authorize":   {},
	http.MethodPost + " /admin/api/user/session/:provider/callback": {},
	http.MethodPost + " /admin/api/online-sessions/logout":          {},
	http.MethodPost + " /admin/api/ws/tickets":                      {},
	http.MethodGet + " /admin/api/presentation/effective/:pageKey":  {},
}

func authorizeRequest(data any, c *gin.Context) bool {
	verifier, ok := data.(security.Verifier)
	if !ok || verifier == nil || c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	object := authorizationObject(c)
	if IsPersonalAccessTokenVerifier(verifier) &&
		(isInteractiveSensitiveRequest(c.Request.Method, object) ||
			isInteractiveSensitiveRequest(c.Request.Method, c.Request.URL.Path)) {
		return false
	}
	if verifier.Root() {
		return true
	}
	if isSelfServiceRequest(c.Request.Method, object) ||
		isSelfServiceRequest(c.Request.Method, c.Request.URL.Path) {
		return true
	}
	if gormdb.Enforcer == nil || object == "" {
		return false
	}
	enabled, err := gormdb.Enforcer.Enforce(verifier.GetRoleID(), pkg.APIAccessType.String(), object, c.Request.Method)
	if err != nil {
		response.Make(c).AddError(err).Log.Error("Enforcer.Enforce error")
		return false
	}
	if !enabled {
		return false
	}

	// The shipped Casbin model uses keyMatch. A legacy policy such as
	// /user/*/password-reset therefore also matches every suffix after /user/.
	// Require a second, segment-exact match against the loaded policy so a path
	// parameter consumes one segment only while policy reloads still take effect.
	policies, err := gormdb.Enforcer.GetFilteredPolicy(0, verifier.GetRoleID(), pkg.APIAccessType.String())
	if err != nil {
		response.Make(c).AddError(err).Log.Error("Enforcer.GetFilteredPolicy error")
		return false
	}
	for _, policy := range policies {
		if len(policy) < 4 || !routePolicyMatches(object, policy[2]) {
			continue
		}
		methodPattern, compileErr := regexp.Compile("^(?:" + policy[3] + ")$")
		if compileErr != nil {
			response.Make(c).AddError(compileErr).Log.Error("invalid Casbin method policy")
			continue
		}
		if methodPattern.MatchString(c.Request.Method) {
			return true
		}
	}
	return false
}

// authorizeCurrentPolicyRequest reconciles the durable policy revision before
// a request is evaluated by Casbin. Root and exact self-service capabilities
// do not consult Casbin, so they remain available without performing a
// redundant policy reload. A reconciliation failure is a service outage, not
// a user permission denial, and is surfaced as 503 by Unauthorized.
func authorizeCurrentPolicyRequest(data any, c *gin.Context) bool {
	if requestRequiresCasbinPolicy(data, c) {
		if err := ensureAuthorizationPolicyCurrent(c); err != nil {
			c.Set(authorizationPolicyFailureKey, true)
			response.Make(c).AddError(err).Log.Error("reconcile authorization policy")
			return false
		}
	}
	return authorizeRequest(data, c)
}

func requestRequiresCasbinPolicy(data any, c *gin.Context) bool {
	verifier, ok := data.(security.Verifier)
	if !ok || verifier == nil || c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	object := authorizationObject(c)
	if IsPersonalAccessTokenVerifier(verifier) &&
		(isInteractiveSensitiveRequest(c.Request.Method, object) ||
			isInteractiveSensitiveRequest(c.Request.Method, c.Request.URL.Path)) {
		return false
	}
	if verifier.Root() {
		return false
	}
	return !isSelfServiceRequest(c.Request.Method, object) &&
		!isSelfServiceRequest(c.Request.Method, c.Request.URL.Path)
}

func routePolicyMatches(route, policy string) bool {
	routeSegments := strings.Split(strings.Trim(route, "/"), "/")
	policySegments := strings.Split(strings.Trim(policy, "/"), "/")
	if len(routeSegments) != len(policySegments) {
		return false
	}
	for index := range policySegments {
		policySegment := policySegments[index]
		if policySegment == "*" || strings.HasPrefix(policySegment, ":") || strings.HasPrefix(policySegment, "*") {
			continue
		}
		if routeSegments[index] != policySegment {
			return false
		}
	}
	return true
}

func extractLoginUsername(loginVals any) string {
	if loginVals == nil {
		return ""
	}
	if v, ok := loginVals.(interface{ GetUsername() string }); ok {
		return v.GetUsername()
	}
	if v, ok := loginVals.(security.Verifier); ok {
		return v.GetUsername()
	}
	return ""
}

func writeAuthErrorResponse(c *gin.Context, httpStatus, businessCode int, message string) {
	if c == nil {
		return
	}
	if message == "" {
		message = http.StatusText(httpStatus)
		if message == "" {
			message = errors.New("request error").Error()
		}
	}
	c.AbortWithStatusJSON(httpStatus, gin.H{
		"code":         businessCode,
		"status":       "error",
		"msg":          message,
		"errorMessage": message,
		"traceId":      bootpkg.GenerateMsgIDFromContext(c),
	})
}

// validateSessionFromClaims checks that the JWT `sid` claim points to an
// active server-side session owned by the same current user and role. Returns
// false when the auth layer must reject the request (missing sid,
// missing/revoked/expired session, identity mismatch, or unrecoverable DB
// error). On success it kicks off a throttled async last_seen update.
//
// Extracted from middleware.Init so integration tests can exercise active,
// missing-sid, DB-revoked, DB-expired, and identity-mismatch branches.
func validateSessionFromClaims(
	c *gin.Context,
	claims jwt.MapClaims,
	principal security.Verifier,
) bool {
	if principal == nil {
		return false
	}
	sid := cast.ToString(claims["sid"])
	if sid == "" {
		return false
	}
	db := center.Default.GetDB(c, &models.UserSession{})
	res, err := service.Session.Lookup(c, db, sid)
	if err != nil || res.Status != service.LookupActive {
		return false
	}
	if res.Entry.UserID != principal.GetUserID() || res.Entry.RoleID != principal.GetRoleID() {
		return false
	}
	if shouldTouch, terr := service.Session.MarkLastSeen(c, sid); terr == nil && shouldTouch {
		// Capture the request-scoped DB in the request goroutine so the
		// async Touch keeps any tenant scope, and capture the service instance
		// so a runtime/test lifecycle swap cannot redirect an in-flight write.
		// Rebind ctx to a fresh timeout so request cancellation does not abort
		// the bounded bookkeeping update.
		scopedDB := db
		sessionService := service.Session
		sessionTouchGoroutines.Add(1)
		go func(sid string, db *gorm.DB, sessions *service.SessionService) {
			defer sessionTouchGoroutines.Done()
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := sessions.RecordLastSeen(bgCtx, db.WithContext(bgCtx), sid); err != nil {
				slog.Warn("session record last_seen failed", "sid", sid, "err", err)
			}
		}(sid, scopedDB, sessionService)
	}
	return true
}

// sessionTouchGoroutines gives controlled runtime and test shutdown paths a
// way to wait until bounded background bookkeeping no longer owns captured
// database/cache resources. Request handling never waits on this tracker.
var sessionTouchGoroutines sync.WaitGroup

func waitForSessionTouches() {
	sessionTouchGoroutines.Wait()
}

type loginCtxBag struct {
	sid string
}

var loginContextMap sync.Map // goroutine id -> *loginCtxBag

type loginContextHandle struct{}

var loginContext loginContextHandle

func (loginContextHandle) Store(sid string) {
	loginContextMap.Store(goroutineID(), &loginCtxBag{sid: sid})
}

func (loginContextHandle) Load() *loginCtxBag {
	v, _ := loginContextMap.Load(goroutineID())
	if v == nil {
		return &loginCtxBag{}
	}
	return v.(*loginCtxBag)
}

func (loginContextHandle) Clear() {
	loginContextMap.Delete(goroutineID())
}

// goroutineID parses the current goroutine id from runtime.Stack output.
//
// Background: gin-jwt/v2.PayloadFunc receives only the authenticated principal,
// not the *gin.Context. The Authenticator creates the server-side session
// (so failures can short-circuit the login) and stashes the resulting sid in
// loginContextMap keyed by goroutine id; PayloadFunc reads it back to embed
// the sid in the JWT claims.
//
// CAVEAT: The "goroutine N [...]:" prefix is a Go runtime implementation
// detail, not part of the language spec. If a future Go release changes the
// format this parser will silently return zero and PayloadFunc will skip
// session creation. The login regression tests exercise the happy path; bump
// this if Go runtime debug output ever shifts.
func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// "goroutine 12345 [running]:" → 12345
	var id uint64
	for i := len("goroutine "); i < n; i++ {
		if buf[i] < '0' || buf[i] > '9' {
			break
		}
		id = id*10 + uint64(buf[i]-'0')
	}
	return id
}
