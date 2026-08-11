package config_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
	"gopkg.in/yaml.v3"
)

const (
	passwordReference = "env://RUNTIME_CONFIG_TEST_PASSWORD"
	usernameReference = "env://RUNTIME_CONFIG_TEST_USERNAME"
	caReference       = "env://RUNTIME_CONFIG_TEST_CA"
	certReference     = "env://RUNTIME_CONFIG_TEST_CERT"
	keyReference      = "env://RUNTIME_CONFIG_TEST_KEY"

	passwordCanary = "runtime-config-password-canary-9eea7657"
	usernameCanary = "runtime-config-username-canary-9eea7657"
)

type countingResolver struct {
	values map[string]string
	calls  atomic.Int64
}

func (r *countingResolver) Resolve(ctx context.Context, ref runtimeconfig.SecretRef) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r.calls.Add(1)
	value, ok := r.values[ref.Reference()]
	if !ok {
		return "", errors.New("resolver diagnostic contains a secret canary: " + passwordCanary)
	}
	return value, nil
}

func TestStrictDecodeRejectsUnknownFieldsAndMalformedDocuments(t *testing.T) {
	t.Parallel()

	validYAML := []byte(`resources:
  main:
    provider:
      kind: redis
      redis:
        mode: standalone
        standalone:
          endpoint: redis.example.test:6379
        credentials:
          kind: password
          password:
            passwordRef: env://RUNTIME_CONFIG_TEST_PASSWORD
        dialTimeout: 2s
`)
	if _, err := runtimeconfig.DecodeYAML(validYAML); err != nil {
		t.Fatalf("DecodeYAML(valid) error = %v", err)
	}

	validJSON := []byte(`{
  "resources": {
    "main": {
      "provider": {
        "kind": "redis",
        "redis": {
          "mode": "cluster",
          "cluster": {"endpoints": ["redis-a.example.test:6379"]},
          "credentials": {"kind": "anonymous", "anonymous": {}},
          "readTimeout": "3s"
        }
      }
    }
  }
}`)
	if _, err := runtimeconfig.DecodeJSON(validJSON); err != nil {
		t.Fatalf("DecodeJSON(valid) error = %v", err)
	}

	tests := []struct {
		name   string
		format runtimeconfig.Format
		input  string
	}{
		{name: "unknown top-level YAML field", format: runtimeconfig.FormatYAML, input: "runtime: {}\n"},
		{name: "unknown nested YAML field", format: runtimeconfig.FormatYAML, input: strings.Replace(string(validYAML), "        dialTimeout: 2s", "        poolNum: 8", 1)},
		{name: "numeric YAML duration", format: runtimeconfig.FormatYAML, input: strings.Replace(string(validYAML), "dialTimeout: 2s", "dialTimeout: 2000000000", 1)},
		{name: "zero YAML duration", format: runtimeconfig.FormatYAML, input: strings.Replace(string(validYAML), "dialTimeout: 2s", "dialTimeout: 0s", 1)},
		{name: "negative YAML duration", format: runtimeconfig.FormatYAML, input: strings.Replace(string(validYAML), "dialTimeout: 2s", "dialTimeout: -1s", 1)},
		{name: "overflow YAML duration", format: runtimeconfig.FormatYAML, input: strings.Replace(string(validYAML), "dialTimeout: 2s", "dialTimeout: 999999999999999999h", 1)},
		{name: "raw password", format: runtimeconfig.FormatYAML, input: strings.Replace(string(validYAML), passwordReference, passwordCanary, 1)},
		{name: "YAML alias", format: runtimeconfig.FormatYAML, input: "resources: &resources {}\ncopy: *resources\n"},
		{name: "duplicate YAML field", format: runtimeconfig.FormatYAML, input: "resources: {}\nresources: {}\n"},
		{name: "multiple YAML documents", format: runtimeconfig.FormatYAML, input: "resources: {}\n---\nresources: {}\n"},
		{name: "unknown JSON field", format: runtimeconfig.FormatJSON, input: strings.Replace(string(validJSON), `"readTimeout": "3s"`, `"poolNum": 8`, 1)},
		{name: "numeric JSON duration", format: runtimeconfig.FormatJSON, input: strings.Replace(string(validJSON), `"readTimeout": "3s"`, `"readTimeout": 3000000000`, 1)},
		{name: "duplicate JSON field", format: runtimeconfig.FormatJSON, input: `{"resources":{},"resources":{}}`},
		{name: "multiple JSON values", format: runtimeconfig.FormatJSON, input: `{"resources":{}} {"resources":{}}`},
		{name: "JSON array root", format: runtimeconfig.FormatJSON, input: `[]`},
		{name: "JSON null root", format: runtimeconfig.FormatJSON, input: `null`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var err error
			switch test.format {
			case runtimeconfig.FormatYAML:
				_, err = runtimeconfig.DecodeYAML([]byte(test.input))
			case runtimeconfig.FormatJSON:
				_, err = runtimeconfig.DecodeJSON([]byte(test.input))
			default:
				t.Fatalf("unexpected test format %q", test.format)
			}
			if !errors.Is(err, runtimeconfig.ErrInvalidConfiguration) {
				t.Fatalf("Decode() error = %v, want ErrInvalidConfiguration", err)
			}
			if strings.Contains(fmt.Sprint(err), passwordCanary) {
				t.Fatalf("Decode() error exposed rejected value: %v", err)
			}
		})
	}

	if _, err := runtimeconfig.Decode(strings.NewReader("{}"), runtimeconfig.Format("toml")); !errors.Is(err, runtimeconfig.ErrInvalidConfiguration) {
		t.Fatalf("Decode(unknown format) error = %v, want ErrInvalidConfiguration", err)
	}
	if _, err := runtimeconfig.DecodeJSON(make([]byte, 1<<20+1)); !errors.Is(err, runtimeconfig.ErrInvalidConfiguration) {
		t.Fatalf("DecodeJSON(oversize) error = %v, want ErrInvalidConfiguration", err)
	}
}

