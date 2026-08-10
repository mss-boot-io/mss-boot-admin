package cache

import (
	"context"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// This is the deliberately provisional D0 challenge implementation carried by
// the v1.1.0 development train. It closes the unsafe verification-code path
// without freezing the Storage Runtime v2 public API.

var (
	ErrChallengeUnavailable = errors.New("challenge store unavailable")
	ErrChallengePending     = errors.New("challenge delivery already pending")
	ErrChallengeCooldown    = errors.New("challenge resend cooldown active")
	ErrChallengeQuota       = errors.New("challenge issue quota exceeded")
	ErrChallengeStale       = errors.New("challenge issue state is stale")
	ErrChallengeInvalid     = errors.New("invalid challenge input")
	ErrChallengeDelivery    = errors.New("challenge delivery failed")
)

const (
	defaultChallengeNamespace      = "mss:challenge:v1"
	defaultChallengeCodeTTL        = 5 * time.Minute
	defaultChallengePendingTTL     = 30 * time.Second
	defaultChallengeCooldown       = time.Minute
	defaultChallengeQuotaWindow    = time.Hour
	defaultChallengeMaxIssues      = 5
	defaultChallengeMaxAttempts    = 5
	defaultChallengeIdempotencyTTL = 2 * time.Minute
	defaultChallengeCallerWindow   = 10 * time.Minute
	defaultChallengeCallerLimit    = 10
	defaultChallengeGlobalWindow   = 10 * time.Minute
	defaultChallengeGlobalLimit    = 1000
	challengeStateExpiryGrace      = time.Minute
	minimumChallengeSecretBytes    = 32
)

var challengePurposePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type ChallengePurpose string

type ChallengePepper struct {
	Version string
	Secret  []byte
}

type ChallengeOptions struct {
	Namespace      string
	SubjectKey     []byte
	Peppers        []ChallengePepper
	CodeTTL        time.Duration
	PendingTTL     time.Duration
	Cooldown       time.Duration
	QuotaWindow    time.Duration
	MaxIssues      int
	MaxAttempts    int
	IdempotencyTTL time.Duration
	CallerWindow   time.Duration
	CallerLimit    int
	GlobalWindow   time.Duration
	GlobalLimit    int
}

// challengeIssue is an opaque, versioned delivery reservation. Only the code is
// passed to the delivery callback; Redis keys, subject data, and verifier state
// remain private to the store.
type challengeIssue struct {
	code     string
	version  string
	stateKey string
	quotaKey string
	opsKey   string
}

// ProvisionalChallenge deliberately exposes only delivery orchestration and
// verification. Begin/commit/abort remain implementation details until the
// v1.1 Runtime v2 API is designed.
type ProvisionalChallenge interface {
	Ready(context.Context) error
	Issue(context.Context, string, string, ChallengePurpose, func(context.Context, string) error) error
	VerifyChallenge(context.Context, string, ChallengePurpose, string) (bool, error)
}

type RedisChallengeStore struct {
	client         redis.UniversalClient
	namespace      string
	subjectKey     []byte
	peppers        map[string][]byte
	currentPepper  string
	codeTTL        time.Duration
	pendingTTL     time.Duration
	cooldown       time.Duration
	quotaWindow    time.Duration
	maxIssues      int
	maxAttempts    int
	idempotencyTTL time.Duration
	callerWindow   time.Duration
	callerLimit    int
	globalWindow   time.Duration
	globalLimit    int
	random         io.Reader
}

func NewRedisChallengeStore(client redis.UniversalClient, options ChallengeOptions) (*RedisChallengeStore, error) {
	if client == nil {
		return nil, errors.New("challenge Redis client is required")
	}
	if cacheClient, ok := client.(*Redis); ok {
		client = cacheClient.UniversalClient
		if client == nil {
			return nil, errors.New("challenge Redis client is required")
		}
	}
	// The D0 provisional adapter has no non-skippable multi-node Cluster
	// evidence yet. Reject concrete Cluster/Ring clients instead of turning a
	// same-slot design review into an unsupported production claim.
	switch client.(type) {
	case *redis.ClusterClient, *redis.Ring:
		return nil, ErrChallengeUnavailable
	}
	options = defaultChallengeOptions(options)
	if err := validateChallengeOptions(options); err != nil {
		return nil, err
	}
	peppers := make(map[string][]byte, len(options.Peppers))
	for _, pepper := range options.Peppers {
		peppers[pepper.Version] = append([]byte(nil), pepper.Secret...)
	}
	return &RedisChallengeStore{
		client:         client,
		namespace:      strings.TrimSuffix(options.Namespace, ":"),
		subjectKey:     append([]byte(nil), options.SubjectKey...),
		peppers:        peppers,
		currentPepper:  options.Peppers[0].Version,
		codeTTL:        options.CodeTTL,
		pendingTTL:     options.PendingTTL,
		cooldown:       options.Cooldown,
		quotaWindow:    options.QuotaWindow,
		maxIssues:      options.MaxIssues,
		maxAttempts:    options.MaxAttempts,
		idempotencyTTL: options.IdempotencyTTL,
		callerWindow:   options.CallerWindow,
		callerLimit:    options.CallerLimit,
		globalWindow:   options.GlobalWindow,
		globalLimit:    options.GlobalLimit,
		random:         cryptorand.Reader,
	}, nil
}

func defaultChallengeOptions(options ChallengeOptions) ChallengeOptions {
	if strings.TrimSpace(options.Namespace) == "" {
		options.Namespace = defaultChallengeNamespace
	}
	if options.CodeTTL == 0 {
		options.CodeTTL = defaultChallengeCodeTTL
	}
	if options.PendingTTL == 0 {
		options.PendingTTL = defaultChallengePendingTTL
	}
	if options.Cooldown == 0 {
		options.Cooldown = defaultChallengeCooldown
	}
	if options.QuotaWindow == 0 {
		options.QuotaWindow = defaultChallengeQuotaWindow
	}
	if options.MaxIssues == 0 {
		options.MaxIssues = defaultChallengeMaxIssues
	}
	if options.MaxAttempts == 0 {
		options.MaxAttempts = defaultChallengeMaxAttempts
	}
	if options.IdempotencyTTL == 0 {
		options.IdempotencyTTL = defaultChallengeIdempotencyTTL
	}
	if options.CallerWindow == 0 {
		options.CallerWindow = defaultChallengeCallerWindow
	}
	if options.CallerLimit == 0 {
		options.CallerLimit = defaultChallengeCallerLimit
	}
	if options.GlobalWindow == 0 {
		options.GlobalWindow = defaultChallengeGlobalWindow
	}
	if options.GlobalLimit == 0 {
		options.GlobalLimit = defaultChallengeGlobalLimit
	}
	return options
}

func validateChallengeOptions(options ChallengeOptions) error {
	if strings.ContainsAny(options.Namespace, "{}\r\n\t ") {
		return errors.New("challenge namespace contains invalid characters")
	}
	if !plausibleChallengeSecret(options.SubjectKey) {
		return errors.New("challenge subject-key secret must contain at least 32 bytes")
	}
	if len(options.Peppers) == 0 || len(options.Peppers) > 2 {
		return errors.New("challenge requires one current pepper and at most one previous pepper")
	}
	versions := make(map[string]struct{}, len(options.Peppers))
	for _, pepper := range options.Peppers {
		if !challengePurposePattern.MatchString(pepper.Version) {
			return errors.New("challenge pepper version is invalid")
		}
		if !plausibleChallengeSecret(pepper.Secret) {
			return errors.New("challenge pepper secret must contain at least 32 bytes")
		}
		if hmac.Equal(options.SubjectKey, pepper.Secret) {
			return errors.New("challenge subject-key and pepper secrets must be independent")
		}
		if _, exists := versions[pepper.Version]; exists {
			return errors.New("challenge pepper versions must be unique")
		}
		versions[pepper.Version] = struct{}{}
	}
	if len(options.Peppers) == 2 && hmac.Equal(options.Peppers[0].Secret, options.Peppers[1].Secret) {
		return errors.New("challenge pepper versions must use independent secrets")
	}
	if options.CodeTTL <= 0 || options.PendingTTL <= 0 || options.Cooldown <= 0 ||
		options.QuotaWindow <= 0 || options.IdempotencyTTL <= 0 ||
		options.CallerWindow <= 0 || options.GlobalWindow <= 0 {
		return errors.New("challenge durations must be positive")
	}
	for _, duration := range []time.Duration{
		options.CodeTTL,
		options.PendingTTL,
		options.Cooldown,
		options.QuotaWindow,
		options.IdempotencyTTL,
		options.CallerWindow,
		options.GlobalWindow,
	} {
		if duration < time.Millisecond || duration%time.Millisecond != 0 {
			return errors.New("challenge durations must be whole milliseconds")
		}
	}
	if options.PendingTTL >= options.CodeTTL {
		return errors.New("challenge pending lease must be shorter than the active lifetime")
	}
	if options.MaxIssues <= 0 || options.MaxAttempts <= 0 || options.CallerLimit <= 0 || options.GlobalLimit <= 0 {
		return errors.New("challenge limits must be positive")
	}
	return nil
}

func plausibleChallengeSecret(secret []byte) bool {
	if len(secret) < minimumChallengeSecretBytes {
		return false
	}
	// Entropy cannot be proven from one sample, but obvious placeholder values
	// (all-zero/repeated-character material) must never pass as production keys.
	// Deployment guidance still requires generation from a CSPRNG.
	unique := make(map[byte]struct{}, len(secret))
	for _, value := range secret {
		unique[value] = struct{}{}
	}
	return len(unique) >= 16
}

func (s *RedisChallengeStore) Ready(ctx context.Context) error {
	if ctx == nil {
		return ErrChallengeUnavailable
	}
	if err := s.client.Ping(ctx).Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return errors.Join(ErrChallengeUnavailable, ctxErr)
		}
		return ErrChallengeUnavailable
	}
	return nil
}

