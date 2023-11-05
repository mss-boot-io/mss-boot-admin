package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions/authentic"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/middleware"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/15 13:41:22
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/15 13:41:22
 */

func init() {
	e := &Menu{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Menu)),
			controller.WithSearch(new(dto.RoleSearch)),
			controller.WithModelProvider(authentic.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Menu struct {
	*controller.Simple
}

func (e *Menu) Other(r *gin.RouterGroup) {
	r.GET("/menu/tree", middleware.Auth.MiddlewareFunc(), e.Tree)
	r.GET("/menu/authorize/:roleID", middleware.Auth.MiddlewareFunc(), e.GetAuthorize)
	r.PUT("/menu/authorize/:roleID", middleware.Auth.MiddlewareFunc(), e.UpdateAuthorize)
}

// UpdateAuthorize 更新菜单权限
// @Summary 更新菜单权限
// @Description 更新菜单权限
// @Tags menu
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Param data body dto.UpdateAuthorizeRequest true "data"
// @Success 200 {object} response.Response
// @Router /admin/api/menu/{id} [put]
// @Security Bearer
func (e *Menu) UpdateAuthorize(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.UpdateAuthorizeRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnauthorized)
		return
	}
	// todo check roleID
	// todo check menu keys

	// todo commit transaction

	// delete all policy for role
	err := gormdb.DB.Where(&models.CasbinRule{
		PType: "p",
		V0:    req.RoleID,
	}).Delete(&models.CasbinRule{}).Error
	if err != nil {
		api.AddError(err).Log.Error("delete role error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	defer gormdb.Enforcer.LoadPolicy()
	if err != nil {
		api.AddError(err).Log.Error("delete role error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	rules := make([]*models.CasbinRule, len(req.Keys))
	for i := range req.Keys {
		rules[i] = &models.CasbinRule{
			PType: "p",
			V0:    req.RoleID,
			V1:    req.Keys[i],
			V2:    models.MenuAccessType.String(),
		}
	}
	if err = gormdb.DB.Create(&rules).Error; err != nil {
		api.AddError(err).Log.Error("create casbin rule error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(nil)
}

// GetAuthorize 获取菜单权限
// @Summary 获取菜单权限
// @Description 获取菜单权限
// @Tags menu
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Success 200 {object} response.Response{data=[]models.Menu} "{"code": 200, "data": [...]}"
// @Router /admin/api/menu/{id} [get]
// @Security Bearer
func (e *Menu) GetAuthorize(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.GetAuthorizeRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnauthorized)
		return
	}
	list := make([]*models.Menu, 0)
	err := gormdb.DB.WithContext(ctx).Find(&list).Error
	if err != nil {
		api.Log.Error("get menu tree error", "err", err)
		api.Err(http.StatusInternalServerError, err.Error())
		return
	}
	// check select menu
	for i := range list {
		list[i].Select, err = gormdb.Enforcer.Enforce(
			req.RoleID, list[i].Key, models.MenuAccessType.String())
		if err != nil {
			api.AddError(err).Log.Error("get menu tree error", "err", err)
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	api.OK(models.GetMenuTree(list))
}

// Tree 获取菜单树
// @Summary 获取菜单树
// @Description 获取菜单树
// @Tags 获取菜单树
// @Success 200 {object} response.Response{data=[]models.Menu} "{"code": 200, "data": [...]}"
// @Router /admin/api/menu/tree [get]
// @Security Bearer
func (e *Menu) Tree(ctx *gin.Context) {
	api := response.Make(ctx)
	list := make([]*models.Menu, 0)
	err := gormdb.DB.WithContext(ctx).Find(&list).Error
	if err != nil {
		api.Log.Error("get menu tree error", "err", err)
		api.Err(http.StatusInternalServerError, err.Error())
		return
	}
	api.OK(models.GetMenuTree(list))
}
