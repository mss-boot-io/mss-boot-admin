package config

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
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
	Runtime      runtimeconfig.Config     `yaml:"runtime" json:"runtime"`
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
	queueMu        sync.RWMutex
	managedQueue   storage.ManagedAdapterQueue
	retiringQueue  storage.ManagedAdapterQueue
	queueInit      func(context.Context, func(storage.AdapterQueue) error) error
	runtimeMu      sync.Mutex
	runtimeOwner   *challengeRuntimeOwner

	objectStorageMu             sync.Mutex
	objectStorageCloseMu        sync.Mutex
	objectStorageHandle         *frameworkconfig.StorageHandle
	objectStorageLocalRoot      *os.Root
	objectStorageLocalURLPrefix string
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
	if err := frameworkconfig.InitContext(ctx, e, opts...); err != nil {
		return fmt.Errorf("initialize configuration source: %w", err)
	}
	if err := validateProductionAuthKey(e.Application.Mode, e.Auth.Key); err != nil {
		return err
	}
	if err := validateBrowserSession(e.Application.Mode, e.Auth); err != nil {
		return err
	}
	if err := validateBrowserSessionOrigins(
		e.Application.Mode,
		e.Auth,
		e.Application.Origin,
		e.CORS.AllowOrigins,
		e.CORS.AllowHeaders,
	); err != nil {
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

	if e.Cache != nil {
		warnQueryCacheDuration(e.Cache)
		var cacheAdapter storage.AdapterCache
		// Cache.Init invokes set before queryCache in the same goroutine when Redis is configured.
		// bindQueryCache relies on that order so it can reuse the initialized cache adapter.
		e.Cache.Init(func(c storage.AdapterCache) {
			cacheAdapter = c
			center.SetCache(c)
		}, func(tx *gorm.DB, duration time.Duration) {
			bindQueryCache(cacheAdapter, tx, duration)
		})
	}
	runtimeCandidate, runtimeDegraded, runtimeErr := e.prepareChallengeRuntime(ctx)
	if runtimeErr != nil {
		return runtimeErr
	}
	if runtimeDegraded {
		slog.Warn("email challenge runtime unavailable; related flows are disabled")
	}
	runtimePublished := false
	defer func() {
		if runtimePublished || runtimeCandidate == nil {
			return
		}
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), runtimeCloseLimit)
		defer cancel()
		if closeErr := runtimeCandidate.Close(closeCtx); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close uncommitted challenge runtime: %w", closeErr))
		}
	}()
	queueCandidate, queueErr := e.buildOptionalQueue(ctx, newHandle.Enforcer)
	if queueErr != nil {
		return queueErr
	}
	defer func() {
		if committed || queueCandidate == nil {
			return
		}
		if closeErr := closeQueueAdapter(ctx, queueCandidate); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close uncommitted queue: %w", closeErr))
		}
	}()
	if e.Locker != nil {
		e.Locker.Init(func(l storage.AdapterLocker) {
			center.SetLocker(l)
		})
	}
	if len(e.Clusters) > 0 {
		e.Clusters.Init()
	}
	// Publish exactly the snapshot described by this Config. If an optional
	// dependency is unavailable, the new snapshot is explicitly nil so a stale
	// pepper/TTL policy from an older Config cannot survive a successful reload.
	managedCandidate, _ := queueCandidate.(storage.ManagedAdapterQueue)
	// Candidate initialization temporarily needs the legacy database globals,
	// but a retiring queue still belongs to the previous snapshot. Restore that
	// snapshot while it drains so a legacy callback cannot write through the
	// uncommitted replacement handle.
	gormdb.InstallDefault(previousDefault)
	queueRetireErr, queueRetireComplete := e.retireManagedQueueForReplacement(ctx, managedCandidate)
	if !queueRetireComplete {
		return fmt.Errorf("retire previous managed queue: %w", queueRetireErr)
	}
	if err := e.replaceChallengeRuntime(ctx, runtimeCandidate); err != nil {
		return err
	}
	runtimePublished = true
	gormdb.InstallDefault(newHandle)
	e.setManagedQueue(managedCandidate)
	center.SetQueue(queueCandidate)

	oldHandle, oldLeases := e.swapDatabaseHandle(newHandle)
	committed = true
	var commitErr error
	if queueRetireErr != nil {
		commitErr = fmt.Errorf("retire previous managed queue: %w", queueRetireErr)
	}
	if oldHandle != nil && oldHandle != newHandle {
		if closeErr := e.retireDatabaseHandle(oldHandle, oldLeases); closeErr != nil {
			commitErr = errors.Join(commitErr, fmt.Errorf("close previous application database: %w", closeErr))
		}
	}
	return commitErr
}

