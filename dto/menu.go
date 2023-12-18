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
}

type GetAuthorizeRequest struct {
	RoleID string `uri:"roleID" binding:"required"`
}

type UpdateAuthorizeRequest struct {
	GetAuthorizeRequest
	Keys []string `json:"keys" binding:"required"`
}
