package system

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const userAuthTokenHashTestVersion = "2026080617000"

const userAuthTokenHashPartialTestVersion = "2026080617001"

func TestUserAuthTokenHashMigrationFreshAndIdempotent(t *testing.T) {
	db := openUserAuthTokenMigrationTestDB(t)
	if err := migrateUserAuthTokenHashes(db, userAuthTokenHashTestVersion); err != nil {
		t.Fatalf("fresh migration: %v", err)
	}
	if !db.Migrator().HasTable(&models.UserAuthToken{}) {
		t.Fatal("fresh migration did not create PAT table")
	}
	for _, column := range []string{"token", "token_hash", "fingerprint"} {
		if !db.Migrator().HasColumn(&models.UserAuthToken{}, column) {
			t.Fatalf("fresh migration missing column %q", column)
		}
	}
	assertUserAuthTokenMigrationVersionCount(t, db, 1)
	assertNoUserAuthTokenPlaintext(t, db)

	if err := migrateUserAuthTokenHashes(db, userAuthTokenHashTestVersion); err != nil {
		t.Fatalf("second fresh migration: %v", err)
	}
	assertUserAuthTokenMigrationVersionCount(t, db, 1)
}

func TestUserAuthTokenHashMigrationBackfillsLegacyRowsAndFailsEmptyClosed(t *testing.T) {
	db := openUserAuthTokenMigrationTestDB(t)
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatalf("migrate version table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE mss_boot_user_auth_token (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		token TEXT,
		expired_at DATETIME,
		revoked BOOLEAN NOT NULL DEFAULT FALSE,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create legacy PAT table: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	fixtures := []struct {
		ID        string
		RawToken  string
		ExpiredAt time.Time
		Revoked   bool
	}{
		{ID: "active", RawToken: "legacy-active-token", ExpiredAt: now.Add(24 * time.Hour)},
		{ID: "revoked", RawToken: "legacy-revoked-token", ExpiredAt: now.Add(24 * time.Hour), Revoked: true},
		{ID: "expired", RawToken: "legacy-expired-token", ExpiredAt: now.Add(-time.Hour)},
		{ID: "empty", RawToken: "", ExpiredAt: now.Add(24 * time.Hour)},
	}
	for _, fixture := range fixtures {
		if err := db.Exec(
			"INSERT INTO mss_boot_user_auth_token (id, user_id, token, expired_at, revoked, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			fixture.ID,
			"user-1",
			fixture.RawToken,
			fixture.ExpiredAt,
			fixture.Revoked,
			now,
			now,
		).Error; err != nil {
			t.Fatalf("insert %s fixture: %v", fixture.ID, err)
		}
	}

	if err := migrateUserAuthTokenHashes(db, userAuthTokenHashTestVersion); err != nil {
		t.Fatalf("upgrade migration: %v", err)
	}
	assertNoUserAuthTokenPlaintext(t, db)
	assertUserAuthTokenMigrationVersionCount(t, db, 1)

	for _, fixture := range fixtures {
		row := &models.UserAuthToken{}
		if err := db.First(row, "id = ?", fixture.ID).Error; err != nil {
			t.Fatalf("load migrated %s: %v", fixture.ID, err)
		}
		if fixture.RawToken == "" {
			if !row.Revoked || row.TokenHash != "" || row.Fingerprint != "" {
				t.Fatalf("empty row did not fail closed: %+v", row)
			}
			continue
		}
		if row.TokenHash != models.HashUserAuthToken(fixture.RawToken) {
			t.Fatalf("%s digest was not backfilled", fixture.ID)
		}
		if row.Fingerprint != models.UserAuthTokenFingerprint(row.TokenHash) {
			t.Fatalf("%s fingerprint mismatch", fixture.ID)
		}
		if row.Revoked != fixture.Revoked {
			t.Fatalf("%s revoked = %v, want %v", fixture.ID, row.Revoked, fixture.Revoked)
		}
		if !row.ExpiredAt.Equal(fixture.ExpiredAt) {
			t.Fatalf("%s expiry changed: got %s want %s", fixture.ID, row.ExpiredAt, fixture.ExpiredAt)
		}
	}

	// Simulate a restart after DDL/data succeeded but before the version row was
	// durably observed. Already-hardened rows must be safe to process again.
	if err := db.Where("version = ?", userAuthTokenHashTestVersion).
		Delete(&migrationmodels.Migration{}).Error; err != nil {
		t.Fatalf("remove migration marker: %v", err)
	}
	if err := migrateUserAuthTokenHashes(db, userAuthTokenHashTestVersion); err != nil {
		t.Fatalf("restart upgrade migration: %v", err)
	}
	assertNoUserAuthTokenPlaintext(t, db)
	assertUserAuthTokenMigrationVersionCount(t, db, 1)
}

func TestUserAuthTokenHashMigrationPartialDDLRecomputesBeforeClearing(t *testing.T) {
	db := openUserAuthTokenMigrationTestDB(t)
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatalf("migrate version table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE mss_boot_user_auth_token (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		token TEXT,
		token_hash TEXT,
		fingerprint TEXT,
		expired_at DATETIME,
		revoked BOOLEAN NOT NULL DEFAULT FALSE,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create partially expanded PAT table: %v", err)
	}

	rawToken := "partial-ddl-legacy-token"
	wrongHash := models.HashUserAuthToken("different-token")
	if err := db.Exec(
		"INSERT INTO mss_boot_user_auth_token (id, user_id, token, token_hash, fingerprint, expired_at, revoked) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"partial",
		"user-1",
		rawToken,
		wrongHash,
		models.UserAuthTokenFingerprint(wrongHash),
		time.Now().Add(time.Hour),
		false,
	).Error; err != nil {
		t.Fatalf("insert partial DDL fixture: %v", err)
	}
	uppercaseRaw := "partial-uppercase-digest-token"
	canonicalUppercaseHash := models.HashUserAuthToken(uppercaseRaw)
	uppercaseHash := "sha256:" + strings.ToUpper(strings.TrimPrefix(canonicalUppercaseHash, "sha256:"))
	if err := db.Exec(
		"INSERT INTO mss_boot_user_auth_token (id, user_id, token, token_hash, fingerprint, expired_at, revoked) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"uppercase",
		"user-1",
		"",
		uppercaseHash,
		strings.ToUpper(models.UserAuthTokenFingerprint(canonicalUppercaseHash)),
		time.Now().Add(time.Hour),
		false,
	).Error; err != nil {
		t.Fatalf("insert uppercase digest fixture: %v", err)
	}

	if err := migrateUserAuthTokenHashes(db, userAuthTokenHashPartialTestVersion); err != nil {
		t.Fatalf("partial DDL migration: %v", err)
	}
	row := &models.UserAuthToken{}
	if err := db.First(row, "id = ?", "partial").Error; err != nil {
		t.Fatalf("load partially migrated PAT: %v", err)
	}
	if row.LegacyToken != "" {
		t.Fatal("partial DDL migration retained plaintext")
	}
	if row.TokenHash != models.HashUserAuthToken(rawToken) {
		t.Fatal("partial DDL migration trusted a stale digest instead of plaintext source")
	}
	if row.Revoked {
		t.Fatal("recoverable partial DDL token was revoked")
	}

	uppercaseRow := &models.UserAuthToken{}
	if err := db.First(uppercaseRow, "id = ?", "uppercase").Error; err != nil {
		t.Fatalf("load normalized uppercase digest PAT: %v", err)
	}
	if uppercaseRow.TokenHash != canonicalUppercaseHash {
		t.Fatalf("uppercase digest = %q, want canonical %q", uppercaseRow.TokenHash, canonicalUppercaseHash)
	}
	if uppercaseRow.Fingerprint != models.UserAuthTokenFingerprint(canonicalUppercaseHash) {
		t.Fatalf("uppercase digest fingerprint = %q, want canonical fingerprint", uppercaseRow.Fingerprint)
	}
	if uppercaseRow.Revoked {
		t.Fatal("canonicalizable uppercase digest was revoked")
	}
	if !models.VerifyUserAuthToken(uppercaseRaw, uppercaseRow.TokenHash) {
		t.Fatal("normalized uppercase digest cannot authenticate its source token")
	}
}

func openUserAuthTokenMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "pat-migration.db")
	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open migration sqlite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get migration sqlite database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func assertNoUserAuthTokenPlaintext(t *testing.T, db *gorm.DB) {
	t.Helper()
	var count int64
	if err := db.Table((&models.UserAuthToken{}).TableName()).
		Where("COALESCE(token, '') <> ''").
		Count(&count).Error; err != nil {
		t.Fatalf("count plaintext tokens: %v", err)
	}
	if count != 0 {
		t.Fatalf("plaintext token count = %d, want 0", count)
	}
}

func assertUserAuthTokenMigrationVersionCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", userAuthTokenHashTestVersion).
		Count(&count).Error; err != nil {
		t.Fatalf("count PAT migration version: %v", err)
	}
	if count != want {
		t.Fatalf("PAT migration version count = %d, want %d", count, want)
	}
}
