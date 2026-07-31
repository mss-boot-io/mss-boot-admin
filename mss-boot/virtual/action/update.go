package action

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/17 08:55:14
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/17 08:55:14
 */

type Update struct {
	*Base
}

// NewUpdate new update action
func NewUpdate(b *Base) *Update {
	return &Update{
		Base: b,
	}
}

func (*Update) String() string {
	return "update"
}

// Handler update action handler
func (e *Update) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPut:
			api := response.Make(c)
			// update
			m := e.GetModel(c)
			if m == nil {
				// no set model
				api.Err(http.StatusNotFound)
				return
			}
			if m.Auth {
				response.AuthHandler(c)
			}
			req := m.MakeModel()
			if api.Bind(req).Error != nil {
				api.Err(http.StatusUnprocessableEntity)
				return
			}
			query := gormdb.DB.Scopes(m.TableScope, m.URI(c))
			if m.MultiTenant && e.TenantIDFunc != nil {
				query = query.Scopes(m.TenantScope(c, e.TenantIDFunc))
			}
			if err := query.Updates(req).Error; err != nil {
				api.AddError(err).Log.Error("update error", PathKey, c.Param(PathKey))
				api.Err(http.StatusInternalServerError)
				return
			}
			api.OK(nil)
			return
		default:
			c.AbortWithStatus(http.StatusMethodNotAllowed)
		}
	}
	return gin.HandlersChain{h}
}
