package server

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestServerPropagatesRunnableShutdownError(t *testing.T) {
	manager := New(WithoutSignalHandling())
	started := make(chan struct{})
	shutdownErr := errors.New("flush failed")
	manager.Add(&testRunnable{
		name: "worker",
		start: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return shutdownErr
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- manager.Start(ctx)
	}()
	<-started
	cancel()

	if err := receiveError(t, done); !errors.Is(err, shutdownErr) {
		t.Fatalf("expected shutdown error %v, got %v", shutdownErr, err)
	}
}

func TestServerIgnoresMatchingContextErrorDuringShutdown(t *testing.T) {
	manager := New(WithoutSignalHandling())
	started := make(chan struct{})
	manager.Add(&testRunnable{
		name: "worker",
		start: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
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
		t.Fatalf("expected matching context error to be ignored, got %v", err)
	}
}

func TestServerIgnoresWrappedContextErrorDuringShutdown(t *testing.T) {
	manager := New(WithoutSignalHandling())
	started := make(chan struct{})
	manager.Add(&testRunnable{
		name: "worker",
		start: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return fmt.Errorf("worker stopped: %w", ctx.Err())
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
		t.Fatalf("expected wrapped context error to be ignored, got %v", err)
	}
}
