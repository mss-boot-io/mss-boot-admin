package config

import (
	"context"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/middleware"
)

// Deprecated provider names retained for v1.0 source compatibility. They are
// not accepted by Storage.Normalize and do not imply Runtime v2 support.
const (
	OSS   ProviderType = "oss"
	OOS   ProviderType = "oos"
	KODO  ProviderType = "kodo"
	COS   ProviderType = "cos"
	OBS   ProviderType = "obs"
	BOS   ProviderType = "bos"
	GCS   ProviderType = "gcs"
	KS3   ProviderType = "ks3"
	MINIO ProviderType = "minio"
)

// URLTemplate is retained for v1.0 source compatibility only. Runtime v2 does
// not read this mutable map; strict S3-compatible endpoints must be explicit.
//
// Deprecated: use S3StorageConfig.Endpoint.
var URLTemplate = map[ProviderType]string{
	OSS:  "https://%s.aliyuncs.com",
	OOS:  "https://oos-%s.ctyunapi.cn",
	KODO: "https://s3-%s.qiniucs.com",
	COS:  "https://cos.%s.myqcloud.com",
	OBS:  "https://obs.%s.myhuaweicloud.com",
	BOS:  "https://s3.%s.bcebos.com",
	GCS:  "https://storage.googleapis.com",
	KS3:  "https://ks3-%s.ksyuncs.com",
}

// Init preserves the v1.0 method signature. It accepts only complete static
// credentials and never falls back to a default chain, exits the process, or
// performs network I/O. Unsupported or incomplete legacy input leaves the
// client nil and emits a value-free diagnostic.
//
// Deprecated: use Storage.Normalize followed by StorageProfile.Build.
func (o *Storage) Init() {
	if o == nil {
		return
	}
	o.client = nil
	client, reason := buildLegacyStorageClient(*o)
	if reason != "" {
		slog.Warn("deprecated storage configuration was not installed", "reason", reason)
		return
	}
	o.client = client
}

// GetClient preserves the v1.0 method signature. A nil result means the
// deprecated configuration was rejected; callers must fail closed.
//
// Deprecated: borrow the client through StorageHandle.Use.
func (o *Storage) GetClient() *s3.Client {
	if o == nil {
		return nil
	}
	return o.client
}

func (c Storage) hasLegacyFields() bool {
	return c.Type != "" || c.SigningMethod != "" || c.Region != "" || c.Bucket != "" ||
		c.Endpoint != "" || c.AccessKeyID != "" || c.SecretAccessKey != ""
}

func buildLegacyStorageClient(c Storage) (*s3.Client, string) {
	if c.Local != nil || c.S3 != nil {
		return nil, "mixed-strict-and-legacy"
	}
	provider := c.Type
	if provider == "" {
		provider = S3
	}
	region := strings.TrimSpace(c.Region)
	if region == "" && (provider == MINIO || provider == GCS) {
		region = "auto"
	}
	accessKeyID := strings.TrimSpace(c.AccessKeyID)
	secretAccessKey := strings.TrimSpace(c.SecretAccessKey)
	if region == "" || accessKeyID == "" || secretAccessKey == "" {
		return nil, "incomplete-static-profile"
	}

	endpoint, usePathStyle, ok := legacyEndpoint(provider, region, c.Endpoint)
	if !ok {
		return nil, "unsupported-provider"
	}
	if endpoint != "" {
		normalized, _, err := normalizeS3Endpoint(endpoint)
		if err != nil {
			return nil, "invalid-endpoint"
		}
		endpoint = normalized
	}
	options := s3.Options{
		Region: region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"",
		)),
		UsePathStyle: usePathStyle,
	}
	if endpoint != "" {
		options.BaseEndpoint = aws.String(endpoint)
	}
	return s3.New(options), ""
}

func legacyEndpoint(provider ProviderType, region, explicit string) (string, bool, bool) {
	switch provider {
	case S3:
		return "", false, true
	case MINIO:
		endpoint := strings.TrimSpace(explicit)
		return endpoint, true, endpoint != ""
	case OSS:
		return "https://" + region + ".aliyuncs.com", false, true
	case OOS:
		return "https://oos-" + region + ".ctyunapi.cn", false, true
	case KODO:
		return "https://s3-" + region + ".qiniucs.com", false, true
	case COS:
		return "https://cos." + region + ".myqcloud.com", false, true
	case OBS:
		return "https://obs." + region + ".myhuaweicloud.com", false, true
	case BOS:
		return "https://s3." + region + ".bcebos.com", false, true
	case GCS:
		return "https://storage.googleapis.com", false, true
	case KS3:
		return "https://ks3-" + region + ".ksyuncs.com", false, true
	default:
		return "", false, false
	}
}

type acceptEncodingKey struct{}

// GetAcceptEncodingKey preserves the v1.0 middleware helper.
//
// Deprecated: provider-specific signing middleware is not part of Runtime v2.
func GetAcceptEncodingKey(ctx context.Context) string {
	value, _ := middleware.GetStackValue(ctx, acceptEncodingKey{}).(string)
	return value
}

// SetAcceptEncodingKey preserves the v1.0 middleware helper.
//
// Deprecated: provider-specific signing middleware is not part of Runtime v2.
func SetAcceptEncodingKey(ctx context.Context, value string) context.Context {
	return middleware.WithStackValue(ctx, acceptEncodingKey{}, value)
}
