package server

import (
	"testing"
	"time"
)

func TestDefaultGracefulShutdownTimeoutIsOuterDeadline(t *testing.T) {
	options := setDefaultOptions()
	if options.gracefulShutdownTimeout != 30*time.Second {
		t.Fatalf(
			"default graceful shutdown timeout = %s, want 30s",
			options.gracefulShutdownTimeout,
		)
	}
}
