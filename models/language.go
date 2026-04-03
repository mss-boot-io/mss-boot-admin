package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"gorm.io/gorm"
)

type LanguageDefine struct {
	ID    string `json:"id"`
	Group string `json:"group" gorm:"column:group;comment:分组;type:varchar(20);not null" binding:"required"`
	Key   string `json:"key" gorm:"column:key;comment:键;type:varchar(20);not null" binding:"required"`
	Value string `json:"value" gorm:"column:value;comment:值;type:varchar(100);not null" binding:"required"`
}

type Language struct {
	ModelGormTenant
	Name    string           `json:"name" gorm:"column:name;comment:名称;type:varchar(255);not null;uniqueIndex" binding:"required"`
	Remark  string           `json:"remark" gorm:"column:remark;comment:备注;type:varchar(255);not null"`
	Status  enum.Status      `json:"status" gorm:"column:status;comment:状态;size:10"`
	Defines *LanguageDefines `json:"defines,omitempty" gorm:"column:defines;comment:定义;type:json"`
}

func (*Language) TableName() string {
	return "mss_boot_languages"
}

func (l *Language) BeforeCreate(_ *gorm.DB) error {
	if err := l.validateName(); err != nil {
		return err
	}
	return nil
}

func (l *Language) BeforeUpdate(_ *gorm.DB) error {
	if err := l.validateName(); err != nil {
		return err
	}
	return nil
}

func (l *Language) validateName() error {
	if l.Name == "" {
		return errors.New("language name is required")
	}
	matched, _ := regexp.MatchString(`^[a-z]{2,3}(-[A-Z]{2,4})?$`, l.Name)
	if !matched {
		return errors.New("language name must follow ISO 639-1 format (e.g., zh-CN, en-US)")
	}
	return nil
}

type LanguageDefines []*LanguageDefine

func (l *LanguageDefines) Scan(val any) error {
	return json.Unmarshal(val.([]uint8), l)
}

func (l *LanguageDefines) Value() (driver.Value, error) {
	if l == nil {
		return nil, nil
	}
	for i := range *l {
		if (*l)[i].ID == "" {
			(*l)[i].ID = strings.ReplaceAll(uuid.New().String(), "-", "")
		}
	}
	return json.Marshal(*l)
}
