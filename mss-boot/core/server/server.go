package server

/*
 * @Author: lwnmengjing
 * @Date: 2021/6/7 5:43 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/7 5:43 下午
 */

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Server server
type Server struct {
	services               map[string]Runnable
	started                sync.Map
	mux                    sync.Mutex
	errChan                chan error
	waitForRunnable        sync.WaitGroup
	internalCtx            context.Context
	internalCancel         context.CancelFunc
	internalProceduresStop chan struct{}
	shutdownCtx            context.Context
	shutdownCancel         context.CancelFunc
	opts                   Options
}

// New 实例化
func New(opts ...Option) Manager {
	s := &Server{
		services:               make(map[string]Runnable),
		errChan:                make(chan error),
		internalProceduresStop: make(chan struct{}),
	}
	s.opts = setDefaultOptions()
	for i := range opts {
		opts[i](&s.opts)
	}
	return s
}

// Add runnable
func (e *Server) Add(r ...Runnable) {
	if e.services == nil {
		e.services = make(map[string]Runnable)
	}
	for i := range r {
		if r[i] == nil {
			continue
		}
		e.services[r[i].String()] = r[i]
	}
}

// Start runnable
func (e *Server) Start(ctx context.Context) (err error) {
	//e.mux.Lock()
	//defer e.mux.Unlock()
	e.internalCtx, e.internalCancel = context.WithCancel(ctx)
	stopComplete := make(chan struct{})
	defer close(stopComplete)
	defer func() {
		stopErr := e.engageStopProcedure(stopComplete)
		if stopErr != nil {
			if err != nil {
				err = fmt.Errorf("%s, %w", stopErr.Error(), err)
			} else {
				err = stopErr
			}
		}
	}()
	e.errChan = make(chan error, len(e.services))

	for k := range e.services {
		if e.getStarted(k) {
			//先判断是否可以启动
			return errors.New("can't accept new runnable as stop procedure is already engaged")
		}
	}
	//按顺序启动
	for k := range e.services {
		e.startRunnable(e.services[k])
		e.setStarted(k)
	}
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	e.waitForRunnable.Wait()
	select {
	case <-ctx.Done():
		return nil
	case err = <-e.errChan:
		return err
	case <-done:
		// 优雅退出
		fmt.Println("received SIGINT, shutting down server")
		return nil
	}
}

func (e *Server) startRunnable(r Runnable) {
	e.waitForRunnable.Add(1)
	go func() {
		defer e.waitForRunnable.Done()
		if err := r.Start(e.internalCtx); err != nil {
			e.errChan <- err
		}
	}()
}

func (e *Server) engageStopProcedure(stopComplete <-chan struct{}) error {
	if e.opts.gracefulShutdownTimeout > 0 {
		e.shutdownCtx, e.shutdownCancel = context.WithTimeout(
			context.Background(), e.opts.gracefulShutdownTimeout)
	} else {
		e.shutdownCtx, e.shutdownCancel = context.WithCancel(context.Background())
	}
	defer e.shutdownCancel()
	close(e.internalProceduresStop)
	e.internalCancel()

	go func() {
		for {
			select {
			case err, ok := <-e.errChan:
				if ok {
					slog.Error("error received after stop sequence was engaged", slog.Any("err", err))
				}
			case <-stopComplete:
				return
			}
		}
	}()
	if e.opts.gracefulShutdownTimeout == 0 {
		return nil
	}
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.waitForRunnableToEnd(e.shutdownCancel)
}

func (e *Server) waitForRunnableToEnd(shutdownCancel context.CancelFunc) error {
	go func() {
		e.waitForRunnable.Wait()
		shutdownCancel()
	}()
	<-e.shutdownCtx.Done()
	if err := e.shutdownCtx.Err(); err != nil && err != context.Canceled {
		return fmt.Errorf(
			"failed waiting for all runnables to end within grace period of %s: %w",
			e.opts.gracefulShutdownTimeout, err)
	}
	return nil
}

func (e *Server) setStarted(key string) {
	e.started.Store(key, true)
}

func (e *Server) getStarted(keys ...string) bool {
	var start bool
	for i := range keys {
		v, ok := e.started.Load(keys[i])
		start = start || (ok && v.(bool))
		if start {
			return start
		}
	}
	return start
}
