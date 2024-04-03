package service

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot/pkg/config"
	"mime/multipart"
	"path/filepath"
	"strings"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/29 00:42:54
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/29 00:42:54
 */

type Storage struct{}

func (s *Storage) Upload(c *gin.Context, f *multipart.FileHeader, tenantID, userID string) (string, error) {
	storageType, _ := center.GetAppConfig().GetAppConfig(c, "storage.type")
	endpoint, _ := center.GetAppConfig().GetAppConfig(c, "storage.endpoint")
	switch storageType {
	case "s3":
		storage := config.Storage{}
		s3Type, _ := center.GetAppConfig().GetAppConfig(c, "storage.type")
		if s3Type == "" {
			s3Type = string(config.S3)
		}
		storage.Type = config.ProviderType(s3Type)
		storage.Region, _ = center.GetAppConfig().GetAppConfig(c, "storage.s3Region")
		storage.Endpoint, _ = center.GetAppConfig().GetAppConfig(c, "storage.s3Endpoint")
		storage.Bucket, _ = center.GetAppConfig().GetAppConfig(c, "storage.s3Bucket")
		storage.AccessKeyID, _ = center.GetAppConfig().GetAppConfig(c, "storage.s3AccessKeyID")
		storage.SecretAccessKey, _ = center.GetAppConfig().GetAppConfig(c, "storage.s3SecretAccessKey")
		storage.SigningMethod, _ = center.GetAppConfig().GetAppConfig(c, "storage.s3SigningMethod")
		storage.Init()
		//上传文件对象存储
		file, err := f.Open()
		if err != nil {
			return "", err
		}
		key := fmt.Sprintf("%s/%s/%s", tenantID, userID, f.Filename)
		_, err = storage.GetClient().PutObject(c, &s3.PutObjectInput{
			Bucket: &storage.Bucket,
			Key:    aws.String(key),
			Body:   file,
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s/%s", endpoint, key), nil
	default:
		//默认local
		if endpoint == "" {
			return "", errors.New("localEndpoint is empty")
		}
		//上传文件到本地
		key := filepath.Join("public", tenantID, userID, f.Filename)
		err := c.SaveUploadedFile(f, key)
		if err != nil {
			return "", err
		}
		return strings.Join([]string{endpoint, "public", tenantID, userID, f.Filename}, "/"), nil
	}
}
