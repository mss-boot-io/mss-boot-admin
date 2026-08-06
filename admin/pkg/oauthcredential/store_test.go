package oauthcredential

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const (
	testApplicationSecret = "test-only-application-secret"
	testAccessToken       = "provider-access-token-must-stay-secret"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := New([]byte(testApplicationSecret))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return store
}

func integrationRecord() Record {
	return Record{
		Provider:              "github",
		Intent:                IntentIntegration,
		UserID:                "user-1",
		CredentialFingerprint: "interactive-session-fingerprint",
		AccessToken:           testAccessToken,
	}
}

func TestNewRequiresApplicationSecret(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) error = nil, want validation error")
	}
	if _, err := New([]byte{}); err == nil {
		t.Fatal("New(empty) error = nil, want validation error")
	}
}

func TestMemoryIssueLookupAndDelete(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	handle, issued, err := store.Issue(context.Background(), nil, integrationRecord(), DefaultTTL)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if len(handle) != 43 {
		t.Fatalf("handle length = %d, want 43-character encoding of 256 bits", len(handle))
	}
	if strings.Contains(handle, testAccessToken) {
		t.Fatal("opaque handle contains provider access token")
	}
	if want := now.Add(DefaultTTL); !issued.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", issued.ExpiresAt, want)
	}

	key := credentialKey(handle)
	entry, ok := store.local[key]
	if !ok || len(store.local) != 1 {
		t.Fatalf("local entries = %#v, want one digest-keyed entry", store.local)
	}
	if strings.Contains(key, handle) {
		t.Fatal("cache key contains raw handle")
	}
	if bytes.Contains(entry.payload, []byte(testAccessToken)) {
		t.Fatal("local fallback contains plaintext provider token")
	}

	for attempt := 0; attempt < 2; attempt++ {
		got, lookupErr := store.Lookup(context.Background(), nil, handle)
		if lookupErr != nil {
			t.Fatalf("Lookup() attempt %d error = %v", attempt+1, lookupErr)
		}
		if got != issued {
			t.Fatalf("Lookup() attempt %d = %#v, want %#v", attempt+1, got, issued)
		}
	}

	if err = store.Delete(context.Background(), nil, handle); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err = store.Delete(context.Background(), nil, handle); err != nil {
		t.Fatalf("idempotent Delete() error = %v", err)
	}
	if _, err = store.Lookup(context.Background(), nil, handle); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Lookup() after Delete error = %v, want ErrNotFound", err)
	}
}

func TestIssueRequiresCompleteIntegrationBindingAndPositiveTTL(t *testing.T) {
	store := newTestStore(t)
	tests := []struct {
		name   string
		mutate func(*Record)
		ttl    time.Duration
	}{
		{name: "provider", mutate: func(record *Record) { record.Provider = "" }, ttl: time.Minute},
		{name: "intent", mutate: func(record *Record) { record.Intent = "login" }, ttl: time.Minute},
		{name: "user", mutate: func(record *Record) { record.UserID = "" }, ttl: time.Minute},
		{name: "fingerprint", mutate: func(record *Record) { record.CredentialFingerprint = "" }, ttl: time.Minute},
		{name: "token", mutate: func(record *Record) { record.AccessToken = "" }, ttl: time.Minute},
		{name: "zero ttl", mutate: func(*Record) {}, ttl: 0},
		{name: "negative ttl", mutate: func(*Record) {}, ttl: -time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := integrationRecord()
			test.mutate(&record)
			if handle, _, err := store.Issue(context.Background(), nil, record, test.ttl); err == nil || handle != "" {
				t.Fatalf("Issue() = (%q, %v), want empty handle and validation error", handle, err)
			}
		})
	}
}

func TestIssueCapsTTLAndExpiredLookupFailsClosed(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	record := integrationRecord()
	record.ExpiresAt = now.Add(time.Minute)

	handle, issued, err := store.Issue(context.Background(), nil, record, DefaultTTL)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if !issued.ExpiresAt.Equal(record.ExpiresAt) {
		t.Fatalf("ExpiresAt = %v, want capped expiry %v", issued.ExpiresAt, record.ExpiresAt)
	}
	if got := store.local[credentialKey(handle)].expiresAt; !got.Equal(record.ExpiresAt) {
		t.Fatalf("local expiry = %v, want %v", got, record.ExpiresAt)
	}

	now = now.Add(time.Minute)
	if _, err = store.Lookup(context.Background(), nil, handle); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired Lookup() error = %v, want ErrExpired", err)
	}
	if strings.Contains(err.Error(), handle) || strings.Contains(err.Error(), testAccessToken) {
		t.Fatalf("expired error leaked a secret: %v", err)
	}
	if _, err = store.Lookup(context.Background(), nil, handle); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second expired Lookup() error = %v, want ErrNotFound", err)
	}

	expired := integrationRecord()
	expired.ExpiresAt = now
	if _, _, err = store.Issue(context.Background(), nil, expired, DefaultTTL); err == nil {
		t.Fatal("Issue() accepted an already expired record")
	}
}

