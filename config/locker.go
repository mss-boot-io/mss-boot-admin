package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/1 10:20:42
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/1 10:20:42
 */

import (
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/redis/go-redis/v9"
	"log"

	"github.com/mss-boot-io/mss-boot-admin/storage/locker"
)

type Locker struct {
	Redis *RedisConnectOptions
}

// Empty 空设置
func (e *Locker) Empty() bool {
	return e.Redis == nil
}

// Init 启用顺序 redis > 其他 > memory
func (e *Locker) Init() {
	if e.Redis != nil {
		client := GetRedisClient()
		if client == nil {
			options, err := e.Redis.GetRedisOptions()
			if err != nil {
				log.Fatalf("locker redis init error: %s", err.Error())
			}
			client = redis.NewClient(options)
			_redis = client
		}
		center.SetLocker(locker.NewRedis(client))
		return
	}
}
