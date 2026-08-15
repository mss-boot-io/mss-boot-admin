package dto

import "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/7 13:26:39
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/7 13:26:39
 */

type TaskSearch struct {
	actions.Pagination `search:"inline"`
	// ID
	ID string `query:"id" form:"id" binding:"omitempty,max=64" search:"type:contains;column:id"`
	//名称
	Name string `query:"name" form:"name" binding:"omitempty,max=255" search:"type:contains;column:name"`
	//状态
	Status string `query:"status" form:"status" binding:"omitempty,oneof=enabled disabled" search:"type:exact;column:status"`
}

func (e *TaskSearch) GetPageSize() int64 {
	pageSize := e.Pagination.GetPageSize()
	if pageSize > 100 {
		return 100
	}
	return pageSize
}

type TaskOperateRequest struct {
	ID      string `uri:"id" binding:"required"`
	Operate string `uri:"operate" binding:"required"`
}

type TaskFuncItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
