package task

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

type runSoonOnceSchedule struct {
	calls atomic.Int32
}

func (s *runSoonOnceSchedule) Next(now time.Time) time.Time {
	if s.calls.Add(1) == 1 {
		return now.Add(time.Millisecond)
	}
	return now.Add(time.Hour)
}

func TestTaskStartReturnsWhenShutdownTimeoutExpires(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	WithShutdownTimeout(20 * time.Millisecond)(&srv.opts)

	started := make(chan struct{})
	release := make(chan struct{})
	srv.opts.task.Schedule(&runSoonOnceSchedule{}, cron.FuncJob(func() {
		select {
		case <-started:
		default:
			close(started)
		}
		<-release
	}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("scheduled job did not start")
	}
	cancel()

	select {
	case err := <-done:
		close(release)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected shutdown deadline error, got %v", err)
		}
	case <-time.After(time.Second):
		close(release)
		t.Fatal("task server did not honor shutdown timeout")
	}
}
