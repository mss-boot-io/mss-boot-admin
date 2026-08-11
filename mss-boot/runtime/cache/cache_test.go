package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/redisresource"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDatabaseAuthoritativeFallbackAndCommittedWriteOutcome(t *testing.T) {
	fixture := newCacheFixture(t)
	query := mustQueryCache(t, fixture.scope("queries"), testPolicy("records"))
	database := newTestDatabase(t)
	seedRecord(t, database, "before")
	target := testTarget("record-1")

	first, err := query.Load(context.Background(), database, target, recordLoader(1))
	if err != nil {
		t.Fatalf("prime query cache: %v", err)
	}
	if first.Status != StatusStored || decodeRecord(t, first).Value != "before" {
		t.Fatalf("prime outcome = %#v", first)
	}
	write := database.Model(&testRecord{}).Where("id = ?", 1).Update("value", "committed")
	if write.Error != nil || write.RowsAffected != 1 {
		t.Fatalf("commit authoritative write: rows=%d err=%v", write.RowsAffected, write.Error)
	}
	if err := fixture.resource.Close(context.Background()); err != nil {
		t.Fatalf("close provider: %v", err)
	}

	status, err := query.Invalidate(context.Background(), target.dataset())
	if err != nil || status != StatusProviderBypass {
		t.Fatalf("invalidation during outage = %q, %v", status, err)
	}
	outcome, err := query.Load(context.Background(), database, target, recordLoader(1))
	if err != nil {
		t.Fatalf("authoritative fallback: %v", err)
	}
	if outcome.Source != SourceAuthority || outcome.Status != StatusProviderBypass || decodeRecord(t, outcome).Value != "committed" {
		t.Fatalf("fallback outcome = %#v", outcome)
	}
	var committed testRecord
	if err := database.First(&committed, 1).Error; err != nil || committed.Value != "committed" {
		t.Fatalf("committed database result changed: %#v, %v", committed, err)
	}
}

