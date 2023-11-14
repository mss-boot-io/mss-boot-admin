package dto

import (
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:54:13
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:54:13
 */

type MessageReadRequest struct {
	IDS []string `json:"ids"`
}

type MessageSearch struct {
	actions.Pagination `search:"inline"`
	//状态
	//Status enum.Status `query:"status" form:"status"`
}
