package cache

import (
	"context"
	"crypto/sha256"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type queryCacheRecord struct {
	ID   int64
	Name string
}

type queryCacheCommandCounter struct {
	calls atomic.Int64
}

type queryCacheCommandBarrier struct {
	key         string
	blockAt     int64
	seen        atomic.Int64
	reached     chan struct{}
	release     chan struct{}
	releaseOnce sync.Once
}

type queryCacheRecordedCommand struct {
	name     string
	keyCount int
}

type queryCacheCommandRecorder struct {
	mu       sync.Mutex
	commands []queryCacheRecordedCommand
}

func (*queryCacheCommandCounter) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (counter *queryCacheCommandCounter) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		counter.calls.Add(1)
		return next(ctx, cmd)
	}
}

func (counter *queryCacheCommandCounter) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		counter.calls.Add(int64(len(cmds)))
		return next(ctx, cmds)
	}
}

func (*queryCacheCommandBarrier) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (barrier *queryCacheCommandBarrier) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		args := cmd.Args()
		if cmd.Name() == "get" && len(args) == 2 && args[1] == barrier.key && barrier.seen.Add(1) == barrier.blockAt {
			close(barrier.reached)
			<-barrier.release
		}
		return next(ctx, cmd)
	}
}

func (barrier *queryCacheCommandBarrier) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func (barrier *queryCacheCommandBarrier) Release() {
	barrier.releaseOnce.Do(func() { close(barrier.release) })
}

func (*queryCacheCommandRecorder) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (recorder *queryCacheCommandRecorder) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		recorder.mu.Lock()
		recorder.commands = append(recorder.commands, queryCacheRecordedCommand{
			name:     cmd.Name(),
			keyCount: len(cmd.Args()) - 1,
		})
		recorder.mu.Unlock()
		return next(ctx, cmd)
	}
}

func (recorder *queryCacheCommandRecorder) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func (recorder *queryCacheCommandRecorder) Snapshot() []queryCacheRecordedCommand {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]queryCacheRecordedCommand(nil), recorder.commands...)
}

func testRedisClient(t *testing.T) redis.UniversalClient {
	t.Helper()
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        []string{"127.0.0.1:6379"},
		DialTimeout:  200 * time.Millisecond,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
		MaxRetries:   0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("skip redis integration test: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func testMiniredisClient(t *testing.T) (redis.UniversalClient, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{server.Addr()},
	})
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})
	return client, server
}

func miniredisKeysWithPrefix(server *miniredis.Miniredis, prefix string) []string {
	keys := make([]string, 0)
	for _, key := range server.Keys() {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	return keys
}

func waitForQueryCacheBarrier(t *testing.T, barrier *queryCacheCommandBarrier) {
	t.Helper()
	select {
	case <-barrier.reached:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for query-cache command barrier")
	}
}

func TestNewRedisDoesNotCloseCallerOwnedClientAfterConnectFailure(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        []string{server.Addr()},
		DialTimeout:  100 * time.Millisecond,
		ReadTimeout:  100 * time.Millisecond,
		WriteTimeout: 100 * time.Millisecond,
		MaxRetries:   0,
	})
	t.Cleanup(func() { _ = client.Close() })

	server.Close()
	if cache, err := NewRedis(client, nil); err == nil || cache != nil {
		t.Fatalf("NewRedis() with unavailable external client = %#v, %v; want nil, error", cache, err)
	}
	if err := server.Restart(); err != nil {
		t.Fatalf("restart miniredis: %v", err)
	}
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("caller-owned client was closed after connect failure: %v", err)
	}
}

