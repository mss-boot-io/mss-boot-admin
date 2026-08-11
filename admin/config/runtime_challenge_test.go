package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
)

func TestChallengeRuntimeStartsAndReadiesBeforePublicationThenCloses(t *testing.T) {
	server := miniredis.RunT(t)
	configuration := newRuntimeChallengeTestConfig(t, server.Addr())
	previous := center.GetRuntimeChallenge()
	center.SetRuntimeChallenge(nil)
	t.Cleanup(func() { center.SetRuntimeChallenge(previous) })

	owner, degraded, err := configuration.prepareChallengeRuntime(t.Context())
	if err != nil || degraded || owner == nil || owner.challenge == nil {
		t.Fatalf("prepare challenge runtime = %#v, degraded=%v, err=%v", owner, degraded, err)
	}
	if center.GetRuntimeChallenge() != nil {
		t.Fatal("challenge capability published before candidate commit")
	}
	if err := owner.challenge.Ready(t.Context()); err != nil {
		t.Fatalf("prepared challenge scope is not ready: %v", err)
	}
	if err := configuration.replaceChallengeRuntime(t.Context(), owner); err != nil {
		t.Fatalf("publish challenge runtime: %v", err)
	}
	if center.GetRuntimeChallenge() != owner.challenge {
		t.Fatal("ready challenge capability was not published")
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := configuration.closeChallengeRuntime(closeCtx); err != nil {
		t.Fatalf("close challenge runtime: %v", err)
	}
	if center.GetRuntimeChallenge() != nil || configuration.hasChallengeRuntimeOwner() {
		t.Fatal("challenge runtime remained published or owned after close")
	}
	if err := owner.challenge.Ready(t.Context()); err == nil {
		t.Fatal("closed challenge runtime still reports ready")
	}
}

func TestOptionalChallengeRuntimeInvalidOrUnavailableDegradesWithoutLegacyFallback(t *testing.T) {
	previousRuntime := center.GetRuntimeChallenge()
	previousLegacy := center.GetChallenge()
	center.SetRuntimeChallenge(nil)
	center.SetChallenge(nil)
	t.Cleanup(func() {
		center.SetRuntimeChallenge(previousRuntime)
		center.SetChallenge(previousLegacy)
	})

	invalid := newRuntimeChallengeTestConfig(t, "127.0.0.1:6379")
	invalid.Challenge.ResourceRef = "missing"
	owner, degraded, err := invalid.prepareChallengeRuntime(t.Context())
	if err != nil || !degraded || owner != nil {
		t.Fatalf("optional invalid runtime = %#v, degraded=%v, err=%v", owner, degraded, err)
	}
	if center.GetRuntimeChallenge() != nil {
		t.Fatal("optional invalid runtime published a challenge capability")
	}

	unavailable := newRuntimeChallengeTestConfig(t, "127.0.0.1:1")
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	owner, degraded, err = unavailable.prepareChallengeRuntime(ctx)
	if err != nil || !degraded || owner != nil {
		t.Fatalf("optional unavailable runtime = %#v, degraded=%v, err=%v", owner, degraded, err)
	}
	if center.GetRuntimeChallenge() != nil {
		t.Fatal("optional unavailable runtime fell back to a legacy challenge")
	}

	invalid.Challenge.Required = true
	if owner, degraded, err = invalid.prepareChallengeRuntime(t.Context()); err == nil || degraded || owner != nil || !errors.Is(err, ErrChallengeConfigurationInvalid) {
		t.Fatalf("required invalid runtime = %#v, degraded=%v, err=%v", owner, degraded, err)
	}
}

func newRuntimeChallengeTestConfig(t *testing.T, endpoint string) *Config {
	t.Helper()
	locator := sha256.Sum256([]byte(t.Name() + ":locator"))
	pepper := sha256.Sum256([]byte(t.Name() + ":pepper"))
	t.Setenv("MSS_TEST_RUNTIME_CHALLENGE_KEY", base64.StdEncoding.EncodeToString(locator[:]))
	t.Setenv("MSS_TEST_RUNTIME_CHALLENGE_PEPPER", base64.StdEncoding.EncodeToString(pepper[:]))
	shortTimeout, err := runtimeconfig.ParseDuration("50ms")
	if err != nil {
		t.Fatal(err)
	}
	return &Config{
		Runtime: runtimeconfig.Config{Resources: map[string]runtimeconfig.ResourceConfig{
			"main": {
				Provider: runtimeconfig.ProviderConfig{
					Kind: runtimeconfig.ProviderRedis,
					Redis: &runtimeconfig.RedisConfig{
						Mode:         runtimeconfig.RedisStandalone,
						Standalone:   &runtimeconfig.RedisStandaloneConfig{Endpoint: endpoint},
						DialTimeout:  shortTimeout,
						ReadTimeout:  shortTimeout,
						WriteTimeout: shortTimeout,
						Credentials: runtimeconfig.RedisCredentialsConfig{
							Kind:      runtimeconfig.RedisCredentialsAnonymous,
							Anonymous: &runtimeconfig.RedisAnonymousCredentialsConfig{},
						},
					},
				},
			},
		}},
		Challenge: Challenge{
			Enabled:      true,
			ResourceRef:  "main",
			KeySecretRef: "env://MSS_TEST_RUNTIME_CHALLENGE_KEY",
			CurrentPepper: ChallengePepperRef{
				Version:   "v1",
				SecretRef: "env://MSS_TEST_RUNTIME_CHALLENGE_PEPPER",
			},
		},
	}
}