func (s *RedisChallengeStore) Issue(
	ctx context.Context,
	caller string,
	subject string,
	purpose ChallengePurpose,
	deliver func(context.Context, string) error,
) error {
	if ctx == nil || deliver == nil {
		return ErrChallengeInvalid
	}
	if err := s.limitIssue(ctx, caller); err != nil {
		return err
	}
	issue, err := s.beginIssue(ctx, subject, purpose)
	if err != nil {
		return err
	}
	if err = deliver(ctx, issue.code); err != nil {
		// Compensation stays inside the caller-owned lifecycle. If the caller
		// has already been canceled, the short pending lease is the recovery
		// boundary; Issue must not create a detached cleanup goroutine/context.
		abortCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if abortErr := s.abortIssue(abortCtx, issue); abortErr != nil {
			return ErrChallengeUnavailable
		}
		return ErrChallengeDelivery
	}
	return s.commitIssue(ctx, issue)
}

func (s *RedisChallengeStore) limitIssue(ctx context.Context, caller string) error {
	caller = strings.TrimSpace(caller)
	if caller == "" || len(caller) > 512 {
		return ErrChallengeInvalid
	}
	mac := hmac.New(sha256.New, s.subjectKey)
	_, _ = io.WriteString(mac, "mss/challenge-caller/v1")
	_, _ = mac.Write([]byte{0})
	_, _ = io.WriteString(mac, caller)
	callerFingerprint := hex.EncodeToString(mac.Sum(nil))
	operationID, err := randomChallengeVersion(s.random)
	if err != nil {
		return ErrChallengeUnavailable
	}
	result, err := limitChallengeIssueScript.Run(ctx, s.client, []string{
		s.namespace + ":rate:caller:" + callerFingerprint,
		s.namespace + ":rate:global",
	},
		operationID,
		int64(s.callerWindow/time.Millisecond),
		s.callerLimit,
		int64(s.globalWindow/time.Millisecond),
		s.globalLimit,
		int64(challengeStateExpiryGrace/time.Millisecond),
	).Text()
	if err != nil {
		return challengeStoreError(ctx)
	}
	if result == "OK" {
		return nil
	}
	if result == "CALLER" || result == "GLOBAL" {
		return ErrChallengeQuota
	}
	return ErrChallengeUnavailable
}

