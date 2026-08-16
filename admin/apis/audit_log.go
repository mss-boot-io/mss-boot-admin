package apis

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
)

func init() {
	e := &AuditLogAPI{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.AuditLog)),
			controller.WithSearch(new(dto.AuditLogSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type AuditLogAPI struct {
	*controller.Simple
}

type loginLogProjection struct {
	ID        string      `json:"id"`
	UserID    string      `json:"userID"`
	Username  string      `json:"username"`
	IP        string      `json:"ip"`
	Location  string      `json:"location"`
	UserAgent string      `json:"userAgent"`
	Status    enum.Status `json:"status"`
	Message   string      `json:"message"`
	LoginAt   time.Time   `json:"loginAt"`
	LogoutAt  *time.Time  `json:"logoutAt"`
}

type auditLogProjection struct {
	ID        string              `json:"id"`
	UserID    string              `json:"userID"`
	Username  string              `json:"username"`
	Type      models.AuditLogType `json:"type"`
	Action    string              `json:"action"`
	Resource  string              `json:"resource"`
	Method    string              `json:"method"`
	Path      string              `json:"path"`
	IP        string              `json:"ip"`
	UserAgent string              `json:"userAgent"`
	Status    enum.Status         `json:"status"`
	Message   string              `json:"message"`
	Duration  int64               `json:"duration"`
	CreatedAt time.Time           `json:"createdAt"`
}

// GetAction keeps durable audit evidence append-only through the public
// controller surface. Audit rows are created only by the audit service; an
// administrator may inspect them but can never create, replace, or delete
// evidence through the generic CRUD routes.
func (e *AuditLogAPI) GetAction(key string) response.Action {
	// Browser-visible audit routes use the bounded projections below. The
	// generic model includes raw request and response bodies and must never be
	// exposed through an alternate controller path.
	return nil
}

func (e *AuditLogAPI) Other(r *gin.RouterGroup) {
	r.GET("/audit-logs/login", response.AuthHandler, protectOperationalResponse, e.LoginLogs)
	r.GET("/audit-logs/operation", response.AuthHandler, protectOperationalResponse, e.OperationLogs)
}

func (e *AuditLogAPI) LoginLogs(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := response.VerifyHandler(ctx)
	if verify == nil {
		api.Err(401)
		return
	}

	var req dto.LoginLogSearch
	if err := ctx.ShouldBindQuery(&req); err != nil {
		api.AddError(err).Err(400)
		return
	}

	if req.Current <= 0 {
		req.Current = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	page, pageSize := int(req.GetPage()), int(req.GetPageSize())
	logs, total, err := service.Audit.GetLoginLogs(
		center.Default.GetDB(ctx, &models.LoginLog{}),
		req.UserID,
		req.Username,
		page,
		pageSize,
	)
	if err != nil {
		api.AddError(err).Err(500)
		return
	}

	items := make([]loginLogProjection, 0, len(logs))
	for _, item := range logs {
		if item == nil {
			continue
		}
		items = append(items, loginLogProjection{
			ID:        item.ID,
			UserID:    item.UserID,
			Username:  item.Username,
			IP:        item.IP,
			Location:  service.RedactOperationalText(item.Location, 255),
			UserAgent: service.RedactOperationalText(item.UserAgent, 500),
			Status:    item.Status,
			Message:   service.RedactOperationalText(item.Message, 500),
			LoginAt:   item.LoginAt,
			LogoutAt:  item.LogoutAt,
		})
	}
	api.OK(gin.H{
		"data":     items,
		"total":    total,
		"current":  page,
		"pageSize": pageSize,
	})
}

func (e *AuditLogAPI) OperationLogs(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := response.VerifyHandler(ctx)
	if verify == nil {
		api.Err(401)
		return
	}

	var req dto.AuditLogSearch
	if err := ctx.ShouldBindQuery(&req); err != nil {
		api.AddError(err).Err(400)
		return
	}

	if req.Current <= 0 {
		req.Current = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	page, pageSize := int(req.GetPage()), int(req.GetPageSize())
	logs, total, err := service.Audit.GetAuditLogs(
		center.Default.GetDB(ctx, &models.AuditLog{}),
		req.UserID,
		req.Username,
		models.AuditLogType(req.Type),
		page,
		pageSize,
	)
	if err != nil {
		api.AddError(err).Err(500)
		return
	}

	items := make([]auditLogProjection, 0, len(logs))
	for _, item := range logs {
		if item == nil {
			continue
		}
		items = append(items, auditLogProjection{
			ID:        item.ID,
			UserID:    item.UserID,
			Username:  item.Username,
			Type:      item.Type,
			Action:    service.RedactOperationalText(item.Action, 255),
			Resource:  service.RedactOperationalText(item.Resource, 255),
			Method:    item.Method,
			Path:      service.RedactOperationalText(item.Path, 500),
			IP:        item.IP,
			UserAgent: service.RedactOperationalText(item.UserAgent, 500),
			Status:    item.Status,
			Message:   service.RedactOperationalText(item.Message, 4_096),
			Duration:  item.Duration,
			CreatedAt: item.CreatedAt,
		})
	}
	api.OK(gin.H{
		"data":     items,
		"total":    total,
		"current":  page,
		"pageSize": pageSize,
	})
}
