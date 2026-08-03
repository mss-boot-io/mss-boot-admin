package gorms

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/3/9 01:16:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/3/9 01:16:26
 */

import (
	"gorm.io/gorm"
)

// Driver db type
var Driver = "mysql"

// GeneralDelDto  general delete dto
type GeneralDelDto struct {
	ID  int   `uri:"id" json:"id" validate:"required"`
	Ids []int `json:"ids"`
}

// GetIds get ids
func (g GeneralDelDto) GetIds() []int {
	ids := make([]int, 0)
	if g.ID != 0 {
		ids = append(ids, g.ID)
	}
	if len(g.Ids) > 0 {
		for _, id := range g.Ids {
			if id > 0 {
				ids = append(ids, id)
			}
		}
	} else {
		if g.ID > 0 {
			ids = append(ids, g.ID)
		}
	}
	if len(ids) <= 0 {
		// 方式全部删除
		ids = append(ids, 0)
	}
	return ids
}

// GeneralGetDto general get dto
type GeneralGetDto struct {
	ID int `uri:"id" json:"id" validate:"required"`
}

// MakeCondition  make condition
func MakeCondition(q any) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		condition := &GormCondition{
			GormPublic: GormPublic{},
			Join:       make([]*GormJoin, 0),
		}
		ResolveSearchQuery(Driver, q, condition)
		for _, join := range condition.Join {
			if join == nil {
				continue
			}
			db = db.Joins(join.JoinOn)
			for k, v := range join.Where {
				db = db.Where(k, v...)
			}
			for k, v := range join.Or {
				db = db.Or(k, v...)
			}
			for _, o := range join.Order {
				db = db.Order(o)
			}
		}
		for k, v := range condition.Where {
			db = db.Where(k, v...)
		}
		for k, v := range condition.Or {
			db = db.Or(k, v...)
		}
		for _, o := range condition.Order {
			db = db.Order(o)
		}
		return db
	}
}

// Paginate pagination
func Paginate(pageSize, pageIndex int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		offset := (pageIndex - 1) * pageSize
		if offset < 0 {
			offset = 0
		}
		return db.Offset(offset).Limit(pageSize)
	}
}
