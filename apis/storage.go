package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/service"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/29 00:36:27
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/29 00:36:27
 */

type Storage struct {
	*controller.Simple
	service service.Storage
}

func (*Storage) GetKey() string {
	return "storage"
}

func (*Storage) GetAction(string) response.Action {
	return nil
}

func (e *Storage) Other(r *gin.RouterGroup) {
	r.POST("/storage/upload", e.Upload)
}

func (e *Storage) Upload(c *gin.Context) {
	//api := response.Make(c)

}
