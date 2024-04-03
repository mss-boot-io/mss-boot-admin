package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/8 18:14:12
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/8 18:14:12
 */

func init() {
	e := &Tenant{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Tenant)),
			controller.WithSearch(new(dto.TenantSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Tenant struct {
	*controller.Simple
}

// Create 创建租户
// @Summary 创建租户
// @Description 创建租户
// @Tags tenant
// @Accept  application/json
// @Product application/json
// @param data body models.Tenant true "data"
// @Success 201 {object} models.Tenant
// @Router /admin/api/tenants [post]
// @Security Bearer
func (e *Tenant) Create(*gin.Context) {}

// Update 更新租户
// @Summary 更新租户
// @Description 更新租户
// @Tags tenant
// @Accept  application/json
// @Product application/json
// @param id path string true "id"
// @param data body models.Tenant true "data"
// @Success 200 {object} models.Tenant
// @Router /admin/api/tenants/{id} [put]
// @Security Bearer
func (e *Tenant) Update(*gin.Context) {}

// Get 获取租户
// @Summary 获取租户
// @Description 获取租户
// @Tags tenant
// @param id path string true "id"
// @Param preloads query []string false "preloads"
// @Success 200 {object} models.Tenant
// @Router /admin/api/tenants/{id} [get]
// @Security Bearer
func (e *Tenant) Get(*gin.Context) {}

// List 租户列表
// @Summary 租户列表
// @Description 租户列表
// @Tags tenant
// @Accept  application/json
// @Product application/json
// @Param page query int false "page"
// @Param pageSize query int false "pageSize"
// @Param id query string false "id"
// @Param name query string false "name"
// @Param status query string false "status"
// @Success 200 {object} response.Page{data=[]models.Tenant}
// @Router /admin/api/tenants [get]
// @Security Bearer
func (e *Tenant) List(*gin.Context) {}

// Delete 删除租户
// @Summary 删除租户
// @Description 删除租户
// @Tags tenant
// @param id path string true "id"
// @Success 204
// @Router /admin/api/tenants/{id} [delete]
// @Security Bearer
func (e *Tenant) Delete(*gin.Context) {}
