package apis

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func prepareDepartmentCreate(ctx *gin.Context, db *gorm.DB, table schema.Tabler) error {
	department, ok := table.(*models.Department)
	if !ok {
		return gorm.ErrInvalidData
	}
	return service.PrepareDepartmentCreate(ctx, db, department)
}

func prepareDepartmentUpdate(ctx *gin.Context, db *gorm.DB, table schema.Tabler) error {
	department, ok := table.(*models.Department)
	if !ok {
		return gorm.ErrInvalidData
	}
	return service.PrepareDepartmentUpdate(ctx, db, ctx.Param("id"), department)
}

func validateDepartmentDelete(ctx *gin.Context, db *gorm.DB, _ schema.Tabler) error {
	ids, ok := authorityDeleteIDs(ctx)
	if !ok {
		return service.ErrAuthorityPayloadInvalid
	}
	return service.ValidateDepartmentDelete(ctx, db, ids)
}

func preparePostCreate(ctx *gin.Context, db *gorm.DB, table schema.Tabler) error {
	post, ok := table.(*models.Post)
	if !ok {
		return gorm.ErrInvalidData
	}
	return service.PreparePostCreate(ctx, db, post)
}

func preparePostUpdate(ctx *gin.Context, db *gorm.DB, table schema.Tabler) error {
	post, ok := table.(*models.Post)
	if !ok {
		return gorm.ErrInvalidData
	}
	return service.PreparePostUpdate(ctx, db, ctx.Param("id"), post)
}

func validatePostDelete(ctx *gin.Context, db *gorm.DB, _ schema.Tabler) error {
	ids, ok := authorityDeleteIDs(ctx)
	if !ok {
		return service.ErrAuthorityPayloadInvalid
	}
	return service.ValidatePostDelete(ctx, db, ids)
}

func authorityDeleteIDs(ctx *gin.Context) ([]string, bool) {
	value, exists := ctx.Get("ids")
	ids, ok := value.([]string)
	return ids, exists && ok
}

func authorityHierarchyWriteErrorMapper(
	codePrefix string,
	resourceName string,
) actions.WriteErrorMapper {
	return func(
		_ *gin.Context,
		_ actions.WriteOperation,
		err error,
	) (actions.PublicWriteError, bool) {
		public := actions.PublicWriteError{}
		switch {
		case errors.Is(err, service.ErrAuthorityPayloadInvalid):
			public.Status = http.StatusUnprocessableEntity
			public.Error = response.NewError(codePrefix+"_INVALID", resourceName+" payload is invalid")
		case errors.Is(err, service.ErrAuthorityReferenceInvalid):
			public.Status = http.StatusUnprocessableEntity
			public.Error = response.NewError(codePrefix+"_REFERENCE_INVALID", resourceName+" reference is invalid")
		case errors.Is(err, service.ErrAuthorityHierarchyCycle):
			public.Status = http.StatusConflict
			public.Error = response.NewError(codePrefix+"_HIERARCHY_CYCLE", resourceName+" hierarchy would contain a cycle")
		case errors.Is(err, service.ErrAuthorityHierarchyCorrupt):
			public.Status = http.StatusConflict
			public.Error = response.NewError(codePrefix+"_HIERARCHY_INVALID", resourceName+" hierarchy is invalid")
		case errors.Is(err, service.ErrAuthorityRecordInUse):
			public.Status = http.StatusConflict
			public.Error = response.NewError(codePrefix+"_IN_USE", resourceName+" is still referenced")
		case errors.Is(err, gorm.ErrRecordNotFound):
			public.Status = http.StatusNotFound
			public.Error = response.NewError(codePrefix+"_NOT_FOUND", resourceName+" was not found")
		default:
			return actions.PublicWriteError{}, false
		}
		return public, true
	}
}
