package models

import (
	"fmt"
	"gorm.io/gorm/clause"
	"strings"
	"time"

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
	t, err := center.GetTenant().GetTenant(ctx)
	if err != nil {
		return err
	}
	//set cache
	if center.GetCache() != nil {
		err = center.GetCache().Set(ctx, fmt.Sprintf("%s.%s", t.GetID(), key), value, -1)
		if err != nil {
			return err
		}
	}
	c.Auth = auth
	c.Value = value
	c.UpdatedAt = time.Now()
	return center.GetTenant().GetDB(ctx, e).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"},
				{Name: "name"},
				{Name: "group"},
			},
			DoUpdates: clause.AssignmentColumns(
				[]string{
					"auth",
					"value",
					"updated_at"}),
		}).
		Create(c).Error
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
	t, err := center.GetTenant().GetTenant(ctx)
	if err != nil {
		return nil, err
	}
	if center.GetCache() == nil {
		v, _ := center.GetCache().Get(ctx, fmt.Sprintf("%s.%s", t.GetID(), key))
		if v != "" {
			c.Group = group
			c.Name = key
			c.Value = v
			return c, nil
		}
	}
	condition := &AppConfig{
		Group: group,
		Name:  key,
	}
	err = center.GetTenant().GetDB(ctx, c).
		Model(condition).
		Where(condition).
		First(c).Error
	if err != nil {
		return nil, err
	}
	_ = center.GetCache().Set(ctx, fmt.Sprintf("%s.%s", t.GetID(), key), c.Value, -1)
	return c, nil
}

func (e *AppConfig) GetAppConfig(ctx *gin.Context, key string) (string, bool) {
	c, err := getAppConfig(ctx, key)
	if err != nil {
		return "", false
	}
	return c.Value, true
}
