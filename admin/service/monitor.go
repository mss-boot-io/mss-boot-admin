package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
)

const (
	DefaultMonitorSampleInterval = 5 * time.Second
	DefaultMonitorSampleTimeout  = 3 * time.Second
	DefaultMonitorHistorySize    = 120
	MaxMonitorHistorySize        = 120
	monitorFailureLogInterval    = time.Minute
)

var (
	ErrMonitorNotReady            = errors.New("monitor data is not ready")
	ErrMonitorAlreadyStarted      = errors.New("monitor sampler has already been started")
	ErrMonitorSampleInProgress    = errors.New("monitor sample is already in progress")
	ErrInvalidMonitorHistoryLimit = errors.New("monitor history limit must be between 0 and 120")
)

type MonitorOptions struct {
	SampleInterval time.Duration
	SampleTimeout  time.Duration
	HistorySize    int
}

type monitorSampler interface {
	Sample(context.Context, time.Time) (*dto.MonitorResponse, error)
}

// Monitor owns the process-local sampler lifecycle and its bounded last-good
// snapshot. HTTP handlers only call Snapshot; they never sample the host.
type Monitor struct {
	mu         sync.RWMutex
	options    MonitorOptions
	history    monitorHistoryRing
	latest     *dto.MonitorResponse
	stale      bool
	started    bool
	sampler    monitorSampler
	now        func() time.Time
	instanceID string
	collecting atomic.Bool

	lastFailureLog       time.Time
	suppressedFailureLog uint64
}

var DefaultMonitor = mustNewMonitor(MonitorOptions{})

func NewMonitor(options MonitorOptions) (*Monitor, error) {
	return newMonitor(options, newSystemMonitorSampler(), time.Now)
}

func mustNewMonitor(options MonitorOptions) *Monitor {
	monitor, err := NewMonitor(options)
	if err != nil {
		panic(err)
	}
	return monitor
}

func newMonitor(
	options MonitorOptions,
	sampler monitorSampler,
	now func() time.Time,
) (*Monitor, error) {
	normalized, err := normalizeMonitorOptions(options)
	if err != nil {
		return nil, err
	}
	if sampler == nil {
		return nil, errors.New("monitor sampler is required")
	}
	if now == nil {
		return nil, errors.New("monitor clock is required")
	}
	return &Monitor{
		options:    normalized,
		history:    newMonitorHistoryRing(normalized.HistorySize),
		sampler:    sampler,
		now:        now,
		instanceID: monitorInstanceID(),
	}, nil
}

func normalizeMonitorOptions(options MonitorOptions) (MonitorOptions, error) {
	if options.SampleInterval == 0 {
		options.SampleInterval = DefaultMonitorSampleInterval
	}
	if options.SampleInterval < time.Second {
		return MonitorOptions{}, errors.New("monitor sample interval must be at least one second")
	}
	if options.SampleInterval%time.Second != 0 {
		return MonitorOptions{}, errors.New("monitor sample interval must be a whole number of seconds")
	}
	if options.SampleTimeout == 0 {
		options.SampleTimeout = DefaultMonitorSampleTimeout
	}
	if options.SampleTimeout < 0 {
		return MonitorOptions{}, errors.New("monitor sample timeout must be positive")
	}
	if options.SampleTimeout >= options.SampleInterval {
		return MonitorOptions{}, fmt.Errorf(
			"monitor sample timeout %s must be shorter than interval %s",
			options.SampleTimeout,
			options.SampleInterval,
		)
	}
	if options.HistorySize == 0 {
		options.HistorySize = DefaultMonitorHistorySize
	}
	if options.HistorySize < 0 || options.HistorySize > MaxMonitorHistorySize {
		return MonitorOptions{}, fmt.Errorf(
			"monitor history size must be between 1 and %d",
			MaxMonitorHistorySize,
		)
	}
	return options, nil
}

// Configure applies startup configuration before the system cron job is run.
// A started Monitor is deliberately immutable so configuration reloads cannot
// create a second sampler or replace a live ring buffer.
func (e *Monitor) Configure(options MonitorOptions) error {
	normalized, err := normalizeMonitorOptions(options)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return ErrMonitorAlreadyStarted
	}
	e.options = normalized
	e.history = newMonitorHistoryRing(normalized.HistorySize)
	e.latest = nil
	e.stale = false
	return nil
}

