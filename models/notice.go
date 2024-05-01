package models

import (
	"time"
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
	NoticeTypeMail         NoticeType = "mail"
)

func (e NoticeType) String() string {
	return string(e)
}

type Notice struct {
	ModelGormTenant
	UserID      string     `json:"userID" gorm:"column:user_id;type:varchar(64)"`
	Title       string     `json:"title" gorm:"column:title;type:varchar(255)"`
	Key         string     `json:"key" gorm:"column:key;type:varchar(255)"`
	Read        bool       `json:"read" gorm:"column:read;size:1"`
	Avatar      string     `json:"avatar" gorm:"column:avatar;type:varchar(255)"`
	Extra       string     `json:"extra" gorm:"column:extra;type:varchar(255)"`
	Status      string     `json:"status" gorm:"column:status;size:10"`
	Description string     `json:"description" gorm:"column:description;type:text"`
	Datetime    *time.Time `json:"datetime" gorm:"column:datetime"`
	Type        NoticeType `json:"type" gorm:"column:type;type:varchar(20)"`
}

func (e *Notice) TableName() string {
	return "mss_boot_notices"
}
