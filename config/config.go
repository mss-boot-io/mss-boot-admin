package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

import (
	"log/slog"

	"github.com/mss-boot-io/mss-boot/pkg/config"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/config/source"
)

var Cfg Config

type Config struct {
	Auth        Auth             `yaml:"auth" json:"auth"`
	Logger      config.Logger    `yaml:"logger" json:"logger"`
	Server      config.Listen    `yaml:"server" json:"server"`
	Listen      *config.Listen   `yaml:"listen" json:"listen"`
	Database    *gormdb.Database `yaml:"database" json:"database"`
	Application Application      `yaml:"application" json:"application"`
	OAuth2      *config.OAuth2   `yaml:"oauth2" json:"oauth2"`
}

func (e *Config) Init() {
	opts := []source.Option{
		source.WithDir("config"),
		source.WithProvider(source.Local),
	}
	err := config.Init(e, opts...)
	if err != nil {
		slog.Error("cfg init failed", "err", err)
	}

	e.Logger.Init()
	e.Database.Init()
}

func (e *Config) OnChange() {
	e.Logger.Init()
	e.Database.Init()
	slog.Info("!!! cfg change and reload")
}