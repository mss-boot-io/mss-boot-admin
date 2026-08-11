package redisresource

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
)

// Keep this type non-zero-sized: Go permits pointers to distinct zero-sized
// variables to compare equal, which is unsuitable for a capability token.
type keyCapability struct{ marker byte }

// Key is an opaque scope capability. Its physical value and logical key
// are intentionally unavailable to consumers and to ordinary formatting.
type Key struct {
	value        string
	resourceName string
	scopeName    string
	owner        *keyCapability
}

func (k Key) String() string {
	if k.owner == nil {
		return "RedisKey<invalid>"
	}
	return fmt.Sprintf("RedisKey{resource:%q scope:%q}", k.resourceName, k.scopeName)
}

func (k Key) GoString() string { return k.String() }

// Lease is valid only while its Use callback is running. Every command
// accepts a caller context, applies scope isolation, and returns redacted
// errors that retain errors.Is/errors.As classification.
type Lease interface {
	QualifyKey(logical string) (Key, error)
	Get(context.Context, Key) ([]byte, error)
	Set(context.Context, Key, []byte, time.Duration) error
	Delete(context.Context, ...Key) (int64, error)
	Exists(context.Context, ...Key) (int64, error)
}

type lease struct {
	mu           sync.Mutex
	active       bool
	inFlight     int
	inFlight0    chan struct{}
	ctx          context.Context
	cancel       context.CancelCauseFunc
	client       ownedClient
	resourceName string
	scopeName    string
	prefix       string
	cluster      bool
	capability   *keyCapability
}

// Scope is a stable, resource-owned consumer namespace. Repeated calls to
// Resource.Scope with the same canonical name return the same capability.
// Different scopes share the Resource's single client and lifecycle.
type Scope struct {
	resource   *Resource
	name       string
	prefix     string
	cluster    bool
	capability *keyCapability
}

// Scope returns a deterministic consumer capability without constructing a
// client or changing resource lifecycle state.
func (r *Resource) Scope(scopeName string) (*Scope, error) {
	if r == nil {
		return nil, lifecycleError(OperationScope, ErrUseRejected, ErrUseRejected)
	}
	if !validScopeName(scopeName) {
		return nil, invalidCommand("scopeName", "must be canonical")
	}
	r.state.mu.Lock()
	if r.state.closing || r.state.closed {
		r.state.mu.Unlock()
		return nil, lifecycleError(OperationScope, ErrClosing, ErrClosing)
	}
	r.scopeMu.Lock()
	defer r.state.mu.Unlock()
	defer r.scopeMu.Unlock()
	if scope := r.scopes[scopeName]; scope != nil {
		return scope, nil
	}
	prefix := r.name + ":" + scopeName
	scope := &Scope{
		resource:   r,
		name:       scopeName,
		prefix:     prefix,
		cluster:    r.spec.mode == runtimeconfig.RedisCluster,
		capability: &keyCapability{},
	}
	r.scopes[scopeName] = scope
	return scope, nil
}

func (s *Scope) String() string {
	if s == nil || s.resource == nil {
		return "RedisScope<nil>"
	}
	return fmt.Sprintf("RedisScope{resource:%q name:%q}", s.resource.name, s.name)
}

func (s *Scope) GoString() string { return s.String() }

// Use lends a client capability until callback returns. A callback must not
// retain the Lease; retained method calls deterministically return
// ErrLeaseExpired. Close waits for the callback itself to return.
func (s *Scope) Use(ctx context.Context, callback func(Lease) error) error {
	if ctx == nil {
		return lifecycleError(OperationUse, ErrContextRequired, ErrContextRequired)
	}
	if callback == nil {
		return lifecycleError(OperationUse, ErrUseRejected, ErrUseRejected)
	}
	if err := ctx.Err(); err != nil {
		return lifecycleError(OperationUse, ErrUseRejected, err)
	}
	if s == nil || s.resource == nil {
		return lifecycleError(OperationUse, ErrUseRejected, ErrUseRejected)
	}
	client, release, err := s.resource.beginActive(OperationUse)
	if err != nil {
		return err
	}
	leaseContext, cancelLease := context.WithCancelCause(ctx)
	borrowed := &lease{
		ctx:          leaseContext,
		cancel:       cancelLease,
		client:       client,
		resourceName: s.resource.name,
		scopeName:    s.name,
		prefix:       s.prefix,
		cluster:      s.cluster,
		capability:   s.capability,
	}
	borrowed.active = true
	cleaned := false
	defer func() {
		if !cleaned {
			borrowed.expireCancelAndWait()
		}
		release()
	}()
	callbackErr := withContextError(ctx, callback(borrowed))
	detached := borrowed.expireCancelAndWait()
	cleaned = true
	if detached {
		callbackErr = errors.Join(callbackErr, ErrDetachedCommand)
	}
	if callbackErr != nil {
		return lifecycleError(OperationUse, ErrUseRejected, callbackErr)
	}
	if err := ctx.Err(); err != nil {
		return lifecycleError(OperationUse, ErrUseRejected, err)
	}
	return nil
}

