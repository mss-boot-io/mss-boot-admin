package models

import (
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/21 19:46:33
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/21 19:46:33
 */

type Field struct {
	actions.ModelGorm
	ModelID     string `gorm:"column:model_id;type:varchar(64);not null;index;comment:模型id" json:"modelID"`
	Name        string `gorm:"column:name;type:varchar(64);not null;comment:名称" json:"name"`
	JsonTag     string `gorm:"column:json_tag;type:varchar(64);not null;comment:json标签" json:"json_tag"`
	Label       string `gorm:"column:label;type:varchar(64);not null;comment:标签" json:"label"`
	Show        []byte `gorm:"column:show;type:json;comment:显示" json:"show"`
	Type        string `gorm:"column:type;type:varchar(64);not null;comment:数据类型" json:"type"`
	Size        int    `gorm:"column:size;type:int;default:0;comment:大小" json:"size"`
	PrimaryKey  string `gorm:"column:primary_key;type:varchar(100);default:'';comment:主键" json:"primary_key"`
	UniqueIndex string `gorm:"column:unique_index;type:varchar(100);default:'';comment:唯一" json:"unique"`
	Index       string `gorm:"column:index;type:varchar(100);default:'';comment:索引" json:"index"`
	Default     string `gorm:"column:default;type:varchar(255);not null;comment:默认值" json:"default"`
	Comment     string `gorm:"column:comment;type:varchar(255);not null;comment:注释" json:"comment"`
	Search      string `gorm:"column:search;type:varchar(64);not null;comment:搜索类型" json:"search"`
	NotNull     bool   `gorm:"column:not_null;type:tinyint(1);not null;comment:是否非空" json:"notNull"`
}

func (*Field) TableName() string {
	return "mss_boot_fields"
}
