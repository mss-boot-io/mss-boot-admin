package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/robfig/cron/v3"
)

var (
	task = &Server{
		opts: setDefaultOption(),
	}
	// ErrAlreadyStarted is returned when a task server is started twice.
	ErrAlreadyStarted = errors.New("task server has already been started")
	// ErrSystemScheduleImmutable is returned when a caller attempts to mutate a
	// process-local system schedule through the user-task API.
	ErrSystemScheduleImmutable = errors.New("task system schedule is immutable")
	// ErrScheduleKeyConflict is returned when persistent user-task storage uses
	// a key reserved by a process-local system schedule.
	ErrScheduleKeyConflict = errors.New("task schedule key conflicts with a system schedule")
	// ErrUserSchedulesDisabled is returned when the process was started with
	// user-managed scheduling disabled. Built-in system schedules remain active.
	ErrUserSchedulesDisabled = errors.New("task user schedules are disabled")
)

// Server manages scheduled jobs.
type Server struct {
	ctx    context.Context
	opts   options
	mux    sync.Mutex
	jobsMu sync.Mutex
	// loadedSchedules is authoritative for this process. Persistent storage
	// may already contain an edited spec or an entry ID written by another
	// process before this scheduler has reconciled its local cron entries.
	loadedSchedules map[string]runtimeSchedule
	started         bool
}

type runtimeSchedule struct {
	spec    string
	entryID cron.EntryID
}

// New configures and returns the process-wide task server for compatibility.
func New(opts ...Option) *Server {
	task.Options(opts...)
	return task
}

// GetJob returns a configured job.
func GetJob(key string) (string, cron.Job, bool) {
	task.jobsMu.Lock()
	defer task.jobsMu.Unlock()
	if _, spec, job, ok, _ := task.opts.systemSchedules.Get(key); ok {
		return spec, job, true
	}
	if !task.opts.userSchedules {
		return "", nil, false
	}
	_, spec, job, ok, _ := task.opts.storage.Get(key)
	if !ok {
		return "", nil, false
	}
	return spec, job, true
}

// UserSchedulesEnabled reports the startup-time runtime capability. It does
// not follow configuration hot reloads because changing schedule storage and
// loaded cron entries requires a process restart.
func UserSchedulesEnabled() bool {
	task.mux.Lock()
	defer task.mux.Unlock()
	return task.opts.userSchedules
}

// Entry returns a cron entry.
func Entry(entryID cron.EntryID) cron.Entry {
	return task.opts.task.Entry(entryID)
}

// UpdateJob updates or creates a scheduled job.
func UpdateJob(key string, spec string, job cron.Job) error {
	task.jobsMu.Lock()
	defer task.jobsMu.Unlock()
	if _, _, _, ok, err := task.opts.systemSchedules.Get(key); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("%w: %q", ErrSystemScheduleImmutable, key)
	}
	if !task.opts.userSchedules {
		return ErrUserSchedulesDisabled
	}
	storedEntryID, _, _, stored, err := task.opts.storage.Get(key)
	if err != nil {
		return fmt.Errorf("get task schedule %q: %w", key, err)
	}
	loaded, loadedOK := task.loadedSchedules[key]
	if loadedOK && spec == loaded.spec {
		return nil
	}
	newEntryID, err := task.opts.task.AddJob(spec, job)
	if err != nil {
		slog.Error("task add job error", slog.Any("err", err))
		return err
	}
	if stored {
		err = task.opts.storage.Update(key, newEntryID)
	} else {
		err = task.opts.storage.Set(key, newEntryID, spec, job)
	}
	if err != nil {
		task.opts.task.Remove(newEntryID)
		if stored {
			err = errors.Join(err, task.opts.storage.Update(key, storedEntryID))
		}
		return fmt.Errorf("persist task schedule %q: %w", key, err)
	}
	if task.loadedSchedules == nil {
		task.loadedSchedules = make(map[string]runtimeSchedule)
	}
	task.loadedSchedules[key] = runtimeSchedule{spec: spec, entryID: newEntryID}
	if loadedOK && loaded.entryID != 0 {
		task.opts.task.Remove(loaded.entryID)
	}
	return nil
}

// RemoveJob removes a scheduled job.
func RemoveJob(key string) error {
	task.jobsMu.Lock()
	defer task.jobsMu.Unlock()
	if _, _, _, ok, err := task.opts.systemSchedules.Get(key); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("%w: %q", ErrSystemScheduleImmutable, key)
	}
	if !task.opts.userSchedules {
		return ErrUserSchedulesDisabled
	}
	_, _, _, stored, err := task.opts.storage.Get(key)
	if err != nil {
		return fmt.Errorf("get task schedule %q: %w", key, err)
	}
	loaded, loadedOK := task.loadedSchedules[key]
	if !stored && !loadedOK {
		return nil
	}
	if stored {
		if err := task.opts.storage.Remove(key); err != nil {
			return fmt.Errorf("remove task schedule %q: %w", key, err)
		}
	}
	if loadedOK {
		task.opts.task.Remove(loaded.entryID)
		delete(task.loadedSchedules, key)
	}
	return nil
}

// Options applies task options before Start.
func (e *Server) Options(opts ...Option) {
	e.mux.Lock()
	defer e.mux.Unlock()
	if e.started {
		return
	}
	e.jobsMu.Lock()
	defer e.jobsMu.Unlock()
	for _, option := range opts {
		if option != nil {
			option(&e.opts)
		}
	}
}

// String returns the server name.
func (e *Server) String() string {
	return "task"
}

