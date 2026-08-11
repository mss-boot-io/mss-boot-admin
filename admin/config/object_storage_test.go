package config

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	frameworkconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config"
)

func TestObjectStorageSingleClientOwnedClose(t *testing.T) {
	t.Setenv("MSS_TEST_OBJECT_ACCESS_KEY", "test-access-key")
	t.Setenv("MSS_TEST_OBJECT_SECRET_KEY", "test-secret-key")
	profile, err := (frameworkconfig.Storage{S3: &frameworkconfig.S3StorageConfig{
		Endpoint:     "http://127.0.0.1:9000",
		Region:       "test-region",
		Bucket:       "uploads",
		UsePathStyle: true,
		TLS:          frameworkconfig.S3TLSConfig{AllowInsecureHTTP: true},
		Credentials: frameworkconfig.S3CredentialsConfig{Static: &frameworkconfig.StaticCredentialRefs{
			AccessKeyRef: frameworkconfig.SecretRef("env://MSS_TEST_OBJECT_ACCESS_KEY"),
			SecretKeyRef: frameworkconfig.SecretRef("env://MSS_TEST_OBJECT_SECRET_KEY"),
		}},
	}}).Normalize(context.Background(), frameworkconfig.EnvSecretResolver{})
	if err != nil {
		t.Fatalf("normalize S3 profile: %v", err)
	}
	handle, err := profile.Build(context.Background())
	if err != nil {
		t.Fatalf("build S3 handle: %v", err)
	}
	again, err := profile.Build(context.Background())
	if err != nil || again != handle {
		t.Fatalf("repeat build handle=%p err=%v, want %p", again, err, handle)
	}

	configuration := &Config{}
	if err := configuration.InstallObjectStorage(handle, nil, ""); err != nil {
		t.Fatalf("install S3 handle: %v", err)
	}
	leaseStarted := make(chan struct{})
	releaseLease := make(chan struct{})
	leaseDone := make(chan error, 1)
	go func() {
		leaseDone <- configuration.WithObjectStorage(context.Background(), func(lease ObjectStorageLease) error {
			if lease.Profile != profile || lease.S3Client == nil || lease.LocalURLPrefix != "" {
				return errors.New("unexpected S3 lease")
			}
			close(leaseStarted)
			<-releaseLease
			return nil
		})
	}()
	<-leaseStarted

	shortContext, cancel := context.WithCancel(context.Background())
	cancel()
	if err := configuration.CloseContext(shortContext); !errors.Is(err, context.Canceled) {
		t.Fatalf("close with active lease error = %v", err)
	}
	if err := configuration.WithObjectStorage(context.Background(), func(ObjectStorageLease) error {
		return errors.New("lease unexpectedly admitted during close")
	}); !errors.Is(err, ErrObjectStorageUnavailable) {
		t.Fatalf("new lease while closing error = %v", err)
	}

	close(releaseLease)
	if err := <-leaseDone; err != nil {
		t.Fatalf("active lease error: %v", err)
	}
	closeContext, closeCancel := context.WithTimeout(context.Background(), time.Second)
	defer closeCancel()
	if err := configuration.CloseContext(closeContext); err != nil {
		t.Fatalf("retry close after drain: %v", err)
	}
	if err := configuration.CloseContext(closeContext); err != nil {
		t.Fatalf("idempotent close: %v", err)
	}
	if err := configuration.WithObjectStorage(context.Background(), func(ObjectStorageLease) error { return nil }); !errors.Is(err, ErrObjectStorageUnavailable) {
		t.Fatalf("lease after close error = %v", err)
	}
}

func TestObjectStorageConcurrentCloseIsIdempotent(t *testing.T) {
	rootName := t.TempDir()
	profile, err := (frameworkconfig.Storage{
		Local: &frameworkconfig.LocalStorageConfig{Root: rootName},
	}).Normalize(context.Background(), nil)
	if err != nil {
		t.Fatalf("normalize Local profile: %v", err)
	}
	handle, err := profile.Build(context.Background())
	if err != nil {
		t.Fatalf("build Local handle: %v", err)
	}
	root, err := os.OpenRoot(rootName)
	if err != nil {
		t.Fatalf("open Local root: %v", err)
	}
	configuration := &Config{}
	if err := configuration.InstallObjectStorage(handle, root, "/objects"); err != nil {
		_ = root.Close()
		t.Fatalf("install Local owner: %v", err)
	}

	const closers = 8
	var wait sync.WaitGroup
	errorsSeen := make(chan error, closers)
	for range closers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsSeen <- configuration.CloseContext(context.Background())
		}()
	}
	wait.Wait()
	close(errorsSeen)
	for closeErr := range errorsSeen {
		if closeErr != nil {
			t.Fatalf("concurrent CloseContext error: %v", closeErr)
		}
	}
}

func TestObjectStorageCloseDoesNotDeadlockReentrantLease(t *testing.T) {
	rootName := t.TempDir()
	profile, err := (frameworkconfig.Storage{
		Local: &frameworkconfig.LocalStorageConfig{Root: rootName},
	}).Normalize(context.Background(), nil)
	if err != nil {
		t.Fatalf("normalize Local profile: %v", err)
	}
	handle, err := profile.Build(context.Background())
	if err != nil {
		t.Fatalf("build Local handle: %v", err)
	}
	root, err := os.OpenRoot(rootName)
	if err != nil {
		t.Fatalf("open Local root: %v", err)
	}
	configuration := &Config{}
	if err := configuration.InstallObjectStorage(handle, root, "/objects"); err != nil {
		_ = root.Close()
		t.Fatalf("install Local owner: %v", err)
	}

	entered := make(chan struct{})
	tryNested := make(chan struct{})
	outerDone := make(chan error, 1)
	go func() {
		outerDone <- configuration.WithObjectStorage(context.Background(), func(ObjectStorageLease) error {
			close(entered)
			<-tryNested
			deadline := time.NewTimer(time.Second)
			defer deadline.Stop()
			for {
				nestedErr := configuration.WithObjectStorage(context.Background(), func(ObjectStorageLease) error { return nil })
				if errors.Is(nestedErr, ErrObjectStorageUnavailable) {
					return nil
				}
				if nestedErr != nil {
					return nestedErr
				}
				select {
				case <-deadline.C:
					return errors.New("storage owner did not reject nested lease during close")
				default:
				}
			}
		})
	}()
	<-entered
	closeDone := make(chan error, 1)
	go func() {
		closeContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		closeDone <- configuration.CloseContext(closeContext)
	}()
	close(tryNested)
	if err := <-outerDone; err != nil {
		t.Fatalf("outer storage lease error: %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("close with reentrant lease error: %v", err)
	}
}
