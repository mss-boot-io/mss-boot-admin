package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot-admin/service"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
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

func (e *AuditLogAPI) Other(r *gin.RouterGroup) {
	r.GET("/audit-logs/login", e.LoginLogs)
	r.GET("/audit-logs/operation", e.OperationLogs)
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

	logs, total, err := service.Audit.GetLoginLogs(
		center.Default.GetDB(ctx, &models.LoginLog{}),
		req.UserID,
		req.Current,
		req.PageSize,
	)
	if err != nil {
		api.AddError(err).Err(500)
		return
	}

	api.OK(gin.H{
		"data":     logs,
		"total":    total,
		"current":  req.Current,
		"pageSize": req.PageSize,
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

	logs, total, err := service.Audit.GetAuditLogs(
		center.Default.GetDB(ctx, &models.AuditLog{}),
		req.UserID,
		models.AuditLogType(req.Type),
		req.Current,
		req.PageSize,
	)
	if err != nil {
		api.AddError(err).Err(500)
		return
	}

	api.OK(gin.H{
		"data":     logs,
		"total":    total,
		"current":  req.Current,
		"pageSize": req.PageSize,
	})
}
