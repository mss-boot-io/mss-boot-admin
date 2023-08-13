package models

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

import (
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"gorm.io/gorm"
)

type Role struct {
	actions.ModelGorm
	Name   string      `json:"name"`
	Root   bool        `json:"root"`
	Status enum.Status `json:"status"`
	Remark string      `json:"remark" gorm:"type:text"`
}

func (e *Role) BeforeCreate(_ *gorm.DB) (err error) {
	_, err = e.PrepareID(nil)
	return err
}

func (*Role) TableName() string {
	return "mss_boot_roles"
}
