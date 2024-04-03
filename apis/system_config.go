package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/models"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/20 17:52:05
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/20 17:52:05
 */

func init() {
	e := &SystemConfig{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.SystemConfig)),
			controller.WithSearch(new(dto.SystemConfigSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type SystemConfig struct {
	*controller.Simple
}

// Create 创建系统配置
// @Summary 创建系统配置
// @Description 创建系统配置
// @Tags system_config
// @Accept application/json
// @Produce application/json
// @Param data body models.SystemConfig true "data"
// @Success 201 {object} models.SystemConfig
// @Router /admin/api/system-configs [post]
// @Security Bearer
func (*SystemConfig) Create(*gin.Context) {}

// Update 更新系统配置
// @Summary 更新系统配置
// @Description 更新系统配置
// @Tags system_config
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Param data body models.SystemConfig true "data"
// @Success 200 {object} models.SystemConfig
// @Router /admin/api/system-configs/{id} [put]
// @Security Bearer
func (*SystemConfig) Update(*gin.Context) {}

// Delete 删除系统配置
// @Summary 删除系统配置
// @Description 删除系统配置
// @Tags system_config
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/system-configs/{id} [delete]
// @Security Bearer
func (*SystemConfig) Delete(*gin.Context) {}

// Get 获取系统配置
// @Summary 获取系统配置
// @Description 获取系统配置
// @Tags system_config
// @Param id path string true "id"
// @Success 200 {object} models.SystemConfig
// @Router /admin/api/system-configs/{id} [get]
// @Security Bearer
func (*SystemConfig) Get(*gin.Context) {}

// List 系统配置列表数据
// @Summary 系统配置列表数据
// @Description 系统配置列表数据
// @Tags system_config
// @Accept application/json
// @Produce application/json
// @Param page query int false "page"
// @Param pageSize query int false "pageSize"
// @Success 200 {object} response.Page{data=[]models.SystemConfig}
// @Router /admin/api/system-configs [get]
// @Security Bearer
func (*SystemConfig) List(*gin.Context) {}
