package models

import (
	"gorm.io/gorm"
	"log/slog"
	"os"
	"strings"

	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/virtual/model"
	"gorm.io/gorm/schema"

	"github.com/mss-boot-io/mss-boot-admin-api/pkg"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/21 19:46:22
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/21 19:46:22
 */

type Model struct {
	actions.ModelGorm
	Name        string   `gorm:"column:name;type:varchar(255);not null;comment:名称" json:"name"`
	Description string   `gorm:"column:description;type:text;not null;comment:描述" json:"description"`
	HardDeleted bool     `gorm:"column:hard_deleted;type:tinyint(1);not null;default:0;comment:是否硬删除" json:"hardDeleted"`
	Table       string   `gorm:"column:table_name;type:varchar(255);not null;comment:表名" json:"tableName"`
	Path        string   `gorm:"column:path;type:varchar(255);not null;comment:http路径" json:"path"`
	Fields      []*Field `gorm:"foreignKey:ModelID;references:ID" json:"fields"`
	Migrate     bool     `gorm:"column:migrate;type:tinyint(1);default:0;comment:迁移" json:"migrate"`
}

func (*Model) TableName() string {
	return "mss_boot_models"
}

func (e *Model) BeforeCreate(_ *gorm.DB) error {
	_ = e.ModelGorm.BeforeCreate(nil)
	if e.Path == "" {
		e.Path = pkg.Pluralize(strings.ReplaceAll(e.Table, "_", "-"))
	}
	return nil
}

func (e *Model) MakeVirtualModel() *model.Model {
	mm := &model.Model{
		Table:       e.Table,
		HardDeleted: e.HardDeleted,
		Fields:      make([]*model.Field, len(e.Fields)),
	}
	for i := range e.Fields {
		mm.Fields[i] = &model.Field{
			Name:         e.Fields[i].Name,
			JsonTag:      e.Fields[i].JsonTag,
			DataType:     schema.DataType(e.Fields[i].Type),
			PrimaryKey:   e.Fields[i].PrimaryKey,
			DefaultValue: e.Fields[i].Default,
			NotNull:      e.Fields[i].NotNull,
			Unique:       e.Fields[i].UniqueIndex,
			Index:        e.Fields[i].Index,
			Comment:      e.Fields[i].Comment,
			Size:         e.Fields[i].Size,
			Search:       e.Fields[i].Search,
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
		slog.Error("get models failed", "err", err)
		os.Exit(-1)
	}
	return models, err
}
