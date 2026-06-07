package system

import (
	"runtime"

	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260607072000RelaxLegacyTenantColumns)
}

func _20260607072000RelaxLegacyTenantColumns(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := relaxLegacyTenantColumns(tx); err != nil {
			return err
		}
		return migration.Migrate.CreateVersion(tx, version)
	})
}
