package apis

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	bootpkg "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

const (
	maxOptionPageSize         = 100
	maxOptionPage             = 1_000_000
	maxOptionWriteRequestSize = models.MaxOptionItemsEncodedSize + 64*1024
)

var errOptionIfMatchRequired = errors.New("option mutation requires If-Match")

func init() {
	response.AppendController(newOptionController())
}

func newOptionController() *Option {
	return &Option{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Option)),
			controller.WithSearch(new(dto.OptionSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
		service: service.NewOption(),
	}
}

// Option exposes an explicit, bounded management contract. The embedded
// Simple controller supplies the canonical /options path only; generic CRUD is
// deliberately disabled because it bypasses option snapshots, cache
// invalidation, built-in protection and optimistic concurrency.
type Option struct {
	*controller.Simple
	service *service.Option
}

func (e *Option) Other(r *gin.RouterGroup) {
	r.GET("/options", response.AuthHandler, e.ManagedList)
	r.GET("/options/:id", response.AuthHandler, e.ManagedGet)
	r.POST("/options", response.AuthHandler, e.ManagedCreate)
	r.PUT("/options/:id", response.AuthHandler, e.ManagedUpdate)
	r.DELETE("/options/:id", response.AuthHandler, e.ManagedDelete)
}

func (*Option) GetAction(string) response.Action { return nil }

func (e *Option) optionService() *service.Option {
	if e != nil && e.service != nil {
		return e.service
	}
	return service.NewOption()
}