func TestMissSingleflightAcrossConcurrentReaders(t *testing.T) {
	fixture := newCacheFixture(t)
	derived := mustDerived(t, fixture.scope("queries"), testPolicy("singleflight"))
	target := testTarget("same-query")
	const readers = 24
	start := make(chan struct{})
	release := make(chan struct{})
	loaderStarted := make(chan struct{})
	var startOnce sync.Once
	var loaderCalls atomic.Int32
	loader := func(ctx context.Context) (Result, error) {
		loaderCalls.Add(1)
		startOnce.Do(func() { close(loaderStarted) })
		select {
		case <-release:
			return Result{Payload: []byte("shared"), RowsAffected: 1}, nil
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}

	results := make(chan struct {
		outcome Outcome
		err     error
	}, readers)
	for range readers {
		go func() {
			<-start
			outcome, err := derived.Load(context.Background(), target, loader)
			results <- struct {
				outcome Outcome
				err     error
			}{outcome: outcome, err: err}
		}()
	}
	close(start)
	<-loaderStarted
	waitForFlightWaiters(t, derived, readers)
	close(release)
	for range readers {
		result := <-results
		if result.err != nil || result.outcome.Status != StatusStored || string(result.outcome.Result.Payload) != "shared" {
			t.Fatalf("singleflight reader = %#v, %v", result.outcome, result.err)
		}
	}
	if calls := loaderCalls.Load(); calls != 1 {
		t.Fatalf("loader calls = %d, want 1", calls)
	}
	hit, err := derived.Load(context.Background(), target, func(context.Context) (Result, error) {
		return Result{}, errors.New("cache hit called loader")
	})
	if err != nil || hit.Status != StatusHit || string(hit.Result.Payload) != "shared" {
		t.Fatalf("post-flight hit = %#v, %v", hit, err)
	}
}

func TestGenerationInvalidationWinsLoaderRace(t *testing.T) {
	fixture := newCacheFixture(t)
	derived := mustDerived(t, fixture.scope("queries"), testPolicy("generation"))
	target := testTarget("generation-race")
	oldStarted := make(chan struct{})
	releaseOld := make(chan struct{})
	oldDone := make(chan error, 1)
	go func() {
		_, err := derived.Load(context.Background(), target, func(ctx context.Context) (Result, error) {
			close(oldStarted)
			select {
			case <-releaseOld:
				return Result{Payload: []byte("old"), RowsAffected: 1}, nil
			case <-ctx.Done():
				return Result{}, ctx.Err()
			}
		})
		oldDone <- err
	}()
	<-oldStarted
	if status, err := derived.Invalidate(context.Background(), target.dataset()); err != nil || status != StatusInvalidated {
		t.Fatalf("invalidate = %q, %v", status, err)
	}

	newOutcome, err := derived.Load(context.Background(), target, func(context.Context) (Result, error) {
		return Result{Payload: []byte("new"), RowsAffected: 1}, nil
	})
	if err != nil || newOutcome.Status != StatusStored || string(newOutcome.Result.Payload) != "new" {
		t.Fatalf("post-invalidation load = %#v, %v", newOutcome, err)
	}
	close(releaseOld)
	if err := <-oldDone; err != nil {
		t.Fatalf("old generation load: %v", err)
	}
	hit, err := derived.Load(context.Background(), target, func(context.Context) (Result, error) {
		return Result{}, errors.New("stale generation became visible")
	})
	if err != nil || hit.Status != StatusHit || string(hit.Result.Payload) != "new" {
		t.Fatalf("generation winner = %#v, %v", hit, err)
	}
}

func TestNamespaceScopeAndCrossInstanceDatasourceIdentity(t *testing.T) {
	fixture := newCacheFixture(t)
	queries := fixture.scope("queries")
	otherScope := fixture.scope("reports")
	first := mustDerived(t, queries, testPolicy("shared"))
	secondInstance := mustDerived(t, queries, testPolicy("shared"))
	otherNamespace := mustDerived(t, queries, testPolicy("private"))
	otherScopeCache := mustDerived(t, otherScope, testPolicy("shared"))
	target := testTarget("identity")

	stored, err := first.Load(context.Background(), target, staticLoader("first"))
	if err != nil || stored.Status != StatusStored {
		t.Fatalf("store first instance = %#v, %v", stored, err)
	}
	hit, err := secondInstance.Load(context.Background(), target, failingLoader("cross-instance miss"))
	if err != nil || hit.Status != StatusHit || string(hit.Result.Payload) != "first" {
		t.Fatalf("cross-instance identity = %#v, %v", hit, err)
	}
	assertAuthorityLoad(t, otherNamespace, target, "namespace")
	assertAuthorityLoad(t, otherScopeCache, target, "scope")
	differentDatasource := target
	differentDatasource.Datasource = "analytics"
	assertAuthorityLoad(t, secondInstance, differentDatasource, "datasource")

	for _, key := range fixture.server.Keys() {
		if containsAny(key, target.Datasource, target.Table, target.QueryIdentity) {
			t.Fatalf("provider key exposed target material: %q", key)
		}
	}
}

func TestPayloadBoundBypassesSharedCache(t *testing.T) {
	fixture := newCacheFixture(t)
	policy := testPolicy("bounded")
	policy.MaxPayloadBytes = 4
	derived := mustDerived(t, fixture.scope("queries"), policy)
	target := testTarget("oversize")
	var calls atomic.Int32
	loader := func(context.Context) (Result, error) {
		calls.Add(1)
		return Result{Payload: []byte("12345"), RowsAffected: 1}, nil
	}
	for index := range 2 {
		outcome, err := derived.Load(context.Background(), target, loader)
		if err != nil || outcome.Status != StatusPayloadBypass || string(outcome.Result.Payload) != "12345" {
			t.Fatalf("oversize load %d = %#v, %v", index, outcome, err)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("oversize loader calls = %d, want 2", calls.Load())
	}
	if keys := fixture.server.Keys(); len(keys) != 0 {
		t.Fatalf("oversize result entered shared provider: %v", keys)
	}
}

func TestNotFoundAndRowsAffectedMetadataRoundTrip(t *testing.T) {
	fixture := newCacheFixture(t)
	query := mustQueryCache(t, fixture.scope("queries"), testPolicy("metadata"))
	database := newTestDatabase(t)
	missing := testTarget("missing")
	var missingCalls atomic.Int32
	missingLoader := func(context.Context, *gorm.DB) (Result, error) {
		missingCalls.Add(1)
		return Result{}, gorm.ErrRecordNotFound
	}
	for index, wantStatus := range []Status{StatusStored, StatusHit} {
		outcome, err := query.Load(context.Background(), database, missing, missingLoader)
		if !errors.Is(err, gorm.ErrRecordNotFound) || !outcome.Result.NotFound || outcome.Result.RowsAffected != 0 || wantStatus != outcome.Status {
			t.Fatalf("missing load %d = %#v, %v", index, outcome, err)
		}
	}
	if missingCalls.Load() != 1 {
		t.Fatalf("missing loader calls = %d, want 1", missingCalls.Load())
	}

	found := testTarget("rows")
	var foundCalls atomic.Int32
	foundLoader := func(context.Context, *gorm.DB) (Result, error) {
		foundCalls.Add(1)
		return Result{Payload: []byte("rows"), RowsAffected: 7}, nil
	}
	for index, wantStatus := range []Status{StatusStored, StatusHit} {
		outcome, err := query.Load(context.Background(), database, found, foundLoader)
		if err != nil || outcome.Result.NotFound || outcome.Result.RowsAffected != 7 || string(outcome.Result.Payload) != "rows" || outcome.Status != wantStatus {
			t.Fatalf("rows load %d = %#v, %v", index, outcome, err)
		}
	}
	if foundCalls.Load() != 1 {
		t.Fatalf("rows loader calls = %d, want 1", foundCalls.Load())
	}
}

func TestActiveTransactionBypassesCacheReadYourWritesAndRollback(t *testing.T) {
	fixture := newCacheFixture(t)
	query := mustQueryCache(t, fixture.scope("queries"), testPolicy("transactions"))
	database := newTestDatabase(t)
	seedRecord(t, database, "outside")
	target := testTarget("record-1")
	prime, err := query.Load(context.Background(), database, target, recordLoader(1))
	if err != nil || prime.Status != StatusStored || decodeRecord(t, prime).Value != "outside" {
		t.Fatalf("prime = %#v, %v", prime, err)
	}

	transaction := database.Begin()
	if transaction.Error != nil {
		t.Fatalf("begin: %v", transaction.Error)
	}
	if err := transaction.Model(&testRecord{}).Where("id = ?", 1).Update("value", "inside").Error; err != nil {
		t.Fatalf("transaction write: %v", err)
	}
	inside, err := query.Load(context.Background(), transaction, target, recordLoader(1))
	if err != nil || inside.Status != StatusTransactionBypass || inside.Source != SourceAuthority || decodeRecord(t, inside).Value != "inside" {
		t.Fatalf("read-your-writes outcome = %#v, %v", inside, err)
	}
	if err := transaction.Rollback().Error; err != nil {
		t.Fatalf("rollback: %v", err)
	}

	after, err := query.Load(context.Background(), database, target, failingQueryLoader("rollback polluted shared cache"))
	if err != nil || after.Status != StatusHit || decodeRecord(t, after).Value != "outside" {
		t.Fatalf("post-rollback shared result = %#v, %v", after, err)
	}
	var authority testRecord
	if err := database.First(&authority, 1).Error; err != nil || authority.Value != "outside" {
		t.Fatalf("rollback authority = %#v, %v", authority, err)
	}
}

func TestCloseAndContextBoundFlightsWithoutClosingScope(t *testing.T) {
	fixture := newCacheFixture(t)
	derived := mustDerived(t, fixture.scope("queries"), testPolicy("lifecycle"))
	target := testTarget("context")
	started := make(chan struct{})
	canceled := make(chan struct{})
	requestContext, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := derived.Load(requestContext, target, func(ctx context.Context) (Result, error) {
		close(started)
		<-ctx.Done()
		close(canceled)
		return Result{}, ctx.Err()
	})
	<-started
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline load = %v", err)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("last waiter cancellation did not cancel loader")
	}

	closeStarted := make(chan struct{})
	loadDone := make(chan error, 1)
	go func() {
		_, loadErr := derived.Load(context.Background(), testTarget("close"), func(ctx context.Context) (Result, error) {
			close(closeStarted)
			<-ctx.Done()
			return Result{}, ctx.Err()
		})
		loadDone <- loadErr
	}()
	<-closeStarted
	closeContext, closeCancel := context.WithTimeout(context.Background(), time.Second)
	defer closeCancel()
	if err := derived.Close(closeContext); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := <-loadDone; !errors.Is(err, ErrClosed) {
		t.Fatalf("in-flight close result = %v", err)
	}
	if _, err := derived.Load(context.Background(), target, staticLoader("late")); !errors.Is(err, ErrClosed) {
		t.Fatalf("load after close = %v", err)
	}
	if err := fixture.scope("queries").Use(context.Background(), func(redisresource.Lease) error { return nil }); err != nil {
		t.Fatalf("cache close closed shared scope: %v", err)
	}
}

type cacheFixture struct {
	server   *miniredis.Miniredis
	resource *redisresource.Resource
}

func newCacheFixture(t *testing.T) *cacheFixture {
	t.Helper()
	server := miniredis.RunT(t)
	redisConfig := &runtimeconfig.RedisConfig{
		Mode:       runtimeconfig.RedisStandalone,
		Standalone: &runtimeconfig.RedisStandaloneConfig{Endpoint: server.Addr()},
		Credentials: runtimeconfig.RedisCredentialsConfig{
			Kind:      runtimeconfig.RedisCredentialsAnonymous,
			Anonymous: &runtimeconfig.RedisAnonymousCredentialsConfig{},
		},
	}
	configuration := runtimeconfig.Config{Resources: map[string]runtimeconfig.ResourceConfig{
		"main-cache": {Provider: runtimeconfig.ProviderConfig{Kind: runtimeconfig.ProviderRedis, Redis: redisConfig}},
	}}
	snapshot, err := configuration.Normalize(context.Background(), runtimeconfig.SecretResolverFunc(func(context.Context, runtimeconfig.SecretRef) (string, error) {
		return "", errors.New("unexpected secret resolution")
	}))
	if err != nil {
		t.Fatalf("normalize Redis profile: %v", err)
	}
	profile, ok := snapshot.Resource("main-cache")
	if !ok {
		t.Fatal("normalized Redis profile missing")
	}
	resource, err := redisresource.Build(profile)
	if err != nil {
		t.Fatalf("build Redis resource: %v", err)
	}
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("start Redis resource: %v", err)
	}
	t.Cleanup(func() { _ = resource.Close(context.Background()) })
	return &cacheFixture{server: server, resource: resource}
}

