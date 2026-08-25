//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

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
	groupErr := syscall.Kill(-pid, signal)
	if groupErr == nil {
		return nil
	}
	directErr := syscall.Kill(pid, signal)
	if errors.Is(groupErr, syscall.ESRCH) && errors.Is(directErr, syscall.ESRCH) {
		return nil
	}
	// A direct signal cannot prove that descendants in the missing or
	// inaccessible process group were terminated, so retain the group error as
	// cleanup evidence even when the direct process accepted the signal.
	return errors.Join(groupErr, directErr)
}