func TestStrictDecodeRejectsCaseVariantFieldsBeforeSecretResolution(t *testing.T) {
	t.Parallel()

	const (
		safeReference   = "env://RUNTIME_CONFIG_CASE_SAFE_PASSWORD"
		shadowReference = "env://RUNTIME_CONFIG_CASE_SHADOW_PASSWORD"
		validYAML       = `resources:
  main:
    provider:
      kind: redis
      redis:
        mode: standalone
        standalone:
          endpoint: redis.example.test:6379
        credentials:
          kind: password
          password:
            passwordRef: env://RUNTIME_CONFIG_CASE_SAFE_PASSWORD
`
		validJSON = `{
  "resources": {
    "main": {
      "provider": {
        "kind": "redis",
        "redis": {
          "mode": "standalone",
          "standalone": {"endpoint": "redis.example.test:6379"},
          "credentials": {
            "kind": "password",
            "password": {"passwordRef": "env://RUNTIME_CONFIG_CASE_SAFE_PASSWORD"}
          }
        }
      }
    }
  }
}`
	)
	for _, baseline := range []struct {
		name   string
		format runtimeconfig.Format
		input  string
	}{
		{name: "YAML baseline", format: runtimeconfig.FormatYAML, input: validYAML},
		{name: "JSON baseline", format: runtimeconfig.FormatJSON, input: validJSON},
	} {
		t.Run(baseline.name, func(t *testing.T) {
			resolver := &countingResolver{values: map[string]string{safeReference: passwordCanary}}
			config, err := runtimeconfig.Decode(strings.NewReader(baseline.input), baseline.format)
			if err != nil {
				t.Fatalf("Decode(valid baseline) error = %v", err)
			}
			if _, err = config.Normalize(context.Background(), resolver); err != nil {
				t.Fatalf("Normalize(valid baseline) error = %v", err)
			}
			if calls := resolver.calls.Load(); calls != 1 {
				t.Fatalf("valid baseline resolver calls = %d, want one", calls)
			}
		})
	}

	tests := []struct {
		name   string
		format runtimeconfig.Format
		input  string
	}{
		{
			name:   "YAML top-level case variant",
			format: runtimeconfig.FormatYAML,
			input:  strings.Replace(validYAML, "resources:", "Resources:", 1),
		},
		{
			name:   "YAML top-level semantic duplicate",
			format: runtimeconfig.FormatYAML,
			input:  validYAML + "Resources: {}\n",
		},
		{
			name:   "YAML nested case variant",
			format: runtimeconfig.FormatYAML,
			input:  strings.Replace(validYAML, "passwordRef:", "PasswordRef:", 1),
		},
		{
			name:   "YAML nested semantic duplicate",
			format: runtimeconfig.FormatYAML,
			input: strings.Replace(
				validYAML,
				"            passwordRef: "+safeReference,
				"            passwordRef: "+safeReference+"\n            PasswordRef: "+shadowReference,
				1,
			),
		},
		{
			name:   "JSON top-level case variant",
			format: runtimeconfig.FormatJSON,
			input:  strings.Replace(validJSON, `"resources"`, `"Resources"`, 1),
		},
		{
			name:   "JSON top-level semantic duplicate",
			format: runtimeconfig.FormatJSON,
			input:  strings.Replace(validJSON, "{", `{"Resources":{},`, 1),
		},
		{
			name:   "JSON nested case variant",
			format: runtimeconfig.FormatJSON,
			input:  strings.Replace(validJSON, `"passwordRef"`, `"PasswordRef"`, 1),
		},
		{
			name:   "JSON nested semantic duplicate",
			format: runtimeconfig.FormatJSON,
			input: strings.Replace(
				validJSON,
				`"passwordRef": "`+safeReference+`"`,
				`"passwordRef": "`+safeReference+`", "PasswordRef": "`+shadowReference+`"`,
				1,
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolver := &countingResolver{values: map[string]string{
				safeReference:   passwordCanary,
				shadowReference: passwordCanary,
			}}
			config, err := runtimeconfig.Decode(strings.NewReader(test.input), test.format)
			if err == nil {
				_, err = config.Normalize(context.Background(), resolver)
			}
			if !errors.Is(err, runtimeconfig.ErrInvalidConfiguration) {
				t.Fatalf("Decode/Normalize() error = %v, want ErrInvalidConfiguration", err)
			}
			if calls := resolver.calls.Load(); calls != 0 {
				t.Fatalf("resolver calls = %d, want zero before rejecting a non-canonical field", calls)
			}
		})
	}
}

func TestRuntimeStrictConfigurationNegativeMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config func(t *testing.T) runtimeconfig.Config
	}{
		{name: "invalid resource name", config: mutateValidConfig(func(_ *testing.T, config *runtimeconfig.Config) {
			config.Resources["Main"] = config.Resources["main"]
			delete(config.Resources, "main")
		})},
		{name: "missing provider kind", config: mutateValidConfig(func(_ *testing.T, config *runtimeconfig.Config) {
			resource := config.Resources["main"]
			resource.Provider.Kind = ""
			config.Resources["main"] = resource
		})},
		{name: "unknown provider", config: mutateValidConfig(func(_ *testing.T, config *runtimeconfig.Config) {
			resource := config.Resources["main"]
			resource.Provider.Kind = runtimeconfig.ProviderKind("pulsar")
			config.Resources["main"] = resource
		})},
		{name: "missing provider branch", config: mutateValidConfig(func(_ *testing.T, config *runtimeconfig.Config) {
			resource := config.Resources["main"]
			resource.Provider.Redis = nil
			config.Resources["main"] = resource
		})},
		{name: "unknown mode", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Mode = runtimeconfig.RedisMode("ring")
		})},
		{name: "missing mode branch", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Standalone = nil
		})},
		{name: "multiple mode branches", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Cluster = &runtimeconfig.RedisClusterConfig{Endpoints: []string{"redis-b.example.test:6379"}}
		})},
		{name: "mode branch mismatch", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Mode = runtimeconfig.RedisSentinel
		})},
		{name: "empty standalone endpoint", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Standalone.Endpoint = ""
		})},
		{name: "endpoint without port", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Standalone.Endpoint = "redis.example.test"
		})},
		{name: "endpoint URL", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Standalone.Endpoint = "redis://redis.example.test:6379"
		})},
		{name: "endpoint userinfo", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Standalone.Endpoint = "user@redis.example.test:6379"
		})},
		{name: "endpoint zero port", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Standalone.Endpoint = "redis.example.test:0"
		})},
		{name: "endpoint wildcard address", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Standalone.Endpoint = "0.0.0.0:6379"
		})},
		{name: "duplicate sentinel endpoint", config: func(t *testing.T) runtimeconfig.Config {
			config := validSentinelConfig(t)
			redis := config.Resources["main"].Provider.Redis
			redis.Sentinel.Endpoints = append(redis.Sentinel.Endpoints, "REDIS-A.EXAMPLE.TEST:26379")
			return config
		}},
		{name: "missing sentinel master", config: func(t *testing.T) runtimeconfig.Config {
			config := validSentinelConfig(t)
			config.Resources["main"].Provider.Redis.Sentinel.MasterName = ""
			return config
		}},
		{name: "negative database", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Database = -1
		})},
		{name: "cluster nonzero database", config: func(t *testing.T) runtimeconfig.Config {
			config := validClusterConfig(t)
			config.Resources["main"].Provider.Redis.Database = 1
			return config
		}},
		{name: "missing credential branch", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Credentials = runtimeconfig.RedisCredentialsConfig{}
		})},
		{name: "multiple credential branches", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Credentials.Anonymous = &runtimeconfig.RedisAnonymousCredentialsConfig{}
		})},
		{name: "credential kind mismatch", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Credentials.Kind = runtimeconfig.RedisCredentialsAnonymous
		})},
		{name: "missing password ref", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.Credentials.Password.PasswordRef = runtimeconfig.SecretRef{}
		})},
		{name: "invalid TLS version", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.TLS = &runtimeconfig.RedisTLSConfig{MinVersion: "1.1"}
		})},
		{name: "invalid TLS server name", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.TLS = &runtimeconfig.RedisTLSConfig{ServerName: "https://redis.example.test"}
		})},
		{name: "certificate without key", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.TLS = &runtimeconfig.RedisTLSConfig{ClientCertificateRef: mustSecretRef(t, certReference)}
		})},
		{name: "key without certificate", config: mutateRedis(func(redis *runtimeconfig.RedisConfig) {
			redis.TLS = &runtimeconfig.RedisTLSConfig{ClientKeyRef: mustSecretRef(t, keyReference)}
		})},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolver := &countingResolver{values: map[string]string{passwordReference: passwordCanary}}
			profile, err := test.config(t).Normalize(context.Background(), resolver)
			if !errors.Is(err, runtimeconfig.ErrInvalidConfiguration) {
				t.Fatalf("Normalize() error = %v, want ErrInvalidConfiguration", err)
			}
			if profile != nil {
				t.Fatalf("Normalize() profile = %#v, want nil", profile)
			}
			if calls := resolver.calls.Load(); calls != 0 {
				t.Fatalf("resolver calls = %d, want zero for structurally invalid config", calls)
			}
		})
	}

	config := validStandaloneConfig(t)
	if _, err := config.Normalize(context.Background(), nil); !errors.Is(err, runtimeconfig.ErrInvalidConfiguration) {
		t.Fatalf("Normalize(nil resolver) error = %v, want ErrInvalidConfiguration", err)
	}
}

func TestRedisDeploymentModesNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		config     func(t *testing.T) runtimeconfig.Config
		mode       runtimeconfig.RedisMode
		endpoints  []string
		masterName string
		database   int
	}{
		{
			name:      "standalone",
			config:    validStandaloneConfig,
			mode:      runtimeconfig.RedisStandalone,
			endpoints: []string{"redis.example.test:6379"},
			database:  2,
		},
		{
			name:       "sentinel",
			config:     validSentinelConfig,
			mode:       runtimeconfig.RedisSentinel,
			endpoints:  []string{"redis-a.example.test:26379", "redis-b.example.test:26379"},
			masterName: "primary",
			database:   3,
		},
		{
			name:      "cluster",
			config:    validClusterConfig,
			mode:      runtimeconfig.RedisCluster,
			endpoints: []string{"redis-a.example.test:6379", "redis-b.example.test:6379"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolver := &countingResolver{values: map[string]string{passwordReference: passwordCanary}}
			snapshot, err := test.config(t).Normalize(context.Background(), resolver)
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			resource, ok := snapshot.Resource("main")
			if !ok {
				t.Fatal("Resource(main) was not found")
			}
			if got := resource.Provider(); got != runtimeconfig.ProviderRedis {
				t.Fatalf("Provider() = %q, want redis", got)
			}
			profile, ok := resource.Redis()
			if !ok {
				t.Fatal("Redis() profile was not found")
			}
			if got := profile.Mode(); got != test.mode {
				t.Fatalf("Mode() = %q, want %q", got, test.mode)
			}
			if got := strings.Join(profile.Endpoints(), ","); got != strings.Join(test.endpoints, ",") {
				t.Fatalf("Endpoints() = %q, want %q", got, strings.Join(test.endpoints, ","))
			}
			if got := profile.SentinelMasterName(); got != test.masterName {
				t.Fatalf("SentinelMasterName() = %q, want %q", got, test.masterName)
			}
			if got := profile.Database(); got != test.database {
				t.Fatalf("Database() = %d, want %d", got, test.database)
			}
			if profile.DialTimeout() != 5*time.Second || profile.ReadTimeout() != 3*time.Second || profile.WriteTimeout() != 3*time.Second {
				t.Fatalf("default timeouts = (%s, %s, %s), want (5s, 3s, 3s)", profile.DialTimeout(), profile.ReadTimeout(), profile.WriteTimeout())
			}
		})
	}
}

