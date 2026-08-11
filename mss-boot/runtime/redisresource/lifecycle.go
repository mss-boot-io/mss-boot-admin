package redisresource

import (
	"context"
	"errors"
	"sync"
)

type lifecycleState struct {
	mu sync.Mutex

	startAttempted  bool
	startInProgress bool
	startDone       chan struct{}
	started         bool
	closing         bool
	closed          bool

	client ownedClient

	active     int
	activeZero chan struct{}

	closeInProgress bool
	closeDone       chan struct{}
	closeErr        error
	clientCloseMade bool
}

func newLifecycleState() lifecycleState {
	zero := make(chan struct{})
	close(zero)
	return lifecycleState{activeZero: zero}
}

// Start creates exactly one owned client and verifies it with PING. A failed
// construction or PING permanently ends this resource instance. Cleanup uses
// the same tracked close generation as Close, so a canceled Start can return
// while the single provider Close continues and remains joinable.
func (r *Resource) Start(ctx context.Context) error {
	if r == nil {
		return lifecycleError(OperationStart, ErrStartRejected, ErrStartRejected)
	}
	if ctx == nil {
		return lifecycleError(OperationStart, ErrContextRequired, ErrContextRequired)
	}
	if err := ctx.Err(); err != nil {
		return lifecycleError(OperationStart, ErrUnavailable, err)
	}

	s := &r.state
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return lifecycleError(OperationStart, ErrClosing, ErrClosing)
	}
	if s.startAttempted {
		s.mu.Unlock()
		return lifecycleError(OperationStart, ErrStartRejected, ErrStartRejected)
	}
	s.startAttempted = true
	s.startInProgress = true
	s.startDone = make(chan struct{})
	s.mu.Unlock()

	client, constructErr := r.factory.New(r.spec.clone())
	if constructErr != nil || client == nil {
		if constructErr == nil {
			constructErr = ErrClientConstruction
		}
		_ = r.finishFailedStart(ctx, nil)
		return lifecycleError(OperationStart, ErrClientConstruction, constructErr)
	}

	s.mu.Lock()
	s.client = client
	closing := s.closing
	s.mu.Unlock()
	if closing {
		closeErr := r.finishFailedStart(ctx, client)
		return lifecycleError(OperationStart, ErrClosing, errors.Join(ErrClosing, closeErr))
	}

	pingErr := client.Ping(ctx)
	if contextErr := contextClassification(ctx); contextErr != nil && !errors.Is(pingErr, contextErr) {
		pingErr = errors.Join(pingErr, contextErr)
	}
	if pingErr != nil {
		closeErr := r.finishFailedStart(ctx, client)
		return lifecycleError(OperationStart, ErrUnavailable, errors.Join(pingErr, closeErr))
	}

	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		closeErr := r.finishFailedStart(ctx, client)
		return lifecycleError(OperationStart, ErrClosing, errors.Join(ErrClosing, closeErr))
	}
	s.started = true
	s.startInProgress = false
	close(s.startDone)
	s.mu.Unlock()
	return nil
}

// finishFailedStart creates (or joins) the sole close generation while Start
// is in progress. Provider Close itself has no context, so it runs in the
// tracked generation while this caller waits only through its own context.
func (r *Resource) finishFailedStart(ctx context.Context, client ownedClient) error {
	s := &r.state
	s.mu.Lock()
	s.closing = true
	s.started = false
	s.startInProgress = false
	if client == nil {
		s.closed = true
	}
	done := r.ensureCloseGenerationLocked(client)
	close(s.startDone)
	s.mu.Unlock()
	if done == nil {
		return nil
	}
	if err := waitContext(ctx, done); err != nil {
		return lifecycleError(OperationClose, ErrCloseFailed, err)
	}
	s.mu.Lock()
	result := s.closeErr
	s.mu.Unlock()
	return closeResult(result)
}

// Ready verifies that the started client can currently serve dependents.
func (r *Resource) Ready(ctx context.Context) error {
	return r.ping(ctx, OperationReady)
}

// Health verifies current provider health without changing ownership.
func (r *Resource) Health(ctx context.Context) error {
	return r.ping(ctx, OperationHealth)
}

