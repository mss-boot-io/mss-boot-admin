package system

import (
	"runtime"

	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin-api/cmd/migrate/migration"
	common "github.com/mss-boot-io/mss-boot-admin-api/common/models"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _1691804837583Test)
}

func _1691804837583Test(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {

		err := tx.Migrator().AutoMigrate(
			new(models.Role),
			new(models.User),
			new(models.Message),
		)
		if err != nil {
			return err
		}

		return tx.Create(&common.Migration{
			Version: version,
		}).Error
	})
}
