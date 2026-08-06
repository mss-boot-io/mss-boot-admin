package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
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
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
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

const authenticationFailureKey = "mss.authentication.failure"

func Init() {
	Auth = &jwt.GinJWTMiddleware{
		Realm:       config.Cfg.Auth.Realm,
		Key:         []byte(config.Cfg.Auth.Key),
		Timeout:     config.Cfg.Auth.Timeout,
		MaxRefresh:  config.Cfg.Auth.MaxRefresh,
		IdentityKey: config.Cfg.Auth.IdentityKey,
		PayloadFunc: func(data any) jwt.MapClaims {

			if v, ok := data.(security.Verifier); ok {
				if v.GetRefreshTokenDisable() {
					return jwt.MapClaims{
						"refreshTokenDisabled": v.GetRefreshTokenDisable(),
						"personAccessToken":    v.GetPersonAccessToken(),
					}
				}
				rb, _ := json.Marshal(v)
				claims := jwt.MapClaims{
					"verifier":             string(rb),
					"refreshTokenDisabled": false,
					"personAccessToken":    "",
				}
				if config.Cfg.Auth.SessionEnabled {
					bag := loginContext.Load()
					loginContext.Clear()
					if bag.sid == "" {
						// Authenticator 已经在 SessionEnabled 路径强制创建 session；
						// 落到这里只可能是 sid 在 store→load 之间被冲掉，属于异常。
						slog.Error("session sid missing in payload")
						return jwt.MapClaims{}
					}
					claims["sid"] = bag.sid
				}
				return claims
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) any {
			claims := jwt.ExtractClaims(c)
			verifier := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
			if personAccessToken, ok := claims["personAccessToken"]; ok && cast.ToString(personAccessToken) != "" {
				return authenticatePersonalAccessToken(c, verifier, cast.ToString(personAccessToken))
			}
			err := json.Unmarshal([]byte(cast.ToString(claims["verifier"])), verifier)
			if err != nil {
				markAuthenticationFailure(c)
				return nil
			}
			if verifier.GetRefreshTokenDisable() {
				// check token revoked
				token := jwt.GetToken(c)
				err = verifier.CheckToken(c, token)
				if err != nil {
					markAuthenticationFailure(c)
					return nil
				}
				return verifier
			}
			if config.Cfg.Auth.SessionEnabled {
				if !validateSessionFromClaims(c, claims) {
					markAuthenticationFailure(c)
					return nil
				}
			}
			return verifier
		},
		Authenticator: func(c *gin.Context) (any, error) {
			loginVals := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			return AuthenticateVerifier(c, loginVals)
		},
		Authorizator: authorizeRequest,
		RefreshResponse: func(c *gin.Context, code int, token string, expire time.Time) {
			jwtToken, err := Auth.ParseTokenString(token)
			if err != nil {
				writeAuthErrorResponse(c, http.StatusOK, http.StatusUnauthorized, "refresh token error")
				return
			}
			claims := jwt.ExtractClaimsFromToken(jwtToken)
			if len(claims) == 0 {
				writeAuthErrorResponse(c, http.StatusOK, http.StatusUnauthorized, "refresh token error")
				return
			}
			if cast.ToBool(claims["refreshTokenDisabled"]) {
				writeAuthErrorResponse(c, http.StatusOK, http.StatusUnauthorized, "refresh token disabled")
				return
			}
			verifier := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
			err = json.Unmarshal([]byte(cast.ToString(claims["verifier"])), verifier)
			if err != nil {
				writeAuthErrorResponse(c, http.StatusOK, http.StatusUnauthorized, "refresh token error")
				return
			}
			if err := validateRefreshVerifier(c, verifier); err != nil {
				writeAuthErrorResponse(c, http.StatusOK, http.StatusUnauthorized, "refresh token error")
				return
			}
			if config.Cfg.Auth.SessionEnabled {
				sid := cast.ToString(claims["sid"])
				if sid == "" {
					writeAuthErrorResponse(c, http.StatusOK, http.StatusUnauthorized, "session missing")
					return
				}
				db := center.Default.GetDB(c, &models.UserSession{})
				res, lerr := service.Session.Lookup(c, db, sid)
				if lerr != nil || res.Status != service.LookupActive {
					writeAuthErrorResponse(c, http.StatusOK, http.StatusUnauthorized, "session revoked")
					return
				}
			}
			//todo 重新颁发token
			c.JSON(http.StatusOK, gin.H{
				"code":   http.StatusOK,
				"token":  token,
				"expire": expire.Format(time.RFC3339),
			})
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			if code == http.StatusForbidden && c.GetBool(authenticationFailureKey) {
				code = http.StatusUnauthorized
			}
			writeAuthErrorResponse(c, code, code, message)
		},
		// TokenLookup is a string in the form of "<source>:<name>" that is used
		// to extract token from the request.
		// Optional. Default value "header:Authorization".
		// Possible values:
		// - "header:<name>"
		// - "query:<name>"
		// - "cookie:<name>"
		// - "param:<name>"
		TokenLookup: "header: Authorization, query: token, cookie: jwt",
		// TokenLookup: "query:token",
		// TokenLookup: "cookie:token",

		// TokenHeadName is a string in the header. Default value is "Bearer"
		TokenHeadName: "Bearer",

		// TimeFunc provides the current time. You can override it to use another time value.
		//This is useful for testing or if your server uses a different time zone than your tokens.
		TimeFunc: time.Now,
	}
	err := Auth.MiddlewareInit()
	if err != nil {
		slog.Error("authMiddleware.MiddlewareInit() Error", "err", err)
		os.Exit(-1)
	}
	response.AuthHandler = Auth.MiddlewareFunc()
	response.VerifyHandler = GetVerify
	Middlewares.Store("auth", Auth.MiddlewareFunc())
}

// AuthenticateVerifier executes the canonical credential verification,
// session creation, and login-audit path for a caller-supplied login verifier.
// Callers that enable sessions must generate the JWT in the same goroutine so
// PayloadFunc can consume the newly-created session identifier.
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
	if config.Cfg.Auth.SessionEnabled {
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
	}
	if logErr := service.Audit.LogLogin(db, user.GetUserID(), user.GetUsername(), ip, userAgent, "login success", true); logErr != nil {
		api.AddError(logErr).Log.Warn("write login log failed")
	}
	return user, nil
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
	Auth.SetCookie(c, token)
	return token, expiresAt, nil
}