func (s *RedisChallengeStore) beginIssue(
	ctx context.Context,
	subject string,
	purpose ChallengePurpose,
) (*challengeIssue, error) {
	stateKey, quotaKey, opsKey, fingerprint, err := s.keys(subject, purpose)
	if err != nil {
		return nil, err
	}
	code, err := randomChallengeCode(s.random)
	if err != nil {
		return nil, ErrChallengeUnavailable
	}
	version, err := randomChallengeVersion(s.random)
	if err != nil {
		return nil, ErrChallengeUnavailable
	}
	digest := challengeDigest(s.peppers[s.currentPepper], fingerprint, version, code)
	result, err := beginChallengeScript.Run(ctx, s.client, []string{stateKey, quotaKey},
		version,
		s.maxIssues,
		hex.EncodeToString(digest),
		s.currentPepper,
		int64(s.pendingTTL/time.Millisecond),
		int64(s.codeTTL/time.Millisecond),
		int64(s.cooldown/time.Millisecond),
		int64(s.quotaWindow/time.Millisecond),
		int64(challengeStateExpiryGrace/time.Millisecond),
	).Text()
	if err != nil {
		return nil, challengeStoreError(ctx)
	}
	switch result {
	case "OK":
		return &challengeIssue{
			code: code, version: version, stateKey: stateKey, quotaKey: quotaKey, opsKey: opsKey,
		}, nil
	case "PENDING":
		return nil, ErrChallengePending
	case "COOLDOWN":
		return nil, ErrChallengeCooldown
	case "QUOTA":
		return nil, ErrChallengeQuota
	default:
		return nil, ErrChallengeUnavailable
	}
}

