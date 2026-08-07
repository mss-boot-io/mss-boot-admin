//go:build linux

package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	gopsutilcommon "github.com/shirou/gopsutil/v3/common"
)

const linuxConnectionTableMaxLineBytes = 1024 * 1024

type linuxConnectionTable struct {
	name       string
	minFields  int
	stateIndex int
}

var linuxConnectionTables = []linuxConnectionTable{
	{name: "tcp", minFields: 4, stateIndex: 3},
	{name: "tcp6", minFields: 4, stateIndex: 3},
	{name: "udp", minFields: 4, stateIndex: -1},
	{name: "udp6", minFields: 4, stateIndex: -1},
	{name: "unix", minFields: 7, stateIndex: -1},
}

// sampleMonitorConnectionCount reads the kernel socket tables directly on
// Linux. In particular, it must not call gopsutil Connections("all"), whose
// Linux implementation walks every /proc/<pid>/fd and performs work that is
// neither needed for aggregate counts nor promptly cancellable.
func sampleMonitorConnectionCount(ctx context.Context) (*dto.MonitorConnectionCount, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	procRoot := monitorHostProc(ctx)
	result := &dto.MonitorConnectionCount{}
	readTables := 0
	for _, table := range linuxConnectionTables {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path := filepath.Join(procRoot, "net", table.name)
		file, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("open Linux connection table %q: %w", path, err)
		}

		count, countErr := countLinuxConnectionTable(ctx, file, table)
		closeErr := file.Close()
		if countErr != nil {
			return nil, fmt.Errorf("read Linux connection table %q: %w", path, countErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close Linux connection table %q: %w", path, closeErr)
		}
		mergeMonitorConnectionCount(result, count)
		readTables++
	}
	if readTables == 0 {
		return nil, fmt.Errorf("no Linux connection tables found below %q", filepath.Join(procRoot, "net"))
	}
	return result, nil
}

func monitorHostProc(ctx context.Context) string {
	if env, ok := ctx.Value(gopsutilcommon.EnvKey).(gopsutilcommon.EnvMap); ok {
		if root := env[gopsutilcommon.HostProcEnvKey]; root != "" {
			return root
		}
	}
	if root := os.Getenv(string(gopsutilcommon.HostProcEnvKey)); root != "" {
		return root
	}
	return "/proc"
}

func countLinuxConnectionTable(
	ctx context.Context,
	reader io.Reader,
	table linuxConnectionTable,
) (*dto.MonitorConnectionCount, error) {
	result := &dto.MonitorConnectionCount{}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), linuxConnectionTableMaxLineBytes)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if lineNumber == 1 && (fields[0] == "sl" || fields[0] == "Num") {
			continue
		}
		if len(fields) < table.minFields {
			return nil, fmt.Errorf(
				"malformed %s record at line %d: got %d fields, want at least %d",
				table.name,
				lineNumber,
				len(fields),
				table.minFields,
			)
		}

		result.Total++
		if table.stateIndex < 0 {
			continue
		}
		switch strings.ToUpper(fields[table.stateIndex]) {
		case "01":
			result.Established++
		case "0A":
			result.Listen++
		case "06":
			result.TimeWait++
		case "08":
			result.CloseWait++
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func mergeMonitorConnectionCount(target, source *dto.MonitorConnectionCount) {
	target.Established += source.Established
	target.Listen += source.Listen
	target.TimeWait += source.TimeWait
	target.CloseWait += source.CloseWait
	target.Total += source.Total
}
