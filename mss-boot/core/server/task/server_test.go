package task

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestStartBlocksUntilCancellation(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("Start returned before cancellation: %v", err)
	default:
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("task shutdown failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("task server did not stop")
	}
}

func TestStartCanOnlyRunOnce(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	if err := srv.Start(context.Background()); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("expected ErrAlreadyStarted, got %v", err)
	}
}

func TestStartReturnsInvalidScheduleError(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	WithSchedule("invalid", "not-a-cron-expression", testJob{})(&srv.opts)
	if err := srv.Start(context.Background()); err == nil {
		t.Fatal("expected invalid schedule error")
	}
}

func TestWithScheduleKeepsConfiguredStorageBehavior(t *testing.T) {
	opts := setDefaultOption()
	persistent := &defaultStorage{schedules: make(map[string]*schedule)}
	WithStorage(persistent)(&opts)
	WithSchedule("stored", "@every 1h", testJob{})(&opts)

	if opts.configurationErr != nil {
		t.Fatalf("WithSchedule() configuration error = %v", opts.configurationErr)
	}
	_, spec, _, ok, err := persistent.Get("stored")
	if err != nil || !ok || spec != "@every 1h" {
		t.Fatalf("persistent schedule = spec %q ok %v err %v", spec, ok, err)
	}
	if _, _, _, ok, _ := opts.systemSchedules.Get("stored"); ok {
		t.Fatal("WithSchedule() unexpectedly registered a system schedule")
	}
}

func TestWithScheduleReportsStorageSetErrorAtStart(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	WithStorage(&setRejectingStorage{})(&srv.opts)
	WithSchedule("stored", "@every 1h", testJob{})(&srv.opts)
	if err := srv.Start(context.Background()); err == nil {
		t.Fatal("Start() storage registration error = nil")
	}
}

func TestWithSystemScheduleDoesNotWritePersistentStorage(t *testing.T) {
	for _, options := range [][]Option{
		{
			WithStorage(&setRejectingStorage{}),
			WithSystemSchedule("system", "@every 1h", testJob{}),
		},
		{
			WithSystemSchedule("system", "@every 1h", testJob{}),
			WithStorage(&setRejectingStorage{}),
		},
	} {
		opts := setDefaultOption()
		for _, option := range options {
			option(&opts)
		}
		if opts.configurationErr != nil {
			t.Fatalf("WithSystemSchedule() configuration error = %v", opts.configurationErr)
		}
		_, spec, _, ok, err := opts.systemSchedules.Get("system")
		if err != nil || !ok || spec != "@every 1h" {
			t.Fatalf("system schedule = spec %q ok %v err %v", spec, ok, err)
		}
	}
}

func TestLoadSchedulesMergesSystemAndPersistentJobs(t *testing.T) {
	opts := setDefaultOption()
	WithSystemSchedule("monitor", "@every 5s", testJob{})(&opts)
	persistent := &defaultStorage{schedules: make(map[string]*schedule)}
	if err := persistent.Set("user", 0, "@every 1m", testJob{}); err != nil {
		t.Fatalf("persistent Set() error = %v", err)
	}

	loaded, err := loadSchedules(opts.systemSchedules, persistent)
	if err != nil {
		t.Fatalf("loadSchedules() error = %v", err)
	}
	keys := make([]string, len(loaded))
	for i := range loaded {
		keys[i] = loaded[i].key
	}
	if want := []string{"monitor", "user"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("loaded keys = %v, want %v", keys, want)
	}
}

func TestLoadSchedulesRejectsPersistentSystemKeyCollision(t *testing.T) {
	opts := setDefaultOption()
	WithSystemSchedule("monitor", "@every 5s", testJob{})(&opts)
	persistent := &defaultStorage{schedules: make(map[string]*schedule)}
	if err := persistent.Set("monitor", 0, "@every 1m", testJob{}); err != nil {
		t.Fatalf("persistent Set() error = %v", err)
	}

	if _, err := loadSchedules(opts.systemSchedules, persistent); !errors.Is(err, ErrScheduleKeyConflict) {
		t.Fatalf("loadSchedules() error = %v, want %v", err, ErrScheduleKeyConflict)
	}
}

