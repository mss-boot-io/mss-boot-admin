package dto

import "github.com/mss-boot-io/mss-boot/pkg/response/actions"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/19 00:14:23
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/19 00:14:23
 */

type NoticeSearch struct {
	actions.Pagination `search:"inline"`
	// UserID 用户ID
	UserID string `query:"userID" form:"userID" search:"type:contains;column:user_id"`
	// Title 标题
	Title string `query:"title" form:"title" search:"type:contains;column:title"`
	// Status 状态
	Status string `query:"status" form:"status" search:"type:exact;column:status"`
	// Type 类型
	Type string `query:"type" form:"type" search:"type:exact;column:type"`
}
