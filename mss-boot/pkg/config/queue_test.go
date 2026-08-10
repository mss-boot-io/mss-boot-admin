package config

import (
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/IBM/sarama"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
)

func TestQueueEmptyIncludesKafka(t *testing.T) {
	var nilQueue *Queue
	if !nilQueue.Empty() {
		t.Fatal("nil queue should be empty")
	}
	if !(&Queue{}).Empty() {
		t.Fatal("zero queue should be empty")
	}
	if (&Queue{Kafka: &Kafka{}}).Empty() {
		t.Fatal("Kafka configuration must make the queue non-empty")
	}
}

func TestKafkaBuildConfigValidatesStartupProfile(t *testing.T) {
	cfg, err := (&Kafka{KafkaParams: KafkaParams{
		Brokers:   []string{"localhost:9092"},
		Provider:  "kafka",
		Version:   "3.6.0",
		Timeout:   4 * time.Second,
		KeepAlive: 2 * time.Second,
	}}).buildConfig(context.TODO())
	if err != nil {
		t.Fatalf("buildConfig() error = %v", err)
	}
	if cfg.Net.DialTimeout != 4*time.Second {
		t.Fatalf("DialTimeout = %v, want 4s", cfg.Net.DialTimeout)
	}
	if cfg.Net.KeepAlive != 2*time.Second {
		t.Fatalf("KeepAlive = %v, want 2s", cfg.Net.KeepAlive)
	}
	if !cfg.Net.TLS.Enable || cfg.Net.TLS.Config == nil {
		t.Fatal("validated Kafka configuration must enable TLS")
	}
	if cfg.Net.TLS.Config.InsecureSkipVerify {
		t.Fatal("validated Kafka configuration must verify certificates")
	}
	if cfg.Net.TLS.Config.MinVersion < tls.VersionTLS12 {
		t.Fatalf("TLS minimum version = %d, want TLS 1.2 or newer", cfg.Net.TLS.Config.MinVersion)
	}
	if !cfg.Producer.Return.Errors || !cfg.Producer.Return.Successes || !cfg.Consumer.Return.Errors {
		t.Fatal("Kafka result and consumer errors must be observable")
	}
	wantVersion, err := sarama.ParseKafkaVersion("3.6.0")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != wantVersion {
		t.Fatalf("Version = %v, want %v", cfg.Version, wantVersion)
	}
}

