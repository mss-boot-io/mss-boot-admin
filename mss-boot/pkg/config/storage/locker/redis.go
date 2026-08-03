package locker

import (
	"context"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
)

// NewRedis 初始化locker
func NewRedis(c redis.UniversalClient) *Redis {
	return &Redis{
		client: c,
	}
}

type Redis struct {
	client redis.UniversalClient
	mutex  *redislock.Client
}

func (*Redis) String() string {
	return "redis"
}

func (r *Redis) Lock(ctx context.Context, key string, ttl time.Duration, options *redislock.Options) (*redislock.Lock, error) {
	if r.mutex == nil {
		r.mutex = redislock.New(r.client)
	}
	return r.mutex.Obtain(ctx, key, ttl, options)
}
