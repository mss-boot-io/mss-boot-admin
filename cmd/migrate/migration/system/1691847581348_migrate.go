package system

import (
	"runtime"
	"time"

	"github.com/mss-boot-io/mss-boot-admin-api/app/admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/enum"

	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _1691847581348Migrate)
}

func _1691847581348Migrate(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {

		expire := time.Now().Add(100 * 365 * 24 * time.Hour)

		tenant := &models.Tenant{
			Name:   "mss-boot-io",
			Remark: "mss-boot-io",
			Status: enum.Enabled,
			Expire: &expire,
			Domains: []*models.TenantDomain{
				{
					Name:   "local",
					Domain: "localhost:8000",
				},
			},
		}
		err := tx.Create(tenant).Error
		if err != nil {
			return err
		}
		err = tx.Table(tenant.TableName()).
			Where("id = ?", tenant.ID).
			Update("default", true).Error
		if err != nil {
			return err
		}

		return migration.Migrate.CreateVersion(tx, version)
	})
}
