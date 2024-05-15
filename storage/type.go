package storage

import (
	"context"
	"time"

	"github.com/bsm/redislock"
	"gorm.io/gorm"
)

const (
	PrefixKey = "__host"
)

type AdapterCache interface {
	Name() string
	String() string
	Initialize(*gorm.DB) error
	RemoveFromTag(ctx context.Context, tag string) error
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, val interface{}, expire time.Duration) error
	Del(ctx context.Context, key string) error
	HashGet(ctx context.Context, hk, key string) (string, error)
	HashDel(ctx context.Context, hk, key string) error
	Increase(ctx context.Context, key string) error
	Decrease(ctx context.Context, key string) error
	Expire(ctx context.Context, key string, dur time.Duration) error
}

type AdapterQueue interface {
	String() string
	Append(opts ...Option) error
	Register(opts ...Option)
	Run(context.Context)
	Shutdown()
}

type Messager interface {
	SetID(string)
	SetStream(string)
	SetValues(map[string]interface{})
	GetID() string
	GetStream() string
	GetValues() map[string]interface{}
	GetPrefix() string
	SetPrefix(string)
	SetErrorCount(count int)
	GetErrorCount() int
	SetContext(ctx context.Context)
	GetContext() context.Context
}

type ConsumerFunc func(Messager) error

type AdapterLocker interface {
	String() string
	Lock(ctx context.Context, key string, ttl time.Duration, options *redislock.Options) (*redislock.Lock, error)
}
