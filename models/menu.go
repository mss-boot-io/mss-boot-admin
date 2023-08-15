package models

import (
	log "github.com/mss-boot-io/mss-boot/core/logger"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/15 11:28:08
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/15 11:28:08
 */

type Menu struct {
	actions.ModelGorm `json:"-" gorm:"embedded"`
	ParentID          string `json:"parentId,omitempty" gorm:"column:parent_id;comment:父级id;type:varchar(255);default:'';index"`
	Name              string `json:"name" gorm:"column:name;comment:菜单名称;type:varchar(255);not null"`
	Key               string `json:"key" gorm:"column:key;comment:菜单key;type:varchar(255);not null"`
	Breadcrumb        bool   `json:"breadcrumb,omitempty" gorm:"column:breadcrumb;comment:是否显示面包屑;type:tinyint(1);not null"`
	Ignore            bool   `json:"ignore,omitempty" gorm:"column:ignore;comment:是否忽略;type:tinyint(1);not null"`
	Children          []Menu `json:"children,omitempty" gorm:"-"`
}

func (e *Menu) BeforeSave(tx *gorm.DB) error {
	_, err := e.PrepareID(nil)
	if err != nil {
		return err
	}
	tx.Where("key = ?", e.Key).First(e)
	for i := range e.Children {
		e.Children[i].ParentID = e.ID
	}
	if len(e.Children) > 0 {
		err = tx.Save(&e.Children).Error
		if err != nil {
			log.Errorf("save menu children error: %v", err)
			return err
		}
	}
	return nil
}

func (*Menu) TableName() string {
	return "mss_boot_menus"
}
