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

// NewKey
// @param ctx
// @param key
// @date 2022-07-02 08:11:44
func NewKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, queryCacheKeyCtx{}, key)
}

// NewTag
// @param ctx
// @param key
// @date 2022-07-02 08:11:43
func NewTag(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, queryCacheTagCtx{}, key)
}

// NewExpiration
// @param ctx
// @param ttl
// @date 2022-07-02 08:11:41
func NewExpiration(ctx context.Context, ttl time.Duration) context.Context {
	return context.WithValue(ctx, queryCacheCtx{}, ttl)
}

// FromExpiration
// @param ctx
// @date 2022-07-02 08:11:40
func FromExpiration(ctx context.Context) (time.Duration, bool) {
	value := ctx.Value(queryCacheCtx{})

	if value != nil {
		if t, ok := value.(time.Duration); ok {
			return t, true
		}
	}

	return 0, false
}

// FromKey
// @param ctx
// @date 2022-07-02 08:11:39
func FromKey(ctx context.Context) (string, bool) {
	value := ctx.Value("gorm:cache:key")

	if value != nil {
		if t, ok := value.(string); ok {
			return t, true
		}

	}

	return "", false
}

// FromTag
// @param ctx
// @date 2022-07-02 08:11:37
func FromTag(ctx context.Context) (string, bool) {
	value := ctx.Value("gorm:cache:tag")

	if value != nil {
		if t, ok := value.(string); ok {
			return t, true
		}

	}

	return "", false
}
