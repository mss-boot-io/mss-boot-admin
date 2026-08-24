//go:build darwin

package dev

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// processStartToken reads Darwin's kernel-owned microsecond-resolution process
// creation time. It does not include mutable command or process-title data.
func processStartToken(pid int) (string, error) {
	if pid <= 1 {
		return "", errProcessNotRunning
	}
	process, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.ENOENT) {
			return "", errProcessNotRunning
		}
		return "", fmt.Errorf("read process %d start identity: %w", pid, err)
	}
	if process == nil || process.Proc.P_pid != int32(pid) {
		return "", errProcessNotRunning
	}
	return processStartTokenFromSnapshot(processCreationSnapshot{
		Platform:    "darwin-sysctl",
		Seconds:     process.Proc.P_starttime.Sec,
		Nanoseconds: int64(process.Proc.P_starttime.Usec) * 1_000,
	})
}
