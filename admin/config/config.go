package config

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	frameworkconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
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
	Auth         Auth                     `yaml:"auth" json:"auth"`
	CORS         CORS                     `yaml:"cors" json:"cors"`
	GRPC         frameworkconfig.GRPC     `yaml:"grpc" json:"grpc"`
	Logger       frameworkconfig.Logger   `yaml:"logger" json:"logger"`
	Server       frameworkconfig.Listen   `yaml:"server" json:"server"`
	Listen       *frameworkconfig.Listen  `yaml:"listen" json:"listen"`
	Database     gormdb.Database          `yaml:"database" json:"database"`
	Application  Application              `yaml:"application" json:"application"`
	Challenge    Challenge                `yaml:"challenge" json:"challenge"`
	Monitor      Monitor                  `yaml:"monitor" json:"monitor"`
	Task         Task                     `yaml:"task" json:"task"`
	Pyroscope    Pyroscope                `yaml:"pyroscope" json:"pyroscope"`
	Cache        *frameworkconfig.Cache   `yaml:"cache" json:"cache"`
	Queue        *frameworkconfig.Queue   `yaml:"queue" json:"queue"`
	Locker       *frameworkconfig.Locker  `yaml:"locker" json:"locker"`
	Secret       *Secret                  `yaml:"secret" json:"secret"`
	Storage      *frameworkconfig.Storage `yaml:"storage" json:"storage"`
	Clusters     Clusters                 `yaml:"clusters" json:"clusters"`
	Notification Notification             `yaml:"notification" json:"notification"`

	databaseMu     sync.RWMutex
	databaseHandle *gormdb.Handle
	databaseLeases map[*gormdb.Handle]*sync.WaitGroup
	databaseReload sync.Mutex
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
	e.databaseReload.Lock()
	defer e.databaseReload.Unlock()

	secretConfig := &SecretConfig{}
	opts = append(opts, source.WithPrefixHook(secretConfig))
	if err := frameworkconfig.Init(e, opts...); err != nil {
		return fmt.Errorf("initialize configuration source: %w", err)
	}
	if err := validateProductionAuthKey(e.Application.Mode, e.Auth.Key); err != nil {
		return err
	}
	if err := e.Challenge.Validate(); err != nil {
		return err
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
			err = errors.Join(err, fmt.Errorf("close failed database handle: %w", closeErr))
		}
	}()

	if e.Pyroscope.ApplicationName == "" {
		e.Pyroscope.ApplicationName = e.Application.Name
	}
	e.Pyroscope.Init()

	var challengeErr error
	var challengeBound bool
	var challengeCandidate center.ChallengeImp
	if e.Cache != nil {
		warnQueryCacheDuration(e.Cache)
		var cacheAdapter storage.AdapterCache
		// Cache.Init invokes set before queryCache in the same goroutine when Redis is configured.
		// bindQueryCache relies on that order so it can reuse the initialized cache adapter.
		e.Cache.Init(func(c storage.AdapterCache) {
			cacheAdapter = c
			center.SetCache(c)
			if e.Challenge.Enabled {
				var built center.ChallengeImp
				built, challengeErr = e.Challenge.Build(c)
				if challengeErr == nil {
					challengeCandidate = built
					challengeBound = true
				}
			}
		}, func(tx *gorm.DB, duration time.Duration) {
			bindQueryCache(cacheAdapter, tx, duration)
		})
	}
	if e.Challenge.Enabled {
		if challengeErr != nil {
			if errors.Is(challengeErr, ErrChallengeConfigurationInvalid) {
				return challengeErr
			}
			slog.Warn("email challenge dependency unavailable; related flows are disabled")
		}
		if !challengeBound && challengeErr == nil {
			slog.Warn("email challenge Redis resource unavailable; related flows are disabled")
		}
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
	// Publish exactly the snapshot described by this Config. If an optional
	// dependency is unavailable, the new snapshot is explicitly nil so a stale
	// pepper/TTL policy from an older Config cannot survive a successful reload.
	center.SetChallenge(challengeCandidate)

	oldHandle, oldLeases := e.swapDatabaseHandle(newHandle)
	committed = true
	if oldHandle != nil && oldHandle != newHandle {
		if closeErr := e.retireDatabaseHandle(oldHandle, oldLeases); closeErr != nil {
			return fmt.Errorf("close previous application database: %w", closeErr)
		}
	}
	return nil
}

