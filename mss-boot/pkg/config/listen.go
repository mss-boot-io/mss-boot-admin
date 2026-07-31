package config

/*
 * @Author: lwnmengjing
 * @Date: 2021/6/17 4:25 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/17 4:25 下午
 */

import (
	"github.com/mss-boot-io/mss-boot/core/server"
	"github.com/mss-boot-io/mss-boot/core/server/listener"
)

// Listen tcp listener config
type Listen struct {
	Name     string `yaml:"name" json:"name"`
	Addr     string `yaml:"addr" json:"addr"`
	CertFile string `yaml:"certFile" json:"certFile"`
	KeyFile  string `yaml:"keyFile" json:"keyFile"`
	Timeout  int    `yaml:"timeout" json:"timeout"` // default: 10s
	Metrics  bool   `yaml:"metrics" json:"metrics"`
	Healthz  bool   `yaml:"healthz" json:"healthz"`
	Readyz   bool   `yaml:"readyz" json:"readyz"`
	Pprof    bool   `yaml:"pprof" json:"pprof"`
}

// Init tcp listener
func (e *Listen) Init(opts ...listener.Option) server.Runnable {
	if e == nil {
		return nil
	}
	if opts == nil {
		opts = make([]listener.Option, 0)
	}
	if e.Timeout == 0 {
		e.Timeout = 10
	}
	if e.Name != "" {
		opts = append(opts, listener.WithName(e.Name))
	}
	if e.Addr != "" {
		opts = append(opts, listener.WithAddr(e.Addr))
	}
	if e.CertFile != "" {
		opts = append(opts, listener.WithCert(e.CertFile))
	}
	if e.KeyFile != "" {
		opts = append(opts, listener.WithKey(e.KeyFile))
	}
	opts = append(opts,
		listener.WithTimeout(e.Timeout),
		listener.WithMetrics(e.Metrics),
		listener.WithHealthz(e.Healthz),
		listener.WithReadyz(e.Readyz),
		listener.WithPprof(e.Pprof),
	)
	return listener.New(opts...)
}
