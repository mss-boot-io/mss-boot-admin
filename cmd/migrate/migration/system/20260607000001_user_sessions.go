package system

import (
	"runtime"

	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/models"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260607000001UserSessions)
}

func _20260607000001UserSessions(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Migrator().AutoMigrate(new(models.UserSession)); err != nil {
			return err
		}
		return migration.Migrate.CreateVersion(tx, version)
	})
}
