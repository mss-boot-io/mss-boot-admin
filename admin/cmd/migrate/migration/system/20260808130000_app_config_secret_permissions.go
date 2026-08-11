package system

import (
	"fmt"
	"runtime"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetV100Version(fileName, _20260808130000AppConfigSecretPermissions)
}

var appConfigSecretComponentSeeds = []adminRouteComponentSeed{
	{
		Name:       "menu.super-permission.appConfig.secrets.read",
		Path:       "/app-config/secrets/read",
		Permission: "app-config:secret-read",
		ParentPath: "/app-config",
	},
	{
		Name:       "menu.super-permission.appConfig.secrets.write",
		Path:       "/app-config/secrets/write",
		Permission: "app-config:secret-write",
		ParentPath: "/app-config",
	},
}

// _20260808130000AppConfigSecretPermissions introduces independently
// assignable field-level capabilities. It intentionally creates no Casbin
// policy rows: administrators must grant read and write access explicitly.
func _20260808130000AppConfigSecretPermissions(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("app-config secret permission migration: database is nil")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := requireActiveAppConfigMenu(tx); err != nil {
			return err
		}
		for i := range appConfigSecretComponentSeeds {
			if err := upsertAdminRouteComponent(tx, appConfigSecretComponentSeeds[i]); err != nil {
				return fmt.Errorf("app-config secret permission migration: upsert component: %w", err)
			}
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error
	})
}
