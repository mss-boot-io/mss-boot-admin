package admin

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/9 17:59:55
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/9 17:59:55
 */

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/mss-boot-io/mss-boot-admin/app/admin/apis"
	"github.com/mss-boot-io/mss-boot/pkg/response"
)

func InitRouter(r *gin.RouterGroup) {
	v1 := r.Group("/api")
	//r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	configCors := cors.DefaultConfig()
	configCors.AllowOrigins = []string{"*"}
	configCors.AllowCredentials = true
	configCors.AddAllowHeaders("Authorization")
	v1.Use(cors.New(configCors))
	v1.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	for i := range response.Controllers {
		response.Controllers[i].Other(r.Group("/api", cors.New(configCors)))
		e := v1.Group(response.Controllers[i].Path(), response.Controllers[i].Handlers()...)
		if action := response.Controllers[i].GetAction(response.Get); action != nil {
			e.GET("/:"+response.Controllers[i].GetKey(), action.Handler()...)
		}
		if action := response.Controllers[i].GetAction(response.Control); action != nil {
			e.POST("", action.Handler()...)
			e.PUT("/:"+response.Controllers[i].GetKey(), action.Handler()...)
		}
		if action := response.Controllers[i].GetAction(response.Create); action != nil {
			e.POST("", action.Handler()...)
		}
		if action := response.Controllers[i].GetAction(response.Update); action != nil {
			e.PUT("/:"+response.Controllers[i].GetKey(), action.Handler()...)
		}
		if action := response.Controllers[i].GetAction(response.Delete); action != nil {
			e.DELETE("/:"+response.Controllers[i].GetKey(), action.Handler()...)
		}
		if action := response.Controllers[i].GetAction(response.Search); action != nil {
			e.GET("", action.Handler()...)
		}
	}
}

var DefaultMakeRouter = &MakeRouter{
	funcs: []func(*gin.RouterGroup){InitRouter},
}

type MakeRouter struct {
	funcs []func(*gin.RouterGroup)
}

func (m *MakeRouter) SetFunc(f ...func(*gin.RouterGroup)) {
	if m.funcs == nil {
		m.funcs = make([]func(*gin.RouterGroup), 0)
	}
	m.funcs = append(m.funcs, f...)
}

func (m *MakeRouter) GetFunc() []func(*gin.RouterGroup) {
	return m.funcs
}

func (m *MakeRouter) MakeRouter(r *gin.RouterGroup) {
	for i := range m.funcs {
		m.funcs[i](r)
	}
}
