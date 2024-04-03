package models

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot-admin/center"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/2 00:21:01
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/2 00:21:01
 */

type UserConfig struct {
	ModelGormTenant
	// UserID 用户id
	UserID string `json:"userID" gorm:"size:64;index;default:'';not null;comment:用户id" binding:"required"`
	// Name 名称
	Name string `json:"name" gorm:"size:128;index;default:'';not null;comment:名称" binding:"required"`
	// Group 分组
	Group string `json:"group" gorm:"size:128;default:'';not null;comment:分组" binding:"required"`
	// Value 值
	Value string `json:"value" gorm:"size:255;default:'';not null;comment:值"`
}

func (*UserConfig) TableName() string {
	return "mss_boot_user_configs"
}

func (e *UserConfig) SetUserConfig(ctx *gin.Context, userID, key string, value string) error {
	if key == "" {
		return nil
	}

	var group string
	keys := strings.Split(key, ".")
	if len(keys) > 1 {
		group = keys[0]
		key = strings.Join(keys[1:], ".")
	}
	c := &UserConfig{
		Group:  group,
		Name:   key,
		UserID: userID,
	}
	err := center.GetDB(ctx, e).
		Where("user_id = ?", userID).
		Where("`group` = ?", group).
		Where("name = ?", key).
		FirstOrCreate(c).Error
	if err != nil {
		return err
	}
	return center.GetTenant().GetDB(ctx, e).
		Model(e).Where("name = ?", key).
		Where("user_id = ?", userID).
		Update("value", value).Error
}

func getUserConfig(ctx *gin.Context, userID, key string) (*UserConfig, error) {
	c := &UserConfig{}
	if key == "" {
		return nil, fmt.Errorf("key is empty")
	}

	var group string
	keys := strings.Split(key, ".")
	if len(keys) > 1 {
		group = keys[0]
		key = strings.Join(keys[1:], ".")
	}
	err := center.GetTenant().GetDB(ctx, c).
		Where("`group` = ?", group).
		Where("user_id = ?", userID).
		Where("name = ?", key).
		First(c).Error
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (e *UserConfig) GetUserConfig(ctx *gin.Context, userID, key string) (string, bool) {
	c, err := getUserConfig(ctx, userID, key)
	if err != nil {
		return "", false
	}
	return c.Value, true
}
