package cache

import (
	"context"
	"time"
)

type (
	// queryCacheCtx
	queryCacheCtx struct{}

	// queryCacheKeyCtx
	queryCacheKeyCtx struct{}

	// queryCacheTagCtx
	queryCacheTagCtx struct{}
)

// NewKey creates a new context with the given key
func NewKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, queryCacheKeyCtx{}, key)
}

// NewTag creates a new context with the given tag
func NewTag(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, queryCacheTagCtx{}, key)
}

// NewExpiration creates a new context with the given expiration time
func NewExpiration(ctx context.Context, ttl time.Duration) context.Context {
	return context.WithValue(ctx, queryCacheCtx{}, ttl)
}

// FromExpiration returns the expiration time from the context
func FromExpiration(ctx context.Context) (time.Duration, bool) {
	value := ctx.Value(queryCacheCtx{})

	if value != nil {
		if t, ok := value.(time.Duration); ok {
			return t, true
		}
	}

	return 0, false
}

// FromKey returns the key from the context
func FromKey(ctx context.Context) (string, bool) {
	value := ctx.Value("gorm:cache:key")

	if value != nil {
		if t, ok := value.(string); ok {
			return t, true
		}

	}

	return "", false
}

// FromTag returns the tag from the context
func FromTag(ctx context.Context) (string, bool) {
	value := ctx.Value("gorm:cache:tag")

	if value != nil {
		if t, ok := value.(string); ok {
			return t, true
		}

	}

	return "", false
}
