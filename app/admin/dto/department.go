package dto

import "github.com/mss-boot-io/mss-boot/pkg/response/actions"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/29 00:17:28
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/29 00:17:28
 */

type DepartmentSearch struct {
	actions.Pagination `search:"inline"`
	// ID
	ID string `query:"id" form:"id" search:"type:contains;column:id"`
	// Name 名称
	Name string `query:"name" form:"name" search:"type:contains;column:name"`
}
