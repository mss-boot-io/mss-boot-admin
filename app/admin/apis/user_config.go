package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/app/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/app/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/middleware"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/2 00:41:41
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/2 00:41:41
 */

type UserConfig struct {
	*controller.Simple
	service service.UserConfig
}

func (e *UserConfig) GetAction(string) response.Action {
	return nil
}

func (e *UserConfig) Other(r *gin.RouterGroup) {
	r.GET("/user-configs/:group", response.AuthHandler, e.Group)
	r.PUT("/user-configs/:group", response.AuthHandler, e.Control)
	r.GET("/user-configs/profile", response.AuthHandler, e.Profile)
}

// Profile 用户配置
// @Summary 用户配置
// @Description 用户配置
// @Tags user-config
// @Accept application/json
// @Product application/json
// @Success 200 {object} map[string]map[string]string
// @Router /admin/api/user-configs/profile [get]
// @Security Bearer
func (e *UserConfig) Profile(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	result, err := e.service.Profile(ctx, verify.GetUserID())
	if err != nil {
		api.AddError(err).Log.Error("get user config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(result)
}

// Group 用户配置分组
// @Summary 用户配置分组
// @Description 用户配置分组
// @Tags user-config
// @Accept application/json
// @Product application/json
// @Param group path string true "group"
// @Success 200 {object} map[string]string
// @Router /admin/api/user-configs/{group} [get]
// @Security Bearer
func (e *UserConfig) Group(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	req := &dto.UserConfigGroupRequest{}
	if err := api.Bind(req).Error; err != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	result, err := e.service.Group(ctx, verify.GetUserID(), req.Group)
	if err != nil {
		api.AddError(err).Log.Error("get user config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(result)
}

// Control 用户配置控制
// @Summary 用户配置控制
// @Description 用户配置控制
// @Tags user-config
// @Accept application/json
// @Product application/json
// @Param group path string true "group"
// @Param data body dto.UserConfigControlRequest true "data"
// @Success 200
// @Router /admin/api/user-configs/{group} [put]
// @Security Bearer
func (e *UserConfig) Control(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	req := &dto.UserConfigControlRequest{}
	if err := api.Bind(req).Error; err != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	err := e.service.CreateOrUpdate(ctx, verify.GetUserID(), req.Group, req.Data)
	if err != nil {
		api.AddError(err).Log.Error("control user config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(nil)
}
