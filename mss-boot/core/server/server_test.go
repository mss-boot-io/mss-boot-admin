package server

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type testRunnable struct {
	name  string
	start func(context.Context) error
}

func (r *testRunnable) String() string {
	return r.name
}

func (r *testRunnable) Start(ctx context.Context) error {
	return r.start(ctx)
}

func TestServerStopsRunnablesOnContextCancellation(t *testing.T) {
	manager := New(WithoutSignalHandling())

	started := make(chan string, 2)
	stopped := make(chan string, 2)
	for _, name := range []string{"http", "worker"} {
		name := name
		manager.Add(&testRunnable{
			name: name,
			start: func(ctx context.Context) error {
				started <- name
				<-ctx.Done()
				stopped <- name
				return nil
			},
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- manager.Start(ctx)
	}()

	waitForNames(t, started, 2)
	cancel()

	if err := receiveError(t, done); err != nil {
		t.Fatalf("expected a clean shutdown, got %v", err)
	}
	waitForNames(t, stopped, 2)
}

func TestServerPropagatesRuntimeErrorAndCancelsPeers(t *testing.T) {
	manager := New(WithoutSignalHandling())
	sentinel := errors.New("serve failed")
	peerStarted := make(chan struct{})
	peerStopped := make(chan struct{})

	manager.Add(
		&testRunnable{
			name: "peer",
			start: func(ctx context.Context) error {
				close(peerStarted)
				<-ctx.Done()
				close(peerStopped)
				return nil
			},
		},
		&testRunnable{
			name: "failing",
			start: func(context.Context) error {
				<-peerStarted
				return sentinel
			},
		},
	)

	err := manager.Start(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected runtime error %v, got %v", sentinel, err)
	}
	select {
	case <-peerStopped:
	case <-time.After(time.Second):
		t.Fatal("peer runnable was not cancelled")
	}
}

func TestServerRejectsUnexpectedSuccessfulExit(t *testing.T) {
	manager := New(WithoutSignalHandling())
	manager.Add(&testRunnable{
		name:  "short-lived",
		start: func(context.Context) error { return nil },
	})

	err := manager.Start(context.Background())
	if !errors.Is(err, ErrRunnableStopped) {
		t.Fatalf("expected ErrRunnableStopped, got %v", err)
	}
}

func TestServerReturnsShutdownTimeout(t *testing.T) {
	manager := New(
		WithoutSignalHandling(),
		WithGracefulShutdownTimeout(20*time.Millisecond),
	)
	started := make(chan struct{})
	release := make(chan struct{})
	manager.Add(&testRunnable{
		name: "stuck",
		start: func(context.Context) error {
			close(started)
			<-release
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- manager.Start(ctx)
	}()
	<-started
	cancel()

	err := receiveError(t, done)
	close(release)
	if !errors.Is(err, ErrGracefulShutdownTimeout) {
		t.Fatalf("expected ErrGracefulShutdownTimeout, got %v", err)
	}
}

func TestServerCanOnlyStartOnce(t *testing.T) {
	manager := New(WithoutSignalHandling())
	started := make(chan struct{})
	manager.Add(&testRunnable{
		name: "service",
		start: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- manager.Start(ctx)
	}()
	<-started
	cancel()
	if err := receiveError(t, done); err != nil {
		t.Fatalf("first start failed: %v", err)
	}

	if err := manager.Start(context.Background()); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("expected ErrAlreadyStarted, got %v", err)
	}
}

func TestServerPreservesRegistrationOrderWhenReplacingRunnable(t *testing.T) {
	manager := New(WithoutSignalHandling())
	serverManager := manager.(*Server)

	manager.Add(
		&testRunnable{name: "first", start: waitForCancellation},
		&testRunnable{name: "second", start: waitForCancellation},
	)
	manager.Add(&testRunnable{name: "first", start: waitForCancellation})

	serverManager.mux.Lock()
	defer serverManager.mux.Unlock()
	if len(serverManager.order) != 2 {
		t.Fatalf("expected two ordered registrations, got %d", len(serverManager.order))
	}
	if serverManager.order[0] != "first" || serverManager.order[1] != "second" {
		t.Fatalf("unexpected registration order: %v", serverManager.order)
	}
}

func waitForCancellation(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func receiveError(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for manager")
		return nil
	}
}

func waitForNames(t *testing.T, ch <-chan string, count int) {
	t.Helper()
	seen := make(map[string]struct{}, count)
	var mux sync.Mutex
	for len(seen) < count {
		select {
		case name := <-ch:
			mux.Lock()
			seen[name] = struct{}{}
			mux.Unlock()
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %d names; got %v", count, seen)
		}
	}
}