func TestKafkaBuildConfigRejectsInvalidProfiles(t *testing.T) {
	canceledContext, cancel := context.WithCancel(context.TODO())
	cancel()
	valid := func() *Kafka {
		return &Kafka{KafkaParams: KafkaParams{Brokers: []string{"localhost:9092"}}}
	}
	tests := []struct {
		name    string
		mutate  func(*Kafka)
		ctx     context.Context
		wantErr string
	}{
		{name: "nil context", ctx: nil, wantErr: "context is required"},
		{name: "canceled context", ctx: canceledContext, wantErr: "context canceled"},
		{name: "no brokers", ctx: context.TODO(), mutate: func(k *Kafka) { k.Brokers = nil }, wantErr: "broker is required"},
		{name: "empty broker", ctx: context.TODO(), mutate: func(k *Kafka) { k.Brokers = []string{" "} }, wantErr: "broker 0 is empty"},
		{name: "broker without port", ctx: context.TODO(), mutate: func(k *Kafka) { k.Brokers = []string{"localhost"} }, wantErr: "host:port"},
		{name: "broker with invalid port", ctx: context.TODO(), mutate: func(k *Kafka) { k.Brokers = []string{"localhost:70000"} }, wantErr: "invalid port"},
		{name: "unknown provider", ctx: context.TODO(), mutate: func(k *Kafka) { k.Provider = "unknown" }, wantErr: "unsupported Kafka provider"},
		{name: "negative timeout", ctx: context.TODO(), mutate: func(k *Kafka) { k.Timeout = -time.Second }, wantErr: "timeout must not be negative"},
		{name: "invalid version", ctx: context.TODO(), mutate: func(k *Kafka) { k.Version = "not-a-version" }, wantErr: "parse Kafka version"},
		{name: "invalid CA", ctx: context.TODO(), mutate: func(k *Kafka) { k.CaFile = "not PEM" }, wantErr: "not valid PEM"},
		{name: "certificate without key", ctx: context.TODO(), mutate: func(k *Kafka) { k.CertFile = "certificate" }, wantErr: "configured together"},
		{name: "disabled SASL credentials", ctx: context.TODO(), mutate: func(k *Kafka) { k.SASL = &SASL{User: "user", Password: "secret"} }, wantErr: "require SASL to be enabled"},
		{name: "SASL region without MSK", ctx: context.TODO(), mutate: func(k *Kafka) { k.SASL = &SASL{Region: "us-east-1"} }, wantErr: "only valid for the MSK"},
		{name: "SASL missing password", ctx: context.TODO(), mutate: func(k *Kafka) { k.SASL = &SASL{Enable: true, User: "user"} }, wantErr: "both user and password"},
		{name: "unsupported SASL mechanism", ctx: context.TODO(), mutate: func(k *Kafka) {
			k.SASL = &SASL{Enable: true, User: "user", Password: "secret", Mechanism: sarama.SASLTypeOAuth}
		}, wantErr: "unsupported Kafka SASL mechanism"},
		{name: "unsupported SASL version", ctx: context.TODO(), mutate: func(k *Kafka) { k.SASL = &SASL{Enable: true, User: "user", Password: "secret", Version: 2} }, wantErr: "unsupported Kafka SASL version"},
		{name: "unsupported GSSAPI profile", ctx: context.TODO(), mutate: func(k *Kafka) {
			k.SASL = &SASL{Enable: true, User: "user", Password: "secret", GSSAPI: sarama.GSSAPIConfig{ServiceName: "kafka"}}
		}, wantErr: "GSSAPI configuration is not supported"},
		{name: "MSK without SASL", ctx: context.TODO(), mutate: func(k *Kafka) { k.Provider = "msk" }, wantErr: "requires a SASL region"},
		{name: "MSK without region", ctx: context.TODO(), mutate: func(k *Kafka) { k.Provider = "msk"; k.SASL = &SASL{} }, wantErr: "requires a SASL region"},
		{name: "MSK static credentials", ctx: context.TODO(), mutate: func(k *Kafka) { k.Provider = "msk"; k.SASL = &SASL{Region: "us-east-1", User: "user"} }, wantErr: "does not accept static SASL"},
		{name: "MSK custom TLS", ctx: context.TODO(), mutate: func(k *Kafka) { k.Provider = "msk"; k.CaFile = "not used"; k.SASL = &SASL{Region: "us-east-1"} }, wantErr: "does not accept custom Kafka TLS"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := valid()
			if test.mutate != nil {
				test.mutate(profile)
			}
			_, err := profile.buildConfig(test.ctx)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("buildConfig() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestKafkaBuildConfigSupportsStrictSASLProfiles(t *testing.T) {
	for _, mechanism := range []sarama.SASLMechanism{
		sarama.SASLTypePlaintext,
		sarama.SASLTypeSCRAMSHA256,
		sarama.SASLTypeSCRAMSHA512,
	} {
		t.Run(string(mechanism), func(t *testing.T) {
			cfg, err := (&Kafka{
				KafkaParams: KafkaParams{Brokers: []string{"localhost:9092"}},
				SASL: &SASL{
					Enable:    true,
					Mechanism: mechanism,
					Version:   sarama.SASLHandshakeV1,
					User:      "user",
					Password:  "secret",
				},
			}).buildConfig(context.TODO())
			if err != nil {
				t.Fatalf("buildConfig() error = %v", err)
			}
			if !cfg.Net.SASL.Enable || cfg.Net.SASL.Mechanism != mechanism {
				t.Fatalf("SASL = enabled:%v mechanism:%q", cfg.Net.SASL.Enable, cfg.Net.SASL.Mechanism)
			}
			if mechanism != sarama.SASLTypePlaintext && cfg.Net.SASL.SCRAMClientGeneratorFunc == nil {
				t.Fatal("SCRAM profile is missing its client generator")
			}
		})
	}
}

func TestKafkaBuildConfigBindsMSKToCallerContext(t *testing.T) {
	type contextKey string
	ctx := context.WithValue(context.TODO(), contextKey("owner"), "test")
	cfg, err := (&Kafka{
		KafkaParams: KafkaParams{
			Brokers:  []string{"broker.example.com:9098"},
			Provider: " MSK ",
		},
		SASL: &SASL{Region: " us-east-1 "},
	}).buildConfig(ctx)
	if err != nil {
		t.Fatalf("buildConfig() error = %v", err)
	}
	provider, ok := cfg.Net.SASL.TokenProvider.(*MSKAccessTokenProvider)
	if !ok {
		t.Fatalf("TokenProvider type = %T", cfg.Net.SASL.TokenProvider)
	}
	if provider.Ctx != ctx {
		t.Fatal("MSK token provider did not retain the caller context")
	}
	if provider.Region != "us-east-1" {
		t.Fatalf("MSK region = %q", provider.Region)
	}
	if cfg.Net.SASL.Mechanism != sarama.SASLTypeOAuth {
		t.Fatalf("MSK SASL mechanism = %q", cfg.Net.SASL.Mechanism)
	}
}

func TestMSKAccessTokenProviderRejectsIncompleteOwnership(t *testing.T) {
	if _, err := (&MSKAccessTokenProvider{Region: "us-east-1"}).Token(); err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("Token() error = %v, want context error", err)
	}
	if _, err := (&MSKAccessTokenProvider{Ctx: context.TODO()}).Token(); err == nil || !strings.Contains(err.Error(), "region") {
		t.Fatalf("Token() error = %v, want region error", err)
	}
}

func TestQueueInitContextInstallsMemoryAndPropagatesInstallerError(t *testing.T) {
	queueConfig := &Queue{Memory: &QueueMemory{PoolSize: 3}}
	var installed storage.AdapterQueue
	if err := queueConfig.InitContext(context.TODO(), func(adapter storage.AdapterQueue) error {
		installed = adapter
		return nil
	}); err != nil {
		t.Fatalf("InitContext() error = %v", err)
	}
	if installed == nil || installed.String() != "memory" {
		t.Fatalf("installed adapter = %#v", installed)
	}

	wantErr := errors.New("policy watcher rejected adapter")
	err := queueConfig.InitContext(context.TODO(), func(storage.AdapterQueue) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("InitContext() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestQueueInitContextRejectsInvalidOwnership(t *testing.T) {
	queueConfig := &Queue{Memory: &QueueMemory{}}
	if err := queueConfig.InitContext(nil, func(storage.AdapterQueue) error { return nil }); err == nil {
		t.Fatal("InitContext(nil) succeeded")
	}
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	if err := queueConfig.InitContext(ctx, func(storage.AdapterQueue) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("InitContext(canceled) error = %v", err)
	}
	if err := queueConfig.InitContext(context.TODO(), nil); err == nil {
		t.Fatal("InitContext() succeeded without an installer")
	}
}

func TestQueueLegacyInitDoesNotTerminateOnKafkaConfigurationError(t *testing.T) {
	queueConfig := &Queue{Kafka: &Kafka{KafkaParams: KafkaParams{
		Brokers: []string{"localhost:9092"},
		Version: "invalid",
	}}}
	called := false
	queueConfig.Init(func(storage.AdapterQueue) { called = true })
	if called {
		t.Fatal("legacy Init installed an invalid Kafka adapter")
	}
}

func TestQueueLegacyInitRequiresOwnerContextForMSK(t *testing.T) {
	queueConfig := &Queue{Kafka: &Kafka{
		KafkaParams: KafkaParams{
			Brokers:  []string{"broker.example.com:9098"},
			Provider: "msk",
		},
		SASL: &SASL{Region: "us-east-1"},
	}}
	called := false
	queueConfig.Init(func(storage.AdapterQueue) { called = true })
	if called {
		t.Fatal("legacy Init installed MSK without an owner context")
	}
}
