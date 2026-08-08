package apis

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserInfoPermissionProjectionFailsClosedOnStalePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Role{}, &models.User{}, &models.ConfigRevision{}))

	previousDB := gormdb.DB
	previousEnforcer := gormdb.Enforcer
	previousEnsureCurrent := ensureCurrentAuthorizationPolicies
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	gormdb.DB = db
	gormdb.Enforcer = nil
	ensureCurrentAuthorizationPolicies = func(*gin.Context, *gorm.DB) error {
		return errors.New("policy reload unavailable")
	}
	config.Cfg.Auth.IdentityKey = "test-identity"
	t.Cleanup(func() {
		gormdb.DB = previousDB
		gormdb.Enforcer = previousEnforcer
		ensureCurrentAuthorizationPolicies = previousEnsureCurrent
		config.Cfg.Auth.IdentityKey = previousIdentityKey
	})

	rootRole := &models.Role{Name: "root", Status: enum.Enabled}
	rootRole.ID = "root-role"
	require.NoError(t, db.Create(rootRole).Error)
	require.NoError(t, db.Model(&models.Role{}).Where("id = ?", rootRole.ID).UpdateColumn("root", true).Error)
	rootRole.Root = true

	delegatedRole := &models.Role{Name: "delegated", Status: enum.Enabled}
	delegatedRole.ID = "delegated-role"
	require.NoError(t, db.Create(delegatedRole).Error)

	rootUser := &models.User{UserLogin: models.UserLogin{
		Username: "root-user",
		RoleID:   rootRole.ID,
		Role:     rootRole,
		Status:   enum.Enabled,
	}}
	rootUser.ID = "root-user"
	require.NoError(t, db.Create(rootUser).Error)

	delegatedUser := &models.User{UserLogin: models.UserLogin{
		Username: "delegated-user",
		RoleID:   delegatedRole.ID,
		Role:     delegatedRole,
		Status:   enum.Enabled,
	}}
	delegatedUser.ID = "delegated-user"
	require.NoError(t, db.Create(delegatedUser).Error)

	tests := []struct {
		name      string
		principal *models.User
		want      int
	}{
		{name: "root bypasses policy projection", principal: rootUser, want: http.StatusOK},
		{name: "delegated identity rejects unavailable policy", principal: delegatedUser, want: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/userInfo", func(c *gin.Context) {
				c.Set(config.Cfg.Auth.IdentityKey, test.principal)
				c.Next()
			}, (&User{}).UserInfo)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/userInfo", nil))
			require.Equal(t, test.want, recorder.Code)
		})
	}
}

func TestProjectActiveUserPermissionsRejectsInactiveAndForeignPolicies(t *testing.T) {
	menuType := pkg.MenuAccessType.String()
	componentType := pkg.ComponentAccessType.String()
	active := map[string]struct{}{
		menuType + "\x00/active":             {},
		componentType + "\x00/action/active": {},
	}
	result := projectActiveUserPermissions("role-a", [][]string{
		{"role-a", menuType, "/active", "GET"},
		{"role-a", componentType, "/action/active", "GET"},
		{"role-a", menuType, "/disabled", "GET"},
		{"role-b", menuType, "/active", "GET"},
		{"role-a", pkg.APIAccessType.String(), "/admin/api/active", "GET"},
		{"role-a", menuType},
	}, active)

	require.Equal(t, map[string]bool{
		"/active":        true,
		"/action/active": true,
	}, result)
}
