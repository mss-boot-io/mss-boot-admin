package business

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/gorm"
)

type testRecord struct{}

type testModule struct {
	name             string
	registration     Registration
	err              error
	afterRegisterErr error
	skip             bool
}

func (module testModule) Name() string { return module.name }

func (module testModule) Register(registry *Registry) error {
	if module.err != nil {
		return module.err
	}
	if module.skip {
		return nil
	}
	if err := registry.Register(module.registration); err != nil {
		return err
	}
	return module.afterRegisterErr
}

type testEventCollector struct{}

func (testEventCollector) Collect(context.Context, Event) {}

func validTestModule(name string, migrationID migration.MigrationID) testModule {
	return testModule{
		name: name,
		registration: Registration{
			Descriptor: Descriptor{Name: name, DisplayName: strings.ToUpper(name), Model: new(testRecord)},
			Migrations: func(runner *migration.Migration) error {
				return runner.Register(migrationID, func(*gorm.DB, string) error { return nil })
			},
			Readiness: func(context.Context, *gorm.DB) error { return nil },
			Routes: func(group *gin.RouterGroup, _ Runtime) error {
				group.GET("/"+name, func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
				return nil
			},
		},
	}
}

func TestComposeRejectsInvalidDuplicateAndFailedModules(t *testing.T) {
	core := migration.New()
	if _, err := Compose(core, (*testModule)(nil)); err == nil || !strings.Contains(err.Error(), "module is required") {
		t.Fatalf("nil module error = %v", err)
	}
	if _, err := Compose(core, testModule{name: "blank", skip: true}); err == nil || !strings.Contains(err.Error(), "did not provide") {
		t.Fatalf("missing registration error = %v", err)
	}
	injected := errors.New("injected registration failure")
	if _, err := Compose(core, testModule{name: "failed", err: injected}); !errors.Is(err, injected) {
		t.Fatalf("registration failure = %v, want injected error", err)
	}

	module := validTestModule("supplier", migration.MigrationID("202608190101"))
	if _, err := Compose(core, module, module); !errors.Is(err, ErrDuplicateModule) {
		t.Fatalf("duplicate module error = %v", err)
	}
}

func TestAddRollsBackEveryFailedRegistrationSideEffect(t *testing.T) {
	tests := []struct {
		name   string
		module func(migration.MigrationID, error) testModule
	}{
		{
			name: "migration registrar fails after staging a migration",
			module: func(id migration.MigrationID, injected error) testModule {
				module := validTestModule("failed", id)
				module.registration.Migrations = func(runner *migration.Migration) error {
					if err := runner.Register(id, func(*gorm.DB, string) error { return nil }); err != nil {
						return err
					}
					return injected
				}
				return module
			},
		},
		{
			name: "module fails after a complete registration",
			module: func(id migration.MigrationID, injected error) testModule {
				module := validTestModule("failed", id)
				module.afterRegisterErr = injected
				return module
			},
		},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := NewRegistry(migration.New())
			if err != nil {
				t.Fatalf("new registry: %v", err)
			}
			migrationID := migration.MigrationID(
				[]string{"202608190110", "202608190111"}[index],
			)
			injected := errors.New("injected transactional registration failure")
			if err := registry.Add(tt.module(migrationID, injected)); !errors.Is(err, injected) {
				t.Fatalf("failed Add error = %v, want injected error", err)
			}
			if descriptors := registry.Descriptors(); len(descriptors) != 0 {
				t.Fatalf("failed module leaked descriptors: %#v", descriptors)
			}

			// Reusing both the failed module identity and its migration ID proves
			// that neither side effect escaped the discarded transaction.
			if err := registry.Add(validTestModule("failed", migrationID)); err != nil {
				t.Fatalf("replacement module inherited failed registration state: %v", err)
			}
			if err := registry.Freeze(); err != nil {
				t.Fatalf("freeze replacement registry: %v", err)
			}
			descriptors := registry.Descriptors()
			if len(descriptors) != 1 || descriptors[0].Name != "failed" {
				t.Fatalf("replacement descriptors = %#v", descriptors)
			}
		})
	}
}

func TestComposePreservesOrderCopiesDescriptorsAndFreezes(t *testing.T) {
	registry, err := Compose(
		migration.New(),
		validTestModule("zeta", migration.MigrationID("202608190102")),
		validTestModule("alpha", migration.MigrationID("202608190103")),
	)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	descriptors := registry.Descriptors()
	got := []string{descriptors[0].Name, descriptors[1].Name}
	if want := []string{"zeta", "alpha"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("descriptor order = %v, want %v", got, want)
	}
	descriptors[0].Name = "mutated"
	if got := registry.Descriptors()[0].Name; got != "zeta" {
		t.Fatalf("descriptor mutation leaked into registry: %q", got)
	}
	if err := registry.Add(validTestModule("later", migration.MigrationID("202608190104"))); !errors.Is(err, ErrRegistryFrozen) {
		t.Fatalf("post-freeze Add error = %v", err)
	}
	if err := registry.Register(Registration{}); !errors.Is(err, ErrRegistryFrozen) {
		t.Fatalf("post-freeze Register error = %v", err)
	}
}

