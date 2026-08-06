package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestDefaultCronChainRecoversPanickingJob(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	job := &panicOnceThenSignalJob{ranAfterPanic: make(chan struct{}, 1)}
	srv.opts.task.Schedule(&runTwiceSoonSchedule{}, job)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Start(ctx) }()

	select {
	case <-job.ranAfterPanic:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("cron did not run the job again after its first invocation panicked")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("task server shutdown error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("task server did not stop")
	}
	if calls := job.calls.Load(); calls < 2 {
		t.Fatalf("job calls = %d, want at least 2", calls)
	}
}

type runTwiceSoonSchedule struct {
	nextCalls atomic.Int32
}

func (e *runTwiceSoonSchedule) Next(now time.Time) time.Time {
	if e.nextCalls.Add(1) <= 2 {
		return now.Add(5 * time.Millisecond)
	}
	return now.Add(time.Hour)
}

type panicOnceThenSignalJob struct {
	calls         atomic.Int32
	ranAfterPanic chan struct{}
}

func (e *panicOnceThenSignalJob) Run() {
	if e.calls.Add(1) == 1 {
		panic("injected task panic")
	}
	select {
	case e.ranAfterPanic <- struct{}{}:
	default:
	}
}

var _ cron.Schedule = (*runTwiceSoonSchedule)(nil)
var _ cron.Job = (*panicOnceThenSignalJob)(nil)