func (l *lease) QualifyKey(logical string) (Key, error) {
	if l == nil {
		return Key{}, lifecycleError(OperationQualify, ErrLeaseExpired, ErrLeaseExpired)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.active {
		return Key{}, lifecycleError(OperationQualify, ErrLeaseExpired, ErrLeaseExpired)
	}
	if !validLogicalKey(logical) {
		return Key{}, unsafeKey("must be a canonical relative key")
	}
	return Key{
		value:        l.prefix + ":" + logical,
		resourceName: l.resourceName,
		scopeName:    l.scopeName,
		owner:        l.capability,
	}, nil
}

func (l *lease) Get(ctx context.Context, key Key) ([]byte, error) {
	commandContext, done, err := l.beginCommand(ctx, []Key{key}, OperationGet)
	if err != nil {
		return nil, err
	}
	defer done()
	value, found, err := l.client.Get(commandContext, key.value)
	err = withContextError(commandContext, err)
	if err != nil {
		return nil, lifecycleError(OperationGet, ErrCommandFailed, err)
	}
	if !found {
		return nil, ErrNotFound
	}
	return append([]byte(nil), value...), nil
}

func (l *lease) Set(ctx context.Context, key Key, value []byte, expiration time.Duration) error {
	commandContext, done, err := l.beginCommand(ctx, []Key{key}, OperationSet)
	if err != nil {
		return err
	}
	defer done()
	if expiration < 0 {
		return invalidCommand("expiration", "must not be negative")
	}
	if err := withContextError(commandContext, l.client.Set(commandContext, key.value, append([]byte(nil), value...), expiration)); err != nil {
		return lifecycleError(OperationSet, ErrCommandFailed, err)
	}
	return nil
}

func (l *lease) Delete(ctx context.Context, keys ...Key) (int64, error) {
	commandContext, done, err := l.beginCommand(ctx, keys, OperationDelete)
	if err != nil {
		return 0, err
	}
	defer done()
	return l.multiKey(commandContext, keys, OperationDelete, l.client.Delete)
}

func (l *lease) Exists(ctx context.Context, keys ...Key) (int64, error) {
	commandContext, done, err := l.beginCommand(ctx, keys, OperationExists)
	if err != nil {
		return 0, err
	}
	defer done()
	return l.multiKey(commandContext, keys, OperationExists, l.client.Exists)
}

// multiKey keeps standalone/Sentinel batching but makes cluster use portable:
// each qualified key is a separate command, so no user-controlled hash tag or
// CROSSSLOT-prone variadic request reaches ClusterClient. Cluster execution is
// deliberately non-atomic and fail-fast; count reports completed work before
// the first redacted error. Cross-key atomic capabilities are a later slice.
func (l *lease) multiKey(
	ctx context.Context,
	keys []Key,
	operation Operation,
	command func(context.Context, ...string) (int64, error),
) (int64, error) {
	qualified := physicalKeys(keys)
	if !l.cluster {
		count, err := command(ctx, qualified...)
		err = withContextError(ctx, err)
		if err != nil {
			return count, lifecycleError(operation, ErrCommandFailed, err)
		}
		return count, nil
	}
	var total int64
	for _, key := range qualified {
		count, err := command(ctx, key)
		total += count
		err = withContextError(ctx, err)
		if err != nil {
			return total, lifecycleError(operation, ErrCommandFailed, err)
		}
	}
	return total, nil
}

func (l *lease) beginCommand(ctx context.Context, keys []Key, operation Operation) (context.Context, func(), error) {
	if l == nil {
		return nil, nil, lifecycleError(operation, ErrLeaseExpired, ErrLeaseExpired)
	}
	if ctx == nil {
		return nil, nil, lifecycleError(operation, ErrContextRequired, ErrContextRequired)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, lifecycleError(operation, ErrUnavailable, err)
	}
	if len(keys) == 0 {
		return nil, nil, unsafeKey("at least one key is required")
	}
	for _, key := range keys {
		if key.owner == nil || key.owner != l.capability || key.resourceName != l.resourceName || key.scopeName != l.scopeName || key.value == "" {
			return nil, nil, unsafeKey("does not belong to this scope capability")
		}
	}

	l.mu.Lock()
	if !l.active {
		l.mu.Unlock()
		return nil, nil, lifecycleError(operation, ErrLeaseExpired, ErrLeaseExpired)
	}
	if l.inFlight == 0 {
		l.inFlight0 = make(chan struct{})
	}
	l.inFlight++
	l.mu.Unlock()
	commandParent := l.ctx
	cancelDeadline := func() {}
	if deadline, ok := ctx.Deadline(); ok {
		commandParent, cancelDeadline = context.WithDeadlineCause(l.ctx, deadline, context.DeadlineExceeded)
	}
	commandContext, cancelCommand := context.WithCancelCause(commandParent)
	stopCallerCancellation := context.AfterFunc(ctx, func() {
		cancelCommand(context.Cause(ctx))
	})

	var once sync.Once
	return commandContext, func() {
		once.Do(func() {
			stopCallerCancellation()
			cancelCommand(context.Canceled)
			cancelDeadline()
			l.mu.Lock()
			l.inFlight--
			if l.inFlight == 0 {
				close(l.inFlight0)
			}
			l.mu.Unlock()
		})
	}, nil
}

func (l *lease) expireCancelAndWait() bool {
	l.mu.Lock()
	l.active = false
	detached := l.inFlight > 0
	l.cancel(ErrLeaseExpired)
	if l.inFlight == 0 {
		l.mu.Unlock()
		return detached
	}
	done := l.inFlight0
	l.mu.Unlock()
	<-done
	return detached
}

func physicalKeys(keys []Key) []string {
	qualified := make([]string, len(keys))
	for index, key := range keys {
		qualified[index] = key.value
	}
	return qualified
}

func withContextError(ctx context.Context, err error) error {
	contextErr := contextClassification(ctx)
	if contextErr == nil || errors.Is(err, contextErr) {
		return err
	}
	return errors.Join(err, contextErr)
}

var _ Lease = (*lease)(nil)
