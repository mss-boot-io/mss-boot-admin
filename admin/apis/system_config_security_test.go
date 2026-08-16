package apis

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	storagecache "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	responsegorm "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions/gorm"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

	t.Run("list", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(config.Cfg.Auth.IdentityKey, &models.User{UserLogin: models.UserLogin{
				RoleID: "delegated",
				Role:   &models.Role{Root: false},
			}})
			c.Next()
		})
		router.GET(
			"/system-configs",
			response.AuthHandler,
			requireRootManagement,
			protectSystemConfigResponse,
			controller.List,
		)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/system-configs", nil))
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})
}

func TestProtectSystemConfigResponseMarksBypassForRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = "system-config-bypass-identity"
	t.Cleanup(func() { config.Cfg.Auth.IdentityKey = previousIdentityKey })

	var bypassed bool
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(config.Cfg.Auth.IdentityKey, &models.User{UserLogin: models.UserLogin{
			RoleID: "root",
			Role:   &models.Role{Root: true},
		}})
		c.Next()
	})
	router.GET(
		"/system-configs",
		requireRootManagement,
		protectSystemConfigResponse,
		func(c *gin.Context) {
			bypassed = storagecache.FromBypass(c.Request.Context()) && storagecache.FromBypass(c)
			c.Status(http.StatusNoContent)
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/system-configs", nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.True(t, bypassed)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}

func TestSystemConfigRootReadsAndUpdateNeverPopulateQueryCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	previousAuthHandler := response.AuthHandler
	previousDB := gormdb.DB
	previousCleaner := responsegorm.CleanCacheFromTag
	config.Cfg.Auth.IdentityKey = "system-config-root-identity"
	response.AuthHandler = func(c *gin.Context) { c.Next() }
	responsegorm.CleanCacheFromTag = nil
	t.Cleanup(func() {
		config.Cfg.Auth.IdentityKey = previousIdentityKey
		response.AuthHandler = previousAuthHandler
		gormdb.DB = previousDB
		responsegorm.CleanCacheFromTag = previousCleaner
	})

	server := miniredis.RunT(t)
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{server.Addr()}})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	queryCache, err := storagecache.NewRedis(
		client,
		nil,
		storagecache.WithQueryCacheKeys("*"),
	)
	require.NoError(t, err)

	db, err := gorm.Open(
		sqlite.Open("file:system_config_cache_bypass?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SystemConfig{}))
	record := &models.SystemConfig{Name: "private", Ext: "json", Content: `{"secret":"value"}`}
	require.NoError(t, db.Create(record).Error)
	require.NoError(t, queryCache.Initialize(db))
	gormdb.DB = db

	controller := newSystemConfigController()
	tests := []struct {
		name   string
		action string
		method string
		path   string
		target string
		body   string
	}{
		{name: "get", action: response.Get, method: http.MethodGet, path: "/system-configs/:id", target: "/system-configs/" + record.ID},
		{
			name:   "update pre-read",
			action: response.Control,
			method: http.MethodPut,
			path:   "/system-configs/:id",
			target: "/system-configs/" + record.ID,
			body:   `{"name":"private-updated","ext":"json","content":"{\"secret\":\"updated\"}","remark":""}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := controller.GetAction(test.action)
			require.NotNil(t, action)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(config.Cfg.Auth.IdentityKey, &models.User{UserLogin: models.UserLogin{
					RoleID: "root",
					Role:   &models.Role{Root: true},
				}})
				c.Next()
			})
			router.Handle(test.method, test.path, action.Handler()...)

			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			require.Empty(t, server.Keys(), "system config action populated shared query cache")
		})
	}

	t.Run("list", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(config.Cfg.Auth.IdentityKey, &models.User{UserLogin: models.UserLogin{
				RoleID: "root",
				Role:   &models.Role{Root: true},
			}})
			c.Next()
		})
		router.GET(
			"/system-configs",
			response.AuthHandler,
			requireRootManagement,
			protectSystemConfigResponse,
			controller.List,
		)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/system-configs", nil))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
		require.NotContains(t, recorder.Body.String(), `"content"`)
		require.Empty(t, server.Keys(), "system config list populated shared query cache")
	})
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
