package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/redisresource"
	runtimeresource "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/resource"
)

const (
	defaultPollInterval   = time.Second
	defaultPollTimeout    = time.Second
	defaultMaxPayloadSize = 1 << 20
	maxPayloadSize        = 8 << 20
	latestLogicalKey      = "latest"
)

// RedisOptions configures bounded latest-revision polling. Reconciler is
// required because Redis notifications are disposable and non-authoritative.
type RedisOptions[T any] struct {
	Codec           Codec[T]
	Reconciler      *Reconciler[T]
	PollInterval    time.Duration
	PollTimeout     time.Duration
	MaxPayloadBytes int
}

// Redis stores a single latest event inside a caller-owned redisresource
// Scope. It never closes that Scope or its Resource.
type Redis[T any] struct {
	core       *core[T]
	scope      *redisresource.Scope
	codec      Codec[T]
	reconciler *Reconciler[T]

	pollInterval time.Duration
	pollTimeout  time.Duration
	maxPayload   int

	publishMu    sync.Mutex
	healthMu     sync.Mutex
	pollErr      error
	reconcileErr error
}

type wireEvent struct {
	Revision Revision `json:"revision"`
	Payload  []byte   `json:"payload"`
}

// BuildRedis validates and copies options without borrowing the Scope,
// performing Redis I/O, calling the reconciler, or starting a goroutine.
func BuildRedis[T any](scope *redisresource.Scope, options RedisOptions[T]) (*Redis[T], error) {
	if scope == nil {
		return nil, invalid("scope", "is required", ErrInvalidOptions)
	}
	if options.Reconciler == nil {
		return nil, invalid("reconciler", "is required", ErrInvalidReconciler)
	}
	codec := options.Codec
	if codec == nil {
		codec = JSONCodec[T]{}
	}
	pollInterval := options.PollInterval
	if pollInterval == 0 {
		pollInterval = defaultPollInterval
	}
	if pollInterval < time.Millisecond {
		return nil, invalid("pollInterval", "must be at least one millisecond", ErrInvalidOptions)
	}
	pollTimeout := options.PollTimeout
	if pollTimeout == 0 {
		pollTimeout = defaultPollTimeout
	}
	if pollTimeout <= 0 {
		return nil, invalid("pollTimeout", "must be positive", ErrInvalidOptions)
	}
	maxPayload := options.MaxPayloadBytes
	if maxPayload == 0 {
		maxPayload = defaultMaxPayloadSize
	}
	if maxPayload < 1 || maxPayload > maxPayloadSize {
		return nil, invalid("maxPayloadBytes", "is outside the supported range", ErrInvalidOptions)
	}
	return &Redis[T]{
		core:         newCore[T](),
		scope:        scope,
		codec:        codec,
		reconciler:   options.Reconciler,
		pollInterval: pollInterval,
		pollTimeout:  pollTimeout,
		maxPayload:   maxPayload,
	}, nil
}

func (b *Redis[T]) Start(ctx context.Context) error {
	if b == nil {
		return operationError(OperationStart, ErrInvalidBus, ErrInvalidBus)
	}
	err := b.core.start(ctx, func(startCtx context.Context) error {
		bounded, cancel := b.bounded(startCtx)
		defer cancel()
		return b.scope.Ready(bounded)
	})
	if err != nil {
		b.setPollError(err)
		return err
	}
	b.setPollError(nil)
	return nil
}

func (b *Redis[T]) Subscribe(handler Handler[T]) (*Subscription, error) {
	if b == nil {
		return nil, operationError(OperationSubscribe, ErrInvalidBus, ErrInvalidBus)
	}
	return b.core.subscribe(handler)
}

// Publish replaces Redis's disposable latest event only when its observed
// revision is newer, then fans the selected event out locally. Cross-replica
// races may still overwrite or miss a notification; authoritative
// reconciliation is the convergence guarantee.
func (b *Redis[T]) Publish(ctx context.Context, event Event[T]) error {
	if b == nil {
		return operationError(OperationPublish, ErrInvalidBus, ErrInvalidBus)
	}
	if event.Revision == 0 {
		return operationError(OperationPublish, ErrInvalidEvent, ErrInvalidEvent)
	}
	release, err := b.core.beginActive(ctx, OperationPublish)
	if err != nil {
		return err
	}
	defer release()

	b.publishMu.Lock()
	selected, providerErr := b.storeLatest(ctx, event)
	b.publishMu.Unlock()
	b.setPollError(providerErr)

	deliveryErr := b.core.deliverActive(ctx, selected, deliveryNotification)
	var reconcileErr error
	if providerErr != nil && ctx.Err() == nil {
		reconcileErr = b.reconcileActive(ctx)
	}
	result := errors.Join(providerErr, deliveryErr, reconcileErr)
	if result != nil {
		return operationError(OperationPublish, ErrPublishFailed, result)
	}
	return nil
}

