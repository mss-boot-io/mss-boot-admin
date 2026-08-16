package models

import (
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
)

type OptionVersion struct {
	ModelGormTenant
	OptionID     string       `json:"optionId" gorm:"column:option_id;type:varchar(64);not null;index:idx_option_versions_option_id;comment:选项ID"`
	Version      int          `json:"version" gorm:"column:version;type:int;not null;comment:版本号"`
	Category     string       `json:"category" gorm:"column:category;type:varchar(50);comment:选项分类快照"`
	Name         string       `json:"name" gorm:"column:name;type:varchar(255);comment:选项名称快照"`
	DisplayName  string       `json:"displayName" gorm:"column:display_name;type:varchar(255);comment:显示名称快照"`
	Description  string       `json:"description" gorm:"column:description;type:text;comment:描述快照"`
	Remark       string       `json:"remark" gorm:"column:remark;type:varchar(255);comment:备注快照"`
	Items        *OptionItems `json:"items" gorm:"column:items;type:json;comment:选项内容快照"`
	OptionStatus enum.Status  `json:"optionStatus" gorm:"column:option_status;type:varchar(10);comment:选项状态快照"`
	BuiltIn      bool         `json:"builtIn" gorm:"column:built_in;type:boolean;comment:内置标记快照"`
	ChangedBy    string       `json:"changedBy" gorm:"column:changed_by;type:varchar(64);comment:修改人ID"`
	ChangeNote   string       `json:"changeNote" gorm:"column:change_note;type:text;comment:修改说明"`
	Status       enum.Status  `json:"status" gorm:"column:status;comment:快照状态"`
}

func (*OptionVersion) TableName() string {
	return "mss_boot_option_versions"
}
