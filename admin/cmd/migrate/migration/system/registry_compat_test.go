package system

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
)

func TestV100MigrationRowsDoNotRerun(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "migration.db")
	db, err := gorm.Open(
		sqlite.Open(databasePath),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQLite database handle: %v", err)
	}
	t.Cleanup(func() {
		migration.Migrate.SetDb(nil)
		migration.Migrate.SetModel(nil)
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close SQLite database: %v", err)
		}
	})
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatalf("create migration table: %v", err)
	}

	// These are the exact values that the v1.0.0 int-based GetFilename helper
	// could persist. The collision has both known deployed forms: ...6205 on
	// upgraded pre-rename databases and ...6206 on databases created from the
	// v1.0.0 tag. The 10-digit source filename was converted to integer zero.
	v100Rows := []string{
		"0",
		"1691804837583",
		"1691847581348",
		"1722316153240",
		"1724396388009",
		"1746193492486",
		"1772445829126",
		"2026040322595",
		"2026040323000",
		"2026060707200",
		"2026060716205",
		"2026060716206",
		"2026061412000",
		"2026080613000",
		"2026080617000",
		"2026080617100",
		"2026080617200",
		"2026080620000",
		"2026080621000",
		"2026080622000",
		"2026080713000",
		"2026080715000",
		"2026080810000",
		"2026080812000",
		"2026080813000",
	}
	for _, version := range v100Rows {
		if err := db.Create(&migrationmodels.Migration{Version: version}).Error; err != nil {
			t.Fatalf("seed v1.0.0 migration row %s: %v", version, err)
		}
	}

	migration.Migrate.SetDb(db)
	migration.Migrate.SetModel(&migrationmodels.Migration{})
	if err := migration.Migrate.MigrateContext(context.Background()); err != nil {
		t.Fatalf("migrate v1.0.0 database: %v", err)
	}
	var rows int64
	if err := db.Model(&migrationmodels.Migration{}).Count(&rows).Error; err != nil {
		t.Fatalf("count migration rows: %v", err)
	}
	if rows != int64(len(v100Rows)) {
		t.Fatalf("migration rows = %d, want unchanged %d", rows, len(v100Rows))
	}
}
