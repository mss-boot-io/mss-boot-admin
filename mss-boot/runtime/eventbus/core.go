package eventbus

import (
	"context"
	"errors"
	"sort"
	"sync"
)

type subscriber[T any] struct {
	id       uint64
	handler  Handler[T]
	revision Revision
}

type deliveryMode uint8

const (
	deliveryNotification deliveryMode = iota
	deliveryReconciliation
)

type core[T any] struct {
	mu sync.Mutex

	startAttempted bool
	started        bool
	startDone      chan struct{}
	startCancel    context.CancelFunc

	runAttempted bool
	runDone      chan struct{}
	runCancel    context.CancelFunc

	closing bool
	closed  bool

	active     int
	activeZero chan struct{}

	nextSubscriber uint64
	subscribers    map[uint64]*subscriber[T]
	lastRevision   Revision
	deliveryErr    error

	// deliveryToken serializes revision acceptance and callback invocation so
	// one subscriber never observes accepted revisions out of order.
	deliveryToken chan struct{}
}

func newCore[T any]() *core[T] {
	startDone := make(chan struct{})
	close(startDone)
	runDone := make(chan struct{})
	close(runDone)
	activeZero := make(chan struct{})
	close(activeZero)
	token := make(chan struct{}, 1)
	token <- struct{}{}
	return &core[T]{
		startDone:     startDone,
		runDone:       runDone,
		activeZero:    activeZero,
		subscribers:   make(map[uint64]*subscriber[T]),
		deliveryToken: token,
	}
}

func (c *core[T]) start(ctx context.Context, probe func(context.Context) error) error {
	if c == nil {
		return operationError(OperationStart, ErrInvalidBus, ErrInvalidBus)
	}
	if ctx == nil {
		return operationError(OperationStart, ErrContextRequired, ErrContextRequired)
	}
	if err := ctx.Err(); err != nil {
		return operationError(OperationStart, ErrStartRejected, err)
	}

	c.mu.Lock()
	if c.closing || c.closed {
		c.mu.Unlock()
		return operationError(OperationStart, ErrClosing, ErrClosing)
	}
	if c.startAttempted {
		c.mu.Unlock()
		return operationError(OperationStart, ErrStartRejected, ErrStartRejected)
	}
	c.startAttempted = true
	c.startDone = make(chan struct{})
	startCtx, cancel := context.WithCancel(ctx)
	c.startCancel = cancel
	c.mu.Unlock()

	var probeErr error
	if probe != nil {
		probeErr = probe(startCtx)
	}
	cancel()

	c.mu.Lock()
	c.startCancel = nil
	closing := c.closing || c.closed
	if probeErr == nil && !closing {
		c.started = true
	}
	close(c.startDone)
	c.mu.Unlock()

	if closing {
		return operationError(OperationStart, ErrClosing, errors.Join(probeErr, ErrClosing))
	}
	if probeErr != nil {
		return operationError(OperationStart, ErrProviderFailed, probeErr)
	}
	return nil
}

func (c *core[T]) beginRun(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if c == nil {
		return nil, nil, operationError(OperationRun, ErrInvalidBus, ErrInvalidBus)
	}
	if ctx == nil {
		return nil, nil, operationError(OperationRun, ErrContextRequired, ErrContextRequired)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, operationError(OperationRun, ErrRunRejected, err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || c.closed {
		return nil, nil, operationError(OperationRun, ErrClosing, ErrClosing)
	}
	if !c.started {
		return nil, nil, operationError(OperationRun, ErrNotStarted, ErrNotStarted)
	}
	if c.runAttempted {
		return nil, nil, operationError(OperationRun, ErrRunRejected, ErrRunRejected)
	}
	c.runAttempted = true
	c.runDone = make(chan struct{})
	runCtx, cancel := context.WithCancel(ctx)
	c.runCancel = cancel
	return runCtx, cancel, nil
}

func (c *core[T]) finishRun() {
	c.mu.Lock()
	c.runCancel = nil
	close(c.runDone)
	c.mu.Unlock()
}

func (c *core[T]) beginActive(ctx context.Context, operation Operation) (func(), error) {
	if c == nil {
		return nil, operationError(operation, ErrInvalidBus, ErrInvalidBus)
	}
	if ctx == nil {
		return nil, operationError(operation, ErrContextRequired, ErrContextRequired)
	}
	if err := ctx.Err(); err != nil {
		return nil, operationError(operation, err, err)
	}
	c.mu.Lock()
	if c.closing || c.closed {
		c.mu.Unlock()
		return nil, operationError(operation, ErrClosing, ErrClosing)
	}
	if !c.started {
		c.mu.Unlock()
		return nil, operationError(operation, ErrNotStarted, ErrNotStarted)
	}
	if c.active == 0 {
		c.activeZero = make(chan struct{})
	}
	c.active++
	c.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			c.active--
			if c.active == 0 {
				close(c.activeZero)
			}
			c.mu.Unlock()
		})
	}, nil
}

