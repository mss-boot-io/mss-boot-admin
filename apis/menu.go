package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
	"net/http"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/15 13:41:22
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/15 13:41:22
 */

func init() {
	e := &Menu{
		Simple: controller.NewSimple(
			controller.WithAuth(false),
			controller.WithModel(new(models.Menu)),
			controller.WithSearch(new(dto.RoleSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Menu struct {
	*controller.Simple
}

func (e *Menu) Other(r *gin.RouterGroup) {
	r.GET("/menu/tree", e.Tree)
}

func (e *Menu) Tree(ctx *gin.Context) {
	api := response.Make(ctx)
	list := make([]*models.Menu, 0)
	err := gormdb.DB.WithContext(ctx).Find(&list).Error
	if err != nil {
		api.Log.Errorf("get menu tree error: %v", err)
		api.Err(http.StatusInternalServerError, err.Error())
		return
	}
	listMap := make(map[string]*models.Menu)
	for i := range list {
		listMap[list[i].ID] = list[i]
	}
	for i := range list {
		if list[i].ParentID != "" {
			if parent, ok := listMap[list[i].ParentID]; ok {
				if parent.Children == nil {
					parent.Children = make([]models.Menu, 0)
				}
				parent.Children = append(parent.Children, *list[i])
				delete(listMap, list[i].ID)
			}
		}
	}
	list = make([]*models.Menu, 0, len(listMap))
	for id := range listMap {
		list = append(list, listMap[id])
	}
	api.OK(list)
}
