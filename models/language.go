package models

import (
	"database/sql/driver"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/12 11:38:05
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/12 11:38:05
 */

type LanguageDefine struct {
	// ID 主键
	ID string `json:"id"`
	// Group 分组
	Group string `json:"group" gorm:"column:group;comment:分组;type:varchar(20);not null" binding:"required"`
	// Key 键
	Key string `json:"key" gorm:"column:key;comment:键;type:varchar(20);not null" binding:"required"`
	// Value 值
	Value string `json:"value" gorm:"column:value;comment:值;type:varchar(100);not null" binding:"required"`
}

type Language struct {
	actions.ModelGorm
	// Name 名称
	Name string `json:"name" gorm:"column:name;comment:名称;type:varchar(255);not null" binding:"required"`
	// Remark 备注
	Remark string `json:"remark" gorm:"column:remark;comment:备注;type:varchar(255);not null"`
	// Statue 状态
	Status enum.Status `json:"status" gorm:"column:status;comment:状态;size:10"`
	// Defines
	Defines *LanguageDefines `json:"defines,omitempty" gorm:"column:defines;comment:定义;type:json"`
}

func (*Language) TableName() string {
	return "mss_boot_languages"
}

type LanguageDefines []*LanguageDefine

func (l *LanguageDefines) Scan(val any) error {
	return json.Unmarshal(val.([]uint8), l)
}

func (l *LanguageDefines) Value() (driver.Value, error) {
	if l == nil {
		return nil, nil
	}
	for i := range *l {
		if (*l)[i].ID == "" {
			(*l)[i].ID = strings.ReplaceAll(uuid.New().String(), "-", "")
		}
	}
	return json.Marshal(*l)
}
