package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:11:42
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:11:42
 */

import (
	"github.com/mss-boot-io/mss-boot/core/server"
	"github.com/mss-boot-io/mss-boot/core/server/listener"
	"github.com/mss-boot-io/mss-boot/pkg/config"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type Mode string

const (
	ModeDev  Mode = "dev"
	ModeTest Mode = "test"
	ModeProd Mode = "prod"
)

type Application struct {
	Name       string            `yaml:"name" json:"name"`
	Mode       Mode              `yaml:"mode" json:"mode"`
	Origin     string            `yaml:"origin" json:"origin"`
	StaticPath map[string]string `yaml:"staticPath" json:"staticPath"`
	Labels     map[string]string `yaml:"labels" json:"labels"`
	UI         UIServer          `yaml:"ui" json:"ui"`
}

func (e *Application) Init(r gin.IRouter) {
	if e.Mode == "" {
		e.Mode = ModeDev
	}

	switch e.Mode {
	case ModeDev:
		// set gin mode
		gin.SetMode(gin.DebugMode)

		// set static path
		for k := range e.StaticPath {
			//if k == "404" {
			//	r.NoRoute(func(c *gin.Context) {
			//		c.File(e.StaticPath[k])
			//	})
			//	continue
			//}
			if filepath.Ext(k) != "" {
				r.StaticFile(k, e.StaticPath[k])
				continue
			}
			r.Static(k, e.StaticPath[k])
		}
		r.StaticFile("/swagger.json", "docs/swagger.json")
		r.StaticFile("/swagger.yaml", "docs/swagger.yaml")
	case ModeTest:
		// set gin mode
		gin.SetMode(gin.TestMode)
		// no static
	case ModeProd:
		// set gin mode
		gin.SetMode(gin.ReleaseMode)
		// no static
	}
}

type UIServer struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	Path          string `yaml:"path" json:"path"`
	config.Listen `yaml:",inline" json:",inline"`
}

func (u *UIServer) Init() server.Runnable {
	if !u.Enabled {
		return nil
	}

	r := gin.Default()
	r.Static("/", u.Path)
	r.LoadHTMLFiles(filepath.Join(u.Path, "index.html"))
	r.NoRoute(func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})
	return u.Listen.Init(listener.WithHandler(r))
}
