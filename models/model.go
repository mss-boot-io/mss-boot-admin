package models

import "github.com/mss-boot-io/mss-boot/pkg/response/actions"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/21 19:46:22
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/21 19:46:22
 */

type Model struct {
	actions.ModelGorm
	Name        string `gorm:"column:name;type:varchar(255);not null;comment:名称" json:"name"`
	Description string `gorm:"column:description;type:text;not null;comment:描述" json:"description"`
	Table       string `gorm:"column:table_name;type:varchar(255);not null;comment:表名" json:"table_name"`
}

func (*Model) TableName() string {
	return "mss_boot_models"
}