// PollOnce performs one bounded Redis latest read followed by one bounded
// authoritative reconciliation. Provider failure never prevents the
// authoritative source from repairing this replica.
func (b *Redis[T]) PollOnce(ctx context.Context) error {
	if b == nil {
		return operationError(OperationPoll, ErrInvalidBus, ErrInvalidBus)
	}
	release, err := b.core.beginActive(ctx, OperationPoll)
	if err != nil {
		return err
	}
	defer release()
	return b.pollActive(ctx)
}

func (b *Redis[T]) pollActive(ctx context.Context) error {
	event, found, providerErr := b.loadLatest(ctx)
	b.setPollError(providerErr)
	var deliveryErr error
	if providerErr == nil && found {
		deliveryErr = b.core.deliverActive(ctx, event, deliveryNotification)
	}
	reconcileErr := b.reconcileActive(ctx)
	result := errors.Join(providerErr, deliveryErr, reconcileErr)
	if result != nil {
		return operationError(OperationPoll, ErrPollFailed, result)
	}
	return nil
}

func (b *Redis[T]) reconcileActive(ctx context.Context) error {
	bounded, cancel := b.bounded(ctx)
	defer cancel()
	event, found, err := b.reconciler.Reconcile(bounded, b.core.last())
	if err == nil && found {
		err = b.core.deliverActive(bounded, event, deliveryReconciliation)
	}
	b.setReconcileError(err)
	return err
}

func (b *Redis[T]) Reconcile(ctx context.Context) error {
	if b == nil {
		return operationError(OperationReconcile, ErrInvalidBus, ErrInvalidBus)
	}
	release, err := b.core.beginActive(ctx, OperationReconcile)
	if err != nil {
		return err
	}
	defer release()
	return b.reconcileActive(ctx)
}

// Run owns bounded polling and blocks until its caller or Close cancels it.
// Poll, decode, subscriber, and reconciliation failures degrade health but do
// not terminate the loop.
func (b *Redis[T]) Run(ctx context.Context) error {
	if b == nil {
		return operationError(OperationRun, ErrInvalidBus, ErrInvalidBus)
	}
	runCtx, cancel, err := b.core.beginRun(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cancel()
		b.core.finishRun()
	}()

	_ = b.PollOnce(runCtx)
	ticker := time.NewTicker(b.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-runCtx.Done():
			return operationError(OperationRun, ErrRunRejected, runCtx.Err())
		case <-ticker.C:
			_ = b.PollOnce(runCtx)
		}
	}
}

func (b *Redis[T]) LastRevision() Revision {
	if b == nil {
		return 0
	}
	return b.core.last()
}

func (b *Redis[T]) Ready(ctx context.Context) error { return b.inspect(ctx, OperationReady) }

func (b *Redis[T]) Health(ctx context.Context) error { return b.inspect(ctx, OperationHealth) }

func (b *Redis[T]) inspect(ctx context.Context, operation Operation) error {
	if b == nil {
		return operationError(operation, ErrInvalidBus, ErrInvalidBus)
	}
	release, err := b.core.beginActive(ctx, operation)
	if err != nil {
		return err
	}
	defer release()
	bounded, cancel := b.bounded(ctx)
	scopeErr := b.scope.Ready(bounded)
	cancel()
	if scopeErr != nil {
		b.setPollError(operationError(operation, ErrProviderFailed, scopeErr))
	}
	b.healthMu.Lock()
	pollErr := b.pollErr
	reconcileErr := b.reconcileErr
	b.healthMu.Unlock()
	b.core.mu.Lock()
	deliveryErr := b.core.deliveryErr
	b.core.mu.Unlock()
	degraded := errors.Join(pollErr, reconcileErr, deliveryErr)
	if degraded != nil {
		return operationError(operation, ErrDegraded, degraded)
	}
	return nil
}

func (b *Redis[T]) Close(ctx context.Context) error {
	if b == nil {
		return nil
	}
	return b.core.close(ctx)
}

