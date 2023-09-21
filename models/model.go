package models

import (
	log "github.com/mss-boot-io/mss-boot/core/logger"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions/authentic"
	"github.com/mss-boot-io/mss-boot/virtual/model"
	"gorm.io/gorm/schema"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/21 19:46:22
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/21 19:46:22
 */

type Model struct {
	authentic.ModelGorm
	Name        string  `gorm:"column:name;type:varchar(255);not null;comment:名称" json:"name"`
	Description string  `gorm:"column:description;type:text;not null;comment:描述" json:"description"`
	HardDeleted bool    `gorm:"column:hard_deleted;type:tinyint(1);not null;default:0;comment:是否硬删除" json:"hardDeleted"`
	Table       string  `gorm:"column:table_name;type:varchar(255);not null;comment:表名" json:"tableName"`
	Path        string  `gorm:"column:path;type:varchar(255);not null;comment:http路径" json:"path"`
	Fields      []Field `gorm:"foreignKey:ModelID;references:ID" json:"fields"`
}

func (*Model) TableName() string {
	return "mss_boot_models"
}

func (m *Model) MakeVirtualModel() *model.Model {
	mm := &model.Model{
		Table:       m.Table,
		HardDeleted: m.HardDeleted,
		Fields:      make([]*model.Field, len(m.Fields)),
	}
	for i := range m.Fields {
		mm.Fields[i] = &model.Field{
			Name:         m.Fields[i].Name,
			JsonTag:      m.Fields[i].JsonTag,
			DataType:     schema.DataType(m.Fields[i].Type),
			PrimaryKey:   m.Fields[i].PrimaryKey,
			DefaultValue: m.Fields[i].Default,
			NotNull:      m.Fields[i].NotNull,
			Unique:       m.Fields[i].UniqueIndex,
			Index:        m.Fields[i].Index,
			Comment:      m.Fields[i].Comment,
			Size:         m.Fields[i].Size,
			Search:       m.Fields[i].Search,
		}
	}
	mm.Init()
	return mm
}

// GetModels get all virtual models info
func GetModels() ([]*Model, error) {
	var models []*Model
	err := gormdb.DB.Preload("Fields").Find(&models).Error
	if err != nil {
		log.Fatalf("get models failed, %s\n", err.Error())
	}
	return models, err
}
