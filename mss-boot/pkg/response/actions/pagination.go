package actions

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/29 00:00:04
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/29 00:00:04
 */

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
