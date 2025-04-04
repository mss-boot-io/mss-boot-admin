package apis

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/19 15:28:20
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/19 15:28:20
 */

import (
	"net/http"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/center"
	"golang.org/x/oauth2"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/middleware"
)

func init() {
	e := &Github{
		Simple: controller.NewSimple(),
	}
	response.AppendController(e)
}

// Github github
type Github struct {
	*controller.Simple
}

func (*Github) GetKey() string {
	return "github"
}

func (*Github) GetAction(string) response.Action {
	return nil
}

func (e *Github) Other(r *gin.RouterGroup) {
	r.Use(middleware.GetMiddlewares()...)
	r.GET("/github/get-login-url", e.GetLoginURL)
}

// GetLoginURL 获取github登录地址
// @Summary 获取github登录地址
// @Description 获取github登录地址
// @Tags generator
// @Accept  application/json
// @Product application/json
// @Param state query string true "state"
// @Success 200 {object} string
// @Router /admin/api/github/get-login-url [get]
func (e *Github) GetLoginURL(c *gin.Context) {
	api := response.Make(c)
	req := &dto.OauthGetLoginURLReq{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	clientID, _ := center.GetAppConfig().GetAppConfig(c, "security.githubClientId")
	clientSecret, _ := center.GetAppConfig().GetAppConfig(c, "security.githubClientSecret")
	redirectURL, _ := center.GetAppConfig().GetAppConfig(c, "security.githubRedirectURL")
	scopes, _ := center.GetAppConfig().GetAppConfig(c, "security.githubScope")
	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       strings.Split(scopes, ","),
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}
	//api.OK(conf.AuthCodeURL(req.State))
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(conf.AuthCodeURL(req.State)))
}
