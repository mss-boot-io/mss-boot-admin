package system

import (
	"path/filepath"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const oauthLocalPasswordTestVersion = "2026080617100"

func TestOAuthLocalPasswordDisableMigrationBackfillsAndIsIdempotent(t *testing.T) {
	db := openOAuthLocalPasswordMigrationTestDB(t)
	if err := db.Exec(`CREATE TABLE mss_boot_users (
		id TEXT PRIMARY KEY,
		username TEXT,
		password_hash TEXT,
		salt TEXT
	)`).Error; err != nil {
		t.Fatalf("create legacy user table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE mss_boot_user_oauth2 (
		id TEXT PRIMARY KEY,
		user_id TEXT,
		provider TEXT,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create legacy OAuth table: %v", err)
	}
	if err := db.Exec(
		"INSERT INTO mss_boot_users (id, username, password_hash, salt) VALUES (?, ?, ?, ?), (?, ?, ?, ?), (?, ?, ?, ?)",
		"oauth-user", "oauth", "legacy-provider-derived-hash", "salt-1",
		"local-user", "local", "local-password-hash", "salt-2",
		"previously-linked", "previous", "previous-password-hash", "salt-3",
	).Error; err != nil {
		t.Fatalf("insert legacy users: %v", err)
	}
	if err := db.Exec(
		"INSERT INTO mss_boot_user_oauth2 (id, user_id, provider, deleted_at) VALUES (?, ?, ?, ?), (?, ?, ?, ?)",
		"binding-1", "oauth-user", "github",
		nil,
		"binding-2", "previously-linked", "lark", "2026-08-01 00:00:00",
	).Error; err != nil {
		t.Fatalf("insert legacy OAuth binding: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if err := migrateOAuthLocalPasswordDisable(db, oauthLocalPasswordTestVersion); err != nil {
			t.Fatalf("migration attempt %d: %v", attempt, err)
		}
	}
	if !db.Migrator().HasColumn(&models.User{}, "LocalPasswordDisabled") {
		t.Fatal("migration did not add local password flag")
	}

	var rows []struct {
		ID                    string
		LocalPasswordDisabled bool
	}
	if err := db.Table((&models.User{}).TableName()).
		Select("id", "local_password_disabled").
		Order("id").
		Scan(&rows).Error; err != nil {
		t.Fatalf("load migrated users: %v", err)
	}
	if len(rows) != 3 || rows[0].ID != "local-user" || rows[0].LocalPasswordDisabled ||
		rows[1].ID != "oauth-user" || !rows[1].LocalPasswordDisabled ||
		rows[2].ID != "previously-linked" || !rows[2].LocalPasswordDisabled {
		t.Fatalf("migrated local-password flags = %#v", rows)
	}

	var versionCount int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", oauthLocalPasswordTestVersion).
		Count(&versionCount).Error; err != nil {
		t.Fatalf("count migration versions: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("migration version count = %d, want 1", versionCount)
	}
}

func TestOAuthLocalPasswordDisableMigrationFreshSchema(t *testing.T) {
	db := openOAuthLocalPasswordMigrationTestDB(t)
	if err := migrateOAuthLocalPasswordDisable(db, oauthLocalPasswordTestVersion); err != nil {
		t.Fatalf("fresh migration: %v", err)
	}
	if !db.Migrator().HasTable(&models.User{}) ||
		!db.Migrator().HasColumn(&models.User{}, "LocalPasswordDisabled") {
		t.Fatal("fresh migration did not create the hardened user schema")
	}
}

func openOAuthLocalPasswordMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "oauth-local-password.db")
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
