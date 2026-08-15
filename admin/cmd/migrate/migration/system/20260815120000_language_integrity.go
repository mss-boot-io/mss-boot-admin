package system

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"
)

const languageIntegrityMigrationID migration.MigrationID = "20260815120000"

func init() {
	_ = migration.Migrate.Register(
		languageIntegrityMigrationID,
		_20260815120000LanguageIntegrity,
	)
}

func _20260815120000LanguageIntegrity(db *gorm.DB, version string) error {
	return migrateLanguageIntegrity(db, version)
}

// migrateLanguageIntegrity repairs the two legacy values that became possible
// when Language.BeforeCreate shadowed the embedded model hook: an empty primary
// key and an empty status. It intentionally does not rewrite locale names or
// definitions because those values require a product decision when malformed.
func migrateLanguageIntegrity(db *gorm.DB, version string) error {
	if db == nil {
		return errors.New("language integrity migration: database is nil")
	}
	db = db.Clauses(dbresolver.Write)
	if !db.Migrator().HasTable(&models.Language{}) {
		return errors.New("language integrity migration: language table is missing")
	}
	if !db.Migrator().HasColumn(&models.Language{}, "ID") ||
		!db.Migrator().HasColumn(&models.Language{}, "Status") {
		return errors.New("language integrity migration: required language columns are missing")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		tx = tx.Clauses(dbresolver.Write).Session(&gorm.Session{SkipHooks: true})
		table := (&models.Language{}).TableName()
		var blankIDCount int64
		if err := tx.Table(table).
			Where("id IS NULL OR TRIM(id) = ''").
			Count(&blankIDCount).Error; err != nil {
			return errors.New("language integrity migration: count empty IDs failed")
		}
		if blankIDCount > 1 {
			return fmt.Errorf(
				"language integrity migration: %d records have an empty ID",
				blankIDCount,
			)
		}
		if blankIDCount == 1 {
			newID := strings.ReplaceAll(uuid.NewString(), "-", "")
			result := tx.Table(table).
				Where("id IS NULL OR TRIM(id) = ''").
				UpdateColumn("id", newID)
			if result.Error != nil || result.RowsAffected != 1 {
				return errors.New("language integrity migration: repair empty ID failed")
			}
		}

		if err := tx.Table(table).
			Where("status IS NULL OR TRIM(status) = ''").
			UpdateColumn("status", enum.Enabled).Error; err != nil {
			return errors.New("language integrity migration: repair empty status failed")
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error
	})
}
