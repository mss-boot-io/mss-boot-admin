package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/app/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/app/admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/24 01:47:02
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/24 01:47:02
 */

func init() {
	e := &API{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.API)),
			controller.WithSearch(new(dto.APISearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type API struct {
	*controller.Simple
}

// Create 创建API
// @Summary 创建API
// @Description 创建API
// @Tags api
// @Accept application/json
// @Accept application/json
// @Param data body models.API true "data"
// @Success 201 {object} models.API
// @Router /admin/api/apis [post]
// @Security Bearer
func (e *API) Create(*gin.Context) {}

// Update 更新API
// @Summary 更新API
// @Description 更新API
// @Tags api
// @Accept application/json
// @Accept application/json
// @Param id path string true "id"
// @Param data body models.API true "data"
// @Success 200 {object} models.API
// @Router /admin/api/apis/{id} [put]
// @Security Bearer
func (e *API) Update(*gin.Context) {}

// Delete 删除API
// @Summary 删除API
// @Description 删除API
// @Tags api
// @Accept application/json
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/apis/{id} [delete]
// @Security Bearer
func (e *API) Delete(*gin.Context) {}

// Get 获取API
// @Summary 获取API
// @Description 获取API
// @Tags api
// @Accept application/json
// @Param id path string true "id"
// @Success 200 {object} models.API
// @Router /admin/api/apis/{id} [get]
// @Security Bearer
func (e *API) Get(*gin.Context) {}

// List API列表数据
// @Summary API列表数据
// @Description API列表数据
// @Tags api
// @Accept application/json
// @Accept application/json
// @Param page query int false "page"
// @Param pageSize query int false "pageSize"
// @Success 200 {object} response.Page{data=[]models.API}
// @Router /admin/api/apis [get]
// @Security Bearer
func (e *API) List(*gin.Context) {}
