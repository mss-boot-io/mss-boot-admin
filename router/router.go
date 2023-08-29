package router

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/mss-boot-io/mss-boot-admin-api/apis"
	_ "github.com/mss-boot-io/mss-boot-admin-api/docs"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"net/http"
)

func Init(r *gin.RouterGroup) {
	v1 := r.Group("/api")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	configCors := cors.DefaultConfig()
	configCors.AllowOrigins = []string{"http://localhost:3000"}
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
			e.GET("/:"+response.Controllers[i].GetKey(), action.Handler())
		}
		if action := response.Controllers[i].GetAction(response.Control); action != nil {
			e.POST("", action.Handler())
			e.PUT("/:"+response.Controllers[i].GetKey(), action.Handler())
		}
		if action := response.Controllers[i].GetAction(response.Delete); action != nil {
			e.DELETE("/:"+response.Controllers[i].GetKey(), action.Handler())
		}
		if action := response.Controllers[i].GetAction(response.Search); action != nil {
			e.GET("", action.Handler())
		}
	}
}
