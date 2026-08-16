package system

import (
	"fmt"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const accountReauthenticationMigrationID migration.MigrationID = "20260816163000"

func init() {
	_ = migration.Migrate.Register(
		accountReauthenticationMigrationID,
		_20260816163000AccountReauthentication,
	)
}

func _20260816163000AccountReauthentication(db *gorm.DB, version string) error {
	return migrateAccountReauthentication(db, version)
}

// migrateAccountReauthentication adds only server-owned, nullable/defaulted
// step-up state. Existing sessions continue ordinary requests but do not gain
// recent-authentication proof merely because an upgrade was deployed.
func migrateAccountReauthentication(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("account reauthentication migration: database is nil")
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		return fmt.Errorf("account reauthentication migration: migrate version table: %w", err)
	}
	if !db.Migrator().HasTable(&models.UserSession{}) {
		if err := db.AutoMigrate(&models.UserSession{}); err != nil {
			return fmt.Errorf("account reauthentication migration: create session table: %w", err)
		}
	} else {
		for _, field := range []string{
			"ReauthenticatedAt",
			"ReauthFailedAttempts",
			"ReauthFailedAt",
			"ReauthLockedUntil",
		} {
			if db.Migrator().HasColumn(&models.UserSession{}, field) {
				continue
			}
			if err := db.Migrator().AddColumn(&models.UserSession{}, field); err != nil {
				return fmt.Errorf("account reauthentication migration: add %s: %w", field, err)
			}
		}
	}
	return db.Transaction(func(tx *gorm.DB) error {
		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error; err != nil {
			return fmt.Errorf("account reauthentication migration: record version: %w", err)
		}
		return nil
	})
}
