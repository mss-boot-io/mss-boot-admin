package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/models"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/1 12:07:53
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/1 12:07:53
 */

func init() {
	e := &Option{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Option)),
			controller.WithSearch(new(dto.OptionSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
			controller.WithScope(center.Default.Scope),
		),
	}
	response.AppendController(e)
}

type Option struct {
	*controller.Simple
}

func (e *Option) Other(r *gin.RouterGroup) {
	r.Use(middleware.Auth.MiddlewareFunc())
}

// Create 创建Option
// @Summary 创建Option
// @Description 创建Option
// @Tags option
// @Accept  application/json
// @Product application/json
// @Param data body models.Option true "data"
// @Success 201 {object} models.Option
// @Router /admin/api/options [post]
// @Security Bearer
func (*Option) Create(*gin.Context) {}

// Update 更新Option
// @Summary 更新Option
// @Description 更新Option
// @Tags option
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Param data body models.Option true "data"
// @Success 200 {object} models.Option
// @Router /admin/api/options/{id} [put]
// @Security Bearer
func (*Option) Update(*gin.Context) {}

// Delete 删除Option
// @Summary 删除Option
// @Description 删除Option
// @Tags option
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/options/{id} [delete]
// @Security Bearer
func (*Option) Delete(*gin.Context) {}

// Get 获取Option
// @Summary 获取Option
// @Description 获取Option
// @Tags option
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Success 200 {object} models.Option
// @Router /admin/api/options/{id} [get]
// @Security Bearer
func (*Option) Get(*gin.Context) {}

// List Option列表数据
// @Summary Option列表数据
// @Description Option列表数据
// @Tags option
// @Accept  application/json
// @Product application/json
// @Param name query string false "name"
// @Param status query string false "status"
// @Param page query int false "page"
// @Param pageSize query int false "pageSize"
// @Success 200 {object} response.Page{data=[]models.Option}
// @Router /admin/api/options [get]
// @Security Bearer
func (*Option) List(*gin.Context) {}
