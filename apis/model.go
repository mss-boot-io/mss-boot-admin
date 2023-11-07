package apis

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions/authentic"
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
			controller.WithModelProvider(authentic.ModelProviderGorm),
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
// @Success 200
// @Router /admin/api/model/migrate/{id} [get]
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
	api.OK(nil)
}
