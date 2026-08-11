package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type storageTestSecretResolver map[SecretRef]string

func (r storageTestSecretResolver) Resolve(ctx context.Context, ref SecretRef) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	value, ok := r[ref]
	if !ok {
		return "", errors.New("secret unavailable")
	}
	return value, nil
}

var storageTestSecrets = storageTestSecretResolver{
	"env://S3_ACCESS_KEY": "access-key",
	"env://S3_SECRET_KEY": "secret-key",
	"env://S3_SESSION":    "session-token",
	"env://TLS_CA":        "not-a-pem-certificate",
	"env://TLS_CERT":      "not-a-pem-certificate",
	"env://TLS_KEY":       "not-a-pem-private-key",
}

func validS3StorageConfig() Storage {
	return Storage{S3: &S3StorageConfig{
		Endpoint:     "https://objects.example.test/",
		Region:       "test-region-1",
		Bucket:       "uploads",
		UsePathStyle: true,
		Credentials: S3CredentialsConfig{Static: &StaticCredentialRefs{
			AccessKeyRef:    "env://S3_ACCESS_KEY",
			SecretKeyRef:    "env://S3_SECRET_KEY",
			SessionTokenRef: "env://S3_SESSION",
		}},
	}}
}

func TestObjectStorageProfileRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		storage func(t *testing.T) Storage
	}{
		{name: "missing provider", storage: func(*testing.T) Storage { return Storage{} }},
		{name: "multiple providers", storage: func(t *testing.T) Storage {
			value := validS3StorageConfig()
			value.Local = &LocalStorageConfig{Root: t.TempDir()}
			return value
		}},
		{name: "empty local root", storage: func(*testing.T) Storage {
			return Storage{Local: &LocalStorageConfig{}}
		}},
		{name: "relative local root", storage: func(*testing.T) Storage {
			return Storage{Local: &LocalStorageConfig{Root: filepath.Join("data", "uploads")}}
		}},
		{name: "empty endpoint", storage: mutateValidS3(func(c *S3StorageConfig) { c.Endpoint = "" })},
		{name: "endpoint without scheme", storage: mutateValidS3(func(c *S3StorageConfig) { c.Endpoint = "objects.example.test" })},
		{name: "endpoint without hostname", storage: mutateValidS3(func(c *S3StorageConfig) { c.Endpoint = "https://:443" })},
		{name: "endpoint with credentials", storage: mutateValidS3(func(c *S3StorageConfig) { c.Endpoint = "https://user:pass@objects.example.test" })},
		{name: "endpoint with path", storage: mutateValidS3(func(c *S3StorageConfig) { c.Endpoint = "https://objects.example.test/api" })},
		{name: "endpoint with query", storage: mutateValidS3(func(c *S3StorageConfig) { c.Endpoint = "https://objects.example.test?bucket=uploads" })},
		{name: "endpoint with fragment", storage: mutateValidS3(func(c *S3StorageConfig) { c.Endpoint = "https://objects.example.test#uploads" })},
		{name: "http without explicit opt in", storage: mutateValidS3(func(c *S3StorageConfig) { c.Endpoint = "http://objects.example.test" })},
		{name: "insecure opt in on https", storage: mutateValidS3(func(c *S3StorageConfig) { c.TLS.AllowInsecureHTTP = true })},
		{name: "empty region", storage: mutateValidS3(func(c *S3StorageConfig) { c.Region = "" })},
		{name: "empty bucket", storage: mutateValidS3(func(c *S3StorageConfig) { c.Bucket = "" })},
		{name: "bucket with path", storage: mutateValidS3(func(c *S3StorageConfig) { c.Bucket = "uploads/private" })},
		{name: "bucket with uppercase", storage: mutateValidS3(func(c *S3StorageConfig) { c.Bucket = "Uploads" })},
		{name: "bucket too short", storage: mutateValidS3(func(c *S3StorageConfig) { c.Bucket = "ab" })},
		{name: "bucket too long", storage: mutateValidS3(func(c *S3StorageConfig) { c.Bucket = strings.Repeat("a", 64) })},
		{name: "bucket starts with dash", storage: mutateValidS3(func(c *S3StorageConfig) { c.Bucket = "-uploads" })},
		{name: "bucket ends with dash", storage: mutateValidS3(func(c *S3StorageConfig) { c.Bucket = "uploads-" })},
		{name: "bucket formatted as IP address", storage: mutateValidS3(func(c *S3StorageConfig) { c.Bucket = "127.0.0.1" })},
		{name: "bucket with adjacent dots", storage: mutateValidS3(func(c *S3StorageConfig) { c.Bucket = "uploads..private" })},
		{name: "bucket with dot dash adjacency", storage: mutateValidS3(func(c *S3StorageConfig) { c.Bucket = "uploads.-private" })},
		{name: "missing credential source", storage: mutateValidS3(func(c *S3StorageConfig) {
			c.Credentials = S3CredentialsConfig{}
		})},
		{name: "multiple credential sources", storage: mutateValidS3(func(c *S3StorageConfig) {
			c.Credentials.DefaultChain = &DefaultChainCredentials{}
		})},
		{name: "missing access key ref", storage: mutateValidS3(func(c *S3StorageConfig) {
			c.Credentials.Static.AccessKeyRef = ""
		})},
		{name: "raw secret is not a ref", storage: mutateValidS3(func(c *S3StorageConfig) {
			c.Credentials.Static.AccessKeyRef = "raw-access-key"
		})},
		{name: "unresolved secret ref", storage: mutateValidS3(func(c *S3StorageConfig) {
			c.Credentials.Static.AccessKeyRef = "env://MISSING_ACCESS_KEY"
		})},
		{name: "partial client certificate", storage: mutateValidS3(func(c *S3StorageConfig) {
			c.TLS.ClientCertificateRef = "env://TLS_CERTIFICATE"
		})},
		{name: "malformed custom CA", storage: mutateValidS3(func(c *S3StorageConfig) {
			c.TLS.CARef = "env://TLS_CA"
		})},
		{name: "malformed client keypair", storage: mutateValidS3(func(c *S3StorageConfig) {
			c.TLS.ClientCertificateRef = "env://TLS_CERT"
			c.TLS.ClientKeyRef = "env://TLS_KEY"
		})},
		{name: "tls settings with http endpoint", storage: mutateValidS3(func(c *S3StorageConfig) {
			c.Endpoint = "http://objects.example.test"
			c.TLS.AllowInsecureHTTP = true
			c.TLS.CARef = "env://TLS_CA"
		})},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storage := test.storage(t)
			profile, err := storage.Normalize(context.Background(), storageTestSecrets)
			if !errors.Is(err, ErrInvalidStorageConfiguration) {
				t.Fatalf("Normalize() error = %v, want ErrInvalidStorageConfiguration", err)
			}
			if profile != nil {
				t.Fatalf("Normalize() profile = %#v, want nil", profile)
			}
		})
	}
}