func TestRuntimeConfigSecretResolutionAndRedaction(t *testing.T) {
	t.Parallel()

	certificate, privateKey := testCertificate(t)
	config := validStandaloneConfig(t)
	redis := config.Resources["main"].Provider.Redis
	redis.Credentials.Password.UsernameRef = mustSecretRef(t, usernameReference)
	redis.TLS = &runtimeconfig.RedisTLSConfig{
		MinVersion:           "1.3",
		ServerName:           "redis.example.test",
		CARef:                mustSecretRef(t, caReference),
		ClientCertificateRef: mustSecretRef(t, certReference),
		ClientKeyRef:         mustSecretRef(t, keyReference),
	}
	resolver := &countingResolver{values: map[string]string{
		passwordReference: passwordCanary,
		usernameReference: usernameCanary,
		caReference:       certificate,
		certReference:     certificate,
		keyReference:      privateKey,
	}}

	snapshot, err := config.Normalize(context.Background(), resolver)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if got := resolver.calls.Load(); got != 5 {
		t.Fatalf("resolver calls = %d, want 5", got)
	}
	resource, _ := snapshot.Resource("main")
	profile, _ := resource.Redis()
	credentials := profile.Credentials()
	username, ok := credentials.Username()
	if !ok || username.Reveal() != usernameCanary {
		t.Fatalf("Username() = (%v, %v), want resolved username", username, ok)
	}
	password, ok := credentials.Password()
	if !ok || password.Reveal() != passwordCanary {
		t.Fatalf("Password() = (%v, %v), want resolved password", password, ok)
	}
	tlsProfile, ok := profile.TLS()
	if !ok || tlsProfile.ServerName() != "redis.example.test" {
		t.Fatalf("TLS() = (%v, %v), want enabled TLS", tlsProfile, ok)
	}
	if _, ok := tlsProfile.CA(); !ok {
		t.Fatal("TLS CA was not resolved")
	}
	if _, ok := tlsProfile.ClientCertificate(); !ok {
		t.Fatal("TLS client certificate was not resolved")
	}
	if _, ok := tlsProfile.ClientKey(); !ok {
		t.Fatal("TLS client key was not resolved")
	}

	plan, err := snapshot.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	assertRedacted(t, config, snapshot, plan, resource, profile, credentials, tlsProfile, username, password)

	missingResolver := &countingResolver{values: map[string]string{}}
	_, err = validStandaloneConfig(t).Normalize(context.Background(), missingResolver)
	if !errors.Is(err, runtimeconfig.ErrSecretUnavailable) {
		t.Fatalf("Normalize(missing secret) error = %v, want ErrSecretUnavailable", err)
	}
	if strings.Contains(fmt.Sprintf("%v %+v %#v", err, err, err), passwordCanary) || strings.Contains(fmt.Sprint(err), "RUNTIME_CONFIG_TEST_PASSWORD") {
		t.Fatalf("Normalize(missing secret) error exposed resolver material or locator: %v", err)
	}

	cancelingResolver := runtimeconfig.SecretResolverFunc(func(context.Context, runtimeconfig.SecretRef) (string, error) {
		return "", fmt.Errorf("%s: %w", passwordCanary, context.Canceled)
	})
	_, err = validStandaloneConfig(t).Normalize(context.Background(), cancelingResolver)
	if !errors.Is(err, context.Canceled) || strings.Contains(fmt.Sprint(err), passwordCanary) {
		t.Fatalf("Normalize(canceling resolver) error = %v, want redacted context.Canceled", err)
	}

	var nilResolver runtimeconfig.SecretResolverFunc
	_, err = validStandaloneConfig(t).Normalize(context.Background(), nilResolver)
	if !errors.Is(err, runtimeconfig.ErrSecretUnavailable) {
		t.Fatalf("Normalize(nil resolver function) error = %v, want ErrSecretUnavailable", err)
	}

	badCAResolver := &countingResolver{values: map[string]string{
		passwordReference: passwordCanary,
		caReference:       "not-a-certificate-" + passwordCanary,
	}}
	badCAConfig := validStandaloneConfig(t)
	badCAConfig.Resources["main"].Provider.Redis.TLS = &runtimeconfig.RedisTLSConfig{CARef: mustSecretRef(t, caReference)}
	_, err = badCAConfig.Normalize(context.Background(), badCAResolver)
	if !errors.Is(err, runtimeconfig.ErrSecretUnavailable) || strings.Contains(fmt.Sprint(err), passwordCanary) {
		t.Fatalf("Normalize(malformed CA) error = %v, want redacted ErrSecretUnavailable", err)
	}

	badPairResolver := &countingResolver{values: map[string]string{
		passwordReference: passwordCanary,
		certReference:     certificate,
		keyReference:      "not-a-private-key-" + passwordCanary,
	}}
	badPairConfig := validStandaloneConfig(t)
	badPairConfig.Resources["main"].Provider.Redis.TLS = &runtimeconfig.RedisTLSConfig{
		ClientCertificateRef: mustSecretRef(t, certReference),
		ClientKeyRef:         mustSecretRef(t, keyReference),
	}
	_, err = badPairConfig.Normalize(context.Background(), badPairResolver)
	if !errors.Is(err, runtimeconfig.ErrSecretUnavailable) || strings.Contains(fmt.Sprint(err), passwordCanary) {
		t.Fatalf("Normalize(malformed client pair) error = %v, want redacted ErrSecretUnavailable", err)
	}
}

