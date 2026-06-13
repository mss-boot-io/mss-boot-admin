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

var enhanceOptionsColumns = []struct {
	name string
	def  string
}{
	{"category", "VARCHAR(50) NOT NULL DEFAULT 'system'"},
	{"display_name", "VARCHAR(255)"},
	{"description", "TEXT"},
	{"version", "INT DEFAULT 1"},
	{"built_in", "BOOLEAN DEFAULT FALSE"},
}

func _20260403225953EnhanceOptions(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		dialect := tx.Dialector.Name()

		for _, c := range enhanceOptionsColumns {
			exists, err := optionsColumnExists(tx, dialect, "mss_boot_options", c.name)
			if err != nil {
				return err
			}
			if exists {
				continue
			}
			if err := tx.Exec("ALTER TABLE mss_boot_options ADD COLUMN " + c.name + " " + c.def).Error; err != nil {
				return err
			}
		}

		if err := ensureOptionsIndex(tx, dialect, "mss_boot_options", "idx_category",
			"CREATE INDEX idx_category ON mss_boot_options(category)"); err != nil {
			return err
		}

		if err := tx.Exec(`
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
		`).Error; err != nil {
			return err
		}
		if err := ensureOptionsIndex(tx, dialect, "mss_boot_option_versions", "idx_option_versions_option_id",
			"CREATE INDEX idx_option_versions_option_id ON mss_boot_option_versions(option_id)"); err != nil {
			return err
		}

		if err := tx.Exec(`
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
		`).Error; err != nil {
			return err
		}
		if err := ensureOptionsIndex(tx, dialect, "mss_boot_option_usages", "idx_option_usages_option_id",
			"CREATE INDEX idx_option_usages_option_id ON mss_boot_option_usages(option_id)"); err != nil {
			return err
		}

		return migration.Migrate.CreateVersion(tx, version)
	})
}

func optionsColumnExists(tx *gorm.DB, dialect, table, column string) (bool, error) {
	var count int64
	switch dialect {
	case "sqlite":
		// pragma_table_info requires a literal table name; callers pass
		// hard-coded internal names so concatenation is safe here.
		if err := tx.Raw(
			"SELECT COUNT(*) FROM pragma_table_info('"+table+"') WHERE name = ?",
			column,
		).Scan(&count).Error; err != nil {
			return false, err
		}
	case "postgres":
		if err := tx.Raw(
			`SELECT COUNT(*) FROM information_schema.columns
			 WHERE table_schema = CURRENT_SCHEMA() AND table_name = ? AND column_name = ?`,
			table, column,
		).Scan(&count).Error; err != nil {
			return false, err
		}
	default: // mysql and mysql-compatible
		if err := tx.Raw(
			`SELECT COUNT(*) FROM information_schema.columns
			 WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			table, column,
		).Scan(&count).Error; err != nil {
			return false, err
		}
	}
	return count > 0, nil
}

// ensureOptionsIndex creates an index when missing. createSQL must be a bare
// CREATE INDEX (no IF NOT EXISTS) because MySQL does not support that clause
// on CREATE INDEX.
func ensureOptionsIndex(tx *gorm.DB, dialect, table, indexName, createSQL string) error {
	var count int64
	switch dialect {
	case "sqlite":
		if err := tx.Raw(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?",
			indexName,
		).Scan(&count).Error; err != nil {
			return err
		}
	case "postgres":
		if err := tx.Raw(
			"SELECT COUNT(*) FROM pg_indexes WHERE schemaname = CURRENT_SCHEMA() AND indexname = ?",
			indexName,
		).Scan(&count).Error; err != nil {
			return err
		}
	default: // mysql and mysql-compatible
		if err := tx.Raw(
			`SELECT COUNT(*) FROM information_schema.statistics
			 WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`,
			table, indexName,
		).Scan(&count).Error; err != nil {
			return err
		}
	}
	if count > 0 {
		return nil
	}
	return tx.Exec(createSQL).Error
}
