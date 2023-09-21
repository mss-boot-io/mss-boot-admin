package dto

import "github.com/mss-boot-io/mss-boot/pkg/response/actions/authentic"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/18 12:56:46
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/18 12:56:46
 */

type ModelSearch struct {
	authentic.Pagination `search:"inline"`
	// ID
	ID string `query:"id" form:"id" search:"type:contains;column:id"`
	//名称
	Name string `query:"name" form:"name" search:"type:contains;column:name"`
}
