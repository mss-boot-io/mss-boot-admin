package gorm

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type createCacheRecord struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func TestControlCreateRunsAfterCommitHookAfterTransaction(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "after-commit.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	observer, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open observer sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&createCacheRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })
	previousCleaner := CleanCacheFromTag
	cacheCleaned := false
	CleanCacheFromTag = func(_ context.Context, tag string) error {
		if tag != "create_cache_records" {
			return fmt.Errorf("cleaned cache tag = %q, want create_cache_records", tag)
		}
		cacheCleaned = true
		return nil
	}
	t.Cleanup(func() { CleanCacheFromTag = previousCleaner })
	hookCalled := false
	hook := func(_ *gin.Context, _ *gorm.DB, _ schema.Tabler) error {
		hookCalled = true
		if !cacheCleaned {
			return fmt.Errorf("after-commit hook ran before the generic cache tag was cleaned")
		}
		var count int64
		if err := observer.Model(&createCacheRecord{}).Where("name = ?", "committed").Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("observer row count = %d, want 1 committed row", count)
		}
		return nil
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	control := NewControl(WithModel(&createCacheRecord{}), WithAfterCommitCreate(hook))
	router.POST("/records", control.Handler()...)
	req := httptest.NewRequest(http.MethodPost, "/records", bytes.NewBufferString(`{"name":"committed"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK && resp.Code != http.StatusCreated {
		t.Fatalf("expected successful response, got %d: %s", resp.Code, resp.Body.String())
	}
	if !hookCalled {
		t.Fatal("after-commit create hook was not called")
	}
}

func (*createCacheRecord) TableName() string {
	return "create_cache_records"
}

func TestControlCreateCleansQueryCacheTag(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&createCacheRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	defer func() {
		gormdb.DB = previousDB
	}()

	previousCleaner := CleanCacheFromTag
	var cleanedTag string
	CleanCacheFromTag = func(_ context.Context, tag string) error {
		cleanedTag = tag
		return nil
	}
	defer func() {
		CleanCacheFromTag = previousCleaner
	}()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	control := NewControl(WithModel(&createCacheRecord{}))
	router.POST("/records", control.Handler()...)

	req := httptest.NewRequest(http.MethodPost, "/records", bytes.NewBufferString(`{"name":"acme"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK && resp.Code != http.StatusCreated {
		t.Fatalf("expected successful response, got %d: %s", resp.Code, resp.Body.String())
	}
	if cleanedTag != "create_cache_records" {
		t.Fatalf("expected create to clean table cache tag, got %q", cleanedTag)
	}
}
