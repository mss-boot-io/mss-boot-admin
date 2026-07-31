package writer

import "time"

/*
 * @Author: lwnmengjing
 * @Date: 2021/6/3 8:33 上午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/3 8:33 上午
 */

// Options 可配置参数
type Options struct {
	path         string
	suffix       string // 文件扩展名
	cap          uint
	lokiURL      string
	lokiLabels   map[string]string
	bufferSize   uint
	lokiInterval time.Duration
}

func setDefault() Options {
	return Options{
		path:    "/tmp/go-admin",
		suffix:  "log",
		lokiURL: "http://localhost:3100/loki/api/v1/push",
		lokiLabels: map[string]string{
			"frame": "mss-boot",
		},
		bufferSize:   10000,
		lokiInterval: 5 * time.Second,
	}
}

// Option set options
type Option func(*Options)

// WithPath set path
func WithPath(s string) Option {
	return func(o *Options) {
		o.path = s
	}
}

// WithSuffix set suffix
func WithSuffix(s string) Option {
	return func(o *Options) {
		o.suffix = s
	}
}

// WithCap set cap
func WithCap(n uint) Option {
	return func(o *Options) {
		o.cap = n
	}
}

// WithLokiEndpoint set loki endpoint
func WithLokiEndpoint(s string) Option {
	return func(o *Options) {
		o.lokiURL = s
	}
}

// WithLokiLabels set loki labels
func WithLokiLabels(m map[string]string) Option {
	return func(o *Options) {
		o.lokiLabels = m
	}
}

// WithBufferSize set loki buffer size
func WithBufferSize(n uint) Option {
	return func(o *Options) {
		o.bufferSize = n
	}
}

// WithLokiInterval set loki interval
func WithLokiInterval(d time.Duration) Option {
	return func(o *Options) {
		o.lokiInterval = d
	}
}
