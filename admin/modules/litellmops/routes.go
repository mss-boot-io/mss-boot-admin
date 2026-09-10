package litellmops

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var ErrOperationDisabled = errors.New("litellmops operation is disabled for this release")

// registerBusinessRoutes mounts the module routes below the protected /api
// group. Every route is bound to a casbin-enforced permission.
func registerBusinessRoutes(group *gin.RouterGroup, runtime business.Runtime) error {
	if group == nil {
		return errors.New("litellmops protected route group is required")
	}
	authorizer, err := NewAdminAuthorizer(runtime.RequestDatabase, runtime.Principal)
	if err != nil {
		return err
	}
	handler := &requestHandler{runtime: runtime, authorizer: authorizer}

	resource := group.Group("/litellmops")
	resource.GET("/users", handler.secure(PermissionUserList, handler.listUsers))
	resource.POST("/users", handler.secure(PermissionUserWrite, handler.createUser))
	resource.POST("/users/invite", handler.secure(PermissionUserWrite, handler.inviteUser))
	resource.GET("/users/:id", handler.secure(PermissionUserRead, handler.getUser))
	resource.PATCH("/users/:id", handler.secure(PermissionUserWrite, handler.updateUser))
	resource.POST("/users/:id/block", handler.secure(PermissionUserWrite, handler.blockUser))
	resource.POST("/users/:id/unblock", handler.secure(PermissionUserWrite, handler.unblockUser))
	resource.DELETE("/users/:id", handler.secure(PermissionUserWrite, handler.deleteUser))
	resource.GET("/keys", handler.secure(PermissionKeyList, handler.listKeys))
	resource.POST("/keys", handler.secure(PermissionKeyIssue, handler.issueKey))
	resource.GET("/keys/:id", handler.secure(PermissionKeyRead, handler.getKey))
	resource.PATCH("/keys/:id", handler.secure(PermissionKeyWrite, handler.updateKey))
	resource.POST("/keys/:id/block", handler.secure(PermissionKeyRevoke, handler.blockKey))
	resource.POST("/keys/:id/unblock", handler.secure(PermissionKeyRevoke, handler.unblockKey))
	resource.DELETE("/keys/:id", handler.secure(PermissionKeyRevoke, handler.deleteKey))
	resource.POST("/keys/:id/rotate", handler.secure(PermissionKeyIssue, handler.rotateKey))
	resource.POST("/keys/:id/reset-spend", handler.secure(PermissionKeyWrite, handler.resetKeySpend))
	resource.GET("/gateway/models", handler.secure(PermissionGatewayRead, handler.gatewayModels))
	resource.GET("/gateway/health", handler.secure(PermissionGatewayRead, handler.gatewayHealth))
	resource.POST("/gateway/models/:id/block", handler.secure(PermissionGatewayWrite, handler.blockModel))
	resource.POST("/gateway/models/:id/unblock", handler.secure(PermissionGatewayWrite, handler.unblockModel))
	resource.POST("/sync", handler.secure(PermissionSync, handler.sync))
	resource.GET("/bills", handler.secure(PermissionBills, handler.bills))
	resource.POST("/users/:id/recharge", handler.secure(PermissionRecharge, handler.recharge))
	resource.GET("/users/:id/recharges", handler.secure(PermissionUserRead, handler.listRecharges))
	resource.POST("/recharges/:id/reconcile", handler.secure(PermissionRecharge, handler.reconcileRecharge))
	resource.GET("/management/commands", handler.secure(PermissionManagementResolve, handler.listManagementCommands))
	resource.POST("/management/commands/:id/reconcile", handler.secure(PermissionManagementResolve, handler.reconcileManagementCommand))
	resource.POST("/management/commands/:id/resolve", handler.secure(PermissionManagementResolve, handler.resolveManagementCommand))
	resource.GET("/organizations", handler.secure(PermissionOrgList, handler.listOrganizations))
	resource.POST("/organizations", handler.secure(PermissionOrgWrite, handler.operationDisabled))
	resource.GET("/organizations/:id", handler.secure(PermissionOrgRead, handler.getOrganization))
	resource.PATCH("/organizations/:id", handler.secure(PermissionOrgWrite, handler.operationDisabled))
	resource.DELETE("/organizations/:id", handler.secure(PermissionOrgWrite, handler.operationDisabled))
	resource.POST("/organizations/:id/recharge", handler.secure(PermissionOrgWrite, handler.operationDisabled))
	resource.POST("/organizations/:id/members", handler.secure(PermissionOrgWrite, handler.operationDisabled))
	resource.POST("/organizations/:id/members/remove", handler.secure(PermissionOrgWrite, handler.operationDisabled))
	resource.POST("/organizations/:id/teams", handler.secure(PermissionOrgWrite, handler.operationDisabled))
	resource.POST("/teams/:teamId/recharge", handler.secure(PermissionOrgWrite, handler.operationDisabled))
	resource.DELETE("/teams/:teamId", handler.secure(PermissionOrgWrite, handler.operationDisabled))
	resource.GET("/sales/products", handler.secure(PermissionProductRead, handler.listProducts))
	resource.POST("/sales/products", handler.secure(PermissionProductWrite, handler.createProduct))
	resource.PATCH("/sales/products/:id", handler.secure(PermissionProductWrite, handler.updateProduct))
	resource.GET("/sales/orders", handler.secure(PermissionOrderRead, handler.listOrders))
	resource.POST("/sales/orders", handler.secure(PermissionOrderImport, handler.createOrder))
	resource.POST("/sales/orders/import", handler.secure(PermissionOrderImport, handler.importOrder))
	resource.GET("/sales/orders/:id", handler.secure(PermissionOrderRead, handler.getOrder))
	resource.POST("/sales/orders/:id/verify", handler.secure(PermissionOrderVerify, handler.verifyOrder))
	resource.POST("/sales/orders/:id/match", handler.secure(PermissionOrderVerify, handler.matchOrder))
	resource.POST("/sales/orders/:id/approve", handler.secure(PermissionOrderApprove, handler.approveOrder))
	resource.POST("/sales/orders/:id/execute", handler.secure(PermissionOrderExecute, handler.executeOrder))
	resource.POST("/sales/orders/:id/reconcile", handler.secure(PermissionOrderReconcile, handler.reconcileOrder))
	resource.POST("/sales/orders/:id/refund-review", handler.secure(PermissionOrderRefundReview, handler.refundReviewOrder))
	return nil
}

