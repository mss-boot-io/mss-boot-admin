package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	adminconfig "github.com/mss-boot-io/mss-boot-admin/admin/config"
	frameworkconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config"
)

type failingUploadPolicySnapshot struct{ err error }

type legacyUploadPolicy map[string]string

func (l legacyUploadPolicy) SetAppConfig(*gin.Context, string, bool, string) error { return nil }
func (l legacyUploadPolicy) GetAppConfig(_ *gin.Context, key string) (string, bool) {
	value, ok := l[key]
	return value, ok
}

func (*failingUploadPolicySnapshot) SetAppConfig(*gin.Context, string, bool, string) error {
	return nil
}
func (*failingUploadPolicySnapshot) GetAppConfig(*gin.Context, string) (string, bool) {
	panic("per-key policy reads must not be used when snapshot support is available")
}
func (f *failingUploadPolicySnapshot) GetAppConfigSnapshot(*gin.Context, ...string) (map[string]string, error) {
	return nil, f.err
}

func TestStorageUnknownProviderFailsClosed(t *testing.T) {
	setStorageProviderPolicy(t)
	if _, err := (frameworkconfig.Storage{}).Normalize(context.Background(), nil); !errors.Is(err, frameworkconfig.ErrInvalidStorageConfiguration) {
		t.Fatalf("empty/unknown provider error = %v", err)
	}
	root := filepath.Join(t.TempDir(), "objects")
	storage := &Storage{useObjectStorage: func(context.Context, func(adminconfig.ObjectStorageLease) error) error {
		return adminconfig.ErrObjectStorageUnavailable
	}}
	result, err := uploadStorageText(t, storage, "unknown.txt", []byte("must-not-write"))
	if !errors.Is(err, ErrStorageUnavailable) || result != nil {
		t.Fatalf("unknown provider result=%#v err=%v", result, err)
	}
	assertNoFiles(t, root)
}

func TestStorageUnavailableBackendDoesNotFallbackLocal(t *testing.T) {
	setStorageProviderPolicy(t)
	root := filepath.Join(t.TempDir(), "would-have-been-local")
	backendErr := errors.New("backend unavailable")
	storage := &Storage{useObjectStorage: func(context.Context, func(adminconfig.ObjectStorageLease) error) error {
		return errors.Join(adminconfig.ErrObjectStorageUnavailable, backendErr)
	}}
	result, err := uploadStorageText(t, storage, "fallback.txt", []byte("must-not-write"))
	if !errors.Is(err, ErrStorageUnavailable) || !errors.Is(err, backendErr) || result != nil {
		t.Fatalf("unavailable backend result=%#v err=%v", result, err)
	}
	assertNoFiles(t, root)
}

func TestStorageInvalidCredentialsFailBeforeWrite(t *testing.T) {
	setStorageProviderPolicy(t)
	t.Setenv("MSS_TEST_STORAGE_ACCESS_KEY", "")
	t.Setenv("MSS_TEST_STORAGE_SECRET_KEY", "")
	_, err := (frameworkconfig.Storage{S3: &frameworkconfig.S3StorageConfig{
		Endpoint: "http://127.0.0.1:9000",
		Region:   "test",
		Bucket:   "uploads",
		TLS:      frameworkconfig.S3TLSConfig{AllowInsecureHTTP: true},
		Credentials: frameworkconfig.S3CredentialsConfig{Static: &frameworkconfig.StaticCredentialRefs{
			AccessKeyRef: frameworkconfig.SecretRef("env://MSS_TEST_STORAGE_ACCESS_KEY"),
			SecretKeyRef: frameworkconfig.SecretRef("env://MSS_TEST_STORAGE_SECRET_KEY"),
		}},
	}}).Normalize(context.Background(), frameworkconfig.EnvSecretResolver{})
	if !errors.Is(err, frameworkconfig.ErrInvalidStorageConfiguration) {
		t.Fatalf("invalid credential profile error = %v", err)
	}
	root := filepath.Join(t.TempDir(), "objects")
	storage := &Storage{useObjectStorage: func(context.Context, func(adminconfig.ObjectStorageLease) error) error {
		return adminconfig.ErrObjectStorageUnavailable
	}}
	if result, uploadErr := uploadStorageText(t, storage, "credentials.txt", []byte("must-not-write")); !errors.Is(uploadErr, ErrStorageUnavailable) || result != nil {
		t.Fatalf("invalid credentials result=%#v err=%v", result, uploadErr)
	}
	assertNoFiles(t, root)
}