func TestRedis_QueryStoresGenerationKeyedEntryAndCleansLegacyTagSet(t *testing.T) {
	const queryCachePrefix = "test-query-cache:"

	client, server := testMiniredisClient(t)
	r, err := NewRedis(
		client,
		nil,
		WithQueryCacheKeys("*"),
		WithQueryCacheDuration(time.Minute),
		WithQueryCachePrefix(queryCachePrefix),
	)
	if err != nil {
		t.Fatalf("new redis cache: %v", err)
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&queryCacheRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&queryCacheRecord{Name: "acme"}).Error; err != nil {
		t.Fatalf("seed record: %v", err)
	}
	if err := r.Initialize(db); err != nil {
		t.Fatalf("initialize query cache: %v", err)
	}

	ctx := context.WithValue(context.Background(), "gorm:cache:tag", "query_cache_records")
	var records []queryCacheRecord
	if err := db.WithContext(ctx).Find(&records).Error; err != nil {
		t.Fatalf("query records: %v", err)
	}
	if len(records) != 1 || records[0].Name != "acme" {
		t.Fatalf("expected seeded record, got %#v", records)
	}

	tag := queryCachePrefix + "query_cache_records"
	keys := miniredisKeysWithPrefix(server, queryCachePrefix+generatedQueryCacheKeyNamespace)
	if len(keys) != 1 {
		t.Fatalf("expected one generated cache key, got %d: %#v", len(keys), keys)
	}
	if wantPrefix := queryCachePrefix + generatedQueryCacheKeyNamespace; !strings.HasPrefix(keys[0], wantPrefix) {
		t.Fatalf("cached key = %q; want prefix %q", keys[0], wantPrefix)
	}
	if exists, err := client.Exists(context.Background(), keys[0]).Result(); err != nil || exists != 1 {
		t.Fatalf("expected cached key to exist, exists=%d err=%v", exists, err)
	}

	if err := client.SAdd(context.Background(), tag, keys[0]).Err(); err != nil {
		t.Fatalf("seed legacy tag set: %v", err)
	}
	if err := r.RemoveFromTag(context.Background(), tag); err != nil {
		t.Fatalf("remove from tag: %v", err)
	}
	if exists, err := client.Exists(context.Background(), keys[0]).Result(); err != nil || exists != 1 {
		t.Fatalf("old generation data key exists=%d err=%v; want it retained until TTL", exists, err)
	}
	if exists, err := client.Exists(context.Background(), tag).Result(); err != nil || exists != 0 {
		t.Fatalf("legacy tag set exists=%d err=%v; want compatibility cleanup", exists, err)
	}
	generationKey := queryCacheGenerationKey(r.opts.QueryCachePrefix, queryCacheTagIdentity(tag))
	if generation, err := client.Get(context.Background(), generationKey).Result(); err != nil || !validQueryCacheGenerationToken(generation) {
		t.Fatalf("tag generation=%q err=%v; want a valid token", generation, err)
	}
}

func TestRedis_QueryCacheMissingGenerationCannotReviveOldData(t *testing.T) {
	client, server := testMiniredisClient(t)
	r, err := NewRedis(client, nil, WithQueryCacheKeys("*"), WithQueryCacheDuration(time.Minute))
	if err != nil {
		t.Fatalf("new redis cache: %v", err)
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&queryCacheRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	record := &queryCacheRecord{Name: "old"}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("seed record: %v", err)
	}
	if err := r.Initialize(db); err != nil {
		t.Fatalf("initialize query cache: %v", err)
	}

	rawTag := "query_cache_records"
	ctx := NewTag(context.Background(), rawTag)
	var first queryCacheRecord
	if err := db.WithContext(ctx).Where("id = ?", record.ID).Take(&first).Error; err != nil {
		t.Fatalf("prime cache: %v", err)
	}
	if first.Name != "old" {
		t.Fatalf("primed value = %q; want old", first.Name)
	}
	oldKeys := miniredisKeysWithPrefix(server, r.opts.QueryCachePrefix+generatedQueryCacheKeyNamespace)
	if len(oldKeys) != 1 {
		t.Fatalf("old data keys = %#v; want one", oldKeys)
	}
	generationKey := queryCacheGenerationKey(
		r.opts.QueryCachePrefix,
		queryCacheTagIdentity(r.opts.QueryCachePrefix+rawTag),
	)
	oldGeneration, err := client.Get(context.Background(), generationKey).Result()
	if err != nil {
		t.Fatalf("read initial generation: %v", err)
	}

	if err := db.Model(&queryCacheRecord{}).Where("id = ?", record.ID).Update("name", "new").Error; err != nil {
		t.Fatalf("update database: %v", err)
	}
	if err := client.Del(context.Background(), generationKey).Err(); err != nil {
		t.Fatalf("evict generation key: %v", err)
	}
	if exists, err := client.Exists(context.Background(), oldKeys[0]).Result(); err != nil || exists != 1 {
		t.Fatalf("old data key exists=%d err=%v; want retained", exists, err)
	}

	var second queryCacheRecord
	if err := db.WithContext(ctx).Where("id = ?", record.ID).Take(&second).Error; err != nil {
		t.Fatalf("query after generation eviction: %v", err)
	}
	if second.Name != "new" {
		t.Fatalf("query after generation eviction = %q; want new", second.Name)
	}
	newGeneration, err := client.Get(context.Background(), generationKey).Result()
	if err != nil {
		t.Fatalf("read replacement generation: %v", err)
	}
	if !validQueryCacheGenerationToken(newGeneration) || newGeneration == oldGeneration {
		t.Fatalf("replacement generation = %q; old=%q", newGeneration, oldGeneration)
	}
}

