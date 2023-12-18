package models

import "github.com/mss-boot-io/mss-boot/pkg/response/actions"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/12 11:36:52
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/12 11:36:52
 */

type LanguageDefine struct {
	actions.ModelGorm
	// LanguageID 语言ID
	LanguageID string `json:"languageID" gorm:"primaryKey;column:language_id;comment:语言ID;type:varchar(64);not null" binding:"required"`
	// Group 分组
	Group string `json:"group" gorm:"column:group;comment:分组;type:varchar(20);not null" binding:"required"`
	// Key 键
	Key string `json:"key" gorm:"column:key;comment:键;type:varchar(20);not null" binding:"required"`
	// Value 值
	Value string `json:"value" gorm:"column:value;comment:值;type:varchar(100);not null" binding:"required"`
}

func (*LanguageDefine) TableName() string {
	return "mss_boot_language_defines"
}
