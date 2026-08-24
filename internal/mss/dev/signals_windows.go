//go:build windows

package dev

import "os"

func terminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
