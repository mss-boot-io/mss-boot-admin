package system

import (
	"database/sql"
	"fmt"
	"runtime"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const userAuthTokenMigrationBatchSize = 100

type legacyUserAuthTokenRow struct {
	ID          string         `gorm:"column:id"`
	LegacyToken sql.NullString `gorm:"column:token"`
	TokenHash   sql.NullString `gorm:"column:token_hash"`
	Fingerprint sql.NullString `gorm:"column:fingerprint"`
	Revoked     bool           `gorm:"column:revoked"`
}

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetV100Version(fileName, _20260806170000UserAuthTokenHash)
}

func _20260806170000UserAuthTokenHash(db *gorm.DB, version string) error {
	return migrateUserAuthTokenHashes(db, version)
}

// migrateUserAuthTokenHashes expands the legacy schema, backfills each
// recoverable token digest in process, and clears plaintext in the same row
// update. It is restartable after partial MySQL DDL and safe to invoke twice.
func migrateUserAuthTokenHashes(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("user auth token hash migration: database is nil")
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		return fmt.Errorf("user auth token hash migration: migrate version table: %w", err)
	}
	if err := db.AutoMigrate(&models.UserAuthToken{}); err != nil {
		return fmt.Errorf("user auth token hash migration: expand token schema: %w", err)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		secureTx := tx
		if tx.Logger != nil {
			secureTx = tx.Session(&gorm.Session{Logger: tx.Logger.LogMode(logger.Silent)})
		}

		if err := backfillUserAuthTokenHashes(secureTx); err != nil {
			return err
		}
		if err := assertUserAuthTokenPlaintextCleared(secureTx); err != nil {
			return err
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		return secureTx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error
	})
}

func backfillUserAuthTokenHashes(tx *gorm.DB) error {
	lastID := ""
	hasCursor := false
	for {
		rows := make([]legacyUserAuthTokenRow, 0, userAuthTokenMigrationBatchSize)
		query := tx.Table((&models.UserAuthToken{}).TableName()).
			Select("id", "token", "token_hash", "fingerprint", "revoked").
			Order("id ASC").
			Limit(userAuthTokenMigrationBatchSize)
		if hasCursor {
			query = query.Where("id > ?", lastID)
		}
		if err := query.Find(&rows).Error; err != nil {
			return fmt.Errorf("user auth token hash migration: read legacy batch: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}

		for i := range rows {
			if err := hardenLegacyUserAuthTokenRow(tx, rows[i]); err != nil {
				return err
			}
		}
		lastID = rows[len(rows)-1].ID
		hasCursor = true
	}
}

func hardenLegacyUserAuthTokenRow(tx *gorm.DB, row legacyUserAuthTokenRow) error {
	legacyToken := row.LegacyToken.String
	tokenHash := row.TokenHash.String
	updates := map[string]any{"token": ""}

	switch {
	case legacyToken != "":
		tokenHash = models.HashUserAuthToken(legacyToken)
		updates["token_hash"] = tokenHash
		updates["fingerprint"] = models.UserAuthTokenFingerprint(tokenHash)
	case models.IsValidUserAuthTokenHash(tokenHash):
		fingerprint := models.UserAuthTokenFingerprint(tokenHash)
		if row.LegacyToken.Valid && legacyToken == "" && row.Fingerprint.Valid && row.Fingerprint.String == fingerprint {
			return nil
		}
		updates["fingerprint"] = fingerprint
	case tokenHash != "" && models.IsValidUserAuthTokenHash(strings.ToLower(tokenHash)):
		// Older or partially applied code may have persisted an otherwise
		// valid digest with uppercase hex. Normalize it so the active row is
		// both canonical and verifiable instead of preserving a dead token.
		tokenHash = strings.ToLower(tokenHash)
		updates["token_hash"] = tokenHash
		updates["fingerprint"] = models.UserAuthTokenFingerprint(tokenHash)
	default:
		if row.LegacyToken.Valid && legacyToken == "" && row.TokenHash.Valid && tokenHash == "" &&
			row.Fingerprint.Valid && row.Fingerprint.String == "" && row.Revoked {
			return nil
		}
		updates["token_hash"] = ""
		updates["fingerprint"] = ""
		updates["revoked"] = true
	}

	result := tx.Table((&models.UserAuthToken{}).TableName()).
		Where("id = ?", row.ID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("user auth token hash migration: update token %s: %w", row.ID, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("user auth token hash migration: update token %s affected %d rows", row.ID, result.RowsAffected)
	}
	return nil
}

func assertUserAuthTokenPlaintextCleared(tx *gorm.DB) error {
	var plaintextCount int64
	if err := tx.Table((&models.UserAuthToken{}).TableName()).
		Where("COALESCE(token, '') <> ''").
		Count(&plaintextCount).Error; err != nil {
		return fmt.Errorf("user auth token hash migration: verify plaintext cleanup: %w", err)
	}
	if plaintextCount != 0 {
		return fmt.Errorf("user auth token hash migration: %d plaintext rows remain", plaintextCount)
	}

	var invalidActiveCount int64
	if err := tx.Table((&models.UserAuthToken{}).TableName()).
		Where("revoked = ?", false).
		Where("token_hash IS NULL OR token_hash = ''").
		Count(&invalidActiveCount).Error; err != nil {
		return fmt.Errorf("user auth token hash migration: verify active digests: %w", err)
	}
	if invalidActiveCount != 0 {
		return fmt.Errorf("user auth token hash migration: %d active rows have no digest", invalidActiveCount)
	}
	return nil
}