func TestStartReportsDuplicateSystemSchedule(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	WithSystemSchedule("monitor", "@every 5s", testJob{})(&srv.opts)
	WithSystemSchedule("monitor", "@every 10s", testJob{})(&srv.opts)

	if err := srv.Start(context.Background()); err == nil {
		t.Fatal("Start() duplicate system schedule error = nil")
	}
}

func TestSystemScheduleRunsWhenUserSchedulesAreDisabled(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	WithUserSchedulesEnabled(false)(&srv.opts)
	WithStorage(&unavailableStorage{})(&srv.opts)
	runs := make(chan struct{}, 1)
	WithSystemSchedule("system", "@every 1s", channelJob{runs: runs})(&srv.opts)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Start(ctx) }()

	select {
	case <-runs:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("system schedule did not run while user schedules were disabled")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("task server shutdown error = %v", err)
	}
}

func TestSystemSchedulesAreImmutableThroughRuntimeAPI(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	WithSystemSchedule("system", "@every 1h", testJob{})(&srv.opts)
	installTestTaskServer(t, srv)

	if err := UpdateJob("system", "@every 2h", testJob{}); !errors.Is(err, ErrSystemScheduleImmutable) {
		t.Fatalf("UpdateJob(system) error = %v, want %v", err, ErrSystemScheduleImmutable)
	}
	if err := RemoveJob("system"); !errors.Is(err, ErrSystemScheduleImmutable) {
		t.Fatalf("RemoveJob(system) error = %v, want %v", err, ErrSystemScheduleImmutable)
	}
}

func TestRuntimeAPIRejectsUserSchedulesWhenDisabled(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	WithUserSchedulesEnabled(false)(&srv.opts)
	installTestTaskServer(t, srv)

	if UserSchedulesEnabled() {
		t.Fatal("UserSchedulesEnabled() = true")
	}
	if err := UpdateJob("user", "@every 1h", testJob{}); !errors.Is(err, ErrUserSchedulesDisabled) {
		t.Fatalf("UpdateJob(user) error = %v, want %v", err, ErrUserSchedulesDisabled)
	}
	if err := RemoveJob("user"); !errors.Is(err, ErrUserSchedulesDisabled) {
		t.Fatalf("RemoveJob(user) error = %v, want %v", err, ErrUserSchedulesDisabled)
	}
	if entries := srv.opts.task.Entries(); len(entries) != 0 {
		t.Fatalf("disabled user scheduling registered %d cron entries", len(entries))
	}
}

func TestConcurrentRuntimeUpdatesRegisterOneEntry(t *testing.T) {
	srv := &Server{opts: setDefaultOption()}
	installTestTaskServer(t, srv)

	var wait sync.WaitGroup
	errorsByCall := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsByCall <- UpdateJob("user", "@every 1h", testJob{})
		}()
	}
	wait.Wait()
	close(errorsByCall)
	for err := range errorsByCall {
		if err != nil {
			t.Fatalf("concurrent UpdateJob() error = %v", err)
		}
	}
	if entries := srv.opts.task.Entries(); len(entries) != 1 {
		t.Fatalf("concurrent updates registered %d cron entries, want 1", len(entries))
	}
}

func TestStartRollbackRestoresStoredEntryIDs(t *testing.T) {
	storage := &failOnceUpdateStorage{
		defaultStorage: defaultStorage{schedules: make(map[string]*schedule)},
		failKey:        "second",
	}
	if err := storage.Set("first", 41, "@every 1h", testJob{}); err != nil {
		t.Fatalf("Set(first) error = %v", err)
	}
	if err := storage.Set("second", 42, "@every 2h", testJob{}); err != nil {
		t.Fatalf("Set(second) error = %v", err)
	}
	srv := &Server{opts: setDefaultOption()}
	WithStorage(storage)(&srv.opts)

	if err := srv.Start(context.Background()); err == nil {
		t.Fatal("Start() update failure = nil")
	}
	for key, want := range map[string]cron.EntryID{"first": 41, "second": 42} {
		got, _, _, ok, err := storage.Get(key)
		if err != nil || !ok || got != want {
			t.Fatalf("restored %s entry ID = %d, ok %v, err %v; want %d", key, got, ok, err, want)
		}
	}
	if entries := srv.opts.task.Entries(); len(entries) != 0 {
		t.Fatalf("rollback left %d cron entries", len(entries))
	}
	if loaded := len(srv.loadedSchedules); loaded != 0 {
		t.Fatalf("rollback left %d schedules in the process registry", loaded)
	}
}

