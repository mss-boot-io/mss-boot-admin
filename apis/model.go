package apis

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/18 12:58:01
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/18 12:58:01
 */

func init() {
	e := &Model{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Model)),
			controller.WithSearch(new(dto.ModelSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Model struct {
	*controller.Simple
}

func (e *Model) Other(r *gin.RouterGroup) {
	r.GET("/model/migrate/:id", e.Migrate)
}

// Migrate 迁移虚拟模型
// @Summary 迁移虚拟模型
// @Description 迁移虚拟模型
// @Tags model
// @Param id path string true "id"
// @Success 200
// @Router /admin/api/model/migrate/{id} [put]
// @Security Bearer
func (e *Model) Migrate(ctx *gin.Context) {
	api := response.Make(ctx)
	m := &models.Model{}
	err := gormdb.DB.Preload("Fields").First(m, "id = ?", ctx.Param("id")).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.Err(http.StatusNotFound)
			return
		}
	}
	//事务
	tx := gormdb.DB.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	err = tx.Model(m).Where("id = ?", m.ID).Update("migrate", true).Error
	if err != nil {
		api.AddError(err).Log.Error("update error")
		api.Err(http.StatusInternalServerError)
		return
	}
	vm := m.MakeVirtualModel()
	if vm == nil {
		api.Err(http.StatusNotFound)
		return
	}
	err = vm.Migrate(gormdb.DB)
	if err != nil {
		api.AddError(err).Log.Error("migrate error")
		api.Err(http.StatusInternalServerError)
		return
	}
	tx.Commit()
	api.OK(nil)
}

// Create 创建模型
// @Summary 创建模型
// @Description 创建模型
// @Tags model
// @Accept application/json
// @Produce application/json
// @Param data body models.Model true "data"
// @Success 201 {object} models.Model
// @Router /admin/api/models [post]
// @Security Bearer
func (e *Model) Create(*gin.Context) {}

// Update 更新模型
// @Summary 更新模型
// @Description 更新模型
// @Tags model
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Param model body models.Model true "model"
// @Success 200 {object} models.Model
// @Router /admin/api/models/{id} [put]
// @Security Bearer
func (e *Model) Update(*gin.Context) {}

// Delete 删除模型
// @Summary 删除模型
// @Description 删除模型
// @Tags model
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/models/{id} [delete]
// @Security Bearer
func (e *Model) Delete(*gin.Context) {}

// Get 获取模型
// @Summary 获取模型
// @Description 获取模型
// @Tags model
// @Param id path string true "id"
// @Success 200 {object} models.Model
// @Router /admin/api/models/{id} [get]
// @Security Bearer
func (e *Model) Get(*gin.Context) {}

// List 模型列表
// @Summary 模型列表
// @Description 模型列表
// @Tags model
// @Accept application/json
// @Produce application/json
// @Param page query int false "page"
// @Param pageSize query int false "pageSize"
// @Param preloads query []string false "preloads"
// @Success 200 {object} response.Page{data=[]models.Model}
// @Router /admin/api/models [get]
// @Security Bearer
func (e *Model) List(*gin.Context) {}