// ManagedList returns the sole bounded V6 summary page. Item JSON and the full
// description are loaded only through the detail resource.
// @Summary 获取选项管理列表
// @Description 返回不包含 items 和 description 的有界摘要分页
// @Tags option
// @Produce application/json
// @Param name query string false "name"
// @Param category query string false "category"
// @Param status query string false "status" Enums(enabled,disabled)
// @Param current query int false "current"
// @Param pageSize query int false "pageSize" maximum(100)
// @Success 200 {object} response.Page{data=[]dto.OptionSummary}
// @Router /admin/api/options [get]
// @Security Bearer
func (e *Option) ManagedList(ctx *gin.Context) {
	request := &dto.OptionSearch{}
	if response.Make(ctx).Bind(request).Error != nil {
		optionAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_OPTION_QUERY", "option query is invalid")
	}
	page, pageSize := request.GetPage(), request.GetPageSize()
	if page > maxOptionPage || pageSize > maxOptionPageSize {
		optionAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_OPTION_QUERY", "option pagination is outside the supported range")
		return
	}

	base := func() *gorm.DB {
		query := center.GetDB(ctx, &models.Option{}).Model(&models.Option{})
		if name := strings.TrimSpace(request.Name); name != "" {
			escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(name)
			query = query.Where("name LIKE ? ESCAPE '!'", "%"+escaped+"%")
		}
		if category := strings.TrimSpace(request.Category); category != "" {
			query = query.Where("category = ?", category)
		}
		if request.Status != "" {
			query = query.Where("status = ?", request.Status)
		}
		return query
	}

	var count int64
	if err := base().Count(&count).Error; err != nil {
		optionAPIError(ctx, http.StatusInternalServerError, "OPTION_READ_FAILED", "option records are unavailable")
		return
	}
	query := base().
		Order("updated_at DESC").
		Order("id ASC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize))
	ctx.Header("Cache-Control", "no-store")

	records := make([]models.Option, 0, pageSize)
	if err := query.Select(
		"id", "category", "display_name", "name", "remark", "status", "version", "built_in", "updated_at",
	).Find(&records).Error; err != nil {
		optionAPIError(ctx, http.StatusInternalServerError, "OPTION_READ_FAILED", "option records are unavailable")
		return
	}
	items := make([]dto.OptionSummary, 0, len(records))
	for i := range records {
		if err := validateOptionSummary(&records[i]); err != nil {
			slog.Error("stored option summary is invalid", "optionID", records[i].ID, "error", err)
			optionAPIError(ctx, http.StatusServiceUnavailable, "OPTION_DATA_INVALID", "option data is invalid")
			return
		}
		items = append(items, dto.OptionSummary{
			ID:          records[i].ID,
			Category:    records[i].Category,
			DisplayName: records[i].DisplayName,
			Name:        records[i].Name,
			Remark:      records[i].Remark,
			Status:      records[i].Status.String(),
			Version:     records[i].Version,
			BuiltIn:     records[i].BuiltIn,
			UpdatedAt:   records[i].UpdatedAt,
		})
	}
	response.Make(ctx).PageOK(items, count, page, pageSize)
}

// ManagedGet returns one complete option resource with a strong revision ETag.
// @Summary 获取选项详情
// @Tags option
// @Produce application/json
// @Param id path string true "id"
// @Success 200 {object} models.Option
// @Header 200 {string} ETag "Strong option revision ETag"
// @Router /admin/api/options/{id} [get]
// @Security Bearer
func (e *Option) ManagedGet(ctx *gin.Context) {
	id, ok := optionPathID(ctx)
	if !ok {
		return
	}
	var item models.Option
	err := center.GetDB(ctx, &models.Option{}).
		Clauses(dbresolver.Write).
		First(&item, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		optionAPIError(ctx, http.StatusNotFound, "OPTION_NOT_FOUND", "option was not found")
		return
	}
	if err != nil {
		optionAPIError(ctx, http.StatusInternalServerError, "OPTION_READ_FAILED", "option record is unavailable")
		return
	}
	if !validateStoredOption(ctx, &item) {
		return
	}
	setOptionETag(ctx, &item)
	response.Make(ctx).OK(&item)
}

// ManagedCreate creates a custom option from an allowlisted client surface.
// Record identity, built-in state, revision and item identifiers are owned by
// the service.
// @Summary 创建自定义选项
// @Tags option
// @Accept application/json
// @Produce application/json
// @Param data body dto.OptionWriteRequest true "data"
// @Success 201 {object} models.Option
// @Router /admin/api/options [post]
// @Security Bearer
func (e *Option) ManagedCreate(ctx *gin.Context) {
	actorID, ok := optionActorID(ctx)
	if !ok {
		return
	}
	request, ok := bindOptionWriteRequest(ctx)
	if !ok {
		return
	}
	option := &models.Option{
		Category:    optionString(request.Category, "system"),
		DisplayName: optionString(request.DisplayName, ""),
		Description: optionString(request.Description, ""),
		Name:        optionString(request.Name, ""),
		Remark:      optionString(request.Remark, ""),
		Items:       request.Items,
		Status:      enum.Status(optionString(request.Status, enum.Enabled.String())),
	}
	created, err := e.optionService().CreateOption(ctx, option, actorID, request.ChangeNote)
	if err != nil {
		writeOptionError(ctx, err)
		return
	}
	setOptionETag(ctx, created)
	response.Make(ctx).OK(created)
}

// ManagedUpdate updates an option through a required optimistic revision check.
// @Summary 更新选项
// @Tags option
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Param If-Match header string true "Strong option ETag"
// @Param data body dto.OptionWriteRequest true "data"
// @Success 200 {object} models.Option
// @Failure 412 {object} dto.OptionRevisionConflictResponse
// @Router /admin/api/options/{id} [put]
// @Security Bearer
func (e *Option) ManagedUpdate(ctx *gin.Context) {
	id, ok := optionPathID(ctx)
	if !ok {
		return
	}
	actorID, ok := optionActorID(ctx)
	if !ok {
		return
	}
	request, ok := bindOptionWriteRequest(ctx)
	if !ok {
		return
	}
	expectedVersion, err := resolveOptionExpectedVersion(ctx, id)
	if err != nil {
		status, code := http.StatusBadRequest, "OPTION_IF_MATCH_INVALID"
		if errors.Is(err, errOptionIfMatchRequired) {
			status, code = http.StatusPreconditionRequired, "OPTION_IF_MATCH_REQUIRED"
		}
		optionAPIError(ctx, status, code, err.Error())
		return
	}
	input, err := optionUpdateInput(request, expectedVersion)
	if err != nil {
		optionAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_OPTION", "option payload is invalid")
		return
	}
	updated, err := e.optionService().UpdateOptionResource(ctx, id, input, actorID, request.ChangeNote)
	if err != nil {
		if writeOptionRevisionConflict(ctx, err) {
			return
		}
		writeOptionError(ctx, err)
		return
	}
	setOptionETag(ctx, updated)
	response.Make(ctx).OK(updated)
}

// ManagedDelete deletes a custom, unused option with an optimistic revision
// check. Built-in dictionaries are immutable resources and cannot be deleted.
// @Summary 删除自定义选项
// @Tags option
// @Param id path string true "id"
// @Param If-Match header string true "Strong option ETag"
// @Success 204
// @Failure 412 {object} dto.OptionRevisionConflictResponse
// @Router /admin/api/options/{id} [delete]
// @Security Bearer
func (e *Option) ManagedDelete(ctx *gin.Context) {
	id, ok := optionPathID(ctx)
	if !ok {
		return
	}
	actorID, ok := optionActorID(ctx)
	if !ok {
		return
	}
	expectedVersion, err := resolveOptionExpectedVersion(ctx, id)
	if err != nil {
		status, code := http.StatusBadRequest, "OPTION_IF_MATCH_INVALID"
		if errors.Is(err, errOptionIfMatchRequired) {
			status, code = http.StatusPreconditionRequired, "OPTION_IF_MATCH_REQUIRED"
		}
		optionAPIError(ctx, status, code, err.Error())
		return
	}
	_, err = e.optionService().DeleteOption(ctx, id, expectedVersion, actorID, "delete")
	if err != nil {
		if writeOptionRevisionConflict(ctx, err) {
			return
		}
		writeOptionError(ctx, err)
		return
	}
	response.Make(ctx).OK(nil)
}

func optionPathID(ctx *gin.Context) (string, bool) {
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" || len(id) > 64 {
		optionAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_OPTION", "option identifier is invalid")
		return "", false
	}
	return id, true
}

