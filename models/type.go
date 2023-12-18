package models

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/5 23:09:49
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/5 23:09:49
 */

type ArrayString []string

func (a *ArrayString) Scan(val any) error {
	s := val.([]uint8)
	ss := strings.Split(string(s), "|")
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
