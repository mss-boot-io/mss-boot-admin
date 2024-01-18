package apis

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

import (
	"log/slog"
	"net/http"

	"github.com/mss-boot-io/mss-boot-admin-api/center"

	"github.com/mss-boot-io/mss-boot-admin-api/pkg"

	"github.com/mss-boot-io/mss-boot/pkg/response/actions"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin-api/app/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/app/admin/models"
	"github.com/mss-boot-io/mss-boot-admin-api/middleware"
)

func init() {
	e := &Role{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Role)),
			controller.WithSearch(new(dto.RoleSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
			controller.WithScope(center.Default.Scope),
		),
	}
	response.AppendController(e)
}

type Role struct {
	*controller.Simple
}

func (e *Role) Other(r *gin.RouterGroup) {
	r.POST("/role/authorize/:roleID", middleware.Auth.MiddlewareFunc(), e.SetAuthorize)
	r.GET("/role/authorize/:roleID", middleware.Auth.MiddlewareFunc(), e.GetAuthorize)
}

// GetAuthorize 获取角色授权
// @Summary 获取角色授权
// @Description 获取角色授权
// @Tags role
// @Accept  application/json
// @Product application/json
// @param roleID path string true "roleID"
// @Success 200 {object} dto.GetAuthorizeResponse
// @Router /admin/api/role/authorize/{roleID} [get]
// @Security Bearer
func (e *Role) GetAuthorize(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.GetAuthorizeRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	resp := &dto.GetAuthorizeResponse{
		RoleID: req.RoleID,
		Paths:  make([]string, 0),
	}
	// get permissions
	permissions := gormdb.Enforcer.GetFilteredPolicy(0, req.RoleID, pkg.MenuAccessType.String())
	permissions = append(permissions,
		gormdb.Enforcer.GetFilteredPolicy(0, req.RoleID, pkg.ComponentAccessType.String())...)
	for i := range permissions {
		if len(permissions[i]) < 4 {
			continue
		}
		if permissions[i][0] == req.RoleID &&
			(permissions[i][1] == pkg.MenuAccessType.String() ||
				permissions[i][1] == pkg.ComponentAccessType.String()) {
			resp.Paths = append(resp.Paths, permissions[i][2])
		}
	}
	api.OK(resp)
}

// SetAuthorize 角色授权
// @Summary 角色授权
// @Description 给角色授权
// @Tags role
// @param roleID path string true "roleID"
// @Param data body dto.SetAuthorizeRequest true "data"
// @Success 200
// @Router /admin/api/role/authorize/{roleID} [post]
// @Security Bearer
func (e *Role) SetAuthorize(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.SetAuthorizeRequest{}
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

	//// add permissions
	//for i := range req.Paths {
	//	_, err = gormdb.Enforcer.AddPermissionForUser(req.RoleID, models.MenuAccessType.String(), req.Paths[i], )
	//	if err != nil {
	//		api.AddError(err).Log.Error("add permission for user error", "err", err)
	//		api.Err(http.StatusInternalServerError)
	//		return
	//	}
	//}
	menus := make([]*models.Menu, 0)
	err = center.Default.GetDB(ctx, &models.Menu{}).Model(&models.Menu{}).
		Where("path in (?)", req.Paths).
		Where("type = ? or type = ?", pkg.MenuAccessType, pkg.ComponentAccessType).
		Preload("Children").
		Find(&menus).Error
	for i := range menus {
		_, err = gormdb.Enforcer.AddPermissionForUser(
			req.RoleID, menus[i].Type.String(), menus[i].Path, menus[i].Method)
		if err != nil {
			api.AddError(err).Log.
				Error("add menu and component permission for role error",
					slog.String("roleID", req.RoleID))
			api.Err(http.StatusInternalServerError)
			return
		}
		for j := range menus[i].Children {
			if menus[i].Children[j].Type != pkg.APIAccessType {
				continue
			}
			_, err = gormdb.Enforcer.AddPermissionForUser(
				req.RoleID, pkg.APIAccessType.String(), menus[i].Children[j].Path, menus[i].Children[j].Method)
			if err != nil {
				api.AddError(err).Log.
					Error("add api permission for role error",
						slog.String("roleID", req.RoleID))
				api.Err(http.StatusInternalServerError)
				return
			}
		}
	}
	err = gormdb.Enforcer.SavePolicy()
	if err != nil {
		api.AddError(err).Log.Error("save policy error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
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
// @Success 201 {object} models.Role
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
// @Success 200 {object} models.Role
// @Router /admin/api/roles/{id} [put]
// @Security Bearer
func (e *Role) Update(*gin.Context) {}

// Get 获取角色
// @Summary 获取角色
// @Description 获取角色
// @Tags role
// @Param id path string true "id"
// @Success 200 {object} models.Role
// @Router /admin/api/roles/{id} [get]
// @Security Bearer
func (e *Role) Get(*gin.Context) {}

// List 角色列表
// @Summary 角色列表
// @Description 角色列表
// @Tags role
// @Accept  application/json
// @Product application/json
// @Param page query int false "page"
// @Param pageSize query int false "pageSize"
// @Param id query string false "id"
// @Param name query string false "name"
// @Param status query string false "status"
// @Param remark query string false "remark"
// @Success 200 {object} response.Page{data=[]models.Role}
// @Router /admin/api/roles [get]
// @Security Bearer
func (e *Role) List(*gin.Context) {}
