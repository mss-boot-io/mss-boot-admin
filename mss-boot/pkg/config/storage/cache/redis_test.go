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
	type fields struct {
		client redis.UniversalClient
	}
	type args struct {
		ctx    context.Context
		hk     string
		key    string
		val    interface{}
		expire time.Duration
	}
	client := testRedisClient(t)
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "hash set",
			fields: fields{
				client: client,
			},
			args: args{
				ctx:    context.Background(),
				hk:     "h-set-test",
				key:    "h-set-key",
				val:    "h-set-value",
				expire: time.Minute,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Redis{
				UniversalClient: tt.fields.client,
			}
			if err := r.HSet(tt.args.ctx, tt.args.hk, tt.args.key, tt.args.val, tt.args.expire).Err(); (err != nil) != tt.wantErr {
				t.Errorf("HashSet() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRedis_HashGet(t *testing.T) {
	type fields struct {
		client redis.UniversalClient
	}
	type args struct {
		ctx context.Context
		hk  string
		key string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "hash get",
			fields: fields{
				client: testRedisClient(t),
			},
			args: args{
				ctx: context.Background(),
				hk:  "h-set-test",
				key: "h-set-key",
			},
			want:    "h-set-value",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Redis{
				UniversalClient: tt.fields.client,
			}
			got, err := r.HGet(tt.args.ctx, tt.args.hk, tt.args.key).Result()
			if (err != nil) != tt.wantErr {
				t.Errorf("HashGet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("HashGet() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRedis_HashDel(t *testing.T) {
	type fields struct {
		client redis.UniversalClient
	}
	type args struct {
		ctx context.Context
		hk  string
		key string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "hash del",
			fields: fields{
				client: testRedisClient(t),
			},
			args: args{
				ctx: context.Background(),
				hk:  "h-set-test",
				key: "h-set-key",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Redis{
				UniversalClient: tt.fields.client,
			}
			if err := r.HDel(tt.args.ctx, tt.args.hk, tt.args.key).Err(); (err != nil) != tt.wantErr {
				t.Errorf("HashDel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRedis_Set(t *testing.T) {
	type fields struct {
		client redis.UniversalClient
		opts   Options
	}
	type args struct {
		ctx    context.Context
		key    string
		val    interface{}
		expire time.Duration
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "set",
			fields: fields{
				client: testRedisClient(t),
				opts: Options{
					QueryCacheDuration: 10 * time.Second,
				},
			},
			args: args{
				ctx:    context.Background(),
				key:    "set-test",
				val:    "set-value",
				expire: 10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "set with empty value",
			fields: fields{
				client: testRedisClient(t),
				opts: Options{
					QueryCacheDuration: 10 * time.Second,
				},
			},
			args: args{
				ctx:    context.Background(),
				key:    "set-test",
				val:    "",
				expire: 10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "set with int value",
			fields: fields{
				client: testRedisClient(t),
				opts: Options{
					QueryCacheDuration: 10 * time.Second,
				},
			},
			args: args{
				ctx:    context.Background(),
				key:    "set-test-int",
				val:    123,
				expire: 10 * time.Second,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Redis{
				UniversalClient: tt.fields.client,
				opts:            tt.fields.opts,
			}
			if err := r.Set(tt.args.ctx, tt.args.key, tt.args.val, tt.args.expire).Err(); (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRedis_Get(t *testing.T) {
	type fields struct {
		client redis.UniversalClient
	}
	type args struct {
		ctx context.Context
		key string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "get",
			fields: fields{
				client: testRedisClient(t),
			},
			args: args{
				ctx: context.Background(),
				key: "set-test",
			},
			want:    "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Redis{
				UniversalClient: tt.fields.client,
			}
			got, err := r.Get(tt.args.ctx, tt.args.key).Result()
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRedis_Del(t *testing.T) {
	type fields struct {
		client redis.UniversalClient
	}
	type args struct {
		ctx context.Context
		key string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "del",
			fields: fields{
				client: testRedisClient(t),
			},
			args: args{
				ctx: context.Background(),
				key: "set-test",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Redis{
				UniversalClient: tt.fields.client,
			}
			if err := r.Del(tt.args.ctx, tt.args.key).Err(); (err != nil) != tt.wantErr {
				t.Errorf("Del() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
