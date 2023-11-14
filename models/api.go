package models

import (
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/14 08:41:16
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/14 08:41:16
 */

type AccessType string

const (
	MenuAccessType AccessType = "MENU"
	APIAccessType  AccessType = "API"
)

func (a AccessType) String() string {
	return string(a)
}

type API struct {
	actions.ModelGorm
	Name    string `json:"name"`
	Path    string `json:"path"`
	Method  string `json:"method"`
	Handler string `json:"handler"`
	History bool   `json:"history"`
}

func (*API) TableName() string {
	return "mss_boot_api"
}

func SaveAPI(routes gin.RoutesInfo) error {
	list := make([]*API, 0)
	for i := range routes {
		api := &API{
			Name:    routes[i].Path,
			Method:  routes[i].Method,
			Handler: routes[i].Handler,
		}
		ps := strings.Split(routes[i].Path, "/")
		for j := range ps {
			if strings.HasPrefix(ps[j], ":") {
				ps[j] = "*"
				continue
			}
			if strings.HasPrefix(ps[j], "*") {
				ps[j] = "*"
				continue
			}
		}
		api.Path = strings.Join(ps, "/")
		gormdb.DB.Where("path = ? and method = ?", api.Path, api.Method).First(api)
		list = append(list, api)
	}
	return gormdb.DB.Save(&list).Error
}
