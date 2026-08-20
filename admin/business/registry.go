// Package business defines the narrow, explicit extension boundary for
// compile-time Admin business modules.
package business

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	moduleruntime "github.com/mss-boot-io/mss-boot-admin/admin/modules/runtime"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/gorm"
)

var (
	// ErrRegistryFrozen reports an attempt to mutate a composed application.
	ErrRegistryFrozen = errors.New("business module registry is frozen")
	// ErrDuplicateModule reports two modules with the same stable identity.
	ErrDuplicateModule = errors.New("duplicate business module")
	// ErrRegistryNotFrozen reports runtime use before composition completed.
	ErrRegistryNotFrozen = errors.New("business module registry is not frozen")
)

// Permission declares one backend-enforced business action.
type Permission = moduleruntime.Permission

// Menu declares the default Admin navigation projection for a business module.
type Menu = moduleruntime.Menu

// Descriptor is the human- and machine-readable projection of a business
// module. Model is metadata only and never authorizes inferred production DDL.
type Descriptor = moduleruntime.Descriptor

// Event is the stable event identity required by the current generated module
// contract. Concrete payloads remain owned by their business module.
type Event interface {
	EventName() string
}

// EventCollector receives typed events after a business transaction commits.
type EventCollector interface {
	Collect(context.Context, Event)
}

// DatabaseAccess leases the current authoritative Admin database. The callback
// must not retain the handle after it returns.
type DatabaseAccess func(func(*gorm.DB) error) error

// RequestDatabase resolves the database lease bound to a request context.
type RequestDatabase func(context.Context) (*gorm.DB, bool)

// PrincipalResolver returns the identity installed by the complete Admin
// authentication middleware.
type PrincipalResolver func(*gin.Context) security.Verifier

// Runtime contains only dependencies required by generated business routes.
// The protected route group itself is created by the Admin composition root;
// modules cannot replace authentication, authorization, Session, CORS, or CSRF.
type Runtime struct {
	RequestDatabase RequestDatabase
	Principal       PrincipalResolver
	Events          EventCollector
}

// ReadinessCheck must prove the module's migrations and authorization storage
// are ready before any business route is mounted.
type ReadinessCheck func(context.Context, *gorm.DB) error

// RouteRegistrar mounts routes below an already protected /api group.
type RouteRegistrar func(*gin.RouterGroup, Runtime) error

// MigrationRegistrar adds every schema and authorization migration owned by a
// module to the application-local migration runner.
type MigrationRegistrar func(*migration.Migration) error

// Registration is the complete first-version BusinessModule projection.
type Registration struct {
	Descriptor Descriptor
	Migrations MigrationRegistrar
	Readiness  ReadinessCheck
	Routes     RouteRegistrar
}

// MigrationPhases keeps Foundation-owned migrations ahead of business-owned
// migrations without relying on globally comparable timestamp identifiers.
// Both runners are isolated clones and must execute in Core, then Business,
// order against the same migration ledger.
type MigrationPhases struct {
	Core     *migration.Migration
	Business *migration.Migration
}

// Module is the minimum compile-time extension interface supported by Admin.
type Module interface {
	Name() string
	Register(*Registry) error
}

type entry struct {
	name       string
	descriptor Descriptor
	readiness  ReadinessCheck
	routes     RouteRegistrar
}

// Registry is application-local. It is writable only while modules are being
// composed and immutable once frozen for migration or server execution.
type Registry struct {
	mu                 sync.RWMutex
	registrationMu     sync.Mutex
	coreMigrations     *migration.Migration
	businessMigrations *migration.Migration
	migrations         *migration.Migration
	entries            []entry
	modules            map[string]struct{}
	activeModule       string
	activeRegistered   bool
	frozen             bool
}

// NewRegistry clones only the core migration registrations. It never copies a
// database handle, version model, or previous execution state.
func NewRegistry(coreMigrations *migration.Migration) (*Registry, error) {
	if coreMigrations == nil {
		return nil, errors.New("core migration runner is required")
	}
	coreRunner, err := coreMigrations.CloneRegistrations()
	if err != nil {
		return nil, fmt.Errorf("clone core migrations: %w", err)
	}
	return &Registry{
		coreMigrations:     coreRunner,
		businessMigrations: migration.New(),
		modules:            make(map[string]struct{}),
	}, nil
}

