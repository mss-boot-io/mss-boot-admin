package apis

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
)

var (
	uploadRequestTooLargeError = response.NewError(
		"UPLOAD_REQUEST_TOO_LARGE",
		"upload request exceeds the configured limit",
	)
	invalidUploadError = response.NewError(
		"INVALID_UPLOAD",
		"upload request is malformed or violates the file policy",
	)
	storageUnavailableError = response.NewError(
		"STORAGE_UNAVAILABLE",
		"object storage is temporarily unavailable",
	)
	defaultUploadService = &service.Storage{}
)

func init() {
	e := &Storage{
		Simple:  controller.NewSimple(),
		service: defaultUploadService,
	}
	response.AppendController(e)
}

type Storage struct {
	*controller.Simple
	service *service.Storage
}

func (*Storage) GetKey() string {
	return "storage"
}

func (*Storage) GetAction(string) response.Action {
	return nil
}

func (e *Storage) Other(r *gin.RouterGroup) {
	r.POST("/storage/upload", middleware.Auth.MiddlewareFunc(), e.Upload)
}

// Upload stores one admitted file.
// @Summary Upload a file
// @Description Admit and store one multipart file using the configured storage provider.
// @Tags storage
// @Accept multipart/form-data
// @Param file formData file true "File"
// @Success 201 {object} service.UploadResult
// @Failure 413 {object} response.Response
// @Failure 422 {object} response.Response
// @Failure 503 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/api/storage/upload [post]
// @Security Bearer
func (e *Storage) Upload(ctx *gin.Context) {
	api := response.Make(ctx)
	storageService := defaultUploadService
	if e != nil && e.service != nil {
		storageService = e.service
	}
	result, err := storageService.Upload(ctx, "file")
	if err != nil {
		writeUploadError(api, err)
		return
	}
	api.OK(result)
}

func writeUploadError(api *response.API, err error) {
	if errors.Is(err, service.ErrStorageUnavailable) {
		api.Log.Warn("upload rejected: object storage unavailable")
		api.AddError(storageUnavailableError)
		api.Err(http.StatusServiceUnavailable)
		return
	}
	if errors.Is(err, service.ErrUploadTooLarge) {
		api.Log.Warn("upload rejected: request too large")
		api.AddError(uploadRequestTooLargeError)
		api.Err(http.StatusRequestEntityTooLarge)
		return
	}
	if errors.Is(err, service.ErrInvalidUpload) {
		api.Log.Warn("upload rejected: invalid multipart input")
		api.AddError(invalidUploadError)
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	api.Log.Error("upload failed", "error", err)
	api.Err(http.StatusInternalServerError)
}
