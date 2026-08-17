package apis

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
	runtimechallenge "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/challenge"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/11 17:36:55
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/11 17:36:55
 */

func init() {
	e := &AppConfig{
		Simple: controller.NewSimple(),
	}
	response.AppendController(e)
}

type AppConfig struct {
	*controller.Simple
	service  service.AppConfig
	enforcer appConfigSecretEnforcer
}

type appConfigSecretEnforcer interface {
	Enforce(rvals ...interface{}) (bool, error)
}

const (
	appConfigSecretReadPath  = "/app-config/secrets/read"
	appConfigSecretWritePath = "/app-config/secrets/write"
	emailChallengeReadyLimit = 150 * time.Millisecond
)

var storageAppConfigKeyRemovedError = response.NewError(
	"STORAGE_PROFILE_APP_CONFIG_FORBIDDEN",
	"storage provider and credential settings must come from the startup profile",
)

func (e *AppConfig) GetAction(string) response.Action {
	return nil
}

func (e *AppConfig) Other(r *gin.RouterGroup) {
	r.GET("/app-configs/:group", response.AuthHandler, e.Group)
	r.PUT("/app-configs/:group", response.AuthHandler, e.Control)
	r.DELETE("/app-configs/theme", response.AuthHandler, e.Reset)
	r.GET("/app-configs/profile", middleware.OptionalAuth(), e.Profile)
}

