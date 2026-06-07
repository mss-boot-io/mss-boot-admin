package sessioncache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func newCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	t.Cleanup(mr.Close)
	cli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return New(func() *redis.Client { return cli }), mr
}

func TestSetGet(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	entry := Entry{UserID: "u1", RoleID: "r1", ExpUnix: time.Now().Add(time.Hour).Unix()}

	assert.NoError(t, c.Set(ctx, "sid-1", entry, time.Hour))

	got, ok, err := c.Get(ctx, "sid-1")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "u1", got.UserID)
	assert.Equal(t, "r1", got.RoleID)
}

func TestGetMiss(t *testing.T) {
	c, _ := newCache(t)
	_, ok, err := c.Get(context.Background(), "nope")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestDel(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	_ = c.Set(ctx, "sid-1", Entry{UserID: "u1"}, time.Hour)
	assert.NoError(t, c.Del(ctx, "sid-1"))
	_, ok, _ := c.Get(ctx, "sid-1")
	assert.False(t, ok)
}

func TestDelByUser(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	_ = c.Set(ctx, "sid-1", Entry{UserID: "u1"}, time.Hour)
	_ = c.Set(ctx, "sid-2", Entry{UserID: "u1"}, time.Hour)
	_ = c.Set(ctx, "sid-3", Entry{UserID: "u2"}, time.Hour)

	assert.NoError(t, c.DelByUser(ctx, "u1"))

	_, ok1, _ := c.Get(ctx, "sid-1")
	_, ok2, _ := c.Get(ctx, "sid-2")
	_, ok3, _ := c.Get(ctx, "sid-3")
	assert.False(t, ok1)
	assert.False(t, ok2)
	assert.True(t, ok3)
}

func TestTouchThrottle(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	first, err := c.TryTouch(ctx, "sid-1")
	assert.NoError(t, err)
	assert.True(t, first)
	second, err := c.TryTouch(ctx, "sid-1")
	assert.NoError(t, err)
	assert.False(t, second)
}

func TestTouchThrottleLocalFallback(t *testing.T) {
	// Redis 不可用（clientFn 返回 nil）时仍应按 touchTTL 限频。
	c := New(func() *redis.Client { return nil })
	ctx := context.Background()

	first, err := c.TryTouch(ctx, "sid-1")
	assert.NoError(t, err)
	assert.True(t, first, "first call should win the slot")

	second, err := c.TryTouch(ctx, "sid-1")
	assert.NoError(t, err)
	assert.False(t, second, "second call within touchTTL should be throttled")

	third, err := c.TryTouch(ctx, "other-sid")
	assert.NoError(t, err)
	assert.True(t, third, "different sid should not be throttled")
}
