package action

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/virtual/model"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/17 08:12:38
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/17 08:12:38
 */

// VirtualAction virtual action
type VirtualAction interface {
	String() string
	Handler() gin.HandlerFunc
	SetModel(key string, m *model.Model)
	GetModel(ctx *gin.Context) *model.Model
}

// Pagination pagination params
type Pagination struct {
	Page     int64 `form:"current" query:"current"`
	PageSize int64 `form:"pageSize" query:"pageSize"`
}

// GetPage get page
func (e *Pagination) GetPage() int64 {
	if e.Page <= 0 {
		return 1
	}
	return e.Page
}

// GetPageSize get page size
func (e *Pagination) GetPageSize() int64 {
	if e.PageSize <= 0 {
		return 10
	}
	return e.PageSize
}