func TestStorageValidS3RemainsUnavailableBeforePut(t *testing.T) {
	setStorageProviderPolicy(t)
	var requests atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer backend.Close()
	t.Setenv("MSS_TEST_STORAGE_ACCESS_KEY", "test-access")
	t.Setenv("MSS_TEST_STORAGE_SECRET_KEY", "test-secret")
	profile, err := (frameworkconfig.Storage{S3: &frameworkconfig.S3StorageConfig{
		Endpoint:     backend.URL,
		Region:       "test-region",
		Bucket:       "uploads",
		UsePathStyle: true,
		TLS:          frameworkconfig.S3TLSConfig{AllowInsecureHTTP: true},
		Credentials: frameworkconfig.S3CredentialsConfig{Static: &frameworkconfig.StaticCredentialRefs{
			AccessKeyRef: frameworkconfig.SecretRef("env://MSS_TEST_STORAGE_ACCESS_KEY"),
			SecretKeyRef: frameworkconfig.SecretRef("env://MSS_TEST_STORAGE_SECRET_KEY"),
		}},
	}}).Normalize(context.Background(), frameworkconfig.EnvSecretResolver{})
	if err != nil {
		t.Fatalf("normalize valid S3 profile: %v", err)
	}
	handle, err := profile.Build(context.Background())
	if err != nil {
		t.Fatalf("build valid S3 profile: %v", err)
	}
	t.Cleanup(func() {
		if err := handle.Close(context.Background()); err != nil {
			t.Errorf("close valid S3 handle: %v", err)
		}
	})
	storage := &Storage{useObjectStorage: func(ctx context.Context, operation func(adminconfig.ObjectStorageLease) error) error {
		return handle.Use(ctx, func(profile *frameworkconfig.StorageProfile, client *s3.Client) error {
			return operation(adminconfig.ObjectStorageLease{Profile: profile, S3Client: client})
		})
	}}
	result, uploadErr := uploadStorageText(t, storage, "s3.txt", []byte("must-not-put"))
	if !errors.Is(uploadErr, ErrStorageUnavailable) || result != nil {
		t.Fatalf("pre-Put S3 result=%#v err=%v", result, uploadErr)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("S3 requests before ObjectStore contract = %d, want 0", got)
	}
}

func TestStorageLocalProductionRequiresExplicitDelivery(t *testing.T) {
	setStorageProviderPolicy(t)
	root := filepath.Join(t.TempDir(), "objects")
	profile := mustLocalStorageProfile(t, root)
	storage := storageUsingLease(adminconfig.ObjectStorageLease{
		Profile:   profile,
		LocalRoot: openLocalTestRoot(t, root),
	})
	result, err := uploadStorageText(t, storage, "production.txt", []byte("must-not-write"))
	if !errors.Is(err, ErrStorageUnavailable) || result != nil {
		t.Fatalf("production Local result=%#v err=%v", result, err)
	}
	assertNoFiles(t, root)
}