func mutateValidS3(mutate func(*S3StorageConfig)) func(*testing.T) Storage {
	return func(*testing.T) Storage {
		value := validS3StorageConfig()
		mutate(value.S3)
		return value
	}
}

func TestObjectStorageProfileBuildIsImmutable(t *testing.T) {
	t.Parallel()
	storage := validS3StorageConfig()
	before, err := json.Marshal(storage)
	if err != nil {
		t.Fatalf("marshal storage before Normalize: %v", err)
	}

	profile, err := storage.Normalize(context.Background(), storageTestSecrets)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if got := profile.Provider(); got != S3 {
		t.Fatalf("Provider() = %q, want %q", got, S3)
	}
	if root, ok := profile.LocalRoot(); ok || root != "" {
		t.Fatalf("LocalRoot() = (%q, %v), want (\"\", false)", root, ok)
	}
	assertStorageProfileFormattingRedactsSecrets(t, profile)

	first, err := profile.Build(context.Background())
	if err != nil {
		t.Fatalf("first Build() error = %v", err)
	}
	second, err := profile.Build(context.Background())
	if err != nil {
		t.Fatalf("second Build() error = %v", err)
	}
	if first != second {
		t.Fatal("Build() returned more than one owned handle")
	}
	if first.client == nil {
		t.Fatal("Build() S3 client is nil")
	}
	assertStorageProfileFormattingRedactsSecrets(t, profile)
	after, err := json.Marshal(storage)
	if err != nil {
		t.Fatalf("marshal storage after Build: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("Normalize/Build mutated caller configuration:\nbefore: %s\nafter:  %s", before, after)
	}
	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func assertStorageProfileFormattingRedactsSecrets(t *testing.T, profile *StorageProfile) {
	t.Helper()
	formatted := fmt.Sprintf("%v %+v %#v", profile, profile, profile)
	for _, secret := range storageTestSecrets {
		if strings.Contains(formatted, secret) {
			t.Fatalf("formatted StorageProfile contains secret material: %q", formatted)
		}
	}
}

func TestObjectStorageProfileCanceledBuildCanRetry(t *testing.T) {
	t.Parallel()
	profile, err := (Storage{Local: &LocalStorageConfig{Root: t.TempDir()}}).Normalize(context.Background(), nil)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := profile.Build(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("Build(canceled) error = %v, want context.Canceled", err)
	}
	handle, err := profile.Build(context.Background())
	if err != nil {
		t.Fatalf("Build(healthy) error = %v", err)
	}
	if err := handle.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestObjectStorageHandleUseCoversLocalAndS3(t *testing.T) {
	t.Parallel()

	localRoot := t.TempDir()
	localProfile, err := (Storage{Local: &LocalStorageConfig{Root: localRoot}}).Normalize(context.Background(), nil)
	if err != nil {
		t.Fatalf("normalize local: %v", err)
	}
	localHandle, err := localProfile.Build(context.Background())
	if err != nil {
		t.Fatalf("build local: %v", err)
	}
	if err := localHandle.Use(context.Background(), func(profile *StorageProfile, client *s3.Client) error {
		root, ok := profile.LocalRoot()
		if !ok || root != filepath.Clean(localRoot) {
			t.Fatalf("LocalRoot() = (%q, %v), want (%q, true)", root, ok, filepath.Clean(localRoot))
		}
		if client != nil {
			t.Fatal("local Use() received an S3 client")
		}
		return nil
	}); err != nil {
		t.Fatalf("local Use() error = %v", err)
	}

	s3Profile, err := validS3StorageConfig().Normalize(context.Background(), storageTestSecrets)
	if err != nil {
		t.Fatalf("normalize S3: %v", err)
	}
	s3Handle, err := s3Profile.Build(context.Background())
	if err != nil {
		t.Fatalf("build S3: %v", err)
	}
	if err := s3Handle.Use(context.Background(), func(profile *StorageProfile, client *s3.Client) error {
		if profile.Provider() != S3 || client == nil {
			t.Fatalf("S3 Use() = provider %q, client %p", profile.Provider(), client)
		}
		return nil
	}); err != nil {
		t.Fatalf("S3 Use() error = %v", err)
	}
	if err := localHandle.Close(context.Background()); err != nil {
		t.Fatalf("close local: %v", err)
	}
	if err := s3Handle.Close(context.Background()); err != nil {
		t.Fatalf("close S3: %v", err)
	}
}

func TestObjectStorageHandleCloseDrainsAndIsIdempotent(t *testing.T) {
	t.Parallel()
	var closeCalls atomic.Int32
	handle := newStorageHandle(
		&StorageProfile{provider: Local, localRoot: t.TempDir()},
		nil,
		func() { closeCalls.Add(1) },
	)
	entered := make(chan struct{})
	release := make(chan struct{})
	useDone := make(chan error, 1)
	go func() {
		useDone <- handle.Use(context.Background(), func(*StorageProfile, *s3.Client) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	closeDone := make(chan error, 1)
	go func() { closeDone <- handle.Close(context.Background()) }()
	waitForStorageHandleClosing(t, handle)
	if err := handle.Use(context.Background(), func(*StorageProfile, *s3.Client) error { return nil }); !errors.Is(err, ErrStorageHandleClosing) {
		t.Fatalf("Use() after Close started error = %v, want ErrStorageHandleClosing", err)
	}
	select {
	case err := <-closeDone:
		t.Fatalf("Close() returned before lease drained: %v", err)
	default:
	}
	close(release)
	if err := <-useDone; err != nil {
		t.Fatalf("leased Use() error = %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := handle.Close(context.Background()); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if got := closeCalls.Load(); got != 1 {
		t.Fatalf("owned transport close calls = %d, want 1", got)
	}
}

func TestObjectStorageHandleCloseTimeoutCanRetry(t *testing.T) {
	t.Parallel()
	var closeCalls atomic.Int32
	handle := newStorageHandle(
		&StorageProfile{provider: Local, localRoot: t.TempDir()},
		nil,
		func() { closeCalls.Add(1) },
	)
	entered := make(chan struct{})
	release := make(chan struct{})
	useDone := make(chan error, 1)
	go func() {
		useDone <- handle.Use(context.Background(), func(*StorageProfile, *s3.Client) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	closeCtx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := handle.Close(closeCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed Close() error = %v, want DeadlineExceeded", err)
	}
	if got := closeCalls.Load(); got != 0 {
		t.Fatalf("owned transport closed with active lease: %d", got)
	}
	if err := handle.Use(context.Background(), func(*StorageProfile, *s3.Client) error { return nil }); !errors.Is(err, ErrStorageHandleClosing) {
		t.Fatalf("Use() after timed-out Close error = %v, want ErrStorageHandleClosing", err)
	}

	close(release)
	if err := <-useDone; err != nil {
		t.Fatalf("leased Use() error = %v", err)
	}
	if err := handle.Close(context.Background()); err != nil {
		t.Fatalf("retry Close() error = %v", err)
	}
	if got := closeCalls.Load(); got != 1 {
		t.Fatalf("owned transport close calls = %d, want 1", got)
	}
}

func waitForStorageHandleClosing(t *testing.T, handle *StorageHandle) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		handle.mu.Lock()
		closing := handle.closing
		handle.mu.Unlock()
		if closing {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("storage handle did not enter closing state")
}
