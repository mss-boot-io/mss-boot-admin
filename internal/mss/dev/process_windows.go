//go:build windows

package dev

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const createNewProcessGroup = 0x00000200

func prepareProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}

func processAlive(pid int) bool {
	if pid <= 1 {
		return false
	}
	output, err := exec.Command(
		"tasklist",
		"/FI", fmt.Sprintf("PID eq %d", pid),
		"/FO", "CSV",
		"/NH",
	).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), strconv.Itoa(pid))
}

func signalProcess(pid int, force bool) error {
	if pid <= 1 {
		return errors.New("refusing to signal an invalid process id")
	}
	if !processAlive(pid) {
		return nil
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
