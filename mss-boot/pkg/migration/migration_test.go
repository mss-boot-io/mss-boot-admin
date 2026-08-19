package migration

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
)

var migrationTestDatabaseSequence atomic.Uint64

var (
	_ func(string) int                                    = GetFilename
	_ func(*Migration, int, func(*gorm.DB, string) error) = (*Migration).SetVersion
	_ func(*Migration)                                    = (*Migration).Migrate
	_ func()                                              = New().Migrate
	_ interface{ Migrate() }                              = (*Migration)(nil)
)

type migrationTestEffect struct {
	ID string `gorm:"primaryKey"`
}

func newMigrationTestRunner(t *testing.T) (*Migration, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf(
		"file:migration-%d?mode=memory&cache=shared",
		migrationTestDatabaseSequence.Add(1),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite migration database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQLite migration database handle: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close SQLite migration database: %v", err)
		}
	})
	if err := db.AutoMigrate(&migrationmodels.Migration{}, &migrationTestEffect{}); err != nil {
		t.Fatalf("create migration test schema: %v", err)
	}
	runner := New()
	runner.SetDb(db)
	runner.SetModel(&migrationmodels.Migration{})
	return runner, db
}

func recordMigration(runner *Migration, calls *[]string) MigrationFunc {
	return func(db *gorm.DB, version string) error {
		*calls = append(*calls, version)
		return db.Transaction(func(tx *gorm.DB) error {
			return runner.CreateVersion(tx, version)
		})
	}
}

