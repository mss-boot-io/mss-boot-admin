package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/29 21:55:50
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/29 21:55:50
 */

func init() {
	e := &Field{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Field)),
			controller.WithSearch(new(dto.FieldSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Field struct {
	*controller.Simple
}

// Create 创建字段
// @Summary 创建字段
// @Description 创建字段
// @Tags field
// @Accept application/json
// @Product application/json
// @Param data body models.Field true "data"
// @Success 201 {object} models.Field
// @Router /admin/api/fields [post]
// @Security Bearer
func (e *Field) Create(*gin.Context) {}

// Update 更新字段
// @Summary 更新字段
// @Description 更新字段
// @Tags field
// @Accept application/json
// @Product application/json
// @Param id path string true "id"
// @Param data body models.Field true "data"
// @Success 200 {object} models.Field
// @Router /admin/api/fields/{id} [put]
// @Security Bearer
func (e *Field) Update(*gin.Context) {}

// Delete 删除字段
// @Summary 删除字段
// @Description 删除字段
// @Tags field
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/fields/{id} [delete]
// @Security Bearer
func (e *Field) Delete(*gin.Context) {}

// Get 获取字段
// @Summary 获取字段
// @Description 获取字段
// @Tags field
// @Param id path string true "id"
// @Success 200 {object} models.Field
// @Router /admin/api/fields/{id} [get]
// @Security Bearer
func (e *Field) Get(*gin.Context) {}

// List 字段列表
// @Summary 字段列表
// @Description 字段列表
// @Tags field
// @Accept application/json
// @Product application/json
// @Param page query int false "page"
// @Param pageSize query int false "pageSize"
// @Param modelID query string false "modelID"
// @Success 200 {object} response.Page{data=[]models.Field}
// @Router /admin/api/fields [get]
// @Security Bearer
func (e *Field) List(*gin.Context) {}
