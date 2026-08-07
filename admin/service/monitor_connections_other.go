//go:build !linux

package service

import (
	"context"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	psnet "github.com/shirou/gopsutil/v3/net"
)

func sampleMonitorConnectionCount(ctx context.Context) (*dto.MonitorConnectionCount, error) {
	// Keep the established cross-platform implementation as a compatibility
	// fallback. Linux has a dedicated bounded collector because its gopsutil
	// implementation otherwise traverses every process file-descriptor table.
	connections, err := psnet.ConnectionsWithContext(ctx, "all")
	if err != nil {
		return nil, err
	}
	result := &dto.MonitorConnectionCount{}
	for _, connection := range connections {
		result.Total++
		switch connection.Status {
		case "ESTABLISHED":
			result.Established++
		case "LISTEN":
			result.Listen++
		case "TIME_WAIT":
			result.TimeWait++
		case "CLOSE_WAIT":
			result.CloseWait++
		}
	}
	return result, nil
}
