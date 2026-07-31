package gorm

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"gorm.io/gorm"
)

type createCacheRecord struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
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
