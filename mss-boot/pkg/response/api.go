package response

/*
 * @Author: lwnmengjing
 * @Date: 2021/6/22 4:48 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/22 4:48 下午
 */

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mss-boot-io/mss-boot/pkg/security"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/language"
)

// DefaultLanguage 默认语言
var DefaultLanguage = "zh-CN"

// AuthHandler 鉴权
var AuthHandler gin.HandlerFunc

var VerifyHandler func(ctx *gin.Context) security.Verifier

// API api接口
type API struct {
	Context  *gin.Context
	Log      *slog.Logger
	Error    error
	engine   *gin.RouterGroup
	language string
}

// Path 路径
func (*API) Path() string {
	return ""
}

// Handlers 路由
func (*API) Handlers() gin.HandlersChain {
	return []gin.HandlerFunc{}
}

// Create 创建
func (*API) Create(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"success":      false,
		"errorMessage": "Method Not Allowed",
	})
}

// Update 更新
func (*API) Update(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"success":      false,
		"errorMessage": "Method Not Allowed",
	})
}

// Delete 删除
func (*API) Delete(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"success":      false,
		"errorMessage": "Method Not Allowed",
	})
}

// Get 获取
func (*API) Get(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"success":      false,
		"errorMessage": "Method Not Allowed",
	})
}

// List 列表
func (*API) List(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"success":      false,
		"errorMessage": "Method Not Allowed",
	})
}

// Other 其他方法
func (*API) Other(_ *gin.RouterGroup) {}

// SetEngine 设置路由组
func (e *API) SetEngine(engine *gin.RouterGroup) {
	e.engine = engine
}

// AddError 添加错误
func (e *API) AddError(err error) *API {
	if err == nil {
		return e
	}
	if e.Error == nil {
		e.Error = err
	} else {
		e.Error = fmt.Errorf("%w; %w", e.Error, err)
	}
	if e.Error != nil {
		e.Log = e.Log.With("error", e.Error)
	}
	return e
}

// Make 设置http上下文
func (e *API) Make(c *gin.Context) *API {
	e.Context = c
	e.Log = GetRequestLogger(c)
	return e
}

// Make 设置http上下文, 返回api
func Make(c *gin.Context) *API {
	return &API{
		Context: c,
		Log:     GetRequestLogger(c),
	}
}

// Bind 参数校验
func (e *API) Bind(d any, bindings ...binding.Binding) *API {
	var err error
	if len(bindings) == 0 {
		bindings = constructor.GetBindingForGin(d)
	}
	switch e.Context.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		// 去除json、yaml、xml
		for i := range bindings {
			switch bindings[i] {
			case binding.JSON, binding.XML, binding.YAML:
				bindings = append(bindings[:i], bindings[i:]...)
			}
		}
	}
	needValidateNum := len(bindings) - 1
	for i := range bindings {
		switch bindings[i] {
		case nil:
			err = e.Context.ShouldBindUri(d)
		case binding.Query:
			err = e.Context.ShouldBindWith(d, binding.Query)
		default:
			err = e.Context.ShouldBindWith(d, bindings[i])
		}
		if err != nil && err.Error() == "EOF" {
			e.Log.Warn("request body is not present anymore. ")
			err = nil
			continue
		}
		if err != nil {
			var errs validator.ValidationErrors
			ok := errors.As(err, &errs)
			if ok && i < needValidateNum {
				err = nil
				continue
			}
			if err != nil {
				e.AddError(err)
				return e
			}
			trans, errT := transInit(e.getAcceptLanguage())
			if errT != nil {
				err = fmt.Errorf(errT.Error()+", %w", err)
				e.AddError(err)
				return e
			}
			validatorErrs := errs.Translate(trans)
			strArr := make([]string, 0)
			for k, v := range validatorErrs {
				strArr = append(strArr, k+":"+v)
			}
			if len(strArr) != 0 {
				err = errors.New(strings.Join(strArr, ","))
				e.AddError(err)
				return e
			}
		}
	}
	return e
}

// Err 通常错误数据处理
func (e *API) Err(code int, msg ...string) {
	Default.Error(e.Context, code, e.Error, msg...)
}

// OK 通常成功数据处理
func (e *API) OK(data any, msg ...string) {
	Default.OK(e.Context, data)
}

// PageOK 分页数据处理
func (e *API) PageOK(result any, count int64, pageIndex int64, pageSize int64, msg ...string) {
	Default.PageOK(e.Context, result, count, pageIndex, pageSize)
}

// getAcceptLanguage 获取当前语言
func (e *API) getAcceptLanguage() string {
	languages := language.ParseAcceptLanguage(e.Context.GetHeader("Accept-Language"), nil)
	if len(languages) == 0 {
		return DefaultLanguage
	}
	return languages[0]
}

// GetRequestLogger 获取上下文提供的日志
func GetRequestLogger(c *gin.Context) *slog.Logger {
	if l, exist := c.Get(pkg.LoggerKey); exist {
		log, ok := l.(*slog.Logger)
		if ok && log != nil {
			return log
		}
	}
	// 如果没有在上下文中放入logger
	requestID := pkg.GenerateMsgIDFromContext(c)
	return slog.Default().With(strings.ToLower(pkg.TrafficKey), requestID)
}