// Compose creates, registers, and freezes one deterministic module set.
func Compose(coreMigrations *migration.Migration, modules ...Module) (*Registry, error) {
	registry, err := NewRegistry(coreMigrations)
	if err != nil {
		return nil, err
	}
	for index, module := range modules {
		if err := registry.Add(module); err != nil {
			return nil, fmt.Errorf("compose business module %d: %w", index, err)
		}
	}
	if err := registry.Freeze(); err != nil {
		return nil, err
	}
	return registry, nil
}

// Add explicitly composes one module. Calls are serialized so Register always
// belongs to the module currently being added.
func (r *Registry) Add(module Module) error {
	if r == nil {
		return errors.New("business module registry is required")
	}
	if nilInterface(module) {
		return errors.New("business module is required")
	}
	name := strings.TrimSpace(module.Name())
	if name == "" {
		return errors.New("business module name is required")
	}

	r.registrationMu.Lock()
	defer r.registrationMu.Unlock()

	r.mu.Lock()
	if r.frozen {
		r.mu.Unlock()
		return ErrRegistryFrozen
	}
	if _, exists := r.modules[name]; exists {
		r.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrDuplicateModule, name)
	}
	r.modules[name] = struct{}{}
	r.activeModule = name
	r.activeRegistered = false
	r.mu.Unlock()

	err := module.Register(r)

	r.mu.Lock()
	registered := r.activeRegistered
	r.activeModule = ""
	r.activeRegistered = false
	if err != nil || !registered {
		delete(r.modules, name)
	}
	r.mu.Unlock()
	if err != nil {
		return fmt.Errorf("register business module %s: %w", name, err)
	}
	if !registered {
		return fmt.Errorf("register business module %s: module did not provide a registration", name)
	}
	return nil
}

// Register records the active module's complete runtime projection.
func (r *Registry) Register(registration Registration) error {
	if r == nil {
		return errors.New("business module registry is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if r.activeModule == "" {
		return errors.New("business registration must be called from Module.Register")
	}
	if r.activeRegistered {
		return fmt.Errorf("business module %s provided more than one registration", r.activeModule)
	}

	descriptor := cloneDescriptor(registration.Descriptor)
	descriptor.Name = strings.TrimSpace(descriptor.Name)
	descriptor.DisplayName = strings.TrimSpace(descriptor.DisplayName)
	if descriptor.Name != r.activeModule {
		return fmt.Errorf(
			"business module identity mismatch: Module.Name()=%q descriptor=%q",
			r.activeModule,
			descriptor.Name,
		)
	}
	if descriptor.DisplayName == "" {
		return fmt.Errorf("business module %s display name is required", r.activeModule)
	}
	if descriptor.Model == nil && descriptor.Migrate == nil {
		return fmt.Errorf("business module %s must define a model or explicit compatibility migration", r.activeModule)
	}
	if registration.Readiness == nil {
		return fmt.Errorf("business module %s readiness check is required", r.activeModule)
	}
	if registration.Routes == nil {
		return fmt.Errorf("business module %s route registrar is required", r.activeModule)
	}
	if registration.Migrations != nil {
		if err := registration.Migrations(r.businessMigrations); err != nil {
			return fmt.Errorf("register business module %s migrations: %w", r.activeModule, err)
		}
	}
	r.entries = append(r.entries, entry{
		name:       r.activeModule,
		descriptor: descriptor,
		readiness:  registration.Readiness,
		routes:     registration.Routes,
	})
	r.activeRegistered = true
	return nil
}

// Freeze validates the complete migration set and prevents later mutation.
func (r *Registry) Freeze() error {
	if r == nil {
		return errors.New("business module registry is required")
	}
	r.registrationMu.Lock()
	defer r.registrationMu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return nil
	}
	if r.activeModule != "" {
		return fmt.Errorf("business module %s registration is still in progress", r.activeModule)
	}
	if err := r.coreMigrations.ValidateRegistrations(); err != nil {
		return fmt.Errorf("validate core migration phase: %w", err)
	}
	if err := r.businessMigrations.ValidateRegistrations(); err != nil {
		return fmt.Errorf("validate business migration phase: %w", err)
	}
	combinedRunner, err := migration.CombineRegistrations(r.coreMigrations, r.businessMigrations)
	if err != nil {
		return fmt.Errorf("combine application migrations: %w", err)
	}
	r.migrations = combinedRunner
	r.frozen = true
	return nil
}

