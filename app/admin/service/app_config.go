package service

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/app/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/spf13/cast"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/11 22:01:11
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/11 22:01:11
 */

type AppConfig struct{}

func (e *AppConfig) Group(ctx *gin.Context, group string) (map[string]string, error) {
	list := make([]*models.AppConfig, 0)
	err := center.GetTenant().GetDB(ctx, &models.AppConfig{}).Where("`group` = ?", group).Find(&list).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for i := range list {
		result[list[i].Name] = list[i].Value
	}
	return result, nil
}

func (e *AppConfig) CreateOrUpdate(ctx *gin.Context, group string, data map[string]any) error {
	var err error
	for k, v := range data {
		err = center.GetAppConfig().SetAppConfig(ctx, fmt.Sprintf("%s.%s", group, k), cast.ToString(v))
		if err != nil {
			return err
		}
	}
	return nil
}
