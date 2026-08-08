package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
)

func TestCacheInitInvalidRedisConfigurationDegradesWithoutRegistering(t *testing.T) {
	cacheConfig := Cache{
		QueryCache:         true,
		QueryCacheDuration: time.Minute,
		Redis: &storage.RedisConnectOptions{
			TLS: &storage.TLS{Ca: "/path/that/does/not/exist/redis-ca.pem"},
		},
	}

	setCalled := false
	queryCacheCalled := false
	require.NotPanics(t, func() {
		cacheConfig.Init(
			func(storage.AdapterCache) { setCalled = true },
			func(*gorm.DB, time.Duration) { queryCacheCalled = true },
		)
	})
	require.False(t, setCalled)
	require.False(t, queryCacheCalled)
}
