package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	runtimeeventbus "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/eventbus"
	"gorm.io/gorm"
)

const defaultAuthorizationReconcileInterval = 15 * time.Second

var (
	ErrAuthorizationEventRuntimeInvalid = errors.New("authorization revision event runtime is invalid")
	ErrAuthorizationEventRuntimeClosed  = errors.New("authorization revision event runtime is closed")
	ErrAuthorizationEventRuntimeStarted = errors.New("authorization revision event runtime already started")
	ErrAuthorizationEventBusBound       = errors.New("authorization revision event bus is already bound")
)

// AuthorizationRevisionEvent is the domain marker carried by the typed
// EventBus. Event.Revision contains the committed global authorization
// revision; subscribers always reload policy from the authoritative database.
type AuthorizationRevisionEvent struct{}

// AuthorizationDatabaseUse lends the current application database for one
// bounded operation. Config can implement this with its existing database
// lease so a configuration reload cannot close a handle during reconciliation.
type AuthorizationDatabaseUse func(context.Context, func(*gorm.DB) error) error

// AuthorizationEventRuntime owns the Memory EventBus used by the Admin
// composition and periodically reconciles it against ConfigRevision. It is a
// server Runnable: Open is synchronous startup, Start owns the polling loop,
// and Close withdraws the publisher and closes the bus.
type AuthorizationEventRuntime struct {
	policy       *AuthorizationPolicyService
	useDB        AuthorizationDatabaseUse
	bus          *runtimeeventbus.Memory[AuthorizationRevisionEvent]
	reconciler   *runtimeeventbus.Reconciler[AuthorizationRevisionEvent]
	subscription *runtimeeventbus.Subscription
	interval     time.Duration

	transitionMu sync.Mutex
	mu           sync.Mutex
	opened       bool
	runAttempted bool
	runCancel    context.CancelFunc
	runDone      chan struct{}
	closed       bool
	lastErr      error
}

// BuildMemoryAuthorizationEventRuntime is pure: it allocates the Memory bus,
// subscriber, and authoritative reconciler without database I/O or goroutines.
func BuildMemoryAuthorizationEventRuntime(
	policy *AuthorizationPolicyService,
	useDB AuthorizationDatabaseUse,
	reconcileInterval time.Duration,
) (*AuthorizationEventRuntime, error) {
	if policy == nil || useDB == nil {
		return nil, ErrAuthorizationEventRuntimeInvalid
	}
	if reconcileInterval == 0 {
		reconcileInterval = defaultAuthorizationReconcileInterval
	}
	if reconcileInterval < time.Millisecond {
		return nil, ErrAuthorizationEventRuntimeInvalid
	}

	bus := runtimeeventbus.BuildMemory[AuthorizationRevisionEvent]()
	runtime := &AuthorizationEventRuntime{
		policy:   policy,
		useDB:    useDB,
		bus:      bus,
		interval: reconcileInterval,
		runDone:  closedSignal(),
	}
	subscription, err := bus.Subscribe(runtime.reloadSubscriber)
	if err != nil {
		return nil, fmt.Errorf("subscribe authorization revision handler: %w", err)
	}
	runtime.subscription = subscription
	reconciler, err := runtimeeventbus.BuildReconciler[AuthorizationRevisionEvent](
		runtimeeventbus.AuthoritativeSourceFunc[AuthorizationRevisionEvent](runtime.latestRevision),
	)
	if err != nil {
		subscription.Cancel()
		return nil, fmt.Errorf("build authorization revision reconciler: %w", err)
	}
	runtime.reconciler = reconciler
	return runtime, nil
}

// Open synchronously starts the Memory bus, reconciles the current committed
// revision, and then installs it as the service's sole publisher.
func (r *AuthorizationEventRuntime) Open(ctx context.Context) error {
	if r == nil {
		return ErrAuthorizationEventRuntimeInvalid
	}
	if ctx == nil {
		return errors.Join(ErrAuthorizationEventRuntimeInvalid, runtimeeventbus.ErrContextRequired)
	}
	r.transitionMu.Lock()
	defer r.transitionMu.Unlock()
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return ErrAuthorizationEventRuntimeClosed
	}
	if r.opened {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	if err := r.bus.Start(ctx); err != nil {
		return fmt.Errorf("start authorization revision event bus: %w", err)
	}
	if err := r.bus.Reconcile(ctx, r.reconciler); err != nil {
		_ = r.bus.Close(context.WithoutCancel(ctx))
		return fmt.Errorf("reconcile authorization revision event bus: %w", err)
	}
	if err := r.policy.bindAuthorizationEventBus(r.bus); err != nil {
		_ = r.bus.Close(context.WithoutCancel(ctx))
		return err
	}
	r.mu.Lock()
	r.opened = true
	r.lastErr = nil
	r.mu.Unlock()
	return nil
}

