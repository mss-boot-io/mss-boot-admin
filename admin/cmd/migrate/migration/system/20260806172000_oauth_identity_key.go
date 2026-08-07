package system

import (
	"fmt"
	"runtime"
	"sort"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const oauthIdentityKeyUniqueIndex = "ux_user_oauth2_identity_key"

type legacyOAuthIdentityRow struct {
	ID          string            `gorm:"column:id"`
	Provider    pkg.LoginProvider `gorm:"column:provider"`
	OpenID      string            `gorm:"column:open_id"`
	UnionID     string            `gorm:"column:union_id"`
	IdentityKey *string           `gorm:"column:identity_key"`
}

type oauthIdentityBackfill struct {
	ID  string
	Key string
}

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260806172000OAuthIdentityKey)
}

func _20260806172000OAuthIdentityKey(db *gorm.DB, version string) error {
	return migrateOAuthIdentityKeys(db, version)
}

// migrateOAuthIdentityKeys expands the OAuth binding schema with a nullable,
// provider-scoped key. Every active legacy row is validated and checked for
// duplicates before any data is changed. Soft-deleted rows are deliberately
// left NULL so the single unique index behaves consistently on MySQL and
// PostgreSQL without relying on empty-string or composite-index semantics.
func migrateOAuthIdentityKeys(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("oauth identity key migration: database is nil")
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		return fmt.Errorf("oauth identity key migration: migrate version table: %w", err)
	}

	if !db.Migrator().HasTable(&models.UserOAuth2{}) {
		if err := db.AutoMigrate(&models.UserOAuth2{}); err != nil {
			return fmt.Errorf("oauth identity key migration: create OAuth identity table: %w", err)
		}
	} else if !db.Migrator().HasColumn(&models.UserOAuth2{}, "IdentityKey") {
		if err := db.Migrator().AddColumn(&models.UserOAuth2{}, "IdentityKey"); err != nil {
			return fmt.Errorf("oauth identity key migration: add identity key column: %w", err)
		}
	}
	if err := enforceOAuthIdentityKeyComparison(db); err != nil {
		return err
	}

	backfills, err := prepareOAuthIdentityBackfill(db)
	if err != nil {
		return err
	}
	hasDeletedAt := db.Migrator().HasColumn(&models.UserOAuth2{}, "DeletedAt")

	return db.Transaction(func(tx *gorm.DB) error {
		if hasDeletedAt {
			if err := tx.Table((&models.UserOAuth2{}).TableName()).
				Where("deleted_at IS NOT NULL").
				UpdateColumn("identity_key", nil).Error; err != nil {
				return fmt.Errorf("oauth identity key migration: clear deleted identity keys: %w", err)
			}
		}

		for _, row := range backfills {
			query := tx.Table((&models.UserOAuth2{}).TableName()).Where("id = ?", row.ID)
			if hasDeletedAt {
				query = query.Where("deleted_at IS NULL")
			}
			result := query.UpdateColumn("identity_key", row.Key)
			if result.Error != nil {
				return fmt.Errorf("oauth identity key migration: backfill row %s: %w", row.ID, result.Error)
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("oauth identity key migration: backfill row %s affected %d rows", row.ID, result.RowsAffected)
			}
		}

		activeQuery := tx.Table((&models.UserOAuth2{}).TableName())
		if hasDeletedAt {
			activeQuery = activeQuery.Where("deleted_at IS NULL")
		}
		var missing int64
		if err := activeQuery.
			Where("identity_key IS NULL OR identity_key = ''").
			Count(&missing).Error; err != nil {
			return fmt.Errorf("oauth identity key migration: verify active keys: %w", err)
		}
		if missing != 0 {
			return fmt.Errorf("oauth identity key migration: %d active rows have no identity key", missing)
		}

		if !tx.Migrator().HasIndex(&models.UserOAuth2{}, oauthIdentityKeyUniqueIndex) {
			if err := tx.Migrator().CreateIndex(&models.UserOAuth2{}, oauthIdentityKeyUniqueIndex); err != nil {
				return fmt.Errorf("oauth identity key migration: create unique index: %w", err)
			}
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error; err != nil {
			return fmt.Errorf("oauth identity key migration: record version: %w", err)
		}
		return nil
	})
}

// enforceOAuthIdentityKeyComparison keeps opaque provider identifiers
// case-sensitive across dialects. PostgreSQL and SQLite already use exact
// comparison for this column; MySQL's common utf8mb4_*_ci defaults do not.
func enforceOAuthIdentityKeyComparison(db *gorm.DB) error {
	switch db.Dialector.Name() {
	case "mysql":
		var current struct {
			CollationName string `gorm:"column:COLLATION_NAME"`
		}
		if err := db.Raw(
			`SELECT COLLATION_NAME
			 FROM information_schema.COLUMNS
			 WHERE TABLE_SCHEMA = DATABASE()
			   AND TABLE_NAME = ?
			   AND COLUMN_NAME = ?`,
			(&models.UserOAuth2{}).TableName(),
			"identity_key",
		).Scan(&current).Error; err != nil {
			return fmt.Errorf("oauth identity key migration: inspect MySQL collation: %w", err)
		}
		if current.CollationName == "utf8mb4_bin" {
			return nil
		}
		if err := db.Exec(
			"ALTER TABLE `mss_boot_user_oauth2` " +
				"MODIFY COLUMN `identity_key` VARCHAR(96) " +
				"CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NULL",
		).Error; err != nil {
			return fmt.Errorf("oauth identity key migration: enforce MySQL binary collation: %w", err)
		}
	case "sqlserver":
		if err := db.Exec(
			"ALTER TABLE mss_boot_user_oauth2 " +
				"ALTER COLUMN identity_key VARCHAR(96) COLLATE Latin1_General_100_BIN2 NULL",
		).Error; err != nil {
			return fmt.Errorf("oauth identity key migration: enforce SQL Server binary collation: %w", err)
		}
	case "postgres", "sqlite":
		return nil
	default:
		return fmt.Errorf("oauth identity key migration: unsupported database dialect %q", db.Dialector.Name())
	}
	return nil
}

func prepareOAuthIdentityBackfill(db *gorm.DB) ([]oauthIdentityBackfill, error) {
	query := db.Table((&models.UserOAuth2{}).TableName()).
		Select("id", "provider", "open_id", "union_id", "identity_key")
	if db.Migrator().HasColumn(&models.UserOAuth2{}, "DeletedAt") {
		query = query.Where("deleted_at IS NULL")
	}
	rows := make([]legacyOAuthIdentityRow, 0)
	if err := query.Order("id ASC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("oauth identity key migration: read active identities: %w", err)
	}

	backfills := make([]oauthIdentityBackfill, 0, len(rows))
	seen := make(map[string]string, len(rows))
	for _, row := range rows {
		key, err := models.UserOAuthIdentityKey(row.Provider, row.OpenID, row.UnionID)
		if err != nil {
			return nil, fmt.Errorf("oauth identity key migration: active row %s is invalid: %w", row.ID, err)
		}
		if previousID, exists := seen[key]; exists {
			ids := []string{previousID, row.ID}
			sort.Strings(ids)
			return nil, fmt.Errorf(
				"oauth identity key migration: duplicate active %s identity on rows %s and %s",
				row.Provider,
				ids[0],
				ids[1],
			)
		}
		seen[key] = row.ID
		if row.IdentityKey == nil || *row.IdentityKey != key {
			backfills = append(backfills, oauthIdentityBackfill{ID: row.ID, Key: key})
		}
	}
	return backfills, nil
}
