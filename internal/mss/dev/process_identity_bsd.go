//go:build freebsd || netbsd || openbsd || dragonfly

package dev

import (
	"errors"
)

// The supported release platforms expose a stable, sub-second kernel process
// creation identity (Linux, Darwin, and Windows). These BSD variants do not
// expose an equivalent portable Go API. A second-resolution `ps lstart` value
// can collide when a PID is reused and command output is mutable, so fail
// closed instead of treating either as authority to signal a process.
func processStartToken(pid int) (string, error) {
	if pid <= 1 {
		return "", errProcessNotRunning
	}
	if !processAlive(pid) {
		return "", errProcessNotRunning
	}
	return "", errors.New("stable high-resolution BSD process creation identity is unavailable")
}
