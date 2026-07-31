package gorm

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/3/4 01:38:45
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/3/4 01:38:45
 */

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// Get action
type Get struct {
	opts *Options
}

// String action name
func (*Get) String() string {
	return "get"
}

// NewGet new get action
func NewGet(opts ...Option) *Get {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return &Get{
		opts: o,
	}
}

func (e *Get) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		if e.opts.Model == nil {
			response.Make(c).Err(http.StatusNotImplemented, "not implemented")
			return
		}
		e.get(c, e.opts.Key)
	}
	chain := gin.HandlersChain{h}
	if e.opts.getHandlers != nil {
		chain = append(e.opts.getHandlers, chain...)
	}
	if e.opts.handlers != nil {
		chain = append(e.opts.handlers, chain...)
	}
	return chain
}

// get one record by id
func (e *Get) get(c *gin.Context, key string) {
	api := response.Make(c)
	m := pkg.TablerDeepCopy(e.opts.Model)
	preloads := c.QueryArray("preloads[]")
	query := gormdb.DB.WithContext(context.WithValue(c, "gorm:cache:tag", m.TableName())).Model(m).Where("id = ?", c.Param(key))

	if e.opts.BeforeGet != nil {
		if err := e.opts.BeforeGet(c, query, m); err != nil {
			api.AddError(err).Log.Error("BeforeGet error")
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	for _, preload := range preloads {
		query = query.Preload(preload)
	}
	if e.opts.Scope != nil {
		query = query.Clauses(dbresolver.Use(m.TableName())).Scopes(e.opts.Scope(c, m))
	}
	err := query.First(m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.Err(http.StatusNotFound)
			return
		}
		api.AddError(err).Log.ErrorContext(c, "Get error", "error", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	if e.opts.AfterGet != nil {
		if err = e.opts.AfterGet(c, query, m); err != nil {
			api.AddError(err).Log.Error("AfterGet error", "error", err)
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	api.OK(m)
}
