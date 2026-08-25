//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package command

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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
			closing := strings.LastIndexByte(string(data), ')')
			if closing >= 0 && len(data) > closing+2 && data[closing+2] == 'Z' {
				return false
			}
		}
	}
	return true
}
