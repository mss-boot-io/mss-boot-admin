package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestWithTimeoutSetsConnectionTimeout(t *testing.T) {
	options := defaultOptions()
	keepAlive := options.keepAlive
	WithTimeout(7 * time.Second)(options)
	if options.timeout != 7*time.Second {
		t.Fatalf("timeout = %s, want 7s", options.timeout)
	}
	if options.keepAlive != keepAlive {
		t.Fatalf("WithTimeout changed keepalive from %s to %s", keepAlive, options.keepAlive)
	}
}

func TestMultipleServersCanShareMetricsRegistry(t *testing.T) {
	registry := prometheus.NewRegistry()
	first := New("first", WithPrometheusRegisterer(registry), WithReflection(false))
	second := New("second", WithPrometheusRegisterer(registry), WithReflection(false))
	if first.initErr != nil {
		t.Fatalf("first server initialization failed: %v", first.initErr)
	}
	if second.initErr != nil {
		t.Fatalf("second server initialization failed: %v", second.initErr)
	}
}

func TestStartBlocksUntilCancellation(t *testing.T) {
	started := make(chan struct{})
	srv := New(
		"test",
		WithAddr("127.0.0.1:0"),
		WithReflection(false),
		WithPrometheusRegisterer(prometheus.NewRegistry()),
		WithStartedHook(func() { close(started) }),
		WithShutdownTimeout(time.Second),
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("gRPC server did not start")
	}
	select {
	case err := <-done:
		t.Fatalf("Start returned before cancellation: %v", err)
	default:
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("graceful shutdown failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("gRPC server did not stop")
	}
}

func TestInvalidTLSConfigurationIsReportedByStart(t *testing.T) {
	srv := New(
		"tls",
		WithCert("missing.crt"),
		WithKey("missing.key"),
		WithPrometheusRegisterer(prometheus.NewRegistry()),
	)
	err := srv.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "load gRPC TLS credentials") {
		t.Fatalf("expected TLS credential error, got %v", err)
	}
}

func TestIncompleteTLSConfigurationIsRejected(t *testing.T) {
	srv := New(
		"tls",
		WithCert("server.crt"),
		WithPrometheusRegisterer(prometheus.NewRegistry()),
	)
	if err := srv.Start(context.Background()); !errors.Is(err, ErrIncompleteTLSConfiguration) {
		t.Fatalf("expected ErrIncompleteTLSConfiguration, got %v", err)
	}
}

func TestStartCanOnlyRunOnce(t *testing.T) {
	started := make(chan struct{})
	srv := New(
		"test",
		WithAddr("127.0.0.1:0"),
		WithReflection(false),
		WithPrometheusRegisterer(prometheus.NewRegistry()),
		WithStartedHook(func() { close(started) }),
	)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()
	<-started
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	if err := srv.Start(context.Background()); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("expected ErrAlreadyStarted, got %v", err)
	}
}
