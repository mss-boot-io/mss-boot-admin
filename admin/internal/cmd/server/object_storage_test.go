package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	adminconfig "github.com/mss-boot-io/mss-boot-admin/admin/config"
	frameworkconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config"
)

func TestStorageInstalledBeforeApplicationDelivery(t *testing.T) {
	root := filepath.Join(t.TempDir(), "objects")
	configuration := &adminconfig.Config{
		Application: adminconfig.Application{
			Mode:       adminconfig.ModeDev,
			StaticPath: map[string]string{"/objects": root},
		},
		Storage: &frameworkconfig.Storage{
			Local: &frameworkconfig.LocalStorageConfig{Root: root},
		},
	}
	if err := initializeApplicationDelivery(context.Background(), configuration, gin.New()); err != nil {
		t.Fatalf("install storage before application delivery: %v", err)
	}
	if err := configuration.WithObjectStorage(context.Background(), func(lease adminconfig.ObjectStorageLease) error {
		if lease.Profile == nil || lease.Profile.Provider() != frameworkconfig.Local || lease.S3Client != nil || lease.LocalRoot == nil || lease.LocalURLPrefix != "/objects" {
			return errors.New("unexpected installed Local lease")
		}
		return nil
	}); err != nil {
		t.Fatalf("lease installed storage: %v", err)
	}
	closeContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := configuration.CloseContext(closeContext); err != nil {
		t.Fatalf("close installed storage: %v", err)
	}
}

func TestStorageAPICheckClosesOwnedResources(t *testing.T) {
	rootName := filepath.Join(t.TempDir(), "objects")
	if err := os.MkdirAll(rootName, 0o750); err != nil {
		t.Fatalf("create Local root: %v", err)
	}
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
	localRoot, err := os.OpenRoot(rootName)
	if err != nil {
		t.Fatalf("open Local root: %v", err)
	}
	configuration := &adminconfig.Config{}
	if err := configuration.InstallObjectStorage(handle, localRoot, "/objects"); err != nil {
		t.Fatalf("install Local owner: %v", err)
	}

	previousConfig := adminconfig.Cfg
	previousAPICheck := apiCheck
	adminconfig.Cfg = configuration
	apiCheck = true
	t.Cleanup(func() {
		adminconfig.Cfg = previousConfig
		apiCheck = previousAPICheck
	})

	if err := run(context.Background()); err != nil {
		t.Fatalf("run API-check close path: %v", err)
	}
	if err := configuration.WithObjectStorage(context.Background(), func(adminconfig.ObjectStorageLease) error {
		return errors.New("closed API-check owner was still available")
	}); !errors.Is(err, adminconfig.ErrObjectStorageUnavailable) {
		t.Fatalf("post API-check lease error = %v", err)
	}
	if _, err := localRoot.Stat("."); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("post API-check pinned root error = %v, want os.ErrClosed", err)
	}
}

func TestStoragePinnedRootSurvivesConfiguredPathReplacement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := t.TempDir()
	rootName := filepath.Join(base, "objects")
	configuration := &adminconfig.Config{
		Application: adminconfig.Application{
			Mode:       adminconfig.ModeDev,
			StaticPath: map[string]string{"/objects": rootName},
		},
		Storage: &frameworkconfig.Storage{
			Local: &frameworkconfig.LocalStorageConfig{Root: rootName},
		},
	}
	engine := gin.New()
	if err := initializeApplicationDelivery(context.Background(), configuration, engine); err != nil {
		t.Fatalf("install pinned Local storage: %v", err)
	}
	t.Cleanup(func() {
		closeContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := configuration.CloseContext(closeContext); err != nil {
			t.Errorf("close pinned Local storage: %v", err)
		}
	})

	pinnedName := filepath.Join(base, "objects-pinned")
	if err := os.Rename(rootName, pinnedName); err != nil {
		t.Fatalf("rename configured root after startup: %v", err)
	}
	if err := os.MkdirAll(rootName, 0o750); err != nil {
		t.Fatalf("create replacement configured root: %v", err)
	}
	want := []byte("pinned-root-content")
	if err := configuration.WithObjectStorage(context.Background(), func(lease adminconfig.ObjectStorageLease) error {
		if lease.LocalRoot == nil {
			return errors.New("local root lease is nil")
		}
		if err := lease.LocalRoot.MkdirAll("uploads", 0o750); err != nil {
			return err
		}
		return lease.LocalRoot.WriteFile("uploads/pinned.txt", want, 0o600)
	}); err != nil {
		t.Fatalf("write through pinned root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rootName, "uploads", "pinned.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replacement root unexpectedly received object: %v", err)
	}

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/objects/uploads/pinned.txt", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != string(want) {
		t.Fatalf("pinned delivery status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestStorageSymlinkRootRemainsUninstalled(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatalf("create target: %v", err)
	}
	rootName := filepath.Join(base, "objects")
	if err := os.Symlink(target, rootName); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	configuration := &adminconfig.Config{
		Application: adminconfig.Application{
			Mode:       adminconfig.ModeDev,
			StaticPath: map[string]string{"/objects": rootName},
		},
		Storage: &frameworkconfig.Storage{Local: &frameworkconfig.LocalStorageConfig{Root: rootName}},
	}
	if err := initializeApplicationDelivery(context.Background(), configuration, gin.New()); err != nil {
		t.Fatalf("evaluate symlink root: %v", err)
	}
	if err := configuration.WithObjectStorage(context.Background(), func(adminconfig.ObjectStorageLease) error {
		return errors.New("symlink root was unexpectedly installed")
	}); !errors.Is(err, adminconfig.ErrObjectStorageUnavailable) {
		t.Fatalf("symlink root lease error = %v", err)
	}
}

