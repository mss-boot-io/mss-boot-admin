package models

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 00:33:07
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 00:33:07
 */

import (
	"strings"

	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

type Message struct {
	actions.ModelGorm
	UserID   string   `json:"userID"`
	Title    string   `json:"title"`
	Type     string   `json:"type"`
	SubTitle string   `json:"subTitle"`
	Avatar   string   `json:"avatar"`
	Content  string   `json:"content"`
	Time     string   `json:"time"`
	Tag      []string `json:"tag" gorm:"-"`
	TagData  string   `json:"-" gorm:"type:text"`
	Read     bool     `json:"read"`
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
