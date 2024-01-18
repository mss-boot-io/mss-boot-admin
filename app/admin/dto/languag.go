package dto

import "github.com/mss-boot-io/mss-boot/pkg/response/actions"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/12 12:08:11
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/12 12:08:11
 */

type LanguageSearch struct {
	actions.Pagination `search:"inline"`
	// ID
	ID string `query:"id" form:"id" search:"type:contains;column:id"`
	//名称
	Name string `query:"name" form:"name" search:"type:contains;column:name"`
}
