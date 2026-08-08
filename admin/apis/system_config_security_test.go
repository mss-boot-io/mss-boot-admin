package apis

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/stretchr/testify/require"
)

func TestSystemConfigActionsRequireRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	previousAuthHandler := response.AuthHandler
	config.Cfg.Auth.IdentityKey = "test-identity"
	response.AuthHandler = func(c *gin.Context) { c.Next() }
	t.Cleanup(func() {
		config.Cfg.Auth.IdentityKey = previousIdentityKey
		response.AuthHandler = previousAuthHandler
	})

	controller := newSystemConfigController()
	tests := []struct {
		name   string
		action string
		method string
		path   string
	}{
		{name: "list", action: response.Search, method: http.MethodGet, path: "/system-configs"},
		{name: "get", action: response.Get, method: http.MethodGet, path: "/system-configs/:id"},
		{name: "create", action: response.Control, method: http.MethodPost, path: "/system-configs"},
		{name: "update", action: response.Control, method: http.MethodPut, path: "/system-configs/:id"},
		{name: "delete", action: response.Delete, method: http.MethodDelete, path: "/system-configs/:id"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := controller.GetAction(test.action)
			require.NotNil(t, action)

			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(config.Cfg.Auth.IdentityKey, &models.User{UserLogin: models.UserLogin{
					RoleID: "delegated",
					Role:   &models.Role{Root: false},
				}})
				c.Next()
			})
			router.Handle(test.method, test.path, action.Handler()...)

			target := "/system-configs"
			if test.path == "/system-configs/:id" {
				target += "/secret"
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(test.method, target, nil))
			require.Equal(t, http.StatusForbidden, recorder.Code)
		})
	}
}

func TestAuthorityInputMutationsRequireRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	previousAuthHandler := response.AuthHandler
	config.Cfg.Auth.IdentityKey = "authority-input-identity"
	response.AuthHandler = func(c *gin.Context) { c.Next() }
	t.Cleanup(func() {
		config.Cfg.Auth.IdentityKey = previousIdentityKey
		response.AuthHandler = previousAuthHandler
	})

	tests := []struct {
		name       string
		controller interface {
			GetAction(string) response.Action
		}
		action string
		method string
		path   string
	}{
		{name: "create department", controller: newDepartmentController(), action: response.Control, method: http.MethodPost, path: "/departments"},
		{name: "update department", controller: newDepartmentController(), action: response.Control, method: http.MethodPut, path: "/departments/:id"},
		{name: "delete department", controller: newDepartmentController(), action: response.Delete, method: http.MethodDelete, path: "/departments/:id"},
		{name: "create post", controller: newPostController(), action: response.Control, method: http.MethodPost, path: "/posts"},
		{name: "update post", controller: newPostController(), action: response.Control, method: http.MethodPut, path: "/posts/:id"},
		{name: "delete post", controller: newPostController(), action: response.Delete, method: http.MethodDelete, path: "/posts/:id"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := test.controller.GetAction(test.action)
			require.NotNil(t, action)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(config.Cfg.Auth.IdentityKey, &models.User{UserLogin: models.UserLogin{
					RoleID: "delegated",
					Role:   &models.Role{Root: false},
				}})
				c.Next()
			})
			router.Handle(test.method, test.path, action.Handler()...)

			target := test.path
			if test.method != http.MethodPost {
				target = test.path[:len(test.path)-len(":id")] + "target"
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(test.method, target, nil))
			require.Equal(t, http.StatusForbidden, recorder.Code)
		})
	}
}
