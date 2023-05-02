/*
 * @Author: lwnmengjing
 * @Date: 2023/1/11 03:34:10
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/1/11 03:34:10
 */

package dto

import (
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

type RoleSearch struct {
	actions.Pagination `search:"inline"`
	//名称
	Name string `query:"name" form:"name" search:"type:contains;column:name"`
	//状态
	//Status enum.Status `query:"status" form:"status"`
}
