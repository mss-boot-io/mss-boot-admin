package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/1 10:20:42
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/1 10:20:42
 */

import (
	"log"

	"github.com/redis/go-redis/v9"

	"github.com/mss-boot-io/mss-boot/pkg/config/storage"
	"github.com/mss-boot-io/mss-boot/pkg/config/storage/locker"
)

type Locker struct {
	Redis *storage.RedisConnectOptions
}

// Empty 空设置
func (e *Locker) Empty() bool {
	return e.Redis == nil
}

// Init 启用顺序 redis > 其他 > memory
func (e *Locker) Init(set func(storage.AdapterLocker)) {
	if e.Redis != nil {
		client := storage.GetRedisClient()
		if client == nil {
			options, err := e.Redis.GetRedisOptions()
			if err != nil {
				log.Fatalf("locker redis init error: %s", err.Error())
			}
			client = redis.NewUniversalClient(options)
			storage.SetRedisClient(client)
		}
		if set != nil {
			set(locker.NewRedis(client))
		}
		return
	}
}
