package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
)

func TestMonitorSnapshotRequiresFirstSuccessfulSample(t *testing.T) {
	monitor := newTestMonitor(t, MonitorOptions{}, &incrementingMonitorSampler{}, time.Now)
	if _, err := monitor.Snapshot(DefaultMonitorHistorySize); !errors.Is(err, ErrMonitorNotReady) {
		t.Fatalf("Snapshot() error = %v, want %v", err, ErrMonitorNotReady)
	}
}

func TestMonitorSnapshotReadsBoundedLastGoodHistoryWithoutSampling(t *testing.T) {
	var clockN atomic.Int64
	base := time.Unix(1_700_000_000, 0)
	clock := func() time.Time {
		return base.Add(time.Duration(clockN.Add(1)) * time.Millisecond)
	}
	sampler := &incrementingMonitorSampler{}
	monitor := newTestMonitor(t, MonitorOptions{HistorySize: 3}, sampler, clock)
	monitor.instanceID = "test-instance"

	for range 5 {
		if err := monitor.collectOnce(context.Background()); err != nil {
			t.Fatalf("collectOnce() error = %v", err)
		}
	}
	before := sampler.calls.Load()

	response, err := monitor.Snapshot(2)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if sampler.calls.Load() != before {
		t.Fatalf("Snapshot() sampled the host: calls = %d, want %d", sampler.calls.Load(), before)
	}
	if len(response.History) != 2 {
		t.Fatalf("history length = %d, want 2", len(response.History))
	}
	if response.History[0].CPUUsage != 4 || response.History[1].CPUUsage != 5 {
		t.Fatalf("history CPU values = %#v, want 4,5", response.History)
	}
	if response.History[0].Timestamp >= response.History[1].Timestamp {
		t.Fatalf("history is not chronological: %#v", response.History)
	}
	if response.CollectedAt != response.History[1].Timestamp {
		t.Fatalf("collectedAt = %d, latest history = %d", response.CollectedAt, response.History[1].Timestamp)
	}
	if response.SampleIntervalMS != DefaultMonitorSampleInterval.Milliseconds() {
		t.Fatalf("sampleIntervalMs = %d", response.SampleIntervalMS)
	}
	if response.InstanceID != "test-instance" || response.Stale {
		t.Fatalf("metadata = instance %q stale %v", response.InstanceID, response.Stale)
	}

	response.History[0].CPUUsage = 999
	response.Runtime.Goroutines = 999
	again, err := monitor.Snapshot(2)
	if err != nil {
		t.Fatalf("second Snapshot() error = %v", err)
	}
	if again.History[0].CPUUsage == 999 || again.Runtime.Goroutines == 999 {
		t.Fatal("Snapshot() returned mutable collector-owned data")
	}

	empty, err := monitor.Snapshot(0)
	if err != nil {
		t.Fatalf("Snapshot(0) error = %v", err)
	}
	if empty.History == nil || len(empty.History) != 0 {
		t.Fatalf("Snapshot(0) history = %#v, want non-nil empty slice", empty.History)
	}
	if _, err := monitor.Snapshot(MaxMonitorHistorySize + 1); !errors.Is(err, ErrInvalidMonitorHistoryLimit) {
		t.Fatalf("Snapshot(over limit) error = %v", err)
	}
}

func TestMonitorTransientFailurePreservesLastGoodAndMarksStale(t *testing.T) {
	sampleErr := errors.New("temporary sampler failure")
	sampler := &scriptedMonitorSampler{steps: []monitorSampleStep{
		{response: testMonitorResponse(10)},
		{err: sampleErr},
		{response: testMonitorResponse(20)},
	}}
	now := time.Unix(1_700_000_000, 0)
	monitor := newTestMonitor(t, MonitorOptions{}, sampler, func() time.Time {
		now = now.Add(time.Second)
		return now
	})

	if err := monitor.collectOnce(context.Background()); err != nil {
		t.Fatalf("first collectOnce() error = %v", err)
	}
	first, err := monitor.Snapshot(DefaultMonitorHistorySize)
	if err != nil {
		t.Fatalf("first Snapshot() error = %v", err)
	}
	if err := monitor.collectOnce(context.Background()); !errors.Is(err, sampleErr) {
		t.Fatalf("failed collectOnce() error = %v, want %v", err, sampleErr)
	}
	stale, err := monitor.Snapshot(DefaultMonitorHistorySize)
	if err != nil {
		t.Fatalf("stale Snapshot() error = %v", err)
	}
	if !stale.Stale || stale.CollectedAt != first.CollectedAt || len(stale.History) != 1 {
		t.Fatalf("stale last-good response = %#v", stale)
	}

	if err := monitor.collectOnce(context.Background()); err != nil {
		t.Fatalf("recovery collectOnce() error = %v", err)
	}
	recovered, err := monitor.Snapshot(DefaultMonitorHistorySize)
	if err != nil {
		t.Fatalf("recovered Snapshot() error = %v", err)
	}
	if recovered.Stale || len(recovered.History) != 2 || recovered.CPUUsage != 20 {
		t.Fatalf("recovered response = %#v", recovered)
	}
}

