/*
 * @Author: lwnmengjing
 * @Date: 2023/5/1 19:46:15
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/5/1 19:46:15
 */

package router

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/mss-boot-io/mss-boot-admin-api/apis"
	_ "github.com/mss-boot-io/mss-boot-admin-api/docs"
)

func Init(r *gin.RouterGroup) {
	v1 := r.Group("/api/v1")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
