package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
)

const monitorSlowSampleInterval = 30 * time.Second

var errMonitorCPUBaseline = errors.New("monitor CPU baseline initialized")

// Prime establishes the CPU counter baseline before the task scheduler starts.
// It performs one bounded collection attempt but never prevents application
// startup; failures are surfaced through stale/not-ready monitor state.
func (e *Monitor) Prime(ctx context.Context) {
	e.collectAndReport(ctx)
}

// Run implements cron.Job. Sampling remains process-local and never creates a
// database Task or TaskRun record.
func (e *Monitor) Run() {
	e.collectAndReport(context.Background())
}

func (e *Monitor) collectAndReport(parent context.Context) {
	if parent == nil {
		parent = context.Background()
	}
	err := e.collectOnce(parent)
	if err == nil || errors.Is(err, errMonitorCPUBaseline) ||
		errors.Is(err, ErrMonitorSampleInProgress) || parent.Err() != nil {
		return
	}
	if logFailure, suppressed := e.shouldLogSampleFailure(e.now()); logFailure {
		slog.Warn(
			"monitor sample failed; serving last-good data",
			"err", err,
			"suppressedSinceLastLog", suppressed,
		)
	}
}

func (e *Monitor) collectOnce(parent context.Context) error {
	if !e.collecting.CompareAndSwap(false, true) {
		return ErrMonitorSampleInProgress
	}
	defer e.collecting.Store(false)

	if parent == nil {
		parent = context.Background()
	}
	e.mu.Lock()
	e.started = true
	timeout := e.options.SampleTimeout
	e.mu.Unlock()

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	collectedAt := e.now().UTC()
	sample, err := e.sampler.Sample(ctx, collectedAt)
	if err != nil {
		if errors.Is(err, errMonitorCPUBaseline) {
			return err
		}
		e.markStale()
		return err
	}
	if sample == nil {
		e.markStale()
		return errors.New("monitor sampler returned a nil snapshot")
	}
	e.storeSample(sample, collectedAt)
	return nil
}

type systemMonitorSampler struct {
	startTime time.Time

	previousCPU *cpu.TimesStat

	staticReady       bool
	lastStaticAttempt time.Time
	logicalCores      int
	physicalCores     int
	cpuInfo           []cpu.InfoStat

	lastSlowSample time.Time
	diskMount      string
	slow           monitorSlowSnapshot
}

type monitorSlowSnapshot struct {
	diskTotal        uint64
	diskTotalGB      float64
	diskUsage        uint64
	diskUsageGB      float64
	diskUsagePercent float64
	network          dto.MonitorNetwork
}

func newSystemMonitorSampler() monitorSampler {
	return &systemMonitorSampler{startTime: time.Now()}
}

func (e *systemMonitorSampler) Sample(ctx context.Context, now time.Time) (*dto.MonitorResponse, error) {
	cpuTimes, err := cpu.TimesWithContext(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("read CPU times: %w", err)
	}
	if len(cpuTimes) == 0 {
		return nil, errors.New("read CPU times: no aggregate CPU returned")
	}
	currentCPU := cpuTimes[0]
	if e.previousCPU == nil {
		e.previousCPU = &currentCPU
		return nil, errMonitorCPUBaseline
	}
	cpuUsage := cpuUsageBetween(*e.previousCPU, currentCPU)
	e.previousCPU = &currentCPU

	memory, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("read virtual memory: %w", err)
	}

	e.refreshStatic(ctx, now)
	e.refreshSlow(ctx, now)

	cpuInfo := make([]dto.MonitorCPUInfo, 0, len(e.cpuInfo))
	for i := range e.cpuInfo {
		info := e.cpuInfo[i]
		info.Flags = append([]string(nil), e.cpuInfo[i].Flags...)
		cpuInfo = append(cpuInfo, dto.MonitorCPUInfo{
			InfoStat: info,
			// Preserve the existing per-CPU field's 0..1 ratio contract.
			CPUUsagePercent: cpuUsage / 100,
		})
	}

	network := e.slow.network
	network.Connections = append(network.Connections[:0:0], e.slow.network.Connections...)
	if e.slow.network.ConnectionCount != nil {
		connectionCount := *e.slow.network.ConnectionCount
		network.ConnectionCount = &connectionCount
	}

	response := &dto.MonitorResponse{
		CPUPhysicalCore:    e.physicalCores,
		CPULogicalCore:     e.logicalCores,
		CPUUsage:           cpuUsage,
		CPUInfo:            cpuInfo,
		MemoryTotal:        memory.Total,
		MemoryUsage:        memory.Used,
		MemoryUsagePercent: roundPercent(memory.UsedPercent),
		MemoryAvailable:    memory.Available,
		MemoryFree:         memory.Free,
		DiskTotal:          e.slow.diskTotal,
		DiskTotalGB:        e.slow.diskTotalGB,
		DiskUsage:          e.slow.diskUsage,
		DiskUsageGB:        e.slow.diskUsageGB,
		DiskUsagePercent:   e.slow.diskUsagePercent,
		Network:            &network,
		Runtime:            monitorRuntimeSnapshot(),
		GoVersion:          runtime.Version(),
		StartTime:          e.startTime.Unix(),
		Uptime:             int64(now.Sub(e.startTime).Seconds()),
	}
	if response.CPULogicalCore == 0 {
		response.CPULogicalCore = runtime.NumCPU()
	}
	return response, nil
}

