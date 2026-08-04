package models

import (
	"sort"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"gorm.io/gorm"
)

type DataScope string

const (
	DataScopeAll                    DataScope = "all"
	DataScopeCurrentDept            DataScope = "currentDept"
	DataScopeCurrentAndChildrenDept DataScope = "currentAndChildrenDept"
	DataScopeCustomDept             DataScope = "customDept"
	DataScopeSelf                   DataScope = "self"
	DataScopeSelfAndChildren        DataScope = "selfAndChildren"
	DataScopeSelfAndAllChildren     DataScope = "selfAndAllChildren"
)

type PostList []*Post

type Post struct {
	ModelGormTenant
	ParentID   string      `json:"parentID,omitempty" gorm:"column:parent_id;comment:父级id;type:varchar(255);default:'';index"`
	Name       string      `json:"name" gorm:"column:name;comment:岗位名称;type:varchar(255);not null"`
	Code       string      `json:"code" gorm:"column:code;comment:岗位编码;type:varchar(255);not null"`
	Status     enum.Status `json:"status" gorm:"column:status;comment:状态;size:10"`
	Sort       int         `json:"sort" gorm:"column:sort;comment:排序;type:int;size:5;default:0"`
	DataScope  DataScope   `json:"dataScope" gorm:"column:data_scope;comment:数据权限;type:varchar(50)"`
	DeptIDS    string      `json:"-" gorm:"column:dept_ids;comment:部门id;type:varchar(255)"`
	DeptIDSArr []string    `json:"deptIDS" gorm:"-"`
	Children   []*Post     `json:"children,omitempty" gorm:"foreignKey:ParentID;references:ID" swaggerignore:"true"`
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
	children := e.children(tx)
	ids := make([]string, 0, len(children))
	for _, child := range children {
		if child != nil {
			ids = append(ids, child.ID)
		}
	}
	return ids
}

func (e *Post) GetAllChildrenID(tx *gorm.DB) []string {
	children := e.children(tx)
	ids := make([]string, 0, len(children))
	for _, child := range children {
		if child == nil {
			continue
		}
		ids = append(ids, child.ID)
		ids = append(ids, child.GetAllChildrenID(tx)...)
	}
	return ids
}

func (e *Post) children(tx *gorm.DB) []*Post {
	if len(e.Children) > 0 || tx == nil {
		return e.Children
	}
	_ = tx.Model(&Post{}).Where("parent_id = ?", e.ID).Find(&e.Children).Error
	return e.Children
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
	for _, child := range e.Children {
		if child != nil {
			child.SortChildren()
		}
	}
}

func (e *Post) AddChildren(children []pkg.TreeImp) {
	if e.Children == nil {
		e.Children = make([]*Post, 0, len(children))
	}
	for _, child := range children {
		post, ok := child.(*Post)
		if ok && post != nil {
			e.Children = append(e.Children, post)
		}
	}
}
