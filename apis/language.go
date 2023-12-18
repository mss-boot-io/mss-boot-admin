package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/12 11:54:56
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/12 11:54:56
 */

func init() {
	e := &Language{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Language)),
			controller.WithSearch(new(dto.LanguageSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Language struct {
	*controller.Simple
}

func (e *Language) Other(r *gin.RouterGroup) {
	r.GET("/language/all", e.GetAll)
}

// GetAll 获取所有语言
// @Summary 获取所有语言
// @Description 获取所有语言
// @Tags language
// @Success 200 {array} models.Language
// @Router /admin/api/language/all [get]
// @Security Bearer
func (e *Language) GetAll(ctx *gin.Context) {
	api := response.Make(ctx)
	list := make([]*models.Language, 0)
	err := gormdb.DB.Model(&models.Language{}).
		Where("status = ?", enum.Enabled).
		Preload("Defines").
		Find(&list).Error
	if err != nil {
		api.AddError(err).Log.Error("get language error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(list)
}

// Create 创建Language
// @Summary 创建Language
// @Description 创建Language
// @Tags language
// @Accept  application/json
// @Product application/json
// @Param data body models.Language true "data"
// @Success 201
// @Router /admin/api/languages [post]
// @Security Bearer
func (*Language) Create(*gin.Context) {}

// Update 更新Language
// @Summary 更新Language
// @Description 更新Language
// @Tags language
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Param data body models.Language true "data"
// @Success 200
// @Router /admin/api/languages/{id} [put]
// @Security Bearer
func (*Language) Update(*gin.Context) {}

// Get 获取Language
// @Summary 获取Language
// @Description 获取Language
// @Tags language
// @Param id path string true "id"
// @Success 200 {object} models.Language
// @Router /admin/api/languages/{id} [get]
// @Security Bearer
func (*Language) Get(*gin.Context) {}

// List Language列表数据
// @Summary Language列表数据
// @Description Language列表数据
// @Tags language
// @Accept  application/json
// @Product application/json
// @Param name query string false "name"
// @Param status query string false "status"
// @Param page query int false "page"
// @Param pageSize query int false "pageSize"
// @Success 200 {object} response.Page{data=[]models.Language}
// @Router /admin/api/languages [get]
// @Security Bearer
func (*Language) List(*gin.Context) {}

// Delete 删除Language
// @Summary 删除Language
// @Description 删除Language
// @Tags language
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/languages/{id} [delete]
// @Security Bearer
func (*Language) Delete(*gin.Context) {}
