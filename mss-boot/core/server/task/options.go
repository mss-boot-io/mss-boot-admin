package task

import (
	"time"

	"github.com/robfig/cron/v3"
)

const defaultShutdownTimeout = 5 * time.Second

// Option configures the task server.
type Option func(*options)

type schedule struct {
	spec    string
	job     cron.Job
	entryID cron.EntryID
}

type options struct {
	task            *cron.Cron
	storage         Storage
	shutdownTimeout time.Duration
}

// WithSchedule registers a schedule in task storage.
func WithSchedule(key string, spec string, job cron.Job) Option {
	return func(o *options) {
		_ = o.storage.Set(key, 0, spec, job)
	}
}

// WithStorage replaces task storage.
func WithStorage(storage Storage) Option {
	return func(o *options) {
		if storage != nil {
			o.storage = storage
		}
	}
}

// WithShutdownTimeout limits how long Start waits for running cron jobs after
// cancellation. A non-positive duration waits without a timeout.
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.shutdownTimeout = timeout
	}
}

func setDefaultOption() options {
	return options{
		task:            cron.New(cron.WithSeconds(), cron.WithChain()),
		storage:         &defaultStorage{schedules: make(map[string]*schedule)},
		shutdownTimeout: defaultShutdownTimeout,
	}
}
