package pkg

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	cacheconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
)

func TestLanguageProfileCacheStoresCompleteSnapshot(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache, err := cacheconfig.NewRedis(client, nil)
	if err != nil {
		t.Fatalf("create Redis cache: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	ctx := context.Background()
	profile := LanguageProfile{
		"en-US": {"menu.welcome": "Welcome"},
		"zh-CN": {"menu.welcome": "欢迎"},
	}
	stored, err := StoreLanguageProfileCache(ctx, cache, 0, profile)
	if err != nil || !stored {
		t.Fatalf("store language profile = (%v, %v), want (true, nil)", stored, err)
	}
	loaded, generation, hit, err := LoadLanguageProfileCache(ctx, cache)
	if err != nil || !hit || generation != 0 {
		t.Fatalf("load language profile = (%v, %d, %v, %v)", loaded, generation, hit, err)
	}
	if loaded["en-US"]["menu.welcome"] != "Welcome" || loaded["zh-CN"]["menu.welcome"] != "欢迎" {
		t.Fatalf("loaded language profile = %v", loaded)
	}
}

func TestLanguageCacheGenerationRejectsLateStalePublication(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache, err := cacheconfig.NewRedis(client, nil)
	if err != nil {
		t.Fatalf("create Redis cache: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	ctx := context.Background()
	stale := LanguageProfile{"en-US": {"menu.welcome": "Old"}}
	if err := InvalidateLanguageCache(ctx, cache, "en-US"); err != nil {
		t.Fatalf("invalidate language cache: %v", err)
	}
	stored, err := StoreLanguageProfileCache(ctx, cache, 0, stale)
	if err != nil {
		t.Fatalf("store stale language profile: %v", err)
	}
	if stored {
		t.Fatal("stale generation was published")
	}
	loaded, generation, hit, err := LoadLanguageProfileCache(ctx, cache)
	if err != nil || hit || generation != 1 || loaded != nil {
		t.Fatalf("load invalidated profile = (%v, %d, %v, %v)", loaded, generation, hit, err)
	}
}

func TestLanguageProfileSnapshotHasBoundedTTL(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache, err := cacheconfig.NewRedis(client, nil)
	if err != nil {
		t.Fatalf("create Redis cache: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	stored, err := StoreLanguageProfileCache(
		context.Background(),
		cache,
		0,
		LanguageProfile{"en-US": {"menu.welcome": "Welcome"}},
	)
	if err != nil || !stored {
		t.Fatalf("store language profile = (%v, %v), want (true, nil)", stored, err)
	}
	ttl := server.TTL(languageCacheSnapshotKey(0))
	if ttl <= 0 || ttl > languageCacheSnapshotTTL {
		t.Fatalf("snapshot TTL = %s, want within (0, %s]", ttl, languageCacheSnapshotTTL)
	}
}

func TestLanguageCacheOperationsHaveShortDeadline(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "blocked.invalid:6379",
		Dialer: func(ctx context.Context, _, _ string) (net.Conn, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
		MaxRetries: -1,
	})
	cache := &cacheconfig.Redis{UniversalClient: client}
	t.Cleanup(func() { _ = cache.Close() })

	startedAt := time.Now()
	_, _, hit, err := LoadLanguageProfileCache(context.Background(), cache)
	elapsed := time.Since(startedAt)
	if err == nil || hit {
		t.Fatalf("blocked cache load = (hit %v, err %v), want miss with error", hit, err)
	}
	if elapsed > languageCacheOperationLimit+250*time.Millisecond {
		t.Fatalf("blocked cache load took %s, limit %s", elapsed, languageCacheOperationLimit)
	}
}
