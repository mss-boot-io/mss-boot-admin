package system

import (
	"runtime"

	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin-api/cmd/migrate/migration"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	common "github.com/mss-boot-io/mss-boot/pkg/migration/models"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _1691804837583Tables)
}

func _1691804837583Tables(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {

		err := tx.Migrator().AutoMigrate(
			new(models.Role),
			new(models.User),
			new(models.Message),
			new(models.API),
			new(models.Menu),
			new(models.Model),
			new(models.Field),
			new(models.Github),
		)
		if err != nil {
			return err
		}

		return tx.Create(&common.Migration{
			Version: version,
		}).Error
	})
}
