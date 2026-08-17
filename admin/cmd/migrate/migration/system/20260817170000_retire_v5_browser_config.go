package system

import (
	"fmt"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const retireV5BrowserConfigMigrationID migration.MigrationID = "20260817170000"

var retiredV5OAuthConfigNames = []string{
	"githubClientId",
	"githubClientSecret",
	"githubRedirectURI",
	"githubRedirectUrl",
	"githubRedirectURL",
	"githubScope",
	"larkAppId",
	"larkAppSecret",
	"larkRedirectURI",
	"larkRedirectUrl",
}

func init() {
	_ = migration.Migrate.Register(
		retireV5BrowserConfigMigrationID,
		_20260817170000RetireV5BrowserConfig,
	)
}

// _20260817170000RetireV5BrowserConfig removes only configuration keys whose
// runtime consumers were the retired V5 browser protocol. V6 OAuth credentials
// use the explicit BrowserSession names and are intentionally preserved.
func _20260817170000RetireV5BrowserConfig(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("retire V5 browser configuration: database is nil")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var applied int64
		if err := tx.Model(&migrationmodels.Migration{}).
			Where("version = ?", version).
			Count(&applied).Error; err != nil {
			return fmt.Errorf("retire V5 browser configuration: check version: %w", err)
		}
		if applied > 0 {
			return nil
		}

		if tx.Migrator().HasTable(&models.AppConfig{}) {
			if err := deleteExactAppConfigRows(tx, "security", retiredV5OAuthConfigNames); err != nil {
				return fmt.Errorf("retire V5 browser configuration: delete OAuth settings: %w", err)
			}
			if err := deleteExactAppConfigRows(tx, "theme", []string{"pwa"}); err != nil {
				return fmt.Errorf("retire V5 browser configuration: delete application pwa setting: %w", err)
			}
		}
		if tx.Migrator().HasTable(&models.UserConfig{}) {
			if err := deleteExactUserConfigRows(tx, "theme", []string{"pwa"}); err != nil {
				return fmt.Errorf("retire V5 browser configuration: delete user pwa settings: %w", err)
			}
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error; err != nil {
			return fmt.Errorf("retire V5 browser configuration: record version: %w", err)
		}
		return nil
	})
}

func deleteExactAppConfigRows(tx *gorm.DB, group string, names []string) error {
	var candidates []models.AppConfig
	if err := tx.Unscoped().
		Where(&models.AppConfig{Group: group}).
		Where("name IN ?", names).
		Find(&candidates).Error; err != nil {
		return err
	}
	exact := make([]models.AppConfig, 0, len(candidates))
	for i := range candidates {
		if candidates[i].Group == group && containsExactConfigName(names, candidates[i].Name) {
			exact = append(exact, candidates[i])
		}
	}
	if len(exact) == 0 {
		return nil
	}
	return tx.Unscoped().Delete(&exact).Error
}

func deleteExactUserConfigRows(tx *gorm.DB, group string, names []string) error {
	var candidates []models.UserConfig
	if err := tx.Unscoped().
		Where(&models.UserConfig{Group: group}).
		Where("name IN ?", names).
		Find(&candidates).Error; err != nil {
		return err
	}
	exact := make([]models.UserConfig, 0, len(candidates))
	for i := range candidates {
		if candidates[i].Group == group && containsExactConfigName(names, candidates[i].Name) {
			exact = append(exact, candidates[i])
		}
	}
	if len(exact) == 0 {
		return nil
	}
	return tx.Unscoped().Delete(&exact).Error
}

func containsExactConfigName(names []string, candidate string) bool {
	for _, name := range names {
		if candidate == name {
			return true
		}
	}
	return false
}
