package listener

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestHealthzEchoesScopedDevLaunchNonce(t *testing.T) {
	const nonce = "launch-nonce-for-test"
	t.Setenv(devHealthLaunchEnvironment, nonce)

	for name, handler := range map[string]http.Handler{
		"serve-mux": http.NewServeMux(),
		"gin":       gin.New(),
	} {
		t.Run(name, func(t *testing.T) {
			runnable := New(WithHandler(handler), WithHealthz(true))
			listener, ok := runnable.(*Server)
			if !ok {
				t.Fatalf("listener type = %T, want *Server", runnable)
			}
			request, err := http.NewRequest(http.MethodGet, "/healthz", nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			response := &recordingResponseWriter{header: make(http.Header)}
			listener.options.handler.ServeHTTP(response, request)
			if response.status != http.StatusOK {
				t.Fatalf("health status = %d, want %d", response.status, http.StatusOK)
			}
			if got := response.header.Get(devHealthLaunchHeader); got != nonce {
				t.Fatalf("%s = %q, want %q", devHealthLaunchHeader, got, nonce)
			}
		})
	}
}

func TestHealthzOmitsDevLaunchHeaderWithoutScopedNonce(t *testing.T) {
	t.Setenv(devHealthLaunchEnvironment, "")
	handler := http.NewServeMux()
	runnable := New(WithHandler(handler), WithHealthz(true))
	request, err := http.NewRequest(http.MethodGet, "/healthz", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	response := &recordingResponseWriter{header: make(http.Header)}
	runnable.(*Server).options.handler.ServeHTTP(response, request)
	if got := response.header.Get(devHealthLaunchHeader); got != "" {
		t.Fatalf("%s = %q, want empty", devHealthLaunchHeader, got)
	}
}

type recordingResponseWriter struct {
	header http.Header
	status int
}

func (writer *recordingResponseWriter) Header() http.Header { return writer.header }
func (writer *recordingResponseWriter) Write(payload []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	return len(payload), nil
}
func (writer *recordingResponseWriter) WriteHeader(status int) { writer.status = status }

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
