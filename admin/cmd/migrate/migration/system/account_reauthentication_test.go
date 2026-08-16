package system

import (
	"path/filepath"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const accountReauthenticationTestVersion = "20260816163000-test"

func TestAccountReauthenticationMigrationAddsFailClosedSessionStateAndIsIdempotent(t *testing.T) {
	db := openAccountReauthenticationTestDB(t)
	if err := db.Exec(`CREATE TABLE mss_boot_user_sessions (
		id TEXT PRIMARY KEY,
		user_id TEXT,
		login_at DATETIME,
		last_seen_at DATETIME,
		expired_at DATETIME,
		revoked NUMERIC DEFAULT false
	)`).Error; err != nil {
		t.Fatalf("create legacy session table: %v", err)
	}
	if err := db.Exec(
		"INSERT INTO mss_boot_user_sessions (id, user_id, login_at, last_seen_at, expired_at, revoked) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, DATETIME('now', '+1 hour'), ?)",
		"legacy-session", "user-1", false,
	).Error; err != nil {
		t.Fatalf("insert legacy session: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if err := migrateAccountReauthentication(db, accountReauthenticationTestVersion); err != nil {
			t.Fatalf("migration attempt %d: %v", attempt, err)
		}
	}
	for _, field := range []string{"ReauthenticatedAt", "ReauthFailedAttempts", "ReauthFailedAt", "ReauthLockedUntil"} {
		if !db.Migrator().HasColumn(&models.UserSession{}, field) {
			t.Fatalf("migration did not add %s", field)
		}
	}
	var row struct {
		ReauthenticatedAt    *string
		ReauthFailedAttempts int
		ReauthLockedUntil    *string
	}
	if err := db.Table((&models.UserSession{}).TableName()).
		Select("reauthenticated_at", "reauth_failed_attempts", "reauth_locked_until").
		Where("id = ?", "legacy-session").
		Scan(&row).Error; err != nil {
		t.Fatalf("load migrated session: %v", err)
	}
	if row.ReauthenticatedAt != nil || row.ReauthFailedAttempts != 0 || row.ReauthLockedUntil != nil {
		t.Fatalf("legacy session received unexpected proof state: %#v", row)
	}
	var versions int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", accountReauthenticationTestVersion).
		Count(&versions).Error; err != nil {
		t.Fatalf("count migration versions: %v", err)
	}
	if versions != 1 {
		t.Fatalf("migration version count = %d, want 1", versions)
	}
}

func TestAccountReauthenticationMigrationCreatesFreshSessionSchema(t *testing.T) {
	db := openAccountReauthenticationTestDB(t)
	if err := migrateAccountReauthentication(db, accountReauthenticationTestVersion); err != nil {
		t.Fatalf("fresh migration: %v", err)
	}
	if !db.Migrator().HasTable(&models.UserSession{}) ||
		!db.Migrator().HasColumn(&models.UserSession{}, "ReauthenticatedAt") {
		t.Fatal("fresh migration did not create the account reauthentication schema")
	}
}

func openAccountReauthenticationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "account-reauthentication.db")
	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get migration database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