func TestRedis_QueryCacheSeparatesBoundVariables(t *testing.T) {
	client, server := testMiniredisClient(t)
	r, err := NewRedis(client, nil, WithQueryCacheKeys("*"), WithQueryCacheDuration(time.Minute))
	if err != nil {
		t.Fatalf("new redis cache: %v", err)
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&queryCacheRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&[]queryCacheRecord{{Name: "acme"}, {Name: "globex"}}).Error; err != nil {
		t.Fatalf("seed records: %v", err)
	}
	if err := r.Initialize(db); err != nil {
		t.Fatalf("initialize query cache: %v", err)
	}

	ctx := NewTag(context.Background(), "query_cache_records")
	var acme queryCacheRecord
	if err := db.WithContext(ctx).Where("name = ?", "acme").Take(&acme).Error; err != nil {
		t.Fatalf("query acme: %v", err)
	}
	var globex queryCacheRecord
	if err := db.WithContext(ctx).Where("name = ?", "globex").Take(&globex).Error; err != nil {
		t.Fatalf("query globex: %v", err)
	}
	if acme.Name != "acme" || globex.Name != "globex" {
		t.Fatalf("bound-variable queries shared cached data: acme=%#v globex=%#v", acme, globex)
	}

	keys := miniredisKeysWithPrefix(server, "gorm.cache:"+generatedQueryCacheKeyNamespace)
	if len(keys) != 2 {
		t.Fatalf("expected two cache keys for two bound values, got %d: %#v", len(keys), keys)
	}
}

func TestRedis_QueryCacheSeparatesDatabasesAndStillHitsWithinDatabase(t *testing.T) {
	client, server := testMiniredisClient(t)
	r, err := NewRedis(client, nil, WithQueryCacheKeys("*"), WithQueryCacheDuration(time.Minute))
	if err != nil {
		t.Fatalf("new redis cache: %v", err)
	}
	openDatabase := func(name string) *gorm.DB {
		t.Helper()
		db, openErr := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		if openErr != nil {
			t.Fatalf("open %s database: %v", name, openErr)
		}
		if migrateErr := db.AutoMigrate(&queryCacheRecord{}); migrateErr != nil {
			t.Fatalf("migrate %s database: %v", name, migrateErr)
		}
		if initializeErr := r.Initialize(db); initializeErr != nil {
			t.Fatalf("initialize %s query cache: %v", name, initializeErr)
		}
		return db
	}
	dbOne := openDatabase("one")
	dbTwo := openDatabase("two")
	if err := dbOne.Create(&queryCacheRecord{Name: "database-one"}).Error; err != nil {
		t.Fatalf("seed database one: %v", err)
	}
	if err := dbTwo.Create(&queryCacheRecord{Name: "database-two"}).Error; err != nil {
		t.Fatalf("seed database two: %v", err)
	}

	ctx := NewTag(context.Background(), "query_cache_records")
	var fromOne queryCacheRecord
	if err := dbOne.WithContext(ctx).Where("id = ?", 1).Take(&fromOne).Error; err != nil {
		t.Fatalf("query database one: %v", err)
	}
	var fromTwo queryCacheRecord
	if err := dbTwo.WithContext(ctx).Where("id = ?", 1).Take(&fromTwo).Error; err != nil {
		t.Fatalf("query database two: %v", err)
	}
	if fromOne.Name != "database-one" || fromTwo.Name != "database-two" {
		t.Fatalf("databases shared cached data: one=%#v two=%#v", fromOne, fromTwo)
	}

	if err := dbOne.Model(&queryCacheRecord{}).Where("id = ?", 1).Update("name", "database-one-updated").Error; err != nil {
		t.Fatalf("update database one behind cache: %v", err)
	}
	var cachedAgain queryCacheRecord
	if err := dbOne.WithContext(ctx).Where("id = ?", 1).Take(&cachedAgain).Error; err != nil {
		t.Fatalf("query database one again: %v", err)
	}
	if cachedAgain.Name != "database-one" {
		t.Fatalf("same database did not hit its existing cache entry: %#v", cachedAgain)
	}

	keys := miniredisKeysWithPrefix(server, "gorm.cache:"+generatedQueryCacheKeyNamespace)
	if len(keys) != 2 {
		t.Fatalf("two databases produced %d cache keys; want 2: %#v", len(keys), keys)
	}
}