func TestRedisUsesDigestKeyAndEncryptedReusablePayload(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newTestStore(t)

	handle, issued, err := store.Issue(context.Background(), client, integrationRecord(), DefaultTTL)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	keys := server.Keys()
	if len(keys) != 1 || keys[0] != credentialKey(handle) {
		t.Fatalf("Redis keys = %#v, want only digest-derived key %q", keys, credentialKey(handle))
	}
	if strings.Contains(keys[0], handle) {
		t.Fatal("Redis key contains raw handle")
	}
	raw, err := server.Get(keys[0])
	if err != nil {
		t.Fatalf("read Redis payload: %v", err)
	}
	if strings.Contains(raw, testAccessToken) || strings.Contains(raw, issued.UserID) {
		t.Fatal("Redis payload contains plaintext credential record")
	}
	if ttl := server.TTL(keys[0]); ttl <= 0 || ttl > DefaultTTL {
		t.Fatalf("Redis TTL = %v, want (0, %v]", ttl, DefaultTTL)
	}

	for attempt := 0; attempt < 2; attempt++ {
		got, lookupErr := store.Lookup(context.Background(), client, handle)
		if lookupErr != nil {
			t.Fatalf("Lookup() attempt %d error = %v", attempt+1, lookupErr)
		}
		if got != issued {
			t.Fatalf("Lookup() attempt %d = %#v, want %#v", attempt+1, got, issued)
		}
	}
	if err = store.Delete(context.Background(), client, handle); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err = store.Lookup(context.Background(), client, handle); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Lookup() after Delete error = %v, want ErrNotFound", err)
	}
}

func TestConsumeIsExactlyOnceAcrossConcurrentRequests(t *testing.T) {
	tests := []struct {
		name   string
		client func(*testing.T) redis.UniversalClient
	}{
		{name: "local", client: func(*testing.T) redis.UniversalClient { return nil }},
		{name: "redis", client: func(t *testing.T) redis.UniversalClient {
			server := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: server.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			return client
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newTestStore(t)
			client := test.client(t)
			handle, issued, err := store.Issue(context.Background(), client, integrationRecord(), DefaultTTL)
			if err != nil {
				t.Fatalf("Issue() error = %v", err)
			}

			const contenders = 32
			var successes atomic.Int32
			var notFound atomic.Int32
			var wait sync.WaitGroup
			wait.Add(contenders)
			for range contenders {
				go func() {
					defer wait.Done()
					record, consumeErr := store.Consume(context.Background(), client, handle)
					switch {
					case consumeErr == nil:
						if record != issued {
							t.Errorf("Consume() record = %#v, want %#v", record, issued)
						}
						successes.Add(1)
					case errors.Is(consumeErr, ErrNotFound):
						notFound.Add(1)
					default:
						t.Errorf("Consume() error = %v, want nil or ErrNotFound", consumeErr)
					}
				}()
			}
			wait.Wait()
			if successes.Load() != 1 || notFound.Load() != contenders-1 {
				t.Fatalf("consume outcomes: success=%d not-found=%d, want 1/%d",
					successes.Load(), notFound.Load(), contenders-1)
			}
		})
	}
}

func TestConsumeBurnsExpiredCredential(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	handle, _, err := store.Issue(context.Background(), nil, integrationRecord(), time.Minute)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	now = now.Add(time.Minute)
	if _, err = store.Consume(context.Background(), nil, handle); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired Consume() error = %v, want ErrExpired", err)
	}
	if _, err = store.Consume(context.Background(), nil, handle); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second expired Consume() error = %v, want ErrNotFound", err)
	}
}

func TestWrongEncryptionKeyAndTamperingFailClosedWithoutLeaks(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	issuer := newTestStore(t)
	handle, _, err := issuer.Issue(context.Background(), client, integrationRecord(), DefaultTTL)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	other, err := New([]byte("different-application-secret"))
	if err != nil {
		t.Fatalf("New(other) error = %v", err)
	}
	if _, err = other.Lookup(context.Background(), client, handle); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Lookup() with wrong key error = %v, want ErrInvalid", err)
	}
	assertNoSecretInError(t, err, handle, testAccessToken)

	key := credentialKey(handle)
	payload, getErr := server.Get(key)
	if getErr != nil {
		t.Fatalf("read Redis payload: %v", getErr)
	}
	tampered := []byte(payload)
	tampered[len(tampered)-1] ^= 0xff
	server.Set(key, string(tampered))
	if _, err = issuer.Lookup(context.Background(), client, handle); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Lookup() after tampering error = %v, want ErrInvalid", err)
	}
	assertNoSecretInError(t, err, handle, testAccessToken)
}

func TestLocalFallbackIsProcessScoped(t *testing.T) {
	issuer := newTestStore(t)
	handle, _, err := issuer.Issue(context.Background(), nil, integrationRecord(), DefaultTTL)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	other := newTestStore(t)
	if _, err = other.Lookup(context.Background(), nil, handle); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-instance local Lookup() error = %v, want ErrNotFound", err)
	}
	assertNoSecretInError(t, err, handle, testAccessToken)
}

func TestLookupRejectsEmptyHandle(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.Lookup(context.Background(), nil, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Lookup(empty) error = %v, want ErrNotFound", err)
	}
	if err := store.Delete(context.Background(), nil, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(empty) error = %v, want ErrNotFound", err)
	}
	if _, err := store.Consume(context.Background(), nil, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Consume(empty) error = %v, want ErrNotFound", err)
	}
}

func assertNoSecretInError(t *testing.T, err error, secrets ...string) {
	t.Helper()
	if err == nil {
		return
	}
	for _, secret := range secrets {
		if secret != "" && strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked secret %q: %v", secret, err)
		}
	}
}
