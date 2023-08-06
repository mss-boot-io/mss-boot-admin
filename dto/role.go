package dto

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

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
