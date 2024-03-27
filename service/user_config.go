package service

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/spf13/cast"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/2 00:42:39
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/2 00:42:39
 */

type UserConfig struct{}

func (e *UserConfig) Profile(ctx *gin.Context, userID string) (map[string]gin.H, error) {
	list := make([]*models.UserConfig, 0)
	err := center.GetDB(ctx, &models.UserConfig{}).
		Where("user_id = ?", userID).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]gin.H)
	for i := range list {
		if _, ok := result[list[i].Group]; !ok {
			result[list[i].Group] = make(gin.H)
		}
		result[list[i].Group][list[i].Name] = list[i].Value
	}
	return result, nil
}

func (e *UserConfig) Group(ctx *gin.Context, userID, group string) (map[string]string, error) {
	list := make([]*models.UserConfig, 0)
	err := center.GetTenant().GetDB(ctx, &models.UserConfig{}).
		Where("`group` = ?", group).
		Where("user_id = ?", userID).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for i := range list {
		result[list[i].Name] = list[i].Value
	}
	return result, nil
}

func (e *UserConfig) CreateOrUpdate(ctx *gin.Context, userID, group string, data map[string]any) error {
	var err error
	for k, v := range data {
		err = center.GetUserConfig().SetUserConfig(ctx, userID, fmt.Sprintf("%s.%s", group, k), cast.ToString(v))
		if err != nil {
			return err
		}
	}
	return nil
}
