package models

import (
	"github.com/mss-boot-io/mss-boot/pkg/enum"
)

type OptionUsage struct {
	ModelGormTenant
	OptionID string      `json:"optionId" gorm:"column:option_id;type:varchar(36);not null;index:idx_option_id;comment:选项ID"`
	UsedBy   string      `json:"usedBy" gorm:"column:used_by;type:varchar(255);comment:使用者(页面/模块)"`
	UsedAt   string      `json:"usedAt" gorm:"column:used_at;type:text;comment:使用位置描述"`
	UseCount int         `json:"useCount" gorm:"column:use_count;type:int;default:0;comment:使用次数"`
	Status   enum.Status `json:"status" gorm:"column:status;comment:状态"`
}

func (*OptionUsage) TableName() string {
	return "mss_boot_option_usages"
}