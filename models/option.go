package models

import (
	"database/sql/driver"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/1 11:57:51
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/1 11:57:51
 */

type OptionItem struct {
	ID    string         `json:"id"`
	Key   string         `json:"key"`
	Label string         `json:"label"`
	Value string         `json:"value"`
	Color string         `json:"color"`
	Sort  int            `json:"sort"`
	Icon  string         `json:"icon,omitempty"`
	Extra map[string]any `json:"extra,omitempty"`
}

type OptionItems []*OptionItem

func (o *OptionItems) Value() (driver.Value, error) {
	if o == nil {
		return nil, nil
	}
	for i := range *o {
		if (*o)[i].ID == "" {
			(*o)[i].ID = strings.ReplaceAll(uuid.New().String(), "-", "")
		}
	}
	return json.Marshal(o)
}

func (o *OptionItems) Scan(val any) error {
	return json.Unmarshal(val.([]uint8), o)
}

type Option struct {
	ModelGormTenant
	// Category 选项分类
	Category string `json:"category" gorm:"column:category;type:varchar(50);not null;index:idx_category;comment:选项分类"`
	// DisplayName 显示名称
	DisplayName string `json:"displayName" gorm:"column:display_name;type:varchar(255);comment:显示名称"`
	// Description 描述
	Description string `json:"description" gorm:"column:description;type:text;comment:描述"`
	// Name 选项名称
	Name string `json:"name" gorm:"column:name;type:varchar(255);not null;unique_index:idx_name;comment:选项名称"`
	// Remark 备注
	Remark string `json:"remark" gorm:"column:remark;type:varchar(255);not null;comment:备注"`
	// Items 选项内容
	Items *OptionItems `json:"items" gorm:"column:items;type:json;comment:选项内容"`
	// Status 状态
	Status enum.Status `json:"status" gorm:"column:status;comment:状态;size:10"`
	// Version 版本号
	Version int `json:"version" gorm:"column:version;type:int;default:1;comment:版本号"`
	// BuiltIn 是否内置
	BuiltIn bool `json:"builtIn" gorm:"column:built_in;type:boolean;default:false;comment:是否内置"`
}

func (*Option) TableName() string {
	return "mss_boot_options"
}
