package system

import (
	"runtime"

	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _1746193492486Migrate)
}

func _1746193492486Migrate(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {

		err := tx.
			Exec(`CREATE UNIQUE INDEX idx_user_config ON mss_boot_user_configs(tenant_id, user_id, name, "group")`).
			Error
		if err != nil {
			return err
		}

		err = tx.
			Exec(`CREATE UNIQUE INDEX idx_app_config ON mss_boot_app_configs(tenant_id, name, "group")`).
			Error
		if err != nil {
			return err
		}

		return migration.Migrate.CreateVersion(tx, version)
	})
}
