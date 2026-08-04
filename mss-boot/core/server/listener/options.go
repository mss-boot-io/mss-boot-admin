package listener

import (
	"net/http"
	"time"
)

// Option configures an HTTP listener.
type Option func(*Options)

// Options controls the HTTP server and its lifecycle.
type Options struct {
	name, addr, certFile, keyFile string
	handler                       http.Handler
	startedHook                   func()
	endHook                       func()
	timeout                       time.Duration
	readHeaderTimeout             time.Duration
	readTimeout                   time.Duration
	writeTimeout                  time.Duration
	idleTimeout                   time.Duration
	shutdownTimeout               time.Duration
	metrics                       bool
	healthz                       bool
	readyz                        bool
	pprof                         bool
}

func defaultOptions() *Options {
	return &Options{
		name:              "http",
		addr:              ":5000",
		timeout:           10 * time.Second,
		readHeaderTimeout: 10 * time.Second,
		readTimeout:       30 * time.Second,
		writeTimeout:      30 * time.Second,
		idleTimeout:       2 * time.Minute,
		shutdownTimeout:   10 * time.Second,
		handler:           http.DefaultServeMux,
	}
}

// WithName sets the listener name.
func WithName(name string) Option {
	return func(o *Options) {
		o.name = name
	}
}

// WithMetrics enables the Prometheus metrics route.
func WithMetrics(enable bool) Option {
	return func(o *Options) {
		o.metrics = enable
	}
}

// WithHealthz enables the liveness route.
func WithHealthz(enable bool) Option {
	return func(o *Options) {
		o.healthz = enable
	}
}

// WithReadyz enables the readiness route.
func WithReadyz(enable bool) Option {
	return func(o *Options) {
		o.readyz = enable
	}
}

// WithPprof enables pprof routes.
func WithPprof(enable bool) Option {
	return func(o *Options) {
		o.pprof = enable
	}
}

// WithEndHook registers a function invoked by http.Server during shutdown.
func WithEndHook(f func()) Option {
	return func(o *Options) {
		o.endHook = f
	}
}

// WithStartedHook registers a function invoked after the listening socket is
// ready and the serve loop has started.
func WithStartedHook(f func()) Option {
	return func(o *Options) {
		o.startedHook = f
	}
}

// WithAddr sets the listen address.
func WithAddr(s string) Option {
	return func(o *Options) {
		o.addr = s
	}
}

// WithHandler sets the HTTP handler.
func WithHandler(handler http.Handler) Option {
	return func(o *Options) {
		o.handler = handler
	}
}

// WithCert sets the TLS certificate path.
func WithCert(s string) Option {
	return func(o *Options) {
		o.certFile = s
	}
}

// WithKey sets the TLS private-key path.
func WithKey(s string) Option {
	return func(o *Options) {
		o.keyFile = s
	}
}

// WithTimeout preserves the legacy seconds-based timeout option while applying
// it to all HTTP I/O and graceful-shutdown timeouts.
func WithTimeout(seconds int) Option {
	return func(o *Options) {
		timeout := time.Second * time.Duration(seconds)
		o.timeout = timeout
		o.readHeaderTimeout = timeout
		o.readTimeout = timeout
		o.writeTimeout = timeout
		o.idleTimeout = timeout
		o.shutdownTimeout = timeout
	}
}

// WithReadHeaderTimeout limits the time used to read request headers.
func WithReadHeaderTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.readHeaderTimeout = timeout
	}
}

// WithReadTimeout limits the time used to read an entire request.
func WithReadTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.readTimeout = timeout
	}
}

// WithWriteTimeout limits the time used to write a response.
func WithWriteTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.writeTimeout = timeout
	}
}

// WithIdleTimeout limits how long keep-alive connections remain idle.
func WithIdleTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.idleTimeout = timeout
	}
}

// WithShutdownTimeout limits graceful shutdown before active connections are
// forcefully closed. A non-positive duration waits without a timeout.
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.shutdownTimeout = timeout
	}
}
