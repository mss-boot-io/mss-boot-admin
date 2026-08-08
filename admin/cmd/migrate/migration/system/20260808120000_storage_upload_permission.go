package system

import (
	"fmt"
	"runtime"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260808120000StorageUploadPermission)
}

var storageUploadComponentSeed = adminRouteComponentSeed{
	Name:       "menu.super-permission.storage.upload",
	Path:       "/storage/upload",
	Permission: "storage:upload",
	ParentPath: "/app-config",
}

var storageUploadPermissionSeed = adminRoutePermissionSeed{
	Name:       "api.storage.upload",
	Path:       "/admin/api/storage/upload",
	Method:     "POST",
	Permission: "storage:upload",
	ParentPath: "/storage/upload",
	ParentType: pkg.ComponentAccessType,
}

// _20260808120000StorageUploadPermission makes general-purpose object storage
// an explicit grant. It deliberately does not inherit the capability from a
// page or the historical default role: existing authenticated-self access was
// the security defect this migration is designed to close.
func _20260808120000StorageUploadPermission(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("storage upload permission migration: database is nil")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := requireActiveAppConfigMenu(tx); err != nil {
			return err
		}
		if err := upsertAdminRouteComponent(tx, storageUploadComponentSeed); err != nil {
			return fmt.Errorf("storage upload permission migration: upsert component: %w", err)
		}
		if err := upsertAdminRoutePermission(tx, storageUploadPermissionSeed); err != nil {
			return fmt.Errorf("storage upload permission migration: upsert API metadata: %w", err)
		}
		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error
	}); err != nil {
		return err
	}
	if gormdb.Enforcer != nil {
		if err := gormdb.Enforcer.LoadPolicy(); err != nil {
			return fmt.Errorf("storage upload permission migration: reload Casbin policy: %w", err)
		}
	}
	return nil
}