func TestMonitorOptionsDefaultsAndValidation(t *testing.T) {
	monitor := newTestMonitor(t, MonitorOptions{}, &incrementingMonitorSampler{}, time.Now)
	if monitor.SampleInterval() != DefaultMonitorSampleInterval {
		t.Fatalf("default sample interval = %s", monitor.SampleInterval())
	}
	if monitor.options.SampleTimeout != DefaultMonitorSampleTimeout ||
		monitor.options.HistorySize != DefaultMonitorHistorySize {
		t.Fatalf("default options = %#v", monitor.options)
	}

	tests := []MonitorOptions{
		{SampleInterval: -time.Second},
		{SampleInterval: 999 * time.Millisecond, SampleTimeout: 500 * time.Millisecond},
		{SampleInterval: 1500 * time.Millisecond, SampleTimeout: time.Second},
		{SampleTimeout: -time.Second},
		{SampleInterval: time.Second, SampleTimeout: time.Second},
		{HistorySize: MaxMonitorHistorySize + 1},
		{HistorySize: -1},
	}
	for _, options := range tests {
		if _, err := newMonitor(options, &incrementingMonitorSampler{}, time.Now); err == nil {
			t.Fatalf("newMonitor(%#v) error = nil", options)
		}
	}
	if _, err := newMonitor(MonitorOptions{
		SampleInterval: time.Second,
		SampleTimeout:  500 * time.Millisecond,
	}, &incrementingMonitorSampler{}, time.Now); err != nil {
		t.Fatalf("newMonitor(whole-second interval) error = %v", err)
	}
}

func TestMonitorConcurrentCollectionAndSnapshots(t *testing.T) {
	var clockN atomic.Int64
	monitor := newTestMonitor(t, MonitorOptions{}, &incrementingMonitorSampler{}, func() time.Time {
		return time.UnixMilli(clockN.Add(1))
	})
	if err := monitor.collectOnce(context.Background()); err != nil {
		t.Fatalf("prime collectOnce() error = %v", err)
	}

	var wait sync.WaitGroup
	wait.Add(9)
	go func() {
		defer wait.Done()
		for range 200 {
			if err := monitor.collectOnce(context.Background()); err != nil {
				t.Errorf("collectOnce() error = %v", err)
				return
			}
		}
	}()
	for range 8 {
		go func() {
			defer wait.Done()
			for range 200 {
				response, err := monitor.Snapshot(DefaultMonitorHistorySize)
				if err != nil {
					t.Errorf("Snapshot() error = %v", err)
					return
				}
				if len(response.History) > MaxMonitorHistorySize {
					t.Errorf("history length = %d", len(response.History))
					return
				}
			}
		}()
	}
	wait.Wait()
}

type incrementingMonitorSampler struct {
	calls atomic.Int64
}

func (e *incrementingMonitorSampler) Sample(context.Context, time.Time) (*dto.MonitorResponse, error) {
	return testMonitorResponse(float64(e.calls.Add(1))), nil
}

type monitorSampleStep struct {
	response *dto.MonitorResponse
	err      error
	release  <-chan struct{}
}

type scriptedMonitorSampler struct {
	mu    sync.Mutex
	steps []monitorSampleStep
	calls int
	call  chan int
}

func (e *scriptedMonitorSampler) Sample(context.Context, time.Time) (*dto.MonitorResponse, error) {
	e.mu.Lock()
	index := e.calls
	e.calls++
	step := monitorSampleStep{response: testMonitorResponse(float64(e.calls))}
	if index < len(e.steps) {
		step = e.steps[index]
	}
	call := e.call
	e.mu.Unlock()
	if call != nil {
		call <- index + 1
	}
	if step.release != nil {
		<-step.release
	}
	return cloneMonitorResponse(step.response), step.err
}

func testMonitorResponse(cpuUsage float64) *dto.MonitorResponse {
	return &dto.MonitorResponse{
		CPUUsage:           cpuUsage,
		MemoryUsagePercent: cpuUsage + 10,
		CPUInfo:            make([]dto.MonitorCPUInfo, 0),
		Network:            &dto.MonitorNetwork{ConnectionCount: &dto.MonitorConnectionCount{}},
		Runtime:            &dto.MonitorRuntime{Goroutines: 1},
		GoVersion:          "test",
	}
}

func newTestMonitor(
	t *testing.T,
	options MonitorOptions,
	sampler monitorSampler,
	now func() time.Time,
) *Monitor {
	t.Helper()
	monitor, err := newMonitor(options, sampler, now)
	if err != nil {
		t.Fatalf("newMonitor() error = %v", err)
	}
	return monitor
}
