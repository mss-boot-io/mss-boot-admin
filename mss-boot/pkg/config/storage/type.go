package storage

import (
	"context"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	PrefixKey = "__host"
)

type AdapterCache interface {
	redis.UniversalClient
	Name() string
	String() string
	Initialize(*gorm.DB) error
	RemoveFromTag(ctx context.Context, tag string) error
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
	SetValues(map[string]any)
	GetID() string
	GetStream() string
	GetValues() map[string]any
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