func (b *Redis[T]) storeLatest(ctx context.Context, proposed Event[T]) (Event[T], error) {
	payload, err := b.codec.Marshal(proposed.Payload)
	if err != nil {
		return proposed, operationError(OperationEncode, ErrEncodeFailed, err)
	}
	if len(payload) > b.maxPayload {
		return proposed, operationError(OperationEncode, ErrEncodeFailed, ErrInvalidEvent)
	}
	encoded, err := json.Marshal(wireEvent{Revision: proposed.Revision, Payload: payload})
	if err != nil {
		return proposed, operationError(OperationEncode, ErrEncodeFailed, err)
	}

	bounded, cancel := b.bounded(ctx)
	defer cancel()
	var current []byte
	found := false
	err = b.scope.Use(bounded, func(lease redisresource.Lease) error {
		key, keyErr := lease.QualifyKey(latestLogicalKey)
		if keyErr != nil {
			return keyErr
		}
		var getErr error
		current, getErr = lease.Get(bounded, key)
		if errors.Is(getErr, redisresource.ErrNotFound) {
			return nil
		}
		found = getErr == nil
		return getErr
	})
	if err != nil {
		return proposed, operationError(OperationPublish, ErrProviderFailed, err)
	}
	if found {
		existing, decodeErr := b.decode(current)
		if decodeErr != nil {
			return proposed, decodeErr
		}
		if existing.Revision >= proposed.Revision {
			return existing, nil
		}
	}
	err = b.scope.Use(bounded, func(lease redisresource.Lease) error {
		key, keyErr := lease.QualifyKey(latestLogicalKey)
		if keyErr != nil {
			return keyErr
		}
		return lease.Set(bounded, key, encoded, 0)
	})
	if err != nil {
		return proposed, operationError(OperationPublish, ErrProviderFailed, err)
	}
	return proposed, nil
}

func (b *Redis[T]) loadLatest(ctx context.Context) (Event[T], bool, error) {
	bounded, cancel := b.bounded(ctx)
	defer cancel()
	var encoded []byte
	err := b.scope.Use(bounded, func(lease redisresource.Lease) error {
		key, keyErr := lease.QualifyKey(latestLogicalKey)
		if keyErr != nil {
			return keyErr
		}
		var getErr error
		encoded, getErr = lease.Get(bounded, key)
		return getErr
	})
	if errors.Is(err, redisresource.ErrNotFound) {
		return Event[T]{}, false, nil
	}
	if err != nil {
		return Event[T]{}, false, operationError(OperationPoll, ErrProviderFailed, err)
	}
	event, err := b.decode(encoded)
	if err != nil {
		return Event[T]{}, false, err
	}
	return event, true, nil
}

func (b *Redis[T]) decode(encoded []byte) (Event[T], error) {
	if len(encoded) > b.maxPayload*2+256 {
		return Event[T]{}, operationError(OperationDecode, ErrDecodeFailed, ErrInvalidEvent)
	}
	var wire wireEvent
	if err := json.Unmarshal(encoded, &wire); err != nil || wire.Revision == 0 || len(wire.Payload) > b.maxPayload {
		return Event[T]{}, operationError(OperationDecode, ErrDecodeFailed, err)
	}
	payload, err := b.codec.Unmarshal(wire.Payload)
	if err != nil {
		return Event[T]{}, operationError(OperationDecode, ErrDecodeFailed, err)
	}
	return Event[T]{Revision: wire.Revision, Payload: payload}, nil
}

func (b *Redis[T]) bounded(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, b.pollTimeout)
}

func (b *Redis[T]) setPollError(err error) {
	b.healthMu.Lock()
	defer b.healthMu.Unlock()
	if err == nil {
		b.pollErr = nil
		return
	}
	b.pollErr = safeClassifications(errors.Join(err, ErrPollFailed))
}

func (b *Redis[T]) setReconcileError(err error) {
	b.healthMu.Lock()
	defer b.healthMu.Unlock()
	if err == nil {
		b.reconcileErr = nil
		return
	}
	b.reconcileErr = safeClassifications(err)
}

var (
	_ EventBus[struct{}]               = (*Redis[struct{}])(nil)
	_ runtimeresource.Resource         = (*Redis[struct{}])(nil)
	_ runtimeresource.Runner           = (*Redis[struct{}])(nil)
	_ runtimeresource.HealthChecker    = (*Redis[struct{}])(nil)
	_ runtimeresource.ReadinessChecker = (*Redis[struct{}])(nil)
)
