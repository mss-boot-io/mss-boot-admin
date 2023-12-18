package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin-api/middleware"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/13 11:24:21
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/13 11:24:21
 */

type LanguageDefine struct {
	*controller.Simple
}

func init() {
	e := &LanguageDefine{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.LanguageDefine)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

func (e *LanguageDefine) GetAction(string) response.Action {
	return nil
}

func (e *LanguageDefine) Other(r *gin.RouterGroup) {
	r.DELETE("/language-defines/:id", middleware.Auth.MiddlewareFunc(), e.Delete)
	r.POST("/language-defines", middleware.Auth.MiddlewareFunc(), e.Control)
}

// Delete 删除
// @Summary 删除
// @Description 删除
// @Tags language_define
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/language-defines/{id} [delete]
// @Security Bearer
func (e *LanguageDefine) Delete(ctx *gin.Context) {
	api := response.Make(ctx)
	id := ctx.Param("id")
	if id == "" {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	err := gormdb.DB.Delete(&models.LanguageDefine{}, "id = ?", id).Error
	if err != nil {
		api.AddError(err).Log.Error("delete language_define error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(nil)
}

// Control 创建/更新
// @Summary 创建/更新
// @Description 创建/更新
// @Tags language_define
// @Accept  application/json
// @Product application/json
// @Param data body models.LanguageDefine true "data"
// @Success 201
// @Router /admin/api/language-defines [post]
// @Security Bearer
func (e *LanguageDefine) Control(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &models.LanguageDefine{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if req.ID == "" {
		// create
		err := gormdb.DB.Create(req).Error
		if err != nil {
			api.AddError(err).Log.Error("create language_define error")
			api.Err(http.StatusInternalServerError)
			return
		}
		api.OK(req)
		return
	}
	// update
	err := gormdb.DB.Model(req).Updates(req).Error
	if err != nil {
		api.AddError(err).Log.Error("update language_define error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(req)
}
