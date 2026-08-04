package config

import (
	"context"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
)

func TestReloadDatabasePublishesReplacementThenClosesOwnedPreviousHandle(t *testing.T) {
	oldHandle := openConfigSQLiteHandle(t, "config-reload-old")
	configuration := &Config{
		Database: gormdb.Database{
			Driver: "sqlite",
			Source: "file:config-reload-new?mode=memory&cache=shared",
		},
		databaseHandle: oldHandle,
	}
	gormdb.InstallDefault(oldHandle)
	defer func() {
		_ = configuration.Close()
		gormdb.ClearDefault(nil)
	}()

	if err := configuration.reloadDatabase(context.Background()); err != nil {
		t.Fatalf("reload database: %v", err)
	}
	newHandle := configuration.DatabaseHandle()
	if newHandle == nil || newHandle == oldHandle {
		t.Fatalf("replacement handle = %p, old = %p", newHandle, oldHandle)
	}
	if gormdb.DefaultHandle() != newHandle || gormdb.DB != newHandle.DB {
		t.Fatal("replacement was not published through compatibility accessors")
	}
	if err := oldHandle.SQLDB().Ping(); err == nil {
		t.Fatal("previous owned pool remains open after successful replacement")
	}
	if err := newHandle.SQLDB().Ping(); err != nil {
		t.Fatalf("replacement pool is not usable: %v", err)
	}
}

func TestReloadDatabaseFailureKeepsPreviousHandleUsable(t *testing.T) {
	oldHandle := openConfigSQLiteHandle(t, "config-reload-rollback")
	configuration := &Config{
		Database: gormdb.Database{
			Driver: "unsupported",
			Source: "ignored",
		},
		databaseHandle: oldHandle,
	}
	gormdb.InstallDefault(oldHandle)
	defer func() {
		_ = configuration.Close()
		gormdb.ClearDefault(nil)
	}()

	if err := configuration.reloadDatabase(context.Background()); err == nil {
		t.Fatal("expected replacement open failure")
	}
	if configuration.DatabaseHandle() != oldHandle || gormdb.DefaultHandle() != oldHandle {
		t.Fatal("failed reload replaced the working handle")
	}
	if err := oldHandle.SQLDB().Ping(); err != nil {
		t.Fatalf("previous pool was closed after failed reload: %v", err)
	}
}

func TestConfigCloseDoesNotClearAnotherOwnerDefault(t *testing.T) {
	owned := openConfigSQLiteHandle(t, "config-owned")
	newer := openConfigSQLiteHandle(t, "config-newer")
	configuration := &Config{databaseHandle: owned}
	gormdb.InstallDefault(newer)
	defer func() {
		_ = newer.Close()
		gormdb.ClearDefault(nil)
	}()

	if err := configuration.Close(); err != nil {
		t.Fatalf("close configuration: %v", err)
	}
	if gormdb.DefaultHandle() != newer || gormdb.DB != newer.DB {
		t.Fatal("closing a stale owner cleared the newer default handle")
	}
	if err := owned.SQLDB().Ping(); err == nil {
		t.Fatal("owned pool remains open after Config.Close")
	}
}

func openConfigSQLiteHandle(t *testing.T, name string) *gormdb.Handle {
	t.Helper()
	handle, err := (&gormdb.Database{
		Driver: "sqlite",
		Source: "file:" + name + "?mode=memory&cache=shared",
	}).Open(context.Background())
	if err != nil {
		t.Fatalf("open SQLite handle %s: %v", name, err)
	}
	return handle
}