func optionActorID(ctx *gin.Context) (string, bool) {
	verify := middleware.GetVerify(ctx)
	if verify == nil || strings.TrimSpace(verify.GetUserID()) == "" {
		optionAPIError(ctx, http.StatusUnauthorized, "OPTION_ACTOR_REQUIRED", "authenticated option actor is required")
		return "", false
	}
	actorID := strings.TrimSpace(verify.GetUserID())
	if len(actorID) > 64 {
		optionAPIError(ctx, http.StatusForbidden, "OPTION_ACTOR_INVALID", "authenticated option actor is invalid")
		return "", false
	}
	return actorID, true
}

func bindOptionWriteRequest(ctx *gin.Context) (*dto.OptionWriteRequest, bool) {
	if ctx.Request.ContentLength > maxOptionWriteRequestSize {
		optionAPIError(ctx, http.StatusRequestEntityTooLarge, "OPTION_PAYLOAD_TOO_LARGE", "option payload is too large")
		return nil, false
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxOptionWriteRequestSize)
	request := &dto.OptionWriteRequest{}
	api := response.Make(ctx).Bind(request)
	if api.Error != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(api.Error, &maxBytesError) {
			optionAPIError(ctx, http.StatusRequestEntityTooLarge, "OPTION_PAYLOAD_TOO_LARGE", "option payload is too large")
			return nil, false
		}
		optionAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_OPTION", "option payload is invalid")
		return nil, false
	}
	return request, true
}

func optionUpdateInput(request *dto.OptionWriteRequest, expectedVersion int) (service.OptionUpdateInput, error) {
	input := service.OptionUpdateInput{
		Category:        request.Category,
		DisplayName:     request.DisplayName,
		Description:     request.Description,
		Name:            request.Name,
		Remark:          request.Remark,
		Items:           request.Items,
		ExpectedVersion: expectedVersion,
	}
	if request.Status != nil {
		value := enum.Status(strings.TrimSpace(*request.Status))
		if value != enum.Enabled && value != enum.Disabled {
			return service.OptionUpdateInput{}, models.ErrOptionInvalid
		}
		input.Status = &value
	}
	return input, nil
}