func TestStorageLocalSuccessURLIsActuallyServed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := center.GetAppConfig()
	center.SetAppConfig(&fakeAppConfig{values: map[string]string{
		maxSizeConfigKey:      "1024",
		allowedTypesConfigKey: "text/plain",
	}})
	t.Cleanup(func() { center.SetAppConfig(previous) })

	root := filepath.Join(t.TempDir(), "objects")
	profile := mustLocalStorageProfile(t, root)
	storage := storageUsingLease(adminconfig.ObjectStorageLease{
		Profile:        profile,
		LocalRoot:      openLocalTestRoot(t, root),
		LocalURLPrefix: "/objects",
	})
	want := []byte("served-after-router-restart")
	result, err := uploadStorageText(t, storage, "served.txt", want)
	if err != nil {
		t.Fatalf("upload explicit Local object: %v", err)
	}
	if result == nil || result.URL == "" {
		t.Fatalf("upload result = %#v", result)
	}

	for restart := 0; restart < 2; restart++ {
		engine := gin.New()
		engine.Static("/objects", root)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, result.URL, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("restart %d delivery status=%d body=%q", restart, recorder.Code, recorder.Body.String())
		}
		if sha256.Sum256(recorder.Body.Bytes()) != sha256.Sum256(want) {
			t.Fatalf("restart %d delivered different bytes", restart)
		}
	}
}

func TestStoragePolicySnapshotFailureFailsClosed(t *testing.T) {
	previous := center.GetAppConfig()
	center.SetAppConfig(&failingUploadPolicySnapshot{err: errors.New("database unavailable")})
	t.Cleanup(func() { center.SetAppConfig(previous) })
	root := filepath.Join(t.TempDir(), "objects")
	storage := storageUsingLease(adminconfig.ObjectStorageLease{
		Profile:        mustLocalStorageProfile(t, root),
		LocalRoot:      openLocalTestRoot(t, root),
		LocalURLPrefix: "/objects",
	})
	result, err := uploadStorageText(t, storage, "policy.txt", []byte("must-not-write"))
	if !errors.Is(err, ErrStorageUnavailable) || result != nil {
		t.Fatalf("policy failure result=%#v err=%v", result, err)
	}
	assertNoFiles(t, root)
}

func TestStoragePolicyWithoutSnapshotFailsClosed(t *testing.T) {
	previous := center.GetAppConfig()
	center.SetAppConfig(legacyUploadPolicy{
		maxSizeConfigKey:      "1024",
		allowedTypesConfigKey: "text/plain",
	})
	t.Cleanup(func() { center.SetAppConfig(previous) })
	root := filepath.Join(t.TempDir(), "objects")
	storage := storageUsingLease(adminconfig.ObjectStorageLease{
		Profile:        mustLocalStorageProfile(t, root),
		LocalRoot:      openLocalTestRoot(t, root),
		LocalURLPrefix: "/objects",
	})
	result, err := uploadStorageText(t, storage, "legacy-policy.txt", []byte("must-not-write"))
	if !errors.Is(err, ErrStorageUnavailable) || result != nil {
		t.Fatalf("legacy policy result=%#v err=%v", result, err)
	}
	assertNoFiles(t, root)
}

func uploadStorageText(t *testing.T, storage *Storage, filename string, data []byte) (*UploadResult, error) {
	t.Helper()
	context, _ := multipartRequestContext(t, filename, "text/plain", data)
	return storage.Upload(context, "file")
}

func mustLocalStorageProfile(t *testing.T, root string) *frameworkconfig.StorageProfile {
	t.Helper()
	profile, err := (frameworkconfig.Storage{
		Local: &frameworkconfig.LocalStorageConfig{Root: root},
	}).Normalize(context.Background(), nil)
	if err != nil {
		t.Fatalf("normalize local profile: %v", err)
	}
	return profile
}

func storageUsingLease(lease adminconfig.ObjectStorageLease) *Storage {
	return &Storage{useObjectStorage: func(_ context.Context, operation func(adminconfig.ObjectStorageLease) error) error {
		return operation(lease)
	}}
}

func setStorageProviderPolicy(t *testing.T) {
	t.Helper()
	previous := center.GetAppConfig()
	center.SetAppConfig(&fakeAppConfig{values: map[string]string{
		maxSizeConfigKey:      "1024",
		allowedTypesConfigKey: "text/plain",
	}})
	t.Cleanup(func() { center.SetAppConfig(previous) })
}
