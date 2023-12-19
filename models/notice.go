package models

import (
	"time"

	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/18 23:50:18
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/18 23:50:18
 */

type NoticeType string

const (
	NoticeTypeNotification NoticeType = "notification"
	NoticeTypeMessage      NoticeType = "message"
	NoticeTypeEvent        NoticeType = "event"
)

type Notice struct {
	actions.ModelGorm
	UserID   string     `json:"userID" gorm:"column:user_id;type:varchar(64)"`
	Title    string     `json:"title" gorm:"column:title;type:varchar(255)"`
	Key      string     `json:"key" gorm:"column:key;type:varchar(255)"`
	Read     bool       `json:"read" gorm:"column:read;type:tinyint(1)"`
	Avatar   string     `json:"avatar" gorm:"column:avatar;type:varchar(255)"`
	Extra    string     `json:"extra" gorm:"column:extra;type:varchar(255)"`
	Status   string     `json:"status" gorm:"column:status;type:varchar(20)"`
	Datetime time.Time  `json:"datetime" gorm:"column:datetime;type:datetime"`
	Type     NoticeType `json:"type" gorm:"column:type;type:varchar(20)"`
}

func (e *Notice) TableName() string {
	return "mss_boot_notices"
}
