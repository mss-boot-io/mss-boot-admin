package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/security"
	"github.com/spf13/cast"

	"github.com/mss-boot-io/mss-boot-admin/center"
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
				rb, _ := json.Marshal(v)

				return jwt.MapClaims{
					"verifier": string(rb),
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) any {
			claims := jwt.ExtractClaims(c)
			verifier := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
			err := json.Unmarshal([]byte(cast.ToString(claims["verifier"])), verifier)
			if err != nil {
				return nil
			}
			return verifier
		},
		Authenticator: func(c *gin.Context) (any, error) {
			// login
			loginVals := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			ok, user, err := loginVals.Verify(c)
			if err != nil {
				return nil, err
			}
			if ok {
				return user, nil
			}
			return nil, jwt.ErrFailedAuthentication
		},
		Authorizator: func(data any, c *gin.Context) bool {
			switch c.Request.URL.Path {
			case "/admin/api/user/userInfo",
				"/admin/api/menu/authorize",
				"/admin/api/system-configs",
				"/admin/api/notice/unread",
				"/admin/api/notice/:id",
				"/admin/api/user-configs/:group",
				"/admin/api/user-configs/profile",
				"/admin/api/languages":
				return true
			}
			api := response.Make(c)
			if v, ok := data.(security.Verifier); ok {
				//todo check tenant domain
				tenant, err := center.GetTenant().GetTenant(c)
				if err != nil {
					api.AddError(err).Log.Error("GetTenant error")
					return false
				}
				if v.GetTenantID() != tenant.GetID() {
					return false
				}
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
			claims := jwt.ExtractClaims(c)
			if len(claims) == 0 {
				c.JSON(http.StatusOK, gin.H{
					"code":   http.StatusUnauthorized,
					"status": "error",
					"msg":    "refresh token error",
				})
				return
			}
			verifier := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
			err := json.Unmarshal([]byte(cast.ToString(claims["verifier"])), verifier)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"code":   http.StatusUnauthorized,
					"status": "error",
					"msg":    "refresh token error",
				})
				return
			}
			ok, _, err := verifier.Verify(c)
			if err != nil || !ok {
				c.JSON(http.StatusOK, gin.H{
					"code":   http.StatusUnauthorized,
					"status": "error",
					"msg":    "refresh token error",
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"code":   http.StatusOK,
				"token":  token,
				"expire": expire.Format(time.RFC3339),
			})
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"code":   code,
				"status": "error",
				"msg":    message,
			})
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
	verifier := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
	err = json.Unmarshal([]byte(cast.ToString(claims["verifier"])), verifier)
	if err != nil {
		slog.Debug("GetVerify", "err", err)
		return nil
	}
	return verifier
}
