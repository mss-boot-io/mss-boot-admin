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
			if err := tx.Unscoped().
				Where(&models.AppConfig{Group: "security"}).
				Where("name IN ?", retiredV5OAuthConfigNames).
				Delete(&models.AppConfig{}).Error; err != nil {
				return fmt.Errorf("retire V5 browser configuration: delete OAuth settings: %w", err)
			}
			if err := tx.Unscoped().
				Where(&models.AppConfig{Group: "theme", Name: "pwa"}).
				Delete(&models.AppConfig{}).Error; err != nil {
				return fmt.Errorf("retire V5 browser configuration: delete application pwa setting: %w", err)
			}
		}
		if tx.Migrator().HasTable(&models.UserConfig{}) {
			if err := tx.Unscoped().
				Where(&models.UserConfig{Group: "theme", Name: "pwa"}).
				Delete(&models.UserConfig{}).Error; err != nil {
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
