package oauthstate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestMemoryStateIsHighEntropyBoundAndOneTime(t *testing.T) {
	store := New()
	record := Record{Provider: "github", Intent: IntentBinding, UserID: "user-1", CredentialFingerprint: "fingerprint"}
	state, browserNonce, issued, err := store.Issue(context.Background(), nil, record, DefaultTTL)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if len(state) < 43 || len(browserNonce) < 43 || state == browserNonce {
		t.Fatalf("expected independent 256-bit url-safe tokens, state=%q browser length=%d", state, len(browserNonce))
	}
	if issued.BrowserHash != Digest(browserNonce) || issued.BrowserHash == browserNonce {
		t.Fatal("browser nonce must only be retained as a digest")
	}

	consumed, err := store.Consume(context.Background(), nil, state)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if consumed.Provider != record.Provider || consumed.Intent != record.Intent || consumed.UserID != record.UserID || consumed.CredentialFingerprint != record.CredentialFingerprint {
		t.Fatalf("consumed record lost binding context: %#v", consumed)
	}
	if _, err = store.Consume(context.Background(), nil, state); !errors.Is(err, ErrNotFound) {
		t.Fatalf("replay error = %v, want ErrNotFound", err)
	}
}

func TestExpiredMemoryStateIsConsumed(t *testing.T) {
	store := New()
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	state, _, _, err := store.Issue(context.Background(), nil, Record{Provider: "lark", Intent: IntentLogin}, time.Minute)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err = store.Consume(context.Background(), nil, state); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired error = %v, want ErrExpired", err)
	}
	if _, err = store.Consume(context.Background(), nil, state); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired replay error = %v, want ErrNotFound", err)
	}
}

func TestRedisConsumeIsAtomic(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := New()
	state, _, _, err := store.Issue(context.Background(), client, Record{Provider: "github", Intent: IntentLogin}, DefaultTTL)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if _, err = store.Consume(context.Background(), client, state); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if _, err = store.Consume(context.Background(), client, state); !errors.Is(err, ErrNotFound) {
		t.Fatalf("redis replay error = %v, want ErrNotFound", err)
	}
}
