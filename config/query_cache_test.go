package config

import (
	"context"
	"testing"
	"time"

	responsegorm "github.com/mss-boot-io/mss-boot/pkg/response/actions/gorm"
	"gorm.io/gorm"
)

type queryCacheStub struct {
	initialized bool
	db          *gorm.DB
	removedTag  string
}

func (s *queryCacheStub) Initialize(db *gorm.DB) error {
	s.initialized = true
	s.db = db
	return nil
}

func (s *queryCacheStub) RemoveFromTag(_ context.Context, tag string) error {
	s.removedTag = tag
	return nil
}

func TestBindQueryCacheInitializesAndCleansPrefixedTag(t *testing.T) {
	previousCleaner := responsegorm.CleanCacheFromTag
	defer func() {
		responsegorm.CleanCacheFromTag = previousCleaner
	}()

	db := &gorm.DB{}
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

	if err := responsegorm.CleanCacheFromTag(context.Background(), "tenants"); err != nil {
		t.Fatalf("clean cache from tag: %v", err)
	}
	if cache.removedTag != "gorm.cache:tenants" {
		t.Fatalf("expected prefixed tag, got %q", cache.removedTag)
	}
}