func TestRedis_QueryCacheHonorsExplicitTagAllowlist(t *testing.T) {
	const (
		allowedTag = "query_cache_records"
		deniedTag  = "other_records"
	)

	client, server := testMiniredisClient(t)
	r, err := NewRedis(
		client,
		nil,
		WithQueryCacheKeys(allowedTag),
		WithQueryCacheDuration(time.Minute),
	)
	if err != nil {
		t.Fatalf("new redis cache: %v", err)
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&queryCacheRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&[]queryCacheRecord{{Name: "acme"}, {Name: "globex"}}).Error; err != nil {
		t.Fatalf("seed records: %v", err)
	}
	if err := r.Initialize(db); err != nil {
		t.Fatalf("initialize query cache: %v", err)
	}

	var allowed queryCacheRecord
	if err := db.WithContext(NewTag(context.Background(), allowedTag)).
		Where("name = ?", "acme").
		Take(&allowed).Error; err != nil {
		t.Fatalf("query allowed table: %v", err)
	}
	if allowed.Name != "acme" {
		t.Fatalf("allowed query returned %#v; want acme", allowed)
	}
	allowedKeys := miniredisKeysWithPrefix(server, "gorm.cache:"+generatedQueryCacheKeyNamespace)
	if len(allowedKeys) != 1 {
		t.Fatalf("allowed query cached keys = %#v; want one", allowedKeys)
	}

	var denied queryCacheRecord
	if err := db.WithContext(NewTag(context.Background(), deniedTag)).
		Where("name = ?", "globex").
		Take(&denied).Error; err != nil {
		t.Fatalf("query denied table: %v", err)
	}
	if denied.Name != "globex" {
		t.Fatalf("denied query returned %#v; want globex from database", denied)
	}
	if exists, err := client.Exists(context.Background(), "gorm.cache:"+deniedTag).Result(); err != nil || exists != 0 {
		t.Fatalf("denied table tag exists=%d err=%v; want no cache entry", exists, err)
	}
	deniedGenerationKey := queryCacheGenerationKey(
		r.opts.QueryCachePrefix,
		queryCacheTagIdentity(r.opts.QueryCachePrefix+deniedTag),
	)
	if exists, err := client.Exists(context.Background(), deniedGenerationKey).Result(); err != nil || exists != 0 {
		t.Fatalf("denied generation exists=%d err=%v; want no cache metadata", exists, err)
	}
	if keys := server.Keys(); len(keys) != 2 {
		t.Fatalf("Redis keys after denied query = %#v; want only allowed data and generation keys", keys)
	}
}

