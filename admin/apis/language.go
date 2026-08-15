package apis

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/dbresolver"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/12 11:54:56
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/12 11:54:56
 */

const (
	maxLanguageRecords          = 64
	maxLanguagePageSize         = 100
	maxLanguagePage             = 1_000_000
	maxLanguagePublicPayload    = 1 * 1024 * 1024
	maxLanguageWriteRequestSize = models.MaxLanguageDefinitionsSize + 64*1024
)

var (
	errLanguageCapacity         = errors.New("language capacity reached")
	errLanguageDataInvalid      = errors.New("stored language data is invalid")
	errLanguageRevisionConflict = errors.New("language revision changed")
	errLanguageNameUnavailable  = errors.New("language name unavailable")
)

func init() {
	response.AppendController(newLanguageController())
}

func newLanguageController() *Language {
	return &Language{
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
}

type Language struct {
	*controller.Simple
}

func (e *Language) Other(r *gin.RouterGroup) {
	r.GET("/language/profile", e.Profile)
	r.GET("/languages/public", e.PublicList)
	r.GET("/languages", response.AuthHandler, e.ManagedList)
	r.GET("/languages/:id", response.AuthHandler, e.ManagedGet)
	r.POST("/languages", response.AuthHandler, e.ManagedCreate)
	r.PUT("/languages/:id", response.AuthHandler, e.ManagedUpdate)
}

func (e *Language) GetAction(key string) response.Action {
	if key == response.Get || key == response.Search || key == response.Control {
		return nil
	}
	return e.Simple.GetAction(key)
}

// PublicList 获取公开语言列表
// @Summary 获取公开语言列表
// @Description 获取公开语言列表（无需认证）
// @Tags language
// @Accept application/json
// @Product application/json
// @Success 200 {array} dto.PublicLanguage
// @Router /admin/api/languages/public [get]
func (e *Language) PublicList(ctx *gin.Context) {
	api := response.Make(ctx)
	items := make([]models.Language, 0)
	err := center.GetDB(ctx, &models.Language{}).
		Where("status = ?", "enabled").
		Order("name ASC").
		Limit(maxLanguageRecords + 1).
		Find(&items).Error
	if err != nil {
		languageAPIError(ctx, http.StatusInternalServerError, "LANGUAGE_READ_FAILED", "language resources are unavailable")
		return
	}
	if len(items) > maxLanguageRecords {
		languageAPIError(ctx, http.StatusServiceUnavailable, "LANGUAGE_CAPACITY_EXCEEDED", "language resources exceed the supported limit")
		return
	}
	result := make([]dto.PublicLanguage, 0, len(items))
	for i := range items {
		if !validateStoredLanguage(ctx, &items[i]) {
			return
		}
		defines := models.LanguageDefines{}
		if items[i].Defines != nil {
			defines = append(defines, (*items[i].Defines)...)
		}
		result = append(result, dto.PublicLanguage{Name: items[i].Name, Defines: defines})
	}
	if !validateLanguagePublicPayload(ctx, result) {
		return
	}
	api.OK(result)
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
	err := center.GetDB(ctx, &models.Language{}).
		Where("status = ?", "enabled").
		Order("name ASC").
		Limit(maxLanguageRecords + 1).
		Find(&items).Error
	if err != nil {
		slog.Error("get language profile error", "error", err)
		languageAPIError(ctx, http.StatusInternalServerError, "LANGUAGE_READ_FAILED", "language resources are unavailable")
		return
	}
	if len(items) > maxLanguageRecords {
		languageAPIError(ctx, http.StatusServiceUnavailable, "LANGUAGE_CAPACITY_EXCEEDED", "language resources exceed the supported limit")
		return
	}
	for i := range items {
		if !validateStoredLanguage(ctx, items[i]) {
			return
		}
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
	if !validateLanguagePublicPayload(ctx, resp) {
		return
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

// ManagedList returns a bounded management page. `view=summary` deliberately
// omits the potentially large definitions JSON used only by the detail/editor.
// @Summary 获取语言管理列表
// @Description 返回有界分页；view=summary 时不返回 defines
// @Tags language
// @Produce application/json
// @Param name query string false "name"
// @Param status query string false "status" Enums(enabled,disabled)
// @Param view query string false "view" Enums(summary,full)
// @Param current query int false "current"
// @Param pageSize query int false "pageSize" maximum(100)
// @Success 200 {object} response.Page{data=[]dto.LanguageSummary}
// @Router /admin/api/languages [get]
// @Security Bearer
func (e *Language) ManagedList(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.LanguageSearch{}
	if api.Bind(req).Error != nil {
		languageAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_LANGUAGE_QUERY", "language query is invalid")
		return
	}
	page, pageSize := req.GetPage(), req.GetPageSize()
	if page > maxLanguagePage || pageSize > maxLanguagePageSize {
		languageAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_LANGUAGE_QUERY", "language pagination is outside the supported range")
		return
	}
	base := func() *gorm.DB {
		query := center.GetDB(ctx, &models.Language{}).Model(&models.Language{})
		if name := strings.TrimSpace(req.Name); name != "" {
			escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(name)
			query = query.Where("name LIKE ? ESCAPE '!'", "%"+escaped+"%")
		}
		if req.Status != "" {
			query = query.Where("status = ?", req.Status)
		}
		return query
	}
	var count int64
	if err := base().Count(&count).Error; err != nil {
		languageAPIError(ctx, http.StatusInternalServerError, "LANGUAGE_READ_FAILED", "language records are unavailable")
		return
	}
	query := base().
		Order("updated_at DESC").
		Order("id ASC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize))
	if req.View == "summary" {
		records := make([]models.Language, 0, pageSize)
		if err := query.Select("id", "name", "remark", "status", "updated_at").Find(&records).Error; err != nil {
			languageAPIError(ctx, http.StatusInternalServerError, "LANGUAGE_READ_FAILED", "language records are unavailable")
			return
		}
		items := make([]dto.LanguageSummary, 0, len(records))
		for i := range records {
			if !validateStoredLanguage(ctx, &records[i]) {
				return
			}
			items = append(items, dto.LanguageSummary{
				ID:        records[i].ID,
				Name:      records[i].Name,
				Remark:    records[i].Remark,
				Status:    records[i].Status.String(),
				UpdatedAt: records[i].UpdatedAt,
			})
		}
		api.PageOK(items, count, page, pageSize)
		return
	}
	items := make([]models.Language, 0, pageSize)
	if err := query.Find(&items).Error; err != nil {
		languageAPIError(ctx, http.StatusInternalServerError, "LANGUAGE_READ_FAILED", "language records are unavailable")
		return
	}
	for i := range items {
		if !validateStoredLanguage(ctx, &items[i]) {
			return
		}
	}
	api.PageOK(items, count, page, pageSize)
}

// ManagedGet returns one complete, bounded language resource.
// @Summary 获取语言详情
// @Description 返回完整 defines；损坏或超出资源上限的数据失败关闭
// @Tags language
// @Produce application/json
// @Param id path string true "id"
// @Success 200 {object} models.Language
// @Router /admin/api/languages/{id} [get]
// @Security Bearer
func (e *Language) ManagedGet(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" || len(id) > 64 {
		languageAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_LANGUAGE", "language identifier is invalid")
		return
	}
	var item models.Language
	err := center.GetDB(ctx, &models.Language{}).First(&item, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		languageAPIError(ctx, http.StatusNotFound, "LANGUAGE_NOT_FOUND", "language was not found")
		return
	}
	if err != nil {
		slog.Error("get managed language error", "languageID", id, "error", err)
		languageAPIError(ctx, http.StatusInternalServerError, "LANGUAGE_READ_FAILED", "language record is unavailable")
		return
	}
	if !validateStoredLanguage(ctx, &item) {
		return
	}
	response.Make(ctx).OK(&item)
}

// ManagedCreate creates a language from an allowlisted, client-owned payload.
// @Summary 创建语言
// @Description ID、时间戳和租户元数据由服务端生成
// @Tags language
// @Accept application/json
// @Produce application/json
// @Param data body dto.LanguageWriteRequest true "data"
// @Success 201 {object} models.Language
// @Router /admin/api/languages [post]
// @Security Bearer
func (e *Language) ManagedCreate(ctx *gin.Context) {
	request, ok := bindLanguageWriteRequest(ctx)
	if !ok {
		return
	}
	language := &models.Language{
		Name:    request.Name,
		Remark:  request.Remark,
		Status:  enumStatus(request.Status),
		Defines: cloneLanguageDefines(request.Defines, true),
	}
	if language.Status == "" {
		language.Status = "enabled"
	}
	if err := language.NormalizeAndValidate(); err != nil {
		writeLanguageError(ctx, err)
		return
	}
	db := center.GetDB(ctx, language).Clauses(dbresolver.Write)
	err := db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.Language{}).Count(&count).Error; err != nil {
			return err
		}
		if count >= maxLanguageRecords {
			return errLanguageCapacity
		}
		var duplicate int64
		if err := tx.Model(&models.Language{}).Where("name = ?", language.Name).Count(&duplicate).Error; err != nil {
			return err
		}
		if duplicate > 0 {
			return errLanguageNameUnavailable
		}
		return tx.Create(language).Error
	})
	if err != nil {
		writeLanguageError(ctx, err)
		return
	}
	_ = LanguageAddCache(ctx, db, language)
	response.Make(ctx).OK(language)
}

// ManagedUpdate updates a language and supports an optimistic revision check.
// @Summary 更新语言
// @Description expectedUpdatedAt 不匹配时返回 LANGUAGE_REVISION_CONFLICT
// @Tags language
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Param data body dto.LanguageWriteRequest true "data"
// @Success 200 {object} models.Language
// @Router /admin/api/languages/{id} [put]
// @Security Bearer
func (e *Language) ManagedUpdate(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" || len(id) > 64 {
		languageAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_LANGUAGE", "language identifier is invalid")
		return
	}
	request, ok := bindLanguageWriteRequest(ctx)
	if !ok {
		return
	}
	var expectedUpdatedAt *time.Time
	if request.ExpectedUpdatedAt != "" {
		parsed, err := time.Parse(time.RFC3339Nano, request.ExpectedUpdatedAt)
		if err != nil {
			languageAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_LANGUAGE", "language revision is invalid")
			return
		}
		expectedUpdatedAt = &parsed
	}
	db := center.GetDB(ctx, &models.Language{}).Clauses(dbresolver.Write)
	var updated models.Language
	err := db.Transaction(func(tx *gorm.DB) error {
		var current models.Language
		if err := tx.First(&current, "id = ?", id).Error; err != nil {
			return err
		}
		if err := current.ValidateStored(); err != nil {
			slog.Error("update stored language is invalid", "languageID", current.ID, "error", err)
			return errors.Join(errLanguageDataInvalid, err)
		}
		if expectedUpdatedAt != nil && !current.UpdatedAt.Equal(*expectedUpdatedAt) {
			return errLanguageRevisionConflict
		}
		next := current
		next.Name = request.Name
		next.Remark = request.Remark
		if request.Status != "" {
			next.Status = enumStatus(request.Status)
		}
		if request.Defines != nil {
			next.Defines = cloneLanguageDefines(request.Defines, false)
			if err := validateLanguageDefinitionOwnership(current.Defines, next.Defines); err != nil {
				return err
			}
		}
		if err := next.NormalizeAndValidate(); err != nil {
			return err
		}
		var duplicate int64
		if err := tx.Model(&models.Language{}).
			Where("name = ? AND id <> ?", next.Name, current.ID).
			Count(&duplicate).Error; err != nil {
			return err
		}
		if duplicate > 0 {
			return errLanguageNameUnavailable
		}
		now := time.Now().UTC()
		minimumNextRevision := current.UpdatedAt.Add(time.Millisecond)
		if now.Before(minimumNextRevision) {
			now = minimumNextRevision
		}
		query := tx.Session(&gorm.Session{SkipHooks: true}).
			Model(&models.Language{}).
			Where("id = ? AND updated_at = ?", current.ID, current.UpdatedAt)
		result := query.Updates(map[string]any{
			"name":       next.Name,
			"remark":     next.Remark,
			"status":     next.Status,
			"defines":    next.Defines,
			"updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errLanguageRevisionConflict
		}
		return tx.Clauses(dbresolver.Write).First(&updated, "id = ?", current.ID).Error
	})
	if err != nil {
		writeLanguageError(ctx, err)
		return
	}
	_ = LanguageAddCache(ctx, db, &updated)
	response.Make(ctx).OK(&updated)
}

func bindLanguageWriteRequest(ctx *gin.Context) (*dto.LanguageWriteRequest, bool) {
	if ctx.Request.ContentLength > maxLanguageWriteRequestSize {
		languageAPIError(ctx, http.StatusRequestEntityTooLarge, "LANGUAGE_PAYLOAD_TOO_LARGE", "language payload is too large")
		return nil, false
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxLanguageWriteRequestSize)
	request := &dto.LanguageWriteRequest{}
	api := response.Make(ctx).Bind(request)
	if api.Error != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(api.Error, &maxBytesError) {
			languageAPIError(ctx, http.StatusRequestEntityTooLarge, "LANGUAGE_PAYLOAD_TOO_LARGE", "language payload is too large")
			return nil, false
		}
		languageAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_LANGUAGE", "language payload is invalid")
		return nil, false
	}
	return request, true
}

