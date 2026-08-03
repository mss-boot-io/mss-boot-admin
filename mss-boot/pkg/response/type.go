package response

import "github.com/gin-gonic/gin"

/*
 * @Author: lwnmengjing
 * @Date: 2021/6/8 5:51 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/8 5:51 下午
 */

// Responses responses
type Responses interface {
	SetCode(int)
	SetTraceID(string)
	SetMsg(...string)
	SetList(any)
	SetStatus(string)
	Clone() Responses
	SetErrorCode(string)
	Error(ctx *gin.Context, code int, err error, msg ...string)
	OK(ctx *gin.Context, data any)
	PageOK(ctx *gin.Context, result any, count, pageIndex, pageSize int64)
}
