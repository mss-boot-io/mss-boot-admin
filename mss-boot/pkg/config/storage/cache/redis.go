package cache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
)

// NewRedis redis模式
func NewRedis(client redis.UniversalClient, options *redis.UniversalOptions, opts ...Option) (*Redis, error) {
	o := DefaultOptions()
	for _, option := range opts {
		option(&o)
	}
	if client == nil {
		client = redis.NewUniversalClient(options)
	}
	r := &Redis{
		UniversalClient: client,
		opts:            o,
	}
	err := r.connect()
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Redis cache implement
type Redis struct {
	redis.UniversalClient
	opts Options
}

func (r *Redis) Initialize(tx *gorm.DB) error {
	return tx.Callback().Query().Replace("gorm:query", r.Query)
}

func (*Redis) Name() string {
	return "gorm:cache"
}

func (*Redis) String() string {
	return "redis"
}

// connect connect test
func (r *Redis) connect() error {
	var err error
	_, err = r.Ping(context.TODO()).Result()
	return err
}

func (r *Redis) Query(tx *gorm.DB) {
	ctx := tx.Statement.Context

	var (
		key    string
		hasKey bool
	)

	// 调用gorm的方法生产SQL
	callbacks.BuildQuerySQL(tx)

	// 是否有自定义key
	if key, hasKey = FromKey(ctx); !hasKey || !r.opts.HasKey(key) {
		key = r.generateKey(tx.Statement.SQL.String())
	}

	var useCache bool
	tag, hasTag := FromTag(ctx)
	tag = r.opts.QueryCachePrefix + tag
	if hasTag && r.opts.HasKey(tag) {
		useCache = true
	}

	// 查询缓存数据

	if useCache {
		if err := r.QueryCache(ctx, key, tx.Statement.Dest); err == nil {
			_ = r.SaveTagKey(ctx, tag, key)
			return
		}
	}

	// 查询数据库
	QueryDB(tx)

	if tx.Error != nil {
		return
	}
	if !useCache {
		return
	}

	// 写入缓存
	if err := r.SaveCache(ctx, key, tx.Statement.Dest, r.opts.QueryCacheDuration); err != nil {
		tx.Logger.Error(ctx, err.Error())
		return
	}
	_ = r.SaveTagKey(ctx, tag, key)
}

func (r *Redis) QueryCache(ctx context.Context, key string, dest any) error {
	s, err := r.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	if s == "" {
		return gorm.ErrRecordNotFound
	}
	switch dest.(type) {
	case *int64:
		dest = 0
	}
	return json.Unmarshal([]byte(s), dest)
}

func (r *Redis) SaveCache(ctx context.Context, key string, dest any, ttl time.Duration) error {
	s, err := json.Marshal(dest)
	if err != nil {
		return err
	}
	return r.Set(ctx, key, string(s), ttl).Err()
}

func (r *Redis) SaveTagKey(ctx context.Context, tag, key string) error {
	return r.SAdd(ctx, tag, key).Err()
}

func (r *Redis) RemoveFromTag(ctx context.Context, tag string) error {
	keys, err := r.SMembers(ctx, tag).Result()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return r.Del(ctx, tag).Err()
	}
	keys = append(keys, tag)
	return r.Del(ctx, keys...).Err()
}

// GetClient 暴露原生client
func (r *Redis) GetClient() redis.UniversalClient {
	return r
}

func (r *Redis) generateKey(key string) string {
	return base64.StdEncoding.EncodeToString([]byte(key))
}
