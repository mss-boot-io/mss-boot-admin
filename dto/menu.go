package dto

import (
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/25 17:03:45
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/25 17:03:45
 */

type MenuSearch struct {
	actions.Pagination `search:"inline"`
	// ID id
	ID string `query:"id" form:"id" search:"type:contains;column:id"`
	// Name 名称
	Name string `query:"name" form:"name" search:"type:contains;column:name"`
	// Status 状态
	Status enum.Status `query:"status" form:"status" search:"type:exact;column:status"`
	// ParentID 父级id
	ParentID string `query:"parentID" form:"parentID" search:"-"`
	// Type 类型
	Type []string `query:"type[]" form:"type[]" search:"type:in;column:type"`
	// Show 是否显示
	Show bool `query:"show" form:"show" search:"type:exact;column:hide_in_menu"`
}

type GetAuthorizeRequest struct {
	RoleID string `uri:"roleID" binding:"required"`
}

type UpdateAuthorizeRequest struct {
	GetAuthorizeRequest
	Keys []string `json:"keys" binding:"required"`
}

type MenuBindAPIRequest struct {
	MenuID string   `json:"menuID" binding:"required"`
	Paths  []string `json:"paths" binding:"required"`
}
