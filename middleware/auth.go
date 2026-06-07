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
	"sync"
	"time"

	"github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot-admin/service"
	bootpkg "github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/security"
	"github.com/spf13/cast"

	"github.com/mss-boot-io/mss-boot-admin/config"
	"github.com/mss-boot-io/mss-boot-admin/pkg"
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
					if bag.req == nil {
						slog.Error("session create skipped: missing login context")
						return jwt.MapClaims{}
					}
					sid, err := service.Session.Create(bag.req, center.Default.GetDB(bag.req, &models.UserSession{}), service.CreateSessionInput{
						UserID:    v.GetUserID(),
						Username:  v.GetUsername(),
						RoleID:    v.GetRoleID(),
						IP:        bag.req.ClientIP(),
						UserAgent: bag.req.GetHeader("User-Agent"),
						TTL:       config.Cfg.Auth.Timeout,
					})
					if err != nil {
						slog.Error("session create failed", "err", err)
						return jwt.MapClaims{}
					}
					claims["sid"] = sid
				}
				return claims
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) any {
			claims := jwt.ExtractClaims(c)
			verifier := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
			if personAccessToken, ok := claims["personAccessToken"]; ok && personAccessToken != "" {
				verifier.SetRefreshTokenDisable(true)
				verifier.SetPersonAccessToken(personAccessToken.(string))
				err := verifier.CheckToken(c, personAccessToken.(string))
				if err != nil {
					return nil
				}
				return verifier
			}
			err := json.Unmarshal([]byte(cast.ToString(claims["verifier"])), verifier)
			if err != nil {
				return nil
			}
			if verifier.GetRefreshTokenDisable() {
				// check token revoked
				token := jwt.GetToken(c)
				err = verifier.CheckToken(c, token)
				if err != nil {
					return nil
				}
				return verifier
			}
			if config.Cfg.Auth.SessionEnabled {
				sid := cast.ToString(claims["sid"])
				if sid == "" {
					return nil
				}
				db := center.Default.GetDB(c, &models.UserSession{})
				res, err := service.Session.Lookup(c, db, sid)
				if err != nil || res.Status != service.LookupActive {
					return nil
				}
				if shouldTouch, terr := service.Session.MarkLastSeen(c, sid); terr == nil && shouldTouch {
					go func(sid string) {
						bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						if err := service.Session.RecordLastSeen(bgCtx, gormdb.DB, sid); err != nil {
							slog.Warn("session record last_seen failed", "sid", sid, "err", err)
						}
					}(sid)
				}
			}
			return verifier
		},
		Authenticator: func(c *gin.Context) (any, error) {
			api := response.Make(c)
			loginVals := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			ok, user, err := loginVals.Verify(c)
			ip := c.ClientIP()
			userAgent := c.GetHeader("User-Agent")
			db := center.Default.GetDB(c, nil)

			if err != nil {
				if logErr := service.Audit.LogLogin(db, "", extractLoginUsername(loginVals), ip, userAgent, err.Error(), false); logErr != nil {
					api.AddError(logErr).Log.Warn("write login log failed")
				}
				return nil, err
			}
			if ok {
				if logErr := service.Audit.LogLogin(db, user.GetUserID(), user.GetUsername(), ip, userAgent, "login success", true); logErr != nil {
					api.AddError(logErr).Log.Warn("write login log failed")
				}
				if config.Cfg.Auth.SessionEnabled {
					loginContext.Store(c)
				}
				return user, nil
			}
			if logErr := service.Audit.LogLogin(db, "", extractLoginUsername(loginVals), ip, userAgent, "authentication failed", false); logErr != nil {
				api.AddError(logErr).Log.Warn("write login log failed")
			}
			return nil, jwt.ErrFailedAuthentication
		},
		Authorizator: func(data any, c *gin.Context) bool {
			switch c.Request.URL.Path {
			case "/admin/api/user/userInfo",
				"/admin/api/menu/authorize",
				"/admin/api/system-configs",
				"/admin/api/notice/unread",
				"/admin/api/user-configs/profile",
				"user-auth-tokens",
				"/admin/api/user/oauth2",
				"/admin/api/user-configs/notification",
				"/admin/api/app-configs/theme",
				"/admin/api/user-auth-tokens",
				"/admin/api/languages":
				if c.Request.Method == http.MethodGet {
					return true
				}
			}
			passPath := []string{
				"/admin/api/notice/.*",
				"/admin/api/user-configs/.*",
				"/admin/api/departments/.*",
				"/admin/api/posts/.*",
			}
			for i := range passPath {
				// 使用正则匹配
				if ok, _ := regexp.MatchString(passPath[i], c.Request.URL.Path); ok {
					return true
				}
			}
			api := response.Make(c)
			if v, ok := data.(security.Verifier); ok {
				if v.Root() {
					return true
				}
				enable, err := gormdb.Enforcer.Enforce(v.GetRoleID(), pkg.APIAccessType.String(), c.Request.URL.Path, c.Request.Method)
				if err != nil {
					api.AddError(err).Log.Error("Enforcer.Enforce error")
					return false
				}
				return enable
			}
			return false
		},
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
			verifier := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
			if verifier.GetRefreshTokenDisable() {
				writeAuthErrorResponse(c, http.StatusOK, http.StatusUnauthorized, "refresh token disabled")
				return
			}
			err = json.Unmarshal([]byte(cast.ToString(claims["verifier"])), verifier)
			if err != nil {
				writeAuthErrorResponse(c, http.StatusOK, http.StatusUnauthorized, "refresh token error")
				return
			}
			ok, _, err := verifier.Verify(c)
			if err != nil || !ok {
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

// GetVerify 获取当前登录用户
func GetVerify(ctx *gin.Context) security.Verifier {
	api := response.Make(ctx)
	token, err := Auth.ParseToken(ctx)
	if err != nil {
		api.AddError(err).Log.WarnContext(ctx, "parseToken failed")
		return nil
	}
	claims := jwt.ExtractClaimsFromToken(token)
	if len(claims) == 0 {
		slog.Debug("GetVerify claims is empty")
		return nil
	}
	if personAccessToken, ok := claims["personAccessToken"]; ok && personAccessToken != "" {
		verifier := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
		verifier.SetPersonAccessToken(personAccessToken.(string))
		verifier.SetRefreshTokenDisable(true)
		err = verifier.CheckToken(ctx, personAccessToken.(string))
		if err != nil {
			return nil
		}
		return verifier
	}
	verifier := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
	err = json.Unmarshal([]byte(cast.ToString(claims["verifier"])), verifier)
	if err != nil {
		slog.Debug("GetVerify", "err", err)
		return nil
	}
	return verifier
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

type loginCtxBag struct {
	req *gin.Context
}

var loginContextMap sync.Map // goroutine id -> *loginCtxBag

type loginContextHandle struct{}

var loginContext loginContextHandle

func (loginContextHandle) Store(c *gin.Context) {
	loginContextMap.Store(goroutineID(), &loginCtxBag{req: c})
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
// not the *gin.Context. To create a server-side session bound to the request
// (IP / User-Agent) we stash the context in loginContextMap keyed by goroutine
// id inside Authenticator and read it back inside PayloadFunc.
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