func (f *cacheFixture) scope(name string) *redisresource.Scope {
	scope, err := f.resource.Scope(name)
	if err != nil {
		panic(fmt.Sprintf("scope %q: %v", name, err))
	}
	return scope
}

func testPolicy(namespace string) Policy {
	return Policy{
		Authority:       AuthorityDatabase,
		Namespace:       namespace,
		TTL:             time.Minute,
		MaxPayloadBytes: 1024,
		FailureMode:     FailureModeBypassAuthority,
		Reconstruction:  ReconstructionLoader,
	}
}

func testTarget(identity string) Target {
	return Target{Datasource: "primary", Table: "test_records", QueryIdentity: identity}
}

func mustDerived(t *testing.T, scope *redisresource.Scope, policy Policy) *Derived {
	t.Helper()
	derived, err := NewDerived(scope, policy)
	if err != nil {
		t.Fatalf("NewDerived: %v", err)
	}
	t.Cleanup(func() { _ = derived.Close(context.Background()) })
	return derived
}

func mustQueryCache(t *testing.T, scope *redisresource.Scope, policy Policy) *QueryCache {
	t.Helper()
	query, err := NewQueryCache(scope, policy)
	if err != nil {
		t.Fatalf("NewQueryCache: %v", err)
	}
	t.Cleanup(func() { _ = query.Close(context.Background()) })
	return query
}

