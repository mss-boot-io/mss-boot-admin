package dto

import "github.com/mss-boot-io/mss-boot/pkg/response/actions"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/1 12:06:44
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/1 12:06:44
 */

type OptionSearch struct {
	actions.Pagination `search:"inline"`
	// ID
	ID string `query:"id" form:"id" search:"type:contains;column:id"`
	// Name 名称
	Name string `query:"name" form:"name" search:"type:contains;column:name"`
	// Status 状态
	Status string `query:"status" form:"status" search:"type:exact;column:status"`
}
