package gorm

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/3/9 00:57:48
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/3/9 00:57:48
 */

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/search/gorms"
)

// Search action
type Search struct {
	opts *Options
}

// String action name
func (*Search) String() string {
	return "search"
}

// NewSearch new search action
func NewSearch(opts ...Option) *Search {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return &Search{
		opts: o,
	}
}

// Handler action handler
func (e *Search) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		if e.opts.Model == nil {
			response.Make(c).Err(http.StatusNotImplemented, "not implemented")
			return
		}
		e.search(c)
	}
	chain := gin.HandlersChain{h}
	if e.opts.searchHandlers != nil {
		chain = append(e.opts.searchHandlers, chain...)
	}
	if e.opts.handlers != nil {
		chain = append(e.opts.handlers, chain...)
	}
	return chain
}

func (e *Search) search(c *gin.Context) {
	req := pkg.DeepCopy(e.opts.Search).(response.Searcher)
	api := response.Make(c).Bind(req)
	if api.Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	m := pkg.TablerDeepCopy(e.opts.Model)

	db := gormdb.DB.WithContext(context.WithValue(c, "gorm:cache:tag", m.TableName()))

	if e.opts.BeforeSearch != nil {
		if err := e.opts.BeforeSearch(c, db, m); err != nil {
			api.AddError(err).Log.Error("BeforeSearch error")
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	var count int64

	scopes := []func(db *gorm.DB) *gorm.DB{
		gorms.MakeCondition(req),
		gorms.Paginate(int(req.GetPageSize()), int(req.GetPage())),
	}

	query := db.Model(m)

	if e.opts.Scope != nil {
		scopes = append(scopes, e.opts.Scope(c, m))
		query = query.Clauses(dbresolver.Use(m.TableName())).Scopes(scopes...)
	}

	if e.opts.TreeField != "" && e.opts.Depth > 0 {
		treeFields := make([]string, 0, e.opts.Depth)
		for i := range e.opts.Depth {
			treeFields[i] = e.opts.TreeField
		}
		query = query.Preload(strings.Join(treeFields, "."))
	}

	rows, err := query.Rows()
	if err != nil {
		api.AddError(err).Log.ErrorContext(c, "Search error", "error", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	items := make([]any, 0, req.GetPageSize())
	for rows.Next() {
		m = pkg.TablerDeepCopy(e.opts.Model)
		err = db.ScanRows(rows, m)
		if err != nil {
			api.AddError(err).Log.ErrorContext(c, "search error", "error", err)
			api.Err(http.StatusInternalServerError)
			return
		}
		items = append(items, m)
	}
	err = query.Scopes(func(db *gorm.DB) *gorm.DB {
		return db.Limit(-1).Offset(-1)
	}).Count(&count).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		api.AddError(err).Log.ErrorContext(c, "search error", "error", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	if e.opts.AfterSearch != nil {
		if err = e.opts.AfterSearch(c, db, m); err != nil {
			api.AddError(err).Log.ErrorContext(c, "AfterSearch error", "error", err)
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	api.PageOK(items, count, req.GetPage(), req.GetPageSize(), "search success")
	c.Next()
}