func (e *Monitor) String() string {
	return "monitor-sampler"
}

func (e *Monitor) SampleInterval() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.options.SampleInterval
}

// ScheduleSpec returns the robfig/cron descriptor used by the process-local
// system task scheduler.
func (e *Monitor) ScheduleSpec() string {
	return "@every " + e.SampleInterval().String()
}

// Snapshot returns an isolated copy of the last successful sample and at most
// historyLimit chronological points. It performs no host I/O.
func (e *Monitor) Snapshot(historyLimit int) (*dto.MonitorResponse, error) {
	if historyLimit < 0 || historyLimit > MaxMonitorHistorySize {
		return nil, ErrInvalidMonitorHistoryLimit
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.latest == nil {
		return nil, ErrMonitorNotReady
	}
	result := cloneMonitorResponse(e.latest)
	result.Stale = e.stale
	result.History = e.history.latest(historyLimit)
	return result, nil
}

func (e *Monitor) storeSample(sample *dto.MonitorResponse, collectedAt time.Time) {
	owned := cloneMonitorResponse(sample)
	owned.CollectedAt = collectedAt.UTC().UnixMilli()
	owned.InstanceID = e.instanceID
	owned.History = nil

	e.mu.Lock()
	owned.SampleIntervalMS = e.options.SampleInterval.Milliseconds()
	owned.Stale = false
	e.history.append(dto.MonitorHistoryPoint{
		Timestamp:          owned.CollectedAt,
		CPUUsage:           owned.CPUUsage,
		MemoryUsagePercent: owned.MemoryUsagePercent,
	})
	e.latest = owned
	e.stale = false
	e.mu.Unlock()
}

func (e *Monitor) markStale() {
	e.mu.Lock()
	e.stale = true
	e.mu.Unlock()
}

func (e *Monitor) shouldLogSampleFailure(now time.Time) (bool, uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.lastFailureLog.IsZero() || now.Before(e.lastFailureLog) ||
		now.Sub(e.lastFailureLog) >= monitorFailureLogInterval {
		suppressed := e.suppressedFailureLog
		e.lastFailureLog = now
		e.suppressedFailureLog = 0
		return true, suppressed
	}
	e.suppressedFailureLog++
	return false, 0
}

func cloneMonitorResponse(source *dto.MonitorResponse) *dto.MonitorResponse {
	if source == nil {
		return nil
	}
	result := *source
	result.CPUInfo = append([]dto.MonitorCPUInfo(nil), source.CPUInfo...)
	for i := range result.CPUInfo {
		result.CPUInfo[i].Flags = append([]string(nil), source.CPUInfo[i].Flags...)
	}
	if source.Network != nil {
		network := *source.Network
		network.Connections = append(network.Connections[:0:0], source.Network.Connections...)
		if source.Network.ConnectionCount != nil {
			connectionCount := *source.Network.ConnectionCount
			network.ConnectionCount = &connectionCount
		}
		result.Network = &network
	}
	if source.Runtime != nil {
		runtimeInfo := *source.Runtime
		result.Runtime = &runtimeInfo
	}
	result.History = append([]dto.MonitorHistoryPoint(nil), source.History...)
	return &result
}

type monitorHistoryRing struct {
	items []dto.MonitorHistoryPoint
	next  int
	count int
}

func newMonitorHistoryRing(capacity int) monitorHistoryRing {
	return monitorHistoryRing{items: make([]dto.MonitorHistoryPoint, capacity)}
}

func (e *monitorHistoryRing) append(point dto.MonitorHistoryPoint) {
	if len(e.items) == 0 {
		return
	}
	e.items[e.next] = point
	e.next = (e.next + 1) % len(e.items)
	if e.count < len(e.items) {
		e.count++
	}
}

func (e *monitorHistoryRing) latest(limit int) []dto.MonitorHistoryPoint {
	if limit == 0 || e.count == 0 {
		return make([]dto.MonitorHistoryPoint, 0)
	}
	if limit > e.count {
		limit = e.count
	}
	start := (e.next - limit + len(e.items)) % len(e.items)
	result := make([]dto.MonitorHistoryPoint, limit)
	for i := range result {
		result[i] = e.items[(start+i)%len(e.items)]
	}
	return result
}
