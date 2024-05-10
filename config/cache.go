package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/1 10:12:15
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/1 10:12:15
 */

import (
	"context"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions/gorm"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/center"

	"github.com/mss-boot-io/mss-boot-admin/storage/cache"
)

type Cache struct {
	QueryCache         bool          `yaml:"queryCache" json:"queryCache"`
	QueryCacheDuration time.Duration `yaml:"queryCacheDuration" json:"queryCacheDuration"`
	QueryCacheKeys     []string      `yaml:"queryCacheKeys" json:"queryCacheKeys"`
	Redis              *RedisConnectOptions
	Memory             interface{}
}

// Init 构造cache 顺序 redis > 其他 > memory
func (e Cache) Init() {
	opts := make([]cache.Option, 0)
	if len(e.QueryCacheKeys) > 0 {
		opts = append(opts, cache.WithQueryCacheKeys(e.QueryCacheKeys...))
	}
	if e.QueryCacheDuration > 0 {
		opts = append(opts, cache.WithQueryCacheDuration(e.QueryCacheDuration))
	}
	if e.Redis != nil {
		options, err := e.Redis.GetRedisOptions()
		if err != nil {
			log.Fatalf("cache redis init error: %s", err.Error())
		}
		r, err := cache.NewRedis(GetRedisClient(), options, opts...)
		if err != nil {
			log.Fatalf("cache redis init error: %s", err.Error())
		}
		if _redis == nil {
			_redis = r.GetClient()
		}
		center.SetCache(r)
		return
	}
	center.SetCache(cache.NewMemory(opts...))
	if e.QueryCache && e.QueryCacheDuration > 0 && gormdb.DB != nil {
		cache.NewExpiration(context.Background(), e.QueryCacheDuration)
		if err := gormdb.DB.Use(center.GetCache()); err != nil {
			slog.Error("gorm use cache error", "err", err)
			os.Exit(-1)
		}
		gorm.CleanCacheFromTag = center.GetCache().RemoveFromTag
	}
}