func TestMigrationTypedIDOrdering(t *testing.T) {
	runner, _ := newMigrationTestRunner(t)
	var calls []string
	for _, rawID := range []string{
		"202608061300001234567890",
		"1691804837583",
		"10",
		"9",
		"1775118941",
	} {
		id, err := ParseMigrationID(rawID)
		if err != nil {
			t.Fatalf("parse ID %q: %v", rawID, err)
		}
		if err := runner.Register(id, recordMigration(runner, &calls)); err != nil {
			t.Fatalf("register ID %q: %v", rawID, err)
		}
	}

	if err := runner.MigrateContext(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	want := []string{"9", "10", "1775118941", "1691804837583", "202608061300001234567890"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("migration order = %v, want %v", calls, want)
	}

	got := FilenameMigrationID("/tmp/202608061300001234567890_large_id.go")
	if got.String() != "202608061300001234567890" {
		t.Fatalf("lossless filename ID = %q", got)
	}
	if _, err := ParseMigrationID("020260806130000"); !errors.Is(err, ErrInvalidMigrationID) {
		t.Fatalf("leading-zero ID error = %v, want invalid migration ID", err)
	}
}

func TestMigrationDeprecatedRegistrationBridge(t *testing.T) {
	runner, _ := newMigrationTestRunner(t)
	var calls []string
	legacyID := GetFilename("20260806130000_bridge.go")
	if legacyID != 2026080613000 {
		t.Fatalf("deprecated filename ID = %d", legacyID)
	}
	runner.SetVersion(legacyID, recordMigration(runner, &calls))

	runner.Migrate()
	want := []string{"2026080613000"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("compatibility bridge calls = %v, want %v", calls, want)
	}
}

func TestMigrationRegistrationPreflightRequiresNoDatabase(t *testing.T) {
	runner := New()
	var calls atomic.Int64
	migrationFn := func(*gorm.DB, string) error {
		calls.Add(1)
		return nil
	}
	if err := runner.Register(MigrationID("20260806130000"), migrationFn); err != nil {
		t.Fatalf("register first migration: %v", err)
	}
	if err := runner.Register(MigrationID("20260806130000"), migrationFn); !errors.Is(err, ErrDuplicateMigrationID) {
		t.Fatalf("duplicate registration error = %v", err)
	}

	if err := runner.ValidateRegistrations(); !errors.Is(err, ErrDuplicateMigrationID) {
		t.Fatalf("registration preflight error = %v, want duplicate ID", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("migration callbacks executed %d times during registration preflight", got)
	}
}

func TestMigrationCloneRegistrationsIsIsolated(t *testing.T) {
	source := New()
	firstID := MigrationID("202608190001")
	legacyID := MigrationID("1691804837583")
	if err := source.RegisterWithLegacyIDs(
		firstID,
		[]MigrationID{legacyID},
		func(*gorm.DB, string) error { return nil },
	); err != nil {
		t.Fatalf("register source migration: %v", err)
	}

	clone, err := source.CloneRegistrations()
	if err != nil {
		t.Fatalf("clone registrations: %v", err)
	}
	if clone.GetDb() != nil {
		t.Fatal("cloned runner unexpectedly inherited database state")
	}
	if clone.Model != nil {
		t.Fatal("cloned runner unexpectedly inherited version model state")
	}

	secondID := MigrationID("202608190002")
	if err := clone.Register(secondID, func(*gorm.DB, string) error { return nil }); err != nil {
		t.Fatalf("register clone-only migration: %v", err)
	}
	if err := source.Register(secondID, func(*gorm.DB, string) error { return nil }); err != nil {
		t.Fatalf("clone mutation leaked into source runner: %v", err)
	}

	if err := clone.Register(legacyID, func(*gorm.DB, string) error { return nil }); !errors.Is(err, ErrDuplicateMigrationID) {
		t.Fatalf("cloned legacy alias ownership error = %v, want duplicate ID", err)
	}
}

func TestMigrationCloneRegistrationsRejectsInvalidSource(t *testing.T) {
	var nilRunner *Migration
	if _, err := nilRunner.CloneRegistrations(); !errors.Is(err, ErrMigrationNotReady) {
		t.Fatalf("nil clone error = %v, want migration not ready", err)
	}

	source := New()
	migrationFn := func(*gorm.DB, string) error { return nil }
	if err := source.Register(MigrationID("202608190003"), migrationFn); err != nil {
		t.Fatalf("register first migration: %v", err)
	}
	if err := source.Register(MigrationID("202608190003"), migrationFn); !errors.Is(err, ErrDuplicateMigrationID) {
		t.Fatalf("duplicate registration error = %v", err)
	}
	if _, err := source.CloneRegistrations(); !errors.Is(err, ErrDuplicateMigrationID) {
		t.Fatalf("clone invalid source error = %v, want duplicate ID", err)
	}
}

func TestMigrationTemplateUsesLosslessID(t *testing.T) {
	templateSource, err := FS.ReadFile("migrate.tpl")
	if err != nil {
		t.Fatalf("read migration template: %v", err)
	}
	source := string(templateSource)
	if !strings.Contains(source, "Register(migration.FilenameMigrationID(fileName)") {
		t.Fatalf("migration template does not use lossless typed registration:\n%s", source)
	}
	if strings.Contains(source, "GetFilename(fileName)") {
		t.Fatalf("migration template still uses deprecated truncated filename helper:\n%s", source)
	}
}

func TestMigrationDuplicateIDFailsBeforeExecution(t *testing.T) {
	runner, db := newMigrationTestRunner(t)
	var calls atomic.Int64
	migrationFn := func(*gorm.DB, string) error {
		calls.Add(1)
		return nil
	}
	if err := runner.Register(MigrationID("20260806130000"), migrationFn); err != nil {
		t.Fatalf("register first migration: %v", err)
	}
	if err := runner.Register(MigrationID("20260806130000"), migrationFn); !errors.Is(err, ErrDuplicateMigrationID) {
		t.Fatalf("duplicate registration error = %v", err)
	}

	err := runner.MigrateContext(context.Background())
	if !errors.Is(err, ErrDuplicateMigrationID) {
		t.Fatalf("migrate error = %v, want duplicate ID", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("migration callbacks executed %d times before duplicate preflight", got)
	}
	var rows int64
	if err := db.Model(&migrationmodels.Migration{}).Count(&rows).Error; err != nil {
		t.Fatalf("count migration rows: %v", err)
	}
	if rows != 0 {
		t.Fatalf("migration rows = %d, want zero", rows)
	}
}

func TestMigrationErrorPropagatesWithoutExit(t *testing.T) {
	runner, _ := newMigrationTestRunner(t)
	injected := errors.New("injected migration failure")
	if err := runner.Register(MigrationID("20260806130000"), func(*gorm.DB, string) error {
		return injected
	}); err != nil {
		t.Fatalf("register migration: %v", err)
	}

	err := runner.MigrateContext(context.Background())
	if !errors.Is(err, injected) {
		t.Fatalf("migrate error = %v, want injected error", err)
	}
	// Reaching this assertion demonstrates that the library returned control to
	// the caller instead of invoking os.Exit or log.Fatal.
	if err == nil {
		t.Fatal("migration failure was silently ignored")
	}
}

func TestMigrationDeprecatedBridgeReturnsAfterFailure(t *testing.T) {
	runner, _ := newMigrationTestRunner(t)
	var calls atomic.Int64
	injected := errors.New("injected legacy migration failure")
	if err := runner.Register(MigrationID("20260806130000"), func(*gorm.DB, string) error {
		calls.Add(1)
		return injected
	}); err != nil {
		t.Fatalf("register migration: %v", err)
	}

	// The v1.0-compatible bridge cannot return an error, but it must return
	// control to the caller instead of terminating or panicking.
	runner.Migrate()
	if got := calls.Load(); got != 1 {
		t.Fatalf("legacy migration callbacks = %d, want one", got)
	}
}

func TestMigrationContextCancellationPropagates(t *testing.T) {
	runner, _ := newMigrationTestRunner(t)
	var calls atomic.Int64
	if err := runner.Register(MigrationID("20260806130000"), func(*gorm.DB, string) error {
		calls.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("register migration: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := runner.MigrateContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("migrate error = %v, want context cancellation", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("canceled migration executed %d callbacks", got)
	}
}

func TestMigrationRepeatIsNoop(t *testing.T) {
	runner, db := newMigrationTestRunner(t)
	var calls atomic.Int64
	if err := runner.Register(MigrationID("20260806130000"), func(db *gorm.DB, version string) error {
		calls.Add(1)
		return db.Transaction(func(tx *gorm.DB) error {
			return runner.CreateVersion(tx, version)
		})
	}); err != nil {
		t.Fatalf("register migration: %v", err)
	}

	for run := 1; run <= 2; run++ {
		if err := runner.MigrateContext(context.Background()); err != nil {
			t.Fatalf("migration run %d: %v", run, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("migration callback calls = %d, want one", got)
	}
	var rows int64
	if err := db.Model(&migrationmodels.Migration{}).Count(&rows).Error; err != nil {
		t.Fatalf("count migration rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("migration rows = %d, want one", rows)
	}
}

func TestMigrationFailurePreservesAppliedState(t *testing.T) {
	runner, db := newMigrationTestRunner(t)
	injected := errors.New("injected second migration failure")
	var firstCalls atomic.Int64
	var secondCalls atomic.Int64
	var thirdCalls atomic.Int64
	var failSecond atomic.Bool
	failSecond.Store(true)

	register := func(id MigrationID, calls *atomic.Int64, shouldFail bool) {
		t.Helper()
		err := runner.Register(id, func(db *gorm.DB, version string) error {
			calls.Add(1)
			return db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Create(&migrationTestEffect{ID: version}).Error; err != nil {
					return err
				}
				if shouldFail && failSecond.Load() {
					return injected
				}
				return runner.CreateVersion(tx, version)
			})
		})
		if err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
	}
	register("100", &firstCalls, false)
	register("200", &secondCalls, true)
	register("300", &thirdCalls, false)

	if err := runner.MigrateContext(context.Background()); !errors.Is(err, injected) {
		t.Fatalf("first run error = %v, want injected failure", err)
	}
	if got := firstCalls.Load(); got != 1 {
		t.Fatalf("first migration calls = %d, want one", got)
	}
	if got := secondCalls.Load(); got != 1 {
		t.Fatalf("second migration calls = %d, want one", got)
	}
	if got := thirdCalls.Load(); got != 0 {
		t.Fatalf("third migration calls = %d, want zero", got)
	}
	assertMigrationState(t, db, []string{"100"}, []string{"100"})

	failSecond.Store(false)
	if err := runner.MigrateContext(context.Background()); err != nil {
		t.Fatalf("retry migration: %v", err)
	}
	if got := firstCalls.Load(); got != 1 {
		t.Fatalf("applied migration reran; calls = %d", got)
	}
	if got := secondCalls.Load(); got != 2 {
		t.Fatalf("failed migration retry calls = %d, want two", got)
	}
	if got := thirdCalls.Load(); got != 1 {
		t.Fatalf("third migration calls = %d, want one", got)
	}
	assertMigrationState(t, db, []string{"100", "200", "300"}, []string{"100", "200", "300"})
}

func TestMigrationV100AliasesSkipHistoricalRows(t *testing.T) {
	runner, db := newMigrationTestRunner(t)
	for _, version := range []string{"2026060716205", "2026060716206", "0"} {
		if err := db.Create(&migrationmodels.Migration{Version: version}).Error; err != nil {
			t.Fatalf("seed historical migration %s: %v", version, err)
		}
	}

	var calls atomic.Int64
	if err := runner.SetV100Version(
		"20260607162057_user_sessions.go",
		func(*gorm.DB, string) error {
			calls.Add(1)
			return nil
		},
	); err != nil {
		t.Fatalf("register first collision migration: %v", err)
	}
	if err := runner.RegisterWithLegacyIDs(
		FilenameMigrationID("20260607162060_session_menus.go"),
		[]MigrationID{"2026060716205", "2026060716206"},
		func(*gorm.DB, string) error {
			calls.Add(1)
			return nil
		},
	); err != nil {
		t.Fatalf("register renamed collision migration: %v", err)
	}
	if err := runner.SetV100Version(
		"1775118941_migrate.go",
		func(*gorm.DB, string) error {
			calls.Add(1)
			return nil
		},
	); err != nil {
		t.Fatalf("register legacy zero migration: %v", err)
	}

	if err := runner.MigrateContext(context.Background()); err != nil {
		t.Fatalf("migrate historical database: %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("historical migrations reran %d times", got)
	}

	legacyZero, err := V100MigrationIDFromFilename("1775118941_migrate.go")
	if err != nil {
		t.Fatalf("derive v1.0.0 ID: %v", err)
	}
	if legacyZero != "0" {
		t.Fatalf("10-digit v1.0.0 migration ID = %q, want 0", legacyZero)
	}
}

func TestMigrationV100CollisionFreshDatabaseUsesCanonicalIDs(t *testing.T) {
	runner, _ := newMigrationTestRunner(t)
	var calls []string
	if err := runner.SetV100Version(
		"20260607162057_user_sessions.go",
		recordMigration(runner, &calls),
	); err != nil {
		t.Fatalf("register user sessions migration: %v", err)
	}
	if err := runner.RegisterWithLegacyIDs(
		FilenameMigrationID("20260607162060_session_menus.go"),
		[]MigrationID{"2026060716205", "2026060716206"},
		recordMigration(runner, &calls),
	); err != nil {
		t.Fatalf("register session menus migration: %v", err)
	}

	if err := runner.MigrateContext(context.Background()); err != nil {
		t.Fatalf("migrate fresh database: %v", err)
	}
	want := []string{"20260607162057", "20260607162060"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("canonical collision IDs = %v, want %v", calls, want)
	}
}

func TestMigrationV100RenamedAliasSkipsTagRow(t *testing.T) {
	runner, db := newMigrationTestRunner(t)
	if err := db.Create(&migrationmodels.Migration{Version: "2026060716206"}).Error; err != nil {
		t.Fatalf("seed v1.0.0 tag migration row: %v", err)
	}
	var calls atomic.Int64
	if err := runner.RegisterWithLegacyIDs(
		FilenameMigrationID("20260607162060_session_menus.go"),
		[]MigrationID{"2026060716205", "2026060716206"},
		func(*gorm.DB, string) error {
			calls.Add(1)
			return nil
		},
	); err != nil {
		t.Fatalf("register renamed migration: %v", err)
	}

	if err := runner.MigrateContext(context.Background()); err != nil {
		t.Fatalf("migrate v1.0.0 tag database: %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("renamed v1.0.0 migration reran %d times", got)
	}
}

func TestMigrationV100RenamedAliasSkipsPreRenameRow(t *testing.T) {
	runner, db := newMigrationTestRunner(t)
	if err := db.Create(&migrationmodels.Migration{Version: "2026060716205"}).Error; err != nil {
		t.Fatalf("seed pre-rename migration row: %v", err)
	}
	var calls atomic.Int64
	if err := runner.RegisterWithLegacyIDs(
		FilenameMigrationID("20260607162060_session_menus.go"),
		[]MigrationID{"2026060716205", "2026060716206"},
		func(*gorm.DB, string) error {
			calls.Add(1)
			return nil
		},
	); err != nil {
		t.Fatalf("register renamed migration: %v", err)
	}

	if err := runner.MigrateContext(context.Background()); err != nil {
		t.Fatalf("migrate pre-rename database: %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("renamed pre-v1.0 migration reran %d times", got)
	}
}

func assertMigrationState(t *testing.T, db *gorm.DB, versions, effects []string) {
	t.Helper()
	var gotVersions []string
	if err := db.Model(&migrationmodels.Migration{}).
		Order("version").
		Pluck("version", &gotVersions).Error; err != nil {
		t.Fatalf("load migration versions: %v", err)
	}
	if !reflect.DeepEqual(gotVersions, versions) {
		t.Fatalf("migration versions = %v, want %v", gotVersions, versions)
	}
	var gotEffects []string
	if err := db.Model(&migrationTestEffect{}).
		Order("id").
		Pluck("id", &gotEffects).Error; err != nil {
		t.Fatalf("load migration effects: %v", err)
	}
	if !reflect.DeepEqual(gotEffects, effects) {
		t.Fatalf("migration effects = %v, want %v", gotEffects, effects)
	}
}
