package system

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestReplayComposedSupplierAuthorizationClearsOnlyStaleAuthorizationVersion(t *testing.T) {
	db := openSupplierAuthorizationReplayDatabase(t)
	if err := db.Exec("CREATE TABLE biz_suppliers (id TEXT PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create Supplier table: %v", err)
	}
	seedSupplierMigrationVersions(
		t,
		db,
		exampleSupplierSchemaMigrationID.String(),
		exampleSupplierAuthorizationMigrationID.String(),
		hideExampleSupplierMenuMigrationID.String(),
	)

	if err := _20260820130000ReplayComposedSupplierAuthorization(
		db,
		replayComposedSupplierAuthorizationMigrationID.String(),
	); err != nil {
		t.Fatalf("replay compatibility migration: %v", err)
	}

	assertSupplierMigrationVersionCount(t, db, exampleSupplierAuthorizationMigrationID.String(), 0)
	assertSupplierMigrationVersionCount(t, db, exampleSupplierSchemaMigrationID.String(), 1)
	assertSupplierMigrationVersionCount(t, db, hideExampleSupplierMenuMigrationID.String(), 1)
	assertSupplierMigrationVersionCount(t, db, replayComposedSupplierAuthorizationMigrationID.String(), 1)
}

func TestReplayComposedSupplierAuthorizationPreservesUnretiredHistory(t *testing.T) {
	db := openSupplierAuthorizationReplayDatabase(t)
	if err := db.Exec("CREATE TABLE biz_suppliers (id TEXT PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create Supplier table: %v", err)
	}
	seedSupplierMigrationVersions(
		t,
		db,
		exampleSupplierSchemaMigrationID.String(),
		exampleSupplierAuthorizationMigrationID.String(),
	)

	if err := _20260820130000ReplayComposedSupplierAuthorization(
		db,
		replayComposedSupplierAuthorizationMigrationID.String(),
	); err != nil {
		t.Fatalf("replay compatibility migration: %v", err)
	}

	assertSupplierMigrationVersionCount(t, db, exampleSupplierAuthorizationMigrationID.String(), 1)
	assertSupplierMigrationVersionCount(t, db, replayComposedSupplierAuthorizationMigrationID.String(), 1)
}

func openSupplierAuthorizationReplayDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "supplier-replay.db")), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatalf("open Supplier replay database: %v", err)
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatalf("create migration ledger: %v", err)
	}
	return db
}

func seedSupplierMigrationVersions(t *testing.T, db *gorm.DB, versions ...string) {
	t.Helper()
	for _, version := range versions {
		row := &migrationmodels.Migration{}
		row.SetVersion(version)
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed migration version %s: %v", version, err)
		}
	}
}

func assertSupplierMigrationVersionCount(t *testing.T, db *gorm.DB, version string, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).Where("version = ?", version).Count(&count).Error; err != nil {
		t.Fatalf("count migration version %s: %v", version, err)
	}
	if count != want {
		t.Fatalf("migration version %s count = %d, want %d", version, count, want)
	}
}