func optionString(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

func optionETag(id string, version int) string {
	return strconv.Quote(fmt.Sprintf("option-%s-v%d", id, version))
}

func setOptionETag(ctx *gin.Context, option *models.Option) {
	if ctx == nil || option == nil {
		return
	}
	ctx.Header("ETag", optionETag(option.ID, option.Version))
	ctx.Header("Cache-Control", "no-store")
}

func parseOptionIfMatch(ctx *gin.Context, id string) (int, error) {
	if ctx == nil || ctx.Request == nil {
		return 0, errors.New("option request is missing")
	}
	values := ctx.Request.Header.Values("If-Match")
	if len(values) == 0 {
		return 0, errOptionIfMatchRequired
	}
	if len(values) != 1 {
		return 0, errors.New("expected one strong option ETag")
	}
	raw := strings.TrimSpace(values[0])
	if raw == "" || strings.HasPrefix(raw, "W/") || strings.Contains(raw, ",") || raw == "*" {
		return 0, errors.New("expected one strong option ETag")
	}
	value, err := strconv.Unquote(raw)
	if err != nil {
		return 0, errors.New("expected a quoted strong option ETag")
	}
	prefix := "option-" + id + "-v"
	if !strings.HasPrefix(value, prefix) {
		return 0, errors.New("option ETag does not match this resource")
	}
	versionText := strings.TrimPrefix(value, prefix)
	if versionText == "" || strings.Trim(versionText, "0123456789") != "" {
		return 0, errors.New("option ETag version must be a positive integer")
	}
	version64, err := strconv.ParseInt(versionText, 10, 32)
	if err != nil || version64 <= 0 {
		return 0, errors.New("option ETag version is out of range")
	}
	version := int(version64)
	if raw != optionETag(id, version) {
		return 0, errors.New("expected the canonical option ETag")
	}
	return version, nil
}

func resolveOptionExpectedVersion(ctx *gin.Context, id string) (int, error) {
	return parseOptionIfMatch(ctx, id)
}

func writeOptionRevisionConflict(ctx *gin.Context, err error) bool {
	var conflict *service.OptionRevisionConflictError
	if !errors.As(err, &conflict) || conflict.Current == nil {
		return false
	}
	setOptionETag(ctx, conflict.Current)
	ctx.AbortWithStatusJSON(http.StatusPreconditionFailed, dto.OptionRevisionConflictResponse{
		Success:      false,
		Status:       "error",
		Code:         http.StatusPreconditionFailed,
		ErrorCode:    "OPTION_REVISION_CONFLICT",
		ErrorMessage: "option changed since it was loaded",
		TraceID:      bootpkg.GenerateMsgIDFromContext(ctx),
		Data: dto.OptionRevisionConflictResponseData{
			Current: conflict.Current,
		},
	})
	return true
}

func writeOptionError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrOptionDataInvalid):
		optionAPIError(ctx, http.StatusServiceUnavailable, "OPTION_DATA_INVALID", "option data is invalid")
	case errors.Is(err, models.ErrOptionInvalid):
		optionAPIError(ctx, http.StatusUnprocessableEntity, "INVALID_OPTION", "option payload is invalid")
	case errors.Is(err, service.ErrOptionCapacity):
		optionAPIError(ctx, http.StatusConflict, "OPTION_CAPACITY_REACHED", "option capacity has been reached")
	case errors.Is(err, service.ErrOptionNameUnavailable), isOptionUniqueConstraintError(err):
		optionAPIError(ctx, http.StatusConflict, "OPTION_NAME_UNAVAILABLE", "option name is unavailable")
	case errors.Is(err, service.ErrOptionBuiltIn):
		optionAPIError(ctx, http.StatusConflict, "OPTION_BUILT_IN_PROTECTED", "built-in option identity, status and deletion are protected")
	case errors.Is(err, service.ErrOptionInUse):
		optionAPIError(ctx, http.StatusConflict, "OPTION_IN_USE", "option is currently in use")
	case errors.Is(err, gorm.ErrRecordNotFound):
		optionAPIError(ctx, http.StatusNotFound, "OPTION_NOT_FOUND", "option was not found")
	default:
		slog.Error("option write failed", "error", err)
		optionAPIError(ctx, http.StatusInternalServerError, "OPTION_WRITE_FAILED", "option could not be saved")
	}
}

func isOptionUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "error 1062") ||
		strings.Contains(message, "idx_name")
}

func validateStoredOption(ctx *gin.Context, option *models.Option) bool {
	if err := option.ValidateStored(); err != nil {
		slog.Error("stored option is invalid", "optionID", option.ID, "error", err)
		optionAPIError(ctx, http.StatusServiceUnavailable, "OPTION_DATA_INVALID", "option data is invalid")
		return false
	}
	return true
}

func validateOptionSummary(option *models.Option) error {
	if option == nil || strings.TrimSpace(option.ID) == "" || len(option.ID) > 64 {
		return errors.New("option ID is invalid")
	}
	if option.Category == "" || option.Category != strings.TrimSpace(option.Category) ||
		utf8.RuneCountInString(option.Category) > models.MaxOptionCategoryLength {
		return errors.New("option category is invalid")
	}
	if option.Name == "" || option.Name != strings.TrimSpace(option.Name) ||
		utf8.RuneCountInString(option.Name) > models.MaxOptionNameLength {
		return errors.New("option name is invalid")
	}
	if option.DisplayName != strings.TrimSpace(option.DisplayName) ||
		utf8.RuneCountInString(option.DisplayName) > models.MaxOptionDisplayNameLength {
		return errors.New("option display name is invalid")
	}
	if option.Remark != strings.TrimSpace(option.Remark) || utf8.RuneCountInString(option.Remark) > models.MaxOptionRemarkLength {
		return errors.New("option remark is invalid")
	}
	if option.Status != enum.Enabled && option.Status != enum.Disabled {
		return errors.New("option status is invalid")
	}
	if option.Version <= 0 {
		return errors.New("option version is invalid")
	}
	return nil
}

func optionAPIError(ctx *gin.Context, status int, code, message string) {
	api := response.Make(ctx)
	api.Error = response.NewError(code, message)
	api.Err(status)
}

// The standard action methods remain documentation anchors for generators and
// Swagger tooling. Runtime registration is exclusively handled by Other.
func (*Option) Create(*gin.Context) {}
func (*Option) Update(*gin.Context) {}
func (*Option) Delete(*gin.Context) {}
func (*Option) Get(*gin.Context)    {}
func (*Option) List(*gin.Context)   {}
