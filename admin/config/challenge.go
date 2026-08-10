package config

import (
	"encoding/base64"
	"errors"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	"github.com/redis/go-redis/v9"
)

var challengeSecretEnvironmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var challengePepperVersion = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

var (
	ErrChallengeConfigurationInvalid  = errors.New("challenge configuration is invalid")
	ErrChallengeDependencyUnavailable = errors.New("challenge dependency is unavailable")
)

type SecretRef string

type ChallengePepperRef struct {
	Version   string    `yaml:"version" json:"version"`
	SecretRef SecretRef `yaml:"secretRef" json:"secretRef"`
}

// Challenge is the D0 internal provisional Admin composition config. Only typed
// references are decoded from YAML; raw challenge key material is never stored
// in Config or exposed through application configuration.
type Challenge struct {
	Enabled          bool                `yaml:"enabled" json:"enabled"`
	KeySecretRef     SecretRef           `yaml:"keySecretRef" json:"keySecretRef"`
	CurrentPepper    ChallengePepperRef  `yaml:"currentPepper" json:"currentPepper"`
	PreviousPepper   *ChallengePepperRef `yaml:"previousPepper" json:"previousPepper"`
	ActiveTTL        time.Duration       `yaml:"activeTTL" json:"activeTTL"`
	PendingLease     time.Duration       `yaml:"pendingLease" json:"pendingLease"`
	ResendCooldown   time.Duration       `yaml:"resendCooldown" json:"resendCooldown"`
	IssueWindow      time.Duration       `yaml:"issueWindow" json:"issueWindow"`
	IssueLimit       int                 `yaml:"issueLimit" json:"issueLimit"`
	MaxAttempts      int                 `yaml:"maxAttempts" json:"maxAttempts"`
	IdempotencyLease time.Duration       `yaml:"idempotencyLease" json:"idempotencyLease"`
	CallerWindow     time.Duration       `yaml:"callerWindow" json:"callerWindow"`
	CallerLimit      int                 `yaml:"callerLimit" json:"callerLimit"`
	GlobalWindow     time.Duration       `yaml:"globalWindow" json:"globalWindow"`
	GlobalLimit      int                 `yaml:"globalLimit" json:"globalLimit"`
}

// Validate checks the side-effect-free configuration contract even when the
// optional Redis resource is absent. SecretRef resolution remains a runtime
// dependency check so an unavailable secret backend can disable only the
// challenge capability rather than the whole Admin process.
func (c Challenge) Validate() error {
	if !c.Enabled {
		return nil
	}
	if err := c.validateReferences(); err != nil {
		return ErrChallengeConfigurationInvalid
	}
	for _, duration := range []time.Duration{
		c.ActiveTTL,
		c.PendingLease,
		c.ResendCooldown,
		c.IssueWindow,
		c.IdempotencyLease,
		c.CallerWindow,
		c.GlobalWindow,
	} {
		if duration != 0 && (duration < time.Millisecond || duration%time.Millisecond != 0) {
			return ErrChallengeConfigurationInvalid
		}
	}
	if c.IssueLimit < 0 || c.MaxAttempts < 0 || c.CallerLimit < 0 || c.GlobalLimit < 0 {
		return ErrChallengeConfigurationInvalid
	}
	return nil
}

func (c Challenge) Build(client redis.UniversalClient) (cache.ProvisionalChallenge, error) {
	if !c.Enabled {
		return nil, nil
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrChallengeDependencyUnavailable
	}
	keySecret, err := c.KeySecretRef.resolve()
	if err != nil {
		return nil, ErrChallengeDependencyUnavailable
	}
	defer zeroBytes(keySecret)
	currentSecret, err := c.CurrentPepper.SecretRef.resolve()
	if err != nil {
		return nil, ErrChallengeDependencyUnavailable
	}
	defer zeroBytes(currentSecret)
	peppers := []cache.ChallengePepper{{
		Version: c.CurrentPepper.Version,
		Secret:  currentSecret,
	}}
	if c.PreviousPepper != nil {
		previousSecret, resolveErr := c.PreviousPepper.SecretRef.resolve()
		if resolveErr != nil {
			return nil, ErrChallengeDependencyUnavailable
		}
		defer zeroBytes(previousSecret)
		peppers = append(peppers, cache.ChallengePepper{
			Version: c.PreviousPepper.Version,
			Secret:  previousSecret,
		})
	}
	store, err := cache.NewRedisChallengeStore(client, cache.ChallengeOptions{
		SubjectKey:     keySecret,
		Peppers:        peppers,
		CodeTTL:        c.ActiveTTL,
		PendingTTL:     c.PendingLease,
		Cooldown:       c.ResendCooldown,
		QuotaWindow:    c.IssueWindow,
		MaxIssues:      c.IssueLimit,
		MaxAttempts:    c.MaxAttempts,
		IdempotencyTTL: c.IdempotencyLease,
		CallerWindow:   c.CallerWindow,
		CallerLimit:    c.CallerLimit,
		GlobalWindow:   c.GlobalWindow,
		GlobalLimit:    c.GlobalLimit,
	})
	if err != nil {
		if errors.Is(err, cache.ErrChallengeUnavailable) {
			return nil, ErrChallengeDependencyUnavailable
		}
		return nil, ErrChallengeConfigurationInvalid
	}
	return store, nil
}

func (c Challenge) validateReferences() error {
	if err := c.KeySecretRef.validate(); err != nil {
		return err
	}
	if !challengePepperVersion.MatchString(strings.TrimSpace(c.CurrentPepper.Version)) {
		return errors.New("current challenge pepper version is required")
	}
	if err := c.CurrentPepper.SecretRef.validate(); err != nil {
		return err
	}
	if c.PreviousPepper != nil {
		if !challengePepperVersion.MatchString(strings.TrimSpace(c.PreviousPepper.Version)) || c.PreviousPepper.Version == c.CurrentPepper.Version {
			return errors.New("challenge pepper versions must be distinct")
		}
		if err := c.PreviousPepper.SecretRef.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (r SecretRef) validate() error {
	const prefix = "env://"
	value := strings.TrimSpace(string(r))
	if !strings.HasPrefix(value, prefix) || !challengeSecretEnvironmentName.MatchString(strings.TrimPrefix(value, prefix)) {
		return errors.New("invalid environment SecretRef")
	}
	return nil
}

func (r SecretRef) resolve() ([]byte, error) {
	const prefix = "env://"
	value := strings.TrimSpace(string(r))
	if err := r.validate(); err != nil {
		return nil, err
	}
	name := strings.TrimPrefix(value, prefix)
	if !challengeSecretEnvironmentName.MatchString(name) {
		return nil, errors.New("invalid environment SecretRef")
	}
	encoded, exists := os.LookupEnv(name)
	if !exists || strings.TrimSpace(encoded) == "" {
		return nil, errors.New("SecretRef value is missing")
	}
	secret, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(secret) < 32 {
		return nil, errors.New("SecretRef must contain at least 32 base64-encoded bytes")
	}
	return secret, nil
}

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
