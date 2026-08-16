package dto

import "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/19 00:14:23
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/19 00:14:23
 */

type NoticeSearch struct {
	actions.Pagination `search:"inline"`
	// UserID 用户ID
	UserID string `query:"userID" form:"userID" binding:"omitempty,max=64" search:"type:contains;column:user_id"`
	// Title 标题
	Title string `query:"title" form:"title" binding:"omitempty,max=255" search:"type:contains;column:title"`
	// Status 状态
	Status string `query:"status" form:"status" binding:"omitempty,oneof=urgent doing processing todo" search:"type:exact;column:status"`
	// Type 类型
	Type string `query:"type" form:"type" binding:"omitempty,oneof=notification message event mail" search:"type:exact;column:type"`
}

func (e *NoticeSearch) GetPage() int64 {
	page := e.Pagination.GetPage()
	if page > 10_000 {
		return 10_000
	}
	return page
}

func (e *NoticeSearch) GetPageSize() int64 {
	pageSize := e.Pagination.GetPageSize()
	if pageSize > 100 {
		return 100
	}
	return pageSize
}