func TestRuntimeConfigNormalizeAndBuildAreImmutableAndSideEffectFree(t *testing.T) {
	t.Parallel()

	config := validStandaloneConfig(t)
	dialTimeout, err := runtimeconfig.ParseDuration("7s")
	if err != nil {
		t.Fatalf("ParseDuration() error = %v", err)
	}
	config.Resources["main"].Provider.Redis.DialTimeout = dialTimeout
	resolver := &countingResolver{values: map[string]string{passwordReference: passwordCanary}}

	snapshot, err := config.Normalize(context.Background(), resolver)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if got := resolver.calls.Load(); got != 1 {
		t.Fatalf("resolver calls after Normalize = %d, want 1", got)
	}

	config.Resources["main"].Provider.Redis.Standalone.Endpoint = "mutated.example.test:6380"
	config.Resources["main"].Provider.Redis.Database = 15
	config.Resources["second"] = config.Resources["main"]

	const builders = 64
	plans := make([]*runtimeconfig.Plan, builders)
	errorsByIndex := make([]error, builders)
	var wait sync.WaitGroup
	for index := range builders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			plans[index], errorsByIndex[index] = snapshot.Build(context.Background())
		}()
	}
	wait.Wait()
	for index, buildErr := range errorsByIndex {
		if buildErr != nil {
			t.Fatalf("Build()[%d] error = %v", index, buildErr)
		}
	}
	if got := resolver.calls.Load(); got != 1 {
		t.Fatalf("resolver calls after Build = %d, want Build side-effect count to remain 1", got)
	}
	if got := snapshot.ResourceNames(); strings.Join(got, ",") != "main" {
		t.Fatalf("snapshot ResourceNames() = %v, want immutable [main]", got)
	}
	resource, ok := plans[0].Resource("main")
	if !ok {
		t.Fatal("plan Resource(main) not found")
	}
	profile, _ := resource.Redis()
	if got := profile.Endpoints(); len(got) != 1 || got[0] != "redis.example.test:6379" {
		t.Fatalf("plan endpoints = %v, want immutable original endpoint", got)
	}
	if profile.Database() != 2 || profile.DialTimeout() != 7*time.Second {
		t.Fatalf("plan database/dial timeout = %d/%s, want 2/7s", profile.Database(), profile.DialTimeout())
	}
	endpoints := profile.Endpoints()
	endpoints[0] = "caller-mutated.example.test:6379"
	profileAgain, _ := resource.Redis()
	if got := profileAgain.Endpoints()[0]; got != "redis.example.test:6379" {
		t.Fatalf("profile accessor mutation changed snapshot: %q", got)
	}
	names := plans[0].ResourceNames()
	names[0] = "caller-mutated"
	if got := plans[0].ResourceNames()[0]; got != "main" {
		t.Fatalf("plan name accessor mutation changed plan: %q", got)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := snapshot.Build(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("Build(canceled) error = %v, want context.Canceled", err)
	}
}

