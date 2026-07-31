package middlewares

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/response"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/11/4 10:26:49
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/11/4 10:26:49
 */

var DefaultLimiter Limiter

type Limiter interface {
	Allow() bool
	Wait(context.Context) error
}

// LimitMiddleware 限流中间件
func LimitMiddleware() gin.HandlerFunc {
	if DefaultLimiter == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return func(c *gin.Context) {
		var count int8
	GATE:
		count++
		if !DefaultLimiter.Allow() {
			if err := DefaultLimiter.Wait(c.Request.Context()); err != nil {
				api := response.Make(c)
				api.AddError(err).Log.Error("rate limit wait error", "err", err)
				api.Err(http.StatusInternalServerError)
				return
			}
			if count > 5 {
				api := response.Make(c)
				api.Err(http.StatusTooManyRequests)
				return
			}
			goto GATE
		}
		c.Next()
	}
}
