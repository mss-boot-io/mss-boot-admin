package apis

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
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
			controller.WithHandlers(gin.HandlersChain{protectSystemConfigResponse}),
			controller.WithCreateHandlers(gin.HandlersChain{requireRootManagement}),
			controller.WithGetHandlers(gin.HandlersChain{requireRootManagement}),
			controller.WithDeleteHandlers(gin.HandlersChain{requireRootManagement}),
			controller.WithBeforeCreate(prepareSystemConfigCreate),
			controller.WithBeforeUpdate(prepareSystemConfigUpdate),
			controller.WithBeforeDelete(validateSystemConfigDelete),
			controller.WithWriteErrorMapper(
				operationalWriteErrorMapper("SYSTEM_CONFIG", "system configuration"),
			),
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

type systemConfigSummary struct {
	ID        string        `json:"id"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
	Name      string        `json:"name"`
	Ext       source.Scheme `json:"ext"`
	Remark    string        `json:"remark"`
	BuiltIn   bool          `json:"isBuiltIn"`
}

func (e *SystemConfig) GetAction(key string) response.Action {
	if key == response.Search {
		return nil
	}
	return e.Simple.GetAction(key)
}

func (e *SystemConfig) Other(r *gin.RouterGroup) {
	r.GET(
		"/system-configs",
		response.AuthHandler,
		requireRootManagement,
		protectSystemConfigResponse,
		e.List,
	)
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
func (*SystemConfig) List(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.SystemConfigSearch{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	page, pageSize := req.GetPage(), req.GetPageSize()
	if page > 10_000 || pageSize > 100 {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	query := center.Default.GetDB(ctx, &models.SystemConfig{}).Model(&models.SystemConfig{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		api.AddError(err).Log.Error("count system configuration summaries")
		api.Err(http.StatusInternalServerError)
		return
	}
	items := make([]systemConfigSummary, 0, pageSize)
	if err := query.
		Select("id", "created_at", "updated_at", "name", "ext", "remark", "built_in").
		Order("updated_at DESC").
		Order("id DESC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Scan(&items).Error; err != nil {
		api.AddError(err).Log.Error("list system configuration summaries")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.PageOK(items, total, page, pageSize)
}