func (c *core[T]) subscribe(handler Handler[T]) (*Subscription, error) {
	if c == nil {
		return nil, operationError(OperationSubscribe, ErrInvalidBus, ErrInvalidBus)
	}
	if handler == nil {
		return nil, invalid("handler", "is required", ErrInvalidOptions)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || c.closed {
		return nil, operationError(OperationSubscribe, ErrClosing, ErrClosing)
	}
	c.nextSubscriber++
	id := c.nextSubscriber
	c.subscribers[id] = &subscriber[T]{id: id, handler: handler}
	return &Subscription{cancel: func() {
		c.mu.Lock()
		delete(c.subscribers, id)
		c.mu.Unlock()
	}}, nil
}

func (c *core[T]) deliver(ctx context.Context, event Event[T], mode deliveryMode) error {
	release, err := c.beginActive(ctx, OperationPublish)
	if err != nil {
		return err
	}
	defer release()
	return c.deliverActive(ctx, event, mode)
}

func (c *core[T]) deliverActive(ctx context.Context, event Event[T], mode deliveryMode) error {
	if event.Revision == 0 {
		return operationError(OperationPublish, ErrInvalidEvent, ErrInvalidEvent)
	}
	select {
	case <-ctx.Done():
		return operationError(OperationPublish, ErrPublishFailed, ctx.Err())
	case <-c.deliveryToken:
	}
	defer func() { c.deliveryToken <- struct{}{} }()

	c.mu.Lock()
	if event.Revision < c.lastRevision || event.Revision == c.lastRevision && mode == deliveryNotification {
		c.mu.Unlock()
		return nil
	}
	if event.Revision > c.lastRevision {
		c.lastRevision = event.Revision
	}
	ids := make([]uint64, 0, len(c.subscribers))
	for id, current := range c.subscribers {
		if current.revision < event.Revision {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	current := make([]*subscriber[T], 0, len(ids))
	for _, id := range ids {
		current = append(current, c.subscribers[id])
	}
	c.mu.Unlock()

	var result error
	for _, target := range current {
		if err := ctx.Err(); err != nil {
			result = errors.Join(result, operationError(OperationPublish, ErrPublishFailed, err))
			break
		}
		handlerErr := callHandler(ctx, target.handler, event)
		if handlerErr != nil {
			result = errors.Join(result, handlerErr)
			continue
		}
		c.mu.Lock()
		if registered := c.subscribers[target.id]; registered == target && registered.revision < event.Revision {
			registered.revision = event.Revision
		}
		c.mu.Unlock()
	}

	c.mu.Lock()
	if errors.Is(result, ErrSubscriberFailed) || errors.Is(result, ErrSubscriberPanicked) {
		c.deliveryErr = safeClassifications(result)
	} else if mode == deliveryReconciliation || len(current) > 0 {
		c.deliveryErr = nil
	}
	c.mu.Unlock()
	return result
}

func callHandler[T any](ctx context.Context, handler Handler[T], event Event[T]) (result error) {
	panicked := true
	defer func() {
		if recover() != nil || panicked {
			result = operationError(OperationPublish, ErrSubscriberPanicked, ErrSubscriberPanicked)
		}
	}()
	err := handler(ctx, event)
	panicked = false
	if err != nil {
		return operationError(OperationPublish, ErrSubscriberFailed, err)
	}
	return nil
}

func (c *core[T]) last() Revision {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastRevision
}

func (c *core[T]) inspection(ctx context.Context, operation Operation) error {
	release, err := c.beginActive(ctx, operation)
	if err != nil {
		return err
	}
	defer release()
	c.mu.Lock()
	degraded := c.deliveryErr
	c.mu.Unlock()
	if degraded != nil {
		return operationError(operation, ErrDegraded, degraded)
	}
	return nil
}

func (c *core[T]) close(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if ctx == nil {
		return operationError(OperationClose, ErrContextRequired, ErrContextRequired)
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closing = true
	startCancel := c.startCancel
	runCancel := c.runCancel
	startDone := c.startDone
	runDone := c.runDone
	c.mu.Unlock()

	if startCancel != nil {
		startCancel()
	}
	if runCancel != nil {
		runCancel()
	}
	for _, done := range []<-chan struct{}{startDone, runDone} {
		select {
		case <-done:
		case <-ctx.Done():
			return operationError(OperationClose, ErrClosing, ctx.Err())
		}
	}
	for {
		c.mu.Lock()
		if c.active == 0 {
			c.closed = true
			c.started = false
			c.subscribers = make(map[uint64]*subscriber[T])
			c.mu.Unlock()
			return nil
		}
		zero := c.activeZero
		c.mu.Unlock()
		select {
		case <-zero:
		case <-ctx.Done():
			return operationError(OperationClose, ErrClosing, ctx.Err())
		}
	}
}

// Subscription removes one subscriber. Cancellation is idempotent and does
// not retract a delivery whose current-subscriber snapshot was already taken.
type Subscription struct {
	once   sync.Once
	cancel func()
}

func (s *Subscription) Cancel() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
	})
}
