package middleware

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cast"
	"reflect"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin-api/config"
	log "github.com/mss-boot-io/mss-boot/core/logger"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/security"
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
			loginVals := reflect.New(reflect.TypeOf(Verifier).Elem()).Interface().(security.Verifier)
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			ok, user, err := loginVals.Verify()
			if err != nil {
				return nil, err
			}
			if ok {
				return user, nil
			}
			return nil, jwt.ErrFailedAuthentication
		},
		Authorizator: func(data any, c *gin.Context) bool {
			if v, ok := data.(security.Verifier); ok {
				//todo verify permission
				path := c.Request.URL.Path
				fmt.Println(v.GetRoleID(), path, c.Request.Method)
				return true
			}
			return false
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

		// TimeFunc provides the current time. You can override it to use another time value. This is useful for testing or if your server uses a different time zone than your tokens.
		TimeFunc: time.Now,
	}
	err := Auth.MiddlewareInit()
	if err != nil {
		log.Fatal("authMiddleware.MiddlewareInit() Error:" + err.Error())
	}
	response.AuthHandler = Auth.MiddlewareFunc()
}
