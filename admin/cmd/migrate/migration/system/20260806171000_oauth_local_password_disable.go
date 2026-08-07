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
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260806171000OAuthLocalPasswordDisable)
}

func _20260806171000OAuthLocalPasswordDisable(db *gorm.DB, version string) error {
	return migrateOAuthLocalPasswordDisable(db, version)
}

// migrateOAuthLocalPasswordDisable disables local-password login for every
// OAuth-linked account. Legacy data cannot distinguish an OAuth-created user
// from a local user that was linked later, so the migration deliberately
// chooses the secure fail-closed behavior. A password reset clears the flag
// and establishes a new local credential.
func migrateOAuthLocalPasswordDisable(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("oauth local password migration: database is nil")
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		return fmt.Errorf("oauth local password migration: migrate version table: %w", err)
	}
	if !db.Migrator().HasTable(&models.User{}) {
		if err := db.AutoMigrate(&models.User{}); err != nil {
			return fmt.Errorf("oauth local password migration: create user table: %w", err)
		}
	} else if !db.Migrator().HasColumn(&models.User{}, "LocalPasswordDisabled") {
		if err := db.Migrator().AddColumn(&models.User{}, "LocalPasswordDisabled"); err != nil {
			return fmt.Errorf("oauth local password migration: add local password flag: %w", err)
		}
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable(&models.UserOAuth2{}) {
			boundUsers := tx.Table((&models.UserOAuth2{}).TableName()).
				Select("user_id").
				Where("user_id <> ?", "")
			if err := tx.Table((&models.User{}).TableName()).
				Where("id IN (?)", boundUsers).
				UpdateColumn("local_password_disabled", true).Error; err != nil {
				return fmt.Errorf("oauth local password migration: disable linked local credentials: %w", err)
			}
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error; err != nil {
			return fmt.Errorf("oauth local password migration: record version: %w", err)
		}
		return nil
	})
}
