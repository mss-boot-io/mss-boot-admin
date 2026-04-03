package service

import (
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"

	"github.com/mss-boot-io/mss-boot-admin/dto"
)

var startTime = time.Now()

type Monitor struct{}

func (e *Monitor) Monitor(ctx *gin.Context) (*dto.MonitorResponse, error) {
	resp := &dto.MonitorResponse{
		CPUInfo:   make([]dto.MonitorCPUInfo, 0),
		GoVersion: runtime.Version(),
		StartTime: startTime.Unix(),
		Uptime:    int64(time.Since(startTime).Seconds()),
		Network:   &dto.MonitorNetwork{},
		Runtime:   &dto.MonitorRuntime{},
	}
	var err error

	resp.CPULogicalCore, err = cpu.CountsWithContext(ctx, true)
	if err != nil {
		return nil, err
	}
	resp.CPUPhysicalCore, err = cpu.CountsWithContext(ctx, false)
	if err != nil {
		return nil, err
	}
	cpuInfo, err := cpu.InfoWithContext(ctx)
	physicalCPU := make([]cpu.InfoStat, 0)
	for i := range cpuInfo {
		var exist bool
		for j := range physicalCPU {
			if cpuInfo[i].PhysicalID == physicalCPU[j].PhysicalID {
				exist = true
				break
			}
		}
		if !exist {
			physicalCPU = append(physicalCPU, cpuInfo[i])
		}
	}
	percent, err := cpu.PercentWithContext(ctx, time.Second, false)
	if err != nil {
		return nil, err
	}
	if len(percent) > 0 {
		resp.CPUUsage = percent[0]
	}
	for i := range physicalCPU {
		idx := i
		if idx >= len(percent) {
			idx = len(percent) - 1
		}
		resp.CPUInfo = append(resp.CPUInfo, dto.MonitorCPUInfo{
			InfoStat:        physicalCPU[i],
			CPUUsagePercent: percent[idx] / 100,
		})
	}

	m, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	resp.MemoryTotal = m.Total
	resp.MemoryUsage = m.Used
	resp.MemoryUsagePercent = m.UsedPercent
	resp.MemoryAvailable = m.Available
	resp.MemoryFree = m.Free

	d, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}
	diskUsageStat, err := disk.Usage(d[0].Mountpoint)
	if err != nil {
		return nil, err
	}
	resp.DiskTotal = diskUsageStat.Total
	resp.DiskUsage = diskUsageStat.Used
	resp.DiskUsageGB = float64(diskUsageStat.Used) / 1024 / 1024 / 1024
	resp.DiskUsagePercent = diskUsageStat.UsedPercent

	netIO, err := net.IOCountersWithContext(ctx, false)
	if err == nil && len(netIO) > 0 {
		resp.Network.BytesSent = netIO[0].BytesSent
		resp.Network.BytesRecv = netIO[0].BytesRecv
		resp.Network.PacketsSent = netIO[0].PacketsSent
		resp.Network.PacketsRecv = netIO[0].PacketsRecv
		resp.Network.Errin = netIO[0].Errin
		resp.Network.Errout = netIO[0].Errout
		resp.Network.Dropin = netIO[0].Dropin
		resp.Network.Dropout = netIO[0].Dropout
	}

	conns, err := net.ConnectionsWithContext(ctx, "all")
	if err == nil {
		resp.Network.ConnectionCount = &dto.MonitorConnectionCount{}
		for _, c := range conns {
			resp.Network.ConnectionCount.Total++
			switch c.Status {
			case "ESTABLISHED":
				resp.Network.ConnectionCount.Established++
			case "LISTEN":
				resp.Network.ConnectionCount.Listen++
			case "TIME_WAIT":
				resp.Network.ConnectionCount.TimeWait++
			case "CLOSE_WAIT":
				resp.Network.ConnectionCount.CloseWait++
			}
		}
	}

	var mStats runtime.MemStats
	runtime.ReadMemStats(&mStats)
	resp.Runtime.Goroutines = runtime.NumGoroutine()
	resp.Runtime.HeapAlloc = mStats.HeapAlloc
	resp.Runtime.HeapSys = mStats.HeapSys
	resp.Runtime.HeapIdle = mStats.HeapIdle
	resp.Runtime.HeapInuse = mStats.HeapInuse
	resp.Runtime.HeapObjects = mStats.HeapObjects
	resp.Runtime.StackInuse = mStats.StackInuse
	resp.Runtime.StackSys = mStats.StackSys
	resp.Runtime.MSpanInuse = mStats.MSpanInuse
	resp.Runtime.MCacheInuse = mStats.MCacheInuse
	resp.Runtime.NumGC = mStats.NumGC
	resp.Runtime.GCPauseTotalNs = mStats.PauseTotalNs
	if len(mStats.PauseEnd) > 0 {
		resp.Runtime.LastGCTime = mStats.PauseEnd[(mStats.NumGC+255)%256]
	}

	return resp, nil
}