func TestStorageProductionLocalRemainsUninstalled(t *testing.T) {
	root := filepath.Join(t.TempDir(), "objects")
	configuration := &adminconfig.Config{
		Application: adminconfig.Application{
			Mode:       adminconfig.ModeProd,
			StaticPath: map[string]string{"/objects": root},
		},
		Storage: &frameworkconfig.Storage{
			Local: &frameworkconfig.LocalStorageConfig{Root: root},
		},
	}
	if err := initializeApplicationDelivery(context.Background(), configuration, gin.New()); err != nil {
		t.Fatalf("evaluate production Local profile: %v", err)
	}
	if err := configuration.WithObjectStorage(context.Background(), func(adminconfig.ObjectStorageLease) error {
		return errors.New("production Local profile was unexpectedly installed")
	}); !errors.Is(err, adminconfig.ErrObjectStorageUnavailable) {
		t.Fatalf("production Local lease error = %v", err)
	}
}

func TestStorageInvalidProfileRemainsUninstalled(t *testing.T) {
	t.Setenv("MISSING_OBJECT_ACCESS_KEY", "")
	t.Setenv("MISSING_OBJECT_SECRET_KEY", "")
	tests := []struct {
		name    string
		storage *frameworkconfig.Storage
	}{
		{name: "missing provider", storage: &frameworkconfig.Storage{}},
		{name: "invalid bucket", storage: &frameworkconfig.Storage{S3: &frameworkconfig.S3StorageConfig{
			Endpoint:    "https://objects.example.test",
			Region:      "test-region",
			Bucket:      "invalid/bucket",
			Credentials: frameworkconfig.S3CredentialsConfig{DefaultChain: &frameworkconfig.DefaultChainCredentials{}},
		}}},
		{name: "missing static secret refs", storage: &frameworkconfig.Storage{S3: &frameworkconfig.S3StorageConfig{
			Endpoint: "https://objects.example.test",
			Region:   "test-region",
			Bucket:   "uploads",
			Credentials: frameworkconfig.S3CredentialsConfig{Static: &frameworkconfig.StaticCredentialRefs{
				AccessKeyRef: "env://MISSING_OBJECT_ACCESS_KEY",
				SecretKeyRef: "env://MISSING_OBJECT_SECRET_KEY",
			}},
		}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rootName := filepath.Join(t.TempDir(), "must-not-exist")
			configuration := &adminconfig.Config{
				Application: adminconfig.Application{
					Mode:       adminconfig.ModeDev,
					StaticPath: map[string]string{"/objects": rootName},
				},
				Storage: test.storage,
			}
			if err := initializeApplicationDelivery(context.Background(), configuration, gin.New()); err != nil {
				t.Fatalf("evaluate invalid profile: %v", err)
			}
			if err := configuration.WithObjectStorage(context.Background(), func(adminconfig.ObjectStorageLease) error {
				return errors.New("invalid profile was unexpectedly installed")
			}); !errors.Is(err, adminconfig.ErrObjectStorageUnavailable) {
				t.Fatalf("invalid profile lease error = %v", err)
			}
			if _, err := os.Stat(rootName); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid profile created local residue: %v", err)
			}
		})
	}
}