// DatabaseHandle returns the database Handle owned by this Config.
func (e *Config) DatabaseHandle() *gormdb.Handle {
	e.databaseMu.RLock()
	defer e.databaseMu.RUnlock()
	return e.databaseHandle
}

// WithDatabase leases the currently published GORM handle for one bounded
// operation. Configuration reload publishes the replacement for new leases,
// then waits for operations using the previous handle before closing its pool.
// Callers must not retain the leased *gorm.DB after operation returns.
func (e *Config) WithDatabase(operation func(*gorm.DB) error) error {
	if operation == nil {
		return errors.New("database operation is required")
	}
	e.databaseMu.Lock()
	handle := e.databaseHandle
	if handle == nil || handle.DB == nil {
		e.databaseMu.Unlock()
		return errors.New("application database handle is not initialized")
	}
	if e.databaseLeases == nil {
		e.databaseLeases = make(map[*gormdb.Handle]*sync.WaitGroup)
	}
	leases := e.databaseLeases[handle]
	if leases == nil {
		leases = &sync.WaitGroup{}
		e.databaseLeases[handle] = leases
	}
	leases.Add(1)
	e.databaseMu.Unlock()
	defer leases.Done()
	return operation(handle.DB)
}

// Close releases the database Handle owned by this Config. It only clears the
// legacy globals when the same Handle is still installed.
func (e *Config) Close() error {
	e.databaseReload.Lock()
	defer e.databaseReload.Unlock()
	handle, leases := e.swapDatabaseHandle(nil)
	if handle != nil {
		gormdb.ClearDefault(handle)
	}
	if handle == nil {
		return nil
	}
	return e.retireDatabaseHandle(handle, leases)
}

func (e *Config) swapDatabaseHandle(next *gormdb.Handle) (*gormdb.Handle, *sync.WaitGroup) {
	e.databaseMu.Lock()
	defer e.databaseMu.Unlock()
	previous := e.databaseHandle
	e.databaseHandle = next
	if next != nil {
		if e.databaseLeases == nil {
			e.databaseLeases = make(map[*gormdb.Handle]*sync.WaitGroup)
		}
		if e.databaseLeases[next] == nil {
			e.databaseLeases[next] = &sync.WaitGroup{}
		}
	}
	return previous, e.databaseLeases[previous]
}

func (e *Config) retireDatabaseHandle(handle *gormdb.Handle, leases *sync.WaitGroup) error {
	if handle == nil {
		return nil
	}
	if leases != nil {
		leases.Wait()
	}
	err := handle.Close()
	e.databaseMu.Lock()
	if e.databaseHandle != handle {
		delete(e.databaseLeases, handle)
	}
	e.databaseMu.Unlock()
	return err
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
		slog.Error("configuration database reload did not complete cleanly", "err", err)
		return
	}
	slog.Info("configuration changed and database handle reloaded")
}

func (e *Config) reloadDatabase(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	e.databaseReload.Lock()
	defer e.databaseReload.Unlock()

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

	gormdb.InstallDefault(newHandle)
	oldHandle, oldLeases := e.swapDatabaseHandle(newHandle)
	if oldHandle != nil && oldHandle != newHandle {
		if err := e.retireDatabaseHandle(oldHandle, oldLeases); err != nil {
			return fmt.Errorf("close replaced database handle: %w", err)
		}
	}
	return nil
}
