package system

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const presentationProfilesTestVersion = "20260824120000-test"

func TestPresentationProfileMigrationFreshRepeatedAndUpgradeSafe(t *testing.T) {
	db := openPresentationMigrationDB(t)
	if err := db.Exec(`CREATE TABLE legacy_business_records (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create legacy business table: %v", err)
	}
	updatedAt := time.Now().UTC().Truncate(time.Second)
	if err := db.Exec(
		"INSERT INTO legacy_business_records (id, name, updated_at) VALUES (?, ?, ?)",
		"legacy-1", "must-survive", updatedAt,
	).Error; err != nil {
		t.Fatalf("insert legacy business row: %v", err)
	}
	legacySQL := readPresentationSQLiteCreateSQL(t, db, "legacy_business_records")

	if err := migratePresentationProfiles(db, presentationProfilesTestVersion); err != nil {
		t.Fatalf("run fresh presentation migration: %v", err)
	}
	assertPresentationProfileSchema(t, db)
	assertPresentationMigrationVersion(t, db, presentationProfilesTestVersion, 1)

	if err := db.Exec(
		"INSERT INTO mss_boot_presentation_profiles "+
			"(id, scope, subject_id, page_key, version, draft_document, draft_digest, draft_definition_hash, published_revision, created_by, updated_by, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"profile-1", "application", "", "orders.list", 1, `{}`, "sha256:draft", "sha256:definition", 0,
		"author", "author", updatedAt, updatedAt,
	).Error; err != nil {
		t.Fatalf("insert presentation aggregate: %v", err)
	}
	if err := db.Exec(
		"INSERT INTO mss_boot_presentation_revisions "+
			"(id, profile_id, revision, aggregate_version, document, content_digest, definition_hash, transition, actor_id, idempotency_key_hash, request_hash, created_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"revision-1", "profile-1", 1, 2, `{}`, "sha256:content", "sha256:definition", "publish",
		"publisher", "sha256:key", "sha256:request", updatedAt,
	).Error; err != nil {
		t.Fatalf("insert presentation revision: %v", err)
	}

	if err := migratePresentationProfiles(db, presentationProfilesTestVersion); err != nil {
		t.Fatalf("repeat presentation migration: %v", err)
	}
	assertPresentationProfileSchema(t, db)
	assertPresentationMigrationVersion(t, db, presentationProfilesTestVersion, 1)

	var legacyName string
	if err := db.Raw("SELECT name FROM legacy_business_records WHERE id = ?", "legacy-1").Scan(&legacyName).Error; err != nil {
		t.Fatalf("read legacy row: %v", err)
	}
	if legacyName != "must-survive" {
		t.Fatalf("legacy row changed to %q", legacyName)
	}
	if got := readPresentationSQLiteCreateSQL(t, db, "legacy_business_records"); got != legacySQL {
		t.Fatalf("legacy schema changed:\n got: %s\nwant: %s", got, legacySQL)
	}
	var aggregateCount, revisionCount int64
	if err := db.Model(&models.PresentationProfile{}).Count(&aggregateCount).Error; err != nil {
		t.Fatalf("count aggregates: %v", err)
	}
	if err := db.Model(&models.PresentationRevision{}).Count(&revisionCount).Error; err != nil {
		t.Fatalf("count revisions: %v", err)
	}
	if aggregateCount != 1 || revisionCount != 1 {
		t.Fatalf("migration rerun changed rows: aggregates=%d revisions=%d", aggregateCount, revisionCount)
	}
}

func TestPresentationProfileMigrationEnforcesIdentityRevisionAndVersionConstraints(t *testing.T) {
	db := openPresentationMigrationDB(t)
	if err := migratePresentationProfiles(db, presentationProfilesTestVersion); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	insertProfile := func(id, scope, subject, page string, version int64) error {
		return db.Exec(
			"INSERT INTO mss_boot_presentation_profiles "+
				"(id, scope, subject_id, page_key, version, draft_document, draft_digest, draft_definition_hash, published_revision, created_by, updated_by, created_at, updated_at) "+
				"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			id, scope, subject, page, version, `{}`, "sha256:draft", "sha256:definition", 0,
			"author", "author", now, now,
		).Error
	}
	if err := insertProfile("profile-1", "role", "role-1", "orders.list", 1); err != nil {
		t.Fatalf("insert first profile: %v", err)
	}
	if err := insertProfile("profile-2", "role", "role-1", "orders.list", 1); err == nil {
		t.Fatal("duplicate scoped identity was accepted")
	}
	if err := insertProfile("profile-invalid", "application", "", "invalid.list", 0); err == nil {
		t.Fatal("non-positive aggregate version was accepted")
	}

	insertRevision := func(id string, revision int64, idempotency string) error {
		return db.Exec(
			"INSERT INTO mss_boot_presentation_revisions "+
				"(id, profile_id, revision, aggregate_version, document, content_digest, definition_hash, transition, actor_id, idempotency_key_hash, request_hash, created_at) "+
				"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			id, "profile-1", revision, 2, `{}`, "sha256:content", "sha256:definition", "publish",
			"publisher", idempotency, "sha256:request", now,
		).Error
	}
	if err := insertRevision("revision-1", 1, "sha256:key-1"); err != nil {
		t.Fatalf("insert first revision: %v", err)
	}
	if err := insertRevision("revision-number-duplicate", 1, "sha256:key-2"); err == nil {
		t.Fatal("duplicate profile-local revision was accepted")
	}
	if err := insertRevision("revision-key-duplicate", 2, "sha256:key-1"); err == nil {
		t.Fatal("duplicate profile-local idempotency digest was accepted")
	}
	if err := insertRevision("revision-invalid", 0, "sha256:key-3"); err == nil {
		t.Fatal("non-positive revision was accepted")
	}
}

type presentationSQLiteColumn struct {
	Name         string         `gorm:"column:name"`
	NotNull      int            `gorm:"column:notnull"`
	DefaultValue sql.NullString `gorm:"column:dflt_value"`
}

func assertPresentationProfileSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, model := range []any{&models.PresentationProfile{}, &models.PresentationRevision{}} {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("presentation table for %T was not created", model)
		}
	}
	for _, check := range []struct {
		model any
		index string
	}{
		{&models.PresentationProfile{}, "ux_presentation_profile_identity"},
		{&models.PresentationRevision{}, "ux_presentation_revision_number"},
		{&models.PresentationRevision{}, "ux_presentation_revision_idempotency"},
	} {
		if !db.Migrator().HasIndex(check.model, check.index) {
			t.Fatalf("presentation index %q was not created", check.index)
		}
	}
	profileSQL := readPresentationSQLiteCreateSQL(t, db, "mss_boot_presentation_profiles")
	revisionSQL := readPresentationSQLiteCreateSQL(t, db, "mss_boot_presentation_revisions")
	for _, fragment := range []string{"version > 0", "published_revision >= 0"} {
		if !strings.Contains(profileSQL, fragment) {
			t.Fatalf("aggregate schema missing constraint %q: %s", fragment, profileSQL)
		}
	}
	for _, fragment := range []string{"revision > 0", "aggregate_version > 0"} {
		if !strings.Contains(revisionSQL, fragment) {
			t.Fatalf("revision schema missing constraint %q: %s", fragment, revisionSQL)
		}
	}
}

func openPresentationMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "presentation-migration.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open presentation migration database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get presentation migration handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func readPresentationSQLiteCreateSQL(t *testing.T, db *gorm.DB, table string) string {
	t.Helper()
	var result string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&result).Error; err != nil {
		t.Fatalf("read SQLite schema for %s: %v", table, err)
	}
	return result
}

func assertPresentationMigrationVersion(t *testing.T, db *gorm.DB, version string, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).Where("version = ?", version).Count(&count).Error; err != nil {
		t.Fatalf("count presentation migration version: %v", err)
	}
	if count != want {
		t.Fatalf("presentation migration version count = %d, want %d", count, want)
	}
}
