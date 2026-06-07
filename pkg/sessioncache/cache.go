package sessioncache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	sessionKeyPrefix = "mss:session:"
	userKeyPrefix    = "mss:session:user:"
	seenKeyPrefix    = "mss:session:seen:"
	touchTTL         = 60 * time.Second
)

type Entry struct {
	UserID  string `json:"userID"`
	RoleID  string `json:"roleID"`
	ExpUnix int64  `json:"exp"`
}

type Cache struct {
	clientFn func() *redis.Client
}

// New builds a Cache that resolves the Redis client lazily via fn. fn may
// return nil when Redis is not configured; in that case all cache methods
// degrade gracefully and the caller falls back to DB.
func New(fn func() *redis.Client) *Cache {
	return &Cache{clientFn: fn}
}

func (c *Cache) client() *redis.Client {
	if c == nil || c.clientFn == nil {
		return nil
	}
	return c.clientFn()
}

func sessionKey(sid string) string { return sessionKeyPrefix + sid }
func userKey(uid string) string    { return userKeyPrefix + uid }
func seenKey(sid string) string    { return seenKeyPrefix + sid }

func (c *Cache) Set(ctx context.Context, sid string, e Entry, ttl time.Duration) error {
	cli := c.client()
	if cli == nil {
		return nil
	}
	if ttl <= 0 {
		return errors.New("sessioncache: ttl must be positive")
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	pipe := cli.TxPipeline()
	pipe.Set(ctx, sessionKey(sid), payload, ttl)
	pipe.SAdd(ctx, userKey(e.UserID), sid)
	pipe.Expire(ctx, userKey(e.UserID), ttl+time.Hour)
	_, err = pipe.Exec(ctx)
	return err
}

func (c *Cache) Get(ctx context.Context, sid string) (Entry, bool, error) {
	cli := c.client()
	if cli == nil {
		return Entry{}, false, nil
	}
	raw, err := cli.Get(ctx, sessionKey(sid)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, err
	}
	var e Entry
	if err := json.Unmarshal(raw, &e); err != nil {
		return Entry{}, false, err
	}
	return e, true, nil
}

func (c *Cache) Del(ctx context.Context, sid string) error {
	cli := c.client()
	if cli == nil {
		return nil
	}
	entry, ok, err := c.Get(ctx, sid)
	if err != nil {
		return err
	}
	pipe := cli.TxPipeline()
	pipe.Del(ctx, sessionKey(sid))
	if ok {
		pipe.SRem(ctx, userKey(entry.UserID), sid)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (c *Cache) DelByUser(ctx context.Context, uid string) error {
	cli := c.client()
	if cli == nil {
		return nil
	}
	sids, err := cli.SMembers(ctx, userKey(uid)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	pipe := cli.TxPipeline()
	for _, sid := range sids {
		pipe.Del(ctx, sessionKey(sid))
	}
	pipe.Del(ctx, userKey(uid))
	_, err = pipe.Exec(ctx)
	return err
}

func (c *Cache) TryTouch(ctx context.Context, sid string) (bool, error) {
	cli := c.client()
	if cli == nil {
		return true, nil
	}
	ok, err := cli.SetNX(ctx, seenKey(sid), "1", touchTTL).Result()
	if err != nil {
		return false, fmt.Errorf("sessioncache: touch %s: %w", sid, err)
	}
	return ok, nil
}
