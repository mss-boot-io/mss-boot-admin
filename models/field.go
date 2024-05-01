package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/pkg"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/21 19:46:33
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/21 19:46:33
 */

type Fields []*Field

func (x Fields) Len() int           { return len(x) }
func (x Fields) Less(i, j int) bool { return x[i].Sort > x[j].Sort }
func (x Fields) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

type Field struct {
	actions.ModelGorm
	ModelID        string `gorm:"column:model_id;type:varchar(64);not null;index;comment:模型id" json:"modelID"`
	Name           string `gorm:"column:name;type:varchar(64);not null;comment:名称" json:"name"`
	AssociationsID string `gorm:"column:associations_id;type:varchar(64);comment:关联id" json:"associationsID"`
	JsonTag        string `gorm:"column:json_tag;type:varchar(64);not null;comment:json标签" json:"jsonTag"`
	Label          string `gorm:"column:label;type:varchar(64);not null;comment:标签" json:"label"`
	Type           string `gorm:"column:type;type:varchar(64);not null;comment:数据类型" json:"type"`
	Size           int    `gorm:"column:size;type:int;default:0;comment:大小" json:"size"`
	Sort           uint   `gorm:"column:sort;type:int;default:0;comment:排序" json:"sort"`
	PrimaryKey     string `gorm:"column:primary_key;type:varchar(100);default:'';comment:主键" json:"primaryKey"`
	UniqueIndex    string `gorm:"column:unique_index;type:varchar(100);default:'';comment:唯一" json:"unique"`
	Index          string `gorm:"column:index;type:varchar(100);default:'';comment:索引" json:"index"`
	Default        string `gorm:"column:default;type:varchar(255);not null;comment:默认值" json:"default"`
	Comment        string `gorm:"column:comment;type:varchar(255);not null;comment:注释" json:"comment"`
	Search         string `gorm:"column:search;type:varchar(64);not null;comment:搜索类型" json:"search"`
	NotNull        bool   `gorm:"column:not_null;size:1;not null;comment:是否非空" json:"notNull"`
	ValueEnumName  string `gorm:"column:value_enum_name;type:varchar(64);not null;comment:枚举值名称" json:"valueEnumName"`
	*FieldFrontend `gorm:"column:field_frontend;type:json;comment:前端配置"`
}

func (*Field) TableName() string {
	return "mss_boot_fields"
}

func (e *Field) BeforeSave(_ *gorm.DB) error {
	if e.FieldFrontend != nil {
		for i := range e.FieldFrontend.Rules {
			if e.FieldFrontend.Rules[i].ID == "" {
				e.FieldFrontend.Rules[i].ID = pkg.SimpleID()
			}
		}
	}
	return nil
}

func (e *Field) AfterCreate(tx *gorm.DB) error {
	var m Model
	err := tx.Where("id = ?", e.ModelID).Preload("Fields").First(&m).Error
	if err != nil {
		return err
	}
	if m.GeneratedData {
		vm := m.MakeVirtualModel()
		if vm == nil {
			return fmt.Errorf("make virtual model error")
		}
		err = vm.Migrate(tx)
		if err != nil {
			return err
		}
	}
	return nil
}

type FieldFrontend struct {
	HideInTable        bool           `json:"hideInTable,omitempty"`
	HideInForm         bool           `json:"hideInForm,omitempty"`
	HideInDescriptions bool           `json:"hideInDescriptions,omitempty"`
	Width              string         `json:"width,omitempty"`
	Rules              []pkg.BaseRule `json:"rules,omitempty"`
	FormComponent      string         `json:"formComponent,omitempty"`
	TableComponent     string         `json:"tableComponent,omitempty"`
}

func (f *FieldFrontend) Value() (driver.Value, error) {
	if f == nil {
		return nil, nil
	}
	return json.Marshal(f)
}

func (f *FieldFrontend) Scan(val any) error {
	return json.Unmarshal(val.([]uint8), f)
}
