//go:build !windows

package dev

import (
	"errors"
	"os/exec"
	"syscall"
)

func prepareProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func processAlive(pid int) bool {
	if pid <= 1 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func signalProcess(pid int, force bool) error {
	if pid <= 1 {
		return errors.New("refusing to signal an invalid process id")
	}
	signal := syscall.SIGTERM
	if force {
		signal = syscall.SIGKILL
	}

	// Services are started in a dedicated process group so child processes such
	// as `go run` and package-manager wrappers are terminated together.
	if err := syscall.Kill(-pid, signal); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if directErr := syscall.Kill(pid, signal); directErr != nil && !errors.Is(directErr, syscall.ESRCH) {
			return directErr
		}
	}
	return nil
}
