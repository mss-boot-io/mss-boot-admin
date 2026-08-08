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

func setupAppConfigSecretPermissionMigrationTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(
		&models.Menu{},
		&models.CasbinRule{},
		&migrationmodels.Migration{},
	))
	return db
}

func TestAppConfigSecretPermissionMigrationIsExplicitAndIdempotent(t *testing.T) {
	db := setupAppConfigSecretPermissionMigrationTest(t)
	parent := &models.Menu{
		Name:       "menu.super-permission.appConfig",
		Path:       "/app-config",
		Method:     "GET",
		Type:       pkg.MenuAccessType,
		Permission: "config:read",
		Status:     enum.Enabled,
	}
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(parent).Error)
	require.NoError(t, db.Create(&models.CasbinRule{
		PType: "p",
		V0:    "existing-app-config-reader",
		V1:    pkg.MenuAccessType.String(),
		V2:    "/app-config",
		V3:    "GET",
	}).Error)

	const version = "20260808130000-test"
	require.NoError(t, _20260808130000AppConfigSecretPermissions(db, version))
	require.NoError(t, _20260808130000AppConfigSecretPermissions(db, version))

	var components []models.Menu
	require.NoError(t, db.Where(
		"type = ? AND path IN ?",
		pkg.ComponentAccessType,
		[]string{"/app-config/secrets/read", "/app-config/secrets/write"},
	).Order("path").Find(&components).Error)
	require.Len(t, components, 2)
	require.Equal(t, "/app-config/secrets/read", components[0].Path)
	require.Equal(t, "app-config:secret-read", components[0].Permission)
	require.Equal(t, "menu.super-permission.appConfig.secrets.read", components[0].Name)
	require.Equal(t, "/app-config/secrets/write", components[1].Path)
	require.Equal(t, "app-config:secret-write", components[1].Permission)
	require.Equal(t, "menu.super-permission.appConfig.secrets.write", components[1].Name)
	for i := range components {
		require.Equal(t, parent.ID, components[i].ParentID)
		require.Equal(t, "GET", components[i].Method)
		require.Equal(t, enum.Enabled, components[i].Status)
		require.True(t, components[i].HideInMenu)
	}

	var policyCount int64
	require.NoError(t, db.Model(&models.CasbinRule{}).Count(&policyCount).Error)
	require.EqualValues(t, 1, policyCount, "field-level permissions must never be implicitly granted")
	var componentPolicyCount int64
	require.NoError(t, db.Model(&models.CasbinRule{}).
		Where("v1 = ? AND v2 IN ?", pkg.ComponentAccessType.String(), []string{
			"/app-config/secrets/read",
			"/app-config/secrets/write",
		}).
		Count(&componentPolicyCount).Error)
	require.Zero(t, componentPolicyCount)

	var versionCount int64
	require.NoError(t, db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&versionCount).Error)
	require.EqualValues(t, 1, versionCount)
}

func TestAppConfigSecretPermissionMigrationFailsClosedWithoutActiveParent(t *testing.T) {
	db := setupAppConfigSecretPermissionMigrationTest(t)
	const version = "20260808130000-missing-parent"

	err := _20260808130000AppConfigSecretPermissions(db, version)
	require.Error(t, err)

	var componentCount int64
	require.NoError(t, db.Model(&models.Menu{}).
		Where("type = ?", pkg.ComponentAccessType).
		Count(&componentCount).Error)
	require.Zero(t, componentCount)

	var versionCount int64
	require.NoError(t, db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&versionCount).Error)
	require.Zero(t, versionCount)
}
