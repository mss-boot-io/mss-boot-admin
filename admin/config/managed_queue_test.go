package config

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
)

type configManagedQueue struct {
	mu              sync.Mutex
	registerContext context.Context
	registerOptions *storage.Options
	registerErr     error
	registerCalls   int
	closeCalls      int
	closeFunc       func(context.Context, int) error
	onClose         func()
	startErr        error
	errors          chan error
}

type configManagedQueueCloseState struct {
	*configManagedQueue
	complete bool
}

func (q *configManagedQueueCloseState) CloseComplete() bool { return q.complete }

func (q *configManagedQueue) String() string { return "config-managed-queue" }

func (q *configManagedQueue) Append(...storage.Option) error { return nil }

func (q *configManagedQueue) Register(opts ...storage.Option) {
	q.mu.Lock()
	q.registerCalls++
	q.registerOptions = storage.SetOptions(opts...)
	q.mu.Unlock()
}

func (q *configManagedQueue) RegisterContext(ctx context.Context, opts ...storage.Option) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.registerCalls++
	q.registerContext = ctx
	q.registerOptions = storage.SetOptions(opts...)
	return q.registerErr
}

func (q *configManagedQueue) Run(context.Context) {}

func (q *configManagedQueue) Shutdown() {}

func (q *configManagedQueue) Start(context.Context) error { return q.startErr }

func (q *configManagedQueue) Errors() <-chan error { return q.errors }

func (q *configManagedQueue) Close(ctx context.Context) error {
	q.mu.Lock()
	q.closeCalls++
	call := q.closeCalls
	closeFunc := q.closeFunc
	onClose := q.onClose
	q.mu.Unlock()
	if onClose != nil {
		onClose()
	}
	if closeFunc != nil {
		return closeFunc(ctx, call)
	}
	return nil
}

func TestConfigInitialQueueOutageCommitsWithoutQueue(t *testing.T) {
	isolateManagedQueueGlobals(t)
	queueUnavailable := errors.New("broker unavailable")
	initCalls := 0
	configuration := &Config{
		queueInit: func(context.Context, func(storage.AdapterQueue) error) error {
			initCalls++
			return &storage.DependencyUnavailableError{Adapter: "Kafka", Err: queueUnavailable}
		},
	}

	if err := configuration.InitContext(context.Background(), managedQueueConfigSource("queue-outage")); err != nil {
		t.Fatalf("InitContext with unavailable optional queue: %v", err)
	}
	t.Cleanup(func() { _ = configuration.Close() })
	if initCalls != 1 {
		t.Fatalf("queue initialization calls = %d, want 1", initCalls)
	}
	if center.GetQueue() != nil || configuration.ManagedQueue() != nil {
		t.Fatal("unavailable queue published a stale or partially initialized adapter")
	}
	handle := configuration.DatabaseHandle()
	if handle == nil || handle.SQLDB() == nil {
		t.Fatal("application database was not committed after optional queue outage")
	}
	if err := handle.SQLDB().Ping(); err != nil {
		t.Fatalf("application database after queue outage: %v", err)
	}
}

func TestConfigQueueConfigurationAndWatcherFailuresRollBackDatabase(t *testing.T) {
	tests := []struct {
		name string
		init func(func(storage.AdapterQueue) error) error
		want error
	}{
		{
			name: "invalid configuration",
			want: storage.ErrInvalidConfiguration,
			init: func(func(storage.AdapterQueue) error) error {
				return &storage.InvalidConfigurationError{Adapter: "Kafka", Err: errors.New("unsupported provider")}
			},
		},
		{
			name: "unclassified initialization failure",
			want: errors.New("unexpected initialization failure"),
		},
		{
			name: "invalid classification wins over outage",
			want: storage.ErrInvalidConfiguration,
			init: func(func(storage.AdapterQueue) error) error {
				return errors.Join(
					&storage.InvalidConfigurationError{Adapter: "Kafka", Err: errors.New("manual commit")},
					&storage.DependencyUnavailableError{Adapter: "Kafka", Err: errors.New("broker unavailable")},
				)
			},
		},
		{
			name: "watcher dependency failure",
			want: storage.ErrDependencyUnavailable,
			init: func(install func(storage.AdapterQueue) error) error {
				queue := &configManagedQueue{registerErr: &storage.DependencyUnavailableError{
					Adapter: "Kafka",
					Err:     errors.New("consumer broker unavailable"),
				}}
				return install(queue)
			},
		},
	}
	tests[1].init = func(func(storage.AdapterQueue) error) error { return tests[1].want }

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateManagedQueueGlobals(t)
			configuration := &Config{
				queueInit: func(_ context.Context, install func(storage.AdapterQueue) error) error {
					return test.init(install)
				},
			}

			err := configuration.InitContext(context.Background(), managedQueueConfigSource("queue-fail-"+test.name))
			if !errors.Is(err, test.want) {
				t.Fatalf("InitContext() error = %v, want wrapped %v", err, test.want)
			}
			if configuration.DatabaseHandle() != nil || gormdb.DefaultHandle() != nil {
				t.Fatal("invalid or watcher queue failure committed a database snapshot")
			}
			if center.GetQueue() != nil || configuration.ManagedQueue() != nil {
				t.Fatal("invalid or watcher queue failure published an adapter")
			}
		})
	}
}