func (r *Resource) ping(ctx context.Context, operation Operation) error {
	if ctx == nil {
		return lifecycleError(operation, ErrContextRequired, ErrContextRequired)
	}
	if err := ctx.Err(); err != nil {
		return lifecycleError(operation, ErrUnavailable, err)
	}
	client, release, err := r.beginActive(operation)
	if err != nil {
		return err
	}
	defer release()
	pingErr := client.Ping(ctx)
	if contextErr := contextClassification(ctx); contextErr != nil && !errors.Is(pingErr, contextErr) {
		pingErr = errors.Join(pingErr, contextErr)
	}
	if pingErr != nil {
		return lifecycleError(operation, ErrUnavailable, pingErr)
	}
	if err := ctx.Err(); err != nil {
		return lifecycleError(operation, ErrUnavailable, err)
	}
	return nil
}

func (r *Resource) beginActive(operation Operation) (ownedClient, func(), error) {
	if r == nil {
		return nil, nil, lifecycleError(operation, ErrNotStarted, ErrNotStarted)
	}
	s := &r.state
	s.mu.Lock()
	if s.closing || s.closed {
		s.mu.Unlock()
		return nil, nil, lifecycleError(operation, ErrClosing, ErrClosing)
	}
	if !s.started || s.client == nil {
		s.mu.Unlock()
		return nil, nil, lifecycleError(operation, ErrNotStarted, ErrNotStarted)
	}
	if s.active == 0 {
		s.activeZero = make(chan struct{})
	}
	s.active++
	client := s.client
	s.mu.Unlock()

	var once sync.Once
	release := func() {
		once.Do(func() {
			s.mu.Lock()
			s.active--
			if s.active == 0 {
				close(s.activeZero)
			}
			s.mu.Unlock()
		})
	}
	return client, release, nil
}

// Close irreversibly rejects new Ready, Health, and Use calls, waits for all
// active callbacks/inspections, and calls the owned client's Close exactly
// once. If a caller deadline expires while draining, a later Close continues
// the same shutdown; the resource never reopens.
func (r *Resource) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return lifecycleError(OperationClose, ErrContextRequired, ErrContextRequired)
	}
	s := &r.state

	s.mu.Lock()
	s.closing = true
	if s.closed {
		result := s.closeErr
		s.mu.Unlock()
		return closeResult(result)
	}
	if !s.startAttempted {
		s.closed = true
		s.mu.Unlock()
		return nil
	}
	startDone := s.startDone
	s.mu.Unlock()

	if err := waitContext(ctx, startDone); err != nil {
		return lifecycleError(OperationClose, ErrCloseFailed, err)
	}

	for {
		s.mu.Lock()
		if s.closed {
			result := s.closeErr
			s.mu.Unlock()
			return closeResult(result)
		}
		zero := s.activeZero
		if s.active > 0 {
			s.mu.Unlock()
			if err := waitContext(ctx, zero); err != nil {
				return lifecycleError(OperationClose, ErrCloseFailed, err)
			}
			continue
		}
		done := r.ensureCloseGenerationLocked(s.client)
		if done == nil {
			result := s.closeErr
			s.mu.Unlock()
			return closeResult(result)
		}
		s.mu.Unlock()
		if err := waitContext(ctx, done); err != nil {
			return lifecycleError(OperationClose, ErrCloseFailed, err)
		}
		s.mu.Lock()
		result := s.closeErr
		s.mu.Unlock()
		return closeResult(result)
	}
}

// ensureCloseGenerationLocked starts at most one provider Close. It must be
// called with state.mu held. Every caller observes the same done channel and
// terminal error; a caller deadline never causes another provider Close.
func (r *Resource) ensureCloseGenerationLocked(client ownedClient) <-chan struct{} {
	s := &r.state
	if s.closed {
		return nil
	}
	if s.closeInProgress || s.clientCloseMade {
		return s.closeDone
	}
	if client == nil {
		s.closed = true
		s.started = false
		return nil
	}
	s.closeInProgress = true
	s.clientCloseMade = true
	s.closeDone = make(chan struct{})
	done := s.closeDone
	go func() {
		closeErr := client.Close()
		s.mu.Lock()
		s.closeErr = closeErr
		s.closeInProgress = false
		s.closed = true
		s.started = false
		close(done)
		s.mu.Unlock()
	}()
	return done
}

func closeResult(err error) error {
	if err == nil {
		return nil
	}
	return lifecycleError(OperationClose, ErrCloseFailed, err)
}

func waitContext(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
