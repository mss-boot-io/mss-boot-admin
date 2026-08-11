package server

import (
	"context"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
)

func TestBuildAuthorizationEventRuntimeUsesManagedDatabaseLease(t *testing.T) {
	db := openServerSchemaReadinessSQLite(t)
	if err := db.AutoMigrate(&models.ConfigRevision{}); err != nil {
		t.Fatal(err)
	}

	var databaseUses atomic.Int64
	runtime, err := buildAuthorizationEventRuntime(func(operation func(*gorm.DB) error) error {
		databaseUses.Add(1)
		return operation(db)
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := databaseUses.Load(); got != 0 {
		t.Fatalf("Build database uses = %d, want 0", got)
	}
	if err := runtime.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })
	if got := databaseUses.Load(); got != 1 {
		t.Fatalf("Open database uses = %d, want one authoritative revision read", got)
	}
	if got := runtime.String(); got != "authorization-revision-eventbus" {
		t.Fatalf("runtime name = %q", got)
	}
	if err := runtime.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestBuildAuthorizationEventRuntimeRejectsMissingDatabaseLease(t *testing.T) {
	if _, err := buildAuthorizationEventRuntime(nil); err == nil {
		t.Fatal("Build accepted a missing database lease")
	}
}
