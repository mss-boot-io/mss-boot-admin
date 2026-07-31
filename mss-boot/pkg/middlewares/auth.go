package middlewares

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/4/13 22:44
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/4/13 22:44
 */

import (
	"errors"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/store"
)

// AuthMiddleware 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		api := response.Make(c)
		// 登录认证
		accessToken := getTokenFromHeader(c)
		if accessToken == "" {
			api.AddError(errors.New("token is empty"))
			api.Err(http.StatusUnauthorized)
			return
		}
		client, err := store.DefaultOAuth2Store.
			GetClientByDomain(c.Request.Context(), c.Request.Host)
		if err != nil {
			api.AddError(err)
			api.Err(http.StatusUnauthorized)
			return
		}
		provider, err := oidc.NewProvider(c, client.GetIssuer())
		if err != nil {
			api.AddError(err)
			api.Err(http.StatusUnauthorized)
			return
		}
		idTokenVerifier := provider.Verifier(&oidc.Config{ClientID: client.GetClientID()})
		idToken, err := idTokenVerifier.Verify(c, accessToken)
		if err != nil {
			api.AddError(err)
			api.Err(http.StatusUnauthorized)
			return
		}
		user := &User{}
		err = idToken.Claims(user)
		if err != nil {
			api.AddError(err)
			api.Err(http.StatusUnauthorized)
			return
		}
		// 鉴权
		c.Set("user", user)
		c.Next()
	}
}

// getTokenFromHeader 获取token
func getTokenFromHeader(c *gin.Context) string {
	return strings.ReplaceAll(strings.ReplaceAll(
		c.GetHeader("Authorization"),
		"Bearer ",
		""),
		"bearer",
		"")
}

// GetLoginUser 获取登录用户
func GetLoginUser(c *gin.Context) *User {
	user, ok := c.Get("user")
	if !ok {
		return nil
	}
	return user.(*User)
}
