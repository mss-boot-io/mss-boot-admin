package apis

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
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
			controller.WithAfterDelete(LanguageDeleteCache),
			controller.WithAfterUpdate(LanguageAddCache),
			controller.WithAfterCommitCreate(LanguageAddCache),
		),
	}
	response.AppendController(e)
}

type Language struct {
	*controller.Simple
}

func (e *Language) Other(r *gin.RouterGroup) {
	r.GET("/language/profile", e.Profile)
	r.GET("/languages/public", e.PublicList)
}

// PublicList 获取公开语言列表
// @Summary 获取公开语言列表
// @Description 获取公开语言列表（无需认证）
// @Tags language
// @Accept application/json
// @Product application/json
// @Success 200 {object} []models.Language
// @Router /admin/api/languages/public [get]
func (e *Language) PublicList(ctx *gin.Context) {
	api := response.Make(ctx)
	items := make([]*models.Language, 0)
	err := center.GetDB(ctx, &models.Language{}).Find(&items).Error
	if err != nil {
		api.AddError(err).Err(http.StatusInternalServerError)
		return
	}
	api.OK(items)
}

// Profile 获取语言配置
// @Summary 获取语言配置
// @Description 获取语言配置
// @Tags language
// @Accept application/json
// @Product application/json
// @Success 200 {object} map[string]map[string]string
// @Router /admin/api/language/profile [get]
func (e *Language) Profile(ctx *gin.Context) {
	api := response.Make(ctx)
	items := make([]*models.Language, 0)
	resp := make(map[string]map[string]string)
	var cacheGeneration int64
	cacheReady := false
	if center.GetCache() != nil {
		cached, generation, hit, err := pkg.LoadLanguageProfileCache(ctx, center.GetCache())
		if err != nil {
			slog.Error("load language profile cache error", "error", err)
		} else {
			cacheGeneration = generation
			cacheReady = true
			if hit {
				api.OK(map[string]map[string]string(cached))
				return
			}
		}
	}
	err := center.GetDB(ctx, &models.Language{}).Find(&items).Error
	if err != nil {
		api.AddError(err).Log.Error("get languages error")
		api.Err(http.StatusInternalServerError)
		return
	}
	for i := range items {
		if items[i].Defines == nil || len(*items[i].Defines) == 0 {
			continue
		}
		for j := range *items[i].Defines {
			if resp[items[i].Name] == nil {
				resp[items[i].Name] = make(map[string]string)
			}
			resp[items[i].Name][(*items[i].Defines)[j].Group+"."+(*items[i].Defines)[j].Key] =
				(*items[i].Defines)[j].Value
		}
	}
	api.OK(resp)

	// An empty profile is still a complete snapshot. Caching it prevents an
	// empty or freshly initialized deployment from querying the language table
	// on every request until the first definition is created.
	if cacheReady {
		if _, err := pkg.StoreLanguageProfileCache(
			ctx,
			center.GetCache(),
			cacheGeneration,
			pkg.LanguageProfile(resp),
		); err != nil {
			slog.Error("set language profile cache error", "error", err)
		}
	}
}

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

func LanguageDeleteCache(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error {
	if center.GetCache() == nil {
		return nil
	}
	if db == nil {
		return nil
	}
	err := pkg.InvalidateLanguageCache(ctx, center.GetCache())
	if err != nil {
		slog.Error("delete language cache error", "error", err)
	}
	return nil
}

func LanguageAddCache(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error {
	if center.GetCache() == nil {
		return nil
	}
	if db == nil {
		return nil
	}
	err := pkg.InvalidateLanguageCache(ctx, center.GetCache())
	if err != nil {
		slog.Error("add language cache error", "error", err)
	}
	return nil
}