func (e *Config) buildOptionalQueue(
	ctx context.Context,
	enforcer casbin.IEnforcer,
) (storage.AdapterQueue, error) {
	initializer := e.queueInit
	if initializer == nil {
		if e.Queue == nil {
			return nil, nil
		}
		initializer = e.Queue.InitContext
	}

	var candidate storage.AdapterQueue
	err := initializer(ctx, func(adapter storage.AdapterQueue) error {
		candidate = adapter
		return bindPolicyWatcher(ctx, adapter, enforcer)
	})
	if err == nil {
		return candidate, nil
	}

	var closeErr error
	if candidate != nil {
		closeErr = closeQueueAdapter(ctx, candidate)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, errors.Join(
			fmt.Errorf("initialize queue with canceled context: %w", ctxErr),
			err,
			closeErr,
		)
	}
	if (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) &&
		!errors.Is(err, storage.ErrDependencyUnavailable) {
		return nil, errors.Join(fmt.Errorf("initialize queue: %w", err), closeErr)
	}
	if candidate != nil {
		return nil, errors.Join(
			fmt.Errorf("initialize queue policy watcher: %w", err),
			closeErr,
		)
	}
	if closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("initialize queue: %w", err),
			fmt.Errorf("close failed queue candidate: %w", closeErr),
		)
	}
	if errors.Is(err, storage.ErrInvalidConfiguration) {
		return nil, fmt.Errorf("initialize queue: %w", err)
	}
	if errors.Is(err, storage.ErrDependencyUnavailable) {
		// Queue-backed policy propagation is optional. Only an explicitly typed
		// broker/dependency outage may degrade; invalid profiles and installer
		// failures fail closed and roll back the candidate database.
		slog.Warn("queue dependency unavailable; distributed policy propagation is disabled", "err", err)
		return nil, nil
	}
	return nil, fmt.Errorf("initialize queue: %w", err)
}

func closeQueueAdapter(parent context.Context, adapter storage.AdapterQueue) error {
	if adapter == nil {
		return nil
	}
	if parent == nil {
		return errors.New("queue cleanup context is required")
	}
	if managed, ok := adapter.(storage.ManagedAdapterQueue); ok {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
		defer cancel()
		return managed.Close(closeCtx)
	}
	adapter.Shutdown()
	return nil
}

// DatabaseHandle returns the database Handle owned by this Config.
func (e *Config) DatabaseHandle() *gormdb.Handle {
	e.databaseMu.RLock()
	defer e.databaseMu.RUnlock()
	return e.databaseHandle
}

// ManagedQueue returns the managed queue lifecycle owned by this Config. The
// returned adapter is registered as a server Runnable; callers must not close
// it directly.
func (e *Config) ManagedQueue() storage.ManagedAdapterQueue {
	e.queueMu.RLock()
	defer e.queueMu.RUnlock()
	return e.managedQueue
}

func (e *Config) setManagedQueue(queue storage.ManagedAdapterQueue) {
	e.queueMu.Lock()
	e.managedQueue = queue
	e.queueMu.Unlock()
}

func (e *Config) hasManagedQueueOwner() bool {
	e.queueMu.RLock()
	defer e.queueMu.RUnlock()
	return e.managedQueue != nil || e.retiringQueue != nil
}