func enumStatus(value string) enum.Status {
	return enum.Status(strings.TrimSpace(value))
}

func writeLanguageError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, models.ErrLanguageInvalid):
		languageAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_LANGUAGE", "language payload is invalid")
	case errors.Is(err, errLanguageCapacity):
		languageAPIError(ctx, http.StatusConflict, "LANGUAGE_CAPACITY_REACHED", "language capacity has been reached")
	case errors.Is(err, errLanguageDataInvalid):
		languageAPIError(ctx, http.StatusServiceUnavailable, "LANGUAGE_DATA_INVALID", "language data is invalid")
	case errors.Is(err, errLanguageNameUnavailable), isLanguageUniqueConstraintError(err):
		languageAPIError(ctx, http.StatusConflict, "LANGUAGE_NAME_UNAVAILABLE", "language name is unavailable")
	case errors.Is(err, errLanguageRevisionConflict):
		languageAPIError(ctx, http.StatusConflict, "LANGUAGE_REVISION_CONFLICT", "language changed concurrently")
	case errors.Is(err, gorm.ErrRecordNotFound):
		languageAPIError(ctx, http.StatusNotFound, "LANGUAGE_NOT_FOUND", "language was not found")
	default:
		languageAPIError(ctx, http.StatusInternalServerError, "LANGUAGE_WRITE_FAILED", "language could not be saved")
	}
}

