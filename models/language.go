package models

import (
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/12 11:38:05
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/12 11:38:05
 */

type Language struct {
	actions.ModelGorm
	// Name 名称
	Name string `json:"name" gorm:"column:name;comment:名称;type:varchar(255);not null"`
	// Statue 状态
	Status enum.Status `json:"status" gorm:"column:status;comment:状态;type:tinyint(1);not null;default:1"`
	// Defines
	Defines []LanguageDefine `json:"defines,omitempty" gorm:"foreignKey:LanguageID;references:ID"`
}

func (*Language) TableName() string {
	return "mss_boot_languages"
}

func (e *Language) BeforeSave(_ *gorm.DB) error {
	err := e.ModelGorm.BeforeCreate(nil)
	if err != nil {
		return err
	}
	for i := range e.Defines {
		e.Defines[i].LanguageID = e.ID
	}
	return nil
}
