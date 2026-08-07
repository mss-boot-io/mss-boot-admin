package system

import (
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
)

const monitorReadPermissionTestVersion = "2026080621000"

func TestMonitorReadPermissionMigrationIsAssignableAndUpgradeSafe(t *testing.T) {
	db, parentIDs := setupAdminRoutePermissionMigrationTest(t)
	if err := db.Create(&models.CasbinRule{
		PType: "p",
		V0:    "role-welcome",
		V1:    pkg.MenuAccessType.String(),
		V2:    "/welcome",
		V3:    "GET",
	}).Error; err != nil {
		t.Fatalf("create welcome grant: %v", err)
	}
	if err := db.Create(&models.CasbinRule{
		PType: "p",
		V0:    "role-unrelated",
		V1:    pkg.MenuAccessType.String(),
		V2:    "/users",
		V3:    "GET",
	}).Error; err != nil {
		t.Fatalf("create unrelated grant: %v", err)
	}

	affected, err := migrateMonitorReadPermission(db, monitorReadPermissionTestVersion)
	if err != nil {
		t.Fatalf("migrate monitor read permission: %v", err)
	}
	if len(affected) != 1 || affected[0].RoleID != "role-welcome" {
		t.Fatalf("affected roles = %#v, want role-welcome", affected)
	}
	permission := accessAPIByRoute(t, db, "GET", "/admin/api/monitor")
	if permission.ParentID != parentIDs["/welcome"] || permission.Permission != "monitor:read" ||
		!permission.HideInMenu {
		t.Fatalf("monitor permission metadata = %+v", permission)
	}
	assertRemovalPolicyPresent(
		t,
		db,
		"role-welcome",
		pkg.APIAccessType.String(),
		"/admin/api/monitor",
		"GET",
	)
	assertRemovalPolicyMissing(
		t,
		db,
		"role-unrelated",
		pkg.APIAccessType.String(),
		"/admin/api/monitor",
		"GET",
	)

	secondAffected, err := migrateMonitorReadPermission(db, monitorReadPermissionTestVersion)
	if err != nil {
		t.Fatalf("rerun monitor read permission migration: %v", err)
	}
	if len(secondAffected) != 0 {
		t.Fatalf("rerun affected roles = %#v, want none", secondAffected)
	}
	var versionCount int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", monitorReadPermissionTestVersion).
		Count(&versionCount).Error; err != nil {
		t.Fatalf("count monitor permission migration version: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("monitor permission migration version count = %d, want 1", versionCount)
	}
}

func TestMonitorReadPermissionMigrationSkipsUnavailableWelcomeParent(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, db *gorm.DB)
	}{
		{
			name: "missing",
			mutate: func(t *testing.T, db *gorm.DB) {
				if err := db.Session(&gorm.Session{SkipHooks: true}).
					Where("type = ? AND path = ?", pkg.MenuAccessType, "/welcome").
					Unscoped().Delete(&models.Menu{}).Error; err != nil {
					t.Fatalf("delete welcome parent: %v", err)
				}
			},
		},
		{
			name: "renamed",
			mutate: func(t *testing.T, db *gorm.DB) {
				if err := db.Model(&models.Menu{}).
					Where("type = ? AND path = ?", pkg.MenuAccessType, "/welcome").
					Update("path", "/dashboard").Error; err != nil {
					t.Fatalf("rename welcome parent: %v", err)
				}
			},
		},
		{
			name: "soft deleted",
			mutate: func(t *testing.T, db *gorm.DB) {
				if err := db.Session(&gorm.Session{SkipHooks: true}).
					Where("type = ? AND path = ?", pkg.MenuAccessType, "/welcome").
					Delete(&models.Menu{}).Error; err != nil {
					t.Fatalf("soft-delete welcome parent: %v", err)
				}
			},
		},
		{
			name: "disabled",
			mutate: func(t *testing.T, db *gorm.DB) {
				if err := db.Model(&models.Menu{}).
					Where("type = ? AND path = ?", pkg.MenuAccessType, "/welcome").
					Update("status", enum.Disabled).Error; err != nil {
					t.Fatalf("disable welcome parent: %v", err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, _ := setupAdminRoutePermissionMigrationTest(t)
			test.mutate(t, db)
			if err := db.Create(&models.CasbinRule{
				PType: "p",
				V0:    "role-former-welcome",
				V1:    pkg.MenuAccessType.String(),
				V2:    "/welcome",
				V3:    "GET",
			}).Error; err != nil {
				t.Fatalf("create former welcome grant: %v", err)
			}

			affected, err := migrateMonitorReadPermission(db, monitorReadPermissionTestVersion)
			if err != nil {
				t.Fatalf("migrate without an active welcome parent: %v", err)
			}
			if len(affected) != 0 {
				t.Fatalf("affected roles = %#v, want none", affected)
			}
			var permissionCount int64
			if err := db.Unscoped().Model(&models.Menu{}).
				Where("type = ? AND path = ? AND method = ?", pkg.APIAccessType, "/admin/api/monitor", "GET").
				Count(&permissionCount).Error; err != nil {
				t.Fatalf("count monitor permission metadata: %v", err)
			}
			if permissionCount != 0 {
				t.Fatalf("monitor permission metadata count = %d, want 0", permissionCount)
			}
			assertRemovalPolicyMissing(
				t,
				db,
				"role-former-welcome",
				pkg.APIAccessType.String(),
				"/admin/api/monitor",
				"GET",
			)

			var versionCount int64
			if err := db.Model(&migrationmodels.Migration{}).
				Where("version = ?", monitorReadPermissionTestVersion).
				Count(&versionCount).Error; err != nil {
				t.Fatalf("count monitor permission migration version: %v", err)
			}
			if versionCount != 1 {
				t.Fatalf("monitor permission migration version count = %d, want 1", versionCount)
			}
			if rerunAffected, err := migrateMonitorReadPermission(db, monitorReadPermissionTestVersion); err != nil {
				t.Fatalf("rerun skipped monitor permission migration: %v", err)
			} else if len(rerunAffected) != 0 {
				t.Fatalf("rerun affected roles = %#v, want none", rerunAffected)
			}
		})
	}
}
