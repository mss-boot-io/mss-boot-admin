package apis

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestOnlineSessionTargetHierarchyProtectsRootUsers(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Role{}, &models.User{}))

	rootRole := &models.Role{Name: "root", Status: enum.Enabled}
	rootRole.ID = "root-role"
	require.NoError(t, db.Create(rootRole).Error)
	require.NoError(t, db.Exec("UPDATE mss_boot_roles SET root = ? WHERE id = ?", true, rootRole.ID).Error)
	memberRole := &models.Role{Name: "member", Status: enum.Enabled}
	memberRole.ID = "member-role"
	require.NoError(t, db.Create(memberRole).Error)

	createUser := func(id, roleID string) {
		user := &models.User{UserLogin: models.UserLogin{
			Username: id,
			RoleID:   roleID,
			Password: "test-password",
			Status:   enum.Enabled,
		}}
		user.ID = id
		require.NoError(t, db.Create(user).Error)
	}
	createUser("root-target", rootRole.ID)
	createUser("member-target", memberRole.ID)

	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = "online-session-target-identity"
	t.Cleanup(func() { config.Cfg.Auth.IdentityKey = previousIdentityKey })

	tests := []struct {
		name      string
		principal *models.User
		target    string
		want      int
	}{
		{
			name: "delegated operator cannot revoke root target",
			principal: &models.User{UserLogin: models.UserLogin{
				RoleID: memberRole.ID,
				Role:   &models.Role{Root: false},
			}},
			target: "root-target",
			want:   http.StatusForbidden,
		},
		{
			name: "delegated operator may revoke ordinary target",
			principal: &models.User{UserLogin: models.UserLogin{
				RoleID: memberRole.ID,
				Role:   &models.Role{Root: false},
			}},
			target: "member-target",
			want:   http.StatusNoContent,
		},
		{
			name: "root may revoke root target",
			principal: &models.User{UserLogin: models.UserLogin{
				RoleID: rootRole.ID,
				Role:   &models.Role{Root: true},
			}},
			target: "root-target",
			want:   http.StatusNoContent,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := &OnlineSessionAPI{db: db}
			router := gin.New()
			router.DELETE("/users/:userID", func(c *gin.Context) {
				c.Set(config.Cfg.Auth.IdentityKey, test.principal)
				c.Next()
			}, func(c *gin.Context) {
				if api.protectSessionTarget(c, c.Param("userID")) {
					c.Status(http.StatusNoContent)
				}
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(
				recorder,
				httptest.NewRequest(http.MethodDelete, "/users/"+test.target, nil),
			)
			require.Equal(t, test.want, recorder.Code)
		})
	}
}

func TestOnlineSessionAdministrativeRoutesRequireRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	previousAuthHandler := response.AuthHandler
	config.Cfg.Auth.IdentityKey = "online-session-route-identity"
	response.AuthHandler = func(c *gin.Context) { c.Next() }
	t.Cleanup(func() {
		config.Cfg.Auth.IdentityKey = previousIdentityKey
		response.AuthHandler = previousAuthHandler
	})

	principal := &models.User{UserLogin: models.UserLogin{
		RoleID: "delegated-role",
		Role:   &models.Role{Root: false},
	}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(config.Cfg.Auth.IdentityKey, principal)
		c.Next()
	})
	(&OnlineSessionAPI{}).Other(router.Group(""))
	(&WS{}).Other(router.Group(""))

	for _, test := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "list", method: http.MethodGet, path: "/online-sessions"},
		{name: "get", method: http.MethodGet, path: "/online-sessions/session-1"},
		{name: "revoke session", method: http.MethodDelete, path: "/online-sessions/session-1"},
		{name: "revoke user", method: http.MethodDelete, path: "/online-sessions/user/user-1"},
		{name: "online websocket inventory", method: http.MethodGet, path: "/ws/online"},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
			require.Equal(t, http.StatusForbidden, recorder.Code)
		})
	}

	// Current-session logout remains an authenticated-self capability. The
	// request reaches its own sid validation instead of the root boundary.
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/online-sessions/logout", nil))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
