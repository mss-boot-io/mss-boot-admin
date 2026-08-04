package server

import (
	"os"
	"syscall"
	"time"
)

const defaultGracefulShutdownTimeout = 30 * time.Second

// Option configures a Server manager.
type Option func(*Options)

// Options controls process-signal handling and graceful shutdown.
type Options struct {
	gracefulShutdownTimeout time.Duration
	signals                 []os.Signal
}

func setDefaultOptions() Options {
	return Options{
		// The manager is the outer lifecycle deadline. Keep its default longer
		// than the built-in HTTP, gRPC, and task shutdown deadlines so adapters
		// can report their own timeout errors before the manager gives up.
		gracefulShutdownTimeout: defaultGracefulShutdownTimeout,
		signals:                 []os.Signal{os.Interrupt, syscall.SIGTERM},
	}
}

// WithGracefulShutdownTimeout sets the maximum time the manager waits for all
// runnables to return after cancellation. A non-positive duration waits without
// a timeout. This outer deadline should exceed each component's own shutdown
// timeout so component-specific errors can be collected.
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