func TestComposeDetectsMigrationCollisionWithoutMutatingCore(t *testing.T) {
	core := migration.New()
	id := migration.MigrationID("202608190105")
	if err := core.Register(id, func(*gorm.DB, string) error { return nil }); err != nil {
		t.Fatalf("register core migration: %v", err)
	}
	if _, err := Compose(core, validTestModule("collision", id)); !errors.Is(err, migration.ErrDuplicateMigrationID) {
		t.Fatalf("migration collision error = %v", err)
	}
	if err := core.ValidateRegistrations(); err != nil {
		t.Fatalf("failed composition mutated core migrations: %v", err)
	}
}

func TestMigrationRunnerRequiresFreezeAndReturnsIsolatedClones(t *testing.T) {
	registry, err := NewRegistry(migration.New())
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	if _, err := registry.MigrationRunner(); !errors.Is(err, ErrRegistryNotFrozen) {
		t.Fatalf("unfrozen migration runner error = %v", err)
	}
	if err := registry.Add(validTestModule("supplier", migration.MigrationID("202608190106"))); err != nil {
		t.Fatalf("add module: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	first, err := registry.MigrationRunner()
	if err != nil {
		t.Fatalf("first migration runner: %v", err)
	}
	second, err := registry.MigrationRunner()
	if err != nil {
		t.Fatalf("second migration runner: %v", err)
	}
	extraID := migration.MigrationID("202608190107")
	if err := first.Register(extraID, func(*gorm.DB, string) error { return nil }); err != nil {
		t.Fatalf("register first clone: %v", err)
	}
	if err := second.Register(extraID, func(*gorm.DB, string) error { return nil }); err != nil {
		t.Fatalf("migration runner clones share state: %v", err)
	}
}

func TestMigrationPhaseRunnersKeepCoreAheadOfBusiness(t *testing.T) {
	var calls []string
	registrationCalls := 0
	core := migration.New()
	if err := core.Register("20260820150000", func(db *gorm.DB, version string) error {
		calls = append(calls, "core")
		return core.CreateVersion(db, version)
	}); err != nil {
		t.Fatalf("register core migration: %v", err)
	}
	businessModule := validTestModule("supplier", "20260810150000")
	businessModule.registration.Migrations = func(runner *migration.Migration) error {
		registrationCalls++
		return runner.Register("20260810150000", func(db *gorm.DB, version string) error {
			calls = append(calls, "business")
			return runner.CreateVersion(db, version)
		})
	}

	registry, err := Compose(core, businessModule)
	if err != nil {
		t.Fatalf("compose phased registry: %v", err)
	}
	phases, err := registry.MigrationPhaseRunners()
	if err != nil {
		t.Fatalf("migration phases: %v", err)
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open migration phase database: %v", err)
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatalf("create migration ledger: %v", err)
	}
	for _, runner := range []*migration.Migration{phases.Core, phases.Business} {
		runner.SetDb(db)
		runner.SetModel(&migrationmodels.Migration{})
		if err := runner.MigrateContext(t.Context()); err != nil {
			t.Fatalf("run migration phase: %v", err)
		}
	}
	if want := []string{"core", "business"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("migration phase order = %v, want %v", calls, want)
	}
	if registrationCalls != 1 {
		t.Fatalf("business migration registrar called %d times, want once", registrationCalls)
	}
}

func TestMountChecksAllReadinessBeforeAnyRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	firstMounted := false
	secondReadiness := errors.New("second module is not ready")
	first := validTestModule("first", migration.MigrationID("202608190108"))
	first.registration.Routes = func(*gin.RouterGroup, Runtime) error {
		firstMounted = true
		return nil
	}
	second := validTestModule("second", migration.MigrationID("202608190109"))
	second.registration.Readiness = func(context.Context, *gorm.DB) error { return secondReadiness }
	registry, err := Compose(migration.New(), first, second)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	engine := gin.New()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open readiness database: %v", err)
	}

	err = registry.Mount(
		context.Background(),
		func(operation func(*gorm.DB) error) error { return operation(db) },
		engine.Group("/admin/api"),
		Runtime{
			RequestDatabase: func(context.Context) (*gorm.DB, bool) { return db, true },
			Principal:       nil,
			Events:          testEventCollector{},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "principal resolver") {
		t.Fatalf("invalid runtime error = %v", err)
	}

	// A valid principal resolver is supplied below; it may return nil because
	// readiness does not authenticate a request.
	err = registry.Mount(
		context.Background(),
		func(operation func(*gorm.DB) error) error { return operation(db) },
		engine.Group("/admin/api"),
		Runtime{
			RequestDatabase: func(context.Context) (*gorm.DB, bool) { return db, true },
			Principal:       func(*gin.Context) security.Verifier { return nil },
			Events:          testEventCollector{},
		},
	)
	if !errors.Is(err, secondReadiness) {
		t.Fatalf("readiness error = %v, want second module failure", err)
	}
	if firstMounted {
		t.Fatal("a route was mounted before all module readiness checks passed")
	}

	request := httptest.NewRequest(http.MethodGet, "/admin/api/first", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unready route status = %d, want 404", recorder.Code)
	}
}
