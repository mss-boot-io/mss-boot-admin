package models

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/25 17:24:19
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/25 17:24:19
 */

type CasbinRule struct {
	ID    int    `json:"id" gorm:"column:id"`
	PType string `json:"ptype" gorm:"column:ptype"`
	V0    string `json:"v0" gorm:"column:v0"`
	V1    string `json:"v1" gorm:"column:v1"`
	V2    string `json:"v2" gorm:"column:v2"`
	V3    string `json:"v3" gorm:"column:v3"`
	V4    string `json:"v4" gorm:"column:v4"`
	V5    string `json:"v5" gorm:"column:v5"`
}

func (*CasbinRule) TableName() string {
	return "mss_boot_casbin_rule"
}
