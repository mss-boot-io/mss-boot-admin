package system

import (
	"fmt"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestStorageUploadPermissionMigrationIsExplicitAndIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Menu{},
		&models.CasbinRule{},
		&migrationmodels.Migration{},
	))

	parent := &models.Menu{
		Name:       "menu.super-permission.appConfig",
		Path:       "/app-config",
		Method:     "GET",
		Type:       pkg.MenuAccessType,
		Permission: "config:read",
		Status:     enum.Enabled,
	}
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(parent).Error)

	const version = "20260808120000"
	require.NoError(t, _20260808120000StorageUploadPermission(db, version))
	require.NoError(t, _20260808120000StorageUploadPermission(db, version))

	var component models.Menu
	require.NoError(t, db.Where("type = ? AND path = ?", pkg.ComponentAccessType, "/storage/upload").First(&component).Error)
	require.Equal(t, parent.ID, component.ParentID)
	require.Equal(t, "storage:upload", component.Permission)
	require.Equal(t, enum.Enabled, component.Status)

	var api models.Menu
	require.NoError(t, db.Where(
		"type = ? AND path = ? AND method = ?",
		pkg.APIAccessType,
		"/admin/api/storage/upload",
		"POST",
	).First(&api).Error)
	require.Equal(t, component.ID, api.ParentID)
	require.Equal(t, "storage:upload", api.Permission)

	var policyCount int64
	require.NoError(t, db.Model(&models.CasbinRule{}).Count(&policyCount).Error)
	require.Zero(t, policyCount, "the migration must not grant storage upload implicitly")

	var componentCount, apiCount, versionCount int64
	require.NoError(t, db.Model(&models.Menu{}).Where("type = ? AND path = ?", pkg.ComponentAccessType, "/storage/upload").Count(&componentCount).Error)
	require.NoError(t, db.Model(&models.Menu{}).Where("type = ? AND path = ? AND method = ?", pkg.APIAccessType, "/admin/api/storage/upload", "POST").Count(&apiCount).Error)
	require.NoError(t, db.Model(&migrationmodels.Migration{}).Where("version = ?", version).Count(&versionCount).Error)
	require.EqualValues(t, 1, componentCount)
	require.EqualValues(t, 1, apiCount)
	require.EqualValues(t, 1, versionCount)
}
