package system

import (
	"runtime"

	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260403225953EnhanceOptions)
}

func _20260403225953EnhanceOptions(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// Add new columns to mss_boot_options table
		err := tx.Exec(`
			ALTER TABLE mss_boot_options 
			ADD COLUMN IF NOT EXISTS category VARCHAR(50) NOT NULL DEFAULT 'system',
			ADD COLUMN IF NOT EXISTS display_name VARCHAR(255),
			ADD COLUMN IF NOT EXISTS description TEXT,
			ADD COLUMN IF NOT EXISTS version INT DEFAULT 1,
			ADD COLUMN IF NOT EXISTS built_in BOOLEAN DEFAULT FALSE
		`).Error
		if err != nil {
			return err
		}

		// Add index on category
		err = tx.Exec(`
			CREATE INDEX IF NOT EXISTS idx_category ON mss_boot_options(category)
		`).Error
		if err != nil {
			return err
		}

		// Create option_versions table
		err = tx.Exec(`
			CREATE TABLE IF NOT EXISTS mss_boot_option_versions (
				id VARCHAR(36) PRIMARY KEY,
				tenant_id VARCHAR(36),
				creator_id VARCHAR(64),
				created_at TIMESTAMP,
				updated_at TIMESTAMP,
				option_id VARCHAR(36) NOT NULL,
				version INT NOT NULL,
				items JSON,
				changed_by VARCHAR(36),
				change_note TEXT,
				status VARCHAR(10),
				deleted_at TIMESTAMP
			)
		`).Error
		if err != nil {
			return err
		}

		// Add index on option_id for option_versions
		err = tx.Exec(`
			CREATE INDEX IF NOT EXISTS idx_option_id ON mss_boot_option_versions(option_id)
		`).Error
		if err != nil {
			return err
		}

		// Create option_usages table
		err = tx.Exec(`
			CREATE TABLE IF NOT EXISTS mss_boot_option_usages (
				id VARCHAR(36) PRIMARY KEY,
				tenant_id VARCHAR(36),
				creator_id VARCHAR(64),
				created_at TIMESTAMP,
				updated_at TIMESTAMP,
				option_id VARCHAR(36) NOT NULL,
				used_by VARCHAR(255),
				used_at TEXT,
				use_count INT DEFAULT 0,
				status VARCHAR(10),
				deleted_at TIMESTAMP
			)
		`).Error
		if err != nil {
			return err
		}

		// Add index on option_id for option_usages
		err = tx.Exec(`
			CREATE INDEX IF NOT EXISTS idx_option_id ON mss_boot_option_usages(option_id)
		`).Error
		if err != nil {
			return err
		}

		return migration.Migrate.CreateVersion(tx, version)
	})
}