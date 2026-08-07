package system

import (
	"fmt"
	"log/slog"
	"runtime"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260806210000MonitorReadPermission)
}

func _20260806210000MonitorReadPermission(db *gorm.DB, version string) error {
	affected, err := migrateMonitorReadPermission(db, version)
	if err != nil {
		return err
	}
	if len(affected) > 0 {
		slog.Info(
			"monitor read permission inherited from existing welcome grants",
			"affectedCount", len(affected),
			"affectedRoles", affected,
		)
	}
	return nil
}

func migrateMonitorReadPermission(
	db *gorm.DB,
	version string,
) ([]adminRoutePermissionAffectedRole, error) {
	if db == nil {
		return nil, fmt.Errorf("monitor permission migration: database is nil")
	}
	var affected []adminRoutePermissionAffectedRole
	err := db.Transaction(func(tx *gorm.DB) error {
		var alreadyApplied int64
		if err := tx.Model(&migrationmodels.Migration{}).
			Where("version = ?", version).
			Count(&alreadyApplied).Error; err != nil {
			return fmt.Errorf("monitor permission migration: check version: %w", err)
		}
		if alreadyApplied > 0 {
			return nil
		}

		var activeWelcome int64
		if err := tx.Model(&models.Menu{}).
			Where(
				"type = ? AND path = ? AND status = ?",
				monitorReadPermissionSeed.ParentType,
				monitorReadPermissionSeed.ParentPath,
				enum.Enabled,
			).
			Count(&activeWelcome).Error; err != nil {
			return fmt.Errorf("monitor permission migration: inspect welcome menu: %w", err)
		}
		if activeWelcome > 0 {
			if err := upsertAdminRoutePermission(tx, monitorReadPermissionSeed); err != nil {
				return fmt.Errorf("monitor permission migration: upsert permission metadata: %w", err)
			}
			inherited, err := inheritExistingAdminRoutePermission(tx, monitorReadPermissionSeed)
			if err != nil {
				return err
			}
			affected = inherited
		} else {
			slog.Warn(
				"monitor read permission parent is unavailable; leaving the API fail-closed",
				"parentType", monitorReadPermissionSeed.ParentType.String(),
				"parentPath", monitorReadPermissionSeed.ParentPath,
			)
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error
	})
	if err != nil {
		return nil, err
	}
	if gormdb.Enforcer != nil {
		if err := gormdb.Enforcer.LoadPolicy(); err != nil {
			return affected, fmt.Errorf("monitor permission migration: reload Casbin policy: %w", err)
		}
	}
	return affected, nil
}
