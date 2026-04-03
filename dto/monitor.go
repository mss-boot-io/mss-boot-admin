package dto

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/net"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/23 23:45:37
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/23 23:45:37
 */

type MonitorResponse struct {
	// CPUPhysicalCore CPU物理核心数
	CPUPhysicalCore int `json:"cpuPhysicalCore"`
	// CPULogicalCore CPU逻辑核心数
	CPULogicalCore int `json:"cpuLogicalCore"`
	// CPUUsage CPU使用率百分比(保留2位小数)
	CPUUsage float64 `json:"cpuUsage"`
	// CPUInfo CPU信息
	CPUInfo []MonitorCPUInfo `json:"cpuInfo"`
	// MemoryTotal 内存总量
	MemoryTotal uint64 `json:"memoryTotal"`
	// MemoryUsage 内存使用量
	MemoryUsage uint64 `json:"memoryUsage"`
	// MemoryUsagePercent 内存使用率(保留2位小数)
	MemoryUsagePercent float64 `json:"memoryUsagePercent"`
	// MemoryAvailable 内存可用量
	MemoryAvailable uint64 `json:"memoryAvailable"`
	// MemoryFree 内存空闲量
	MemoryFree uint64 `json:"memoryFree"`
	// DiskTotal 磁盘总量(bytes)
	DiskTotal uint64 `json:"diskTotalBytes"`
	// DiskTotalGB 磁盘总量(GB, 保留2位小数)
	DiskTotalGB float64 `json:"diskTotal"`
	// DiskUsage 磁盘使用量(bytes)
	DiskUsage uint64 `json:"diskUsageBytes"`
	// DiskUsageGB 磁盘使用量(GB, 保留2位小数)
	DiskUsageGB float64 `json:"diskUsage"`
	// DiskUsagePercent 磁盘使用率(保留2位小数)
	DiskUsagePercent float64 `json:"diskUsagePercent"`
	// Network 网络信息
	Network *MonitorNetwork `json:"network,omitempty"`
	// Runtime 运行时信息
	Runtime *MonitorRuntime `json:"runtime,omitempty"`
	// GoVersion Go版本
	GoVersion string `json:"goVersion"`
	// StartTime 启动时间
	StartTime int64 `json:"startTime"`
	// Uptime 运行时长(秒)
	Uptime int64 `json:"uptime"`
}

type MonitorCPUInfo struct {
	cpu.InfoStat
	// CPUUsagePercent CPU使用率
	CPUUsagePercent float64 `json:"cpuUsagePercent"`
}

type MonitorNetwork struct {
	// BytesSent 发送字节数
	BytesSent uint64 `json:"bytesSent"`
	// BytesRecv 接收字节数
	BytesRecv uint64 `json:"bytesRecv"`
	// PacketsSent 发送包数
	PacketsSent uint64 `json:"packetsSent"`
	// PacketsRecv 接收包数
	PacketsRecv uint64 `json:"packetsRecv"`
	// Errin 接收错误数
	Errin uint64 `json:"errin"`
	// Errout 发送错误数
	Errout uint64 `json:"errout"`
	// Dropin 接收丢包数
	Dropin uint64 `json:"dropin"`
	// Dropout 发送丢包数
	Dropout uint64 `json:"dropout"`
	// Connections 连接数
	Connections []net.ConnectionStat `json:"connections,omitempty"`
	// ConnectionCount 连接数统计
	ConnectionCount *MonitorConnectionCount `json:"connectionCount,omitempty"`
}

type MonitorConnectionCount struct {
	// Established 已建立连接数
	Established int `json:"established"`
	// Listen 监听状态数
	Listen int `json:"listen"`
	// TimeWait TIME_WAIT状态数
	TimeWait int `json:"timeWait"`
	// CloseWait CLOSE_WAIT状态数
	CloseWait int `json:"closeWait"`
	// Total 总连接数
	Total int `json:"total"`
}

type MonitorRuntime struct {
	// Goroutines 协程数
	Goroutines int `json:"goroutines"`
	// HeapAlloc 堆分配字节数
	HeapAlloc uint64 `json:"heapAlloc"`
	// HeapSys 堆系统字节数
	HeapSys uint64 `json:"heapSys"`
	// HeapIdle 堆空闲字节数
	HeapIdle uint64 `json:"heapIdle"`
	// HeapInuse 堆使用字节数
	HeapInuse uint64 `json:"heapInuse"`
	// HeapObjects 堆对象数
	HeapObjects uint64 `json:"heapObjects"`
	// StackInuse 栈使用字节数
	StackInuse uint64 `json:"stackInuse"`
	// StackSys 栈系统字节数
	StackSys uint64 `json:"stackSys"`
	// MSpanInuse MSpan使用字节数
	MSpanInuse uint64 `json:"mSpanInuse"`
	// MCacheInuse MCache使用字节数
	MCacheInuse uint64 `json:"mCacheInuse"`
	// NumGC GC次数
	NumGC uint32 `json:"numGC"`
	// GCPauseTotalNs GC暂停总时间(纳秒)
	GCPauseTotalNs uint64 `json:"gcPauseTotalNs"`
	// LastGCTime 最后一次GC时间(纳秒时间戳)
	LastGCTime uint64 `json:"lastGCTime"`
}
