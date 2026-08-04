package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

var (
	// ErrAlreadyStarted is returned when a gRPC server is started twice.
	ErrAlreadyStarted = errors.New("gRPC server has already been started")
	// ErrIncompleteTLSConfiguration is returned when only one TLS file is set.
	ErrIncompleteTLSConfiguration = errors.New("both TLS certificate and key files are required")
)

// Server is a managed gRPC server.
type Server struct {
	name    string
	srv     *grpc.Server
	mux     sync.Mutex
	started bool
	options Options
	initErr error
}

// New creates a gRPC server and records initialization errors for Start.
func New(name string, options ...Option) *Server {
	s := &Server{name: name}
	s.Options(options...)
	s.NewServer()
	return s
}

// String returns the server name.
func (e *Server) String() string {
	return e.name
}

// Options resets defaults and applies options.
func (e *Server) Options(options ...Option) {
	e.options = *defaultOptions()
	for _, option := range options {
		if option != nil {
			option(&e.options)
		}
	}
}

// Server returns the underlying gRPC server.
func (e *Server) Server() *grpc.Server {
	return e.srv
}

// NewServer rebuilds the underlying gRPC server. Initialization failures are
// returned later by Start so the historical constructor remains source
// compatible.
func (e *Server) NewServer() {
	e.srv, e.initErr = e.buildServer()
}

// Register invokes a service registration callback when initialization
// succeeded. Start still reports any initialization failure.
func (e *Server) Register(register func(server *Server)) {
	if register != nil && e.srv != nil {
		register(e)
	}
}

// Start listens and blocks until the gRPC server exits or its lifecycle context
// is cancelled.
func (e *Server) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, stop := mergeContexts(ctx, e.options.ctx)
	defer stop()

	e.mux.Lock()
	if e.started {
		e.mux.Unlock()
		return ErrAlreadyStarted
	}
	if e.initErr != nil {
		e.mux.Unlock()
		return e.initErr
	}
	if e.srv == nil {
		e.mux.Unlock()
		return errors.New("gRPC server is not initialized")
	}

	listener, err := net.Listen("tcp", e.options.addr)
	if err != nil {
		e.mux.Unlock()
		return fmt.Errorf("gRPC server listening on %s failed: %w", e.options.addr, err)
	}
	e.started = true
	srv := e.srv
	e.mux.Unlock()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(listener)
	}()

	if e.options.startedHook != nil {
		e.options.startedHook()
	}
	server.PrintRunningInfo(listener.Addr().String(), "grpc")

	select {
	case err := <-serveErr:
		return normalizeServeError(err)
	case <-ctx.Done():
		shutdownErr := e.shutdownWithTimeout()
		return errors.Join(shutdownErr, normalizeServeError(<-serveErr))
	}
}

// Shutdown gracefully stops the server and forcefully stops it when ctx ends.
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

	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		srv.Stop()
		<-done
		return ctx.Err()
	}
}

func (e *Server) buildServer() (*grpc.Server, error) {
	options, err := e.initGrpcServerOptions()
	if err != nil {
		return nil, err
	}
	if err := registerCollector(e.options.metricsRegisterer, e.options.metricsServer); err != nil {
		return nil, fmt.Errorf("register gRPC metrics: %w", err)
	}

	srv := grpc.NewServer(options...)
	if e.options.metricsServer != nil {
		e.options.metricsServer.InitializeMetrics(srv)
	}
	if e.options.reflection {
		reflection.Register(srv)
	}
	return srv, nil
}

func (e *Server) initGrpcServerOptions() ([]grpc.ServerOption, error) {
	options := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ConnectionTimeout(e.options.timeout),
		grpc.ChainUnaryInterceptor(e.options.unaryServerInterceptors...),
		grpc.ChainStreamInterceptor(e.options.streamServerInterceptors...),
		grpc.MaxConcurrentStreams(uint32(e.options.maxConcurrentStreams)),
		grpc.MaxRecvMsgSize(e.options.maxMsgSize),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime: e.options.keepAlive / defaultMiniKeepAliveTimeRate,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:                  e.options.keepAlive,
			Timeout:               e.options.timeout,
			MaxConnectionAge:      e.options.maxConnectionAge,
			MaxConnectionAgeGrace: e.options.maxConnectionAgeGrace,
		}),
	}

	if e.options.tls != nil {
		options = append(options, grpc.Creds(credentials.NewTLS(e.options.tls.Clone())))
		return options, nil
	}
	if (e.options.certFile == "") != (e.options.keyFile == "") {
		return nil, ErrIncompleteTLSConfiguration
	}
	if e.options.certFile != "" {
		creds, err := credentials.NewServerTLSFromFile(e.options.certFile, e.options.keyFile)
		if err != nil {
			return nil, fmt.Errorf("load gRPC TLS credentials: %w", err)
		}
		options = append(options, grpc.Creds(creds))
	}
	return options, nil
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
	if err == nil || errors.Is(err, grpc.ErrServerStopped) {
		return nil
	}
	return err
}

func registerCollector(registerer prometheus.Registerer, collector prometheus.Collector) error {
	if registerer == nil || collector == nil {
		return nil
	}
	if err := registerer.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			return nil
		}
		return err
	}
	return nil
}

func mergeContexts(primary, secondary context.Context) (context.Context, context.CancelFunc) {
	if secondary == nil {
		return context.WithCancel(primary)
	}
	ctx, cancel := context.WithCancel(primary)
	stopSecondary := context.AfterFunc(secondary, cancel)
	return ctx, func() {
		stopSecondary()
		cancel()
	}
}
