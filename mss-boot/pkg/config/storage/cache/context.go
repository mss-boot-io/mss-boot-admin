package cache

import (
	"context"
	"time"
)

const (
	legacyQueryCacheBypass = "gorm:cache:bypass"
	legacyQueryCacheKey    = "gorm:cache:key"
	legacyQueryCacheTag    = "gorm:cache:tag"
)

type (
	// queryCacheCtx
	queryCacheCtx struct{}

	// queryCacheKeyCtx
	queryCacheKeyCtx struct{}

	// queryCacheTagCtx
	queryCacheTagCtx struct{}

	// queryCacheBypassCtx
	queryCacheBypassCtx struct{}
)

// NewBypass marks a query so it always reads from the database. The legacy
// literal is retained while existing callers migrate to the typed helper.
func NewBypass(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, legacyQueryCacheBypass, true)
	return context.WithValue(ctx, queryCacheBypassCtx{}, true)
}

// FromBypass reports whether the query cache must be bypassed.
func FromBypass(ctx context.Context) bool {
	if value, ok := ctx.Value(queryCacheBypassCtx{}).(bool); ok {
		return value
	}
	value, _ := ctx.Value(legacyQueryCacheBypass).(bool)
	return value
}

// NewKey creates a new context with the given key
func NewKey(ctx context.Context, key string) context.Context {
	ctx = context.WithValue(ctx, legacyQueryCacheKey, key)
	return context.WithValue(ctx, queryCacheKeyCtx{}, key)
}

// NewTag creates a new context with the given tag
func NewTag(ctx context.Context, key string) context.Context {
	ctx = context.WithValue(ctx, legacyQueryCacheTag, key)
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
	if value, ok := ctx.Value(queryCacheKeyCtx{}).(string); ok {
		return value, true
	}
	if value, ok := ctx.Value(legacyQueryCacheKey).(string); ok {
		return value, true
	}
	return "", false
}

// FromTag returns the tag from the context
func FromTag(ctx context.Context) (string, bool) {
	if value, ok := ctx.Value(queryCacheTagCtx{}).(string); ok {
		return value, true
	}
	if value, ok := ctx.Value(legacyQueryCacheTag).(string); ok {
		return value, true
	}
	return "", false
}
