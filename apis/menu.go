package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/middleware"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
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
			controller.WithModelProvider(actions.ModelProviderGorm),
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
		api.AddError(err).Log.Errorf("delete role error: %v", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	defer gormdb.Enforcer.LoadPolicy()
	if err != nil {
		api.AddError(err).Log.Errorf("delete role error: %v", err)
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
		api.AddError(err).Log.Errorf("create casbin rule error: %v", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(nil)
}

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
		api.Log.Errorf("get menu tree error: %v", err)
		api.Err(http.StatusInternalServerError, err.Error())
		return
	}
	// check select menu
	for i := range list {
		list[i].Select, err = gormdb.Enforcer.Enforce(
			req.RoleID, list[i].Key, models.MenuAccessType.String())
		if err != nil {
			api.AddError(err).Log.Errorf("get menu tree error: %v", err)
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	api.OK(models.GetMenuTree(list))
}

func (e *Menu) Tree(ctx *gin.Context) {
	api := response.Make(ctx)
	list := make([]*models.Menu, 0)
	err := gormdb.DB.WithContext(ctx).Find(&list).Error
	if err != nil {
		api.Log.Errorf("get menu tree error: %v", err)
		api.Err(http.StatusInternalServerError, err.Error())
		return
	}
	api.OK(models.GetMenuTree(list))
}
