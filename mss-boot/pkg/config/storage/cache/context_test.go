package cache

import (
	"context"
	"testing"
	"time"
)

func TestQueryCacheContextHelpersSupportTypedAndLegacyKeys(t *testing.T) {
	t.Run("typed", func(t *testing.T) {
		ctx := NewKey(NewTag(context.Background(), "typed-tag"), "typed-key")
		if got := ctx.Value("gorm:cache:key"); got != "typed-key" {
			t.Fatalf("legacy key lookup = %#v; want typed-key", got)
		}
		if got := ctx.Value("gorm:cache:tag"); got != "typed-tag" {
			t.Fatalf("legacy tag lookup = %#v; want typed-tag", got)
		}

		key, hasKey := FromKey(ctx)
		if !hasKey || key != "typed-key" {
			t.Fatalf("FromKey() = %q, %v; want typed-key, true", key, hasKey)
		}
		tag, hasTag := FromTag(ctx)
		if !hasTag || tag != "typed-tag" {
			t.Fatalf("FromTag() = %q, %v; want typed-tag, true", tag, hasTag)
		}
	})

	t.Run("legacy literals", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "gorm:cache:key", "legacy-key")
		ctx = context.WithValue(ctx, "gorm:cache:tag", "legacy-tag")

		key, hasKey := FromKey(ctx)
		if !hasKey || key != "legacy-key" {
			t.Fatalf("FromKey() = %q, %v; want legacy-key, true", key, hasKey)
		}
		tag, hasTag := FromTag(ctx)
		if !hasTag || tag != "legacy-tag" {
			t.Fatalf("FromTag() = %q, %v; want legacy-tag, true", tag, hasTag)
		}
	})
}

func TestQueryCacheExpirationContextRoundTrip(t *testing.T) {
	ctx := NewExpiration(context.Background(), 3*time.Second)

	ttl, ok := FromExpiration(ctx)
	if !ok || ttl != 3*time.Second {
		t.Fatalf("FromExpiration() = %v, %v; want 3s, true", ttl, ok)
	}
}

func TestQueryCacheBypassSupportsTypedAndLegacyKeys(t *testing.T) {
	typed := NewBypass(context.Background())
	if !FromBypass(typed) {
		t.Fatal("FromBypass(NewBypass()) = false; want true")
	}
	if got := typed.Value("gorm:cache:bypass"); got != true {
		t.Fatalf("legacy bypass lookup = %#v; want true", got)
	}

	legacy := context.WithValue(context.Background(), "gorm:cache:bypass", true)
	if !FromBypass(legacy) {
		t.Fatal("FromBypass(legacy context) = false; want true")
	}
	if FromBypass(context.Background()) {
		t.Fatal("FromBypass(background) = true; want false")
	}
}
