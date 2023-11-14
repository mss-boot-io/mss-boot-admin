package apis

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

import (
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/middleware"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
)

func init() {
	e := &Role{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Role)),
			controller.WithSearch(new(dto.RoleSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Role struct {
	*controller.Simple
}

func (e *Role) Other(r *gin.RouterGroup) {
	r.POST("/role/authorize", middleware.Auth.MiddlewareFunc(), e.Authorize)
}

// Authorize 角色授权
// @Summary 角色授权
// @Description 给角色授权
// @Tags role
// @Param data body dto.AuthorizeRequest true "data"
// @Success 200
// @Router /admin/api/role/authorize [post]
// @Security Bearer
func (e *Role) Authorize(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.AuthorizeRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	// authorize
	_, err := gormdb.Enforcer.DeletePermissionsForUser(req.RoleID)
	if err != nil {
		api.AddError(err).Log.Error("delete permissions for user error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}

	// add permissions
	for i := range req.MenuIDS {
		_, err = gormdb.Enforcer.AddPermissionForUser(req.RoleID, models.APIAccessType.String(), req.MenuIDS[i])
		if err != nil {
			api.AddError(err).Log.Error("add permission for user error", "err", err)
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	for i := range req.APIIDS {
		_, err = gormdb.Enforcer.AddPermissionForUser(req.RoleID, models.MenuAccessType.String(), req.APIIDS[i])
		if err != nil {
			api.AddError(err).Log.Error("add permission for user error", "err", err)
			api.Err(http.StatusInternalServerError)
			return
		}
	}

	api.OK(nil)
}

// Create 创建角色
// @Summary 创建角色
// @Description 创建角色
// @Tags role
// @Accept  application/json
// @Product application/json
// @Param data body models.Role true "data"
// @Success 201
// @Router /admin/api/roles [post]
// @Security Bearer
func (e *Role) Create(*gin.Context) {}

// Delete 删除角色
// @Summary 删除角色
// @Description 删除角色
// @Tags role
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/roles/{id} [delete]
// @Security Bearer
func (e *Role) Delete(*gin.Context) {}

// Update 更新角色
// @Summary 更新角色
// @Description 更新角色
// @Tags role
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Param data body models.Role true "data"
// @Success 200
// @Router /admin/api/roles/{id} [put]
// @Security Bearer
func (e *Role) Update(*gin.Context) {}

// List 角色列表
// @Summary 角色列表
// @Description 角色列表
// @Tags role
// @Accept  application/json
// @Product application/json
// @Param page query int false "page"
// @Param page_size query int false "pageSize"
// @Param id query string false "id"
// @Param name query string false "name"
// @Param status query int false "status"
// @Param remark query string false "remark"
// @Success 200 {object} response.Page{data=[]models.Role}
// @Router /admin/api/roles [get]
// @Security Bearer
func (e *Role) List(*gin.Context) {}
