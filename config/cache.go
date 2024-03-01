package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/1 10:12:15
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/1 10:12:15
 */

import (
	"log"

	"github.com/mss-boot-io/mss-boot-admin/center"

	"github.com/mss-boot-io/mss-boot-admin/storage/cache"
)

type Cache struct {
	Redis  *RedisConnectOptions
	Memory interface{}
}

// Init 构造cache 顺序 redis > 其他 > memory
func (e Cache) Init() {
	if e.Redis != nil {
		options, err := e.Redis.GetRedisOptions()
		if err != nil {
			log.Fatalf("cache redis init error: %s", err.Error())
		}
		r, err := cache.NewRedis(GetRedisClient(), options)
		if err != nil {
			log.Fatalf("cache redis init error: %s", err.Error())
		}
		if _redis == nil {
			_redis = r.GetClient()
		}
		center.SetCache(r)
		return
	}
	center.SetCache(cache.NewMemory())
}
