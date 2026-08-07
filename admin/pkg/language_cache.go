package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
	"github.com/redis/go-redis/v9"
)

const (
	LanguageCacheGenerationKey  = "language:profile:generation"
	languageCacheSnapshotPrefix = "language:profile:snapshot:"
	languageCacheSnapshotTTL    = 5 * time.Minute
	languageCacheOperationLimit = 500 * time.Millisecond
)

type LanguageProfile map[string]map[string]string

func languageCacheGeneration(ctx context.Context, cache storage.AdapterCache) (int64, error) {
	value, err := cache.Get(ctx, LanguageCacheGenerationKey).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}
	generation, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse language cache generation: %w", err)
	}
	return generation, nil
}

func languageCacheSnapshotKey(generation int64) string {
	return languageCacheSnapshotPrefix + strconv.FormatInt(generation, 10)
}

// LoadLanguageProfileCache reads one complete language snapshot. A generation
// check after the payload read prevents an invalidation racing with the read
// from publishing a partial or indefinitely stale cache hit.
func LoadLanguageProfileCache(
	ctx context.Context,
	cache storage.AdapterCache,
) (LanguageProfile, int64, bool, error) {
	if cache == nil {
		return nil, 0, false, nil
	}
	ctx, cancel := boundedLanguageCacheContext(ctx)
	defer cancel()
	generation, err := languageCacheGeneration(ctx, cache)
	if err != nil {
		return nil, 0, false, err
	}
	key := languageCacheSnapshotKey(generation)
	payload, err := cache.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, generation, false, nil
		}
		return nil, generation, false, err
	}
	profile := make(LanguageProfile)
	if err := json.Unmarshal(payload, &profile); err != nil {
		_ = cache.Del(ctx, key).Err()
		return nil, generation, false, nil
	}
	currentGeneration, err := languageCacheGeneration(ctx, cache)
	if err != nil {
		return nil, generation, false, err
	}
	if currentGeneration != generation {
		return nil, currentGeneration, false, nil
	}
	return profile, generation, true, nil
}

// StoreLanguageProfileCache publishes the whole language map under the
// generation observed before the database read. If an invalidation happened
// meanwhile, the stale snapshot is discarded rather than becoming visible.
func StoreLanguageProfileCache(
	ctx context.Context,
	cache storage.AdapterCache,
	generation int64,
	profile LanguageProfile,
) (bool, error) {
	if cache == nil {
		return false, nil
	}
	ctx, cancel := boundedLanguageCacheContext(ctx)
	defer cancel()
	currentGeneration, err := languageCacheGeneration(ctx, cache)
	if err != nil {
		return false, err
	}
	if currentGeneration != generation {
		return false, nil
	}
	payload, err := json.Marshal(profile)
	if err != nil {
		return false, fmt.Errorf("marshal language profile cache: %w", err)
	}
	if err := cache.Set(ctx, languageCacheSnapshotKey(generation), payload, languageCacheSnapshotTTL).Err(); err != nil {
		return false, err
	}
	return true, nil
}

// InvalidateLanguageCache atomically advances the cache generation. Old
// snapshots are ignored immediately and expire independently, so invalidation
// is safe across processes and Redis Cluster hash slots.
func InvalidateLanguageCache(ctx context.Context, cache storage.AdapterCache, names ...string) error {
	if cache == nil {
		return nil
	}
	ctx, cancel := boundedLanguageCacheContext(ctx)
	defer cancel()
	_, err := cache.Incr(ctx, LanguageCacheGenerationKey).Result()
	return err
}

func boundedLanguageCacheContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, languageCacheOperationLimit)
}
