package gorm

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/search/gorms"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// Search is the GORM-backed list action.
type Search struct {
	opts *Options
}

// String returns the action name.
func (*Search) String() string {
	return "search"
}

// NewSearch creates a search action.
func NewSearch(opts ...Option) *Search {
	o := &Options{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return &Search{opts: o}
}

// Handler returns the action middleware chain.
func (e *Search) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		if e.opts.Model == nil || e.opts.Search == nil {
			response.Make(c).Err(http.StatusNotImplemented, "search model is not configured")
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
	req, ok := pkg.DeepCopy(e.opts.Search).(response.Searcher)
	if !ok || req == nil {
		response.Make(c).Err(http.StatusInternalServerError, "invalid search configuration")
		return
	}
	api := response.Make(c).Bind(req)
	if api.Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	m := pkg.TablerDeepCopy(e.opts.Model)
	if m == nil {
		api.AddError(errors.New("invalid search model"))
		api.Err(http.StatusInternalServerError)
		return
	}
	if gormdb.DB == nil {
		api.AddError(errors.New("GORM database is not initialized"))
		api.Err(http.StatusServiceUnavailable)
		return
	}

	db := gormdb.DB.WithContext(context.WithValue(c, "gorm:cache:tag", m.TableName()))
	if e.opts.BeforeSearch != nil {
		if err := e.opts.BeforeSearch(c, db, m); err != nil {
			api.AddError(err).Log.Error("BeforeSearch error")
			api.Err(http.StatusInternalServerError)
			return
		}
	}

	filterScopes := []func(*gorm.DB) *gorm.DB{gorms.MakeCondition(req)}
	if e.opts.Scope != nil {
		if scope := e.opts.Scope(c, m); scope != nil {
			filterScopes = append(filterScopes, scope)
		}
	}

	baseQuery := func() *gorm.DB {
		return db.
			Clauses(dbresolver.Use(m.TableName())).
			Model(m).
			Scopes(filterScopes...)
	}
	query := baseQuery().Scopes(gorms.Paginate(int(req.GetPageSize()), int(req.GetPage())))

	if e.opts.TreeField != "" && e.opts.Depth > 0 {
		treeFields := make([]string, e.opts.Depth)
		for i := range treeFields {
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
	items := make([]any, 0, req.GetPageSize())
	for rows.Next() {
		item := pkg.TablerDeepCopy(e.opts.Model)
		if item == nil {
			_ = rows.Close()
			api.AddError(errors.New("invalid search model copy"))
			api.Err(http.StatusInternalServerError)
			return
		}
		if err = query.ScanRows(rows, item); err != nil {
			_ = rows.Close()
			api.AddError(err).Log.ErrorContext(c, "search scan error", "error", err)
			api.Err(http.StatusInternalServerError)
			return
		}
		items = append(items, item)
		m = item
	}
	rowsErr := rows.Err()
	closeErr := rows.Close()
	if err = errors.Join(rowsErr, closeErr); err != nil {
		api.AddError(err).Log.ErrorContext(c, "search rows error", "error", err)
		api.Err(http.StatusInternalServerError)
		return
	}

	var count int64
	if err = baseQuery().Count(&count).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		api.AddError(err).Log.ErrorContext(c, "search count error", "error", err)
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
