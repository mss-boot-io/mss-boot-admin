package migration

import (
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/26 11:04:27
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/26 11:04:27
 */

type Version interface {
	schema.Tabler
	SetVersion(string)
	Done(*gorm.DB) (bool, error)
}
