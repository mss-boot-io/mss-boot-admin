// Package app exposes the complete Admin backend as an importable application.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"github.com/mss-boot-io/mss-boot-admin/admin/cmd"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
)

var (
	// ErrApplicationExecuted reports reuse of an Application command lifecycle.
	ErrApplicationExecuted = errors.New("Admin application has already executed")
	// ErrApplicationRunning reports concurrent use of process-global Admin
	// compatibility state. Independent Applications may be built concurrently,
	// but only one owns the runtime lifecycle at a time.
	ErrApplicationRunning = errors.New("another Admin application is already running")
	applicationRunning    atomic.Bool
)

type options struct {
	modules []business.Module
}

// Option configures one complete Admin Application.
type Option func(*options) error

// WithBusinessModules explicitly composes compile-time business modules in the
// provided order.
func WithBusinessModules(modules ...business.Module) Option {
	return func(options *options) error {
		if options == nil {
			return errors.New("Admin application options are required")
		}
		options.modules = append(options.modules, modules...)
		return nil
	}
}

// Application owns one frozen business registry. Runtime command trees are
// private implementation details and may execute only through the guarded
// Execute entrypoints.
type Application struct {
	registry *business.Registry
	executed atomic.Bool
}

// New constructs an Application without mutating process-global runtime state.
func New(applicationOptions ...Option) (*Application, error) {
	configuration := &options{}
	for index, option := range applicationOptions {
		if option == nil {
			return nil, fmt.Errorf("Admin application option %d is nil", index)
		}
		if err := option(configuration); err != nil {
			return nil, fmt.Errorf("apply Admin application option %d: %w", index, err)
		}
	}
	registry, err := business.Compose(migration.Migrate, configuration.modules...)
	if err != nil {
		return nil, fmt.Errorf("compose Admin business modules: %w", err)
	}
	return &Application{registry: registry}, nil
}

// BusinessDescriptors returns immutable metadata in explicit composition
// order and is useful to hosts and compatibility tests.
func (application *Application) BusinessDescriptors() []business.Descriptor {
	if application == nil || application.registry == nil {
		return nil
	}
	return application.registry.Descriptors()
}

// ExecuteContext runs the current process arguments.
func (application *Application) ExecuteContext(ctx context.Context) error {
	return application.ExecuteArgsContext(ctx, os.Args[1:])
}

// ExecuteArgsContext is the testable command entrypoint. One Application may
// execute once; construct another Application for another lifecycle.
func (application *Application) ExecuteArgsContext(ctx context.Context, args []string) error {
	if application == nil || application.registry == nil {
		return errors.New("Admin application is not initialized")
	}
	if ctx == nil {
		return errors.New("Admin application context is required")
	}
	if !applicationRunning.CompareAndSwap(false, true) {
		return ErrApplicationRunning
	}
	defer applicationRunning.Store(false)
	if !application.executed.CompareAndSwap(false, true) {
		return ErrApplicationExecuted
	}
	command := cmd.New(application.registry)
	command.SetArgs(append([]string(nil), args...))
	return command.ExecuteContext(ctx)
}

// ExecuteContext constructs and executes the complete Admin application.
func ExecuteContext(ctx context.Context, applicationOptions ...Option) error {
	application, err := New(applicationOptions...)
	if err != nil {
		return err
	}
	return application.ExecuteContext(ctx)
}