// Profile 获取应用配置
// @Summary 获取应用配置
// @Description 获取应用配置
// @Tags app-config
// @Accept application/json
// @Produce application/json
// @Success 200 {object} dto.AppConfigPublicProfile
// @Router /admin/api/app-configs/profile [get]
// @Security Bearer
func (e *AppConfig) Profile(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	api := response.Make(ctx)
	// The profile endpoint bootstraps the login page as well as authenticated
	// pages. Authentication must not broaden this response: privileged callers
	// use the authorized group endpoint when they need the complete settings.
	profile, err := e.service.Profile(ctx)
	if err != nil {
		api.AddError(err).Log.Error("get app config profile error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(projectEmailChallengeReadiness(profile, emailChallengeReady(ctx)))
}

// emailChallengeReady is evaluated for every public profile response. The
// rest of the profile may come from the versioned Redis cache, but runtime
// dependency health must never be persisted in that cache or reused after a
// failed check.
func emailChallengeReady(ctx *gin.Context) bool {
	if ctx == nil {
		return false
	}
	appConfig := center.GetAppConfig()
	if appConfig == nil {
		return false
	}
	host, hostOK := appConfig.GetAppConfig(ctx, "email:smtpHost")
	portValue, portOK := appConfig.GetAppConfig(ctx, "email:smtpPort")
	username, usernameOK := appConfig.GetAppConfig(ctx, "email:username")
	password, passwordOK := appConfig.GetAppConfig(ctx, "email:password")
	port, portErr := strconv.Atoi(strings.TrimSpace(portValue))
	if !hostOK || !portOK || !usernameOK || !passwordOK ||
		strings.TrimSpace(host) == "" || strings.TrimSpace(username) == "" || password == "" ||
		portErr != nil || port < 1 || port > 65535 {
		return false
	}
	challenge := center.GetRuntimeChallenge()
	if challenge == nil {
		return false
	}
	requestCtx := context.Background()
	if ctx.Request != nil {
		requestCtx = ctx.Request.Context()
	}
	readyCtx, cancel := context.WithTimeout(requestCtx, emailChallengeReadyLimit)
	defer cancel()
	return challenge.Ready(readyCtx) == nil
}

var _ center.RuntimeChallengeImp = (*runtimechallenge.Redis)(nil)

func projectEmailChallengeReadiness(profile map[string]gin.H, ready bool) map[string]gin.H {
	result := make(map[string]gin.H, len(profile)+1)
	for group, values := range profile {
		result[group] = values
	}
	security := make(gin.H, len(profile["security"])+1)
	for name, value := range profile["security"] {
		security[name] = value
	}
	security["emailChallengeReady"] = ready
	result["security"] = security
	return result
}

// Group 应用配置分组
// @Summary 应用配置分组
// @Description 应用配置分组
// @Tags app-config
// @Accept application/json
// @Produce application/json,application/vnd.mss.theme.v1+json
// @Param group path string true "group"
// @Success 200 {object} map[string]interface{} "Theme returns the canonical versioned resource; other groups return key/value objects, with credential fields omitted unless app-config:secret-read is granted"
// @Header 200 {string} ETag "Strong theme resource ETag when group=theme"
// @Failure 503 {object} response.Response "Credential authorization policy is unavailable"
// @Router /admin/api/app-configs/{group} [get]
// @Security Bearer
func (e *AppConfig) Group(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	api := response.Make(ctx)
	req := &dto.AppConfigGroupRequest{}
	if err := api.Bind(req).Error; err != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if req.Group == service.ThemeConfigGroup {
		result, err := e.service.ThemeResource(ctx)
		if err != nil {
			api.AddError(err).Log.Error("get application theme resource error")
			api.Err(http.StatusInternalServerError)
			return
		}
		writeThemeResource(ctx, result)
		return
	}
	includeSensitive := false
	if service.AppConfigGroupContainsSensitiveValues(req.Group) {
		var err error
		includeSensitive, err = e.canAccessSensitiveValues(ctx, appConfigSecretReadPath)
		if err != nil {
			api.AddError(err).Log.Error("authorize sensitive application config read")
			api.Err(http.StatusServiceUnavailable)
			return
		}
	}
	result, err := e.service.GroupWithSensitiveValues(ctx, req.Group, includeSensitive)
	if err != nil {
		if errors.Is(err, service.ErrThemeGroupCaseMismatch) ||
			errors.Is(err, service.ErrAppConfigKeyCaseMismatch) {
			api.AddError(err).Err(http.StatusUnprocessableEntity)
			return
		}
		api.AddError(err).Log.Error("get app config group error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(result)
}

// Control 应用配置控制
// @Summary 应用配置控制
// @Description 应用配置控制
// @Tags app-config
// @Accept application/json
// @Produce application/json,application/vnd.mss.theme.v1+json
// @Param group path string true "group"
// @Param If-Match header string true "Strong application theme ETag used for optimistic concurrency"
// @Param data body dto.AppConfigControlRequest true "data"
// @Success 200 {object} map[string]interface{} "Theme returns the canonical versioned resource; other groups return an empty object"
// @Header 200 {string} ETag "Strong theme resource ETag when group=theme"
// @Failure 412 {object} dto.ThemeRevisionConflictResponse "Theme revision conflict"
// @Failure 403 {object} response.Response "Credential fields require app-config:secret-write"
// @Failure 422 {object} response.Response "Storage provider and credential fields are not AppConfig settings"
// @Failure 503 {object} response.Response "Credential authorization policy is unavailable"
// @Header 412 {string} ETag "Current strong application theme ETag"
// @Router /admin/api/app-configs/{group} [put]
// @Security Bearer
func (e *AppConfig) Control(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	api := response.Make(ctx)
	req := &dto.AppConfigControlRequest{
		Data: make(map[string]any),
	}
	if err := api.Bind(req).Error; err != nil {
		if req.Group == service.ThemeConfigGroup {
			middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
				Scope: service.ThemeScopeApplication, ChangedKeys: submittedThemeKeys(req.Data), Outcome: "binding_error",
			})
		}
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if req.Group == service.ThemeConfigGroup {
		keys := submittedThemeKeys(req.Data)
		expectedRevision, err := parseThemeIfMatch(ctx, service.ThemeScopeApplication)
		if err != nil {
			middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
				Scope: service.ThemeScopeApplication, ChangedKeys: keys, Outcome: "bad_precondition",
			})
			status, code := http.StatusBadRequest, themeIfMatchInvalidCode
			if errors.Is(err, errThemeIfMatchRequired) {
				status, code = http.StatusPreconditionRequired, themeIfMatchRequiredCode
			}
			api.AddError(response.NewError(code, err.Error())).Err(status)
			return
		}
		result, err := e.service.UpdateTheme(ctx, req.Data, expectedRevision)
		if err != nil {
			if errors.Is(err, service.ErrInvalidThemePatch) {
				middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
					Scope: service.ThemeScopeApplication, ChangedKeys: keys, Outcome: "validation_failed",
				})
				api.AddError(err).Err(http.StatusUnprocessableEntity)
				return
			}
			var conflict *service.ThemeRevisionConflictError
			if errors.As(err, &conflict) {
				middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
					Scope: service.ThemeScopeApplication, ChangedKeys: keys, Outcome: "conflict",
					Revision: conflict.Current.Meta.Revision,
				})
				writeThemeRevisionConflict(ctx, err)
				return
			}
			middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
				Scope: service.ThemeScopeApplication, ChangedKeys: keys, Outcome: "database_error",
			})
			api.AddError(err).Log.Error("update application theme error")
			api.Err(http.StatusInternalServerError)
			return
		}
		middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
			Scope: service.ThemeScopeApplication, ChangedKeys: keys, Outcome: "success",
			Revision: result.Meta.Revision,
		})
		writeThemeResource(ctx, result)
		return
	}
	if service.AppConfigMutationContainsSensitiveValues(req.Group, req.Data) {
		if middleware.GetVerify(ctx) == nil {
			api.Err(http.StatusUnauthorized)
			return
		}
		allowed, err := e.canAccessSensitiveValues(ctx, appConfigSecretWritePath)
		if err != nil {
			api.AddError(err).Log.Error("authorize sensitive application config write")
			api.Err(http.StatusServiceUnavailable)
			return
		}
		if !allowed {
			api.Err(http.StatusForbidden)
			return
		}
	}
	err := e.service.CreateOrUpdate(ctx, req.Group, req.Data)
	if err != nil {
		if errors.Is(err, service.ErrAppConfigKeyNotAllowed) {
			api.AddError(storageAppConfigKeyRemovedError).Err(http.StatusUnprocessableEntity)
			return
		}
		if errors.Is(err, service.ErrInvalidThemePatch) ||
			errors.Is(err, service.ErrAppConfigKeyCaseMismatch) ||
			errors.Is(err, service.ErrThemeGroupCaseMismatch) {
			if errors.Is(err, service.ErrThemeGroupCaseMismatch) {
				middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
					Scope: service.ThemeScopeApplication, ChangedKeys: submittedThemeKeys(req.Data), Outcome: "validation_failed",
				})
			}
			api.AddError(err).Err(http.StatusUnprocessableEntity)
			return
		}
		api.AddError(err).Log.Error("update app config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(struct{}{})
}

