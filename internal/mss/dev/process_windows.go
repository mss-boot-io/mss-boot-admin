//go:build windows

package dev

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	createNewProcessGroup          = 0x00000200
	processQueryLimitedInformation = 0x00001000
	errorInvalidParameter          = syscall.Errno(87)
)

var (
	kernel32DevelopmentProcess = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess            = kernel32DevelopmentProcess.NewProc("OpenProcess")
	procGetProcessTimes        = kernel32DevelopmentProcess.NewProc("GetProcessTimes")
	procCloseHandle            = kernel32DevelopmentProcess.NewProc("CloseHandle")
)

func prepareProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}

func processAlive(pid int) bool {
	_, err := processStartToken(pid)
	return err == nil
}

// processStartToken uses the kernel creation timestamp associated with the
// process object. It changes when Windows reuses a PID.
func processStartToken(pid int) (string, error) {
	if pid <= 1 {
		return "", errProcessNotRunning
	}
	handle, _, callErr := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(uint32(pid)))
	if handle == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && (errno == errorInvalidParameter || errno == syscall.ERROR_NOT_FOUND) {
			return "", errProcessNotRunning
		}
		if callErr != nil && callErr != syscall.Errno(0) {
			return "", fmt.Errorf("open process %d for identity: %w", pid, callErr)
		}
		return "", fmt.Errorf("open process %d for identity", pid)
	}
	defer procCloseHandle.Call(handle)

	var creation syscall.Filetime
	var exit syscall.Filetime
	var kernel syscall.Filetime
	var user syscall.Filetime
	result, _, callErr := procGetProcessTimes.Call(
		handle,
		uintptr(unsafe.Pointer(&creation)),
		uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if result == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return "", fmt.Errorf("read process %d start identity: %w", pid, callErr)
		}
		return "", fmt.Errorf("read process %d start identity", pid)
	}
	return fmt.Sprintf("windows-filetime:%08x%08x", creation.HighDateTime, creation.LowDateTime), nil
}

func signalProcess(pid int, force bool) error {
	if pid <= 1 {
		return errors.New("refusing to signal an invalid process id")
	}
	if _, err := processStartToken(pid); errors.Is(err, errProcessNotRunning) {
		return nil
	} else if err != nil {
		return fmt.Errorf("verify process %d before taskkill: %w", pid, err)
	}
	args := []string{"/PID", strconv.Itoa(pid), "/T"}
	if force {
		args = append(args, "/F")
	}
	output, err := exec.Command("taskkill", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("taskkill pid %d: %w: %s", pid, err, strings.TrimSpace(string(output)))
	}
	return nil
}
