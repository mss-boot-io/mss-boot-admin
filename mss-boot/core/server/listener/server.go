package listener

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"sync"

	ginPprof "github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// ErrAlreadyStarted is returned when an HTTP listener is started twice.
	ErrAlreadyStarted = errors.New("HTTP server has already been started")
	// ErrIncompleteTLSConfiguration is returned when only one TLS file is set.
	ErrIncompleteTLSConfiguration = errors.New("both TLS certificate and key files are required")
)

const (
	devHealthLaunchEnvironment = "MSS_DEV_HEALTH_NONCE"
	devHealthLaunchHeader      = "X-MSS-Dev-Launch"
)

// Server is a managed HTTP listener.
type Server struct {
	mux     sync.Mutex
	srv     *http.Server
	options Options
	started bool
}

// New creates an HTTP listener. It returns nil when no handler is configured,
// preserving the historical optional-listener behavior.
func New(opts ...Option) server.Runnable {
	s := &Server{}
	s.Options(opts...)
	if s.options.handler == nil {
		return nil
	}
	s.registerOperationalRoutes()
	return s
}

// Options resets the listener to defaults and applies options.
func (e *Server) Options(options ...Option) {
	e.options = *defaultOptions()
	for _, option := range options {
		if option != nil {
			option(&e.options)
		}
	}
}

// String returns the listener name.
func (e *Server) String() string {
	return e.options.name
}

// Start listens and blocks until the HTTP server exits or ctx is cancelled.
func (e *Server) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if (e.options.certFile == "") != (e.options.keyFile == "") {
		return ErrIncompleteTLSConfiguration
	}

	e.mux.Lock()
	if e.started {
		e.mux.Unlock()
		return ErrAlreadyStarted
	}
	listener, err := net.Listen("tcp", e.options.addr)
	if err != nil {
		e.mux.Unlock()
		return fmt.Errorf("HTTP server listening on %s failed: %w", e.options.addr, err)
	}

	e.srv = &http.Server{
		Handler:           e.options.handler,
		ReadHeaderTimeout: e.options.readHeaderTimeout,
		ReadTimeout:       e.options.readTimeout,
		WriteTimeout:      e.options.writeTimeout,
		IdleTimeout:       e.options.idleTimeout,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	e.started = true
	srv := e.srv
	e.mux.Unlock()

	serveErr := make(chan error, 1)
	go func() {
		if e.options.certFile == "" {
			serveErr <- srv.Serve(listener)
			return
		}
		serveErr <- srv.ServeTLS(listener, e.options.certFile, e.options.keyFile)
	}()

	if e.options.startedHook != nil {
		e.options.startedHook()
	}
	if e.options.endHook != nil {
		defer e.options.endHook()
	}
	server.PrintRunningInfo(listener.Addr().String(), "http")

	select {
	case err := <-serveErr:
		return normalizeServeError(err)
	case <-ctx.Done():
		shutdownErr := e.shutdownWithTimeout()
		if shutdownErr != nil {
			closeErr := srv.Close()
			shutdownErr = errors.Join(shutdownErr, normalizeServeError(closeErr))
		}
		return errors.Join(shutdownErr, normalizeServeError(<-serveErr))
	}
}

// Shutdown gracefully stops the HTTP server.
func (e *Server) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	e.mux.Lock()
	srv := e.srv
	e.mux.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

func (e *Server) shutdownWithTimeout() error {
	if e.options.shutdownTimeout <= 0 {
		return e.Shutdown(context.Background())
	}
	ctx, cancel := context.WithTimeout(context.Background(), e.options.shutdownTimeout)
	defer cancel()
	return e.Shutdown(ctx)
}

func normalizeServeError(err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (e *Server) registerOperationalRoutes() {
	switch handler := e.options.handler.(type) {
	case *http.ServeMux:
		if e.options.pprof && handler != http.DefaultServeMux {
			handler.HandleFunc("/debug/pprof/", pprof.Index)
			handler.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			handler.HandleFunc("/debug/pprof/profile", pprof.Profile)
			handler.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			handler.HandleFunc("/debug/pprof/trace", pprof.Trace)
		}
		if e.options.metrics {
			handler.Handle("/metrics", promhttp.Handler())
		}
		if e.options.healthz {
			handler.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
				setDevHealthLaunchHeader(w.Header())
				w.WriteHeader(http.StatusOK)
			})
		}
		if e.options.readyz {
			handler.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		}
		e.options.handler = handler
	case *gin.Engine:
		if e.options.pprof {
			ginPprof.Register(handler)
		}
		if e.options.metrics {
			handler.GET("/metrics", gin.WrapH(promhttp.Handler()))
		}
		if e.options.healthz {
			handler.GET("/healthz", func(c *gin.Context) {
				setDevHealthLaunchHeader(c.Writer.Header())
				c.AbortWithStatus(http.StatusOK)
			})
		}
		if e.options.readyz {
			handler.GET("/readyz", func(c *gin.Context) {
				c.AbortWithStatus(http.StatusOK)
			})
		}
	}
}

func setDevHealthLaunchHeader(header http.Header) {
	if nonce := os.Getenv(devHealthLaunchEnvironment); nonce != "" {
		header.Set(devHealthLaunchHeader, nonce)
	}
}
