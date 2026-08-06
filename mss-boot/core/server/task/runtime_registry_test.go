package task

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestConcurrentUpdateJobReloadsPersistedCronSpec(t *testing.T) {
	storage := &startupSignalStorage{
		defaultStorage: defaultStorage{schedules: make(map[string]*schedule)},
		updated:        make(chan struct{}),
	}
	srv := &Server{opts: setDefaultOption()}
	WithStorage(storage)(&srv.opts)
	installTestTaskServer(t, srv)

	const (
		key     = "user"
		oldSpec = "@every 1h"
		newSpec = "@every 2h"
	)
	if err := storage.Set(key, 0, oldSpec, testJob{}); err != nil {
		t.Fatalf("persist initial job: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	startDone := make(chan error, 1)
	go func() { startDone <- srv.Start(ctx) }()
	stopped := false
	t.Cleanup(func() {
		if stopped {
			return
		}
		cancel()
		select {
		case startErr := <-startDone:
			if startErr != nil {
				t.Errorf("task server cleanup: %v", startErr)
			}
		case <-time.After(time.Second):
			t.Error("task server cleanup timed out")
		}
	})
	select {
	case <-storage.updated:
	case <-time.After(time.Second):
		t.Fatal("task server did not load the initial schedule")
	}
	oldEntryID, _, _, ok, err := storage.Get(key)
	if err != nil || !ok || oldEntryID == 0 {
		t.Fatalf("initial schedule entry ID = %d, ok %v, err %v", oldEntryID, ok, err)
	}

	// User task persistence is updated before the runtime scheduler is
	// reconciled, so Storage now contains the requested spec while cron still
	// runs oldSpec.
	if err := storage.Set(key, oldEntryID, newSpec, testJob{}); err != nil {
		t.Fatalf("persist edited schedule: %v", err)
	}

	const callers = 20
	var wait sync.WaitGroup
	errorsByCall := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsByCall <- UpdateJob(key, newSpec, testJob{})
		}()
	}
	wait.Wait()
	close(errorsByCall)
	for updateErr := range errorsByCall {
		if updateErr != nil {
			t.Fatalf("concurrent UpdateJob() error = %v", updateErr)
		}
	}

	newEntryID, _, _, ok, err := storage.Get(key)
	if err != nil || !ok {
		t.Fatalf("updated schedule unavailable: ok %v, err %v", ok, err)
	}
	if newEntryID == oldEntryID {
		t.Fatalf("runtime entry ID remained %d after spec changed", newEntryID)
	}
	if entry := srv.opts.task.Entry(oldEntryID); entry.ID != 0 {
		t.Fatalf("old cron entry %d is still loaded", oldEntryID)
	}
	entries := srv.opts.task.Entries()
	if len(entries) != 1 || entries[0].ID != newEntryID {
		t.Fatalf("loaded cron entries = %+v, want only entry %d", entries, newEntryID)
	}
	cancel()
	if err := <-startDone; err != nil {
		t.Fatalf("task server shutdown: %v", err)
	}
	stopped = true
}

func TestUpdateJobRollbackPreservesLoadedSpecAndEntry(t *testing.T) {
	storage := &runtimeUpdateFailStorage{
		defaultStorage: defaultStorage{schedules: make(map[string]*schedule)},
	}
	srv := &Server{opts: setDefaultOption()}
	WithStorage(storage)(&srv.opts)
	installTestTaskServer(t, srv)

	const (
		key     = "user"
		oldSpec = "@every 1h"
		newSpec = "@every 2h"
	)
	if err := UpdateJob(key, oldSpec, testJob{}); err != nil {
		t.Fatalf("register initial job: %v", err)
	}
	oldEntryID, _, _, _, _ := storage.Get(key)
	if err := storage.Set(key, oldEntryID, newSpec, testJob{}); err != nil {
		t.Fatalf("persist edited schedule: %v", err)
	}
	storage.failNextUpdate()

	if err := UpdateJob(key, newSpec, testJob{}); err == nil {
		t.Fatal("UpdateJob() persistence failure = nil")
	}
	storedEntryID, _, _, ok, err := storage.Get(key)
	if err != nil || !ok || storedEntryID != oldEntryID {
		t.Fatalf("entry ID after rollback = %d, ok %v, err %v; want %d", storedEntryID, ok, err, oldEntryID)
	}
	if entry := srv.opts.task.Entry(oldEntryID); entry.ID != oldEntryID {
		t.Fatalf("old cron entry %d was removed during rollback", oldEntryID)
	}
	if entries := srv.opts.task.Entries(); len(entries) != 1 {
		t.Fatalf("rollback left %d cron entries, want 1", len(entries))
	}

	// A successful retry must still compare against the loaded oldSpec. If a
	// failed attempt advanced the in-process registry, this retry would be
	// incorrectly treated as a no-op because Storage already has newSpec.
	if err := UpdateJob(key, newSpec, testJob{}); err != nil {
		t.Fatalf("retry UpdateJob(): %v", err)
	}
	newEntryID, _, _, _, _ := storage.Get(key)
	if newEntryID == oldEntryID {
		t.Fatalf("retry kept old cron entry ID %d", oldEntryID)
	}
	if entry := srv.opts.task.Entry(oldEntryID); entry.ID != 0 {
		t.Fatalf("retry left old cron entry %d loaded", oldEntryID)
	}
}

type runtimeUpdateFailStorage struct {
	defaultStorage
	mu       sync.Mutex
	failNext bool
}

type startupSignalStorage struct {
	defaultStorage
	once    sync.Once
	updated chan struct{}
}

func (e *startupSignalStorage) Update(key string, entryID cron.EntryID) error {
	if err := e.defaultStorage.Update(key, entryID); err != nil {
		return err
	}
	e.once.Do(func() { close(e.updated) })
	return nil
}

func (e *runtimeUpdateFailStorage) failNextUpdate() {
	e.mu.Lock()
	e.failNext = true
	e.mu.Unlock()
}

func (e *runtimeUpdateFailStorage) Update(key string, entryID cron.EntryID) error {
	e.mu.Lock()
	if e.failNext {
		e.failNext = false
		e.mu.Unlock()
		return errors.New("injected runtime update failure")
	}
	e.mu.Unlock()
	return e.defaultStorage.Update(key, entryID)
}
