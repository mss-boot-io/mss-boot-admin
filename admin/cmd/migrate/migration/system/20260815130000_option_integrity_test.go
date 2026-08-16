package system

import (
	"fmt"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openOptionIntegrityMigrationDB(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrate(&migrationmodels.Migration{}, &models.Option{}); err != nil {
		t.Fatalf("migrate option fixture schema: %v", err)
	}
	if err := db.Exec(`CREATE TABLE mss_boot_option_versions (
		id VARCHAR(64) PRIMARY KEY,
		created_at TIMESTAMP,
		updated_at TIMESTAMP,
		deleted_at TIMESTAMP,
		option_id VARCHAR(36) NOT NULL,
		version INT NOT NULL,
		items JSON,
		changed_by VARCHAR(36),
		change_note TEXT,
		status VARCHAR(10)
	)`).Error; err != nil {
		t.Fatalf("create legacy option version table: %v", err)
	}
	return db
}

func TestOptionIntegrityMigrationExpandsSnapshotsAndRepairsCanonicalValues(t *testing.T) {
	db := openOptionIntegrityMigrationDB(t)
	items := `[
		{"id":"","key":"first","label":"First","value":"1","sort":1},
		{"id":"same","key":"second","label":"Second","value":"2","sort":2},
		{"id":"same","key":"third","label":"Third","value":"3","sort":3}
	]`
	if err := db.Exec(`INSERT INTO mss_boot_options
		(id, category, display_name, description, name, remark, items, status, version, built_in, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"option-id", "system", "States", "preserve", "state", "remark", items, "", 0, false,
	).Error; err != nil {
		t.Fatalf("seed legacy option: %v", err)
	}

	const version = "20260815130000-test"
	for attempt := 1; attempt <= 2; attempt++ {
		if err := migrateOptionIntegrity(db, version); err != nil {
			t.Fatalf("migration attempt %d: %v", attempt, err)
		}
	}

	var option models.Option
	if err := db.First(&option, "id = ?", "option-id").Error; err != nil {
		t.Fatalf("load migrated option: %v", err)
	}
	if err := option.ValidateStored(); err != nil {
		t.Fatalf("migrated option is invalid: %v", err)
	}
	if option.Version != 1 || option.Status != "enabled" || option.Description != "preserve" {
		t.Fatalf("migrated option = %#v", option)
	}
	if option.Items == nil || len(*option.Items) != 3 {
		t.Fatalf("migrated items = %#v", option.Items)
	}
	ids := map[string]bool{}
	for _, item := range *option.Items {
		if item.ID == "" || ids[item.ID] {
			t.Fatalf("item IDs were not repaired uniquely: %#v", option.Items)
		}
		ids[item.ID] = true
	}
	for _, field := range optionVersionSnapshotColumns {
		if !db.Migrator().HasColumn(&models.OptionVersion{}, field) {
			t.Fatalf("snapshot column %s is missing", field)
		}
	}
	var versionCount int64
	if err := db.Model(&migrationmodels.Migration{}).Where("version = ?", version).Count(&versionCount).Error; err != nil {
		t.Fatalf("count migration version: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("migration version count = %d, want 1", versionCount)
	}
}

func TestOptionIntegrityMigrationFailsClosedForAmbiguousDictionary(t *testing.T) {
	db := openOptionIntegrityMigrationDB(t)
	items := `[
		{"id":"one","key":"duplicate","label":"First","value":"1"},
		{"id":"two","key":"duplicate","label":"Second","value":"2"}
	]`
	if err := db.Exec(`INSERT INTO mss_boot_options
		(id, category, name, remark, items, status, version, built_in, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"invalid-option", "system", "invalid", "", items, "enabled", 1, false,
	).Error; err != nil {
		t.Fatalf("seed invalid option: %v", err)
	}
	const version = "20260815130000-invalid"
	if err := migrateOptionIntegrity(db, version); err == nil {
		t.Fatal("migration accepted duplicate business keys")
	}
	var versionCount int64
	if err := db.Model(&migrationmodels.Migration{}).Where("version = ?", version).Count(&versionCount).Error; err != nil {
		t.Fatalf("count migration version: %v", err)
	}
	if versionCount != 0 {
		t.Fatalf("migration version count = %d, want 0", versionCount)
	}
}

func TestOptionIntegrityMigrationFailsClosedWithoutRequiredTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatalf("migrate version schema: %v", err)
	}
	if err := migrateOptionIntegrity(db, "20260815130000-missing"); err == nil {
		t.Fatal("migration succeeded without option tables")
	}
}
