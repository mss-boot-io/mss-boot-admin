package system

import (
	"fmt"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const presentationProfilesMigrationID migration.MigrationID = "20260824120000"

func init() {
	_ = migration.Migrate.Register(presentationProfilesMigrationID, _20260824120000PresentationProfiles)
}

func _20260824120000PresentationProfiles(db *gorm.DB, version string) error {
	return migratePresentationProfiles(db, version)
}

// migratePresentationProfiles creates only the P1 aggregate and append-only
// revision resources. Existing configuration and business tables are not
// inspected or rewritten, which keeps both fresh and upgrade execution
// resumable after a partially completed DDL operation.
func migratePresentationProfiles(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("presentation profile migration: database is nil")
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		return fmt.Errorf("presentation profile migration: migrate version table: %w", err)
	}
	if err := db.AutoMigrate(&models.PresentationProfile{}); err != nil {
		return fmt.Errorf("presentation profile migration: create aggregate table: %w", err)
	}
	if err := db.AutoMigrate(&models.PresentationRevision{}); err != nil {
		return fmt.Errorf("presentation profile migration: create revision table: %w", err)
	}

	versionRow := &migrationmodels.Migration{}
	versionRow.SetVersion(version)
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "version"}},
		DoNothing: true,
	}).Create(versionRow).Error; err != nil {
		return fmt.Errorf("presentation profile migration: record version: %w", err)
	}
	return nil
}
