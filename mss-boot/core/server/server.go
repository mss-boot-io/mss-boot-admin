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

	type result struct {
		name string
		err  error
	}

	results := make(chan result, len(runnables))
	var wait sync.WaitGroup
	wait.Add(len(runnables))
	for _, runnable := range runnables {
		runnable := runnable
		go func() {
			defer wait.Done()
			results <- result{name: runnable.String(), err: runnable.Start(runCtx)}
		}()
	}

	var runErr error
	select {
	case <-runCtx.Done():
		// Caller cancellation and process signals are normal shutdown paths.
	case result := <-results:
		if result.err != nil {
			runErr = fmt.Errorf("runnable %q failed: %w", result.name, result.err)
		} else {
			runErr = fmt.Errorf("runnable %q: %w", result.name, ErrRunnableStopped)
		}
	}

	cancel()
	shutdownErr := waitForRunnables(&wait, options.gracefulShutdownTimeout)
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