func (s *RedisChallengeStore) commitIssue(ctx context.Context, issue *challengeIssue) error {
	if !s.validIssue(issue) {
		return ErrChallengeInvalid
	}
	result, err := commitChallengeScript.Run(ctx, s.client, []string{issue.stateKey, issue.opsKey},
		issue.version,
		int64(challengeStateExpiryGrace/time.Millisecond),
	).Text()
	if err != nil {
		return challengeStoreError(ctx)
	}
	switch result {
	case "OK":
		return nil
	case "EXPIRED", "STALE":
		return ErrChallengeStale
	default:
		return ErrChallengeUnavailable
	}
}

func (s *RedisChallengeStore) abortIssue(ctx context.Context, issue *challengeIssue) error {
	if !s.validIssue(issue) {
		return ErrChallengeInvalid
	}
	result, err := abortChallengeScript.Run(ctx, s.client, []string{issue.stateKey}, issue.version).Text()
	if err != nil {
		return challengeStoreError(ctx)
	}
	if result == "OK" {
		return nil
	}
	if result == "STALE" {
		return ErrChallengeStale
	}
	return ErrChallengeUnavailable
}

func (s *RedisChallengeStore) VerifyChallenge(
	ctx context.Context,
	subject string,
	purpose ChallengePurpose,
	code string,
) (bool, error) {
	if ctx == nil {
		return false, ErrChallengeUnavailable
	}
	stateKey, _, opsKey, fingerprint, err := s.keys(subject, purpose)
	if err != nil || !validChallengeCode(code) {
		return false, nil
	}
	values, redisErr := s.client.HMGet(ctx, stateKey,
		"active_id", "active_digest", "active_pepper").Result()
	if redisErr != nil {
		return false, challengeStoreError(ctx)
	}
	activeID, ok := redisString(values, 0)
	if !ok {
		return false, nil
	}
	digestHex, digestOK := redisString(values, 1)
	pepperVersion, pepperOK := redisString(values, 2)
	if !digestOK || !pepperOK {
		return false, ErrChallengeUnavailable
	}

	pepper, exists := s.peppers[pepperVersion]
	storedDigest, decodeErr := hex.DecodeString(digestHex)
	if decodeErr != nil || len(storedDigest) != sha256.Size || !exists {
		// Keep corrupt and unknown-pepper paths from becoming a cheap timing
		// oracle while failing closed without charging an attempt.
		dummy := make([]byte, sha256.Size)
		_ = constantTimeDigestEqual(dummy, challengeDigest(make([]byte, sha256.Size), fingerprint, activeID, code))
		return false, ErrChallengeUnavailable
	}
	candidate := challengeDigest(pepper, fingerprint, activeID, code)
	matched := constantTimeDigestEqual(storedDigest, candidate)

	operationID, randomErr := randomChallengeVersion(s.random)
	if randomErr != nil {
		return false, ErrChallengeUnavailable
	}
	result, scriptErr := completeChallengeVerifyScript.Run(ctx, s.client, []string{stateKey, opsKey},
		activeID,
		digestHex,
		operationID,
		s.maxAttempts,
		boolInt(matched),
		int64(s.idempotencyTTL/time.Millisecond),
	).Text()
	if scriptErr != nil {
		return false, challengeStoreError(ctx)
	}
	switch result {
	case "SUCCESS":
		return true, nil
	case "INVALID", "LOCKED", "EXPIRED", "MISSING":
		return false, nil
	case "STALE":
		return false, nil
	default:
		return false, ErrChallengeUnavailable
	}
}

