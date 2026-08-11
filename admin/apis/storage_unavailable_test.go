package apis

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
)

func TestStorageUploadUnboundReturnsFixed503(t *testing.T) {
	assertUnboundStorageRoute(t, storageUploadRoute, func(context *gin.Context) {
		(&Storage{service: &service.Storage{}}).Upload(context)
	})
}

func TestAvatarUploadUnboundReturnsFixed503(t *testing.T) {
	previous := defaultUploadService
	defaultUploadService = &service.Storage{}
	t.Cleanup(func() { defaultUploadService = previous })
	assertUnboundStorageRoute(t, avatarUploadRoute, func(context *gin.Context) {
		(&User{}).UpdateAvatar(context)
	})
}

func TestStorageInvalidPolicyReturnsFixed503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := center.GetAppConfig()
	t.Cleanup(func() { center.SetAppConfig(previous) })
	setUploadAdmissionConfig("not-a-byte-count")
	body, contentType := uploadAdmissionMultipartBody(t, []byte("unavailable"), "text/plain", 0)
	request := httptest.NewRequest(http.MethodPost, storageUploadRoute, bytes.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	recorder := executeUploadRoute(request, storageUploadRoute, func(context *gin.Context) {
		(&Storage{service: &service.Storage{}}).Upload(context)
	}, 1<<20)
	assertUploadErrorResponse(t, recorder, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE")
}

func assertUnboundStorageRoute(t *testing.T, route string, handler gin.HandlerFunc) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	previous := center.GetAppConfig()
	t.Cleanup(func() { center.SetAppConfig(previous) })
	setUploadAdmissionConfig("1024")
	body, contentType := uploadAdmissionMultipartBody(t, []byte("unavailable"), "text/plain", 0)
	request := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	recorder := executeUploadRoute(request, route, handler, 1<<20)
	assertUploadErrorResponse(t, recorder, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE")
}
