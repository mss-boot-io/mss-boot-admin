package task

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/2/21 15:35:43
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/2/21 15:35:43
 */

import (
	"context"
	"log/slog"

	"github.com/robfig/cron/v3"
)

var task = &Server{
	opts: setDefaultOption(),
}

// Server manage
type Server struct {
	ctx  context.Context
	opts options
}

// New server
func New(opts ...Option) *Server {
	task.Options(opts...)
	return task
}

// GetJob get job
func GetJob(key string) (string, cron.Job, bool) {
	_, spec, job, ok, _ := task.opts.storage.Get(key)
	if !ok {
		return "", nil, false
	}
	return spec, job, true
}

// Entry get entry
func Entry(entryID cron.EntryID) cron.Entry {
	return task.opts.task.Entry(entryID)

}

// UpdateJob update or create job
func UpdateJob(key string, spec string, job cron.Job) error {
	var err error
	entryID, entrySpec, _, ok, _ := task.opts.storage.Get(key)
	if ok && spec != entrySpec && entryID != 0 {
		task.opts.task.Remove(entryID)
		entryID, err = task.opts.task.AddJob(spec, job)
		if err != nil {
			slog.Error("task update job error", slog.Any("err", err))
			return err
		}
		return task.opts.storage.Update(key, entryID)
	}
	if ok && entryID != 0 {
		return nil
	}
	entryID, err = task.opts.task.AddJob(spec, job)
	if err != nil {
		slog.Error("task add job error", slog.Any("err", err))
		return err
	}
	return task.opts.storage.Update(key, entryID)
}

// RemoveJob remove job
func RemoveJob(key string) error {
	entryID, _, _, ok, _ := task.opts.storage.Get(key)
	if !ok {
		return nil
	}
	task.opts.task.Remove(entryID)
	return task.opts.storage.Remove(key)
}

// Options set options
func (e *Server) Options(opts ...Option) {
	for _, o := range opts {
		o(&e.opts)
	}
}

// String server name
func (e *Server) String() string {
	return "task"
}

// Start server
func (e *Server) Start(ctx context.Context) error {
	var err error
	e.ctx = ctx
	keys, _ := e.opts.storage.ListKeys()
	for i := range keys {
		_, spec, job, ok, _ := e.opts.storage.Get(keys[i])
		if !ok {
			continue
		}
		entryID, err := e.opts.task.AddJob(spec, job)
		if err != nil {
			slog.ErrorContext(ctx, "task add job error", slog.Any("err", err))
			return err
		}
		err = e.opts.storage.Update(keys[i], entryID)
		if err != nil {
			slog.ErrorContext(ctx, "task update job error", slog.Any("err", err))
			return err
		}
	}
	go func() {
		e.opts.task.Run()
		<-ctx.Done()
		err = e.Shutdown(ctx)
		if err != nil {
			slog.ErrorContext(ctx, e.String()+" Server shutdown error", slog.Any("err", err.Error()))
		}
	}()
	return nil
}

// Shutdown server
func (e *Server) Shutdown(_ context.Context) error {
	e.opts.task.Stop()
	return nil
}