func (s *RedisChallengeStore) keys(
	subject string,
	purpose ChallengePurpose,
) (stateKey, quotaKey, opsKey, fingerprint string, err error) {
	// Subject canonicalization belongs to the consumer domain. The framework
	// only removes surrounding transport whitespace; lower-casing an arbitrary
	// identity can merge principals that the authoritative store keeps distinct.
	normalizedSubject := strings.TrimSpace(subject)
	normalizedPurpose := strings.ToLower(strings.TrimSpace(string(purpose)))
	if normalizedSubject == "" || !challengePurposePattern.MatchString(normalizedPurpose) {
		return "", "", "", "", ErrChallengeInvalid
	}
	mac := hmac.New(sha256.New, s.subjectKey)
	_, _ = io.WriteString(mac, "mss/challenge-key/v1")
	_, _ = mac.Write([]byte{0})
	_, _ = io.WriteString(mac, normalizedPurpose)
	_, _ = mac.Write([]byte{0})
	_, _ = io.WriteString(mac, normalizedSubject)
	fingerprint = hex.EncodeToString(mac.Sum(nil))
	hashTag := "{" + fingerprint + "}"
	stateKey = s.namespace + ":" + hashTag + ":state"
	quotaKey = s.namespace + ":" + hashTag + ":quota"
	opsKey = s.namespace + ":" + hashTag + ":ops"
	if err = ensureSameRedisHashTag(stateKey, quotaKey, opsKey); err != nil {
		return "", "", "", "", err
	}
	return stateKey, quotaKey, opsKey, fingerprint, nil
}

func (s *RedisChallengeStore) validIssue(issue *challengeIssue) bool {
	if issue == nil || issue.version == "" || issue.stateKey == "" || issue.quotaKey == "" || issue.opsKey == "" {
		return false
	}
	if !strings.HasPrefix(issue.stateKey, s.namespace+":") || !strings.HasPrefix(issue.quotaKey, s.namespace+":") ||
		!strings.HasPrefix(issue.opsKey, s.namespace+":") {
		return false
	}
	return ensureSameRedisHashTag(issue.stateKey, issue.quotaKey, issue.opsKey) == nil
}

func challengeDigest(pepper []byte, fingerprint, issueID, code string) []byte {
	mac := hmac.New(sha256.New, pepper)
	_, _ = io.WriteString(mac, "mss/challenge-verifier/v1")
	_, _ = mac.Write([]byte{0})
	_, _ = io.WriteString(mac, fingerprint)
	_, _ = mac.Write([]byte{0})
	_, _ = io.WriteString(mac, issueID)
	_, _ = mac.Write([]byte{0})
	_, _ = io.WriteString(mac, code)
	return mac.Sum(nil)
}

// constantTimeDigestEqual is intentionally tiny so the security regression
// test can prove that the live VerifyChallenge data path delegates digest
// comparison to the standard-library constant-time primitive.
func constantTimeDigestEqual(stored, candidate []byte) bool {
	return hmac.Equal(stored, candidate)
}

func randomChallengeCode(reader io.Reader) (string, error) {
	value, err := cryptorand.Int(reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func randomChallengeVersion(reader io.Reader) (string, error) {
	value := make([]byte, 24)
	if _, err := io.ReadFull(reader, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func validChallengeCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, value := range code {
		if value < '0' || value > '9' {
			return false
		}
	}
	return true
}

func challengeStoreError(ctx context.Context) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Join(ErrChallengeUnavailable, err)
		}
	}
	return ErrChallengeUnavailable
}

