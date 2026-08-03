package grpc

/*
 * @Author: lwnmengjing
 * @Date: 2021/6/2 4:30 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/2 4:30 下午
 */

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
	"github.com/prometheus/client_golang/prometheus/promauto"
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

// Option set options
type Option func(*Options)

// Options options
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
	maxConnectionAge         time.Duration
	maxConnectionAgeGrace    time.Duration
	maxConcurrentStreams     int
	maxMsgSize               int
	unaryServerInterceptors  []grpc.UnaryServerInterceptor
	streamServerInterceptors []grpc.StreamServerInterceptor
	ctx                      context.Context
	metrcsServer             *grpcprom.ServerMetrics
}

// WithContext set ContextOption
func WithContext(c context.Context) Option {
	return func(o *Options) {
		o.ctx = c
	}
}

// WithID set IDOption
func WithID(s string) Option {
	return func(o *Options) {
		o.id = s
	}
}

// WithDomain set DomainOption
func WithDomain(s string) Option {
	return func(o *Options) {
		o.domain = s
	}
}

// WithAddr set AddrOption
func WithAddr(s string) Option {
	return func(o *Options) {
		o.addr = s
	}
}

// WithStartedHook 设置启动回调函数
func WithStartedHook(f func()) Option {
	return func(o *Options) {
		o.startedHook = f
	}
}

// WithCert 设置cert
func WithCert(s string) Option {
	return func(o *Options) {
		o.certFile = s
	}
}

// WithKey 设置key
func WithKey(s string) Option {
	return func(o *Options) {
		o.keyFile = s
	}
}

// WithTLS set TlsOption
func WithTLS(tls *tls.Config) Option {
	return func(o *Options) {
		o.tls = tls
	}
}

// WithKeepAlive set KeepAliveOption
func WithKeepAlive(t time.Duration) Option {
	return func(o *Options) {
		o.keepAlive = t
	}
}

// WithTimeout set TimeoutOption
func WithTimeout(t time.Duration) Option {
	return func(o *Options) {
		o.keepAlive = t
	}
}

// WithMaxConnectionAge set MaxConnectionAgeOption
func WithMaxConnectionAge(t time.Duration) Option {
	return func(o *Options) {
		o.maxConnectionAge = t
	}
}

// WithMaxConnectionAgeGrace set MaxConnectionAgeGraceOption
func WithMaxConnectionAgeGrace(t time.Duration) Option {
	return func(o *Options) {
		o.maxConnectionAgeGrace = t
	}
}

// WithMaxConcurrentStreamsOption set MaxConcurrentStreamsOption
func WithMaxConcurrentStreamsOption(i int) Option {
	return func(o *Options) {
		o.maxConcurrentStreams = i
	}
}

// WithMaxMsgSizeOption set MaxMsgSizeOption
func WithMaxMsgSizeOption(i int) Option {
	return func(o *Options) {
		o.maxMsgSize = i
	}
}

// WithUnaryServerInterceptors set UnaryServerInterceptorsOption
func WithUnaryServerInterceptors(u ...grpc.UnaryServerInterceptor) Option {
	return func(o *Options) {
		if o.unaryServerInterceptors == nil {
			o.unaryServerInterceptors = make([]grpc.UnaryServerInterceptor, 0)
		}
		o.unaryServerInterceptors = append(o.unaryServerInterceptors, u...)
	}
}

// WithStreamServerInterceptors set StreamServerInterceptorsOption
func WithStreamServerInterceptors(u ...grpc.StreamServerInterceptor) Option {
	return func(o *Options) {
		if o.streamServerInterceptors == nil {
			o.streamServerInterceptors = make([]grpc.StreamServerInterceptor, 0)
		}
		o.streamServerInterceptors = append(o.streamServerInterceptors, u...)
	}
}

func defaultOptions() *Options {
	reg := prometheus.NewRegistry()
	reg.MustRegister(defaultMetricsServer)
	// Setup metric for panic recoveries.
	panicsTotal := promauto.With(reg).NewCounter(prometheus.CounterOpts{
		Name: "grpc_req_panics_recovered_total",
		Help: "Total number of gRPC requests recovered from internal panic.",
	})
	grpcPanicRecoveryHandler := func(p any) (err error) {
		panicsTotal.Inc()
		slog.Error("msg", "recovered from panic", "panic", p, "stack", debug.Stack())
		return status.Errorf(codes.Internal, "%s", p)
	}
	return &Options{
		addr:                  ":0",
		keepAlive:             defaultKeepAliveTime,
		timeout:               defaultConnectionIdleTime,
		maxConnectionAge:      infinity,
		maxConnectionAgeGrace: defaultMaxServerConnectionAgeGrace,
		maxConcurrentStreams:  defaultMaxConcurrentStreams,
		maxMsgSize:            defaultMaxMsgSize,
		metrcsServer:          defaultMetricsServer,
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

// customRecovery custom recovery
func customRecovery(id, domain string) recovery.RecoveryHandlerFunc {
	return func(p interface{}) (err error) {
		slog.Error(fmt.Sprintf("panic triggered: %v", p))
		return fmt.Errorf("%s[%s] panic triggered: %v", id, domain, p)
	}
}

// InterceptorLogger adapts slog logger to interceptor logger.
// This code is simple enough to be copied and not imported.
func InterceptorLogger(l *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		l.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}