type requestHandler struct {
	runtime    business.Runtime
	authorizer *AdminAuthorizer
}

func writeAPIError(ctx *gin.Context, err error) {
	var commandResult *managementCommandResultError
	if errors.As(err, &commandResult) {
		ctx.AbortWithStatusJSON(http.StatusAccepted, gin.H{
			"code": "management_reconcile_required", "error": commandResult.Error(),
			"command": publicManagementCommand(commandResult.Command),
		})
		return
	}
	status, code, message := http.StatusInternalServerError, "internal_error", "litellmops operation failed"
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		status, code, message = http.StatusNotFound, "not_found", "litellmops resource not found"
	case errors.Is(err, ErrManagementPending):
		status, code, message = http.StatusConflict, "management_pending", err.Error()
	case errors.Is(err, ErrManagementUnverified):
		status, code, message = http.StatusConflict, "management_reconcile_required", err.Error()
	case errors.Is(err, ErrIdempotencyConflict), errors.Is(err, ErrOrderConflict), errors.Is(err, ErrRechargeBusy), errors.Is(err, ErrPendingRecharge), errors.Is(err, ErrInvalidOrderState), errors.Is(err, ErrOrderBusy):
		status, code, message = http.StatusConflict, "conflict", err.Error()
	case errors.Is(err, ErrInvalidRecharge), errors.Is(err, ErrUnlimitedBudget), errors.Is(err, ErrInvalidProduct), errors.Is(err, ErrInvalidOrder):
		status, code, message = http.StatusUnprocessableEntity, "invalid_request", err.Error()
	case errors.Is(err, ErrReconcileRequired):
		status, code, message = http.StatusConflict, "reconcile_required", err.Error()
	case errors.Is(err, ErrOperationDisabled):
		status, code, message = http.StatusNotImplemented, "operation_disabled", err.Error()
	default:
		var upstream *UpstreamError
		if errors.As(err, &upstream) {
			status, code, message = http.StatusBadGateway, "upstream_error", upstream.Error()
		}
	}
	ctx.AbortWithStatusJSON(status, gin.H{"error": message, "code": code})
}

func (handler *requestHandler) operationDisabled(ctx *gin.Context) {
	writeAPIError(ctx, ErrOperationDisabled)
}

// secure enforces the casbin permission before the handler runs.
func (handler *requestHandler) secure(permission string, next gin.HandlerFunc) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if err := handler.authorizer.Authorize(ctx, permission); err != nil {
			status := http.StatusForbidden
			if errors.Is(err, ErrAuthenticationRequired) {
				status = http.StatusUnauthorized
			}
			ctx.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
			return
		}
		next(ctx)
	}
}

func (handler *requestHandler) database(ctx *gin.Context) (*gorm.DB, bool) {
	db, ok := handler.runtime.RequestDatabase(ctx.Request.Context())
	if !ok || db == nil {
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "litellmops database unavailable"})
		return nil, false
	}
	return db, true
}

type pageQuery struct {
	Page     int
	PageSize int
}

func bindPage(ctx *gin.Context) pageQuery {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return pageQuery{Page: page, PageSize: pageSize}
}

func parseTimeQuery(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		utc := parsed.UTC()
		return &utc
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		utc := parsed.UTC()
		return &utc
	}
	return nil
}

func (handler *requestHandler) listUsers(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	query := db.Model(&UserSnapshot{})
	if email := strings.TrimSpace(ctx.Query("email")); email != "" {
		query = query.Where("email LIKE ?", "%"+email+"%")
	}
	if role := strings.TrimSpace(ctx.Query("user_role")); role != "" {
		query = query.Where("user_role = ?", role)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	page := bindPage(ctx)
	items := make([]UserSnapshot, 0, page.PageSize)
	if err := query.Session(&gorm.Session{}).
		Order("spend DESC").
		Offset((page.Page - 1) * page.PageSize).
		Limit(page.PageSize).
		Find(&items).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page.Page, "page_size": page.PageSize})
}

