package system

import (
	"fmt"
	"runtime"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260807150000ConfigRevisions)
}

func _20260807150000ConfigRevisions(db *gorm.DB, version string) error {
	return migrateConfigRevisions(db, version)
}

// migrateConfigRevisions creates the authoritative revision resource used by
// optimistic configuration writes. It deliberately leaves existing app and
// user configuration tables untouched and is safe to resume after partial
// DDL or invoke repeatedly.
func migrateConfigRevisions(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("config revision migration: database is nil")
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		return fmt.Errorf("config revision migration: migrate version table: %w", err)
	}
	if err := db.AutoMigrate(&models.ConfigRevision{}); err != nil {
		return fmt.Errorf("config revision migration: create revision table: %w", err)
	}

	versionRow := &migrationmodels.Migration{}
	versionRow.SetVersion(version)
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "version"}},
		DoNothing: true,
	}).Create(versionRow).Error; err != nil {
		return fmt.Errorf("config revision migration: record version: %w", err)
	}
	return nil
}
