// Package wsticket owns short-lived, single-use WebSocket authentication
// tickets. Raw tickets are never persisted: cache keys contain only SHA-256
// digests and records contain the already-authenticated principal snapshot.
package wsticket

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
	"strings"
	"sync"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/browsersecurity"
	"github.com/redis/go-redis/v9"
)

const (
	ticketBytes         = 32
	EncodedTicketLength = 43
	keyPrefix           = "mss:websocket:ticket:"
)

var (
	ErrNotFound = errors.New("websocket ticket not found")
	ErrExpired  = errors.New("websocket ticket expired")
)

type Record struct {
	UserID    string    `json:"userID"`
	RoleID    string    `json:"roleID"`
	SessionID string    `json:"sessionID"`
	Origin    string    `json:"origin"`
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type localEntry struct {
	payload   []byte
	expiresAt time.Time
}

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

func (s *Store) Issue(
	ctx context.Context,
	client redis.UniversalClient,
	record Record,
	ttl time.Duration,
) (ticket string, issued Record, err error) {
	if s == nil {
		return "", Record{}, errors.New("websocket ticket store is nil")
	}
	if ttl <= 0 {
		return "", Record{}, errors.New("websocket ticket ttl must be positive")
	}
	record, err = normalizeRecord(record)
	if err != nil {
		return "", Record{}, err
	}
	now := s.now().UTC()
	record.IssuedAt = now
	record.ExpiresAt = now.Add(ttl)
	payload, err := json.Marshal(record)
	if err != nil {
		return "", Record{}, fmt.Errorf("encode websocket ticket: %w", err)
	}
	for attempts := 0; attempts < 3; attempts++ {
		ticket, err = randomToken(s.random)
		if err != nil {
			return "", Record{}, fmt.Errorf("generate websocket ticket: %w", err)
		}
		created, storeErr := s.setNX(ctx, client, ticketKey(ticket), payload, ttl)
		if storeErr != nil {
			return "", Record{}, storeErr
		}
		if created {
			return ticket, record, nil
		}
	}
	return "", Record{}, errors.New("generate unique websocket ticket")
}

// Consume atomically removes a ticket before decoding or validating it. A
// failed origin/session check therefore burns the credential and replay always
// fails, including when multiple replicas share Redis.
func (s *Store) Consume(
	ctx context.Context,
	client redis.UniversalClient,
	ticket string,
) (Record, error) {
	if s == nil || !Valid(ticket) {
		return Record{}, ErrNotFound
	}
	payload, err := s.getAndDelete(ctx, client, ticketKey(ticket))
	if err != nil {
		return Record{}, err
	}
	if len(payload) == 0 {
		return Record{}, ErrNotFound
	}
	var record Record
	if err = json.Unmarshal(payload, &record); err != nil {
		return Record{}, fmt.Errorf("decode websocket ticket: %w", err)
	}
	record, err = normalizeRecord(record)
	if err != nil {
		return Record{}, fmt.Errorf("validate websocket ticket: %w", err)
	}
	if !s.now().Before(record.ExpiresAt) {
		return Record{}, ErrExpired
	}
	return record, nil
}

// Valid reports whether a value has the exact canonical encoding emitted by
// Issue. Rejecting arbitrary cache-key input bounds lookup work before Redis.
func Valid(ticket string) bool {
	if len(ticket) != EncodedTicketLength {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(ticket)
	return err == nil && len(decoded) == ticketBytes &&
		base64.RawURLEncoding.EncodeToString(decoded) == ticket
}

func normalizeRecord(record Record) (Record, error) {
	record.UserID = strings.TrimSpace(record.UserID)
	record.RoleID = strings.TrimSpace(record.RoleID)
	record.SessionID = strings.TrimSpace(record.SessionID)
	if record.UserID == "" || record.RoleID == "" || record.SessionID == "" {
		return Record{}, errors.New("websocket ticket identity and session are required")
	}
	origin, ok := browsersecurity.NormalizeOrigin(record.Origin)
	if !ok {
		return Record{}, errors.New("websocket ticket origin is invalid")
	}
	record.Origin = origin
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
			return false, fmt.Errorf("store websocket ticket: %w", err)
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
			return nil, fmt.Errorf("consume websocket ticket: %w", err)
		}
		switch typed := value.(type) {
		case string:
			return []byte(typed), nil
		case []byte:
			return typed, nil
		default:
			return nil, fmt.Errorf("consume websocket ticket: unexpected cache value %T", value)
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

func (s *Store) sweepExpiredLocked() {
	now := s.now()
	for key, entry := range s.local {
		if !now.Before(entry.expiresAt) {
			delete(s.local, key)
		}
	}
}

func randomToken(reader io.Reader) (string, error) {
	buffer := make([]byte, ticketBytes)
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func ticketKey(ticket string) string {
	sum := sha256.Sum256([]byte(ticket))
	return keyPrefix + hex.EncodeToString(sum[:])
}
