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
 * @Date: 2024/1/1 11:57:51
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/1 11:57:51
 */

type OptionItem struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
	Color string `json:"color"`
	Sort  int    `json:"sort"`
}

type OptionItems []*OptionItem

func (o *OptionItems) Value() (driver.Value, error) {
	if o == nil {
		return nil, nil
	}
	for i := range *o {
		if (*o)[i].ID == "" {
			(*o)[i].ID = strings.ReplaceAll(uuid.New().String(), "-", "")
		}
	}
	return json.Marshal(o)
}

func (o *OptionItems) Scan(val any) error {
	return json.Unmarshal(val.([]uint8), o)
}

type Option struct {
	actions.ModelGorm
	// Name 选项名称
	Name string `json:"name" gorm:"column:name;type:varchar(255);not null;unique_index:idx_name;comment:选项名称"`
	// Remark 备注
	Remark string `json:"remark" gorm:"column:remark;type:varchar(255);not null;comment:备注"`
	// Items 选项内容
	Items *OptionItems `json:"items" gorm:"column:items;type:json;comment:选项内容"`
	// Status 状态
	Status enum.Status `json:"status" gorm:"column:status;comment:状态;size:10"`
}

func (*Option) TableName() string {
	return "mss_boot_options"
}
