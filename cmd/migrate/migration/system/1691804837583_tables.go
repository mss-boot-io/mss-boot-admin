package system

import (
	"runtime"

	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/app/admin/models"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _1691804837583Tables)
}

func _1691804837583Tables(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {

		err := tx.Migrator().AutoMigrate(
			new(models.Tenant),
			new(models.TenantDomain),
			new(models.AppConfig),
			new(models.SystemConfig),
			new(models.Role),
			new(models.User),
			new(models.UserOAuth2),
			new(models.Department),
			new(models.Department),
			new(models.API),
			new(models.Menu),
			new(models.Model),
			new(models.Field),
			new(models.Task),
			new(models.TaskRun),
			new(models.TaskRunLog),
			new(models.Language),
			new(models.Notice),
			new(models.Option),
			new(models.Statistics),
		)
		if err != nil {
			return err
		}

		return migration.Migrate.CreateVersion(tx, version)
	})
}