func TestEnvSecretResolverHonorsContextAndRedactsMissingReference(t *testing.T) {
	ref := mustSecretRef(t, "env://RUNTIME_CONFIG_ENV_RESOLVER_TEST")
	t.Setenv("RUNTIME_CONFIG_ENV_RESOLVER_TEST", passwordCanary)
	value, err := (runtimeconfig.EnvSecretResolver{}).Resolve(context.Background(), ref)
	if err != nil || value != passwordCanary {
		t.Fatalf("Resolve() = (%q, %v), want configured value", value, err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (runtimeconfig.EnvSecretResolver{}).Resolve(canceled, ref); !errors.Is(err, context.Canceled) {
		t.Fatalf("Resolve(canceled) error = %v, want context.Canceled", err)
	}

	missing := mustSecretRef(t, "env://RUNTIME_CONFIG_ENV_RESOLVER_MISSING")
	_, err = (runtimeconfig.EnvSecretResolver{}).Resolve(context.Background(), missing)
	if !errors.Is(err, runtimeconfig.ErrSecretUnavailable) {
		t.Fatalf("Resolve(missing) error = %v, want ErrSecretUnavailable", err)
	}
	if strings.Contains(fmt.Sprint(err), "RUNTIME_CONFIG_ENV_RESOLVER_MISSING") {
		t.Fatalf("Resolve(missing) error exposed reference locator: %v", err)
	}
}

func TestSecretRefFingerprintIsStableOpaqueAndDistinct(t *testing.T) {
	const (
		reference      = "env://RUNTIME_CONFIG_FINGERPRINT_LOCATOR_CANARY"
		otherReference = "env://RUNTIME_CONFIG_FINGERPRINT_OTHER_CANARY"
		locatorCanary  = "RUNTIME_CONFIG_FINGERPRINT_LOCATOR_CANARY"
		prefix         = "pbkdf2-sha256:"
	)

	ref := mustSecretRef(t, reference)
	repeated := mustSecretRef(t, reference)
	other := mustSecretRef(t, otherReference)
	if ref.Fingerprint() != repeated.Fingerprint() {
		t.Fatalf("fingerprint changed for the same reference: %q != %q", ref.Fingerprint(), repeated.Fingerprint())
	}
	if ref.Fingerprint() == other.Fingerprint() {
		t.Fatalf("fingerprints for distinct references are equal: %q", ref.Fingerprint())
	}
	encoded := strings.TrimPrefix(ref.Fingerprint(), prefix)
	if encoded == ref.Fingerprint() || len(encoded) != 24 {
		t.Fatalf("Fingerprint() = %q, want %s followed by 12 encoded bytes", ref.Fingerprint(), prefix)
	}
	if _, err := hex.DecodeString(encoded); err != nil {
		t.Fatalf("Fingerprint() payload is not hexadecimal: %v", err)
	}

	jsonValue, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}
	yamlValue, err := yaml.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalYAML() error = %v", err)
	}
	outputs := []string{
		fmt.Sprint(ref),
		fmt.Sprintf("%+v", ref),
		fmt.Sprintf("%#v", ref),
		string(jsonValue),
		string(yamlValue),
	}
	for _, output := range outputs {
		if strings.Contains(output, locatorCanary) || strings.Contains(output, reference) || strings.Contains(output, passwordCanary) {
			t.Fatalf("SecretRef diagnostic exposed locator or secret material: %q", output)
		}
	}
}

