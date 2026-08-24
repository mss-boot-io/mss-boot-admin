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
	// This test isolates the reviewed v1.0.0 aliases. Seed migrations added
	// after v1.0.0 under their lossless IDs so they cannot turn this compatibility
	// fixture into an unrelated schema-integration test.
	if err := db.Create(&migrationmodels.Migration{
		Version: canonicalEmailIdentityMigrationID.String(),
	}).Error; err != nil {
		t.Fatalf("seed post-v1.0 canonical email migration row: %v", err)
	}
	if err := db.Create(&migrationmodels.Migration{
		Version: languageIntegrityMigrationID.String(),
	}).Error; err != nil {
		t.Fatalf("seed post-v1.0 language integrity migration row: %v", err)
	}
	if err := db.Create(&migrationmodels.Migration{
		Version: languageManagementPermissionsMigrationID.String(),
	}).Error; err != nil {
		t.Fatalf("seed post-v1.0 language permission migration row: %v", err)
	}
	if err := db.Create(&migrationmodels.Migration{
		Version: optionIntegrityMigrationID.String(),
	}).Error; err != nil {
		t.Fatalf("seed post-v1.0 option integrity migration row: %v", err)
	}
	if err := db.Create(&migrationmodels.Migration{
		Version: optionManagementPermissionsMigrationID.String(),
	}).Error; err != nil {
		t.Fatalf("seed post-v1.0 option permission migration row: %v", err)
	}
	if err := db.Create(&migrationmodels.Migration{
		Version: hideExampleSupplierMenuMigrationID.String(),
	}).Error; err != nil {
		t.Fatalf("seed post-v1.0 example supplier menu migration row: %v", err)
	}
	if err := db.Create(&migrationmodels.Migration{
		Version: accountReauthenticationMigrationID.String(),
	}).Error; err != nil {
		t.Fatalf("seed post-v1.0 account reauthentication migration row: %v", err)
	}
	if err := db.Create(&migrationmodels.Migration{
		Version: retireExampleSupplierRolesMigrationID.String(),
	}).Error; err != nil {
		t.Fatalf("seed post-v1.0 example supplier role migration row: %v", err)
	}
	if err := db.Create(&migrationmodels.Migration{
		Version: retireV5BrowserConfigMigrationID.String(),
	}).Error; err != nil {
		t.Fatalf("seed post-v1.0 V5 browser retirement migration row: %v", err)
	}
	if err := db.Create(&migrationmodels.Migration{
		Version: replayComposedSupplierAuthorizationMigrationID.String(),
	}).Error; err != nil {
		t.Fatalf("seed post-v1.0 Supplier authorization replay migration row: %v", err)
	}
	if err := db.Create(&migrationmodels.Migration{
		Version: presentationProfilesMigrationID.String(),
	}).Error; err != nil {
		t.Fatalf("seed post-v1.0 presentation profile migration row: %v", err)
	}
	if err := db.Create(&migrationmodels.Migration{
		Version: presentationProfilePermissionsMigrationID.String(),
	}).Error; err != nil {
		t.Fatalf("seed post-v1.0 presentation permission migration row: %v", err)
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
	wantRows := int64(len(v100Rows) + 12)
	if rows != wantRows {
		t.Fatalf("migration rows = %d, want unchanged %d", rows, wantRows)
	}
}