func isLanguageUniqueConstraintError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "error 1062") ||
		strings.Contains(message, "idx_mss_boot_languages_name")
}

func cloneLanguageDefines(
	definitions *models.LanguageDefines,
	resetIDs bool,
) *models.LanguageDefines {
	if definitions == nil {
		return nil
	}
	clone := make(models.LanguageDefines, 0, len(*definitions))
	for _, definition := range *definitions {
		if definition == nil {
			clone = append(clone, nil)
			continue
		}
		item := *definition
		if resetIDs {
			item.ID = ""
		}
		clone = append(clone, &item)
	}
	return &clone
}

func validateLanguageDefinitionOwnership(
	current *models.LanguageDefines,
	next *models.LanguageDefines,
) error {
	currentIDs := make(map[string]struct{})
	if current != nil {
		for _, definition := range *current {
			if definition != nil {
				currentIDs[definition.ID] = struct{}{}
			}
		}
	}
	if next == nil {
		return nil
	}
	for _, definition := range *next {
		if definition == nil || strings.TrimSpace(definition.ID) == "" {
			continue
		}
		if _, exists := currentIDs[strings.TrimSpace(definition.ID)]; !exists {
			return fmt.Errorf("%w: definition ID is not owned by the language", models.ErrLanguageInvalid)
		}
	}
	return nil
}

func validateStoredLanguage(ctx *gin.Context, language *models.Language) bool {
	if err := language.ValidateStored(); err != nil {
		slog.Error("stored language is invalid", "languageID", language.ID, "error", err)
		languageAPIError(ctx, http.StatusServiceUnavailable, "LANGUAGE_DATA_INVALID", "language data is invalid")
		return false
	}
	return true
}

func validateLanguagePublicPayload(ctx *gin.Context, value any) bool {
	payload, err := json.Marshal(value)
	if err != nil {
		slog.Error("encode language public payload error", "error", err)
		languageAPIError(ctx, http.StatusServiceUnavailable, "LANGUAGE_DATA_INVALID", "language data is invalid")
		return false
	}
	if len(payload) > maxLanguagePublicPayload {
		languageAPIError(ctx, http.StatusServiceUnavailable, "LANGUAGE_CAPACITY_EXCEEDED", "language resources exceed the supported limit")
		return false
	}
	return true
}

func languageAPIError(ctx *gin.Context, status int, code, message string) {
	api := response.Make(ctx)
	api.Error = response.NewError(code, message)
	api.Err(status)
}

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