func (e *systemMonitorSampler) refreshStatic(ctx context.Context, now time.Time) {
	if e.staticReady || (!e.lastStaticAttempt.IsZero() && now.Sub(e.lastStaticAttempt) < monitorSlowSampleInterval) {
		return
	}
	e.lastStaticAttempt = now

	logical, err := cpu.CountsWithContext(ctx, true)
	if err != nil {
		slog.Warn("monitor CPU topology sample failed", "err", err)
		return
	}
	physical, err := cpu.CountsWithContext(ctx, false)
	if err != nil {
		slog.Warn("monitor physical CPU count sample failed", "err", err)
		return
	}
	info, err := cpu.InfoWithContext(ctx)
	if err != nil {
		slog.Warn("monitor CPU information sample failed", "err", err)
		return
	}

	e.logicalCores = logical
	e.physicalCores = physical
	e.cpuInfo = uniquePhysicalCPUInfo(info)
	e.staticReady = true
}

func (e *systemMonitorSampler) refreshSlow(ctx context.Context, now time.Time) {
	if !e.lastSlowSample.IsZero() && now.Sub(e.lastSlowSample) < monitorSlowSampleInterval {
		return
	}
	e.lastSlowSample = now

	if e.diskMount == "" {
		partitions, err := disk.PartitionsWithContext(ctx, false)
		if err != nil {
			slog.Warn("monitor disk partition sample failed", "err", err)
		} else if len(partitions) > 0 {
			e.diskMount = partitions[0].Mountpoint
		}
	}
	if e.diskMount != "" {
		usage, err := disk.UsageWithContext(ctx, e.diskMount)
		if err != nil {
			slog.Warn("monitor disk usage sample failed", "err", err)
		} else {
			e.slow.diskTotal = usage.Total
			e.slow.diskTotalGB = roundBytesToGB(usage.Total)
			e.slow.diskUsage = usage.Used
			e.slow.diskUsageGB = roundBytesToGB(usage.Used)
			e.slow.diskUsagePercent = roundPercent(usage.UsedPercent)
		}
	}

	if counters, err := psnet.IOCountersWithContext(ctx, false); err != nil {
		slog.Warn("monitor network IO sample failed", "err", err)
	} else if len(counters) > 0 {
		e.slow.network.BytesSent = counters[0].BytesSent
		e.slow.network.BytesRecv = counters[0].BytesRecv
		e.slow.network.PacketsSent = counters[0].PacketsSent
		e.slow.network.PacketsRecv = counters[0].PacketsRecv
		e.slow.network.Errin = counters[0].Errin
		e.slow.network.Errout = counters[0].Errout
		e.slow.network.Dropin = counters[0].Dropin
		e.slow.network.Dropout = counters[0].Dropout
	}

	connections, err := psnet.ConnectionsWithContext(ctx, "all")
	if err != nil {
		slog.Warn("monitor connection sample failed", "err", err)
		return
	}
	connectionCount := &dto.MonitorConnectionCount{}
	for _, connection := range connections {
		connectionCount.Total++
		switch connection.Status {
		case "ESTABLISHED":
			connectionCount.Established++
		case "LISTEN":
			connectionCount.Listen++
		case "TIME_WAIT":
			connectionCount.TimeWait++
		case "CLOSE_WAIT":
			connectionCount.CloseWait++
		}
	}
	e.slow.network.ConnectionCount = connectionCount
}

func uniquePhysicalCPUInfo(info []cpu.InfoStat) []cpu.InfoStat {
	result := make([]cpu.InfoStat, 0, len(info))
	seen := make(map[string]struct{}, len(info))
	for i := range info {
		key := info[i].PhysicalID
		if key == "" {
			key = strconv.FormatInt(int64(info[i].CPU), 10)
		}
		if key == "" {
			key = strconv.Itoa(i)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		item := info[i]
		item.Flags = append([]string(nil), info[i].Flags...)
		result = append(result, item)
	}
	return result
}

func cpuUsageBetween(previous, current cpu.TimesStat) float64 {
	previousTotal, previousBusy := cpuTotalAndBusy(previous)
	currentTotal, currentBusy := cpuTotalAndBusy(current)
	if currentBusy <= previousBusy {
		return 0
	}
	if currentTotal <= previousTotal {
		return 100
	}
	return roundPercent(math.Min(100, math.Max(0,
		(currentBusy-previousBusy)/(currentTotal-previousTotal)*100,
	)))
}

func cpuTotalAndBusy(value cpu.TimesStat) (float64, float64) {
	total := value.User + value.System + value.Idle + value.Nice + value.Iowait +
		value.Irq + value.Softirq + value.Steal + value.Guest + value.GuestNice
	if runtime.GOOS == "linux" {
		total -= value.Guest + value.GuestNice
	}
	return total, total - value.Idle - value.Iowait
}

func roundPercent(value float64) float64 {
	return math.Round(value*100) / 100
}

func roundBytesToGB(value uint64) float64 {
	return math.Round(float64(value)/1024/1024/1024*100) / 100
}

func monitorRuntimeSnapshot() *dto.MonitorRuntime {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	result := &dto.MonitorRuntime{
		Goroutines:     runtime.NumGoroutine(),
		HeapAlloc:      stats.HeapAlloc,
		HeapSys:        stats.HeapSys,
		HeapIdle:       stats.HeapIdle,
		HeapInuse:      stats.HeapInuse,
		HeapObjects:    stats.HeapObjects,
		StackInuse:     stats.StackInuse,
		StackSys:       stats.StackSys,
		MSpanInuse:     stats.MSpanInuse,
		MCacheInuse:    stats.MCacheInuse,
		NumGC:          stats.NumGC,
		GCPauseTotalNs: stats.PauseTotalNs,
	}
	if stats.NumGC > 0 {
		result.LastGCTime = stats.PauseEnd[(stats.NumGC+255)%256]
	}
	return result
}

func monitorInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown"
	}
	return hostname + ":" + strconv.Itoa(os.Getpid())
}
