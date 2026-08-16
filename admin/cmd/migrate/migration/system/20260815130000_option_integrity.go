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

const optionIntegrityMigrationID migration.MigrationID = "20260815130000"

var optionVersionSnapshotColumns = []string{
	"Category",
	"Name",
	"DisplayName",
	"Description",
	"Remark",
	"OptionStatus",
	"BuiltIn",
}

func init() {
	_ = migration.Migrate.Register(optionIntegrityMigrationID, _20260815130000OptionIntegrity)
}

func _20260815130000OptionIntegrity(db *gorm.DB, version string) error {
	return migrateOptionIntegrity(db, version)
}

// migrateOptionIntegrity expands option history from an items-only record to a
// complete prior-resource snapshot, then repairs only legacy values with one
// unambiguous canonical meaning. Business keys, labels and values are never
// guessed; malformed dictionaries fail the migration closed for operator
// review.
func migrateOptionIntegrity(db *gorm.DB, version string) error {
	if db == nil {
		return errors.New("option integrity migration: database is nil")
	}
	db = db.Clauses(dbresolver.Write)
	if !db.Migrator().HasTable(&models.Option{}) {
		return errors.New("option integrity migration: option table is missing")
	}
	if !db.Migrator().HasTable(&models.OptionVersion{}) {
		return errors.New("option integrity migration: option version table is missing")
	}
	for _, field := range []string{"ID", "Category", "Name", "Items", "Status", "Version", "BuiltIn"} {
		if !db.Migrator().HasColumn(&models.Option{}, field) {
			return fmt.Errorf("option integrity migration: required option column %s is missing", field)
		}
	}
	for _, field := range []string{"OptionID", "Version", "Items", "ChangedBy", "ChangeNote", "Status"} {
		if !db.Migrator().HasColumn(&models.OptionVersion{}, field) {
			return fmt.Errorf("option integrity migration: required option version column %s is missing", field)
		}
	}

	return db.Transaction(func(tx *gorm.DB) error {
		tx = tx.Clauses(dbresolver.Write).Session(&gorm.Session{SkipHooks: true})
		for _, field := range optionVersionSnapshotColumns {
			if tx.Migrator().HasColumn(&models.OptionVersion{}, field) {
				continue
			}
			if err := tx.Migrator().AddColumn(&models.OptionVersion{}, field); err != nil {
				return fmt.Errorf("option integrity migration: add snapshot column %s: %w", field, err)
			}
		}

		options := make([]models.Option, 0)
		if err := tx.Unscoped().Order("id ASC").Find(&options).Error; err != nil {
			return fmt.Errorf("option integrity migration: load options: %w", err)
		}
		if len(options) > serviceOptionMigrationCapacity {
			return fmt.Errorf("option integrity migration: %d records exceed supported capacity", len(options))
		}
		for i := range options {
			if err := repairLegacyOption(tx, &options[i]); err != nil {
				return err
			}
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error
	})
}

// Keep the migration independent from the runtime service package while
// enforcing the same reviewed record ceiling.
const serviceOptionMigrationCapacity = 256

func repairLegacyOption(tx *gorm.DB, option *models.Option) error {
	if option == nil || strings.TrimSpace(option.ID) == "" {
		return errors.New("option integrity migration: option ID is empty")
	}
	changed := false
	if option.Status == "" {
		option.Status = enum.Enabled
		changed = true
	}
	if option.Version <= 0 {
		option.Version = 1
		changed = true
	}
	if option.Items == nil {
		empty := models.OptionItems{}
		option.Items = &empty
		changed = true
	}
	seen := make(map[string]struct{}, len(*option.Items))
	for _, item := range *option.Items {
		if item == nil {
			continue
		}
		itemID := strings.TrimSpace(item.ID)
		_, duplicate := seen[itemID]
		if itemID == "" || duplicate {
			for {
				itemID = strings.ReplaceAll(uuid.NewString(), "-", "")
				if _, exists := seen[itemID]; !exists {
					break
				}
			}
			item.ID = itemID
			changed = true
		}
		seen[itemID] = struct{}{}
	}
	if err := option.ValidateStored(); err != nil {
		return fmt.Errorf("option integrity migration: option %s is invalid: %w", option.ID, err)
	}
	if !changed {
		return nil
	}
	result := tx.Unscoped().Table(option.TableName()).Where("id = ?", option.ID).Updates(map[string]any{
		"status":  option.Status,
		"version": option.Version,
		"items":   option.Items,
	})
	if result.Error != nil {
		return fmt.Errorf("option integrity migration: repair option %s: %w", option.ID, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("option integrity migration: repair option %s affected %d rows", option.ID, result.RowsAffected)
	}
	return nil
}
