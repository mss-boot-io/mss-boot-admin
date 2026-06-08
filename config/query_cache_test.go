package config

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	bootconfig "github.com/mss-boot-io/mss-boot/pkg/config"
	responsegorm "github.com/mss-boot-io/mss-boot/pkg/response/actions/gorm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
)

type queryCacheStub struct {
	initialized     bool
	queryCallbacked bool
	db              *gorm.DB
	removedTag      string
	removeErr       error
}

func (s *queryCacheStub) Initialize(db *gorm.DB) error {
	s.initialized = true
	s.db = db
	return db.Callback().Query().Replace("gorm:query", func(tx *gorm.DB) {
		s.queryCallbacked = true
		callbacks.Query(tx)
	})
}

func (s *queryCacheStub) RemoveFromTag(_ context.Context, tag string) error {
	s.removedTag = tag
	return s.removeErr
}

type queryCacheTenant struct {
	ID   int64
	Name string
}

func TestBindQueryCacheInitializesAndCleansPrefixedTag(t *testing.T) {
	previousCleaner := responsegorm.CleanCacheFromTag
	defer func() {
		responsegorm.CleanCacheFromTag = previousCleaner
	}()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&queryCacheTenant{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&queryCacheTenant{Name: "acme"}).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}

	cache := &queryCacheStub{}

	bindQueryCache(cache, db, time.Hour)

	if !cache.initialized {
		t.Fatal("expected query cache to be initialized")
	}
	if cache.db != db {
		t.Fatal("expected query cache to initialize with the provided gorm db")
	}
	if responsegorm.CleanCacheFromTag == nil {
		t.Fatal("expected query cache cleaner to be registered")
	}

	var tenants []queryCacheTenant
	if err := db.Find(&tenants).Error; err != nil {
		t.Fatalf("query tenants: %v", err)
	}
	if !cache.queryCallbacked {
		t.Fatal("expected query cache callback to run on a real gorm query")
	}
	if len(tenants) != 1 || tenants[0].Name != "acme" {
		t.Fatalf("expected seeded tenant from real query callback path, got %#v", tenants)
	}

	if err := responsegorm.CleanCacheFromTag(context.Background(), "tenants"); err != nil {
		t.Fatalf("clean cache from tag: %v", err)
	}
	if cache.removedTag != "gorm.cache:tenants" {
		t.Fatalf("expected prefixed tag, got %q", cache.removedTag)
	}
}

func TestCleanCacheFromTagLogsRemoveError(t *testing.T) {
	previousCleaner := responsegorm.CleanCacheFromTag
	defer func() {
		responsegorm.CleanCacheFromTag = previousCleaner
	}()

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(previousLogger)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	removeErr := errors.New("redis unavailable")
	cache := &queryCacheStub{removeErr: removeErr}
	bindQueryCache(cache, db, time.Hour)

	err = responsegorm.CleanCacheFromTag(context.Background(), "tenants")
	if !errors.Is(err, removeErr) {
		t.Fatalf("expected remove error, got %v", err)
	}
	if cache.removedTag != "gorm.cache:tenants" {
		t.Fatalf("expected prefixed tag, got %q", cache.removedTag)
	}
	if !strings.Contains(logs.String(), "query cache invalidation failed") ||
		!strings.Contains(logs.String(), "tenants") {
		t.Fatalf("expected invalidation failure log with tag, got logs: %s", logs.String())
	}
}

func TestBindQueryCacheWarnsWhenAdapterMissing(t *testing.T) {
	previousCleaner := responsegorm.CleanCacheFromTag
	responsegorm.CleanCacheFromTag = nil
	defer func() {
		responsegorm.CleanCacheFromTag = previousCleaner
	}()

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(previousLogger)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	bindQueryCache(nil, db, time.Hour)

	if responsegorm.CleanCacheFromTag != nil {
		t.Fatal("expected cleaner to remain unregistered without a cache adapter")
	}
	if !strings.Contains(logs.String(), "query cache enabled but no cache adapter available") {
		t.Fatalf("expected missing cache adapter warning, got logs: %s", logs.String())
	}
}

func TestWarnQueryCacheDurationWhenZero(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(previousLogger)

	warnQueryCacheDuration(&bootconfig.Cache{QueryCache: true})

	if !strings.Contains(logs.String(), "cache.queryCache enabled but queryCacheDuration is zero") {
		t.Fatalf("expected query cache duration warning, got logs: %s", logs.String())
	}
}

func TestBindQueryCacheSafelyReturnsForNilTx(t *testing.T) {
	cache := &queryCacheStub{}

	bindQueryCache(cache, nil, time.Hour)

	if cache.initialized {
		t.Fatal("expected nil tx to skip query cache initialization")
	}
}
