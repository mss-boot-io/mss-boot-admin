package apis

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRequireRootManagementFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = "test-identity"
	t.Cleanup(func() { config.Cfg.Auth.IdentityKey = previousIdentityKey })

	tests := []struct {
		name      string
		principal *models.User
		want      int
		called    bool
	}{
		{name: "missing identity", want: http.StatusUnauthorized},
		{
			name: "delegated administrator",
			principal: &models.User{UserLogin: models.UserLogin{
				RoleID: "delegated",
				Role:   &models.Role{Root: false},
			}},
			want: http.StatusForbidden,
		},
		{
			name: "current root",
			principal: &models.User{UserLogin: models.UserLogin{
				RoleID: "root",
				Role:   &models.Role{Root: true},
			}},
			want:   http.StatusNoContent,
			called: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			router := gin.New()
			router.POST("/authority", func(c *gin.Context) {
				if test.principal != nil {
					c.Set(config.Cfg.Auth.IdentityKey, test.principal)
				}
				c.Next()
			}, requireRootManagement, func(c *gin.Context) {
				called = true
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/authority", nil))
			require.Equal(t, test.want, recorder.Code)
			require.Equal(t, test.called, called)
		})
	}
}

func TestProtectRootUserLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Role{}, &models.User{}))
	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })

	rootRole := &models.Role{Name: "protected-root", Status: enum.Enabled}
	rootRole.ID = "protected-root-role"
	require.NoError(t, db.Create(rootRole).Error)
	require.NoError(t, db.Exec("UPDATE mss_boot_roles SET root = ? WHERE id = ?", true, rootRole.ID).Error)
	ordinaryRole := &models.Role{Name: "ordinary", Status: enum.Enabled}
	ordinaryRole.ID = "ordinary-role"
	require.NoError(t, db.Create(ordinaryRole).Error)

	rootUser := &models.User{UserLogin: models.UserLogin{Username: "root-user", RoleID: rootRole.ID, Status: enum.Enabled}}
	rootUser.ID = "root-user"
	require.NoError(t, db.Create(rootUser).Error)
	ordinaryUser := &models.User{UserLogin: models.UserLogin{Username: "ordinary-user", RoleID: ordinaryRole.ID, Status: enum.Enabled}}
	ordinaryUser.ID = "ordinary-user"
	require.NoError(t, db.Create(ordinaryUser).Error)

	tests := []struct {
		name   string
		userID string
		want   int
		called bool
	}{
		{name: "root principal protected", userID: rootUser.ID, want: http.StatusForbidden},
		{name: "ordinary principal mutable", userID: ordinaryUser.ID, want: http.StatusNoContent, called: true},
		{name: "missing principal", userID: "missing-user", want: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			router := gin.New()
			router.PUT("/users/:id", protectRootUserLifecycle, func(c *gin.Context) {
				called = true
				c.Status(http.StatusNoContent)
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/users/"+test.userID, nil))
			require.Equal(t, test.want, recorder.Code)
			require.Equal(t, test.called, called)
		})
	}

	t.Run("create has no lifecycle target", func(t *testing.T) {
		called := false
		router := gin.New()
		router.POST("/users", protectRootUserLifecycle, func(c *gin.Context) {
			called = true
			c.Status(http.StatusNoContent)
		})
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/users", nil))
		require.Equal(t, http.StatusNoContent, recorder.Code)
		require.True(t, called)
	})
}
