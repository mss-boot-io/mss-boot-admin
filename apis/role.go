package apis

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

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

func init() {
	e := &Role{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Role)),
			controller.WithSearch(new(dto.RoleSearch)),
			controller.WithModelProvider(authentic.ModelProviderGorm),
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
// @Success 200 {object} response.Response
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
