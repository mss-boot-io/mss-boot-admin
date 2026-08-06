package oauthcredential

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
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
	// DefaultTTL limits integration credentials to the interactive generator
	// authorization window. Callers may request a shorter lifetime.
	DefaultTTL = 15 * time.Minute

	handleBytes         = 32
	keyPrefix           = "mss:oauth:credential:"
	keyDerivationDomain = "mss-boot-admin/oauthcredential/aes-gcm/v1"
	envelopeAADDomain   = "mss-boot-admin/oauthcredential/envelope/v1:"
	envelopeVersion     = byte(1)
)

var (
	ErrNotFound = errors.New("oauth credential not found")
	ErrExpired  = errors.New("oauth credential expired")
	ErrInvalid  = errors.New("oauth credential invalid")
)

type Intent string

const IntentIntegration Intent = "integration"

func (i Intent) Valid() bool {
	return i == IntentIntegration
}

// Record is encrypted before it is persisted. CredentialFingerprint binds the
// provider credential to the interactive Admin session without retaining that
// session credential itself.
type Record struct {
	Provider              string    `json:"provider"`
	Intent                Intent    `json:"intent"`
	UserID                string    `json:"userID"`
	CredentialFingerprint string    `json:"credentialFingerprint"`
	AccessToken           string    `json:"accessToken"`
	ExpiresAt             time.Time `json:"expiresAt"`
}

type localEntry struct {
	payload   []byte
	expiresAt time.Time
}

// Store writes encrypted credentials to Redis when a client is supplied. Its
// process-local fallback preserves single-instance development behavior. In a
// multi-replica deployment without Redis, a lookup on a replica other than the
// issuer fails closed with ErrNotFound.
type Store struct {
	mu     sync.Mutex
	local  map[string]localEntry
	now    func() time.Time
	random io.Reader
	aead   cipher.AEAD
}

// New derives an AES-256-GCM key from the caller's application secret. The
// domain-separated derivation ensures the resulting key is independent from
// other uses of the same application secret.
func New(applicationSecret []byte) (*Store, error) {
	if len(applicationSecret) == 0 {
		return nil, errors.New("oauth credential application secret is required")
	}
	mac := hmac.New(sha256.New, applicationSecret)
	_, _ = mac.Write([]byte(keyDerivationDomain))
	key := mac.Sum(nil)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("initialize oauth credential encryption")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("initialize oauth credential encryption")
	}
	return &Store{
		local:  make(map[string]localEntry),
		now:    time.Now,
		random: rand.Reader,
		aead:   aead,
	}, nil
}

// Issue stores an integration credential and returns a 256-bit opaque handle.
// A non-zero Record.ExpiresAt caps the requested ttl; Store always writes the
// final effective expiry into the returned and encrypted records.
func (s *Store) Issue(
	ctx context.Context,
	client redis.UniversalClient,
	record Record,
	ttl time.Duration,
) (handle string, issued Record, err error) {
	if s == nil || s.aead == nil {
		return "", Record{}, errors.New("oauth credential store is nil")
	}
	if ttl <= 0 {
		return "", Record{}, errors.New("oauth credential ttl must be positive")
	}
	if err := validateRecord(record, false); err != nil {
		return "", Record{}, err
	}

	now := s.now().UTC()
	expiresAt := now.Add(ttl)
	if !record.ExpiresAt.IsZero() {
		requestedExpiry := record.ExpiresAt.UTC()
		if !now.Before(requestedExpiry) {
			return "", Record{}, errors.New("oauth credential expiry must be in the future")
		}
		if requestedExpiry.Before(expiresAt) {
			expiresAt = requestedExpiry
		}
	}
	record.ExpiresAt = expiresAt
	effectiveTTL := expiresAt.Sub(now)
	if effectiveTTL <= 0 {
		return "", Record{}, errors.New("oauth credential ttl must be positive")
	}

	plaintext, err := json.Marshal(record)
	if err != nil {
		return "", Record{}, errors.New("encode oauth credential")
	}
	for attempts := 0; attempts < 3; attempts++ {
		handle, err = randomHandle(s.random)
		if err != nil {
			return "", Record{}, fmt.Errorf("generate oauth credential handle: %w", err)
		}
		key := credentialKey(handle)
		payload, sealErr := s.seal(key, plaintext)
		if sealErr != nil {
			return "", Record{}, sealErr
		}
		created, storeErr := s.setNX(ctx, client, key, payload, effectiveTTL, expiresAt)
		if storeErr != nil {
			return "", Record{}, storeErr
		}
		if created {
			return handle, record, nil
		}
	}
	return "", Record{}, errors.New("generate unique oauth credential handle")
}

// Lookup returns a credential without consuming it so a generator can perform
// multiple provider operations during the same short authorization window.
// Callers must validate provider, user, session fingerprint, and intent before
// using AccessToken.
func (s *Store) Lookup(
	ctx context.Context,
	client redis.UniversalClient,
	handle string,
) (Record, error) {
	if s == nil || s.aead == nil || handle == "" {
		return Record{}, ErrNotFound
	}
	key := credentialKey(handle)
	payload, err := s.get(ctx, client, key)
	if err != nil {
		return Record{}, err
	}
	if len(payload) == 0 {
		return Record{}, ErrNotFound
	}
	plaintext, err := s.open(key, payload)
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err = json.Unmarshal(plaintext, &record); err != nil {
		return Record{}, ErrInvalid
	}
	if err = validateRecord(record, true); err != nil {
		return Record{}, ErrInvalid
	}
	if !s.now().UTC().Before(record.ExpiresAt.UTC()) {
		_ = s.delete(ctx, client, key)
		return Record{}, ErrExpired
	}
	return record, nil
}

