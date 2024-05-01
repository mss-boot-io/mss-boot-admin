package dto

import "github.com/mss-boot-io/mss-boot-admin/pkg"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/2 17:35:45
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/2 17:35:45
 */

type ColumnType struct {
	Title              string                   `json:"title"`
	DataIndex          string                   `json:"dataIndex"`
	HideInForm         bool                     `json:"hideInForm,omitempty"`
	HideInTable        bool                     `json:"hideInTable,omitempty"`
	HideInDescriptions bool                     `json:"hideInDescriptions,omitempty"`
	ValueEnum          map[string]ValueEnumType `json:"valueEnum,omitempty"`
	ValueType          string                   `json:"valueType,omitempty"`
	ValidateRules      []pkg.BaseRule           `json:"validateRules,omitempty"`
	PK                 bool                     `json:"pk,omitempty"`
}

type ValueEnumType struct {
	Text     string `json:"text"`
	Status   string `json:"status"`
	Color    string `json:"color,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type VirtualModelObject struct {
	Name    string        `json:"name"`
	Columns []*ColumnType `json:"columns"`
}
