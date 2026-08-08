package apis

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/2 00:41:41
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/2 00:41:41
 */

func init() {
	e := &UserConfig{
		Simple: controller.NewSimple(),
	}
	response.AppendController(e)
}

type UserConfig struct {
	*controller.Simple
	service service.UserConfig
}

func (e *UserConfig) GetAction(string) response.Action {
	return nil
}

func (e *UserConfig) Other(r *gin.RouterGroup) {
	r.GET("/user-configs/:group", response.AuthHandler, e.Group)
	r.PUT("/user-configs/:group", response.AuthHandler, e.Control)
	r.DELETE("/user-configs/theme", response.AuthHandler, e.Reset)
	r.GET("/user-configs/profile", middleware.OptionalAuth(), e.Profile)
}

// Profile 用户配置
// @Summary 用户配置
// @Description 用户配置
// @Tags user-config
// @Accept application/json
// @Produce application/json
// @Success 200 {object} map[string]map[string]string
// @Router /admin/api/user-configs/profile [get]
// @Security Bearer
func (e *UserConfig) Profile(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	api := response.Make(ctx)
	verify := response.VerifyHandler(ctx)
	if verify == nil {
		api.OK(nil)
		return
	}
	result, err := e.service.Profile(ctx, verify.GetUserID())
	if err != nil {
		api.AddError(err).Log.Error("get user config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(result)
}

// Group 用户配置分组
// @Summary 用户配置分组
// @Description 用户配置分组
// @Tags user-config
// @Accept application/json
// @Produce application/json,application/vnd.mss.theme.v1+json
// @Param group path string true "group"
// @Param Accept header string false "Use application/vnd.mss.theme.v1+json for the canonical versioned theme resource; omitted keeps the legacy user-theme response"
// @Success 200 {object} map[string]interface{} "Theme may return the negotiated canonical resource; other groups and legacy theme clients receive key/value objects"
// @Header 200 {string} ETag "Strong theme resource ETag when group=theme"
// @Router /admin/api/user-configs/{group} [get]
// @Security Bearer
func (e *UserConfig) Group(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	req := &dto.UserConfigGroupRequest{}
	if err := api.Bind(req).Error; err != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if req.Group == service.ThemeConfigGroup {
		canonicalContract := wantsCanonicalThemeContract(ctx)
		resource, err := e.service.ThemeResource(ctx, verify.GetUserID())
		if err != nil {
			api.AddError(err).Log.Error("get user theme resource error")
			api.Err(http.StatusInternalServerError)
			return
		}
		if canonicalContract {
			writeThemeResource(ctx, resource)
			return
		}
		result, err := e.service.LegacyThemeGroup(ctx, verify.GetUserID(), resource)
		if err != nil {
			api.AddError(err).Log.Error("get legacy user theme group error")
			api.Err(http.StatusInternalServerError)
			return
		}
		setThemeETag(ctx, resource)
		api.OK(result)
		return
	}
	result, err := e.service.Group(ctx, verify.GetUserID(), req.Group)
	if err != nil {
		if errors.Is(err, service.ErrThemeGroupCaseMismatch) ||
			errors.Is(err, service.ErrUserConfigKeyCaseMismatch) {
			api.AddError(err).Err(http.StatusUnprocessableEntity)
			return
		}
		api.AddError(err).Log.Error("get user config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(result)
}

// Control 用户配置控制
// @Summary 用户配置控制
// @Description 用户配置控制
// @Tags user-config
// @Accept application/json
// @Produce application/json,application/vnd.mss.theme.v1+json
// @Param group path string true "group"
// @Param Accept header string false "Use application/vnd.mss.theme.v1+json for the canonical seven-field theme contract"
// @Param If-Match header string false "Strong current-user theme ETag used for optimistic concurrency"
// @Param data body dto.UserConfigControlRequest true "data"
// @Success 200 {object} map[string]interface{} "Theme may return the negotiated canonical resource; other groups return an empty object"
// @Header 200 {string} ETag "Strong theme resource ETag when group=theme"
// @Failure 412 {object} dto.ThemeRevisionConflictResponse "Theme revision conflict"
// @Header 412 {string} ETag "Current strong user theme ETag"
// @Router /admin/api/user-configs/{group} [put]
// @Security Bearer
func (e *UserConfig) Control(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	req := &dto.UserConfigControlRequest{}
	if err := api.Bind(req).Error; err != nil {
		if req.Group == service.ThemeConfigGroup {
			middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
				Scope: service.ThemeScopeUser, ChangedKeys: submittedThemeKeys(req.Data), Outcome: "binding_error",
			})
		}
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if req.Group == service.ThemeConfigGroup {
		canonicalContract := wantsCanonicalThemeContract(ctx)
		keys := submittedThemeKeys(req.Data)
		expectedRevision, err := parseThemeIfMatch(ctx, service.ThemeScopeUser)
		if err != nil {
			middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
				Scope: service.ThemeScopeUser, ChangedKeys: keys, Outcome: "bad_precondition",
			})
			api.AddError(response.NewError(themeIfMatchInvalidCode, err.Error())).Err(http.StatusBadRequest)
			return
		}
		result, err := e.service.UpdateTheme(ctx, verify.GetUserID(), req.Data, expectedRevision)
		if err != nil {
			if errors.Is(err, service.ErrInvalidThemePatch) {
				middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
					Scope: service.ThemeScopeUser, ChangedKeys: keys, Outcome: "validation_failed",
				})
				api.AddError(err).Err(http.StatusUnprocessableEntity)
				return
			}
			var conflict *service.ThemeRevisionConflictError
			if errors.As(err, &conflict) {
				middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
					Scope: service.ThemeScopeUser, ChangedKeys: keys, Outcome: "conflict",
					Revision: conflict.Current.Meta.Revision,
				})
				writeThemeRevisionConflict(ctx, err, canonicalContract)
				return
			}
			middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
				Scope: service.ThemeScopeUser, ChangedKeys: keys, Outcome: "database_error",
			})
			api.AddError(err).Log.Error("update user theme error")
			api.Err(http.StatusInternalServerError)
			return
		}
		middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
			Scope: service.ThemeScopeUser, ChangedKeys: keys, Outcome: "success",
			Revision: result.Meta.Revision,
		})
		if !canonicalContract {
			legacy, legacyErr := e.service.LegacyThemeGroup(ctx, verify.GetUserID(), result)
			if legacyErr != nil {
				api.AddError(legacyErr).Log.Error("get legacy user theme after update error")
				api.Err(http.StatusInternalServerError)
				return
			}
			setThemeETag(ctx, result)
			api.OK(legacy)
			return
		}
		writeThemeResource(ctx, result)
		return
	}
	err := e.service.CreateOrUpdate(ctx, verify.GetUserID(), req.Group, req.Data)
	if err != nil {
		if errors.Is(err, service.ErrInvalidThemePatch) ||
			errors.Is(err, service.ErrThemeGroupCaseMismatch) ||
			errors.Is(err, service.ErrUserConfigKeyCaseMismatch) {
			if errors.Is(err, service.ErrThemeGroupCaseMismatch) {
				middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
					Scope: service.ThemeScopeUser, ChangedKeys: submittedThemeKeys(req.Data), Outcome: "validation_failed",
				})
			}
			api.AddError(err).Err(http.StatusUnprocessableEntity)
			return
		}
		api.AddError(err).Log.Error("control user config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(struct{}{})
}

