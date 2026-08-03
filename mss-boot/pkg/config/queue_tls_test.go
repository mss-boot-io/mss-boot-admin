package config

import (
	"crypto/tls"
	"testing"
)

func TestKafkaTLSConfigVerifiesCertificates(t *testing.T) {
	cfg := (&Kafka{}).getConfig()
	if !cfg.Net.TLS.Enable {
		t.Fatal("Kafka TLS is disabled")
	}
	if cfg.Net.TLS.Config == nil {
		t.Fatal("Kafka TLS config is nil")
	}
	if cfg.Net.TLS.Config.InsecureSkipVerify {
		t.Fatal("Kafka TLS certificate verification is disabled")
	}
	if cfg.Net.TLS.Config.MinVersion < tls.VersionTLS12 {
		t.Fatalf("Kafka TLS minimum version = %d, want TLS 1.2 or newer", cfg.Net.TLS.Config.MinVersion)
	}
}

func TestMSKTLSConfigVerifiesCertificates(t *testing.T) {
	cfg := (&Kafka{KafkaParams: KafkaParams{Provider: "msk"}, SASL: &SASL{Region: "us-east-1"}}).getConfig()
	if !cfg.Net.TLS.Enable {
		t.Fatal("MSK TLS is disabled")
	}
	if cfg.Net.TLS.Config == nil {
		t.Fatal("MSK TLS config is nil")
	}
	if cfg.Net.TLS.Config.InsecureSkipVerify {
		t.Fatal("MSK TLS certificate verification is disabled")
	}
	if cfg.Net.TLS.Config.MinVersion < tls.VersionTLS12 {
		t.Fatalf("MSK TLS minimum version = %d, want TLS 1.2 or newer", cfg.Net.TLS.Config.MinVersion)
	}
}
