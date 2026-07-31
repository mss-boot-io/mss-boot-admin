package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/12/11 07:33:01
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/12/11 07:33:01
 */

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// ProviderType storage provider type
type ProviderType string

const (
	// S3 aws s3
	S3 ProviderType = "s3"
	// OSS aliyun oss
	OSS ProviderType = "oss"
	// OOS ctyun oos
	OOS ProviderType = "oos"
	// KODO qiniu kodo
	KODO ProviderType = "kodo"
	// COS tencent cos
	COS ProviderType = "cos"
	// OBS huawei obs
	OBS ProviderType = "obs"
	// BOS baidu bos
	BOS ProviderType = "bos"
	// GCS google gcs
	GCS ProviderType = "gcs"
	// KS3 kingsoft ks3
	KS3 ProviderType = "ks3"
	// MINIO minio storage
	MINIO ProviderType = "minio"
)

// URLTemplate storage provider url template
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

var endpointResolverFunc = func(urlTemplate, signingMethod string) s3.EndpointResolverFunc {
	return func(region string, options s3.EndpointResolverOptions) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:           fmt.Sprintf(urlTemplate, region),
			SigningRegion: region,
			SigningMethod: signingMethod,
		}, nil
	}
}

var endpointResolverFuncMinio = func(urlTemplate, signingMethod string) s3.EndpointResolverFunc {
	return func(region string, options s3.EndpointResolverOptions) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               urlTemplate,
			SigningRegion:     region,
			SigningMethod:     signingMethod,
			HostnameImmutable: true,
		}, nil
	}
}

// Storage storage
type Storage struct {
	Type            ProviderType `yaml:"type"`
	SigningMethod   string       `yaml:"signingMethod"`
	Region          string       `yaml:"region"`
	Bucket          string       `yaml:"bucket"`
	Endpoint        string       `yaml:"endpoint"`
	AccessKeyID     string       `yaml:"accessKeyID"`
	SecretAccessKey string       `yaml:"secretAccessKey"`
	client          *s3.Client
}

// Init init
func (o *Storage) Init() {
	var endpointResolver s3.EndpointResolver
	switch o.Type {
	case GCS:
		endpointResolver = s3.EndpointResolverFromURL(URLTemplate[GCS])
		o.Region = "auto"
	case MINIO:
		endpointResolver = endpointResolverFuncMinio(o.Endpoint, o.SigningMethod)
		o.Region = "auto"
	case S3:
	default:
		if urlTemplate, exist := URLTemplate[o.Type]; exist && urlTemplate != "" {
			endpointResolver = endpointResolverFunc(urlTemplate, o.SigningMethod)
		}
	}

	if o.Region == "" || o.AccessKeyID == "" || o.SecretAccessKey == "" {
		// use default config
		opts := make([]func(*config.LoadOptions) error, 0)
		if o.Region != "" {
			opts = append(opts, config.WithRegion(o.Region))
		}
		cfg, err := config.LoadDefaultConfig(context.TODO(), opts...)
		if err != nil {
			log.Fatalf("failed to load SDK configuration, %v", err)
		}
		o.client = s3.NewFromConfig(cfg)
		return
	}

	o.client = s3.New(s3.Options{
		Region: o.Region,
		Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     o.AccessKeyID,
				SecretAccessKey: o.SecretAccessKey,
			}, nil
		}),
		EndpointResolver: endpointResolver,
	}, func(s3Options *s3.Options) {
		switch o.Type {
		case GCS:
			s3Options.APIOptions = append(s3Options.APIOptions, func(stack *middleware.Stack) error {
				if err := stack.Finalize.Insert(dropAcceptEncodingHeader, "Signing", middleware.Before); err != nil {
					return err
				}

				if err := stack.Finalize.Insert(replaceAcceptEncodingHeader, "Signing", middleware.After); err != nil {
					return err
				}

				return nil
			})
		}
	})
}

// GetClient get client
func (o *Storage) GetClient() *s3.Client {
	return o.client
}

const acceptEncodingHeader = "Accept-Encoding"

type acceptEncodingKey struct{}

func GetAcceptEncodingKey(ctx context.Context) (v string) {
	v, _ = middleware.GetStackValue(ctx, acceptEncodingKey{}).(string)
	return v
}

func SetAcceptEncodingKey(ctx context.Context, value string) context.Context {
	return middleware.WithStackValue(ctx, acceptEncodingKey{}, value)
}

var dropAcceptEncodingHeader = middleware.FinalizeMiddlewareFunc("DropAcceptEncodingHeader",
	func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (out middleware.FinalizeOutput, metadata middleware.Metadata, err error) {
		req, ok := in.Request.(*smithyhttp.Request)
		if !ok {
			return out, metadata, &v4.SigningError{Err: fmt.Errorf("unexpected request middleware type %T", in.Request)}
		}

		ae := req.Header.Get(acceptEncodingHeader)
		ctx = SetAcceptEncodingKey(ctx, ae)
		req.Header.Del(acceptEncodingHeader)
		in.Request = req

		return next.HandleFinalize(ctx, in)
	},
)

var replaceAcceptEncodingHeader = middleware.FinalizeMiddlewareFunc("ReplaceAcceptEncodingHeader",
	func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (out middleware.FinalizeOutput, metadata middleware.Metadata, err error) {
		req, ok := in.Request.(*smithyhttp.Request)
		if !ok {
			return out, metadata, &v4.SigningError{Err: fmt.Errorf("unexpected request middleware type %T", in.Request)}
		}

		ae := GetAcceptEncodingKey(ctx)
		req.Header.Set(acceptEncodingHeader, ae)
		in.Request = req

		return next.HandleFinalize(ctx, in)
	},
)
