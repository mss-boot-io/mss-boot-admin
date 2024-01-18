package config

import (
	"github.com/grafana/pyroscope-go"
	"github.com/mss-boot-io/mss-boot-admin-api/center"
	"log/slog"
	"time"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/11 17:43:40
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/11 17:43:40
 */

type Pyroscope struct {
	Enabled                bool                    `yaml:"enabled" json:"enabled"`
	ApplicationName        string                  `yaml:"applicationName" json:"applicationName"` // e.g backend.purchases
	Tags                   map[string]string       `yaml:"tags" json:"tags"`
	ServerAddress          string                  `yaml:"serverAddress" json:"serverAddress"`         // e.g http://pyroscope.services.internal:4040
	AuthToken              string                  `yaml:"authToken" json:"authToken"`                 // specify this token when using pyroscope cloud
	BasicAuthUser          string                  `yaml:"basicAuthUser" json:"basicAuthUser"`         // http basic auth user
	BasicAuthPassword      string                  `yaml:"basicAuthPassword" json:"basicAuthPassword"` // http basic auth password
	TenantID               string                  `yaml:"tenantID" json:"tenantID"`
	UploadRate             time.Duration           `yaml:"uploadRate" json:"uploadRate"`
	Logger                 bool                    `yaml:"logger" json:"logger"`
	ProfileTypes           []pyroscope.ProfileType `yaml:"profileTypes" json:"profileTypes"`
	DisableGCRuns          bool                    `yaml:"disableGCRuns" json:"disableGCRuns"`                   // this will disable automatic runtime.GC runs between getting the heap profiles
	DisableAutomaticResets bool                    `yaml:"disableAutomaticResets" json:"disableAutomaticResets"` // disable automatic profiler reset every 10 seconds. Reset manually by calling Flush method
	HTTPHeaders            map[string]string       `yaml:"httpHeaders" json:"httpHeaders"`
}

func (e *Pyroscope) Init() {
	if e.Enabled {
		c := pyroscope.Config{
			Tags:                   e.Tags,
			ApplicationName:        e.ApplicationName,
			ServerAddress:          e.ServerAddress,
			BasicAuthUser:          e.BasicAuthUser,
			BasicAuthPassword:      e.BasicAuthPassword,
			TenantID:               e.TenantID,
			UploadRate:             e.UploadRate,
			ProfileTypes:           e.ProfileTypes,
			DisableGCRuns:          e.DisableGCRuns,
			DisableAutomaticResets: e.DisableAutomaticResets,
			HTTPHeaders:            e.HTTPHeaders,
		}
		if len(c.ProfileTypes) == 0 {
			c.ProfileTypes = pyroscope.DefaultProfileTypes
		}
		if e.Tags == nil {
			e.Tags = make(map[string]string)
		}
		if _, ok := e.Tags["stage"]; !ok {
			c.Tags["stage"] = center.Stage()
		}
		profiler, err := pyroscope.Start(c)
		if err != nil {
			slog.Error("pyroscope start failed", "err", err)
			return
		}
		center.SetProfiler(profiler)
		slog.Info("pyroscope start success")
	}
}
