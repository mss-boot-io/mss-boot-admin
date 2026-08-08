package apis

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
)

const (
	roleDeletePoliciesConflictCode = "ROLE_DELETE_HAS_POLICIES"
	roleDeleteUsersConflictCode    = "ROLE_DELETE_HAS_USERS"
)

func init() {
	e := &Role{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Role)),
			controller.WithSearch(new(dto.RoleSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
			// The GORM controller uses one control-handler chain for POST and
			// PUT, so this chain protects both create and update.
			controller.WithCreateHandlers(gin.HandlersChain{requireRootManagement, protectManagedRoleLifecycle}),
			controller.WithDeleteHandlers(gin.HandlersChain{requireRootManagement, protectManagedRoleLifecycle}),
		),
	}
	response.AppendController(e)
}

type Role struct {
	*controller.Simple
}

// protectManagedRoleLifecycle keeps the immutable root and provisioning-role
// classifications outside generic CRUD. Their status or existence may only be
// changed by a dedicated invariant-aware workflow, never by an ordinary role
// update/delete request.
func protectManagedRoleLifecycle(ctx *gin.Context) {
	if ctx.Request == nil || (ctx.Request.Method != http.MethodPut && ctx.Request.Method != http.MethodDelete) {
		ctx.Next()
		return
	}
	roleID := strings.TrimSpace(ctx.Param("id"))
	if roleID == "" {
		ctx.Abort()
		response.Make(ctx).Err(http.StatusUnprocessableEntity)
		return
	}
	if ctx.Request.Method == http.MethodDelete {
		err := service.AuthorizationPolicies.DeleteRole(
			ctx,
			center.Default.GetDB(ctx, &models.Role{}),
			roleID,
		)
		ctx.Abort()
		switch {
		case err == nil:
			response.Make(ctx).OK(nil)
		case errors.Is(err, service.ErrAuthorizationRoleNotFound):
			response.Make(ctx).Err(http.StatusNotFound)
		case errors.Is(err, service.ErrAuthorizationManagedRole):
			response.Make(ctx).Err(http.StatusForbidden)
		case errors.Is(err, service.ErrAuthorizationRolePolicies):
			response.Make(ctx).AddError(response.NewError(
				roleDeletePoliciesConflictCode,
				"clear the canonical role authorization resource before deleting the role",
			)).Err(http.StatusConflict)
		case errors.Is(err, service.ErrAuthorizationRoleUsers):
			response.Make(ctx).AddError(response.NewError(
				roleDeleteUsersConflictCode,
				"move all users to another role before deleting the role",
			)).Err(http.StatusConflict)
		default:
			response.Make(ctx).AddError(err).Log.Error("delete ordinary role", "err", err)
			response.Make(ctx).Err(http.StatusInternalServerError)
		}
		return
	}
	var role models.Role
	err := center.Default.GetDB(ctx, &models.Role{}).
		Select("id", "root", "default").
		Where("id = ?", roleID).
		Take(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ctx.Abort()
		response.Make(ctx).Err(http.StatusNotFound)
		return
	}
	if err != nil {
		ctx.Abort()
		response.Make(ctx).AddError(err).Log.Error("load managed role lifecycle flags", "err", err)
		response.Make(ctx).Err(http.StatusInternalServerError)
		return
	}
	if role.Root || role.Default {
		ctx.Abort()
		response.Make(ctx).Err(http.StatusForbidden)
		return
	}
	ctx.Next()
}

func (e *Role) Other(r *gin.RouterGroup) {
	r.POST("/role/authorize/:roleID", middleware.Auth.MiddlewareFunc(), requireRootManagement, e.SetAuthorize)
	r.GET("/role/authorize/:roleID", middleware.Auth.MiddlewareFunc(), e.GetAuthorize)
}