// Descriptors returns defensive copies in explicit composition order.
func (r *Registry) Descriptors() []Descriptor {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Descriptor, 0, len(r.entries))
	for _, registered := range r.entries {
		result = append(result, cloneDescriptor(registered.descriptor))
	}
	return result
}

// MigrationRunner returns an isolated executable clone after composition.
func (r *Registry) MigrationRunner() (*migration.Migration, error) {
	if r == nil {
		return nil, errors.New("business module registry is required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.frozen {
		return nil, ErrRegistryNotFrozen
	}
	runner, err := r.migrations.CloneRegistrations()
	if err != nil {
		return nil, fmt.Errorf("clone application migrations: %w", err)
	}
	return runner, nil
}

// MigrationPhaseRunners returns isolated Core and Business runners after
// composition. The caller must execute Core before Business so a Foundation
// compatibility migration can never retire metadata that a composed business
// module seeded under an older numeric migration ID.
func (r *Registry) MigrationPhaseRunners() (MigrationPhases, error) {
	if r == nil {
		return MigrationPhases{}, errors.New("business module registry is required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.frozen {
		return MigrationPhases{}, ErrRegistryNotFrozen
	}
	coreRunner, err := r.coreMigrations.CloneRegistrations()
	if err != nil {
		return MigrationPhases{}, fmt.Errorf("clone core migration phase: %w", err)
	}
	businessRunner, err := r.businessMigrations.CloneRegistrations()
	if err != nil {
		return MigrationPhases{}, fmt.Errorf("clone business migration phase: %w", err)
	}
	return MigrationPhases{Core: coreRunner, Business: businessRunner}, nil
}

// Mount verifies every module before registering the first business route.
func (r *Registry) Mount(
	ctx context.Context,
	useDatabase DatabaseAccess,
	protectedAPI *gin.RouterGroup,
	runtime Runtime,
) error {
	if r == nil {
		return errors.New("business module registry is required")
	}
	if ctx == nil {
		return errors.New("business module context is required")
	}
	if useDatabase == nil {
		return errors.New("business module database lease is required")
	}
	if protectedAPI == nil {
		return errors.New("protected business API group is required")
	}
	if err := validateRuntime(runtime); err != nil {
		return err
	}

	r.mu.RLock()
	if !r.frozen {
		r.mu.RUnlock()
		return ErrRegistryNotFrozen
	}
	entries := append([]entry(nil), r.entries...)
	r.mu.RUnlock()

	for _, registered := range entries {
		if err := useDatabase(func(db *gorm.DB) error {
			if db == nil {
				return errors.New("leased database is nil")
			}
			return registered.readiness(ctx, db.WithContext(ctx))
		}); err != nil {
			return fmt.Errorf("business module %s readiness failed: %w", registered.name, err)
		}
	}
	for _, registered := range entries {
		if err := registered.routes(protectedAPI, runtime); err != nil {
			return fmt.Errorf("register business module %s routes: %w", registered.name, err)
		}
	}
	return nil
}

func validateRuntime(runtime Runtime) error {
	if runtime.RequestDatabase == nil {
		return errors.New("business request database resolver is required")
	}
	if runtime.Principal == nil {
		return errors.New("business principal resolver is required")
	}
	if nilInterface(runtime.Events) {
		return errors.New("business event collector is required")
	}
	return nil
}

func cloneDescriptor(descriptor Descriptor) Descriptor {
	clone := descriptor
	clone.Permissions = make([]Permission, len(descriptor.Permissions))
	for index := range descriptor.Permissions {
		clone.Permissions[index] = descriptor.Permissions[index]
		clone.Permissions[index].DefaultRoles = append(
			[]string(nil),
			descriptor.Permissions[index].DefaultRoles...,
		)
	}
	return clone
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
