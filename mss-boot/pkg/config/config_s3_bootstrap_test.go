package config

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
)

type s3BootstrapTestEntity struct {
	Name string `yaml:"name" json:"name"`
	Port int    `yaml:"port" json:"port"`
}

func (*s3BootstrapTestEntity) OnChange() {}

func TestS3BootstrapOwnedHandleClosesAndMissingOverlayIsOptional(t *testing.T) {
	var idleConnections atomic.Int32
	closed := make(chan struct{}, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, "application-test.") {
			writer.Header().Set("Content-Type", "application/xml")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`))
			return
		}
		writer.Header().Set("Content-Type", "application/octet-stream")
		_, _ = writer.Write([]byte("name: bootstrap\nport: 8443\n"))
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateIdle:
			idleConnections.Add(1)
		case http.StateClosed:
			select {
			case closed <- struct{}{}:
			default:
			}
		}
	}
	server.Start()
	defer server.Close()
	configureS3BootstrapTestEnvironment(t, server.URL)

	configuration := &s3BootstrapTestEntity{}
	if err := InitContext(
		context.Background(),
		configuration,
		source.WithProvider(source.S3),
		source.WithDir("config"),
	); err != nil {
		t.Fatalf("initialize S3 bootstrap source: %v", err)
	}
	if configuration.Name != "bootstrap" || configuration.Port != 8443 {
		t.Fatalf("bootstrap configuration = %#v", configuration)
	}
	if idleConnections.Load() == 0 {
		t.Fatal("S3 bootstrap connection never became idle")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("S3 bootstrap owned transport was not closed")
	}
}

func TestS3BootstrapOverlayFailureFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, "application-test.") {
			writer.Header().Set("Content-Type", "application/xml")
			writer.WriteHeader(http.StatusForbidden)
			_, _ = writer.Write([]byte(`<Error><Code>AccessDenied</Code><Message>stage overlay denied</Message></Error>`))
			return
		}
		_, _ = writer.Write([]byte("name: base\nport: 8080\n"))
	}))
	defer server.Close()
	configureS3BootstrapTestEnvironment(t, server.URL)

	configuration := &s3BootstrapTestEntity{}
	err := InitContext(
		context.Background(),
		configuration,
		source.WithProvider(source.S3),
		source.WithDir("config"),
	)
	if err == nil || errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "AccessDenied") {
		t.Fatalf("overlay outage error = %v", err)
	}
}

func TestS3BootstrapGenericNotFoundFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, "application-test.") {
			writer.Header().Set("Content-Type", "application/xml")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`<Error><Code>NotFound</Code><Message>upstream route missing</Message></Error>`))
			return
		}
		_, _ = writer.Write([]byte("name: base\nport: 8080\n"))
	}))
	defer server.Close()
	configureS3BootstrapTestEnvironment(t, server.URL)

	configuration := &s3BootstrapTestEntity{}
	err := InitContext(
		context.Background(),
		configuration,
		source.WithProvider(source.S3),
		source.WithDir("config"),
	)
	if err == nil || errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "NotFound") {
		t.Fatalf("generic 404 overlay error = %v", err)
	}
}

func TestS3BootstrapMalformedOverlayFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, "application-test.") {
			_, _ = writer.Write([]byte("port: [not-valid"))
			return
		}
		_, _ = writer.Write([]byte("name: base\nport: 8080\n"))
	}))
	defer server.Close()
	configureS3BootstrapTestEnvironment(t, server.URL)

	configuration := &s3BootstrapTestEntity{}
	if err := InitContext(
		context.Background(),
		configuration,
		source.WithProvider(source.S3),
		source.WithDir("config"),
	); err == nil {
		t.Fatal("malformed stage overlay was accepted")
	}
}

func TestS3BootstrapRejectsInvalidInsecureHTTPFlag(t *testing.T) {
	t.Setenv("s3_tls_allow_insecure_http", "sometimes")
	if _, err := s3BootstrapStorageFromEnvironment(); !errors.Is(err, ErrInvalidStorageConfiguration) {
		t.Fatalf("invalid allow-insecure flag error = %v", err)
	}
}

func configureS3BootstrapTestEnvironment(t *testing.T, endpoint string) {
	t.Helper()
	t.Setenv("STAGE", "test")
	t.Setenv("stage", "test")
	t.Setenv("s3_endpoint", endpoint)
	t.Setenv("s3_region", "test-region")
	t.Setenv("s3_bucket", "config-bucket")
	t.Setenv("s3_use_path_style", "true")
	t.Setenv("s3_tls_allow_insecure_http", "true")
	t.Setenv("s3_credential_source", "static")
	t.Setenv("s3_access_key_ref", "env://MSS_TEST_BOOTSTRAP_ACCESS_KEY")
	t.Setenv("s3_secret_key_ref", "env://MSS_TEST_BOOTSTRAP_SECRET_KEY")
	t.Setenv("s3_session_token_ref", "")
	t.Setenv("s3_tls_ca_ref", "")
	t.Setenv("s3_tls_client_certificate_ref", "")
	t.Setenv("s3_tls_client_key_ref", "")
	t.Setenv("MSS_TEST_BOOTSTRAP_ACCESS_KEY", "bootstrap-access")
	t.Setenv("MSS_TEST_BOOTSTRAP_SECRET_KEY", "bootstrap-secret")
}
