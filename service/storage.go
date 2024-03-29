package service

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot/pkg/config"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/29 00:42:54
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/29 00:42:54
 */

type Storage struct{}

func (s *Storage) Upload(c *gin.Context) (string, error) {
	storageType, _ := center.GetAppConfig().GetAppConfig(c, "storage.type")
	switch storageType {
	case "s3":
		storage := config.Storage{}
		s3Type, _ := center.GetAppConfig().GetAppConfig(c, "storage.type")
		if s3Type == "" {
			s3Type = string(config.S3)
		}
		storage.Type = config.ProviderType(s3Type)

	default:
		//默认local

	}
	return "", nil
}
