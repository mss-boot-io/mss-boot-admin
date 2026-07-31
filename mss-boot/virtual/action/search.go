package action

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/17 09:07:45
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/17 09:07:45
 */

// Search action
type Search struct {
	*Base
}

// NewSearch new search action
func NewSearch(b *Base) *Search {
	return &Search{
		Base: b,
	}
}

// String print action name
func (*Search) String() string {
	return "search"
}

// Handler search action handler
func (e *Search) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet:
			api := response.Make(c)
			// search
			m := e.GetModel(c)
			if m == nil {
				// no set model
				api.Err(http.StatusNotFound)
				return
			}
			if m.Auth {
				response.AuthHandler(c)
			}
			items := m.MakeList()
			page := &Pagination{}
			var count int64
			query := gormdb.DB.Scopes(m.TableScope, m.Search(c), m.Pagination(c, page))
			if m.MultiTenant && e.TenantIDFunc != nil {
				query = query.Scopes(m.TenantScope(c, e.TenantIDFunc))
			}
			if err := query.Find(items).Offset(-1).Limit(-1).Count(&count).Error; err != nil {
				api.AddError(err).Log.Error("search error", PathKey, c.Param(PathKey))
				api.Err(http.StatusInternalServerError)
				return
			}
			api.PageOK(items, count, page.GetPage(), page.GetPageSize())
		default:
			c.AbortWithStatus(http.StatusMethodNotAllowed)
		}
	}
	return gin.HandlersChain{h}
}
