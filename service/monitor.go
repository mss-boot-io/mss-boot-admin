package service

import (
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/dto"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/23 23:44:53
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/23 23:44:53
 */

type Monitor struct{}

func (e *Monitor) Monitor(ctx *gin.Context) (*dto.MonitorResponse, error) {
	resp := &dto.MonitorResponse{
		CPUInfo: make([]dto.MonitorCPUInfo, 0),
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
	// cpu使用率
	percent, err := cpu.PercentWithContext(ctx, time.Second, false)
	if err != nil {
		return nil, err
	}
	for i := range physicalCPU {
		resp.CPUInfo = append(resp.CPUInfo, dto.MonitorCPUInfo{
			InfoStat:        physicalCPU[i],
			CPUUsagePercent: percent[i] / 100,
		})
	}
	// 内存
	m, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	resp.MemoryTotal = m.Total
	resp.MemoryUsage = m.Used
	resp.MemoryUsagePercent = m.UsedPercent / 100
	resp.MemoryAvailable = m.Available
	resp.MemoryFree = m.Free

	d, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}
	// 只计算总容量
	diskUsageStat, err := disk.Usage(d[0].Mountpoint)
	if err != nil {
		return nil, err
	}
	resp.DiskTotal = diskUsageStat.Total
	resp.DiskUsage = diskUsageStat.Used
	resp.DiskUsagePercent = diskUsageStat.UsedPercent

	return resp, nil
}
