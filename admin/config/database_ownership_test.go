package config

import (
	"context"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"gorm.io/gorm"
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

func TestReloadDatabaseWaitsForActiveDatabaseLease(t *testing.T) {
	oldHandle := openConfigSQLiteHandle(t, "config-reload-lease-old")
	configuration := &Config{
		Database: gormdb.Database{
			Driver: "sqlite",
			Source: "file:config-reload-lease-new?mode=memory&cache=shared",
		},
		databaseHandle: oldHandle,
	}
	gormdb.InstallDefault(oldHandle)
	defer func() {
		_ = configuration.Close()
		gormdb.ClearDefault(nil)
	}()

	leaseAcquired := make(chan struct{})
	releaseLease := make(chan struct{})
	leaseDone := make(chan error, 1)
	go func() {
		leaseDone <- configuration.WithDatabase(func(db *gorm.DB) error {
			close(leaseAcquired)
			<-releaseLease
			return db.Exec("SELECT 1").Error
		})
	}()
	<-leaseAcquired

	reloadDone := make(chan error, 1)
	go func() { reloadDone <- configuration.reloadDatabase(context.Background()) }()
	select {
	case err := <-reloadDone:
		t.Fatalf("reload completed before active lease was released: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseLease)
	if err := <-leaseDone; err != nil {
		t.Fatalf("leased database operation failed: %v", err)
	}
	if err := <-reloadDone; err != nil {
		t.Fatalf("reload after lease release failed: %v", err)
	}
	if err := oldHandle.SQLDB().Ping(); err == nil {
		t.Fatal("previous pool remains open after lease-safe reload")
	}
}

func TestReloadDatabaseAllowsRecursiveLeaseOnReplacementHandle(t *testing.T) {
	oldHandle := openConfigSQLiteHandle(t, "config-recursive-lease-old")
	configuration := &Config{
		Database: gormdb.Database{
			Driver: "sqlite",
			Source: "file:config-recursive-lease-new?mode=memory&cache=shared",
		},
		databaseHandle: oldHandle,
	}
	gormdb.InstallDefault(oldHandle)
	defer func() {
		_ = configuration.Close()
		gormdb.ClearDefault(nil)
	}()

	outerAcquired := make(chan struct{})
	runRecursiveLease := make(chan struct{})
	outerDone := make(chan error, 1)
	go func() {
		outerDone <- configuration.WithDatabase(func(oldDB *gorm.DB) error {
			close(outerAcquired)
			<-runRecursiveLease
			if err := configuration.WithDatabase(func(newDB *gorm.DB) error {
				if newDB == oldDB {
					return gorm.ErrInvalidDB
				}
				return newDB.Exec("SELECT 1").Error
			}); err != nil {
				return err
			}
			return oldDB.Exec("SELECT 1").Error
		})
	}()
	<-outerAcquired

	reloadDone := make(chan error, 1)
	go func() { reloadDone <- configuration.reloadDatabase(context.Background()) }()
	deadline := time.Now().Add(time.Second)
	for configuration.DatabaseHandle() == oldHandle && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if configuration.DatabaseHandle() == oldHandle {
		t.Fatal("reload did not publish the replacement while the old lease was active")
	}
	if err := oldHandle.SQLDB().Ping(); err != nil {
		t.Fatalf("old handle closed before its active lease completed: %v", err)
	}

	close(runRecursiveLease)
	select {
	case err := <-outerDone:
		if err != nil {
			t.Fatalf("recursive leased operation failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("recursive database lease deadlocked during reload")
	}
	if err := <-reloadDone; err != nil {
		t.Fatalf("reload after recursive lease: %v", err)
	}
	if err := oldHandle.SQLDB().Ping(); err == nil {
		t.Fatal("retired handle remains open after its final lease completed")
	}
}

func TestDatabaseLeasesCanRunConcurrently(t *testing.T) {
	handle := openConfigSQLiteHandle(t, "config-concurrent-leases")
	configuration := &Config{databaseHandle: handle}
	t.Cleanup(func() { _ = configuration.Close() })

	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			done <- configuration.WithDatabase(func(*gorm.DB) error {
				entered <- struct{}{}
				<-release
				return nil
			})
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("database leases were serialized")
		}
	}
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatalf("database lease error = %v", err)
		}
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