func authenticatePersonalAccessToken(c *gin.Context, verifier security.Verifier, tokenID string) security.Verifier {
	verifier.SetRefreshTokenDisable(true)
	verifier.SetPersonAccessToken(tokenID)
	if err := verifier.CheckToken(c, jwt.GetToken(c)); err != nil {
		markAuthenticationFailure(c)
		return nil
	}
	return verifier
}

// GetVerify 获取当前登录用户
// validateRefreshVerifier reloads the signed principal from the authoritative
// database. Refresh must never reuse UserLogin.Verify: that method accepts
// public login input, while this path may trust identity fields only after the
// refresh token has been cryptographically validated by gin-jwt.
func validateRefreshVerifier(c *gin.Context, verifier security.Verifier) error {
	if c == nil || verifier == nil {
		return errors.New("refresh identity is missing")
	}
	userID := strings.TrimSpace(verifier.GetUserID())
	if userID == "" {
		return errors.New("refresh identity is missing")
	}
	var user models.User
	if err := center.GetDB(c, &models.User{}).Preload("Role").First(&user, "id = ?", userID).Error; err != nil {
		return errors.New("refresh identity is invalid")
	}
	if user.Status != enum.Enabled || user.Role == nil || user.Role.Status != enum.Enabled {
		return errors.New("refresh identity is disabled")
	}
	if user.RoleID == "" || user.RoleID != verifier.GetRoleID() {
		return errors.New("refresh identity role changed")
	}
	return nil
}

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
	if strings.TrimSpace(c.GetHeader("Authorization")) != "" || strings.TrimSpace(c.Query("token")) != "" {
		return true
	}
	cookie, err := c.Cookie("jwt")
	return err == nil && strings.TrimSpace(cookie) != ""
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

