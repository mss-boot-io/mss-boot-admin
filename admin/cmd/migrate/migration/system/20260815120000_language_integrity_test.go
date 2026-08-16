package system

import (
	"fmt"
	"testing"

	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openLanguageIntegrityMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQLite handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatalf("migrate version table: %v", err)
	}
	return db
}

func createLegacyLanguageTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE mss_boot_languages (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		status TEXT NULL
	)`).Error; err != nil {
		t.Fatalf("create legacy language table: %v", err)
	}
}

func TestLanguageIntegrityMigrationBackfillsLegacyValuesAndIsIdempotent(t *testing.T) {
	db := openLanguageIntegrityMigrationDB(t)
	createLegacyLanguageTable(t, db)
	if err := db.Exec(
		"INSERT INTO mss_boot_languages (id, name, status) VALUES (?, ?, ?), (?, ?, ?)",
		"", "en-US", "", "existing-id", "zh-CN", "disabled",
	).Error; err != nil {
		t.Fatalf("seed legacy languages: %v", err)
	}

	const version = "20260815120000-test"
	for attempt := 1; attempt <= 2; attempt++ {
		if err := migrateLanguageIntegrity(db, version); err != nil {
			t.Fatalf("migration attempt %d: %v", attempt, err)
		}
	}

	type languageRow struct {
		ID     string
		Name   string
		Status string
	}
	rows := make([]languageRow, 0, 2)
	if err := db.Table("mss_boot_languages").Order("name ASC").Find(&rows).Error; err != nil {
		t.Fatalf("load migrated languages: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("language row count = %d, want 2", len(rows))
	}
	if rows[0].Name != "en-US" || len(rows[0].ID) != 32 || rows[0].Status != "enabled" {
		t.Fatalf("backfilled language = %#v", rows[0])
	}
	if rows[1].ID != "existing-id" || rows[1].Status != "disabled" {
		t.Fatalf("preserved language = %#v", rows[1])
	}

	var versionCount int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&versionCount).Error; err != nil {
		t.Fatalf("count migration versions: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("migration version count = %d, want 1", versionCount)
	}
}

func TestLanguageIntegrityMigrationFailsClosedWithoutLanguageTable(t *testing.T) {
	db := openLanguageIntegrityMigrationDB(t)
	const version = "20260815120000-missing-table"
	if err := migrateLanguageIntegrity(db, version); err == nil {
		t.Fatal("migration succeeded without the language table")
	}
	var versionCount int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&versionCount).Error; err != nil {
		t.Fatalf("count migration versions: %v", err)
	}
	if versionCount != 0 {
		t.Fatalf("migration version count = %d, want 0", versionCount)
	}
}
