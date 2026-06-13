package system

import (
	"runtime"

	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/models"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260614120000RepairUserSessions)
}

// _20260614120000RepairUserSessions backfills mss_boot_user_sessions on
// installations that hit the GetFilename() 13-char-prefix collision before
// the rename in this PR.
//
// On affected installs, 20260607162058_session_menus.go registered the same
// version key (2026060716205) as 20260607162057_user_sessions.go. The later
// init() overwrote the earlier handler, so session_menus ran and recorded
// 2026060716205 as applied, while the user_sessions AutoMigrate was silently
// skipped. After renaming session_menus to 20260607162060, user_sessions
// still maps to 2026060716205 and the migrator sees that row as Done, so the
// table stays missing on those installs.
//
// Running tx.Migrator().AutoMigrate(new(models.UserSession)) is idempotent:
// it creates the table only if it does not exist. Fresh DBs and healthy
// upgrades pay one no-op statement.
func _20260614120000RepairUserSessions(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Migrator().AutoMigrate(new(models.UserSession)); err != nil {
			return err
		}
		return migration.Migrate.CreateVersion(tx, version)
	})
}
