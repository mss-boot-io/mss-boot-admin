package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/1 10:12:15
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/1 10:12:15
 */

import (
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/config/storage"
	"github.com/mss-boot-io/mss-boot/pkg/config/storage/cache"
)

type Cache struct {
	QueryCache         bool          `yaml:"queryCache" json:"queryCache"`
	QueryCacheDuration time.Duration `yaml:"queryCacheDuration" json:"queryCacheDuration"`
	QueryCacheKeys     []string      `yaml:"queryCacheKeys" json:"queryCacheKeys"`
	Redis              *storage.RedisConnectOptions
}

// Init 构造cache 顺序 redis > 其他 > memory
func (e Cache) Init(set func(storage.AdapterCache), queryCache func(tx *gorm.DB, duration time.Duration)) {
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
		r, err := cache.NewRedis(storage.GetRedisClient(), options, opts...)
		if err != nil {
			log.Fatalf("cache redis init error: %s", err.Error())
		}
		if storage.GetRedisClient() == nil {
			storage.SetRedisClient(r.GetClient())
		}
		if set != nil {
			set(r)
		}
	}
	if e.QueryCache && e.QueryCacheDuration > 0 && gormdb.DB != nil && queryCache != nil {
		queryCache(gormdb.DB, e.QueryCacheDuration)
	}
}
