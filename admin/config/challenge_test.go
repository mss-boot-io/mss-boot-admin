package config

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestChallengeConfigRequiresTypedHighEntropySecretRefs(t *testing.T) {
	locator := sha256.Sum256([]byte("independent challenge locator test vector"))
	pepper := sha256.Sum256([]byte("independent challenge verifier test vector"))
	t.Setenv("MSS_TEST_CHALLENGE_KEY", base64.StdEncoding.EncodeToString(locator[:]))
	t.Setenv("MSS_TEST_CHALLENGE_PEPPER", base64.StdEncoding.EncodeToString(pepper[:]))
	server := miniredis.RunT(t)
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{server.Addr()}})
	t.Cleanup(func() { _ = client.Close() })

	base := Challenge{
		Enabled:      true,
		KeySecretRef: "env://MSS_TEST_CHALLENGE_KEY",
		CurrentPepper: ChallengePepperRef{
			Version:   "v1",
			SecretRef: "env://MSS_TEST_CHALLENGE_PEPPER",
		},
	}
	if challenge, err := base.Build(client); err != nil || challenge == nil {
		t.Fatalf("valid challenge Build = %#v, %v; want non-nil, nil", challenge, err)
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid challenge Validate = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Challenge)
	}{
		{name: "raw secret", mutate: func(config *Challenge) { config.KeySecretRef = "plaintext-secret" }},
		{name: "missing env", mutate: func(config *Challenge) { config.KeySecretRef = "env://MSS_TEST_MISSING" }},
		{name: "short secret", mutate: func(config *Challenge) {
			t.Setenv("MSS_TEST_SHORT", base64.StdEncoding.EncodeToString([]byte("short")))
			config.KeySecretRef = "env://MSS_TEST_SHORT"
		}},
		{name: "malformed base64", mutate: func(config *Challenge) {
			t.Setenv("MSS_TEST_MALFORMED", "not-base64")
			config.KeySecretRef = "env://MSS_TEST_MALFORMED"
		}},
		{name: "obviously weak secret", mutate: func(config *Challenge) {
			t.Setenv("MSS_TEST_WEAK", base64.StdEncoding.EncodeToString(make([]byte, 32)))
			config.KeySecretRef = "env://MSS_TEST_WEAK"
		}},
		{name: "shared locator and pepper secret", mutate: func(config *Challenge) {
			config.CurrentPepper.SecretRef = config.KeySecretRef
		}},
		{name: "duplicate pepper version", mutate: func(config *Challenge) {
			config.PreviousPepper = &ChallengePepperRef{
				Version: "v1", SecretRef: "env://MSS_TEST_CHALLENGE_PEPPER",
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := base
			test.mutate(&candidate)
			if challenge, err := candidate.Build(client); err == nil || challenge != nil {
				t.Fatalf("invalid challenge Build = %#v, %v; want nil, error", challenge, err)
			}
		})
	}
	if challenge, err := (Challenge{}).Build(client); err != nil || challenge != nil {
		t.Fatalf("disabled challenge Build = %#v, %v; want nil, nil", challenge, err)
	}
	if challenge, err := base.Build(nil); err == nil || challenge != nil {
		t.Fatalf("nil Redis challenge Build = %#v, %v; want nil, error", challenge, err)
	}
	invalidStatic := base
	invalidStatic.ActiveTTL = time.Microsecond
	if err := invalidStatic.Validate(); !errors.Is(err, ErrChallengeConfigurationInvalid) {
		t.Fatalf("sub-millisecond static validation = %v, want configuration invalid", err)
	}
	missingDependency := base
	missingDependency.KeySecretRef = "env://MSS_TEST_MISSING"
	if err := missingDependency.Validate(); err != nil {
		t.Fatalf("unresolved SecretRef is a dependency check, static validation = %v", err)
	}
}