func staticLoader(value string) Loader {
	return func(context.Context) (Result, error) {
		return Result{Payload: []byte(value), RowsAffected: 1}, nil
	}
}

func failingLoader(message string) Loader {
	return func(context.Context) (Result, error) { return Result{}, errors.New(message) }
}

func failingQueryLoader(message string) QueryLoader {
	return func(context.Context, *gorm.DB) (Result, error) { return Result{}, errors.New(message) }
}

func assertAuthorityLoad(t *testing.T, derived *Derived, target Target, value string) {
	t.Helper()
	outcome, err := derived.Load(context.Background(), target, staticLoader(value))
	if err != nil || outcome.Status != StatusStored || outcome.Source != SourceAuthority || string(outcome.Result.Payload) != value {
		t.Fatalf("isolated authority load = %#v, %v", outcome, err)
	}
}

func waitForFlightWaiters(t *testing.T, derived *Derived, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		derived.mu.Lock()
		waiters := 0
		for _, current := range derived.flights {
			waiters += current.waiters
		}
		derived.mu.Unlock()
		if waiters == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("singleflight did not collect %d waiters", want)
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if candidate != "" && len(candidate) <= len(value) {
			for index := 0; index+len(candidate) <= len(value); index++ {
				if value[index:index+len(candidate)] == candidate {
					return true
				}
			}
		}
	}
	return false
}

type testRecord struct {
	ID    uint `gorm:"primaryKey"`
	Value string
}

func newTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", digest(t.Name()))
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := database.AutoMigrate(&testRecord{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("unwrap test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return database
}

func seedRecord(t *testing.T, database *gorm.DB, value string) {
	t.Helper()
	if err := database.Create(&testRecord{ID: 1, Value: value}).Error; err != nil {
		t.Fatalf("seed record: %v", err)
	}
}

func recordLoader(id uint) QueryLoader {
	return func(ctx context.Context, database *gorm.DB) (Result, error) {
		var record testRecord
		query := database.WithContext(ctx).First(&record, id)
		if query.Error != nil {
			return Result{RowsAffected: query.RowsAffected}, query.Error
		}
		payload, err := json.Marshal(record)
		return Result{Payload: payload, RowsAffected: query.RowsAffected}, err
	}
}

func decodeRecord(t *testing.T, outcome Outcome) testRecord {
	t.Helper()
	var record testRecord
	if err := json.Unmarshal(outcome.Result.Payload, &record); err != nil {
		t.Fatalf("decode record: %v", err)
	}
	return record
}