// Start owns periodic reconciliation until cancellation. A transient database
// or subscriber failure degrades Health but does not terminate the optional
// notification loop; a later successful cycle clears degradation.
func (r *AuthorizationEventRuntime) Start(ctx context.Context) (result error) {
	if r == nil {
		return ErrAuthorizationEventRuntimeInvalid
	}
	if ctx == nil {
		return errors.Join(ErrAuthorizationEventRuntimeInvalid, runtimeeventbus.ErrContextRequired)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	if !r.opened || r.closed {
		r.mu.Unlock()
		return ErrAuthorizationEventRuntimeClosed
	}
	if r.runAttempted {
		r.mu.Unlock()
		return ErrAuthorizationEventRuntimeStarted
	}
	r.runAttempted = true
	r.runDone = make(chan struct{})
	runCtx, cancel := context.WithCancel(ctx)
	r.runCancel = cancel
	r.mu.Unlock()

	defer func() {
		cancel()
		r.mu.Lock()
		r.runCancel = nil
		close(r.runDone)
		r.mu.Unlock()
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		result = errors.Join(result, r.Close(closeCtx))
	}()

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-runCtx.Done():
			return nil
		case <-ticker.C:
			err := r.Reconcile(runCtx)
			r.mu.Lock()
			r.lastErr = err
			r.mu.Unlock()
		}
	}
}

// Reconcile performs one authoritative revision comparison without waiting
// for the periodic loop.
func (r *AuthorizationEventRuntime) Reconcile(ctx context.Context) error {
	if r == nil {
		return ErrAuthorizationEventRuntimeInvalid
	}
	r.mu.Lock()
	opened := r.opened && !r.closed
	r.mu.Unlock()
	if !opened {
		return ErrAuthorizationEventRuntimeClosed
	}
	err := r.bus.Reconcile(ctx, r.reconciler)
	r.mu.Lock()
	r.lastErr = err
	r.mu.Unlock()
	return err
}

func (r *AuthorizationEventRuntime) Health(ctx context.Context) error {
	if r == nil {
		return ErrAuthorizationEventRuntimeInvalid
	}
	if err := r.bus.Health(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	degraded := r.lastErr
	r.mu.Unlock()
	if degraded != nil {
		return fmt.Errorf("authorization revision reconciliation is degraded: %w", degraded)
	}
	return nil
}

// Close is idempotent, cancels an active periodic loop, withdraws publication,
// and closes only the bus owned by this runtime.
func (r *AuthorizationEventRuntime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return runtimeeventbus.ErrContextRequired
	}
	r.transitionMu.Lock()
	defer r.transitionMu.Unlock()
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	cancel := r.runCancel
	done := r.runDone
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	r.policy.unbindAuthorizationEventBus(r.bus)
	if r.subscription != nil {
		r.subscription.Cancel()
	}
	if err := r.bus.Close(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	r.closed = true
	r.opened = false
	r.mu.Unlock()
	return nil
}

func (r *AuthorizationEventRuntime) String() string { return "authorization-revision-eventbus" }

func (r *AuthorizationEventRuntime) reloadSubscriber(ctx context.Context, _ runtimeeventbus.Event[AuthorizationRevisionEvent]) error {
	return r.useDB(ctx, func(db *gorm.DB) error {
		return r.policy.EnsureCurrent(ctx, db)
	})
}

func (r *AuthorizationEventRuntime) latestRevision(ctx context.Context) (
	runtimeeventbus.Event[AuthorizationRevisionEvent],
	bool,
	error,
) {
	var revision int64
	err := r.useDB(ctx, func(db *gorm.DB) error {
		var readErr error
		revision, readErr = EnsureAuthorizationGlobalRevision(ctx, db)
		return readErr
	})
	if err != nil {
		return runtimeeventbus.Event[AuthorizationRevisionEvent]{}, false, err
	}
	if revision <= 0 {
		return runtimeeventbus.Event[AuthorizationRevisionEvent]{}, false, nil
	}
	return runtimeeventbus.Event[AuthorizationRevisionEvent]{
		Revision: runtimeeventbus.Revision(revision),
		Payload:  AuthorizationRevisionEvent{},
	}, true, nil
}

func (e *AuthorizationPolicyService) bindAuthorizationEventBus(
	bus runtimeeventbus.EventBus[AuthorizationRevisionEvent],
) error {
	if e == nil || bus == nil {
		return ErrAuthorizationEventRuntimeInvalid
	}
	e.eventMu.Lock()
	defer e.eventMu.Unlock()
	if e.eventBus != nil && e.eventBus != bus {
		return ErrAuthorizationEventBusBound
	}
	e.eventBus = bus
	return nil
}

func (e *AuthorizationPolicyService) unbindAuthorizationEventBus(
	bus runtimeeventbus.EventBus[AuthorizationRevisionEvent],
) {
	if e == nil {
		return
	}
	e.eventMu.Lock()
	if e.eventBus == bus {
		e.eventBus = nil
	}
	e.eventMu.Unlock()
}

func closedSignal() chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}
