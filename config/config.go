package config

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/center"
	frameworkconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/queue"
	responsegorm "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions/gorm"
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
	Auth        Auth                     `yaml:"auth" json:"auth"`
	GRPC        frameworkconfig.GRPC     `yaml:"grpc" json:"grpc"`
	Logger      frameworkconfig.Logger   `yaml:"logger" json:"logger"`
	Server      frameworkconfig.Listen   `yaml:"server" json:"server"`
	Listen      *frameworkconfig.Listen  `yaml:"listen" json:"listen"`
	Database    gormdb.Database          `yaml:"database" json:"database"`
	Application Application              `yaml:"application" json:"application"`
	Task        Task                     `yaml:"task" json:"task"`
	Pyroscope    Pyroscope                `yaml:"pyroscope" json:"pyroscope"`
	Cache        *frameworkconfig.Cache   `yaml:"cache" json:"cache"`
	Queue        *frameworkconfig.Queue   `yaml:"queue" json:"queue"`
	Locker       *frameworkconfig.Locker  `yaml:"locker" json:"locker"`
	Secret       *Secret                  `yaml:"secret" json:"secret"`
	Storage      *frameworkconfig.Storage `yaml:"storage" json:"storage"`
	Clusters     Clusters                 `yaml:"clusters" json:"clusters"`
	Notification Notification             `yaml:"notification" json:"notification"`

	databaseMu     sync.Mutex
	databaseHandle *gormdb.Handle
}

type SecretConfig struct {
	Secret *Secret `yaml:"secret" json:"secret"`
}

func (s *SecretConfig) Init() {
	if s.Secret != nil {
		s.Secret.Init()
	}
}

// Init preserves the ConfigImp compatibility surface. New startup code should
// call InitContext and propagate its error.
func (e *Config) Init(opts ...source.Option) {
	if err := e.InitContext(context.Background(), opts...); err != nil {
		slog.Error("configuration initialization failed", "err", err)
	}
}

// InitContext loads configuration, opens an owned database Handle, publishes it
// through the legacy compatibility bridge, and initializes dependent adapters.
// If a dependent adapter fails, the new Handle is closed and the previous
// default is restored.
func (e *Config) InitContext(ctx context.Context, opts ...source.Option) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	e.databaseMu.Lock()
	defer e.databaseMu.Unlock()

	secretConfig := &SecretConfig{}
	opts = append(opts, source.WithPrefixHook(secretConfig))
	if err := frameworkconfig.Init(e, opts...); err != nil {
		return fmt.Errorf("initialize configuration source: %w", err)
	}

	if e.Pyroscope.Enabled && len(e.Application.Labels) > 0 {
		e.Pyroscope.MergeTags(e.Application.Labels)
	}
	e.Logger.Init()

	newHandle, err := e.Database.Open(ctx)
	if err != nil {
		return fmt.Errorf("initialize application database: %w", err)
	}
	previousDefault := gormdb.InstallDefault(newHandle)
	committed := false
	defer func() {
		if committed {
			return
		}
		gormdb.InstallDefault(previousDefault)
		if closeErr := newHandle.Close(); closeErr != nil {
			err = errorsJoin(err, fmt.Errorf("close failed database handle: %w", closeErr))
		}
	}()

	if e.Pyroscope.ApplicationName == "" {
		e.Pyroscope.ApplicationName = e.Application.Name
	}
	e.Pyroscope.Init()

	if e.Cache != nil {
		warnQueryCacheDuration(e.Cache)
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
		var policyWatcherErr error
		e.Queue.Init(func(q storage.AdapterQueue) {
			policyWatcherErr = bindPolicyWatcher(q, newHandle.Enforcer)
			if policyWatcherErr == nil {
				center.SetQueue(q)
			}
		})
		if policyWatcherErr != nil {
			return fmt.Errorf("initialize policy watcher: %w", policyWatcherErr)
		}
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

	oldHandle := e.databaseHandle
	e.databaseHandle = newHandle
	committed = true
	if oldHandle != nil && oldHandle != newHandle {
		if closeErr := oldHandle.Close(); closeErr != nil {
			return fmt.Errorf("close previous application database: %w", closeErr)
		}
	}
	return nil
}

// DatabaseHandle returns the database Handle owned by this Config.
func (e *Config) DatabaseHandle() *gormdb.Handle {
	e.databaseMu.Lock()
	defer e.databaseMu.Unlock()
	return e.databaseHandle
}

// Close releases the database Handle owned by this Config. It only clears the
// legacy globals when the same Handle is still installed.
func (e *Config) Close() error {
	e.databaseMu.Lock()
	handle := e.databaseHandle
	e.databaseHandle = nil
	if handle != nil {
		gormdb.ClearDefault(handle)
	}
	e.databaseMu.Unlock()
	if handle == nil {
		return nil
	}
	return handle.Close()
}

func warnQueryCacheDuration(cacheConfig *frameworkconfig.Cache) {
	if cacheConfig != nil && cacheConfig.QueryCache && cacheConfig.QueryCacheDuration <= 0 {
		slog.Warn("cache.queryCache enabled but queryCacheDuration is zero; query cache plugin will not register; set queryCacheDuration > 0")
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
		if err := cache.RemoveFromTag(ctx, queryCacheTagPrefix+tag); err != nil {
			slog.ErrorContext(ctx, "query cache invalidation failed", "tag", tag, "err", err)
			return err
		}
		return nil
	}
}

func bindPolicyWatcher(adapter storage.AdapterQueue, enforcer casbin.IEnforcer) error {
	if adapter == nil || enforcer == nil {
		return nil
	}
	watcher := queue.NewSampleWatcher(adapter)
	if err := watcher.SetUpdateCallback(func(string) {
		if err := enforcer.LoadPolicy(); err != nil {
			slog.Error("enforcer load policy failed", "err", err)
		}
	}); err != nil {
		return fmt.Errorf("set Casbin watcher callback: %w", err)
	}
	if err := enforcer.SetWatcher(watcher); err != nil {
		return fmt.Errorf("set Casbin watcher: %w", err)
	}
	enforcer.EnableAutoNotifyWatcher(true)
	return nil
}

func (e *Config) OnChange() {
	e.Logger.Init()
	if err := e.reloadDatabase(context.Background()); err != nil {
		slog.Error("configuration changed but database reload failed; keeping previous handle", "err", err)
		return
	}
	slog.Info("configuration changed and database handle reloaded")
}

func (e *Config) reloadDatabase(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	e.databaseMu.Lock()
	defer e.databaseMu.Unlock()

	newHandle, err := e.Database.Open(ctx)
	if err != nil {
		return fmt.Errorf("open replacement database: %w", err)
	}
	if adapter := center.GetQueue(); adapter != nil {
		if err := bindPolicyWatcher(adapter, newHandle.Enforcer); err != nil {
			_ = newHandle.Close()
			return fmt.Errorf("bind replacement policy watcher: %w", err)
		}
	}

	oldHandle := e.databaseHandle
	gormdb.InstallDefault(newHandle)
	e.databaseHandle = newHandle
	if oldHandle != nil && oldHandle != newHandle {
		if err := oldHandle.Close(); err != nil {
			return fmt.Errorf("close replaced database handle: %w", err)
		}
	}
	return nil
}

func errorsJoin(left, right error) error {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	return fmt.Errorf("%w; %v", left, right)
}
