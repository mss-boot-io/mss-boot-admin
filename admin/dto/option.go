package dto

import (
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
)

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
	Name string `query:"name" form:"name" search:"type:contains;column:name" binding:"max=255"`
	// Category 分类
	Category string `query:"category" form:"category" search:"type:exact;column:category" binding:"max=50"`
	// Status 状态
	Status string `query:"status" form:"status" search:"type:exact;column:status" binding:"omitempty,oneof=enabled disabled"`
}

// OptionWriteRequest is the allowlisted client-owned V6 management surface.
// Resource revisions are supplied only through the strong If-Match header.
type OptionWriteRequest struct {
	Category    *string             `json:"category"`
	DisplayName *string             `json:"displayName"`
	Description *string             `json:"description"`
	Name        *string             `json:"name"`
	Remark      *string             `json:"remark"`
	Items       *models.OptionItems `json:"items"`
	Status      *string             `json:"status"`
	ChangeNote  string              `json:"changeNote"`
}

type OptionSummary struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"`
	DisplayName string    `json:"displayName"`
	Name        string    `json:"name"`
	Remark      string    `json:"remark"`
	Status      string    `json:"status"`
	Version     int       `json:"version"`
	BuiltIn     bool      `json:"builtIn"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// OptionRevisionConflictResponse lets the V6 editor keep its local draft while
// presenting the latest server base for an explicit reload or merge.
type OptionRevisionConflictResponse struct {
	Success      bool                               `json:"success" binding:"required"`
	Status       string                             `json:"status" binding:"required"`
	Code         int                                `json:"code" binding:"required"`
	ErrorCode    string                             `json:"errorCode" binding:"required"`
	ErrorMessage string                             `json:"errorMessage" binding:"required"`
	TraceID      string                             `json:"traceId" binding:"required"`
	Data         OptionRevisionConflictResponseData `json:"data" binding:"required"`
}

type OptionRevisionConflictResponseData struct {
	Current *models.Option `json:"current" binding:"required"`
}