func TestGenerateQueryCacheKeyIsStableOpaqueAndVariableSensitive(t *testing.T) {
	const (
		namespace = "private-namespace"
		querySQL  = "SELECT * FROM records WHERE secret = ? AND id = ?"
		secret    = "super-sensitive-cache-parameter"
	)

	const cachePrefix = "test-query-cache:"
	databaseIdentity := []byte("test-database-identity")
	tagIdentity := queryCacheTagIdentity(cachePrefix + "records")
	first, err := generateQueryCacheKey(cachePrefix, databaseIdentity, tagIdentity, "generation-a", namespace, querySQL, []any{secret, 42})
	if err != nil {
		t.Fatalf("generate first query cache key: %v", err)
	}
	same, err := generateQueryCacheKey(cachePrefix, databaseIdentity, tagIdentity, "generation-a", namespace, querySQL, []any{secret, int64(42)})
	if err != nil {
		t.Fatalf("generate equivalent query cache key: %v", err)
	}
	different, err := generateQueryCacheKey(cachePrefix, databaseIdentity, tagIdentity, "generation-a", namespace, querySQL, []any{"different", 42})
	if err != nil {
		t.Fatalf("generate different query cache key: %v", err)
	}

	if first != same {
		t.Fatalf("equivalent driver values generated different keys: %q != %q", first, same)
	}
	if first == different {
		t.Fatalf("different bound variables generated the same key: %q", first)
	}
	differentGeneration, err := generateQueryCacheKey(cachePrefix, databaseIdentity, tagIdentity, "generation-b", namespace, querySQL, []any{secret, 42})
	if err != nil {
		t.Fatalf("generate different-generation query cache key: %v", err)
	}
	if first == differentGeneration {
		t.Fatalf("different tag generations generated the same key: %q", first)
	}
	wantPrefix := cachePrefix + generatedQueryCacheKeyNamespace
	if !strings.HasPrefix(first, wantPrefix) {
		t.Fatalf("generated key = %q; want prefix %q", first, wantPrefix)
	}
	if got, want := len(first), len(wantPrefix)+(sha256.Size*2); got != want {
		t.Fatalf("generated key length = %d; want %d", got, want)
	}
	for _, sensitive := range []string{string(databaseIdentity), namespace, querySQL, secret} {
		if strings.Contains(first, sensitive) {
			t.Fatalf("generated key leaked sensitive input %q: %q", sensitive, first)
		}
	}
}

func TestRedis_QueryBypassSkipsAllCacheIO(t *testing.T) {
	client, _ := testMiniredisClient(t)
	r, err := NewRedis(client, nil, WithQueryCacheKeys("*"), WithQueryCacheDuration(time.Minute))
	if err != nil {
		t.Fatalf("new redis cache: %v", err)
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&queryCacheRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&queryCacheRecord{Name: "acme"}).Error; err != nil {
		t.Fatalf("seed record: %v", err)
	}
	if err := r.Initialize(db); err != nil {
		t.Fatalf("initialize query cache: %v", err)
	}

	counter := &queryCacheCommandCounter{}
	client.AddHook(counter)
	ctx := NewBypass(NewTag(context.Background(), "query_cache_records"))
	var records []queryCacheRecord
	if err := db.WithContext(ctx).Find(&records).Error; err != nil {
		t.Fatalf("query records: %v", err)
	}
	if len(records) != 1 || records[0].Name != "acme" {
		t.Fatalf("expected seeded record, got %#v", records)
	}
	if calls := counter.calls.Load(); calls != 0 {
		t.Fatalf("bypassed query issued %d Redis commands; want 0", calls)
	}
}

func TestRedis_QueryUsesPerRequestExpiration(t *testing.T) {
	client, server := testMiniredisClient(t)
	r, err := NewRedis(client, nil, WithQueryCacheKeys("*"), WithQueryCacheDuration(time.Minute))
	if err != nil {
		t.Fatalf("new redis cache: %v", err)
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&queryCacheRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&queryCacheRecord{Name: "acme"}).Error; err != nil {
		t.Fatalf("seed record: %v", err)
	}
	if err := r.Initialize(db); err != nil {
		t.Fatalf("initialize query cache: %v", err)
	}

	ctx := NewExpiration(NewTag(context.Background(), "query_cache_records"), 3*time.Second)
	var records []queryCacheRecord
	if err := db.WithContext(ctx).Find(&records).Error; err != nil {
		t.Fatalf("query records: %v", err)
	}
	keys := miniredisKeysWithPrefix(server, "gorm.cache:"+generatedQueryCacheKeyNamespace)
	if len(keys) != 1 {
		t.Fatalf("expected one cached key, got %d: %#v", len(keys), keys)
	}
	if ttl := server.TTL(keys[0]); ttl != 3*time.Second {
		t.Fatalf("cache TTL = %v; want 3s", ttl)
	}
}

