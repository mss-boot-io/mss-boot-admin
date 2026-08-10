package migrate

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/schemahealth"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMigrateContextRejectsDuplicateRegistrationBeforeDatabaseAccess(t *testing.T) {
	runner := migration.New()
	var calls atomic.Int64
	migrationFn := func(*gorm.DB, string) error {
		calls.Add(1)
		return nil
	}
	if err := runner.Register("20260806130000", migrationFn); err != nil {
		t.Fatalf("register first migration: %v", err)
	}
	if err := runner.Register("20260806130000", migrationFn); !errors.Is(err, migration.ErrDuplicateMigrationID) {
		t.Fatalf("duplicate registration error = %v", err)
	}

	// This deliberately uninitialized GORM value is only a non-nil sentinel. Any
	// WithContext, AutoMigrate, or query attempted before registration preflight
	// would panic or fail the test instead of returning the duplicate-ID error.
	dbSentinel := &gorm.DB{}
	err := migrateContextWithRunner(context.Background(), dbSentinel, runner)
	if !errors.Is(err, migration.ErrDuplicateMigrationID) {
		t.Fatalf("migrate error = %v, want duplicate migration ID", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("migration callbacks executed %d times before duplicate preflight", got)
	}
}

func TestMigrateContextUsesCanonicalEmailRuntimeVerifierAfterMigrations(t *testing.T) {
	tests := []struct {
		name        string
		createIndex bool
		wantReady   bool
	}{
		{name: "compatible invariant is ready", createIndex: true, wantReady: true},
		{name: "missing invariant fails closed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := filepath.Join(t.TempDir(), "migrate-readiness.db")
			db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })

			runner := migration.New()
			err = runner.Register(
				migration.MigrationID(schemahealth.CanonicalEmailIdentityMigrationVersion),
				func(db *gorm.DB, version string) error {
					if err := db.Exec(`CREATE TABLE mss_boot_users (
						id TEXT PRIMARY KEY,
						email TEXT NULL,
						deleted_at DATETIME NULL
					)`).Error; err != nil {
						return err
					}
					if test.createIndex {
						if err := db.Exec(
							"CREATE UNIQUE INDEX " + models.EmailIdentityUniqueIndex +
								" ON mss_boot_users (LOWER(TRIM(email)))" +
								" WHERE deleted_at IS NULL AND TRIM(email) <> ''",
						).Error; err != nil {
							return err
						}
					}
					row := &migrationmodels.Migration{}
					row.SetVersion(version)
					return db.Create(row).Error
				},
			)
			if err != nil {
				t.Fatal(err)
			}

			err = migrateContextWithRunner(t.Context(), db, runner)
			if test.wantReady {
				if err != nil {
					t.Fatalf("migration readiness failed: %v", err)
				}
				return
			}
			if !errors.Is(err, schemahealth.ErrCanonicalEmailIdentityNotReady) {
				t.Fatalf("migration error = %v, want canonical-email readiness failure", err)
			}
		})
	}
}
