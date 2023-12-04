package apis

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/19 15:28:20
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/19 15:28:20
 */

import (
	"github.com/mss-boot-io/mss-boot-admin-api/config"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/middlewares"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/middleware"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
)

func init() {
	e := &Github{}
	response.AppendController(e)
}

// Github github
type Github struct {
	controller.Simple
}

func (*Github) GetKey() string {
	return "github"
}

func (*Github) GetAction(string) response.Action {
	return nil
}

func (e *Github) Other(r *gin.RouterGroup) {
	r.Use(middleware.GetMiddlewares()...)
	r.POST("/github/control", e.Control)
	r.GET("/github/get", e.Get)
	r.GET("/github/get-login-url", e.GetLoginURL)
	r.GET("/github/callback", e.Callback)
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
	req := &dto.GithubGetLoginURLReq{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	conf, err := config.Cfg.OAuth2.GetOAuth2Config(c)
	if err != nil {
		api.AddError(err).Log.Error("get oauth2 config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	//api.OK(conf.AuthCodeURL(req.State))
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(conf.AuthCodeURL(req.State)))
}

// Callback github回调
// @Summary github回调
// @Description github回调
// @Tags generator
// @Accept  application/json
// @Product application/json
// @Param code query string true "code"
// @Param state query string true "state"
// @Success 200 {object} dto.GithubToken
// @Failure 422 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/api/github/callback [get]
func (e *Github) Callback(c *gin.Context) {
	api := response.Make(c)
	conf, err := config.Cfg.OAuth2.GetOAuth2Config(c)
	if err != nil {
		api.AddError(err).Log.Error("get oauth2 config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	req := &dto.GithubCallbackReq{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}

	token, err := conf.Exchange(c, req.Code)
	if err != nil {
		api.AddError(err).Log.Error("exchange token error")
		api.Err(http.StatusInternalServerError)
		return
	}
	resp := &dto.GithubToken{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
	}
	if !token.Expiry.IsZero() {
		resp.Expiry = &token.Expiry
	}
	api.OK(resp)
}

// Control 创建或更新github配置
// @Summary 创建或更新github配置
// @Description 创建或更新github配置
// @Tags generator
// @Accept  application/json
// @Product application/json
// @Param data body dto.GithubControlReq true "data"
// @Success 200 {object} nil
// @Router /admin/api/github/control [post]
// @Security Bearer
func (e *Github) Control(c *gin.Context) {
	api := response.Make(c)
	req := &dto.GithubControlReq{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	user := middlewares.GetLoginUser(c)
	if user == nil {
		api.Err(http.StatusUnauthorized, "user is empty")
		return
	}

	g := &models.Github{
		Email:    user.Email,
		Username: user.Email,
		Password: req.Password,
	}
	err := gormdb.DB.Create(g).Error
	if err != nil {
		api.AddError(err).Log.Error("insert github config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(nil)
}

// Get 获取github配置
// @Summary 获取github配置
// @Description 获取github配置
// @Tags generator
// @Accept  application/json
// @Product application/json
// @Success 200 {object} dto.GithubGetResp
// @Router /admin/api/github/get [get]
// @Security Bearer
func (e *Github) Get(c *gin.Context) {
	api := response.Make(c)
	user := middlewares.GetLoginUser(c)
	if user == nil {
		api.Err(http.StatusUnauthorized, "user is empty")
		return
	}

	g, err1 := models.GetMyGithubConfig(c, user.Email)
	result := &dto.GithubGetResp{
		Email:     user.Email,
		CreatedAt: g.CreatedAt,
		UpdatedAt: g.UpdatedAt,
	}
	if err1 == nil {
		result.Configured = true
	}
	api.OK(result)
}
