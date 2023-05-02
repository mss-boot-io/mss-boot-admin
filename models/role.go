/*
 * @Author: lwnmengjing
 * @Date: 2023/5/1 19:48:15
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/5/1 19:48:15
 */

package models

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
	return "roles"
}
