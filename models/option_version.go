package models

import (
	"github.com/mss-boot-io/mss-boot/pkg/enum"
)

type OptionVersion struct {
	ModelGormTenant
	OptionID   string       `json:"optionId" gorm:"column:option_id;type:varchar(36);not null;index:idx_option_id;comment:选项ID"`
	Version    int          `json:"version" gorm:"column:version;type:int;not null;comment:版本号"`
	Items      *OptionItems `json:"items" gorm:"column:items;type:json;comment:选项内容快照"`
	ChangedBy  string       `json:"changedBy" gorm:"column:changed_by;type:varchar(36);comment:修改人ID"`
	ChangeNote string       `json:"changeNote" gorm:"column:change_note;type:text;comment:修改说明"`
	Status     enum.Status  `json:"status" gorm:"column:status;comment:状态"`
}

func (*OptionVersion) TableName() string {
	return "mss_boot_option_versions"
}