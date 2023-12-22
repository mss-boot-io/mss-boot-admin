package models

import (
	"sort"

	"github.com/mss-boot-io/mss-boot-admin-api/pkg"

	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/15 11:28:08
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/15 11:28:08
 */

type MenuList []*Menu

type Menu struct {
	actions.ModelGorm
	// ParentID 父级id
	ParentID string `json:"parentID,omitempty" gorm:"column:parent_id;comment:父级id;type:varchar(255);default:'';index"`
	// Name 菜单名称
	Name string `json:"name" gorm:"column:name;comment:菜单名称;type:varchar(255);not null"`
	//// Title 菜单标题
	//Title string `json:"title" gorm:"column:title;comment:菜单标题;type:varchar(255);not null"`
	// Path 路由
	Path string `json:"path" gorm:"column:path;comment:菜单路径;type:varchar(255);not null"`
	// Method 请求方法
	Method string `json:"method" gorm:"column:method;comment:请求方法;type:varchar(10);default:'GET'"`
	// Component 组件
	Component string `json:"component" gorm:"column:component;comment:菜单组件;type:varchar(255);not null"`
	// Icon 图标
	Icon string `json:"icon" gorm:"column:icon;comment:菜单图标;type:varchar(255);not null"`
	// Target 新页面打开
	Target string `json:"target" gorm:"column:target;comment:菜单打开方式;type:varchar(255);not null"`
	// HeaderRender 不展示顶栏
	HeaderRender bool `json:"headerRender,omitempty" gorm:"column:header_render;comment:是否显示在头部;type:tinyint(1);not null"`
	// FooterRender 不展示页脚
	FooterRender bool `json:"footerRender,omitempty" gorm:"column:footer_render;comment:是否显示在底部;type:tinyint(1);not null"`
	// MenuRender 不展示菜单
	MenuRender bool `json:"menuRender,omitempty" gorm:"column:menu_render;comment:是否显示在菜单;type:tinyint(1);not null"`
	// MenuHeaderRender 不展示菜单头部
	MenuHeaderRender bool `json:"menuHeaderRender,omitempty" gorm:"column:menu_header_render;comment:是否显示在菜单头部;type:tinyint(1);not null"`
	// Access 权限配置，需要与 plugin-access 插件配合使用
	Access string `json:"access,omitempty" gorm:"-"`
	// HideChildrenInMenu 隐藏子菜单
	HideChildrenInMenu bool `json:"hideChildrenInMenu,omitempty" gorm:"column:hide_children_in_menu;comment:是否隐藏子菜单;type:tinyint(1);not null"`
	// HideInMenu 隐藏自己和子菜单
	HideInMenu bool `json:"hideInMenu,omitempty" gorm:"column:hide_in_menu;comment:是否隐藏菜单;type:tinyint(1);not null"`
	// HideInBreadcrumb 在面包屑中隐藏
	HideInBreadcrumb bool `json:"hideInBreadcrumb,omitempty" gorm:"column:hide_in_breadcrumb;comment:是否隐藏面包屑;type:tinyint(1);not null"`
	// FlatMenu 子项往上提，仍旧展示
	FlatMenu bool `json:"flatMenu,omitempty" gorm:"column:flat_menu;comment:是否平级菜单;type:tinyint(1);not null"`
	// FixedHeader 固定顶栏
	FixedHeader bool `json:"fixedHeader,omitempty" gorm:"column:fixed_header;comment:是否固定头部;type:tinyint(1);not null"`
	// FixedSideBar 固定菜单
	FixSiderbar bool `json:"fixSiderbar,omitempty" gorm:"column:fix_siderbar;comment:是否固定菜单;type:tinyint(1);not null"`
	// NavTheme 导航菜单的主题
	NavTheme string `json:"navTheme,omitempty" gorm:"column:nav_theme;comment:菜单主题;type:varchar(255);not null"`
	// Layout 导航菜单的位置, side 为正常模式，top菜单显示在顶部，mix 两种兼有
	Layout string `json:"layout,omitempty" gorm:"column:layout;comment:布局;type:varchar(255);not null"`
	// HeaderTheme 顶部导航的主题，mix 模式生效
	HeaderTheme string `json:"headerTheme,omitempty" gorm:"column:header_theme;comment:头部主题;type:varchar(255);not null"`
	// Type 菜单类型
	Type pkg.AccessType `json:"type" gorm:"column:type;comment:菜单类型;type:varchar(20);not null"`
	// Permission 菜单权限
	Permission string `json:"permission" gorm:"column:permission;comment:菜单权限;type:varchar(255);not null"`
	// Status 状态
	Status enum.Status `json:"status" gorm:"column:status;comment:状态;size:10"`
	// Sort 排序
	Sort int `json:"sort" gorm:"column:sort;comment:排序;type:int(11);not null;default:0"`
	// Children 子菜单
	Children []*Menu `json:"children,omitempty" gorm:"foreignKey:ParentID;references:ID" swaggerignore:"true"`
}

func (x MenuList) Len() int           { return len(x) }
func (x MenuList) Less(i, j int) bool { return x[i].Sort > x[j].Sort }
func (x MenuList) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func (e *Menu) BeforeCreate(tx *gorm.DB) error {
	err := e.ModelGorm.BeforeCreate(tx)
	if err != nil {
		return err
	}
	if e.Status == "" {
		e.Status = enum.Enabled
	}
	if e.Type == pkg.APIAccessType ||
		e.Type == pkg.ComponentAccessType {
		e.HideInMenu = true
	}
	return nil
}

func (e *Menu) BeforeSave(_ *gorm.DB) error {
	if e.Type == pkg.APIAccessType ||
		e.Type == pkg.ComponentAccessType {
		e.HideInMenu = true
	}
	for i := range e.Children {
		e.Children[i].ParentID = e.ID
	}
	return nil
}

//func (e *Menu) AfterFind(_ *gorm.DB) error {
//	e.Title = e.Name
//	return nil
//}

func (*Menu) TableName() string {
	return "mss_boot_menus"
}

// GetMenuTree get menu tree
func GetMenuTree(list []*Menu) []*Menu {
	listMap := make(map[string]*Menu)
	for i := range list {
		listMap[list[i].ID] = list[i]
	}
	for i := range list {
		if list[i].ParentID != "" {
			if parent, ok := listMap[list[i].ParentID]; ok {
				if parent.Children == nil {
					parent.Children = make([]*Menu, 0)
				}
				parent.Children = append(parent.Children, list[i])
			}
		}
	}
	var tree MenuList = make([]*Menu, 0)
	for i := range list {
		if list[i].ParentID == "" {
			tree = append(tree, list[i])
		}
	}
	sort.Sort(tree)
	return tree
}

// CompleteName complete menu name
func CompleteName(tree []*Menu) []*Menu {
	for i := range tree {
		for j := range tree[i].Children {
			tree[i].Children[j].Name = tree[i].Name + "." + tree[i].Children[j].Name
		}
		if len(tree[i].Children) > 0 {
			tree[i].Children = CompleteName(tree[i].Children)
		}
	}
	return tree
}
