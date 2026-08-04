package server

import (
	"os"
	"syscall"
	"time"
)

// Option configures a Server manager.
type Option func(*Options)

// Options controls process-signal handling and graceful shutdown.
type Options struct {
	gracefulShutdownTimeout time.Duration
	signals                 []os.Signal
}

func setDefaultOptions() Options {
	return Options{
		gracefulShutdownTimeout: 5 * time.Second,
		signals:                 []os.Signal{os.Interrupt, syscall.SIGTERM},
	}
}

// WithGracefulShutdownTimeout sets the maximum time the manager waits for all
// runnables to return after cancellation. A non-positive duration waits without
// a timeout.
func WithGracefulShutdownTimeout(timeout time.Duration) Option {
	return func(options *Options) {
		options.gracefulShutdownTimeout = timeout
	}
}

// WithSignals replaces the operating-system signals handled by the manager.
// Passing no signals disables signal handling.
func WithSignals(signals ...os.Signal) Option {
	return func(options *Options) {
		options.signals = append([]os.Signal(nil), signals...)
	}
}

// WithoutSignalHandling leaves operating-system signal ownership to the
// embedding application. Context cancellation still stops all runnables.
func WithoutSignalHandling() Option {
	return WithSignals()
}
