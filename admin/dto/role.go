package dto

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

import (
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
)

type RoleSearch struct {
	actions.Pagination `search:"inline"`
	// ID
	ID string `query:"id" form:"id" search:"type:contains;column:id"`
	//名称
	Name string `query:"name" form:"name" search:"type:contains;column:name"`
	//状态
	Status enum.Status `query:"status" form:"status" search:"type:exact;column:status"`
	//备注
	Remark string `query:"remark" form:"remark" search:"type:contains;column:remark"`
}

type SetAuthorizeRequest struct {
	RoleID string    `uri:"roleID" swaggerignore:"true" binding:"required"`
	Paths  *[]string `json:"paths" binding:"required"`
}

type GetAuthorizeResponse struct {
	RoleID string   `json:"roleID" binding:"required"`
	Paths  []string `json:"paths" binding:"required"`
	// Revision is a decimal string so JavaScript clients never lose bigint
	// precision while deriving the strong role-authorization ETag.
	Revision string `json:"revision" binding:"required"`
}

// AuthorizeRevisionConflictResponse documents the canonical 412 payload. A
// client can replace its stale draft base with Current and retry explicitly.
type AuthorizeRevisionConflictResponse struct {
	Success      bool                                  `json:"success" binding:"required"`
	Status       string                                `json:"status" binding:"required"`
	Code         int                                   `json:"code" binding:"required"`
	ErrorCode    string                                `json:"errorCode" binding:"required"`
	ErrorMessage string                                `json:"errorMessage" binding:"required"`
	TraceID      string                                `json:"traceId" binding:"required"`
	Data         AuthorizeRevisionConflictResponseData `json:"data" binding:"required"`
}

type AuthorizeRevisionConflictResponseData struct {
	Current *GetAuthorizeResponse `json:"current" binding:"required"`
}
