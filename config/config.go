package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

import (
	"embed"
	"log/slog"

	"github.com/mss-boot-io/mss-boot/pkg/config"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/config/source"
)

//go:embed *.yml
var FS embed.FS

var Cfg = &Config{}

type Config struct {
	Auth        Auth            `yaml:"auth" json:"auth"`
	GRPC        config.GRPC     `yaml:"grpc" json:"grpc"`
	Logger      config.Logger   `yaml:"logger" json:"logger"`
	Server      config.Listen   `yaml:"server" json:"server"`
	Listen      *config.Listen  `yaml:"listen" json:"listen"`
	Database    gormdb.Database `yaml:"database" json:"database"`
	Application Application     `yaml:"application" json:"application"`
	//OAuth2      *config.OAuth2  `yaml:"oauth2" json:"oauth2"`
	Task      Task            `yaml:"task" json:"task"`
	Pyroscope Pyroscope       `yaml:"pyroscope" json:"pyroscope"`
	Cache     *Cache          `yaml:"cache" json:"cache"`
	Queue     *Queue          `yaml:"queue" json:"queue"`
	Locker    *Locker         `yaml:"locker" json:"locker"`
	Secret    *Secret         `yaml:"secret" json:"secret"`
	Storage   *config.Storage `yaml:"storage" json:"storage"`
	Clusters  Clusters        `yaml:"clusters" json:"clusters"`
}

type SecretConfig struct {
	Secret *Secret `yaml:"secret" json:"secret"`
}

func (s *SecretConfig) Init() {
	if s.Secret != nil {
		s.Secret.Init()
	}
}

func (e *Config) Init(opts ...source.Option) {
	sc := &SecretConfig{}
	opts = append(opts, source.WithPrefixHook(sc))

	err := config.Init(e, opts...)
	if err != nil {
		slog.Error("cfg init failed", "err", err)
	}
	if e.Logger.Loki != nil && len(e.Application.Labels) > 0 {
		e.Logger.Loki.MergeLabels(e.Application.Labels)
	}
	if e.Pyroscope.Enabled && len(e.Application.Labels) > 0 {
		e.Pyroscope.MergeTags(e.Application.Labels)
	}
	e.Logger.Init()
	e.Database.Init()
	if e.Pyroscope.ApplicationName == "" {
		e.Pyroscope.ApplicationName = e.Application.Name
	}
	e.Pyroscope.Init()

	if e.Cache != nil {
		e.Cache.Init()
	}
	if e.Queue != nil {
		e.Queue.Init()
	}
	if e.Storage != nil {
		e.Storage.Init()
	}
	if len(e.Clusters) > 0 {
		e.Clusters.Init()
	}
}

func (e *Config) OnChange() {
	e.Logger.Init()
	e.Database.Init()
	slog.Info("!!! cfg change and reload")
}
