package dto

import "github.com/mss-boot-io/mss-boot/pkg/response/actions/authentic"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/24 01:48:31
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/24 01:48:31
 */

type APISearch struct {
	authentic.Pagination `search:"inline"`
	// ID
	ID string `query:"id" form:"id" search:"type:contains;column:id"`
}
