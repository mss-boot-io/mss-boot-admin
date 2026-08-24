//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package dev

import (
	"os"
	"syscall"
)

func terminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
