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
	"github.com/mss-boot-io/mss-boot/pkg/response"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/mss-boot-io/mss-boot-admin-api/apis"
	_ "github.com/mss-boot-io/mss-boot-admin-api/docs"
)

func Init(r *gin.RouterGroup) {
	v1 := r.Group("/api")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.Use(cors.Default())

	var e *gin.RouterGroup
	for i := range response.Controllers {
		response.Controllers[i].Other(v1)
		e = v1.Group(response.Controllers[i].Path(), response.Controllers[i].Handlers()...)
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
