package cache

import (
	"context"
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

func TestRedis_QueryStoresNewCacheKeyInTagSet(t *testing.T) {
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

	ctx := context.WithValue(context.Background(), "gorm:cache:tag", "query_cache_records")
	var records []queryCacheRecord
	if err := db.WithContext(ctx).Find(&records).Error; err != nil {
		t.Fatalf("query records: %v", err)
	}
	if len(records) != 1 || records[0].Name != "acme" {
		t.Fatalf("expected seeded record, got %#v", records)
	}

	tag := "gorm.cache:query_cache_records"
	keys, err := client.SMembers(context.Background(), tag).Result()
	if err != nil {
		t.Fatalf("read tag set: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected one cached key in tag set, got %d: %#v", len(keys), keys)
	}
	if exists, err := client.Exists(context.Background(), keys[0]).Result(); err != nil || exists != 1 {
		t.Fatalf("expected cached key to exist, exists=%d err=%v", exists, err)
	}

	if err := r.RemoveFromTag(context.Background(), tag); err != nil {
		t.Fatalf("remove from tag: %v", err)
	}
	if exists, err := client.Exists(context.Background(), keys[0], tag).Result(); err != nil || exists != 0 {
		t.Fatalf("expected cached key and tag set to be removed, exists=%d err=%v", exists, err)
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
