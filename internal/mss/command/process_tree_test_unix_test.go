//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package command

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"testing"
)

func processExistsForTest(pid int) bool {
	err := syscall.Kill(pid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false
	}
	if err != nil {
		return true
	}
	if runtime.GOOS == "linux" {
		data, readErr := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if errors.Is(readErr, os.ErrNotExist) {
			return false
		}
		if readErr == nil {
			state, _, ok := linuxProcessStateAndStartTimeForTest(data)
			if ok && processStateExitedForTest(state) {
				return false
			}
		}
	}
	return true
}

func TestLinuxProcessExitStates(t *testing.T) {
	for _, state := range []byte{'Z', 'X', 'x'} {
		if !processStateExitedForTest(state) {
			t.Errorf("state %q should be treated as exited", state)
		}
	}
	for _, state := range []byte{'R', 'S', 'D', 'T', 'I'} {
		if processStateExitedForTest(state) {
			t.Errorf("state %q should be treated as live", state)
		}
	}
}
