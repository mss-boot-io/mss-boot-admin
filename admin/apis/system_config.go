package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/20 17:52:05
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/20 17:52:05
 */

func init() {
	response.AppendController(newSystemConfigController())
}

func newSystemConfigController() *SystemConfig {
	return &SystemConfig{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.SystemConfig)),
			controller.WithSearch(new(dto.SystemConfigSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
			// SystemConfig.Content is an opaque legacy payload and may contain
			// credentials. Until it is split into typed, independently
			// authorized resources, every read and mutation stays root-only.
			controller.WithCreateHandlers(gin.HandlersChain{requireRootManagement, protectSystemConfigResponse}),
			controller.WithGetHandlers(gin.HandlersChain{requireRootManagement, protectSystemConfigResponse}),
			controller.WithDeleteHandlers(gin.HandlersChain{requireRootManagement, protectSystemConfigResponse}),
			controller.WithSearchHandlers(gin.HandlersChain{requireRootManagement, protectSystemConfigResponse}),
		),
	}
}

// protectSystemConfigResponse prevents opaque configuration payloads from
// entering either the shared query cache or an HTTP intermediary cache.
func protectSystemConfigResponse(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	// Generic GORM actions use the Gin context directly. Set the legacy key on
	// Gin as well as the typed value on Request.Context so bypass works even
	// when Gin's ContextWithFallback option is disabled.
	c.Set("gorm:cache:bypass", true)
	if c.Request != nil {
		c.Request = c.Request.WithContext(cache.NewBypass(c.Request.Context()))
	}
	c.Next()
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
// @Param current query int false "current"
// @Param pageSize query int false "pageSize"
// @Success 200 {object} response.Page{data=[]models.SystemConfig}
// @Router /admin/api/system-configs [get]
// @Security Bearer
func (*SystemConfig) List(*gin.Context) {}
