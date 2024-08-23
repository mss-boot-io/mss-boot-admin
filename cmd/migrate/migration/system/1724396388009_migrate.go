package system

import (
	"github.com/mss-boot-io/mss-boot-admin/models"
	"runtime"

	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _1724396388009Migrate)
}

func _1724396388009Migrate(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {

		err := tx.Migrator().AutoMigrate(
			new(models.Task),
		)
		if err != nil {
			return err
		}

		return migration.Migrate.CreateVersion(tx, version)
	})
}
