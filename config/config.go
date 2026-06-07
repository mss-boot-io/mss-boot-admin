package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot/pkg/config"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot/pkg/config/storage"
	"github.com/mss-boot-io/mss-boot/pkg/config/storage/cache"
	"github.com/mss-boot-io/mss-boot/pkg/config/storage/queue"
	responsegorm "github.com/mss-boot-io/mss-boot/pkg/response/actions/gorm"
	"gorm.io/gorm"
)

//go:embed *.yml
var FS embed.FS

var Cfg = &Config{}

const queryCacheTagPrefix = "gorm.cache:"

type queryCacheAdapter interface {
	Initialize(*gorm.DB) error
	RemoveFromTag(context.Context, string) error
}

type Config struct {
	Auth        Auth            `yaml:"auth" json:"auth"`
	GRPC        config.GRPC     `yaml:"grpc" json:"grpc"`
	Logger      config.Logger   `yaml:"logger" json:"logger"`
	Server      config.Listen   `yaml:"server" json:"server"`
	Listen      *config.Listen  `yaml:"listen" json:"listen"`
	Database    gormdb.Database `yaml:"database" json:"database"`
	Application Application     `yaml:"application" json:"application"`
	//OAuth2      *config.OAuth2  `yaml:"oauth2" json:"oauth2"`
	Task         Task            `yaml:"task" json:"task"`
	Pyroscope    Pyroscope       `yaml:"pyroscope" json:"pyroscope"`
	Cache        *config.Cache   `yaml:"cache" json:"cache"`
	Queue        *config.Queue   `yaml:"queue" json:"queue"`
	Locker       *config.Locker  `yaml:"locker" json:"locker"`
	Secret       *Secret         `yaml:"secret" json:"secret"`
	Storage      *config.Storage `yaml:"storage" json:"storage"`
	Clusters     Clusters        `yaml:"clusters" json:"clusters"`
	Notification Notification    `yaml:"notification" json:"notification"`
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
	//if e.Logger.Loki != nil && len(e.Application.Labels) > 0 {
	//	e.Logger.Loki.MergeLabels(e.Application.Labels)
	//}
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
		var cacheAdapter storage.AdapterCache
		// Cache.Init invokes set before queryCache in the same goroutine when Redis is configured.
		// bindQueryCache relies on that order so it can reuse the initialized cache adapter.
		e.Cache.Init(func(c storage.AdapterCache) {
			cacheAdapter = c
			center.SetCache(c)
			center.SetVerifyCodeStore(cache.NewVerifyCode(c))
		}, func(tx *gorm.DB, duration time.Duration) {
			bindQueryCache(cacheAdapter, tx, duration)
		})
	}
	if e.Queue != nil {
		e.Queue.Init(func(q storage.AdapterQueue) {
			center.SetQueue(q)
			w := queue.NewSampleWatcher(q)
			err = w.SetUpdateCallback(func(_ string) {
				err = gormdb.Enforcer.LoadPolicy()
				if err != nil {
					slog.Error("enforcer load policy failed", "err", err)
					return
				}
			})
			if err != nil {
				slog.Error("casbin set callback failed", slog.Any("err", err))
				os.Exit(-1)
			}
			err = gormdb.Enforcer.SetWatcher(w)
			if err != nil {
				slog.Error("casbin set watcher failed", slog.Any("err", err))
				os.Exit(-1)
			}
			gormdb.Enforcer.EnableAutoNotifyWatcher(true)
		})
	}
	if e.Locker != nil {
		e.Locker.Init(func(l storage.AdapterLocker) {
			center.SetLocker(l)
		})
	}
	if e.Storage != nil {
		e.Storage.Init()
	}
	if len(e.Clusters) > 0 {
		e.Clusters.Init()
	}
}

func bindQueryCache(cache queryCacheAdapter, tx *gorm.DB, _ time.Duration) {
	if tx == nil {
		return
	}
	if cache == nil {
		slog.Warn("query cache enabled but no cache adapter available; check cache.redis configuration")
		return
	}
	if err := cache.Initialize(tx); err != nil {
		slog.Error("query cache init failed", "err", err)
		return
	}
	responsegorm.CleanCacheFromTag = func(ctx context.Context, tag string) error {
		if tag == "" {
			slog.Warn("CleanCacheFromTag called with empty tag; model TableName() may be misconfigured")
			return nil
		}
		return cache.RemoveFromTag(ctx, queryCacheTagPrefix+tag)
	}
}

func (e *Config) OnChange() {
	e.Logger.Init()
	e.Database.Init()
	slog.Info("!!! cfg change and reload")
}
