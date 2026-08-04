package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"math"
	"runtime/debug"
	"time"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	infinity                           = time.Duration(math.MaxInt64)
	defaultMaxMsgSize                  = 4 << 20
	defaultMaxConcurrentStreams        = 100000
	defaultKeepAliveTime               = 30 * time.Second
	defaultConnectionIdleTime          = 10 * time.Second
	defaultMaxServerConnectionAgeGrace = 10 * time.Second
	defaultMiniKeepAliveTimeRate       = 2
	defaultShutdownTimeout             = 10 * time.Second
)

var (
	defaultMetricsServer = grpcprom.NewServerMetrics(grpcprom.WithServerCounterOptions())
	logTraceID           = func(ctx context.Context) logging.Fields {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return logging.Fields{"traceID", span.TraceID().String()}
		}
		return nil
	}
)

// Option configures a gRPC server.
type Option func(*Options)

// Options controls transport, middleware, observability, and lifecycle.
type Options struct {
	id                       string
	domain                   string
	addr                     string
	startedHook              func()
	certFile                 string
	keyFile                  string
	tls                      *tls.Config
	keepAlive                time.Duration
	timeout                  time.Duration
	shutdownTimeout          time.Duration
	maxConnectionAge         time.Duration
	maxConnectionAgeGrace    time.Duration
	maxConcurrentStreams     int
	maxMsgSize               int
	unaryServerInterceptors  []grpc.UnaryServerInterceptor
	streamServerInterceptors []grpc.StreamServerInterceptor
	ctx                      context.Context
	metricsServer            *grpcprom.ServerMetrics
	metricsRegisterer        prometheus.Registerer
	reflection               bool
}

// WithContext adds a secondary lifecycle context. The server stops when either
// this context or the context passed to Start is cancelled.
func WithContext(c context.Context) Option {
	return func(o *Options) {
		o.ctx = c
	}
}

// WithID sets a logical server ID.
func WithID(s string) Option {
	return func(o *Options) {
		o.id = s
	}
}

// WithDomain sets a logical server domain.
func WithDomain(s string) Option {
	return func(o *Options) {
		o.domain = s
	}
}

// WithAddr sets the listen address.
func WithAddr(s string) Option {
	return func(o *Options) {
		o.addr = s
	}
}

// WithStartedHook registers a callback invoked after the serve loop starts.
func WithStartedHook(f func()) Option {
	return func(o *Options) {
		o.startedHook = f
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

// WithTLS sets an in-memory TLS configuration. It takes precedence over
// certificate and key file options.
func WithTLS(config *tls.Config) Option {
	return func(o *Options) {
		o.tls = config
	}
}

// WithKeepAlive sets the server keepalive interval.
func WithKeepAlive(t time.Duration) Option {
	return func(o *Options) {
		o.keepAlive = t
	}
}

// WithTimeout sets the gRPC connection and keepalive timeout.
func WithTimeout(t time.Duration) Option {
	return func(o *Options) {
		o.timeout = t
	}
}

// WithShutdownTimeout limits graceful shutdown before Stop is used. A
// non-positive duration waits without a timeout.
func WithShutdownTimeout(t time.Duration) Option {
	return func(o *Options) {
		o.shutdownTimeout = t
	}
}

// WithReflection controls registration of the gRPC reflection service.
func WithReflection(enabled bool) Option {
	return func(o *Options) {
		o.reflection = enabled
	}
}

// WithPrometheusRegisterer changes the Prometheus registry used for the default
// gRPC metrics collector. A nil registerer disables collector registration.
func WithPrometheusRegisterer(registerer prometheus.Registerer) Option {
	return func(o *Options) {
		o.metricsRegisterer = registerer
	}
}

// WithMaxConnectionAge sets the maximum connection age.
func WithMaxConnectionAge(t time.Duration) Option {
	return func(o *Options) {
		o.maxConnectionAge = t
	}
}

// WithMaxConnectionAgeGrace sets the grace period after maximum connection age.
func WithMaxConnectionAgeGrace(t time.Duration) Option {
	return func(o *Options) {
		o.maxConnectionAgeGrace = t
	}
}

// WithMaxConcurrentStreamsOption sets the maximum concurrent stream count.
func WithMaxConcurrentStreamsOption(i int) Option {
	return func(o *Options) {
		o.maxConcurrentStreams = i
	}
}

// WithMaxMsgSizeOption sets the maximum received message size.
func WithMaxMsgSizeOption(i int) Option {
	return func(o *Options) {
		o.maxMsgSize = i
	}
}

// WithUnaryServerInterceptors appends unary server interceptors.
func WithUnaryServerInterceptors(interceptors ...grpc.UnaryServerInterceptor) Option {
	return func(o *Options) {
		o.unaryServerInterceptors = append(o.unaryServerInterceptors, interceptors...)
	}
}

// WithStreamServerInterceptors appends stream server interceptors.
func WithStreamServerInterceptors(interceptors ...grpc.StreamServerInterceptor) Option {
	return func(o *Options) {
		o.streamServerInterceptors = append(o.streamServerInterceptors, interceptors...)
	}
}

func defaultOptions() *Options {
	panicsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "grpc_req_panics_recovered_total",
		Help: "Total number of gRPC requests recovered from internal panic.",
	})
	grpcPanicRecoveryHandler := func(p any) error {
		panicsTotal.Inc()
		slog.Error("recovered from gRPC panic", "panic", p, "stack", string(debug.Stack()))
		return status.Error(codes.Internal, "internal server error")
	}

	return &Options{
		addr:                  ":0",
		keepAlive:             defaultKeepAliveTime,
		timeout:               defaultConnectionIdleTime,
		shutdownTimeout:       defaultShutdownTimeout,
		maxConnectionAge:      infinity,
		maxConnectionAgeGrace: defaultMaxServerConnectionAgeGrace,
		maxConcurrentStreams:  defaultMaxConcurrentStreams,
		maxMsgSize:            defaultMaxMsgSize,
		metricsServer:         defaultMetricsServer,
		metricsRegisterer:     prometheus.DefaultRegisterer,
		reflection:            true,
		unaryServerInterceptors: []grpc.UnaryServerInterceptor{
			logging.UnaryServerInterceptor(InterceptorLogger(slog.Default()), logging.WithFieldsFromContext(logTraceID)),
			defaultMetricsServer.UnaryServerInterceptor(),
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler)),
		},
		streamServerInterceptors: []grpc.StreamServerInterceptor{
			logging.StreamServerInterceptor(InterceptorLogger(slog.Default()), logging.WithFieldsFromContext(logTraceID)),
			defaultMetricsServer.StreamServerInterceptor(),
			recovery.StreamServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler)),
		},
	}
}

// customRecovery creates a domain-aware recovery handler for callers that need
// to preserve the historical error shape.
func customRecovery(id, domain string) recovery.RecoveryHandlerFunc {
	return func(p interface{}) error {
		slog.Error("gRPC panic triggered", "panic", p, "id", id, "domain", domain)
		return fmt.Errorf("%s[%s] panic triggered: %v", id, domain, p)
	}
}

// InterceptorLogger adapts slog.Logger to the middleware logging interface.
func InterceptorLogger(logger *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, level logging.Level, msg string, fields ...any) {
		logger.Log(ctx, slog.Level(level), msg, fields...)
	})
}
