package task

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/robfig/cron/v3"
)

var (
	task = &Server{
		opts: setDefaultOption(),
	}
	// ErrAlreadyStarted is returned when a task server is started twice.
	ErrAlreadyStarted = errors.New("task server has already been started")
)

// Server manages scheduled jobs.
type Server struct {
	ctx     context.Context
	opts    options
	mux     sync.Mutex
	started bool
}

// New configures and returns the process-wide task server for compatibility.
func New(opts ...Option) *Server {
	task.Options(opts...)
	return task
}

// GetJob returns a configured job.
func GetJob(key string) (string, cron.Job, bool) {
	_, spec, job, ok, _ := task.opts.storage.Get(key)
	if !ok {
		return "", nil, false
	}
	return spec, job, true
}

// Entry returns a cron entry.
func Entry(entryID cron.EntryID) cron.Entry {
	return task.opts.task.Entry(entryID)
}

// UpdateJob updates or creates a scheduled job.
func UpdateJob(key string, spec string, job cron.Job) error {
	entryID, entrySpec, _, ok, _ := task.opts.storage.Get(key)
	if ok && spec != entrySpec && entryID != 0 {
		task.opts.task.Remove(entryID)
		newEntryID, err := task.opts.task.AddJob(spec, job)
		if err != nil {
			slog.Error("task update job error", slog.Any("err", err))
			return err
		}
		return task.opts.storage.Update(key, newEntryID)
	}
	if ok && entryID != 0 {
		return nil
	}
	newEntryID, err := task.opts.task.AddJob(spec, job)
	if err != nil {
		slog.Error("task add job error", slog.Any("err", err))
		return err
	}
	return task.opts.storage.Update(key, newEntryID)
}

// RemoveJob removes a scheduled job.
func RemoveJob(key string) error {
	entryID, _, _, ok, _ := task.opts.storage.Get(key)
	if !ok {
		return nil
	}
	task.opts.task.Remove(entryID)
	return task.opts.storage.Remove(key)
}

// Options applies task options before Start.
func (e *Server) Options(opts ...Option) {
	e.mux.Lock()
	defer e.mux.Unlock()
	if e.started {
		return
	}
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

	keys, err := e.opts.storage.ListKeys()
	if err != nil {
		e.mux.Unlock()
		return err
	}
	for _, key := range keys {
		_, spec, job, ok, getErr := e.opts.storage.Get(key)
		if getErr != nil {
			e.mux.Unlock()
			return getErr
		}
		if !ok {
			continue
		}
		entryID, addErr := e.opts.task.AddJob(spec, job)
		if addErr != nil {
			e.mux.Unlock()
			slog.ErrorContext(ctx, "task add job error", slog.Any("err", addErr))
			return addErr
		}
		if updateErr := e.opts.storage.Update(key, entryID); updateErr != nil {
			e.mux.Unlock()
			slog.ErrorContext(ctx, "task update job error", slog.Any("err", updateErr))
			return updateErr
		}
	}

	e.started = true
	e.opts.task.Start()
	e.mux.Unlock()

	<-ctx.Done()
	return e.shutdownWithTimeout()
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