func (handler *requestHandler) getUser(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	var user UserSnapshot
	if err := db.First(&user, "id = ?", ctx.Param("id")).Error; err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		ctx.AbortWithStatusJSON(status, gin.H{"error": "litellmops user snapshot not found"})
		return
	}
	keys := make([]KeySnapshot, 0)
	if err := db.Where("user_id = ?", user.UserID).Order("is_session_key ASC, spend DESC").Find(&keys).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"user": user, "keys": keys})
}

func (handler *requestHandler) listKeys(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	query := db.Model(&KeySnapshot{})
	if alias := strings.TrimSpace(ctx.Query("alias")); alias != "" {
		query = query.Where("alias LIKE ?", "%"+alias+"%")
	}
	if email := strings.TrimSpace(ctx.Query("user_email")); email != "" {
		query = query.Where("user_email LIKE ?", "%"+email+"%")
	}
	if session := strings.TrimSpace(ctx.Query("is_session_key")); session != "" {
		query = query.Where("is_session_key = ?", session == "true" || session == "1")
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	page := bindPage(ctx)
	items := make([]KeySnapshot, 0, page.PageSize)
	if err := query.Session(&gorm.Session{}).
		Order("is_session_key ASC, synced_at DESC").
		Offset((page.Page - 1) * page.PageSize).
		Limit(page.PageSize).
		Find(&items).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page.Page, "page_size": page.PageSize})
}

func (handler *requestHandler) getKey(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	var key KeySnapshot
	if err := db.First(&key, "id = ?", ctx.Param("id")).Error; err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		ctx.AbortWithStatusJSON(status, gin.H{"error": "litellmops key snapshot not found"})
		return
	}
	ctx.JSON(http.StatusOK, key)
}

func (handler *requestHandler) recharge(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	var snapshot UserSnapshot
	if err := db.First(&snapshot, "id = ?", ctx.Param("id")).Error; err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		ctx.AbortWithStatusJSON(status, gin.H{"error": "litellmops user snapshot not found"})
		return
	}
	var request RechargeRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "litellmops recharge body is invalid"})
		return
	}
	client, err := NewClientFromEnv()
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	operator := ""
	if principal := handler.runtime.Principal(ctx); principal != nil {
		operator = strings.TrimSpace(principal.GetUsername())
		if operator == "" {
			operator = strings.TrimSpace(principal.GetUserID())
		}
	}
	record, err := ApplyRecharge(ctx.Request.Context(), db, client, snapshot, request, operator)
	if err != nil {
		if record != nil && (record.Status == RechargeAppliedUnverified || record.Status == RechargeRetryableFailed || record.Status == RechargeReconcileRequired) {
			ctx.JSON(http.StatusAccepted, record)
			return
		}
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, record)
}

func (handler *requestHandler) reconcileRecharge(ctx *gin.Context) {
	db, client, ok := handler.managementClient(ctx)
	if !ok {
		return
	}
	record, err := ReconcileRechargeAs(ctx.Request.Context(), db, client, strings.TrimSpace(ctx.Param("id")), handler.operator(ctx))
	if err != nil {
		if record != nil && (record.Status == RechargeAppliedUnverified || record.Status == RechargeRetryableFailed || record.Status == RechargeReconcileRequired) {
			ctx.JSON(http.StatusAccepted, record)
			return
		}
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, record)
}

func (handler *requestHandler) listRecharges(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	var snapshot UserSnapshot
	if err := db.First(&snapshot, "id = ?", ctx.Param("id")).Error; err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		ctx.AbortWithStatusJSON(status, gin.H{"error": "litellmops user snapshot not found"})
		return
	}
	items := make([]RechargeRecord, 0)
	if err := db.Where("user_id = ?", snapshot.UserID).Order("created_at DESC").Limit(50).Find(&items).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"items": items})
}

func (handler *requestHandler) sync(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	client, err := NewClientFromEnv()
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	report, err := SyncSnapshots(ctx.Request.Context(), db, client)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, report)
}

func (handler *requestHandler) bills(ctx *gin.Context) {
	db, err := billsDatabase(func(dsn string) (*gorm.DB, error) {
		return gorm.Open(postgres.Open(dsn), &gorm.Config{})
	})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	service, err := NewBillsService(db)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	page := bindPage(ctx)
	result, err := service.List(ctx.Request.Context(), BillsQuery{
		Model:         ctx.Query("model"),
		Status:        ctx.Query("status"),
		KeyHashPrefix: ctx.Query("key_hash_prefix"),
		Since:         parseTimeQuery(ctx.Query("since")),
		Until:         parseTimeQuery(ctx.Query("until")),
		Page:          page.Page,
		PageSize:      page.PageSize,
	})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, result)
}
