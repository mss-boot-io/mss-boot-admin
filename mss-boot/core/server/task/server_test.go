package task

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStartBlocksUntilCancellation(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("Start returned before cancellation: %v", err)
	default:
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("task shutdown failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("task server did not stop")
	}
}

func TestStartCanOnlyRunOnce(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	if err := srv.Start(context.Background()); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("expected ErrAlreadyStarted, got %v", err)
	}
}

func TestStartReturnsInvalidScheduleError(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	WithSchedule("invalid", "not-a-cron-expression", testJob{})(&srv.opts)
	if err := srv.Start(context.Background()); err == nil {
		t.Fatal("expected invalid schedule error")
	}
}

type testJob struct{}

func (testJob) Run() {}