// Start loads jobs, starts cron, and blocks until ctx is cancelled and running
// jobs have completed or the configured shutdown timeout expires.
func (e *Server) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	e.mux.Lock()
	if e.started {
		e.mux.Unlock()
		return ErrAlreadyStarted
	}
	e.ctx = ctx
	if e.opts.configurationErr != nil {
		e.mux.Unlock()
		return e.opts.configurationErr
	}
	e.jobsMu.Lock()

	persistentStorage := e.opts.storage
	if !e.opts.userSchedules {
		persistentStorage = nil
	}
	schedules, err := loadSchedules(e.opts.systemSchedules, persistentStorage)
	if err != nil {
		e.jobsMu.Unlock()
		e.mux.Unlock()
		return err
	}
	registered := make([]cron.EntryID, 0, len(schedules))
	for i, item := range schedules {
		entryID, addErr := e.opts.task.AddJob(item.spec, item.job)
		if addErr != nil {
			err = fmt.Errorf("add task schedule %q: %w", item.key, addErr)
			err = rollbackSchedules(e.opts.task, registered, schedules[:i], err)
			e.jobsMu.Unlock()
			e.mux.Unlock()
			slog.ErrorContext(ctx, "task add job error", slog.Any("err", err))
			return err
		}
		registered = append(registered, entryID)
		if updateErr := item.storage.Update(item.key, entryID); updateErr != nil {
			err = fmt.Errorf("update task schedule %q: %w", item.key, updateErr)
			err = rollbackSchedules(e.opts.task, registered, schedules[:i+1], err)
			e.jobsMu.Unlock()
			e.mux.Unlock()
			slog.ErrorContext(ctx, "task update job error", slog.Any("err", err))
			return err
		}
	}
	if e.loadedSchedules == nil {
		e.loadedSchedules = make(map[string]runtimeSchedule, len(schedules))
	}
	for i, item := range schedules {
		if previous, ok := e.loadedSchedules[item.key]; ok && previous.entryID != 0 {
			e.opts.task.Remove(previous.entryID)
		}
		e.loadedSchedules[item.key] = runtimeSchedule{spec: item.spec, entryID: registered[i]}
	}

	e.started = true
	e.opts.task.Start()
	e.jobsMu.Unlock()
	e.mux.Unlock()

	<-ctx.Done()
	return e.shutdownWithTimeout()
}

type loadedSchedule struct {
	key     string
	spec    string
	job     cron.Job
	entryID cron.EntryID
	storage Storage
}

func loadSchedules(systemSchedules, persistent Storage) ([]loadedSchedule, error) {
	result, systemKeys, err := loadSchedulesFrom(systemSchedules)
	if err != nil {
		return nil, fmt.Errorf("load task system schedules: %w", err)
	}
	persistentSchedules, _, err := loadSchedulesFrom(persistent)
	if err != nil {
		return nil, fmt.Errorf("load task persistent schedules: %w", err)
	}
	for _, item := range persistentSchedules {
		if _, reserved := systemKeys[item.key]; reserved {
			return nil, fmt.Errorf("%w: %q", ErrScheduleKeyConflict, item.key)
		}
		result = append(result, item)
	}
	return result, nil
}

func loadSchedulesFrom(storage Storage) ([]loadedSchedule, map[string]struct{}, error) {
	keysByName := make(map[string]struct{})
	if storage == nil {
		return nil, keysByName, nil
	}
	keys, err := storage.ListKeys()
	if err != nil {
		return nil, nil, err
	}
	keys = append([]string(nil), keys...)
	sort.Strings(keys)
	result := make([]loadedSchedule, 0, len(keys))
	for _, key := range keys {
		if _, duplicate := keysByName[key]; duplicate {
			return nil, nil, fmt.Errorf("task storage listed duplicate key %q", key)
		}
		entryID, spec, job, ok, getErr := storage.Get(key)
		if getErr != nil {
			return nil, nil, fmt.Errorf("get task schedule %q: %w", key, getErr)
		}
		if !ok || job == nil {
			return nil, nil, fmt.Errorf("task schedule %q was listed but is unavailable", key)
		}
		keysByName[key] = struct{}{}
		result = append(result, loadedSchedule{key: key, spec: spec, job: job, entryID: entryID, storage: storage})
	}
	return result, keysByName, nil
}

func removeEntries(cronServer *cron.Cron, entries []cron.EntryID) {
	for _, entryID := range entries {
		cronServer.Remove(entryID)
	}
}

func rollbackSchedules(
	cronServer *cron.Cron,
	registered []cron.EntryID,
	updated []loadedSchedule,
	primary error,
) error {
	removeEntries(cronServer, registered)
	result := primary
	for i := len(updated) - 1; i >= 0; i-- {
		item := updated[i]
		if err := item.storage.Update(item.key, item.entryID); err != nil {
			result = errors.Join(result, fmt.Errorf("restore task schedule %q entry ID: %w", item.key, err))
		}
	}
	return result
}

// Shutdown stops scheduling new work and waits for running jobs to finish.
func (e *Server) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	e.mux.Lock()
	started := e.started
	cronServer := e.opts.task
	e.mux.Unlock()
	if !started || cronServer == nil {
		return nil
	}

	stopped := cronServer.Stop()
	select {
	case <-stopped.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *Server) shutdownWithTimeout() error {
	if e.opts.shutdownTimeout <= 0 {
		return e.Shutdown(context.Background())
	}
	ctx, cancel := context.WithTimeout(context.Background(), e.opts.shutdownTimeout)
	defer cancel()
	return e.Shutdown(ctx)
}