func TestShutdownDoesNotWaitForBlockedRuntimeStorageMutation(t *testing.T) {
	storage := &blockingRuntimeUpdateStorage{
		defaultStorage: defaultStorage{schedules: make(map[string]*schedule)},
		startupUpdated: make(chan struct{}),
		runtimeStarted: make(chan struct{}),
		releaseRuntime: make(chan struct{}),
	}
	if err := storage.Set("user", 0, "@every 1h", testJob{}); err != nil {
		t.Fatalf("Set(user) error = %v", err)
	}
	srv := &Server{opts: setDefaultOption()}
	WithStorage(storage)(&srv.opts)
	installTestTaskServer(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	startDone := make(chan error, 1)
	go func() { startDone <- srv.Start(ctx) }()
	<-storage.startupUpdated

	updateDone := make(chan error, 1)
	go func() { updateDone <- UpdateJob("user", "@every 2h", testJob{}) }()
	<-storage.runtimeStarted

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer shutdownCancel()
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- srv.Shutdown(shutdownCtx) }()
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	case <-shutdownCtx.Done():
		t.Fatal("Shutdown() waited for blocked runtime storage mutation")
	}

	close(storage.releaseRuntime)
	if err := <-updateDone; err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}
	cancel()
	if err := <-startDone; err != nil {
		t.Fatalf("Start() shutdown error = %v", err)
	}
}

func installTestTaskServer(t *testing.T, srv *Server) {
	t.Helper()
	previous := task
	task = srv
	t.Cleanup(func() { task = previous })
}

type testJob struct{}

func (testJob) Run() {}

type channelJob struct {
	runs chan<- struct{}
}

func (e channelJob) Run() {
	select {
	case e.runs <- struct{}{}:
	default:
	}
}

type setRejectingStorage struct{}

func (*setRejectingStorage) Get(string) (cron.EntryID, string, cron.Job, bool, error) {
	return 0, "", nil, false, nil
}

func (*setRejectingStorage) Set(string, cron.EntryID, string, cron.Job) error {
	return errors.New("persistent Set must not be called for system schedules")
}

func (*setRejectingStorage) Update(string, cron.EntryID) error { return nil }
func (*setRejectingStorage) Remove(string) error               { return nil }
func (*setRejectingStorage) ListKeys() ([]string, error)       { return nil, nil }

type unavailableStorage struct{}

func (*unavailableStorage) Get(string) (cron.EntryID, string, cron.Job, bool, error) {
	return 0, "", nil, false, errors.New("user storage must not be read")
}

func (*unavailableStorage) Set(string, cron.EntryID, string, cron.Job) error {
	return errors.New("user storage must not be written")
}

func (*unavailableStorage) Update(string, cron.EntryID) error {
	return errors.New("user storage must not be written")
}

func (*unavailableStorage) Remove(string) error {
	return errors.New("user storage must not be written")
}

func (*unavailableStorage) ListKeys() ([]string, error) {
	return nil, errors.New("user storage must not be listed")
}

type failOnceUpdateStorage struct {
	defaultStorage
	mu      sync.Mutex
	failKey string
	failed  bool
}

func (e *failOnceUpdateStorage) Update(key string, entryID cron.EntryID) error {
	e.mu.Lock()
	if key == e.failKey && !e.failed {
		e.failed = true
		e.mu.Unlock()
		return errors.New("injected update failure")
	}
	e.mu.Unlock()
	return e.defaultStorage.Update(key, entryID)
}

type blockingRuntimeUpdateStorage struct {
	defaultStorage
	mu             sync.Mutex
	updates        int
	startupUpdated chan struct{}
	runtimeStarted chan struct{}
	releaseRuntime chan struct{}
}

func (e *blockingRuntimeUpdateStorage) Update(key string, entryID cron.EntryID) error {
	e.mu.Lock()
	e.updates++
	updateNumber := e.updates
	e.mu.Unlock()
	if updateNumber == 2 {
		close(e.runtimeStarted)
		<-e.releaseRuntime
	}
	if err := e.defaultStorage.Update(key, entryID); err != nil {
		return err
	}
	if updateNumber == 1 {
		close(e.startupUpdated)
	}
	return nil
}