// Reset removes every explicit override in an application configuration group.
// Theme is the only group with reset semantics; removing an override restores
// the code-level default.
// @Summary Reset application theme overrides
// @Tags app-config
// @Produce application/json,application/vnd.mss.theme.v1+json
// @Param If-Match header string true "Strong application theme ETag used for optimistic concurrency"
// @Success 200 {object} dto.ThemeResource "Returns the canonical versioned resource"
// @Header 200 {string} ETag "Strong application theme resource ETag"
// @Failure 412 {object} dto.ThemeRevisionConflictResponse "Theme revision conflict"
// @Header 412 {string} ETag "Current strong application theme ETag"
// @Router /admin/api/app-configs/theme [delete]
// @Security Bearer
func (e *AppConfig) Reset(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	api := response.Make(ctx)
	keys := service.ThemeFieldNames()
	expectedRevision, err := parseThemeIfMatch(ctx, service.ThemeScopeApplication)
	if err != nil {
		middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
			Scope: service.ThemeScopeApplication, ChangedKeys: keys, Outcome: "bad_precondition",
		})
		status, code := http.StatusBadRequest, themeIfMatchInvalidCode
		if errors.Is(err, errThemeIfMatchRequired) {
			status, code = http.StatusPreconditionRequired, themeIfMatchRequiredCode
		}
		api.AddError(response.NewError(code, err.Error())).Err(status)
		return
	}
	result, err := e.service.ResetTheme(ctx, expectedRevision)
	if err != nil {
		var conflict *service.ThemeRevisionConflictError
		if errors.As(err, &conflict) {
			middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
				Scope: service.ThemeScopeApplication, ChangedKeys: keys, Outcome: "conflict",
				Revision: conflict.Current.Meta.Revision,
			})
			writeThemeRevisionConflict(ctx, err)
			return
		}
		middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
			Scope: service.ThemeScopeApplication, ChangedKeys: keys, Outcome: "database_error",
		})
		api.AddError(err).Log.Error("reset app config error")
		api.Err(http.StatusInternalServerError)
		return
	}
	middleware.SetThemeAuditMetadata(ctx, middleware.ThemeAuditMetadata{
		Scope: service.ThemeScopeApplication, ChangedKeys: keys, Outcome: "success",
		Revision: result.Meta.Revision,
	})
	writeThemeResource(ctx, result)
}

func (e *AppConfig) canAccessSensitiveValues(ctx *gin.Context, path string) (bool, error) {
	verify := middleware.GetVerify(ctx)
	if verify == nil {
		return false, nil
	}
	if verify.Root() {
		return true, nil
	}
	roleID := strings.TrimSpace(verify.GetRoleID())
	if roleID == "" {
		return false, nil
	}
	enforcer := e.enforcer
	if enforcer == nil {
		enforcer = gormdb.Enforcer
	}
	if enforcer == nil {
		return false, errors.New("application config secret permission enforcer is unavailable")
	}
	return enforcer.Enforce(
		roleID,
		pkg.ComponentAccessType.String(),
		path,
		http.MethodGet,
	)
}
