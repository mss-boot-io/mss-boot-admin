package listener

/*
 * @Author: lwnmengjing
 * @Date: 2021/6/8 2:15 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/8 2:15 下午
 */

import (
	"net/http"
	"time"
)

// Option 参数设置类型
type Option func(*Options)

type Options struct {
	name, addr, certFile, keyFile string
	handler                       http.Handler
	startedHook                   func()
	endHook                       func()
	timeout                       time.Duration
	metrics                       bool
	healthz                       bool
	readyz                        bool
	pprof                         bool
}

func defaultOptions() *Options {
	return &Options{
		name:    "http",
		addr:    ":5000",
		timeout: 10 * time.Second,
		handler: http.DefaultServeMux,
	}
}

// WithName set name
func WithName(name string) Option {
	return func(o *Options) {
		o.name = name
	}
}

// WithMetrics set metrics
func WithMetrics(enable bool) Option {
	return func(o *Options) {
		o.metrics = enable
	}
}

// WithHealthz set healthz
func WithHealthz(enable bool) Option {
	return func(o *Options) {
		o.healthz = enable
	}
}

// WithReadyz set readyz
func WithReadyz(enable bool) Option {
	return func(o *Options) {
		o.readyz = enable
	}
}

// WithPprof set pprof
func WithPprof(enable bool) Option {
	return func(o *Options) {
		o.pprof = enable
	}
}

// WithEndHook set EndHook
func WithEndHook(f func()) Option {
	return func(o *Options) {
		o.endHook = f
	}
}

// WithStartedHook 设置启动回调函数
func WithStartedHook(f func()) Option {
	return func(o *Options) {
		o.startedHook = f
	}
}

// WithAddr 设置addr
func WithAddr(s string) Option {
	return func(o *Options) {
		o.addr = s
	}
}

// WithHandler 设置handler
func WithHandler(handler http.Handler) Option {
	return func(o *Options) {
		o.handler = handler
	}
}

// WithCert 设置cert
func WithCert(s string) Option {
	return func(o *Options) {
		o.certFile = s
	}
}

// WithKey 设置key
func WithKey(s string) Option {
	return func(o *Options) {
		o.keyFile = s
	}
}

// WithTimeout 设置timeout
func WithTimeout(t int) Option {
	return func(o *Options) {
		o.timeout = time.Second * time.Duration(t)
	}
}
