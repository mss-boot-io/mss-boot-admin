package apis

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/19 15:28:20
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/19 15:28:20
 */

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/middlewares"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
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
	r.Use(middlewares.AuthMiddleware())
	r.POST("/github/create-or-update", e.CreatOrUpdate)
	r.GET("/github/get", e.Get)
}

// CreatOrUpdate 创建或更新github配置
// @Summary 创建或更新github配置
// @Description 创建或更新github配置
// @Tags generator
// @Accept  application/json
// @Product application/json
// @Param data body dto.GithubCreateReq true "data"
// @Success 200 {object} response.Response
// @Router /generator/api/v1/github/create-or-update [post]
// @Security Bearer
func (e *Github) CreatOrUpdate(c *gin.Context) {
	api := response.Make(c)
	req := &dto.GithubCreateReq{}
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
// @Success 200 {object} response.Response{data=dto.GithubGetResp}
// @Router /generator/api/v1/github/get [get]
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
