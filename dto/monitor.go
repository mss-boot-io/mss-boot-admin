package dto

import (
	"github.com/shirou/gopsutil/cpu"
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
	// CPUInfo CPU信息
	CPUInfo []MonitorCPUInfo `json:"cpuInfo"`
	// MemoryTotal 内存总量
	MemoryTotal uint64 `json:"memoryTotal"`
	// MemoryUsage 内存使用量
	MemoryUsage uint64 `json:"memoryUsage"`
	// MemoryUsagePercent 内存使用率
	MemoryUsagePercent float64 `json:"memoryUsagePercent"`
	// MemoryAvailable 内存可用量
	MemoryAvailable uint64 `json:"memoryAvailable"`
	// MemoryFree 内存空闲量
	MemoryFree uint64 `json:"memoryFree"`
	// DiskTotal 磁盘总量
	DiskTotal uint64 `json:"diskTotal"`
	// DiskUsage 磁盘使用量
	DiskUsage uint64 `json:"diskUsage"`
	// DiskUsagePercent 磁盘使用率
	DiskUsagePercent float64 `json:"diskUsagePercent"`
}

type MonitorCPUInfo struct {
	cpu.InfoStat
	// CPUUsagePercent CPU使用率
	CPUUsagePercent float64 `json:"cpuUsagePercent"`
}