func TestRedis_QueryDoesNotPublishDatabaseReadAfterConcurrentInvalidation(t *testing.T) {
	client, server := testMiniredisClient(t)
	r, err := NewRedis(client, nil, WithQueryCacheKeys("*"), WithQueryCacheDuration(time.Minute))
	if err != nil {
		t.Fatalf("new redis cache: %v", err)
	}
	db, err := gorm.Open(
		sqlite.Open("file:query_cache_generation_miss?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&queryCacheRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	record := &queryCacheRecord{Name: "before-invalidation"}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("seed record: %v", err)
	}
	if err := r.Initialize(db); err != nil {
		t.Fatalf("initialize query cache: %v", err)
	}

	const rawTag = "query_cache_records"
	tag := "gorm.cache:" + rawTag
	barrier := &queryCacheCommandBarrier{
		key:     queryCacheGenerationKey(r.opts.QueryCachePrefix, queryCacheTagIdentity(tag)),
		blockAt: 2,
		reached: make(chan struct{}),
		release: make(chan struct{}),
	}
	t.Cleanup(barrier.Release)
	client.AddHook(barrier)

	type queryResult struct {
		record queryCacheRecord
		err    error
	}
	result := make(chan queryResult, 1)
	go func() {
		var loaded queryCacheRecord
		queryErr := db.WithContext(NewTag(context.Background(), rawTag)).
			Where("id = ?", record.ID).
			Take(&loaded).Error
		result <- queryResult{record: loaded, err: queryErr}
	}()

	waitForQueryCacheBarrier(t, barrier)
	if err := db.Model(&queryCacheRecord{}).
		Where("id = ?", record.ID).
		Update("name", "after-invalidation").Error; err != nil {
		t.Fatalf("update record while old database result is pending: %v", err)
	}
	if err := r.RemoveFromTag(context.Background(), tag); err != nil {
		t.Fatalf("advance tag generation: %v", err)
	}
	barrier.Release()

	select {
	case completed := <-result:
		if completed.err != nil {
			t.Fatalf("query old database result: %v", completed.err)
		}
		if completed.record.Name != "before-invalidation" {
			t.Fatalf("controlled old read returned %#v; want before-invalidation", completed.record)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for old database read")
	}
	if keys := miniredisKeysWithPrefix(server, "gorm.cache:"+generatedQueryCacheKeyNamespace); len(keys) != 0 {
		t.Fatalf("old database result was resurrected in cache: %#v", keys)
	}

	var current queryCacheRecord
	if err := db.WithContext(NewTag(context.Background(), rawTag)).
		Where("id = ?", record.ID).
		Take(&current).Error; err != nil {
		t.Fatalf("query current database result: %v", err)
	}
	if current.Name != "after-invalidation" {
		t.Fatalf("current query returned %#v; want after-invalidation", current)
	}
	if keys := miniredisKeysWithPrefix(server, "gorm.cache:"+generatedQueryCacheKeyNamespace); len(keys) != 1 {
		t.Fatalf("stable current result produced cache keys %#v; want one", keys)
	}
}

func TestRedis_QueryRejectsCacheHitInvalidatedBeforeGenerationConfirmation(t *testing.T) {
	client, server := testMiniredisClient(t)
	r, err := NewRedis(client, nil, WithQueryCacheKeys("*"), WithQueryCacheDuration(time.Minute))
	if err != nil {
		t.Fatalf("new redis cache: %v", err)
	}
	db, err := gorm.Open(
		sqlite.Open("file:query_cache_generation_hit?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&queryCacheRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	record := &queryCacheRecord{Name: "cached-value"}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("seed record: %v", err)
	}
	if err := r.Initialize(db); err != nil {
		t.Fatalf("initialize query cache: %v", err)
	}

	const rawTag = "query_cache_records"
	ctx := NewTag(context.Background(), rawTag)
	var primed queryCacheRecord
	if err := db.WithContext(ctx).Where("id = ?", record.ID).Take(&primed).Error; err != nil {
		t.Fatalf("prime query cache: %v", err)
	}
	if primed.Name != "cached-value" {
		t.Fatalf("primed result = %#v; want cached-value", primed)
	}
	if err := db.Model(&queryCacheRecord{}).
		Where("id = ?", record.ID).
		Update("name", "database-value").Error; err != nil {
		t.Fatalf("update database behind primed cache: %v", err)
	}

	tag := "gorm.cache:" + rawTag
	barrier := &queryCacheCommandBarrier{
		key:     queryCacheGenerationKey(r.opts.QueryCachePrefix, queryCacheTagIdentity(tag)),
		blockAt: 2,
		reached: make(chan struct{}),
		release: make(chan struct{}),
	}
	t.Cleanup(barrier.Release)
	client.AddHook(barrier)

	type queryResult struct {
		record queryCacheRecord
		err    error
	}
	result := make(chan queryResult, 1)
	go func() {
		var loaded queryCacheRecord
		queryErr := db.WithContext(ctx).Where("id = ?", record.ID).Take(&loaded).Error
		result <- queryResult{record: loaded, err: queryErr}
	}()

	waitForQueryCacheBarrier(t, barrier)
	if err := r.RemoveFromTag(context.Background(), tag); err != nil {
		t.Fatalf("invalidate fetched cache hit: %v", err)
	}
	barrier.Release()

	select {
	case completed := <-result:
		if completed.err != nil {
			t.Fatalf("query after concurrent cache-hit invalidation: %v", completed.err)
		}
		if completed.record.Name != "database-value" {
			t.Fatalf("invalidated cache hit returned %#v; want database-value", completed.record)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for cache-hit query")
	}
	if keys := miniredisKeysWithPrefix(server, "gorm.cache:"+generatedQueryCacheKeyNamespace); len(keys) != 2 {
		t.Fatalf("old and current generations produced cache keys %#v; want two TTL-bound entries", keys)
	}
}

func TestRedis_RemoveFromTagUsesClusterSafeSingleKeyCommands(t *testing.T) {
	client, _ := testMiniredisClient(t)
	r, err := NewRedis(client, nil)
	if err != nil {
		t.Fatalf("new redis cache: %v", err)
	}
	const tag = "gorm.cache:cluster_records"
	legacyKeys := []string{"legacy:data:one", "legacy:data:two"}
	for _, key := range legacyKeys {
		if err := client.Set(context.Background(), key, "payload", time.Minute).Err(); err != nil {
			t.Fatalf("seed legacy data key %q: %v", key, err)
		}
	}
	if err := client.SAdd(context.Background(), tag, legacyKeys).Err(); err != nil {
		t.Fatalf("seed legacy tag set: %v", err)
	}

	recorder := &queryCacheCommandRecorder{}
	client.AddHook(recorder)
	if err := r.RemoveFromTag(context.Background(), tag); err != nil {
		t.Fatalf("remove from tag: %v", err)
	}
	commands := recorder.Snapshot()
	if len(commands) != 2 {
		t.Fatalf("invalidation commands = %#v; want SET and one-key DEL", commands)
	}
	wantNames := []string{"set", "del"}
	wantArgumentCounts := []int{2, 1}
	for index, command := range commands {
		if command.name != wantNames[index] || command.keyCount != wantArgumentCounts[index] {
			t.Fatalf("invalidation command %d = %#v; want %s with one key", index, command, wantNames[index])
		}
	}

	generationKey := queryCacheGenerationKey(r.opts.QueryCachePrefix, queryCacheTagIdentity(tag))
	if generation, err := client.Get(context.Background(), generationKey).Result(); err != nil || !validQueryCacheGenerationToken(generation) {
		t.Fatalf("tag generation=%q err=%v; want a valid token", generation, err)
	}
	if exists, err := client.Exists(context.Background(), tag).Result(); err != nil || exists != 0 {
		t.Fatalf("legacy tag set exists=%d err=%v; want deleted", exists, err)
	}
	if exists, err := client.Exists(context.Background(), legacyKeys...).Result(); err != nil || exists != int64(len(legacyKeys)) {
		t.Fatalf("legacy data keys exist=%d err=%v; want TTL-bound keys retained", exists, err)
	}
}

func TestRedis_RemoveFromTagHandlesEmptyTag(t *testing.T) {
	client, _ := testMiniredisClient(t)
	r, err := NewRedis(client, nil)
	if err != nil {
		t.Fatalf("new redis cache: %v", err)
	}

	if err := r.RemoveFromTag(context.Background(), "gorm.cache:missing"); err != nil {
		t.Fatalf("remove missing tag: %v", err)
	}
}

func TestRedis_HashSet(t *testing.T) {
	client := testRedisClient(t)
	r := &Redis{UniversalClient: client}
	ctx := context.Background()
	hashKey := "cache:test:hash-set"
	field := "field"
	value := "value"
	t.Cleanup(func() {
		_ = r.Del(context.Background(), hashKey).Err()
	})

	if err := r.HSet(ctx, hashKey, field, value).Err(); err != nil {
		t.Fatalf("HSet() error = %v", err)
	}
	if err := r.Expire(ctx, hashKey, time.Minute).Err(); err != nil {
		t.Fatalf("Expire() error = %v", err)
	}

	got, err := r.HGet(ctx, hashKey, field).Result()
	if err != nil {
		t.Fatalf("HGet() after HSet error = %v", err)
	}
	if got != value {
		t.Fatalf("HGet() after HSet got = %q, want %q", got, value)
	}
	ttl, err := r.TTL(ctx, hashKey).Result()
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Fatalf("TTL() got = %v, want a positive value no greater than %v", ttl, time.Minute)
	}
}

func TestRedis_HashGet(t *testing.T) {
	client := testRedisClient(t)
	r := &Redis{UniversalClient: client}
	ctx := context.Background()
	hashKey := "cache:test:hash-get"
	field := "field"
	want := "value"
	t.Cleanup(func() {
		_ = r.Del(context.Background(), hashKey).Err()
	})

	if err := r.HSet(ctx, hashKey, field, want).Err(); err != nil {
		t.Fatalf("seed hash: %v", err)
	}
	got, err := r.HGet(ctx, hashKey, field).Result()
	if err != nil {
		t.Fatalf("HGet() error = %v", err)
	}
	if got != want {
		t.Fatalf("HGet() got = %q, want %q", got, want)
	}
}

func TestRedis_HashDel(t *testing.T) {
	client := testRedisClient(t)
	r := &Redis{UniversalClient: client}
	ctx := context.Background()
	hashKey := "cache:test:hash-del"
	field := "field"
	t.Cleanup(func() {
		_ = r.Del(context.Background(), hashKey).Err()
	})

	if err := r.HSet(ctx, hashKey, field, "value").Err(); err != nil {
		t.Fatalf("seed hash: %v", err)
	}
	if err := r.HDel(ctx, hashKey, field).Err(); err != nil {
		t.Fatalf("HDel() error = %v", err)
	}
	exists, err := r.HExists(ctx, hashKey, field).Result()
	if err != nil {
		t.Fatalf("HExists() error = %v", err)
	}
	if exists {
		t.Fatal("HDel() left the field in the hash")
	}
}

func TestRedis_Set(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value any
	}{
		{name: "string", key: "cache:test:set:string", value: "set-value"},
		{name: "empty string", key: "cache:test:set:empty", value: ""},
		{name: "integer", key: "cache:test:set:int", value: 123},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testRedisClient(t)
			r := &Redis{UniversalClient: client}
			ctx := context.Background()
			t.Cleanup(func() {
				_ = r.Del(context.Background(), tt.key).Err()
			})

			if err := r.Set(ctx, tt.key, tt.value, 10*time.Second).Err(); err != nil {
				t.Fatalf("Set() error = %v", err)
			}
			if exists, err := r.Exists(ctx, tt.key).Result(); err != nil || exists != 1 {
				t.Fatalf("Set() did not persist key, exists=%d err=%v", exists, err)
			}
		})
	}
}

func TestRedis_Get(t *testing.T) {
	client := testRedisClient(t)
	r := &Redis{UniversalClient: client}
	ctx := context.Background()
	key := "cache:test:get"
	want := "get-value"
	t.Cleanup(func() {
		_ = r.Del(context.Background(), key).Err()
	})

	if err := r.Set(ctx, key, want, time.Minute).Err(); err != nil {
		t.Fatalf("seed value: %v", err)
	}
	got, err := r.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != want {
		t.Fatalf("Get() got = %q, want %q", got, want)
	}
}

func TestRedis_Del(t *testing.T) {
	client := testRedisClient(t)
	r := &Redis{UniversalClient: client}
	ctx := context.Background()
	key := "cache:test:del"

	if err := r.Set(ctx, key, "value", time.Minute).Err(); err != nil {
		t.Fatalf("seed value: %v", err)
	}
	if err := r.Del(ctx, key).Err(); err != nil {
		t.Fatalf("Del() error = %v", err)
	}
	if exists, err := r.Exists(ctx, key).Result(); err != nil || exists != 0 {
		t.Fatalf("Del() left key behind, exists=%d err=%v", exists, err)
	}
}
