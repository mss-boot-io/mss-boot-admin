package models

import (
	"database/sql/driver"
	"encoding/json"
	"strings"

	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/5 23:09:49
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/5 23:09:49
 */

type JsonRawMessage string

func (j *JsonRawMessage) Scan(val any) error {
	if val == nil {
		return nil
	}
	s := cast.ToString(val)
	*j = JsonRawMessage(s)
	return nil
}

func (j *JsonRawMessage) Value() (driver.Value, error) {
	if len(*j) == 0 {
		return nil, nil
	}
	return json.RawMessage(*j), nil
}

type ArrayString []string

func (a *ArrayString) Scan(val any) error {
	var s string
	switch val.(type) {
	case []uint8:
		// support mysql
		s = string(val.([]uint8))
	case string:
		// support sqlite
		s = val.(string)
	}
	ss := strings.Split(s, "|")
	*a = ss
	return nil
}

func (a *ArrayString) Value() (driver.Value, error) {
	return strings.Join(*a, "|"), nil

}

type Metadata map[string]string

func (m *Metadata) Scan(val any) error {
	s := val.([]uint8)
	return json.Unmarshal(s, m)
}

func (m *Metadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// ModelGormTenant compatibility model for legacy tenant columns
type ModelGormTenant struct {
	actions.ModelGorm
}

func (e *ModelGormTenant) BeforeCreate(tx *gorm.DB) (err error) {
	_, err = e.PrepareID(nil)
	return err
}

func (*ModelGormTenant) BeforeDelete(*gorm.DB) error {
	return nil
}

type ModelCreator struct {
	// CreatorID creator id
	CreatorID string `gorm:"column:creator_id;type:varchar(64);not null;index;comment:创建人ID" json:"creatorID"`
}