func (e *Config) retireManagedQueueForReplacement(
	ctx context.Context,
	next storage.ManagedAdapterQueue,
) (error, bool) {
	e.queueMu.Lock()
	active := e.managedQueue
	if active != nil && active != next {
		if e.retiringQueue != nil && e.retiringQueue != active {
			e.queueMu.Unlock()
			return errors.New("a different managed queue is already retiring"), false
		}
		e.managedQueue = nil
		e.retiringQueue = active
	}
	retiring := e.retiringQueue
	e.queueMu.Unlock()

	if retiring != nil && center.GetQueue() == retiring {
		center.SetQueue(nil)
	}
	if retiring == nil {
		return nil, true
	}
	err := retiring.Close(ctx)
	if !managedQueueCloseCompleted(retiring, err) {
		return err, false
	}
	e.finishRetiringQueue(retiring)
	return err, true
}

func (e *Config) finishRetiringQueue(queue storage.ManagedAdapterQueue) {
	e.queueMu.Lock()
	if e.retiringQueue == queue {
		e.retiringQueue = nil
	}
	e.queueMu.Unlock()
}

func managedQueueCloseCompleted(queue storage.ManagedAdapterQueue, err error) bool {
	complete := err == nil || (!errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded))
	if !complete {
		if state, ok := queue.(storage.ManagedAdapterQueueCloseState); ok {
			complete = state.CloseComplete()
		}
	}
	return complete
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

// Close releases all runtime resources owned by this Config with a bounded
// object-storage drain. It only clears the legacy database globals when the
// same Handle is still installed.
func (e *Config) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.CloseContext(ctx)
}

// CloseContext withdraws and closes the Runtime v2 challenge graph before the
// queue, object storage, and database. A resource that cannot close within the
// caller's deadline keeps later dependencies alive for a safe retry.
func (e *Config) CloseContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("configuration close context is required")
	}
	e.databaseReload.Lock()
	defer e.databaseReload.Unlock()

	runtimeErr := e.closeChallengeRuntime(ctx)
	if e.hasChallengeRuntimeOwner() {
		return runtimeErr
	}
	queueErr := e.closeManagedQueue(ctx)
	if e.hasManagedQueueOwner() {
		return errors.Join(runtimeErr, queueErr)
	}
	storageErr := e.closeObjectStorage(ctx)
	handle, leases := e.swapDatabaseHandle(nil)
	if handle != nil {
		gormdb.ClearDefault(handle)
	}
	if handle == nil {
		return errors.Join(runtimeErr, queueErr, storageErr)
	}
	return errors.Join(runtimeErr, queueErr, storageErr, e.retireDatabaseHandle(handle, leases))
}

func (e *Config) closeManagedQueue(ctx context.Context) error {
	e.queueMu.Lock()
	if e.retiringQueue == nil && e.managedQueue != nil {
		e.retiringQueue = e.managedQueue
		e.managedQueue = nil
	}
	queue := e.retiringQueue
	e.queueMu.Unlock()
	if queue == nil {
		return nil
	}
	if center.GetQueue() == queue {
		center.SetQueue(nil)
	}
	err := queue.Close(ctx)
	if !managedQueueCloseCompleted(queue, err) {
		return fmt.Errorf("close managed queue: %w", err)
	}
	e.finishRetiringQueue(queue)
	if err != nil {
		return fmt.Errorf("close managed queue: %w", err)
	}
	return nil
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

func bindPolicyWatcher(ctx context.Context, adapter storage.AdapterQueue, enforcer casbin.IEnforcer) error {
	if adapter == nil || enforcer == nil {
		return nil
	}
	watcher := queue.NewSampleWatcher(adapter)
	if err := watcher.SetUpdateCallbackContext(ctx, func(string) {
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
		if err := bindPolicyWatcher(ctx, adapter, newHandle.Enforcer); err != nil {
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
