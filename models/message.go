package models

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 00:33:07
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 00:33:07
 */

import (
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"gorm.io/gorm"
)

type Message struct {
	actions.ModelGorm
	Title    string    `json:"title"`
	Type     string    `json:"type"`
	SubTitle string    `json:"subTitle"`
	Avatar   string    `json:"avatar"`
	Content  string    `json:"content"`
	Time     time.Time `json:"time"`
	Tag      []string  `json:"tag" gorm:"-"`
	TagData  string    `json:"-" gorm:"type:text"`
}

func (e *Message) BeforeCreate(_ *gorm.DB) error {
	_, err := e.PrepareID(nil)
	return err
}

func (*Message) TableName() string {
	return "mss_boot_messages"
}

// Marshal json marshal
func (e *Message) Marshal() error {
	e.Tag = strings.Split(e.TagData, ",")
	return nil
}

// Unmarshal json unmarshal
func (e *Message) Unmarshal() error {
	e.TagData = strings.Join(e.Tag, ",")
	return nil
}
