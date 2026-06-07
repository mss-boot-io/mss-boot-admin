package system

import (
	"runtime"

	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260607000002OnlineSessionsNote)
}

// _20260607000002OnlineSessionsNote marks the online-sessions feature as rolled out.
// Legacy JWTs without a sid claim are rejected by the auth middleware once the
// SessionEnabled flag is on. This migration is a trail record only and does not
// touch any data.
func _20260607000002OnlineSessionsNote(db *gorm.DB, version string) error {
	return migration.Migrate.CreateVersion(db, version)
}