// Consume atomically claims and removes a credential before a state-changing
// generator operation starts. This prevents two concurrent requests from
// replaying one handle and both reaching remote side effects.
func (s *Store) Consume(
	ctx context.Context,
	client redis.UniversalClient,
	handle string,
) (Record, error) {
	if s == nil || s.aead == nil || handle == "" {
		return Record{}, ErrNotFound
	}
	key := credentialKey(handle)
	payload, err := s.take(ctx, client, key)
	if err != nil {
		return Record{}, err
	}
	if len(payload) == 0 {
		return Record{}, ErrNotFound
	}
	plaintext, err := s.open(key, payload)
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err = json.Unmarshal(plaintext, &record); err != nil {
		return Record{}, ErrInvalid
	}
	if err = validateRecord(record, true); err != nil {
		return Record{}, ErrInvalid
	}
	if !s.now().UTC().Before(record.ExpiresAt.UTC()) {
		return Record{}, ErrExpired
	}
	return record, nil
}

// Delete revokes a handle. Deletion is idempotent for handles that have
// already expired or were previously deleted.
func (s *Store) Delete(
	ctx context.Context,
	client redis.UniversalClient,
	handle string,
) error {
	if s == nil || s.aead == nil || handle == "" {
		return ErrNotFound
	}
	return s.delete(ctx, client, credentialKey(handle))
}

func validateRecord(record Record, requireExpiry bool) error {
	if record.Provider == "" || !record.Intent.Valid() || record.UserID == "" ||
		record.CredentialFingerprint == "" || record.AccessToken == "" {
		return errors.New("oauth credential integration binding is incomplete")
	}
	if requireExpiry && record.ExpiresAt.IsZero() {
		return errors.New("oauth credential expiry is required")
	}
	return nil
}

func (s *Store) seal(key string, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(s.random, nonce); err != nil {
		return nil, fmt.Errorf("encrypt oauth credential: %w", err)
	}
	payload := make([]byte, 1, 1+len(nonce)+len(plaintext)+s.aead.Overhead())
	payload[0] = envelopeVersion
	payload = append(payload, nonce...)
	return s.aead.Seal(payload, nonce, plaintext, envelopeAAD(key)), nil
}

func (s *Store) open(key string, payload []byte) ([]byte, error) {
	nonceSize := s.aead.NonceSize()
	if len(payload) < 1+nonceSize+s.aead.Overhead() || payload[0] != envelopeVersion {
		return nil, ErrInvalid
	}
	nonce := payload[1 : 1+nonceSize]
	ciphertext := payload[1+nonceSize:]
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, envelopeAAD(key))
	if err != nil {
		return nil, ErrInvalid
	}
	return plaintext, nil
}

func envelopeAAD(key string) []byte {
	return []byte(envelopeAADDomain + key)
}

func (s *Store) setNX(
	ctx context.Context,
	client redis.UniversalClient,
	key string,
	payload []byte,
	ttl time.Duration,
	expiresAt time.Time,
) (bool, error) {
	if client != nil {
		created, err := client.SetNX(ctx, key, payload, ttl).Result()
		if err != nil {
			return false, fmt.Errorf("store oauth credential: %w", err)
		}
		return created, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepExpiredLocked()
	if _, exists := s.local[key]; exists {
		return false, nil
	}
	s.local[key] = localEntry{
		payload:   append([]byte(nil), payload...),
		expiresAt: expiresAt,
	}
	return true, nil
}

func (s *Store) get(
	ctx context.Context,
	client redis.UniversalClient,
	key string,
) ([]byte, error) {
	if client != nil {
		payload, err := client.Get(ctx, key).Bytes()
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("lookup oauth credential: %w", err)
		}
		return payload, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.local[key]
	if !exists {
		return nil, nil
	}
	if !s.now().Before(entry.expiresAt) {
		delete(s.local, key)
	}
	return append([]byte(nil), entry.payload...), nil
}

func (s *Store) take(
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
			return nil, fmt.Errorf("consume oauth credential: %w", err)
		}
		switch typed := value.(type) {
		case string:
			return []byte(typed), nil
		case []byte:
			return append([]byte(nil), typed...), nil
		default:
			return nil, fmt.Errorf("consume oauth credential: unexpected cache value %T", value)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.local[key]
	if !exists {
		return nil, nil
	}
	delete(s.local, key)
	return append([]byte(nil), entry.payload...), nil
}

func (s *Store) delete(
	ctx context.Context,
	client redis.UniversalClient,
	key string,
) error {
	if client != nil {
		if err := client.Del(ctx, key).Err(); err != nil {
			return fmt.Errorf("delete oauth credential: %w", err)
		}
		return nil
	}
	s.mu.Lock()
	delete(s.local, key)
	s.mu.Unlock()
	return nil
}

func (s *Store) sweepExpiredLocked() {
	now := s.now()
	for key, entry := range s.local {
		if !now.Before(entry.expiresAt) {
			delete(s.local, key)
		}
	}
}

func randomHandle(reader io.Reader) (string, error) {
	buffer := make([]byte, handleBytes)
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func credentialKey(handle string) string {
	return keyPrefix + digest(handle)
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
