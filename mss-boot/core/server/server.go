package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"
)

var (
	// ErrAlreadyStarted is returned when a manager is started more than once.
	ErrAlreadyStarted = errors.New("server manager has already been started")
	// ErrRunnableStopped is returned when a runnable exits successfully before
	// the manager has begun its shutdown sequence.
	ErrRunnableStopped = errors.New("runnable stopped before shutdown")
	// ErrGracefulShutdownTimeout is returned when one or more runnables fail to
	// stop within the configured grace period.
	ErrGracefulShutdownTimeout = errors.New("graceful shutdown timed out")
)

type runnableResult struct {
	name string
	err  error
}

// Server coordinates Runnable components as one lifecycle unit.
type Server struct {
	mux      sync.Mutex
	services map[string]Runnable
	order    []string
	started  bool
	opts     Options
}

// New creates a server manager.
func New(opts ...Option) Manager {
	s := &Server{
		services: make(map[string]Runnable),
		opts:     setDefaultOptions(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&s.opts)
		}
	}
	return s
}

// Add registers runnables before Start is called. A runnable with the same name
// replaces the previous registration while preserving deterministic start
// order. Nil runnables and additions after Start are ignored for compatibility
// with the existing interface, which cannot return an error.
func (e *Server) Add(runnables ...Runnable) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if e.started {
		return
	}
	if e.services == nil {
		e.services = make(map[string]Runnable)
	}
	for _, runnable := range runnables {
		if runnable == nil {
			continue
		}
		name := runnable.String()
		if _, exists := e.services[name]; !exists {
			e.order = append(e.order, name)
		}
		e.services[name] = runnable
	}
}

// Start runs all registered components concurrently. The first unexpected
// component exit or runtime error cancels every peer. Cancellation from the
// caller or a configured operating-system signal is treated as a normal stop.
func (e *Server) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	runnables, options, err := e.beginStart()
	if err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if len(options.signals) > 0 {
		var stopSignals context.CancelFunc
		runCtx, stopSignals = signal.NotifyContext(runCtx, options.signals...)
		defer stopSignals()
	}

	if len(runnables) == 0 {
		<-runCtx.Done()
		return nil
	}

	results := make(chan runnableResult, len(runnables))
	var wait sync.WaitGroup
	wait.Add(len(runnables))
	for _, runnable := range runnables {
		runnable := runnable
		go func() {
			defer wait.Done()
			results <- runnableResult{name: runnable.String(), err: runnable.Start(runCtx)}
		}()
	}

	var runErr error
	received := 0
	select {
	case <-runCtx.Done():
		// Caller cancellation and process signals are normal shutdown paths.
	case result := <-results:
		received++
		if runCtx.Err() != nil {
			// A runnable may observe external cancellation and return at the same
			// instant as this select. Preserve real shutdown errors while treating
			// the matching context error as normal.
			runErr = errors.Join(runErr, shutdownResultError(result.name, result.err, runCtx.Err()))
		} else {
			runErr = errors.Join(runErr, runningResultError(result.name, result.err))
		}
	}

	cancel()
	shutdownErr := waitForRunnables(&wait, options.gracefulShutdownTimeout)
	runErr = errors.Join(runErr, collectShutdownResults(results, len(runnables), received, runCtx.Err(), shutdownErr == nil))
	return errors.Join(runErr, shutdownErr)
}

func (e *Server) beginStart() ([]Runnable, Options, error) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if e.started {
		return nil, Options{}, ErrAlreadyStarted
	}
	e.started = true

	runnables := make([]Runnable, 0, len(e.order))
	for _, name := range e.order {
		if runnable := e.services[name]; runnable != nil {
			runnables = append(runnables, runnable)
		}
	}
	options := e.opts
	options.signals = append([]os.Signal(nil), e.opts.signals...)
	return runnables, options, nil
}

func runningResultError(name string, err error) error {
	if err != nil {
		return fmt.Errorf("runnable %q failed: %w", name, err)
	}
	return fmt.Errorf("runnable %q: %w", name, ErrRunnableStopped)
}

func shutdownResultError(name string, err, cancellationErr error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || (cancellationErr != nil && errors.Is(err, cancellationErr)) {
		return nil
	}
	return fmt.Errorf("runnable %q failed during shutdown: %w", name, err)
}

func collectShutdownResults(
	results <-chan runnableResult,
	total int,
	received int,
	cancellationErr error,
	waitCompleted bool,
) error {
	var resultErr error
	if waitCompleted {
		for received < total {
			result := <-results
			received++
			resultErr = errors.Join(resultErr, shutdownResultError(result.name, result.err, cancellationErr))
		}
		return resultErr
	}

	for received < total {
		select {
		case result := <-results:
			received++
			resultErr = errors.Join(resultErr, shutdownResultError(result.name, result.err, cancellationErr))
		default:
			return resultErr
		}
	}
	return resultErr
}

func waitForRunnables(wait *sync.WaitGroup, timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		wait.Wait()
		close(done)
	}()

	if timeout <= 0 {
		<-done
		return nil
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return fmt.Errorf("%w after %s", ErrGracefulShutdownTimeout, timeout)
	}
}