func validStandaloneConfig(t *testing.T) runtimeconfig.Config {
	t.Helper()
	return runtimeconfig.Config{Resources: map[string]runtimeconfig.ResourceConfig{
		"main": {
			Provider: runtimeconfig.ProviderConfig{
				Kind: runtimeconfig.ProviderRedis,
				Redis: &runtimeconfig.RedisConfig{
					Mode:       runtimeconfig.RedisStandalone,
					Standalone: &runtimeconfig.RedisStandaloneConfig{Endpoint: "REDIS.EXAMPLE.TEST:6379"},
					Database:   2,
					Credentials: runtimeconfig.RedisCredentialsConfig{
						Kind: runtimeconfig.RedisCredentialsPassword,
						Password: &runtimeconfig.RedisPasswordCredentialsConfig{
							PasswordRef: mustSecretRef(t, passwordReference),
						},
					},
				},
			},
		},
	}}
}

func validSentinelConfig(t *testing.T) runtimeconfig.Config {
	t.Helper()
	config := validStandaloneConfig(t)
	redis := config.Resources["main"].Provider.Redis
	redis.Mode = runtimeconfig.RedisSentinel
	redis.Standalone = nil
	redis.Sentinel = &runtimeconfig.RedisSentinelConfig{
		Endpoints:  []string{"redis-a.example.test:26379", "redis-b.example.test:26379"},
		MasterName: "primary",
	}
	redis.Database = 3
	return config
}

func validClusterConfig(t *testing.T) runtimeconfig.Config {
	t.Helper()
	config := validStandaloneConfig(t)
	redis := config.Resources["main"].Provider.Redis
	redis.Mode = runtimeconfig.RedisCluster
	redis.Standalone = nil
	redis.Cluster = &runtimeconfig.RedisClusterConfig{
		Endpoints: []string{"redis-a.example.test:6379", "redis-b.example.test:6379"},
	}
	redis.Database = 0
	return config
}

func mutateValidConfig(mutate func(t *testing.T, config *runtimeconfig.Config)) func(*testing.T) runtimeconfig.Config {
	return func(t *testing.T) runtimeconfig.Config {
		config := validStandaloneConfig(t)
		mutate(t, &config)
		return config
	}
}

func mutateRedis(mutate func(redis *runtimeconfig.RedisConfig)) func(*testing.T) runtimeconfig.Config {
	return mutateValidConfig(func(_ *testing.T, config *runtimeconfig.Config) {
		mutate(config.Resources["main"].Provider.Redis)
	})
}

func mustSecretRef(t *testing.T, reference string) runtimeconfig.SecretRef {
	t.Helper()
	ref, err := runtimeconfig.ParseSecretRef(reference)
	if err != nil {
		t.Fatalf("ParseSecretRef(%q) error = %v", reference, err)
	}
	return ref
}

func testCertificate(t *testing.T) (string, string) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "redis.example.test"},
		DNSNames:     []string{"redis.example.test"},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		IsCA:         true,
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
	return string(certificatePEM), string(privateKeyPEM)
}

func assertRedacted(t *testing.T, values ...any) {
	t.Helper()
	for _, value := range values {
		formatted := fmt.Sprintf("%v %+v %#v", value, value, value)
		jsonBytes, jsonErr := json.Marshal(value)
		if jsonErr != nil {
			t.Fatalf("json.Marshal(%T) error = %v", value, jsonErr)
		}
		yamlBytes, yamlErr := yaml.Marshal(value)
		if yamlErr != nil {
			t.Fatalf("yaml.Marshal(%T) error = %v", value, yamlErr)
		}
		output := formatted + string(jsonBytes) + string(yamlBytes)
		for _, canary := range []string{
			passwordCanary,
			usernameCanary,
			"RUNTIME_CONFIG_TEST_PASSWORD",
			"RUNTIME_CONFIG_TEST_USERNAME",
			"RUNTIME_CONFIG_TEST_CA",
			"RUNTIME_CONFIG_TEST_CERT",
			"RUNTIME_CONFIG_TEST_KEY",
		} {
			if strings.Contains(output, canary) {
				t.Fatalf("%T output exposed canary %q: %s", value, canary, output)
			}
		}
	}
}