// Reset removes every explicit override in the current user's configuration
// group. The authenticated identity is the only source of the user ID.
// @Summary Reset current user theme overrides
// @Tags user-config
// @Produce application/json,application/vnd.mss.theme.v1+json
// @Param Accept header string false "Use application/vnd.mss.theme.v1+json for the canonical versioned theme resource"
// @Param If-Match header string false "Strong current-user theme ETag used for optimistic concurrency"
// @Success 200 {object} map[string]interface{} "Returns the negotiated canonical resource or the legacy theme projection"
// @Header 200 {string} ETag "Strong current-user theme resource ETag"
// @Failure 412 {object} dto.ThemeRevisionConflictResponse "Theme revision conflict"
// @Header 412 {string} ETag "Current strong user theme ETag"
// @Router /admin/api/user-configs/theme [delete]
// @Security Bearer
func (e *UserConfig) Reset(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	api := response.Make(ctx)
	canonicalContract := wantsCanonicalThemeContract(ctx)
	verify := middleware.GetVerify(ctx)
	keys := service.ThemeFieldNames()
	expectedRevision, err := parseThemeIfMatch(ctx, service.ThemeScopeUser)
	if err != nil {
		middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
			Scope: service.ThemeScopeUser, ChangedKeys: keys, Outcome: "bad_precondition",
		})
		api.AddError(response.NewError(themeIfMatchInvalidCode, err.Error())).Err(http.StatusBadRequest)
		return
	}
	result, err := e.service.ResetTheme(ctx, verify.GetUserID(), expectedRevision)
	if err != nil {
		var conflict *service.ThemeRevisionConflictError
		if errors.As(err, &conflict) {
			middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
				Scope: service.ThemeScopeUser, ChangedKeys: keys, Outcome: "conflict",
				Revision: conflict.Current.Meta.Revision,
			})
			writeThemeRevisionConflict(ctx, err, canonicalContract)
			return
		}
		middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
			Scope: service.ThemeScopeUser, ChangedKeys: keys, Outcome: "database_error",
		})
		api.AddError(err).Log.Error("reset user config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
		Scope: service.ThemeScopeUser, ChangedKeys: keys, Outcome: "success",
		Revision: result.Meta.Revision,
	})
	if !canonicalContract {
		legacy, legacyErr := e.service.LegacyThemeGroup(ctx, verify.GetUserID(), result)
		if legacyErr != nil {
			api.AddError(legacyErr).Log.Error("get legacy user theme after reset error")
			api.Err(http.StatusInternalServerError)
			return
		}
		setThemeETag(ctx, result)
		api.OK(legacy)
		return
	}
	writeThemeResource(ctx, result)
}
