package response

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg"
)

// Response response
type Response struct {
	Success      bool   `json:"success,omitempty"`
	Status       string `json:"status,omitempty"`
	Code         int    `json:"code,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	ShowType     uint8  `json:"showType,omitempty"`
	TraceID      string `json:"traceId,omitempty"`
	Host         string `json:"host,omitempty"`
}

type response struct {
	Response
	Data any `json:"data,omitempty"`
}

// Page page
type Page struct {
	Count    int64 `json:"total"`
	Current  int64 `json:"current"`
	PageSize int64 `json:"pageSize"`
}

type page struct {
	Page
	response
}

// SetList set data
func (e *response) SetList(data any) {
	e.Data = data
}

// Clone clone
func (e *response) Clone() Responses {
	clone := *e
	return &clone
}

// SetTraceID set trace id
func (e *response) SetTraceID(id string) {
	e.TraceID = id
}

// SetMsg set msg
func (e *response) SetMsg(s ...string) {
	e.ErrorMessage += strings.Join(s, ",")
}

// SetCode set code
func (e *response) SetCode(code int) {
	e.Code = code
}

func (e *response) SetErrorCode(errorCode string) {
	e.ErrorCode = errorCode
}

func (e *response) SetStatus(status string) {
	e.Status = status
}

// DefaultError error
func (e *response) Error(c *gin.Context, code int, err error, msg ...string) {
	checkContext(c)
	res := Default.Clone()
	if msg == nil {
		msg = make([]string, 0)
	}

	if err != nil {
		var responseError Error
		if errors.As(err, &responseError) {
			msg = append(msg, responseError.ErrorMsg())
			res.SetErrorCode(responseError.ErrorCode())
		} else {
			msg = append(msg, err.Error())
		}
	}
	res.SetMsg(msg...)
	res.SetTraceID(pkg.GenerateMsgIDFromContext(c))
	res.SetCode(code)
	res.SetStatus("error")
	c.Set("result", res)
	c.Set("status", code)
	c.AbortWithStatusJSON(code, res)
}

// OK ok
func (e *response) OK(c *gin.Context, data any) {
	checkContext(c)
	res := Default.Clone()
	res.SetList(data)
	res.SetTraceID(pkg.GenerateMsgIDFromContext(c))
	status := http.StatusOK
	switch c.Request.Method {
	case http.MethodDelete:
		status = http.StatusNoContent
	case http.MethodPost:
		status = http.StatusCreated
	}
	res.SetCode(status)
	if data == nil {
		c.AbortWithStatus(status)
		return
	}
	c.AbortWithStatusJSON(status, data)
}

// PageOK page ok
func (e *response) PageOK(c *gin.Context, result any, count, pageIndex, pageSize int64) {
	checkContext(c)
	var res page
	res.Count = count
	res.Current = pageIndex
	res.PageSize = pageSize
	res.SetList(result)
	res.SetTraceID(pkg.GenerateMsgIDFromContext(c))
	c.Set("result", res)
	c.Set("status", http.StatusOK)
	c.AbortWithStatusJSON(http.StatusOK, res)
}
