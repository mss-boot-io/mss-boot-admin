package migrate

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	"gorm.io/gorm"
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
