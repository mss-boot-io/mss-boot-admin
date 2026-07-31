package gorm

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/3/4 01:36:12
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/3/4 01:36:12
 */

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"gorm.io/plugin/dbresolver"
)

// Delete action
type Delete struct {
	opts *Options
}

// NewDelete new delete action
func NewDelete(opts ...Option) *Delete {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return &Delete{
		opts: o,
	}
}

func (e *Delete) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		if e.opts.Model == nil {
			response.Make(c).Err(http.StatusNotImplemented, "not implemented")
			return
		}
		ids := make([]string, 0)
		v := c.Param(e.opts.Key)
		if v == "batch" {
			api := response.Make(c).Bind(&ids)
			if api.Error != nil || len(ids) == 0 {
				api.Err(http.StatusUnprocessableEntity)
				return
			}
			e.delete(c, ids...)
			return
		}
		e.delete(c, v)
	}
	chain := gin.HandlersChain{h}
	if e.opts.deleteHandlers != nil {
		chain = append(e.opts.deleteHandlers, chain...)
	}
	if e.opts.handlers != nil {
		chain = append(e.opts.handlers, chain...)
	}
	return chain
}

// String action name
func (*Delete) String() string {
	return "delete"
}

func (e *Delete) delete(c *gin.Context, ids ...string) {
	api := response.Make(c)
	if len(ids) == 0 {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if e.opts.BeforeDelete != nil {
		if err := e.opts.BeforeDelete(c, gormdb.DB, e.opts.Model); err != nil {
			api.AddError(err).Log.ErrorContext(c, "BeforeDelete error", "error", err)
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	c.Set("ids", ids)
	query := gormdb.DB.WithContext(c).
		Where(fmt.Sprintf("%s IN ?", e.opts.Key), ids)
	if e.opts.Scope != nil {
		query = query.Clauses(dbresolver.Use(e.opts.Model.TableName())).Scopes(e.opts.Scope(c, e.opts.Model))
	}
	err := query.Delete(e.opts.Model).Error
	if err != nil {
		api.AddError(err).Log.ErrorContext(c, "Delete error", "error", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	if CleanCacheFromTag != nil {
		_ = CleanCacheFromTag(c, e.opts.Model.TableName())
	}
	if e.opts.AfterDelete != nil {
		if err = e.opts.AfterDelete(c, gormdb.DB, e.opts.Model); err != nil {
			api.AddError(err).Log.ErrorContext(c, "AfterDelete error", "error", err)
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	api.OK(nil)
}
