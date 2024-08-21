package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/models"
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
			controller.WithScope(center.Default.Scope),
			controller.WithNoAuthAction(response.Search, response.Get),
		),
	}
	response.AppendController(e)
}

type Language struct {
	*controller.Simple
}

//func (e *Language) GetAction(key string) response.Action {
//	if key == response.Search {
//		return nil
//	}
//	return e.Simple.GetAction(key)
//}
//
//func (e *Language) Other(r *gin.RouterGroup) {
//	search := e.Simple.GetAction(response.Search)
//	r.GET("/languages", search.Handler())
//}

// Create 创建Language
// @Summary 创建Language
// @Description 创建Language
// @Tags language
// @Accept  application/json
// @Product application/json
// @Param data body models.Language true "data"
// @Success 201 {object} models.Language
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
// @Success 200 {object} models.Language
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
// @Param current query int false "current"
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
