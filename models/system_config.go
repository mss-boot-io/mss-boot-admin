package models

import (
	"github.com/mss-boot-io/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/20 11:45:42
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/20 11:45:42
 */

type SystemConfig struct {
	actions.ModelGorm
	// Name 名称
	Name string `gorm:"column:name;size:128;index;default:'';not null" json:"name" binding:"required"`
	// Ext 扩展名
	Ext source.Scheme `gorm:"column:ext;size:16;default:'';not null" json:"ext" binding:"required"`
	// Content 内容
	Content string `gorm:"column:content;type:longtext" json:"content"`
	// remark 备注
	Remark string `gorm:"column:remark;size:255;default:'';not null" json:"remark"`
	// 内置配置
	BuiltIn bool `gorm:"->" json:"isBuiltIn"`
}

func (*SystemConfig) TableName() string {
	return "mss_boot_system_configs"
}

func (e *SystemConfig) GetExtend() source.Scheme {
	return e.Ext
}

// GenerateBytes generate bytes
func (e *SystemConfig) GenerateBytes() ([]byte, error) {
	return []byte(e.Content), nil
}