func redisString(values []any, index int) (string, bool) {
	if index < 0 || index >= len(values) || values[index] == nil {
		return "", false
	}
	value, ok := values[index].(string)
	return value, ok && value != ""
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func ensureSameRedisHashTag(keys ...string) error {
	if len(keys) < 2 {
		return nil
	}
	want, ok := redisHashTag(keys[0])
	if !ok {
		return errors.New("challenge Redis key is missing a hash tag")
	}
	for _, key := range keys[1:] {
		got, exists := redisHashTag(key)
		if !exists || got != want {
			return errors.New("challenge Redis keys cross hash slots")
		}
	}
	return nil
}

func redisHashTag(key string) (string, bool) {
	start := strings.IndexByte(key, '{')
	if start < 0 || start+1 >= len(key) {
		return "", false
	}
	end := strings.IndexByte(key[start+1:], '}')
	if end <= 0 {
		return "", false
	}
	return key[start+1 : start+1+end], true
}

var limitChallengeIssueScript = redis.NewScript(`
local caller = KEYS[1]
local global = KEYS[2]
local operation_id = ARGV[1]
local caller_window = tonumber(ARGV[2])
local caller_limit = tonumber(ARGV[3])
local global_window = tonumber(ARGV[4])
local global_limit = tonumber(ARGV[5])
local grace = tonumber(ARGV[6])
local server_time = redis.call('TIME')
local now = tonumber(server_time[1]) * 1000 + math.floor(tonumber(server_time[2]) / 1000)

redis.call('ZREMRANGEBYSCORE', caller, '-inf', now - caller_window)
redis.call('ZREMRANGEBYSCORE', global, '-inf', now - global_window)
if tonumber(redis.call('ZCARD', caller)) >= caller_limit then
  return 'CALLER'
end
if tonumber(redis.call('ZCARD', global)) >= global_limit then
  return 'GLOBAL'
end
redis.call('ZADD', caller, now, operation_id)
redis.call('ZADD', global, now, operation_id)
redis.call('PEXPIRE', caller, math.max(1, caller_window + grace))
redis.call('PEXPIRE', global, math.max(1, global_window + grace))
return 'OK'
`)

var beginChallengeScript = redis.NewScript(`
local state = KEYS[1]
local quota = KEYS[2]
local issue_id = ARGV[1]
local max_issues = tonumber(ARGV[2])
local pending_ttl = tonumber(ARGV[5])
local challenge_ttl = tonumber(ARGV[6])
local cooldown_ttl = tonumber(ARGV[7])
local quota_window = tonumber(ARGV[8])
local grace = tonumber(ARGV[9])
local server_time = redis.call('TIME')
local now = tonumber(server_time[1]) * 1000 + math.floor(tonumber(server_time[2]) / 1000)

local pending_id = redis.call('HGET', state, 'pending_id')
if pending_id then
  local pending_lease = tonumber(redis.call('HGET', state, 'pending_lease_until') or '0')
  if pending_id == issue_id and pending_lease > now then
    return 'OK'
  end
  if pending_lease > now then
    return 'PENDING'
  end
  redis.call('HDEL', state,
    'pending_id', 'pending_digest', 'pending_pepper',
    'pending_lease_until', 'pending_ttl_ms', 'pending_cooldown_ms')
end

local cooldown = tonumber(redis.call('HGET', state, 'cooldown_until') or '0')
if cooldown > now then
  return 'COOLDOWN'
end

redis.call('ZREMRANGEBYSCORE', quota, '-inf', now - quota_window)
if tonumber(redis.call('ZCARD', quota)) >= max_issues then
  return 'QUOTA'
end
redis.call('ZADD', quota, now, issue_id)

redis.call('HSET', state,
  'pending_id', issue_id,
  'pending_digest', ARGV[3],
  'pending_pepper', ARGV[4],
  'pending_lease_until', now + pending_ttl,
  'pending_ttl_ms', challenge_ttl,
  'pending_cooldown_ms', cooldown_ttl)

local active_expires = tonumber(redis.call('HGET', state, 'active_expires_at') or '0')
local state_deadline = math.max(active_expires, now + pending_ttl, cooldown)
redis.call('PEXPIRE', state, math.max(1, state_deadline - now + grace))
redis.call('PEXPIRE', quota, math.max(1, quota_window + grace))
return 'OK'
`)

var commitChallengeScript = redis.NewScript(`
local state = KEYS[1]
local ops = KEYS[2]
local issue_id = ARGV[1]
local grace = tonumber(ARGV[2])
local server_time = redis.call('TIME')
local now = tonumber(server_time[1]) * 1000 + math.floor(tonumber(server_time[2]) / 1000)

if redis.call('HGET', state, 'active_id') == issue_id then
  return 'OK'
end
if redis.call('HGET', state, 'pending_id') ~= issue_id then
  return 'STALE'
end
local lease = tonumber(redis.call('HGET', state, 'pending_lease_until') or '0')
if lease <= now then
  redis.call('HDEL', state,
    'pending_id', 'pending_digest', 'pending_pepper',
    'pending_lease_until', 'pending_ttl_ms', 'pending_cooldown_ms')
  return 'EXPIRED'
end

local digest = redis.call('HGET', state, 'pending_digest')
local pepper = redis.call('HGET', state, 'pending_pepper')
local expires = now + tonumber(redis.call('HGET', state, 'pending_ttl_ms'))
local cooldown = now + tonumber(redis.call('HGET', state, 'pending_cooldown_ms'))
redis.call('HSET', state,
  'active_id', issue_id,
  'active_digest', digest,
  'active_pepper', pepper,
  'active_expires_at', expires,
  'active_attempts', 0,
  'cooldown_until', cooldown)
redis.call('HDEL', state,
  'pending_id', 'pending_digest', 'pending_pepper',
  'pending_lease_until', 'pending_ttl_ms', 'pending_cooldown_ms')
redis.call('DEL', ops)
local deadline = math.max(expires, cooldown)
redis.call('PEXPIRE', state, math.max(1, deadline - now + grace))
return 'OK'
`)

var abortChallengeScript = redis.NewScript(`
local state = KEYS[1]
local issue_id = ARGV[1]
if redis.call('HGET', state, 'last_abort_id') == issue_id then
  return 'OK'
end
if redis.call('HGET', state, 'pending_id') ~= issue_id then
  return 'STALE'
end
redis.call('HDEL', state,
  'pending_id', 'pending_digest', 'pending_pepper',
  'pending_lease_until', 'pending_ttl_ms', 'pending_cooldown_ms')
redis.call('HSET', state, 'last_abort_id', issue_id)
return 'OK'
`)

var completeChallengeVerifyScript = redis.NewScript(`
local state = KEYS[1]
local ops = KEYS[2]
local expected_id = ARGV[1]
local expected_digest = ARGV[2]
local operation_id = ARGV[3]
local max_attempts = tonumber(ARGV[4])
local matched = tonumber(ARGV[5])
local idempotency_ttl = tonumber(ARGV[6])
local server_time = redis.call('TIME')
local now = tonumber(server_time[1]) * 1000 + math.floor(tonumber(server_time[2]) / 1000)

local replay = redis.call('HGET', ops, operation_id)
if replay then
  local separator = string.find(replay, '|', 1, true)
  if separator and string.sub(replay, 1, separator - 1) == expected_id then
    return string.sub(replay, separator + 1)
  end
  return 'STALE'
end
if redis.call('HGET', state, 'active_id') ~= expected_id or
   redis.call('HGET', state, 'active_digest') ~= expected_digest then
  return 'STALE'
end

local function clear_active()
  redis.call('HDEL', state,
    'active_id', 'active_digest', 'active_pepper',
    'active_expires_at', 'active_attempts')
end
local function remember(result)
  redis.call('HSET', ops, operation_id, expected_id .. '|' .. result)
  redis.call('PEXPIRE', ops, idempotency_ttl)
  return result
end

local expires = tonumber(redis.call('HGET', state, 'active_expires_at') or '0')
if expires <= now then
  clear_active()
  return remember('EXPIRED')
end
if matched == 1 then
  clear_active()
  return remember('SUCCESS')
end

local attempts = tonumber(redis.call('HGET', state, 'active_attempts') or '0') + 1
if attempts >= max_attempts then
  clear_active()
  return remember('LOCKED')
end
redis.call('HSET', state, 'active_attempts', attempts)
return remember('INVALID')
`)
