package models

import (
	"sort"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/pkg"

	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/24 18:16:17
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/24 18:16:17
 */

type DataScope string

const (
	// DataScopeAll 全部数据权限
	DataScopeAll DataScope = "all"
	// DataScopeCurrentDept 当前部门数据权限
	DataScopeCurrentDept DataScope = "current_dept"
	// DataScopeCurrentAndChildrenDept 当前部门及以下数据权限
	DataScopeCurrentAndChildrenDept DataScope = "current_and_children_dept"
	// DataScopeCustomDept 自定义部门
	DataScopeCustomDept DataScope = "custom_dept"
	// DataScopeSelf 自己数据权限
	DataScopeSelf DataScope = "self"
	// DataScopeSelfAndChildren 自己和直属下级
	DataScopeSelfAndChildren DataScope = "self_and_children"
	// DataScopeSelfAndAllChildren 自己和全部下级
	DataScopeSelfAndAllChildren DataScope = "self_and_all_children"
)

type PostList []*Post

type Post struct {
	ModelGormTenant
	// ParentID 父级id
	ParentID string `json:"parentID,omitempty" gorm:"column:parent_id;comment:父级id;type:varchar(255);default:'';index"`
	// Name 岗位名称
	Name string `json:"name" gorm:"column:name;comment:岗位名称;type:varchar(255);not null"`
	// Code 岗位编码
	Code string `json:"code" gorm:"column:code;comment:岗位编码;type:varchar(255);not null"`
	// Status 状态
	Status enum.Status `json:"status" gorm:"column:status;comment:状态;size:10"`
	// Sort 排序
	Sort int `json:"sort" gorm:"column:sort;comment:排序;type:tinyint;size:5;defualt:0"`
	// DataScope 数据权限
	DataScope DataScope `json:"dataScope" gorm:"column:data_scope;comment:数据权限;type:varchar(50)"`
	// DeptIDS 部门id
	DeptIDS string `json:"-" gorm:"column:dept_ids;comment:部门id;type:varchar(255)"` // 部门id
	// DeptIDSArr 部门id数组
	DeptIDSArr []string `json:"deptIDS" gorm:"-"`
	// Children 子岗位
	Children []*Post `json:"children,omitempty" gorm:"foreignKey:ParentID;references:ID" swaggerignore:"true"`
}

func (e *Post) BeforeSave(_ *gorm.DB) error {
	if len(e.DeptIDSArr) > 0 {
		e.DeptIDS = strings.Join(e.DeptIDSArr, ",")
	}
	return nil
}

func (e *Post) AfterFind(_ *gorm.DB) error {
	if e.DeptIDS != "" {
		e.DeptIDSArr = strings.Split(e.DeptIDS, ",")
	}
	return nil
}

func (*Post) TableName() string {
	return "mss_boot_posts"
}

func (e *Post) GetChildrenID(tx *gorm.DB) []string {
	ids := make([]string, 0)
	if len(e.Children) == 0 {
		tx.Model(&Post{}).Where("parent_id = ?", e.ID).Find(&e.Children)
	}
	for i := range e.Children {
		ids = append(ids, e.Children[i].ID)
	}
	return ids
}

func (e *Post) GetAllChildrenID(tx *gorm.DB) []string {
	ids := make([]string, 0)
	if len(e.Children) == 0 {
		tx.Model(&Post{}).Where("parent_id = ?", e.ID).Find(&e.Children)
	}
	for i := range e.Children {
		ids = append(ids, e.Children[i].GetChildrenID(tx)...)
	}
	return ids
}

func (x PostList) Len() int           { return len(x) }
func (x PostList) Less(i, j int) bool { return x[i].Sort > x[j].Sort }
func (x PostList) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func (e *Post) GetIndex() string {
	return e.ID
}

func (e *Post) GetParentID() string {
	return e.ParentID
}

func (e *Post) SortChildren() {
	if len(e.Children) == 0 {
		return
	}
	sort.Sort(PostList(e.Children))
	for i := range e.Children {
		e.Children[i].SortChildren()
	}
}

func (e *Post) AddChildren(children []pkg.TreeImp) {
	if e.Children == nil {
		e.Children = make([]*Post, 0)
	}
	for i := range children {
		e.Children = append(e.Children, children[i].(*Post))
	}
}
