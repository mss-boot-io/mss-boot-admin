package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/shirou/gopsutil/v3/cpu"
)

func TestMonitorSystemJobPrimesCPUBaselineThenPublishes(t *testing.T) {
	sampler := &scriptedMonitorSampler{
		steps: []monitorSampleStep{
			{err: errMonitorCPUBaseline},
			{response: testMonitorResponse(20)},
		},
	}
	monitor, err := newMonitor(MonitorOptions{}, sampler, time.Now)
	if err != nil {
		t.Fatalf("newMonitor() error = %v", err)
	}

	monitor.Prime(context.Background())
	if _, err := monitor.Snapshot(DefaultMonitorHistorySize); !errors.Is(err, ErrMonitorNotReady) {
		t.Fatalf("Snapshot() after CPU baseline error = %v, want %v", err, ErrMonitorNotReady)
	}

	monitor.Run()
	response, err := monitor.Snapshot(DefaultMonitorHistorySize)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(response.History) != 1 || response.CPUUsage != 20 {
		t.Fatalf("sampled response = %#v", response)
	}
	if err := monitor.Configure(MonitorOptions{}); !errors.Is(err, ErrMonitorAlreadyStarted) {
		t.Fatalf("Configure() after Prime error = %v, want %v", err, ErrMonitorAlreadyStarted)
	}
}

func TestMonitorSystemJobTransientFailurePreservesLastGood(t *testing.T) {
	sampleErr := errors.New("temporary")
	sampler := &scriptedMonitorSampler{
		steps: []monitorSampleStep{
			{response: testMonitorResponse(10)},
			{err: sampleErr},
			{response: testMonitorResponse(30)},
		},
	}
	monitor, err := newMonitor(MonitorOptions{}, sampler, time.Now)
	if err != nil {
		t.Fatalf("newMonitor() error = %v", err)
	}

	monitor.Run()
	monitor.Run()
	stale, err := monitor.Snapshot(DefaultMonitorHistorySize)
	if err != nil {
		t.Fatalf("stale Snapshot() error = %v", err)
	}
	if !stale.Stale || len(stale.History) != 1 {
		t.Fatalf("stale response = %#v", stale)
	}

	monitor.Run()
	recovered, err := monitor.Snapshot(DefaultMonitorHistorySize)
	if err != nil {
		t.Fatalf("recovered Snapshot() error = %v", err)
	}
	if recovered.Stale || len(recovered.History) != 2 || recovered.CPUUsage != 30 {
		t.Fatalf("recovered response = %#v", recovered)
	}
}

func TestMonitorSystemJobSkipsOverlappingSample(t *testing.T) {
	release := make(chan struct{})
	sampler := &scriptedMonitorSampler{
		steps: []monitorSampleStep{{response: testMonitorResponse(10), release: release}},
		call:  make(chan int, 2),
	}
	monitor, err := newMonitor(MonitorOptions{}, sampler, time.Now)
	if err != nil {
		t.Fatalf("newMonitor() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		monitor.Run()
		close(done)
	}()
	waitForMonitorCall(t, sampler.call, 1)

	monitor.Run()
	sampler.mu.Lock()
	calls := sampler.calls
	sampler.mu.Unlock()
	if calls != 1 {
		t.Fatalf("overlapping Run() sampler calls = %d, want 1", calls)
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("first Run() did not finish")
	}
}

func TestMonitorSystemJobBoundsSampleWithTimeout(t *testing.T) {
	sampler := &deadlineMonitorSampler{}
	monitor, err := newMonitor(MonitorOptions{
		SampleInterval: time.Second,
		SampleTimeout:  10 * time.Millisecond,
	}, sampler, time.Now)
	if err != nil {
		t.Fatalf("newMonitor() error = %v", err)
	}

	monitor.Run()
	if !sampler.deadlineObserved {
		t.Fatal("sampler did not observe the configured deadline")
	}
	if _, err := monitor.Snapshot(DefaultMonitorHistorySize); !errors.Is(err, ErrMonitorNotReady) {
		t.Fatalf("Snapshot() error = %v, want %v", err, ErrMonitorNotReady)
	}
}

func TestCPUUsageBetweenUsesCounterDelta(t *testing.T) {
	previous := cpu.TimesStat{User: 10, System: 5, Idle: 85}
	current := cpu.TimesStat{User: 30, System: 10, Idle: 160}
	if got := cpuUsageBetween(previous, current); got != 25 {
		t.Fatalf("cpuUsageBetween() = %v, want 25", got)
	}
	if got := cpuUsageBetween(current, previous); got != 0 {
		t.Fatalf("cpuUsageBetween(counter reset) = %v, want 0", got)
	}
}

func TestMonitorSampleFailureLogsAreRateLimited(t *testing.T) {
	now := time.Unix(100, 0)
	monitor, err := newMonitor(MonitorOptions{}, &scriptedMonitorSampler{}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMonitor() error = %v", err)
	}

	if logFailure, suppressed := monitor.shouldLogSampleFailure(now); !logFailure || suppressed != 0 {
		t.Fatalf("first failure log decision = (%v, %d), want (true, 0)", logFailure, suppressed)
	}
	for i := 0; i < 11; i++ {
		now = now.Add(5 * time.Second)
		if logFailure, _ := monitor.shouldLogSampleFailure(now); logFailure {
			t.Fatalf("failure %d logged inside throttle window", i+2)
		}
	}
	now = now.Add(5 * time.Second)
	if logFailure, suppressed := monitor.shouldLogSampleFailure(now); !logFailure || suppressed != 11 {
		t.Fatalf("window failure log decision = (%v, %d), want (true, 11)", logFailure, suppressed)
	}
}

func waitForMonitorCall(t *testing.T, calls <-chan int, want int) {
	t.Helper()
	select {
	case got := <-calls:
		if got != want {
			t.Fatalf("sampler call = %d, want %d", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("sampler call %d did not occur", want)
	}
}

type deadlineMonitorSampler struct {
	deadlineObserved bool
}

func (e *deadlineMonitorSampler) Sample(ctx context.Context, _ time.Time) (*dto.MonitorResponse, error) {
	_, e.deadlineObserved = ctx.Deadline()
	<-ctx.Done()
	return nil, ctx.Err()
}
