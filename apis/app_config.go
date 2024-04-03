package apis

import (
	"github.com/mss-boot-io/mss-boot-admin/service"
	"net/http"

	"github.com/mss-boot-io/mss-boot-admin/middleware"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin/dto"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/11 17:36:55
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/11 17:36:55
 */

func init() {
	e := &AppConfig{
		Simple: controller.NewSimple(),
	}
	response.AppendController(e)
}

type AppConfig struct {
	*controller.Simple
	service service.AppConfig
}

func (e *AppConfig) GetAction(string) response.Action {
	return nil
}

func (e *AppConfig) Other(r *gin.RouterGroup) {
	r.GET("/app-configs/:group", response.AuthHandler, e.Group)
	r.PUT("/app-configs/:group", response.AuthHandler, e.Control)
	r.GET("/app-configs/profile", response.AuthHandler, e.Profile)
	r.GET("/app-configs/no-auth-profile", e.Profile)
}

// Profile 获取应用配置
// @Summary 获取应用配置
// @Description 获取应用配置
// @Tags app-config
// @Accept application/json
// @Product application/json
// @Success 200 {object} map[string]map[string]string
// @Router /admin/api/app-configs/profile [get]
// @Security Bearer
func (e *AppConfig) Profile(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	profile, err := e.service.Profile(ctx, verify != nil)
	if err != nil {
		api.AddError(err).Log.Error("get app config profile error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(profile)
}

// NoAuthProfile 获取应用配置(无需认证)
// @Summary 获取应用配置(无需认证)
// @Description 获取应用配置(无需认证)
// @Tags app-config
// @Accept application/json
// @Product application/json
// @Success 200 {object} map[string]map[string]string
// @Router /admin/api/app-configs/no-auth-profile [get]
func (*AppConfig) NoAuthProfile(*gin.Context) {}

// Group 应用配置分组
// @Summary 应用配置分组
// @Description 应用配置分组
// @Tags app-config
// @Accept application/json
// @Product application/json
// @Param group path string true "group"
// @Success 200 {object} map[string]models.AppConfig
// @Router /admin/api/app-configs/{group} [get]
// @Security Bearer
func (e *AppConfig) Group(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.AppConfigGroupRequest{}
	if err := api.Bind(req).Error; err != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	result, err := e.service.Group(ctx, req.Group)
	if err != nil {
		api.AddError(err).Log.Error("get app config group error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(result)
}

// Control 应用配置控制
// @Summary 应用配置控制
// @Description 应用配置控制
// @Tags app-config
// @Accept application/json
// @Product application/json
// @Param group path string true "group"
// @Param data body dto.AppConfigControlRequest true "data"
// @Success 200
// @Router /admin/api/app-configs/{group} [put]
// @Security Bearer
func (e *AppConfig) Control(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.AppConfigControlRequest{
		Data: make(map[string]dto.AppConfigControlItem),
	}
	if err := api.Bind(req).Error; err != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	err := e.service.CreateOrUpdate(ctx, req.Group, req.Data)
	if err != nil {
		api.AddError(err).Log.Error("update app config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(nil)
}
