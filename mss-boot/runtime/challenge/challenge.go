// Package challenge provides delivery-safe, one-time verification challenges
// backed by a named Runtime v2 Redis scope.
package challenge

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
	"log/slog"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/internal/redisbridge"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/redisresource"
)

var (
	ErrUnavailable = errors.New("challenge store unavailable")
	ErrStale       = errors.New("challenge issue state is stale")
	ErrInvalid     = errors.New("invalid challenge input")
	errQuota       = errors.New("challenge issue quota exceeded")
)

const (
	defaultCodeTTL        = 5 * time.Minute
	defaultPendingTTL     = 30 * time.Second
	defaultCooldown       = time.Minute
	defaultQuotaWindow    = time.Hour
	defaultMaxIssues      = 5
	defaultMaxAttempts    = 5
	defaultIdempotencyTTL = 2 * time.Minute
	defaultCallerWindow   = 10 * time.Minute
	defaultCallerLimit    = 10
	defaultGlobalWindow   = 10 * time.Minute
	defaultGlobalLimit    = 1000
	stateExpiryGrace      = time.Minute
	minimumSecretBytes    = 32
)

var purposePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// Purpose separates independent challenge workflows such as login or signup.
type Purpose string

// Pepper is one verifier-secret generation. Peppers[0] is current and an
// optional Peppers[1] permits verification during one rotation window.
type Pepper struct {
	Version string
	Secret  []byte
}

func (Pepper) String() string               { return "ChallengePepper<redacted>" }
func (p Pepper) GoString() string           { return p.String() }
func (p Pepper) LogValue() slog.Value       { return slog.StringValue(p.String()) }
func (Pepper) MarshalJSON() ([]byte, error) { return []byte(`"ChallengePepper<redacted>"`), nil }
func (p Pepper) MarshalYAML() (any, error)  { return p.String(), nil }

