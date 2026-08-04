package listener

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestStartBlocksUntilCancellationAndRunsHooks(t *testing.T) {
	started := make(chan struct{})
	ended := make(chan struct{})
	runnable := New(
		WithAddr("127.0.0.1:0"),
		WithHandler(http.NewServeMux()),
		WithStartedHook(func() { close(started) }),
		WithEndHook(func() { close(ended) }),
		WithShutdownTimeout(time.Second),
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runnable.Start(ctx)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("HTTP listener did not start")
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
		t.Fatal("HTTP listener did not stop")
	}
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("shutdown hook was not invoked")
	}
}

func TestStartReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve address: %v", err)
	}
	defer listener.Close()

	runnable := New(
		WithAddr(listener.Addr().String()),
		WithHandler(http.NewServeMux()),
	)
	if err := runnable.Start(context.Background()); err == nil {
		t.Fatal("expected a listen error")
	}
}

func TestStartRejectsIncompleteTLSConfiguration(t *testing.T) {
	runnable := New(
		WithAddr("127.0.0.1:0"),
		WithHandler(http.NewServeMux()),
		WithCert("server.crt"),
	)
	if err := runnable.Start(context.Background()); !errors.Is(err, ErrIncompleteTLSConfiguration) {
		t.Fatalf("expected ErrIncompleteTLSConfiguration, got %v", err)
	}
}

func TestWithTimeoutConfiguresHTTPAndShutdownTimeouts(t *testing.T) {
	options := defaultOptions()
	WithTimeout(7)(options)
	want := 7 * time.Second
	for name, got := range map[string]time.Duration{
		"legacy":     options.timeout,
		"readHeader": options.readHeaderTimeout,
		"read":       options.readTimeout,
		"write":      options.writeTimeout,
		"idle":       options.idleTimeout,
		"shutdown":   options.shutdownTimeout,
	} {
		if got != want {
			t.Fatalf("%s timeout = %s, want %s", name, got, want)
		}
	}
}

func TestStartCanOnlyRunOnce(t *testing.T) {
	started := make(chan struct{})
	runnable := New(
		WithAddr("127.0.0.1:0"),
		WithHandler(http.NewServeMux()),
		WithStartedHook(func() { close(started) }),
	)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runnable.Start(ctx)
	}()
	<-started
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	if err := runnable.Start(context.Background()); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("expected ErrAlreadyStarted, got %v", err)
	}
}
