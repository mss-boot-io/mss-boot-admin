package dto

import "github.com/mss-boot-io/mss-boot/pkg/response/actions"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 22:16:32
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 22:16:32
 */

type UserSearch struct {
	actions.Pagination `search:"inline"`
	//名称
	Name string `query:"name" form:"name" search:"type:contains;column:name"`
}
