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
	// ErrRegistrationInProgress reports a concurrent parent-registry mutation.
	// Registration is deliberately single-writer and callers must retry only
	// after the active Module.Register callback has returned.
	ErrRegistrationInProgress = errors.New("business module registration is already in progress")
	// ErrRegistrationReentry reports a lifecycle operation attempted through the
	// transaction-local Registry view supplied to Module.Register. The error
	// poisons that transaction even when module code ignores the returned error.
	ErrRegistrationReentry = errors.New("business module registration cannot reenter the registry lifecycle")
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

// registrationTransaction stages every side effect from one Module.Register
// call. The parent registry publishes the staged entry and migrations together
// only after the callback returns and the complete migration set validates.
type registrationTransaction struct {
	mu          sync.Mutex
	module      string
	migrations  *migration.Migration
	entry       *entry
	registering bool
	closed      bool
	err         error
}

func newRegistrationTransaction(module string) *registrationTransaction {
	return &registrationTransaction{
		module:     module,
		migrations: migration.New(),
	}
}

func (transaction *registrationTransaction) recordError(err error) error {
	if err == nil {
		return nil
	}
	transaction.mu.Lock()
	defer transaction.mu.Unlock()
	transaction.recordErrorLocked(err)
	return err
}

func (transaction *registrationTransaction) recordErrorLocked(err error) {
	if transaction.closed {
		return
	}
	if transaction.err == nil {
		transaction.err = err
		return
	}
	if !errors.Is(transaction.err, err) {
		transaction.err = errors.Join(transaction.err, err)
	}
}

func (transaction *registrationTransaction) beginRegistration() error {
	transaction.mu.Lock()
	defer transaction.mu.Unlock()
	if transaction.closed {
		return fmt.Errorf("%w: transaction for %s is closed", ErrRegistrationReentry, transaction.module)
	}
	if transaction.err != nil {
		return transaction.err
	}
	if transaction.registering || transaction.entry != nil {
		err := fmt.Errorf(
			"business module %s provided more than one registration",
			transaction.module,
		)
		transaction.recordErrorLocked(err)
		return err
	}
	transaction.registering = true
	return nil
}

func (transaction *registrationTransaction) failRegistration(err error) error {
	transaction.mu.Lock()
	defer transaction.mu.Unlock()
	transaction.registering = false
	transaction.recordErrorLocked(err)
	return err
}

func (transaction *registrationTransaction) completeRegistration(registered entry) error {
	transaction.mu.Lock()
	defer transaction.mu.Unlock()
	transaction.registering = false
	if transaction.closed {
		return fmt.Errorf("%w: transaction for %s is closed", ErrRegistrationReentry, transaction.module)
	}
	if transaction.err != nil {
		return transaction.err
	}
	if transaction.entry != nil {
		err := fmt.Errorf(
			"business module %s provided more than one registration",
			transaction.module,
		)
		transaction.recordErrorLocked(err)
		return err
	}
	transaction.entry = &registered
	return nil
}

func (transaction *registrationTransaction) close(callbackErr error) (
	*entry,
	*migration.Migration,
	error,
) {
	transaction.mu.Lock()
	defer transaction.mu.Unlock()
	if callbackErr != nil {
		transaction.recordErrorLocked(callbackErr)
	}
	if transaction.registering {
		transaction.recordErrorLocked(fmt.Errorf(
			"business module %s registration is still in progress",
			transaction.module,
		))
		transaction.registering = false
	}
	transaction.closed = true
	return transaction.entry, transaction.migrations, transaction.err
}

