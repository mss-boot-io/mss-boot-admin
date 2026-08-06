package middleware

import (
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type refreshTestAppConfig map[string]string

func (c refreshTestAppConfig) SetAppConfig(*gin.Context, string, bool, string) error {
	return nil
}

func (c refreshTestAppConfig) GetAppConfig(_ *gin.Context, key string) (string, bool) {
	value, ok := c[key]
	return value, ok
}

func TestValidateRefreshVerifierDoesNotDependOnRegistrationSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "refresh.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&models.Role{}, &models.User{}, &models.UserOAuth2{}))

	previousDB := gormdb.DB
	previousAppConfig := center.GetAppConfig()
	gormdb.DB = database
	center.SetAppConfig(refreshTestAppConfig{"security:registerEnabled": "false"})
	t.Cleanup(func() {
		gormdb.DB = previousDB
		center.SetAppConfig(previousAppConfig)
	})

	role := &models.Role{Name: "member", Status: enum.Enabled}
	require.NoError(t, database.Create(role).Error)
	user := &models.User{
		UserLogin: models.UserLogin{
			Username: "refresh-user",
			Password: "stored-password",
			RoleID:   role.ID,
			Status:   enum.Enabled,
		},
	}
	require.NoError(t, database.Create(user).Error)

	claimsPrincipal := &models.User{}
	claimsPrincipal.ID = user.ID
	claimsPrincipal.RoleID = role.ID
	require.NoError(t, validateRefreshVerifier(newTestGinCtx(), claimsPrincipal))
}
