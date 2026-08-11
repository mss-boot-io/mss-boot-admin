package config

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestStorageV100DeprecatedSourceCompatibility(t *testing.T) {
	var _ func(*Storage) = (*Storage).Init
	var _ func(*Storage) *s3.Client = (*Storage).GetClient
	_ = []ProviderType{S3, OSS, OOS, KODO, COS, OBS, BOS, GCS, KS3, MINIO}
	_ = URLTemplate
	ctx := SetAcceptEncodingKey(context.Background(), "gzip")
	if got := GetAcceptEncodingKey(ctx); got != "gzip" {
		t.Fatalf("GetAcceptEncodingKey() = %q, want gzip", got)
	}
}

func TestStorageV100DeprecatedInitFailsClosed(t *testing.T) {
	storage := &Storage{Type: S3, Region: "us-east-1"}
	storage.Init()
	if storage.GetClient() != nil {
		t.Fatal("incomplete legacy storage unexpectedly installed a client")
	}

	storage = &Storage{
		Type:            S3,
		Region:          "us-east-1",
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
	}
	storage.Init()
	if storage.GetClient() == nil {
		t.Fatal("complete static legacy storage did not construct its compatibility client")
	}
	if _, err := storage.Normalize(context.Background(), EnvSecretResolver{}); err == nil {
		t.Fatal("strict normalization accepted legacy fields")
	}
}
