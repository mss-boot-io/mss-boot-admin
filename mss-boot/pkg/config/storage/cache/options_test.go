package cache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryCacheAllowlistIsFailClosedByDefault(t *testing.T) {
	defaults := DefaultOptions()
	require.False(t, defaults.HasKey("mss_boot_users"))

	explicit := DefaultOptions()
	WithQueryCacheKeys("mss_boot_options")(&explicit)
	require.True(t, explicit.HasKey("mss_boot_options"))
	require.False(t, explicit.HasKey("mss_boot_users"))

	wildcard := DefaultOptions()
	WithQueryCacheKeys("*")(&wildcard)
	require.True(t, wildcard.HasKey("mss_boot_users"))
}