func TestConfigQueueCancellationPropagatesAndRollsBackDatabase(t *testing.T) {
	isolateManagedQueueGlobals(t)
	configuration := &Config{
		queueInit: func(context.Context, func(storage.AdapterQueue) error) error {
			return context.Canceled
		},
	}

	err := configuration.InitContext(context.Background(), managedQueueConfigSource("queue-canceled"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("InitContext error = %v, want context canceled", err)
	}
	if configuration.DatabaseHandle() != nil || gormdb.DefaultHandle() != nil {
		t.Fatal("canceled queue initialization committed a database snapshot")
	}
	if center.GetQueue() != nil {
		t.Fatal("canceled initial queue initialization published an adapter")
	}
}

func TestConfigOwnsAndClosesManagedQueueBeforeDatabase(t *testing.T) {
	isolateManagedQueueGlobals(t)
	queue := &configManagedQueue{}
	configuration := &Config{
		queueInit: func(_ context.Context, install func(storage.AdapterQueue) error) error {
			return install(queue)
		},
	}
	if err := configuration.InitContext(context.Background(), managedQueueConfigSource("queue-owned")); err != nil {
		t.Fatalf("InitContext: %v", err)
	}
	handle := configuration.DatabaseHandle()
	queue.onClose = func() {
		if current := configuration.DatabaseHandle(); current != handle {
			t.Errorf("database handle during queue close = %p, want %p", current, handle)
			return
		}
		if err := handle.SQLDB().Ping(); err != nil {
			t.Errorf("database closed before managed queue: %v", err)
		}
	}

	if configuration.ManagedQueue() != queue || center.GetQueue() != queue {
		t.Fatal("managed queue ownership was not published by the committed snapshot")
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := configuration.CloseContext(closeCtx); err != nil {
		t.Fatalf("CloseContext: %v", err)
	}
	if queue.closeCalls != 1 {
		t.Fatalf("managed queue close calls = %d, want 1", queue.closeCalls)
	}
	if configuration.ManagedQueue() != nil || center.GetQueue() != nil {
		t.Fatal("closed managed queue remains published")
	}
	if configuration.DatabaseHandle() != nil {
		t.Fatal("database handle remains owned after close")
	}
	if err := handle.SQLDB().Ping(); err == nil {
		t.Fatal("database pool remains open after queue and configuration close")
	}
}

func TestConfigCloseRetainsDatabaseUntilManagedQueueCanStop(t *testing.T) {
	isolateManagedQueueGlobals(t)
	queue := &configManagedQueue{
		closeFunc: func(_ context.Context, call int) error {
			if call == 1 {
				return context.Canceled
			}
			return nil
		},
	}
	configuration := &Config{
		queueInit: func(_ context.Context, install func(storage.AdapterQueue) error) error {
			return install(queue)
		},
	}
	if err := configuration.InitContext(context.Background(), managedQueueConfigSource("queue-close-retry")); err != nil {
		t.Fatalf("InitContext: %v", err)
	}
	handle := configuration.DatabaseHandle()

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	cancelFirst()
	if err := configuration.CloseContext(firstCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("first CloseContext error = %v, want context canceled", err)
	}
	if configuration.ManagedQueue() != nil || center.GetQueue() != nil {
		t.Fatal("closing managed queue remains available through center")
	}
	configuration.queueMu.RLock()
	retiring := configuration.retiringQueue
	configuration.queueMu.RUnlock()
	if retiring != queue {
		t.Fatal("failed managed queue close discarded the private retirement owner needed for retry")
	}
	if configuration.DatabaseHandle() != handle {
		t.Fatal("database was retired while the managed consumer might still be running")
	}
	if err := handle.SQLDB().Ping(); err != nil {
		t.Fatalf("database is unavailable before queue close retry: %v", err)
	}

	if err := configuration.CloseContext(context.Background()); err != nil {
		t.Fatalf("second CloseContext: %v", err)
	}
	if queue.closeCalls != 2 {
		t.Fatalf("managed queue close calls = %d, want 2", queue.closeCalls)
	}
	if configuration.ManagedQueue() != nil || configuration.DatabaseHandle() != nil {
		t.Fatal("successful close retry retained managed resources")
	}
}

func TestConfigCloseReleasesOwnerAndDatabaseAfterTerminalQueueDiagnostic(t *testing.T) {
	isolateManagedQueueGlobals(t)
	closeDiagnostic := errors.New("Kafka producer close failed")
	queue := &configManagedQueueCloseState{
		configManagedQueue: &configManagedQueue{
			closeFunc: func(context.Context, int) error {
				return errors.Join(context.Canceled, closeDiagnostic)
			},
		},
		complete: true,
	}
	configuration := &Config{
		queueInit: func(_ context.Context, install func(storage.AdapterQueue) error) error {
			return install(queue)
		},
	}
	if err := configuration.InitContext(context.Background(), managedQueueConfigSource("queue-close-diagnostic")); err != nil {
		t.Fatalf("InitContext: %v", err)
	}
	handle := configuration.DatabaseHandle()

	err := configuration.CloseContext(context.Background())
	if !errors.Is(err, closeDiagnostic) {
		t.Fatalf("CloseContext() error = %v, want terminal close diagnostic", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CloseContext() error = %v, want preserved terminal context diagnostic", err)
	}
	if configuration.ManagedQueue() != nil || center.GetQueue() != nil {
		t.Fatal("terminally closed queue remains owned or published")
	}
	if configuration.DatabaseHandle() != nil {
		t.Fatal("terminal queue diagnostic retained the database owner")
	}
	if pingErr := handle.SQLDB().Ping(); pingErr == nil {
		t.Fatal("database remains open after terminal queue close diagnostic")
	}
}

func TestConfigReplacementRetiresOldManagedQueueBeforeOldDatabase(t *testing.T) {
	isolateManagedQueueGlobals(t)
	oldQueue := &configManagedQueue{}
	newQueue := &configManagedQueue{}
	queues := []storage.AdapterQueue{oldQueue, newQueue}
	configuration := &Config{}
	configuration.queueInit = func(_ context.Context, install func(storage.AdapterQueue) error) error {
		candidate := queues[0]
		queues = queues[1:]
		return install(candidate)
	}
	if err := configuration.InitContext(context.Background(), managedQueueConfigSource("queue-replace-old")); err != nil {
		t.Fatalf("initial InitContext: %v", err)
	}
	t.Cleanup(func() { _ = configuration.Close() })
	oldHandle := configuration.DatabaseHandle()
	oldQueue.onClose = func() {
		if configuration.DatabaseHandle() != oldHandle {
			t.Errorf("old database was unpublished before its managed queue stopped")
			return
		}
		if gormdb.DefaultHandle() != oldHandle {
			t.Errorf("legacy database default changed before its managed queue stopped")
			return
		}
		if err := oldHandle.SQLDB().Ping(); err != nil {
			t.Errorf("old database closed before its managed queue: %v", err)
		}
	}

	if err := configuration.InitContext(context.Background(), managedQueueConfigSource("queue-replace-new")); err != nil {
		t.Fatalf("replacement InitContext: %v", err)
	}
	if oldQueue.closeCalls != 1 {
		t.Fatalf("old managed queue close calls = %d, want 1", oldQueue.closeCalls)
	}
	if configuration.ManagedQueue() != newQueue || center.GetQueue() != newQueue {
		t.Fatal("replacement managed queue was not published")
	}
	if configuration.DatabaseHandle() == oldHandle {
		t.Fatal("replacement database was not published")
	}
	if err := oldHandle.SQLDB().Ping(); err == nil {
		t.Fatal("old database remains open after its managed queue retired")
	}
}

func TestConfigReplacementCommitsNewOwnerAfterTerminalQueueDiagnostic(t *testing.T) {
	isolateManagedQueueGlobals(t)
	closeDiagnostic := errors.New("old Kafka producer close failed")
	oldQueue := &configManagedQueueCloseState{
		configManagedQueue: &configManagedQueue{},
		complete:           true,
	}
	newQueue := &configManagedQueue{}
	queues := []storage.AdapterQueue{oldQueue, newQueue}
	configuration := &Config{
		queueInit: func(_ context.Context, install func(storage.AdapterQueue) error) error {
			candidate := queues[0]
			queues = queues[1:]
			return install(candidate)
		},
	}
	if err := configuration.InitContext(context.Background(), managedQueueConfigSource("queue-terminal-old")); err != nil {
		t.Fatalf("initial InitContext: %v", err)
	}
	t.Cleanup(func() { _ = configuration.Close() })
	oldHandle := configuration.DatabaseHandle()
	oldQueue.closeFunc = func(context.Context, int) error {
		return errors.Join(context.Canceled, closeDiagnostic)
	}
	oldQueue.onClose = func() {
		if configuration.DatabaseHandle() != oldHandle || gormdb.DefaultHandle() != oldHandle {
			t.Errorf("old database snapshot was not authoritative while the old queue retired")
		}
	}

	err := configuration.InitContext(context.Background(), managedQueueConfigSource("queue-terminal-new"))
	if !errors.Is(err, closeDiagnostic) || !errors.Is(err, context.Canceled) {
		t.Fatalf("replacement InitContext error = %v, want terminal close diagnostics", err)
	}
	newHandle := configuration.DatabaseHandle()
	if newHandle == nil || newHandle == oldHandle || gormdb.DefaultHandle() != newHandle {
		t.Fatal("terminally closed old queue did not commit the replacement database snapshot")
	}
	if err := newHandle.SQLDB().Ping(); err != nil {
		t.Fatalf("replacement database is unavailable after terminal queue diagnostic: %v", err)
	}
	if configuration.ManagedQueue() != newQueue || center.GetQueue() != newQueue {
		t.Fatal("terminally closed old queue did not commit the replacement queue owner")
	}
	configuration.queueMu.RLock()
	retiring := configuration.retiringQueue
	configuration.queueMu.RUnlock()
	if retiring != nil {
		t.Fatal("terminally closed old queue remains tracked as retiring")
	}
	if oldQueue.closeCalls != 1 || newQueue.closeCalls != 0 {
		t.Fatalf("queue Close calls = old:%d new:%d, want 1/0", oldQueue.closeCalls, newQueue.closeCalls)
	}
	if err := oldHandle.SQLDB().Ping(); err == nil {
		t.Fatal("old database remains open after terminal queue retirement committed replacement")
	}
}

func TestConfigReplacementTimeoutDegradesQueueAndRetainsOldDatabaseForRetry(t *testing.T) {
	isolateManagedQueueGlobals(t)
	oldQueue := &configManagedQueue{}
	newQueue := &configManagedQueue{}
	queues := []storage.AdapterQueue{oldQueue, newQueue}
	configuration := &Config{
		queueInit: func(_ context.Context, install func(storage.AdapterQueue) error) error {
			candidate := queues[0]
			queues = queues[1:]
			return install(candidate)
		},
	}
	if err := configuration.InitContext(context.Background(), managedQueueConfigSource("queue-timeout-old")); err != nil {
		t.Fatalf("initial InitContext: %v", err)
	}
	oldHandle := configuration.DatabaseHandle()
	oldQueue.closeFunc = func(_ context.Context, call int) error {
		if call == 1 {
			return context.DeadlineExceeded
		}
		return nil
	}
	oldQueue.onClose = func() {
		if configuration.DatabaseHandle() != oldHandle || gormdb.DefaultHandle() != oldHandle {
			t.Errorf("old database snapshot was not authoritative while the old queue retired")
		}
	}

	err := configuration.InitContext(context.Background(), managedQueueConfigSource("queue-timeout-new"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("replacement InitContext error = %v, want close deadline", err)
	}
	if configuration.ManagedQueue() != nil || center.GetQueue() != nil {
		t.Fatal("closing old queue remained published after replacement timeout")
	}
	configuration.queueMu.RLock()
	retiring := configuration.retiringQueue
	configuration.queueMu.RUnlock()
	if retiring != oldQueue {
		t.Fatal("timed-out old queue is not retained as the private retirement owner")
	}
	if configuration.DatabaseHandle() != oldHandle || gormdb.DefaultHandle() != oldHandle {
		t.Fatal("replacement timeout did not retain the old database snapshot")
	}
	if err := oldHandle.SQLDB().Ping(); err != nil {
		t.Fatalf("old database is unavailable before queue retirement retry: %v", err)
	}
	if oldQueue.closeCalls != 1 || newQueue.closeCalls != 1 {
		t.Fatalf("queue Close calls after timeout = old:%d new:%d, want 1/1", oldQueue.closeCalls, newQueue.closeCalls)
	}

	if err := configuration.CloseContext(context.Background()); err != nil {
		t.Fatalf("CloseContext retirement retry: %v", err)
	}
	if oldQueue.closeCalls != 2 {
		t.Fatalf("old queue Close calls after retry = %d, want 2", oldQueue.closeCalls)
	}
	if configuration.hasManagedQueueOwner() || configuration.DatabaseHandle() != nil || gormdb.DefaultHandle() != nil {
		t.Fatal("successful retirement retry retained queue or database ownership")
	}
	if err := oldHandle.SQLDB().Ping(); err == nil {
		t.Fatal("old database remains open after retirement retry")
	}
}

func TestReloadDatabaseDuplicateConsumerKeepsPreviousHandle(t *testing.T) {
	isolateManagedQueueGlobals(t)
	oldHandle := openConfigSQLiteHandle(t, "queue-reload-old")
	duplicateErr := errors.New("duplicate topic and group registration")
	queue := &configManagedQueue{registerErr: duplicateErr}
	center.SetQueue(queue)
	configuration := &Config{
		Database: gormdb.Database{
			Driver:      "sqlite",
			Source:      "file:queue-reload-new?mode=memory&cache=shared",
			CasbinModel: managedQueueCasbinModel,
		},
		databaseHandle: oldHandle,
	}
	gormdb.InstallDefault(oldHandle)
	t.Cleanup(func() {
		center.SetQueue(nil)
		_ = configuration.Close()
	})

	err := configuration.reloadDatabase(context.Background())
	if !errors.Is(err, duplicateErr) {
		t.Fatalf("reloadDatabase error = %v, want duplicate registration", err)
	}
	if configuration.DatabaseHandle() != oldHandle || gormdb.DefaultHandle() != oldHandle {
		t.Fatal("duplicate consumer registration replaced the working database handle")
	}
	if err := oldHandle.SQLDB().Ping(); err != nil {
		t.Fatalf("previous database was closed after duplicate registration: %v", err)
	}
	queue.mu.Lock()
	registerCalls := queue.registerCalls
	queue.mu.Unlock()
	if registerCalls != 1 {
		t.Fatalf("managed registration calls = %d, want 1", registerCalls)
	}
}

func TestBindPolicyWatcherRegistersManagedConsumerExactlyOnce(t *testing.T) {
	handle, err := (&gormdb.Database{
		Driver:      "sqlite",
		Source:      "file:queue-bind-watcher?mode=memory&cache=shared",
		CasbinModel: managedQueueCasbinModel,
	}).Open(context.Background())
	if err != nil {
		t.Fatalf("open database with enforcer: %v", err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	queue := &configManagedQueue{}

	if err := bindPolicyWatcher(context.Background(), queue, handle.Enforcer); err != nil {
		t.Fatalf("bindPolicyWatcher: %v", err)
	}
	queue.mu.Lock()
	registerCalls := queue.registerCalls
	queue.mu.Unlock()
	if registerCalls != 1 {
		t.Fatalf("managed registration calls = %d, want exactly 1", registerCalls)
	}
}

func managedQueueConfigSource(name string) source.Option {
	data := fmt.Sprintf(`application:
  mode: test
database:
  driver: sqlite
  source: "file:%s?mode=memory&cache=shared"
  casbinModel: %q
`, name, managedQueueCasbinModel)
	return func(options *source.Options) {
		source.WithProvider(source.FS)(options)
		source.WithFrom(fstest.MapFS{
			"application.yml": &fstest.MapFile{Data: []byte(data)},
		})(options)
	}
}

func isolateManagedQueueGlobals(t *testing.T) {
	t.Helper()
	previousCenter := center.Default
	center.Default = &center.DefaultCenter{}
	gormdb.ClearDefault(nil)
	t.Cleanup(func() {
		gormdb.ClearDefault(nil)
		center.Default = previousCenter
	})
	t.Setenv("STAGE", "test")
}

const managedQueueCasbinModel = `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
`

var _ storage.ManagedAdapterQueue = (*configManagedQueue)(nil)
var _ storage.ManagedAdapterQueueCloseState = (*configManagedQueueCloseState)(nil)
