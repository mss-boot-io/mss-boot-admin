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
		switch tx.Dialector.Name() {
		case "sqlite":
			if err := migrateSQLiteOptions(tx); err != nil {
				return err
			}
		default:
			if err := migrateStandardOptions(tx); err != nil {
				return err
			}
		}
		return migration.Migrate.CreateVersion(tx, version)
	})
}

func migrateStandardOptions(tx *gorm.DB) error {
	cols := []struct {
		name string
		def  string
	}{
		{"category", "VARCHAR(50) NOT NULL DEFAULT 'system'"},
		{"display_name", "VARCHAR(255)"},
		{"description", "TEXT"},
		{"version", "INT DEFAULT 1"},
		{"built_in", "BOOLEAN DEFAULT FALSE"},
	}
	for _, c := range cols {
		var count int64
		schemaExpr := "table_schema = DATABASE()"
		if tx.Dialector.Name() != "mysql" {
			schemaExpr = "table_schema = current_schema()"
		}
		if err := tx.Raw(
			"SELECT COUNT(*) FROM information_schema.columns WHERE "+schemaExpr+" AND table_name = 'mss_boot_options' AND column_name = ?",
			c.name,
		).Scan(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err := tx.Exec("ALTER TABLE mss_boot_options ADD COLUMN " + c.name + " " + c.def).Error; err != nil {
				return err
			}
		}
	}
	tx.Exec(`CREATE INDEX IF NOT EXISTS idx_category ON mss_boot_options(category)`)

	err := tx.Exec(`
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
	tx.Exec(`CREATE INDEX IF NOT EXISTS idx_option_versions_option_id ON mss_boot_option_versions(option_id)`)

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
	tx.Exec(`CREATE INDEX IF NOT EXISTS idx_option_usages_option_id ON mss_boot_option_usages(option_id)`)
	return nil
}

func migrateSQLiteOptions(tx *gorm.DB) error {
	var count int64
	for _, col := range []string{"category", "display_name", "description", "version", "built_in"} {
		tx.Raw("SELECT COUNT(*) FROM pragma_table_info('mss_boot_options') WHERE name=?", col).Scan(&count)
		if count == 0 {
			var def string
			switch col {
			case "category":
				def = "ALTER TABLE mss_boot_options ADD COLUMN category VARCHAR(50) NOT NULL DEFAULT 'system'"
			case "display_name":
				def = "ALTER TABLE mss_boot_options ADD COLUMN display_name VARCHAR(255)"
			case "description":
				def = "ALTER TABLE mss_boot_options ADD COLUMN description TEXT"
			case "version":
				def = "ALTER TABLE mss_boot_options ADD COLUMN version INT DEFAULT 1"
			case "built_in":
				def = "ALTER TABLE mss_boot_options ADD COLUMN built_in BOOLEAN DEFAULT FALSE"
			}
			if err := tx.Exec(def).Error; err != nil {
				return err
			}
		}
	}
	tx.Exec(`CREATE INDEX IF NOT EXISTS idx_category ON mss_boot_options(category)`)

	err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS mss_boot_option_versions (
			id VARCHAR(36) PRIMARY KEY, tenant_id VARCHAR(36), creator_id VARCHAR(64),
			created_at TIMESTAMP, updated_at TIMESTAMP, option_id VARCHAR(36) NOT NULL,
			version INT NOT NULL, items JSON, changed_by VARCHAR(36), change_note TEXT,
			status VARCHAR(10), deleted_at TIMESTAMP
		)
	`).Error
	if err != nil {
		return err
	}
	tx.Exec(`CREATE INDEX IF NOT EXISTS idx_option_versions_option_id ON mss_boot_option_versions(option_id)`)

	err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS mss_boot_option_usages (
			id VARCHAR(36) PRIMARY KEY, tenant_id VARCHAR(36), creator_id VARCHAR(64),
			created_at TIMESTAMP, updated_at TIMESTAMP, option_id VARCHAR(36) NOT NULL,
			used_by VARCHAR(255), used_at TEXT, use_count INT DEFAULT 0,
			status VARCHAR(10), deleted_at TIMESTAMP
		)
	`).Error
	if err != nil {
		return err
	}
	tx.Exec(`CREATE INDEX IF NOT EXISTS idx_option_usages_option_id ON mss_boot_option_usages(option_id)`)
	return nil
}