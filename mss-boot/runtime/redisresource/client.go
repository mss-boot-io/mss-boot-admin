package redisresource

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
	"github.com/redis/go-redis/v9"
)

type ownedClient interface {
	Ping(context.Context) error
	Get(context.Context, string) ([]byte, bool, error)
	Set(context.Context, string, []byte, time.Duration) error
	Delete(context.Context, ...string) (int64, error)
	Exists(context.Context, ...string) (int64, error)
	Close() error
}

type clientFactory interface {
	New(clientSpec) (ownedClient, error)
}

type defaultClientFactory struct{}

func (defaultClientFactory) New(spec clientSpec) (ownedClient, error) {
	var client redis.UniversalClient
	switch spec.mode {
	case runtimeconfig.RedisStandalone:
		client = redis.NewClient(standaloneOptions(spec))
	case runtimeconfig.RedisSentinel:
		client = redis.NewFailoverClient(sentinelOptions(spec))
	case runtimeconfig.RedisCluster:
		if spec.database != 0 {
			return nil, ErrInvalidProfile
		}
		client = redis.NewClusterClient(clusterOptions(spec))
	default:
		return nil, ErrInvalidProfile
	}
	return &goRedisClient{client: client, mode: spec.mode}, nil
}

func standaloneOptions(spec clientSpec) *redis.Options {
	return &redis.Options{
		Addr:                  spec.endpoints[0],
		DB:                    spec.database,
		Username:              spec.username,
		Password:              spec.password,
		DialTimeout:           spec.dialTimeout,
		ReadTimeout:           spec.readTimeout,
		WriteTimeout:          spec.writeTimeout,
		ContextTimeoutEnabled: true,
		TLSConfig:             cloneTLS(spec.tls),
	}
}

func sentinelOptions(spec clientSpec) *redis.FailoverOptions {
	return &redis.FailoverOptions{
		MasterName:    spec.masterName,
		SentinelAddrs: append([]string(nil), spec.endpoints...),
		DB:            spec.database,
		Username:      spec.username,
		Password:      spec.password,
		// Runtime v2 currently models data-plane ACL only. SentinelUsername
		// and SentinelPassword deliberately remain empty rather than guessing
		// that control-plane credentials equal data-plane credentials.
		DialTimeout:           spec.dialTimeout,
		ReadTimeout:           spec.readTimeout,
		WriteTimeout:          spec.writeTimeout,
		ContextTimeoutEnabled: true,
		TLSConfig:             cloneTLS(spec.tls),
	}
}

func clusterOptions(spec clientSpec) *redis.ClusterOptions {
	return &redis.ClusterOptions{
		Addrs:                 append([]string(nil), spec.endpoints...),
		Username:              spec.username,
		Password:              spec.password,
		DialTimeout:           spec.dialTimeout,
		ReadTimeout:           spec.readTimeout,
		WriteTimeout:          spec.writeTimeout,
		ContextTimeoutEnabled: true,
		TLSConfig:             cloneTLS(spec.tls),
	}
}

func cloneTLS(value *tls.Config) *tls.Config {
	if value == nil {
		return nil
	}
	return value.Clone()
}

type goRedisClient struct {
	client redis.UniversalClient
	mode   runtimeconfig.RedisMode
}

type atomicClient interface {
	EvalFixed(context.Context, string, []string, ...any) (any, error)
}

func (c *goRedisClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *goRedisClient) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	return value, err == nil, err
}

func (c *goRedisClient) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

func (c *goRedisClient) Delete(ctx context.Context, keys ...string) (int64, error) {
	return c.client.Del(ctx, keys...).Result()
}

func (c *goRedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.client.Exists(ctx, keys...).Result()
}

func (c *goRedisClient) EvalFixed(ctx context.Context, source string, keys []string, args ...any) (any, error) {
	return redis.NewScript(source).Run(ctx, c.client, keys, args...).Result()
}

func (c *goRedisClient) Close() error {
	return c.client.Close()
}
