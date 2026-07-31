package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/12/14 03:12:11
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/12/14 03:12:11
 */

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestStorage_Init(t *testing.T) {
	type fields struct {
		Type            ProviderType
		SigningMethod   string
		Region          string
		Bucket          string
		Endpoint        string
		AccessKeyID     string
		SecretAccessKey string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "test-oss",
			fields: fields{
				Type:            OSS,
				SigningMethod:   "v4",
				Region:          os.Getenv("oss_region"),
				Bucket:          os.Getenv("oss_bucket"),
				AccessKeyID:     os.Getenv("oss_access_key_id"),
				SecretAccessKey: os.Getenv("oss_secret_access_key"),
			},
		},
		{
			name: "test-bos",
			fields: fields{
				Type:            BOS,
				SigningMethod:   "v4",
				Region:          os.Getenv("bos_region"),
				Bucket:          os.Getenv("bos_bucket"),
				AccessKeyID:     os.Getenv("bos_access_key_id"),
				SecretAccessKey: os.Getenv("bos_secret_access_key"),
			},
		},
		{
			name: "test-ks3",
			fields: fields{
				Type:            KS3,
				SigningMethod:   "v4",
				Region:          os.Getenv("ks3_region"),
				Bucket:          os.Getenv("ks3_bucket"),
				AccessKeyID:     os.Getenv("ks3_access_key_id"),
				SecretAccessKey: os.Getenv("ks3_secret_access_key"),
			},
		},
		{
			name: "test-kodo",
			fields: fields{
				Type:            KODO,
				SigningMethod:   "v4",
				Region:          os.Getenv("kodo_region"),
				Bucket:          os.Getenv("kodo_bucket"),
				AccessKeyID:     os.Getenv("kodo_access_key_id"),
				SecretAccessKey: os.Getenv("kodo_secret_access_key"),
			},
		},
		{
			name: "test-minio",
			fields: fields{
				Type:            MINIO,
				SigningMethod:   "v4",
				Region:          os.Getenv("minio_region"),
				Bucket:          os.Getenv("minio_bucket"),
				Endpoint:        os.Getenv("minio_endpoint"),
				AccessKeyID:     os.Getenv("minio_access_key_id"),
				SecretAccessKey: os.Getenv("minio_secret_access_key"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fields.Region == "" || tt.fields.Bucket == "" || tt.fields.AccessKeyID == "" || tt.fields.SecretAccessKey == "" {
				t.Skipf("skip %s integration test: storage credentials are not configured", tt.name)
			}
			if tt.fields.Type == MINIO && tt.fields.Endpoint == "" {
				t.Skip("skip minio integration test: minio_endpoint is not configured")
			}
			o := &Storage{
				Type:            tt.fields.Type,
				SigningMethod:   tt.fields.SigningMethod,
				Region:          tt.fields.Region,
				Bucket:          tt.fields.Bucket,
				Endpoint:        tt.fields.Endpoint,
				AccessKeyID:     tt.fields.AccessKeyID,
				SecretAccessKey: tt.fields.SecretAccessKey,
			}
			o.Init()
			res, err := o.GetClient().ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
				Bucket:  aws.String(tt.fields.Bucket),
				MaxKeys: aws.Int32(10),
			})
			if err != nil {
				t.Fatalf("failed to list items: %v", err)
			}

			for _, o := range res.Contents {
				fmt.Println(">>> ", *o.Key)
			}
			_, err = o.GetClient().PutObject(context.TODO(), &s3.PutObjectInput{
				Bucket: aws.String(tt.fields.Bucket),
				Key:    aws.String("test.json"),
				Body:   bytes.NewBuffer([]byte(`{"name": "lwx"}`)),
			})
			if err != nil {
				t.Fatalf("failed to put object: %v", err)
			}
		})
	}
}
