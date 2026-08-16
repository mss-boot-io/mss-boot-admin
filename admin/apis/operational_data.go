package apis

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func protectOperationalResponse(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Next()
}

func currentOperationalUserID(ctx *gin.Context) (string, bool) {
	verify := middleware.GetVerify(ctx)
	if verify == nil {
		return "", false
	}
	userID := strings.TrimSpace(verify.GetUserID())
	return userID, userID != ""
}

func noticeOwnerScope(ctx *gin.Context, _ schema.Tabler) func(*gorm.DB) *gorm.DB {
	userID, ok := currentOperationalUserID(ctx)
	return func(db *gorm.DB) *gorm.DB {
		if !ok {
			return db.Where("1 = 0")
		}
		return db.Where("user_id = ?", userID)
	}
}

func prepareNoticeCreate(ctx *gin.Context, db *gorm.DB, table schema.Tabler) error {
	notice, ok := table.(*models.Notice)
	if !ok {
		return gorm.ErrInvalidData
	}
	userID, ok := currentOperationalUserID(ctx)
	if !ok {
		return service.ErrOperationalIdentity
	}
	return service.PrepareNoticeCreate(ctx, db, userID, notice)
}

func prepareNoticeUpdate(ctx *gin.Context, db *gorm.DB, table schema.Tabler) error {
	notice, ok := table.(*models.Notice)
	if !ok {
		return gorm.ErrInvalidData
	}
	userID, ok := currentOperationalUserID(ctx)
	if !ok {
		return service.ErrOperationalIdentity
	}
	return service.PrepareNoticeUpdate(ctx, db, userID, ctx.Param("id"), notice)
}

func prepareTaskCreate(ctx *gin.Context, db *gorm.DB, table schema.Tabler) error {
	definition, ok := table.(*models.Task)
	if !ok {
		return gorm.ErrInvalidData
	}
	return service.PrepareTaskCreate(ctx, db, definition)
}

func prepareTaskUpdate(ctx *gin.Context, db *gorm.DB, table schema.Tabler) error {
	definition, ok := table.(*models.Task)
	if !ok {
		return gorm.ErrInvalidData
	}
	return service.PrepareTaskUpdate(ctx, db, ctx.Param("id"), definition)
}

func validateTaskDelete(ctx *gin.Context, db *gorm.DB, _ schema.Tabler) error {
	ids, ok := authorityDeleteIDs(ctx)
	if !ok {
		return service.ErrOperationalPayloadInvalid
	}
	return service.ValidateTaskDelete(ctx, db, ids)
}

func prepareSystemConfigCreate(ctx *gin.Context, db *gorm.DB, table schema.Tabler) error {
	config, ok := table.(*models.SystemConfig)
	if !ok {
		return gorm.ErrInvalidData
	}
	return service.PrepareSystemConfigCreate(ctx, db, config)
}

func prepareSystemConfigUpdate(ctx *gin.Context, db *gorm.DB, table schema.Tabler) error {
	config, ok := table.(*models.SystemConfig)
	if !ok {
		return gorm.ErrInvalidData
	}
	return service.PrepareSystemConfigUpdate(ctx, db, ctx.Param("id"), config)
}

func validateSystemConfigDelete(ctx *gin.Context, db *gorm.DB, _ schema.Tabler) error {
	ids, ok := authorityDeleteIDs(ctx)
	if !ok {
		return service.ErrOperationalPayloadInvalid
	}
	return service.ValidateSystemConfigDelete(ctx, db, ids)
}

func operationalWriteErrorMapper(codePrefix, resourceName string) actions.WriteErrorMapper {
	return func(
		_ *gin.Context,
		_ actions.WriteOperation,
		err error,
	) (actions.PublicWriteError, bool) {
		public := actions.PublicWriteError{}
		switch {
		case errors.Is(err, service.ErrOperationalIdentity):
			public.Status = http.StatusUnauthorized
			public.Error = response.NewError(codePrefix+"_IDENTITY_REQUIRED", "verified identity is required")
		case errors.Is(err, service.ErrOperationalPayloadInvalid):
			public.Status = http.StatusUnprocessableEntity
			public.Error = response.NewError(codePrefix+"_INVALID", resourceName+" payload is invalid")
		case errors.Is(err, service.ErrOperationalBuiltIn):
			public.Status = http.StatusConflict
			public.Error = response.NewError(codePrefix+"_BUILT_IN", resourceName+" is built-in and protected")
		case errors.Is(err, service.ErrOperationalConflict):
			public.Status = http.StatusConflict
			public.Error = response.NewError(codePrefix+"_CONFLICT", resourceName+" conflicts with current state")
		case errors.Is(err, gorm.ErrRecordNotFound):
			public.Status = http.StatusNotFound
			public.Error = response.NewError(codePrefix+"_NOT_FOUND", resourceName+" was not found")
		default:
			return actions.PublicWriteError{}, false
		}
		return public, true
	}
}