func isInteractiveSensitiveRequest(method, path string) bool {
	switch method + " " + path {
	case http.MethodPost + " /admin/api/user/reset-password",
		http.MethodPut + " /admin/api/user/userInfo",
		http.MethodPost + " /admin/api/user/oauth2/authorize",
		http.MethodPost + " /admin/api/user/:provider/callback",
		http.MethodPost + " /admin/api/user/binding",
		http.MethodDelete + " /admin/api/user/unbinding",
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
	http.MethodGet + " /admin/api/menu/authorize":              {},
	http.MethodGet + " /admin/api/notice/unread":               {},
	http.MethodGet + " /admin/api/notice/read/:id":             {},
	http.MethodPut + " /admin/api/notice/read/:id":             {},
	http.MethodGet + " /admin/api/user-configs/:group":         {},
	http.MethodPut + " /admin/api/user-configs/:group":         {},
	http.MethodGet + " /admin/api/user-configs/profile":        {},
	http.MethodGet + " /admin/api/app-configs/profile":         {},
	http.MethodGet + " /admin/api/app-configs/theme":           {},
	http.MethodGet + " /admin/api/user-auth-tokens":            {},
	http.MethodPost + " /admin/api/user-auth-tokens":           {},
	http.MethodPut + " /admin/api/user-auth-token/:id/revoke":  {},
	http.MethodPut + " /admin/api/user-auth-token/:id/refresh": {},
	http.MethodPost + " /admin/api/user/reset-password":        {},
	http.MethodGet + " /admin/api/user/userInfo":               {},
	http.MethodPut + " /admin/api/user/userInfo":               {},
	http.MethodPost + " /admin/api/user/avatar":                {},
	http.MethodGet + " /admin/api/user/oauth2":                 {},
	http.MethodPost + " /admin/api/user/oauth2/authorize":      {},
	http.MethodPost + " /admin/api/user/:provider/callback":    {},
	http.MethodPost + " /admin/api/user/binding":               {},
	http.MethodDelete + " /admin/api/user/unbinding":           {},
	http.MethodPost + " /admin/api/online-sessions/logout":     {},
	http.MethodGet + " /admin/api/ws/connect":                  {},
	http.MethodPost + " /admin/api/storage/upload":             {},
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
// active server-side session. Returns false when the auth layer must reject
// the request (missing sid, missing/revoked/expired session, or unrecoverable
// DB error). On success it kicks off a throttled async last_seen update.
//
// Extracted from middleware.Init so integration tests can exercise the four
// branches the reviewer flagged (PR #376 review #5): sid present + active,
// missing-sid legacy JWT, DB-revoked session, and DB-expired session.
func validateSessionFromClaims(c *gin.Context, claims jwt.MapClaims) bool {
	sid := cast.ToString(claims["sid"])
	if sid == "" {
		return false
	}
	db := center.Default.GetDB(c, &models.UserSession{})
	res, err := service.Session.Lookup(c, db, sid)
	if err != nil || res.Status != service.LookupActive {
		return false
	}
	if shouldTouch, terr := service.Session.MarkLastSeen(c, sid); terr == nil && shouldTouch {
		// Capture the request-scoped DB in the request goroutine so the
		// async Touch keeps any tenant scope; rebind ctx to a fresh timeout
		// so it doesn't get cancelled when the request finishes.
		scopedDB := db
		go func(sid string, db *gorm.DB) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := service.Session.RecordLastSeen(bgCtx, db.WithContext(bgCtx), sid); err != nil {
				slog.Warn("session record last_seen failed", "sid", sid, "err", err)
			}
		}(sid, scopedDB)
	}
	return true
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
