package models

import (
	"sort"

	"github.com/mss-boot-io/mss-boot-admin/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/24 18:11:32
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/24 18:11:32
 */

type DepartmentList []*Department

type Department struct {
	ModelGormTenant
	// ParentID 父级id
	ParentID string `json:"parentID,omitempty" gorm:"column:parent_id;comment:父级id;type:varchar(255);default:'';index"`
	// Name 部门名称
	Name string `json:"name" gorm:"column:name;comment:部门名称;type:varchar(255);not null"`
	// LeaderID 部分负责人ID
	LeaderID string `json:"leaderID" gorm:"column:leader_id;comment:部分负责人id;type:varchar(64)"`
	// Phone 联系电话
	Phone string `json:"phone" gorm:"column:phone;comment:联系电话;type:varchar(255)"`
	// Email 邮箱
	Email string `json:"email" gorm:"column:email;comment:邮箱;type:varchar(255)"`
	// Code 部门编码
	Code string `json:"code" gorm:"column:code;comment:部门编码;type:varchar(255);not null"`
	// Status 状态
	Status enum.Status `json:"status" gorm:"column:status;comment:状态;size:10"`
	// Sort 排序
	Sort int `json:"sort" gorm:"column:sort;comment:排序;type:tinyint;size:5;defualt:0"`
	// Children 子部门
	Children []*Department `json:"children,omitempty" gorm:"foreignKey:ParentID;references:ID" swaggerignore:"true"`
}

func (*Department) TableName() string {
	return "mss_boot_departments"
}

func (x DepartmentList) Len() int           { return len(x) }
func (x DepartmentList) Less(i, j int) bool { return x[i].Sort > x[j].Sort }
func (x DepartmentList) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func (e *Department) GetAllChildrenID(tx *gorm.DB) []string {
	ids := []string{e.ID}
	if len(e.Children) == 0 {
		tx.Model(&Department{}).Where("parent_id = ?", e.ID).Find(&e.Children)
	}
	for i := range e.Children {
		ids = append(ids, e.Children[i].GetAllChildrenID(tx)...)
	}
	return ids
}

func (e *Department) GetIndex() string {
	return e.ID
}

func (e *Department) GetParentID() string {
	return e.ParentID
}

func (e *Department) AddChildren(children []pkg.TreeImp) {
	if e.Children == nil {
		e.Children = make([]*Department, 0)
	}
	for i := range children {
		e.Children = append(e.Children, children[i].(*Department))
	}
}

func (e *Department) SortChildren() {
	if len(e.Children) == 0 {
		return
	}
	sort.Sort(DepartmentList(e.Children))
	for i := range e.Children {
		e.Children[i].SortChildren()
	}
}