// Options intentionally has no namespace. Physical isolation is owned by the
// named redisresource.Scope and its opaque atomic groups.
type Options struct {
	SubjectKey     []byte
	Peppers        []Pepper
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

func (Options) String() string               { return "ChallengeOptions<redacted>" }
func (o Options) GoString() string           { return o.String() }
func (o Options) LogValue() slog.Value       { return slog.StringValue(o.String()) }
func (Options) MarshalJSON() ([]byte, error) { return []byte(`"ChallengeOptions<redacted>"`), nil }
func (o Options) MarshalYAML() (any, error)  { return o.String(), nil }

// BeginRequest is the complete public input for one issue reservation.
type BeginRequest struct {
	Caller  string
	Subject string
	Purpose Purpose
}

func (BeginRequest) String() string         { return "ChallengeBeginRequest<redacted>" }
func (r BeginRequest) GoString() string     { return r.String() }
func (r BeginRequest) LogValue() slog.Value { return slog.StringValue(r.String()) }
func (BeginRequest) MarshalJSON() ([]byte, error) {
	return []byte(`"ChallengeBeginRequest<redacted>"`), nil
}
func (r BeginRequest) MarshalYAML() (any, error) { return r.String(), nil }

// BeginState is a fixed, non-diagnostic issue decision.
type BeginState uint8

const (
	BeginReserved BeginState = iota + 1
	BeginPending
	BeginCooldown
	BeginQuota
)

func (s BeginState) String() string {
	switch s {
	case BeginReserved:
		return "reserved"
	case BeginPending:
		return "pending"
	case BeginCooldown:
		return "cooldown"
	case BeginQuota:
		return "quota"
	default:
		return "invalid"
	}
}

func (s BeginState) GoString() string { return s.String() }

// BeginOutcome carries a fixed decision and, only when Reserved, the opaque
// delivery reservation. Its fields are private so invalid combinations cannot
// be constructed by consumers.
type BeginOutcome struct {
	state       BeginState
	reservation *Reservation
}

func (o BeginOutcome) State() BeginState { return o.state }

func (o BeginOutcome) Reservation() (*Reservation, bool) {
	return o.reservation, o.state == BeginReserved && o.reservation != nil
}

func (o BeginOutcome) String() string               { return "ChallengeBeginOutcome{" + o.state.String() + "}" }
func (o BeginOutcome) GoString() string             { return o.String() }
func (o BeginOutcome) LogValue() slog.Value         { return slog.StringValue(o.String()) }
func (o BeginOutcome) MarshalJSON() ([]byte, error) { return []byte(`"` + o.String() + `"`), nil }
func (o BeginOutcome) MarshalYAML() (any, error)    { return o.String(), nil }

// VerifyRequest is the complete public input for one verification attempt.
type VerifyRequest struct {
	Subject string
	Purpose Purpose
	Code    string
}

func (VerifyRequest) String() string         { return "ChallengeVerifyRequest<redacted>" }
func (r VerifyRequest) GoString() string     { return r.String() }
func (r VerifyRequest) LogValue() slog.Value { return slog.StringValue(r.String()) }
func (VerifyRequest) MarshalJSON() ([]byte, error) {
	return []byte(`"ChallengeVerifyRequest<redacted>"`), nil
}
func (r VerifyRequest) MarshalYAML() (any, error) { return r.String(), nil }

// VerifyOutcome deliberately collapses missing, malformed, expired, locked,
// stale, and incorrect challenges into Rejected.
type VerifyOutcome uint8

const (
	VerifyRejected VerifyOutcome = iota
	VerifyVerified
)

func (o VerifyOutcome) String() string {
	if o == VerifyVerified {
		return "verified"
	}
	return "rejected"
}

func (o VerifyOutcome) GoString() string { return o.String() }

// Reservation is an opaque delivery reservation. Code is the only challenge
// material intentionally available to the delivery boundary.
type Reservation struct {
	owner    *reservationOwner
	code     string
	version  string
	group    redisbridge.AtomicGroup
	stateKey redisbridge.Key
	quotaKey redisbridge.Key
	opsKey   redisbridge.Key
}

type reservationOwner struct{ marker byte }

// Code returns the fixed-width one-time code to deliver.
func (r *Reservation) Code() string {
	if r == nil || r.owner == nil {
		return ""
	}
	return r.code
}

func (r *Reservation) String() string {
	if r == nil || r.owner == nil {
		return "ChallengeReservation<invalid>"
	}
	return "ChallengeReservation<opaque>"
}

func (r *Reservation) GoString() string             { return r.String() }
func (r *Reservation) LogValue() slog.Value         { return slog.StringValue(r.String()) }
func (r *Reservation) MarshalJSON() ([]byte, error) { return []byte(`"` + r.String() + `"`), nil }
func (r *Reservation) MarshalYAML() (any, error)    { return r.String(), nil }

// Redis implements the public challenge capability without owning or closing
// the shared named Redis resource.
type Redis struct {
	scope          *redisresource.Scope
	owner          *reservationOwner
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
	verifier       verifyExecutor
}

type verifyExecutor interface {
	Read(context.Context, *Redis, redisbridge.AtomicGroup, redisbridge.Key) ([]string, error)
	Complete(context.Context, *Redis, redisbridge.AtomicGroup, redisbridge.Key, redisbridge.Key, string, string, bool) (string, error)
}

type redisVerifyExecutor struct{}

func (s *Redis) String() string {
	if s == nil {
		return "RedisChallenge<nil>"
	}
	return "RedisChallenge<redacted>"
}

func (s *Redis) GoString() string             { return s.String() }
func (s *Redis) LogValue() slog.Value         { return slog.StringValue(s.String()) }
func (s *Redis) MarshalJSON() ([]byte, error) { return []byte(`"` + s.String() + `"`), nil }
func (s *Redis) MarshalYAML() (any, error)    { return s.String(), nil }

// NewRedis is a pure constructor. It validates and copies secrets but performs
// no Redis I/O, opens no connection, and owns no Close operation.
func NewRedis(scope *redisresource.Scope, options Options) (*Redis, error) {
	if scope == nil {
		return nil, ErrInvalid
	}
	options = defaultOptions(options)
	if err := validateOptions(options); err != nil {
		return nil, err
	}
	peppers := make(map[string][]byte, len(options.Peppers))
	for _, pepper := range options.Peppers {
		peppers[pepper.Version] = append([]byte(nil), pepper.Secret...)
	}
	return &Redis{
		scope: scope, owner: &reservationOwner{},
		subjectKey: append([]byte(nil), options.SubjectKey...),
		peppers:    peppers, currentPepper: options.Peppers[0].Version,
		codeTTL: options.CodeTTL, pendingTTL: options.PendingTTL,
		cooldown: options.Cooldown, quotaWindow: options.QuotaWindow,
		maxIssues: options.MaxIssues, maxAttempts: options.MaxAttempts,
		idempotencyTTL: options.IdempotencyTTL,
		callerWindow:   options.CallerWindow, callerLimit: options.CallerLimit,
		globalWindow: options.GlobalWindow, globalLimit: options.GlobalLimit,
		random: cryptorand.Reader, verifier: redisVerifyExecutor{},
	}, nil
}

// Ready checks the shared resource without acquiring ownership of it.
func (s *Redis) Ready(ctx context.Context) error {
	if s == nil || s.scope == nil || ctx == nil {
		return ErrUnavailable
	}
	if err := s.scope.Ready(ctx); err != nil {
		return storeError(ctx)
	}
	return nil
}

// BeginIssue atomically applies caller/global issue limits, then reserves one
// subject challenge without exposing its Redis group, keys, verifier, or ID.
func (s *Redis) BeginIssue(ctx context.Context, request BeginRequest) (BeginOutcome, error) {
	if s == nil || s.scope == nil || ctx == nil {
		return BeginOutcome{}, ErrInvalid
	}
	caller := strings.TrimSpace(request.Caller)
	if caller == "" || len(caller) > 512 {
		return BeginOutcome{}, ErrInvalid
	}
	normalizedSubject, normalizedPurpose, err := normalizeSubject(request.Subject, request.Purpose)
	if err != nil {
		return BeginOutcome{}, err
	}
	if err = s.limitIssue(ctx, caller); err != nil {
		if errors.Is(err, errQuota) {
			return BeginOutcome{state: BeginQuota}, nil
		}
		return BeginOutcome{}, err
	}

	subjectFingerprint := fingerprint(s.subjectKey, "mss/challenge-key/v1", normalizedPurpose, normalizedSubject)
	group, stateKey, quotaKey, opsKey, err := subjectCapabilities(subjectFingerprint)
	if err != nil {
		return BeginOutcome{}, ErrUnavailable
	}
	code, err := randomCode(s.random)
	if err != nil {
		return BeginOutcome{}, ErrUnavailable
	}
	version, err := randomID(s.random)
	if err != nil {
		return BeginOutcome{}, ErrUnavailable
	}
	verifier := digest(s.peppers[s.currentPepper], subjectFingerprint, version, code)
	var result string
	err = redisbridge.Use(ctx, s.scope, group, func(lease redisbridge.Lease) error {
		reply, runErr := lease.Run(ctx, redisbridge.ChallengeBeginScript(), []redisbridge.Key{stateKey, quotaKey},
			version,
			s.maxIssues,
			hex.EncodeToString(verifier),
			s.currentPepper,
			int64(s.pendingTTL/time.Millisecond),
			int64(s.codeTTL/time.Millisecond),
			int64(s.cooldown/time.Millisecond),
			int64(s.quotaWindow/time.Millisecond),
			int64(stateExpiryGrace/time.Millisecond),
		)
		if runErr != nil {
			return runErr
		}
		result, runErr = reply.Text()
		return runErr
	})
	if err != nil {
		return BeginOutcome{}, storeError(ctx)
	}
	switch result {
	case "OK":
		reservation := &Reservation{
			owner: s.owner, code: code, version: version,
			group: group, stateKey: stateKey, quotaKey: quotaKey, opsKey: opsKey,
		}
		return BeginOutcome{state: BeginReserved, reservation: reservation}, nil
	case "PENDING":
		return BeginOutcome{state: BeginPending}, nil
	case "COOLDOWN":
		return BeginOutcome{state: BeginCooldown}, nil
	case "QUOTA":
		return BeginOutcome{state: BeginQuota}, nil
	default:
		return BeginOutcome{}, ErrUnavailable
	}
}

func (s *Redis) limitIssue(ctx context.Context, caller string) error {
	callerFingerprint := fingerprint(s.subjectKey, "mss/challenge-caller/v1", caller, "")
	group, err := redisbridge.NewAtomicGroup("challenge-rate", []byte("v1"))
	if err != nil {
		return ErrUnavailable
	}
	callerKey, err := group.Key("caller-" + callerFingerprint)
	if err != nil {
		return ErrUnavailable
	}
	globalKey, err := group.Key("global")
	if err != nil {
		return ErrUnavailable
	}
	operationID, err := randomID(s.random)
	if err != nil {
		return ErrUnavailable
	}
	var result string
	err = redisbridge.Use(ctx, s.scope, group, func(lease redisbridge.Lease) error {
		reply, runErr := lease.Run(ctx, redisbridge.ChallengeRateScript(), []redisbridge.Key{callerKey, globalKey},
			operationID,
			int64(s.callerWindow/time.Millisecond),
			s.callerLimit,
			int64(s.globalWindow/time.Millisecond),
			s.globalLimit,
			int64(stateExpiryGrace/time.Millisecond),
		)
		if runErr != nil {
			return runErr
		}
		result, runErr = reply.Text()
		return runErr
	})
	if err != nil {
		return storeError(ctx)
	}
	switch result {
	case "OK":
		return nil
	case "CALLER", "GLOBAL":
		return errQuota
	default:
		return ErrUnavailable
	}
}

// Commit atomically activates a successfully delivered reservation.
func (s *Redis) Commit(ctx context.Context, reservation *Reservation) error {
	if !s.validReservation(reservation) || ctx == nil {
		return ErrInvalid
	}
	var result string
	err := redisbridge.Use(ctx, s.scope, reservation.group, func(lease redisbridge.Lease) error {
		reply, runErr := lease.Run(ctx, redisbridge.ChallengeCommitScript(), []redisbridge.Key{reservation.stateKey, reservation.opsKey},
			reservation.version,
			int64(stateExpiryGrace/time.Millisecond),
		)
		if runErr != nil {
			return runErr
		}
		result, runErr = reply.Text()
		return runErr
	})
	if err != nil {
		return storeError(ctx)
	}
	switch result {
	case "OK":
		return nil
	case "EXPIRED", "STALE":
		return ErrStale
	default:
		return ErrUnavailable
	}
}

// Abort atomically compensates a failed delivery. It is idempotent for the
// same reservation and cannot delete a newer pending reservation.
func (s *Redis) Abort(ctx context.Context, reservation *Reservation) error {
	if !s.validReservation(reservation) || ctx == nil {
		return ErrInvalid
	}
	var result string
	err := redisbridge.Use(ctx, s.scope, reservation.group, func(lease redisbridge.Lease) error {
		reply, runErr := lease.Run(ctx, redisbridge.ChallengeAbortScript(), []redisbridge.Key{reservation.stateKey}, reservation.version)
		if runErr != nil {
			return runErr
		}
		result, runErr = reply.Text()
		return runErr
	})
	if err != nil {
		return storeError(ctx)
	}
	switch result {
	case "OK":
		return nil
	case "STALE":
		return ErrStale
	default:
		return ErrUnavailable
	}
}

// Verify performs anti-enumerating, exactly-once verification with a bounded
// attempt counter. Invalid, missing, expired, locked, and stale states all
// produce VerifyRejected without distinguishing whether subject state exists.
func (s *Redis) Verify(ctx context.Context, request VerifyRequest) (VerifyOutcome, error) {
	if s == nil || s.scope == nil || ctx == nil {
		return VerifyRejected, ErrUnavailable
	}
	normalizedSubject, normalizedPurpose, err := normalizeSubject(request.Subject, request.Purpose)
	if err != nil || !validCode(request.Code) {
		return VerifyRejected, nil
	}
	subjectFingerprint := fingerprint(s.subjectKey, "mss/challenge-key/v1", normalizedPurpose, normalizedSubject)
	group, stateKey, _, opsKey, err := subjectCapabilities(subjectFingerprint)
	if err != nil {
		return VerifyRejected, ErrUnavailable
	}

	values, err := s.verifier.Read(ctx, s, group, stateKey)
	if err != nil {
		return VerifyRejected, err
	}
	if len(values) != 3 {
		return VerifyRejected, ErrUnavailable
	}
	if values[0] == "" {
		if err = s.completeDummyVerify(ctx, group, stateKey, opsKey, subjectFingerprint, request.Code); err != nil {
			return VerifyRejected, err
		}
		return VerifyRejected, nil
	}
	activeID, digestHex, pepperVersion := values[0], values[1], values[2]
	pepper, exists := s.peppers[pepperVersion]
	storedDigest, decodeErr := hex.DecodeString(digestHex)
	if decodeErr != nil || len(storedDigest) != sha256.Size || !exists {
		if err = s.completeDummyVerify(ctx, group, stateKey, opsKey, subjectFingerprint, request.Code); err != nil {
			return VerifyRejected, err
		}
		return VerifyRejected, ErrUnavailable
	}
	candidate := digest(pepper, subjectFingerprint, activeID, request.Code)
	matched := constantTimeEqual(storedDigest, candidate)
	result, err := s.verifier.Complete(ctx, s, group, stateKey, opsKey, activeID, digestHex, matched)
	if err != nil {
		return VerifyRejected, err
	}
	switch result {
	case "SUCCESS":
		return VerifyVerified, nil
	case "INVALID", "LOCKED", "EXPIRED", "MISSING", "STALE":
		return VerifyRejected, nil
	default:
		return VerifyRejected, ErrUnavailable
	}
}

func (redisVerifyExecutor) Read(
	ctx context.Context,
	store *Redis,
	group redisbridge.AtomicGroup,
	stateKey redisbridge.Key,
) ([]string, error) {
	var values []string
	err := redisbridge.Use(ctx, store.scope, group, func(lease redisbridge.Lease) error {
		reply, runErr := lease.Run(ctx, redisbridge.ChallengeReadVerifierScript(), []redisbridge.Key{stateKey})
		if runErr != nil {
			return runErr
		}
		values, runErr = reply.Strings()
		return runErr
	})
	if err != nil {
		return nil, storeError(ctx)
	}
	return values, nil
}

func (s *Redis) completeDummyVerify(
	ctx context.Context,
	group redisbridge.AtomicGroup,
	stateKey, opsKey redisbridge.Key,
	subjectFingerprint, code string,
) error {
	dummyDigest := make([]byte, sha256.Size)
	candidate := digest(make([]byte, sha256.Size), subjectFingerprint, "!", code)
	_ = constantTimeEqual(dummyDigest, candidate)
	result, err := s.verifier.Complete(ctx, s, group, stateKey, opsKey, "!", hex.EncodeToString(dummyDigest), false)
	if err != nil {
		return err
	}
	if result != "STALE" {
		return ErrUnavailable
	}
	return nil
}

func (redisVerifyExecutor) Complete(
	ctx context.Context,
	store *Redis,
	group redisbridge.AtomicGroup,
	stateKey, opsKey redisbridge.Key,
	expectedID, expectedDigest string,
	matched bool,
) (string, error) {
	operationID, err := randomID(store.random)
	if err != nil {
		return "", ErrUnavailable
	}
	var result string
	err = redisbridge.Use(ctx, store.scope, group, func(lease redisbridge.Lease) error {
		reply, runErr := lease.Run(ctx, redisbridge.ChallengeCompleteVerifyScript(), []redisbridge.Key{stateKey, opsKey},
			expectedID,
			expectedDigest,
			operationID,
			store.maxAttempts,
			boolInt(matched),
			int64(store.idempotencyTTL/time.Millisecond),
		)
		if runErr != nil {
			return runErr
		}
		result, runErr = reply.Text()
		return runErr
	})
	if err != nil {
		return "", storeError(ctx)
	}
	return result, nil
}

func subjectCapabilities(subjectFingerprint string) (redisbridge.AtomicGroup, redisbridge.Key, redisbridge.Key, redisbridge.Key, error) {
	group, err := redisbridge.NewAtomicGroup("challenge-subject", []byte(subjectFingerprint))
	if err != nil {
		return redisbridge.AtomicGroup{}, redisbridge.Key{}, redisbridge.Key{}, redisbridge.Key{}, err
	}
	stateKey, err := group.Key("state")
	if err != nil {
		return redisbridge.AtomicGroup{}, redisbridge.Key{}, redisbridge.Key{}, redisbridge.Key{}, err
	}
	quotaKey, err := group.Key("quota")
	if err != nil {
		return redisbridge.AtomicGroup{}, redisbridge.Key{}, redisbridge.Key{}, redisbridge.Key{}, err
	}
	opsKey, err := group.Key("ops")
	return group, stateKey, quotaKey, opsKey, err
}

func (s *Redis) validReservation(reservation *Reservation) bool {
	return s != nil && s.scope != nil && s.owner != nil && reservation != nil &&
		reservation.owner == s.owner && reservation.version != "" && reservation.code != ""
}

func constantTimeEqual(stored, candidate []byte) bool { return hmac.Equal(stored, candidate) }

func defaultOptions(options Options) Options {
	if options.CodeTTL == 0 {
		options.CodeTTL = defaultCodeTTL
	}
	if options.PendingTTL == 0 {
		options.PendingTTL = defaultPendingTTL
	}
	if options.Cooldown == 0 {
		options.Cooldown = defaultCooldown
	}
	if options.QuotaWindow == 0 {
		options.QuotaWindow = defaultQuotaWindow
	}
	if options.MaxIssues == 0 {
		options.MaxIssues = defaultMaxIssues
	}
	if options.MaxAttempts == 0 {
		options.MaxAttempts = defaultMaxAttempts
	}
	if options.IdempotencyTTL == 0 {
		options.IdempotencyTTL = defaultIdempotencyTTL
	}
	if options.CallerWindow == 0 {
		options.CallerWindow = defaultCallerWindow
	}
	if options.CallerLimit == 0 {
		options.CallerLimit = defaultCallerLimit
	}
	if options.GlobalWindow == 0 {
		options.GlobalWindow = defaultGlobalWindow
	}
	if options.GlobalLimit == 0 {
		options.GlobalLimit = defaultGlobalLimit
	}
	return options
}

func validateOptions(options Options) error {
	if !plausibleSecret(options.SubjectKey) || len(options.Peppers) == 0 || len(options.Peppers) > 2 {
		return ErrInvalid
	}
	versions := make(map[string]struct{}, len(options.Peppers))
	for _, pepper := range options.Peppers {
		if !purposePattern.MatchString(pepper.Version) || !plausibleSecret(pepper.Secret) || hmac.Equal(options.SubjectKey, pepper.Secret) {
			return ErrInvalid
		}
		if _, exists := versions[pepper.Version]; exists {
			return ErrInvalid
		}
		versions[pepper.Version] = struct{}{}
	}
	if len(options.Peppers) == 2 && hmac.Equal(options.Peppers[0].Secret, options.Peppers[1].Secret) {
		return ErrInvalid
	}
	durations := []time.Duration{options.CodeTTL, options.PendingTTL, options.Cooldown, options.QuotaWindow, options.IdempotencyTTL, options.CallerWindow, options.GlobalWindow}
	for _, duration := range durations {
		if duration < time.Millisecond || duration%time.Millisecond != 0 {
			return ErrInvalid
		}
	}
	if options.PendingTTL >= options.CodeTTL || options.MaxIssues <= 0 || options.MaxAttempts <= 0 || options.CallerLimit <= 0 || options.GlobalLimit <= 0 {
		return ErrInvalid
	}
	return nil
}

func plausibleSecret(secret []byte) bool {
	if len(secret) < minimumSecretBytes {
		return false
	}
	unique := make(map[byte]struct{}, len(secret))
	for _, value := range secret {
		unique[value] = struct{}{}
	}
	return len(unique) >= 16
}

func randomCode(reader io.Reader) (string, error) {
	value, err := cryptorand.Int(reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func randomID(reader io.Reader) (string, error) {
	value := make([]byte, 24)
	if _, err := io.ReadFull(reader, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func fingerprint(secret []byte, domain, first, second string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = io.WriteString(mac, domain)
	_, _ = mac.Write([]byte{0})
	_, _ = io.WriteString(mac, first)
	_, _ = mac.Write([]byte{0})
	_, _ = io.WriteString(mac, second)
	return hex.EncodeToString(mac.Sum(nil))
}

func normalizeSubject(subject string, purpose Purpose) (string, string, error) {
	subject = strings.TrimSpace(subject)
	normalizedPurpose := strings.ToLower(strings.TrimSpace(string(purpose)))
	if subject == "" || len(subject) > 4096 || !purposePattern.MatchString(normalizedPurpose) {
		return "", "", ErrInvalid
	}
	return subject, normalizedPurpose, nil
}

func storeError(ctx context.Context) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Join(ErrUnavailable, err)
		}
	}
	return ErrUnavailable
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func validCode(code string) bool {
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

func digest(pepper []byte, fingerprint, issueID, code string) []byte {
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