// GetAuthorize 获取角色授权
// @Summary 获取角色授权
// @Description 获取角色授权
// @Tags role
// @Accept  application/json
// @Produce application/json
// @param roleID path string true "roleID"
// @Success 200 {object} dto.GetAuthorizeResponse
// @Header 200 {string} ETag "Strong role-authorization ETag"
// @Failure 404 {object} response.Error "Role not found"
// @Router /admin/api/role/authorize/{roleID} [get]
// @Security Bearer
func (e *Role) GetAuthorize(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.GetAuthorizeRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	resource, err := service.AuthorizationPolicies.ReadRole(
		ctx,
		center.Default.GetDB(ctx, &models.CasbinRule{}),
		req.RoleID,
	)
	if err != nil {
		if errors.Is(err, service.ErrAuthorizationRoleNotFound) {
			api.Err(http.StatusNotFound)
			return
		}
		api.AddError(err).Log.Error("read role authorization resource error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	setRoleAuthorizationETag(ctx, resource)
	api.OK(resource)
}

// SetAuthorize 角色授权
// @Summary 角色授权
// @Description 给角色授权
// @Tags role
// @Accept application/json
// @Produce application/json
// @param roleID path string true "roleID"
// @Param If-Match header string false "Strong role-authorization ETag; optional only during the rolling compatibility window"
// @Param data body dto.SetAuthorizeRequest true "data"
// @Success 200 {object} dto.GetAuthorizeResponse
// @Header 200 {string} ETag "Strong role-authorization ETag"
// @Header 200 {string} Warning "Compatibility warning when If-Match is omitted"
// @Header 200 {string} X-MSS-Authorization-Precondition "missing when the compatibility path accepted a request without If-Match"
// @Failure 400 {object} response.Error "Malformed If-Match"
// @Failure 401 {object} response.Error "Current principal missing"
// @Failure 403 {object} response.Error "Current principal is not root"
// @Failure 404 {object} response.Error "Role not found"
// @Failure 409 {object} response.Error "Role is inactive"
// @Failure 412 {object} dto.AuthorizeRevisionConflictResponse "Role authorization revision conflict"
// @Header 412 {string} ETag "Current strong role-authorization ETag"
// @Failure 422 {object} response.Error "Missing paths or inactive/unknown path"
// @Failure 503 {object} response.Error "Policy committed but local reload or watcher publication failed"
// @Router /admin/api/role/authorize/{roleID} [post]
// @Security Bearer
func (e *Role) SetAuthorize(ctx *gin.Context) {
	api := response.Make(ctx)
	if !requireCurrentRoot(ctx) {
		return
	}
	req := &dto.SetAuthorizeRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	req.RoleID = resolveAuthorizeRoleID(req.RoleID, ctx.Param("roleID"))
	if hasEmptyAuthorizeRoleID(req.RoleID) {
		api.Err(http.StatusUnprocessableEntity)
		return
	}

	paths := sanitizeAuthorizePaths(*req.Paths)
	if len(*req.Paths) > 0 && len(paths) == 0 {
		respondInvalidAuthorizeRequest(api, "set role authorize request contains no valid paths", req.RoleID, nil)
		return
	}
	expectedRevision, err := parseRoleAuthorizationIfMatch(ctx, req.RoleID)
	if err != nil {
		api.AddError(response.NewError(roleAuthorizationIfMatchInvalidCode, err.Error())).Err(http.StatusBadRequest)
		return
	}
	resource, err := service.AuthorizationPolicies.ReplaceRole(
		ctx,
		center.Default.GetDB(ctx, &models.CasbinRule{}),
		req.RoleID,
		paths,
		expectedRevision,
	)
	if err != nil {
		if errors.Is(err, service.ErrAuthorizationRoleNotFound) {
			api.Err(http.StatusNotFound)
			return
		}
		if errors.Is(err, service.ErrAuthorizationRoleInactive) {
			api.AddError(response.NewError(
				roleAuthorizationInactiveCode,
				"authorization cannot be changed while the role is inactive",
			)).Err(http.StatusConflict)
			return
		}
		var invalid *service.InvalidAuthorizationPathsError
		if errors.As(err, &invalid) {
			respondInvalidAuthorizeRequest(api, "set role authorize request contains invalid paths", req.RoleID, invalid.Paths)
			return
		}
		if writeRoleAuthorizationRevisionConflict(ctx, err) {
			return
		}
		var propagation *service.AuthorizationPropagationError
		if errors.As(err, &propagation) {
			setRoleAuthorizationETag(ctx, propagation.Current)
			api.AddError(err).Log.Error("propagate role authorization policy error", "err", err, slog.String("roleID", req.RoleID))
			api.Err(http.StatusServiceUnavailable)
			return
		}
		api.AddError(err).Log.Error("update role authorize policy error", "err", err, slog.String("roleID", req.RoleID))
		api.Err(http.StatusInternalServerError)
		return
	}
	setRoleAuthorizationETag(ctx, resource)
	if expectedRevision == nil {
		setMissingRoleAuthorizationPreconditionWarning(ctx)
	}
	api.OK(resource)
}

// Create 创建角色
// @Summary 创建角色
// @Description 创建角色
// @Tags role
// @Accept  application/json
// @Produce application/json
// @Param data body models.Role true "data"
// @Success 201 {object} models.Role
// @Router /admin/api/roles [post]
// @Security Bearer
func (e *Role) Create(*gin.Context) {}

// Delete 删除角色
// @Summary 删除角色
// @Description 删除角色
// @Tags role
// @Param id path string true "id"
// @Success 204
// @Failure 401 {object} response.Error "Current principal missing"
// @Failure 403 {object} response.Error "Current principal is not root or target is a managed role"
// @Failure 404 {object} response.Error "Role not found"
// @Failure 409 {object} response.Error "Role still has Casbin policies or user assignments"
// @Router /admin/api/roles/{id} [delete]
// @Security Bearer
func (e *Role) Delete(*gin.Context) {}

// Update 更新角色
// @Summary 更新角色
// @Description 更新角色
// @Tags role
// @Accept  application/json
// @Produce application/json
// @Param id path string true "id"
// @Param data body models.Role true "data"
// @Success 200 {object} models.Role
// @Router /admin/api/roles/{id} [put]
// @Security Bearer
func (e *Role) Update(*gin.Context) {}

// Get 获取角色
// @Summary 获取角色
// @Description 获取角色
// @Tags role
// @Param id path string true "id"
// @Success 200 {object} models.Role
// @Router /admin/api/roles/{id} [get]
// @Security Bearer
func (e *Role) Get(*gin.Context) {}

// List 角色列表
// @Summary 角色列表
// @Description 角色列表
// @Tags role
// @Accept  application/json
// @Produce application/json
// @Param current query int false "current"
// @Param pageSize query int false "pageSize"
// @Param id query string false "id"
// @Param name query string false "name"
// @Param status query string false "status"
// @Param remark query string false "remark"
// @Success 200 {object} response.Page{data=[]models.Role}
// @Router /admin/api/roles [get]
// @Security Bearer
func (e *Role) List(*gin.Context) {}
