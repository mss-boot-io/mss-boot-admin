package oauthstate

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// DefaultTTL keeps authorization attempts short-lived while leaving enough
	// time for a user to complete the provider consent screen.
	DefaultTTL = 5 * time.Minute
	stateBytes = 32
	keyPrefix  = "mss:oauth:state:"
)

var (
	ErrNotFound = errors.New("oauth state not found")
	ErrExpired  = errors.New("oauth state expired")
)

type Intent string

const (
	IntentLogin       Intent = "login"
	IntentBinding     Intent = "binding"
	IntentIntegration Intent = "integration"
)

func (i Intent) Valid() bool {
	return i == IntentLogin || i == IntentBinding || i == IntentIntegration
}

// Record is stored only on the server. CredentialFingerprint and BrowserHash
// are one-way digests; raw login credentials and the HttpOnly browser nonce are
// never persisted or logged.
type Record struct {
	Provider              string    `json:"provider"`
	Intent                Intent    `json:"intent"`
	UserID                string    `json:"userID,omitempty"`
	CredentialFingerprint string    `json:"credentialFingerprint,omitempty"`
	BrowserHash           string    `json:"browserHash"`
	IssuedAt              time.Time `json:"issuedAt"`
	ExpiresAt             time.Time `json:"expiresAt"`
}

type localEntry struct {
	payload   []byte
	expiresAt time.Time
}

// Store uses the repository cache when one is configured. The mutex-protected
// local fallback keeps single-process development functional. A multi-replica
// deployment must configure the shared Redis cache: otherwise a callback that
// lands on a different replica fails closed because that replica cannot find
// the issuing process's state.
type Store struct {
	mu     sync.Mutex
	local  map[string]localEntry
	now    func() time.Time
	random io.Reader
}

func New() *Store {
	return &Store{
		local:  make(map[string]localEntry),
		now:    time.Now,
		random: rand.Reader,
	}
}

// Issue creates independent, 256-bit state and browser-binding nonces. Only a
// digest of the browser nonce is retained in the server-side record.
func (s *Store) Issue(
	ctx context.Context,
	client redis.UniversalClient,
	record Record,
	ttl time.Duration,
) (state string, browserNonce string, issued Record, err error) {
	if ttl <= 0 {
		return "", "", Record{}, errors.New("oauth state ttl must be positive")
	}
	if !record.Intent.Valid() || record.Provider == "" {
		return "", "", Record{}, errors.New("oauth state provider and intent are required")
	}
	if s == nil {
		return "", "", Record{}, errors.New("oauth state store is nil")
	}

	now := s.now().UTC()
	record.IssuedAt = now
	record.ExpiresAt = now.Add(ttl)

	for attempts := 0; attempts < 3; attempts++ {
		state, err = randomToken(s.random)
		if err != nil {
			return "", "", Record{}, fmt.Errorf("generate oauth state: %w", err)
		}
		browserNonce, err = randomToken(s.random)
		if err != nil {
			return "", "", Record{}, fmt.Errorf("generate oauth browser nonce: %w", err)
		}
		record.BrowserHash = Digest(browserNonce)
		payload, marshalErr := json.Marshal(record)
		if marshalErr != nil {
			return "", "", Record{}, fmt.Errorf("encode oauth state: %w", marshalErr)
		}
		created, storeErr := s.setNX(ctx, client, stateKey(state), payload, ttl)
		if storeErr != nil {
			return "", "", Record{}, storeErr
		}
		if created {
			return state, browserNonce, record, nil
		}
	}
	return "", "", Record{}, errors.New("generate unique oauth state")
}

// Consume atomically removes a state before returning its record. Validation
// failures after this call intentionally burn the state, preventing retries,
// provider swapping, and replay.
func (s *Store) Consume(
	ctx context.Context,
	client redis.UniversalClient,
	state string,
) (Record, error) {
	if s == nil || state == "" {
		return Record{}, ErrNotFound
	}
	payload, err := s.getAndDelete(ctx, client, stateKey(state))
	if err != nil {
		return Record{}, err
	}
	if len(payload) == 0 {
		return Record{}, ErrNotFound
	}
	var record Record
	if err = json.Unmarshal(payload, &record); err != nil {
		return Record{}, fmt.Errorf("decode oauth state: %w", err)
	}
	if !s.now().Before(record.ExpiresAt) {
		return Record{}, ErrExpired
	}
	return record, nil
}

func (s *Store) setNX(
	ctx context.Context,
	client redis.UniversalClient,
	key string,
	payload []byte,
	ttl time.Duration,
) (bool, error) {
	if client != nil {
		created, err := client.SetNX(ctx, key, payload, ttl).Result()
		if err != nil {
			return false, fmt.Errorf("store oauth state: %w", err)
		}
		return created, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepExpiredLocked()
	if _, exists := s.local[key]; exists {
		return false, nil
	}
	s.local[key] = localEntry{payload: append([]byte(nil), payload...), expiresAt: s.now().Add(ttl)}
	return true, nil
}

func (s *Store) getAndDelete(
	ctx context.Context,
	client redis.UniversalClient,
	key string,
) ([]byte, error) {
	if client != nil {
		const script = `local value = redis.call("GET", KEYS[1]); if value then redis.call("DEL", KEYS[1]); end; return value`
		value, err := client.Eval(ctx, script, []string{key}).Result()
		if errors.Is(err, redis.Nil) || value == nil {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("consume oauth state: %w", err)
		}
		switch typed := value.(type) {
		case string:
			return []byte(typed), nil
		case []byte:
			return typed, nil
		default:
			return nil, fmt.Errorf("consume oauth state: unexpected cache value %T", value)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.local[key]
	if !exists {
		return nil, nil
	}
	delete(s.local, key)
	if !s.now().Before(entry.expiresAt) {
		// Return the payload so Consume can distinguish an expired state from an
		// unknown or replayed state while still guaranteeing one-time use.
		return entry.payload, nil
	}
	return append([]byte(nil), entry.payload...), nil
}

func (s *Store) sweepExpiredLocked() {
	now := s.now()
	for key, entry := range s.local {
		if !now.Before(entry.expiresAt) {
			delete(s.local, key)
		}
	}
}

func randomToken(reader io.Reader) (string, error) {
	buffer := make([]byte, stateBytes)
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func stateKey(state string) string {
	return keyPrefix + Digest(state)
}

// Digest returns a stable one-way representation suitable for cache keys,
// cookie names, and constant-time comparisons.
func Digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
