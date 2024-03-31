package models

import (
	"errors"
	"fmt"
	"github.com/mss-boot-io/mss-boot-admin/middleware"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot-admin/center"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/11 11:58:29
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/11 11:58:29
 */

type AppConfig struct {
	ModelGormTenant
	// Name 名称
	Name string `gorm:"column:name;size:128;index;default:'';not null" json:"name" binding:"required"`
	// Group 分组
	Group string `gorm:"column:group;size:128;index;default:'';not null" json:"group" binding:"required"`
	// Value 值
	Value string `gorm:"column:value;size:255;default:'';not null" json:"value"`
	// Auth 是否需要认证 如果为true，只有登录后才会返回
	Auth bool `gorm:"column:auth;default:false;not null" json:"auth"`
}

func (*AppConfig) TableName() string {
	return "mss_boot_app_configs"
}

func (e *AppConfig) SetAppConfig(ctx *gin.Context, key string, auth bool, value string) error {
	if key == "" {
		return nil
	}

	var group string
	keys := strings.Split(key, ".")
	if len(keys) > 1 {
		group = keys[0]
		key = strings.Join(keys[1:], ".")
	}
	c := &AppConfig{
		Group: group,
		Name:  key,
	}
	verify := middleware.GetVerify(ctx)
	if verify == nil {
		return errors.New("user not login")
	}
	//set cache
	err := center.GetCache().Set(ctx, fmt.Sprintf("%s.%s", verify.GetTenantID(), key), value, -1)
	if err != nil {
		return err
	}
	err = center.GetDB(ctx, e).
		Where("`group` = ?", group).
		Where("name = ?", key).
		FirstOrCreate(c).Error
	if err != nil {
		return err
	}
	c.Auth = auth
	c.Value = value
	return center.GetTenant().GetDB(ctx, e).
		Model(e).Where("name = ?", key).
		Where("`group` = ?", group).
		Select("auth", "value").
		Updates(*c).Error
}

func getAppConfig(ctx *gin.Context, key string) (*AppConfig, error) {
	c := &AppConfig{}
	if key == "" {
		return nil, fmt.Errorf("key is empty")
	}

	var group string
	keys := strings.Split(key, ".")
	if len(keys) > 1 {
		group = keys[0]
		key = strings.Join(keys[1:], ".")
	}
	verify := middleware.GetVerify(ctx)
	if verify == nil {
		return nil, errors.New("user not login")
	}
	v, _ := center.GetCache().Get(ctx, fmt.Sprintf("%s.%s", verify.GetTenantID(), key))
	if v != "" {
		c.Group = group
		c.Name = key
		c.Value = v
		return c, nil
	}

	err := center.GetTenant().GetDB(ctx, c).
		Where("name = ?", key).
		Where("`group` = ?", group).
		First(c).Error
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (e *AppConfig) GetAppConfig(ctx *gin.Context, key string) (string, bool) {
	c, err := getAppConfig(ctx, key)
	if err != nil {
		return "", false
	}
	return c.Value, true
}