// Registry is application-local. A parent Registry is mutable only while
// modules are being composed. Module.Register receives a transaction-local view
// whose only valid operation is Register; lifecycle reentry fails immediately.
type Registry struct {
	mu                 sync.RWMutex
	registrationGate   chan struct{}
	coreMigrations     *migration.Migration
	businessMigrations *migration.Migration
	migrations         *migration.Migration
	entries            []entry
	modules            map[string]struct{}
	active             *registrationTransaction
	transaction        *registrationTransaction
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
		registrationGate:   make(chan struct{}, 1),
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

func (r *Registry) transactionView() bool {
	return r != nil && r.transaction != nil
}

func (r *Registry) rejectTransactionLifecycle(operation string) error {
	err := fmt.Errorf("%w: %s", ErrRegistrationReentry, operation)
	if r != nil && r.transaction != nil {
		return r.transaction.recordError(err)
	}
	return err
}

func (r *Registry) tryLockRegistration() error {
	if r == nil || r.registrationGate == nil {
		return errors.New("business module registry is not initialized")
	}
	select {
	case r.registrationGate <- struct{}{}:
		return nil
	default:
		return ErrRegistrationInProgress
	}
}

func (r *Registry) unlockRegistration() {
	<-r.registrationGate
}

// Add explicitly composes one module. Registration is transactional: a module
// that returns an error, ignores a registration error, or attempts lifecycle
// reentry leaves no descriptor, identity, route, or migration behind.
func (r *Registry) Add(module Module) error {
	if r == nil {
		return errors.New("business module registry is required")
	}
	if r.transactionView() {
		return r.rejectTransactionLifecycle("Add")
	}
	if nilInterface(module) {
		return errors.New("business module is required")
	}
	name := strings.TrimSpace(module.Name())
	if name == "" {
		return errors.New("business module name is required")
	}
	if err := r.tryLockRegistration(); err != nil {
		return err
	}
	defer r.unlockRegistration()

	transaction := newRegistrationTransaction(name)
	r.mu.Lock()
	if r.frozen {
		r.mu.Unlock()
		return ErrRegistryFrozen
	}
	if _, exists := r.modules[name]; exists {
		r.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrDuplicateModule, name)
	}
	r.active = transaction
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		if r.active == transaction {
			r.active = nil
		}
		r.mu.Unlock()
	}()

	view := &Registry{transaction: transaction}
	callbackErr := module.Register(view)
	stagedEntry, stagedMigrations, registrationErr := transaction.close(callbackErr)
	if registrationErr != nil {
		return fmt.Errorf("register business module %s: %w", name, registrationErr)
	}
	if stagedEntry == nil {
		return fmt.Errorf("register business module %s: module did not provide a registration", name)
	}

	nextBusinessMigrations, err := migration.CombineRegistrations(
		r.businessMigrations,
		stagedMigrations,
	)
	if err != nil {
		return fmt.Errorf("register business module %s migrations: %w", name, err)
	}
	if _, err := migration.CombineRegistrations(r.coreMigrations, nextBusinessMigrations); err != nil {
		return fmt.Errorf("validate business module %s migrations: %w", name, err)
	}

	r.mu.Lock()
	r.businessMigrations = nextBusinessMigrations
	r.entries = append(r.entries, *stagedEntry)
	r.modules[name] = struct{}{}
	r.active = nil
	r.mu.Unlock()
	return nil
}

// Register stages the active module's complete runtime projection. It is valid
// only on the transaction-local Registry view passed to Module.Register. User
// migration callbacks execute without a parent Registry lock held.
func (r *Registry) Register(registration Registration) error {
	if r == nil {
		return errors.New("business module registry is required")
	}
	if !r.transactionView() {
		r.mu.RLock()
		frozen := r.frozen
		r.mu.RUnlock()
		if frozen {
			return ErrRegistryFrozen
		}
		return errors.New("business registration must be called from Module.Register")
	}
	transaction := r.transaction
	if err := transaction.beginRegistration(); err != nil {
		return err
	}

	descriptor := cloneDescriptor(registration.Descriptor)
	descriptor.Name = strings.TrimSpace(descriptor.Name)
	descriptor.DisplayName = strings.TrimSpace(descriptor.DisplayName)
	if descriptor.Name != transaction.module {
		return transaction.failRegistration(fmt.Errorf(
			"business module identity mismatch: Module.Name()=%q descriptor=%q",
			transaction.module,
			descriptor.Name,
		))
	}
	if descriptor.DisplayName == "" {
		return transaction.failRegistration(fmt.Errorf(
			"business module %s display name is required",
			transaction.module,
		))
	}
	if descriptor.Model == nil && descriptor.Migrate == nil {
		return transaction.failRegistration(fmt.Errorf(
			"business module %s must define a model or explicit compatibility migration",
			transaction.module,
		))
	}
	if registration.Readiness == nil {
		return transaction.failRegistration(fmt.Errorf(
			"business module %s readiness check is required",
			transaction.module,
		))
	}
	if registration.Routes == nil {
		return transaction.failRegistration(fmt.Errorf(
			"business module %s route registrar is required",
			transaction.module,
		))
	}
	if registration.Migrations != nil {
		if err := registration.Migrations(transaction.migrations); err != nil {
			return transaction.failRegistration(fmt.Errorf(
				"register business module %s migrations: %w",
				transaction.module,
				err,
			))
		}
	}
	if err := transaction.migrations.ValidateRegistrations(); err != nil {
		return transaction.failRegistration(fmt.Errorf(
			"validate business module %s migrations: %w",
			transaction.module,
			err,
		))
	}

	return transaction.completeRegistration(entry{
		name:       transaction.module,
		descriptor: descriptor,
		readiness:  registration.Readiness,
		routes:     registration.Routes,
	})
}

// Freeze validates the complete migration set and prevents later mutation.
func (r *Registry) Freeze() error {
	if r == nil {
		return errors.New("business module registry is required")
	}
	if r.transactionView() {
		return r.rejectTransactionLifecycle("Freeze")
	}
	if err := r.tryLockRegistration(); err != nil {
		return err
	}
	defer r.unlockRegistration()

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return nil
	}
	if r.active != nil {
		return ErrRegistrationInProgress
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
	if r == nil || r.transactionView() {
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
	if r.transactionView() {
		return nil, r.rejectTransactionLifecycle("MigrationRunner")
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
	if r.transactionView() {
		return MigrationPhases{}, r.rejectTransactionLifecycle("MigrationPhaseRunners")
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
	if r.transactionView() {
		return r.rejectTransactionLifecycle("Mount")
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
