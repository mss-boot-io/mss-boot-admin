package task

import (
	"errors"
	"fmt"
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
	task             *cron.Cron
	storage          Storage
	systemSchedules  *defaultStorage
	userSchedules    bool
	configurationErr error
	shutdownTimeout  time.Duration
}

// WithSchedule registers a schedule in the configured Storage. This preserves
// the original public behavior for callers that use persistent task storage.
func WithSchedule(key string, spec string, job cron.Job) Option {
	return func(o *options) {
		if o.storage == nil {
			o.configurationErr = errors.Join(o.configurationErr, errors.New("task storage is required"))
			return
		}
		if err := o.storage.Set(key, 0, spec, job); err != nil {
			o.configurationErr = errors.Join(o.configurationErr, fmt.Errorf("register task schedule %q: %w", key, err))
		}
	}
}

// WithSystemSchedule registers an immutable, process-local system schedule.
// System schedules deliberately live outside the configured Storage so
// database-backed user tasks cannot shadow them or make their registration
// order-dependent.
func WithSystemSchedule(key string, spec string, job cron.Job) Option {
	return func(o *options) {
		if key == "" {
			o.configurationErr = errors.Join(o.configurationErr, errors.New("task system schedule key is required"))
			return
		}
		if spec == "" {
			o.configurationErr = errors.Join(o.configurationErr, fmt.Errorf("task system schedule %q spec is required", key))
			return
		}
		if job == nil {
			o.configurationErr = errors.Join(o.configurationErr, fmt.Errorf("task system schedule %q job is required", key))
			return
		}
		if o.systemSchedules == nil {
			o.systemSchedules = &defaultStorage{schedules: make(map[string]*schedule)}
		}
		if _, _, _, exists, err := o.systemSchedules.Get(key); err != nil {
			o.configurationErr = errors.Join(o.configurationErr, fmt.Errorf("get task system schedule %q: %w", key, err))
			return
		} else if exists {
			o.configurationErr = errors.Join(o.configurationErr, fmt.Errorf("task system schedule %q is registered more than once", key))
			return
		}
		if err := o.systemSchedules.Set(key, 0, spec, job); err != nil {
			o.configurationErr = errors.Join(o.configurationErr, fmt.Errorf("register task system schedule %q: %w", key, err))
		}
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

// WithUserSchedulesEnabled controls the runtime API for user-managed
// schedules. System schedules remain active when this is false. The option is
// evaluated at server startup and deliberately does not follow configuration
// hot reloads; changing it requires a process restart.
func WithUserSchedulesEnabled(enabled bool) Option {
	return func(o *options) {
		o.userSchedules = enabled
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
		task: cron.New(
			cron.WithSeconds(),
			cron.WithChain(cron.Recover(cron.DefaultLogger)),
		),
		storage:         &defaultStorage{schedules: make(map[string]*schedule)},
		systemSchedules: &defaultStorage{schedules: make(map[string]*schedule)},
		userSchedules:   true,
		shutdownTimeout: defaultShutdownTimeout,
	}
}
