/*
 * @Author: lwnmengjing
 * @Date: 2023/5/1 19:58:03
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/5/1 19:58:03
 */

package config

import (
	"net/http"

	log "github.com/mss-boot-io/mss-boot/core/logger"
	"github.com/mss-boot-io/mss-boot/core/server"
	"github.com/mss-boot-io/mss-boot/core/server/listener"
	"github.com/mss-boot-io/mss-boot/pkg/config"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/config/source"
)

var Cfg Config

type Config struct {
	Logger   config.Logger    `yaml:"logger" json:"logger"`
	Server   config.Listen    `yaml:"server" json:"server"`
	Listen   *config.Listen   `yaml:"listen" json:"listen"`
	Database *gormdb.Database `yaml:"database" json:"database"`
}

func (e *Config) Init(handler http.Handler) {
	opts := []source.Option{
		source.WithDir("config"),
		source.WithProvider(source.Local),
	}
	err := config.Init(e, opts...)
	if err != nil {
		log.Fatalf("cfg init failed, %s\n", err.Error())
	}
	log.Info(e)

	e.Logger.Init()
	e.Database.Init()

	runnable := []server.Runnable{
		listener.New("admin",
			e.Server.Init(listener.WithHandler(handler))...),
	}
	if e.Listen != nil {
		runnable = append(runnable, listener.New("listen", e.Listen.Init()...))
	}

	server.Manage.Add(runnable...)
}

func (e *Config) OnChange() {
	e.Logger.Init()
	e.Database.Init()
	log.Info("!!! cfg change and reload")
}
