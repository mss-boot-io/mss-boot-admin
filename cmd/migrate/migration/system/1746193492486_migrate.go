package system

import (
	"fmt"
	"runtime"

	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _1746193492486Migrate)
}

func _1746193492486Migrate(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		q := quoteCol(tx)

		err := tx.Exec(fmt.Sprintf(
			`CREATE UNIQUE INDEX idx_user_config ON mss_boot_user_configs(user_id, name, %s)`, q("group"))).Error
		if err != nil {
			return err
		}

		err = tx.Exec(fmt.Sprintf(
			`CREATE UNIQUE INDEX idx_app_config ON mss_boot_app_configs(name, %s)`, q("group"))).Error
		if err != nil {
			return err
		}

		return migration.Migrate.CreateVersion(tx, version)
	})
}

func quoteCol(db *gorm.DB) func(string) string {
	return func(col string) string {
		switch db.Dialector.Name() {
		case "mysql":
			return "`" + col + "`"
		default:
			return `"` + col + `"`
		}
	}
}
