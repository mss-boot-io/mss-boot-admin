package gorm

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/3/4 01:30:34
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/3/4 01:30:34
 */

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// Control action
type Control struct {
	opts *Options
}

// String action name
func (*Control) String() string {
	return "control"
}

// NewControl new control action
func NewControl(opts ...Option) *Control {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return &Control{
		opts: o,
	}
}

func (e *Control) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		if e.opts.Model == nil {
			response.Make(c).Err(http.StatusNotImplemented, "not implemented")
			return
		}
		switch c.Request.Method {
		case http.MethodPost:
			e.create(c)
		case http.MethodPut:
			e.update(c)
		default:
			response.Make(c).Err(http.StatusNotImplemented, "not implemented")
		}
	}
	chain := gin.HandlersChain{h}
	if e.opts.controlHandlers != nil {
		chain = append(e.opts.controlHandlers, chain...)
	}
	if e.opts.handlers != nil {
		chain = append(e.opts.handlers, chain...)
	}
	return chain
}

func (e *Control) create(c *gin.Context) {
	m := pkg.TablerDeepCopy(e.opts.Model)
	api := response.Make(c).Bind(m)
	if api.Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if e.opts.BeforeCreate != nil {
		err := e.opts.BeforeCreate(c, gormdb.DB, m)
		if err != nil {
			api.AddError(err).Log.Error("BeforeCreate error", "error", err)
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	query := gormdb.DB.WithContext(c)
	if e.opts.Scope != nil {
		query = query.Clauses(dbresolver.Use(m.TableName())).Scopes(e.opts.Scope(c, e.opts.Model))
	}
	err := query.Transaction(func(tx *gorm.DB) error {
		err := tx.Create(m).Error
		if err != nil {
			api.AddError(err).Log.ErrorContext(c, "Create error", "error", err)
			api.Err(http.StatusInternalServerError)
			return err
		}
		if pkg.SupportCreator(m) {
			verify := response.VerifyHandler(c)
			if verify == nil {
				api.Err(http.StatusUnauthorized)
				return errors.New("verify handler is nil")
			}
			err = tx.Model(m).Update(pkg.GetCreatorField(), verify.GetUserID()).Error
			if err != nil {
				api.AddError(err).Log.ErrorContext(c, "Create error", "error", err)
				api.Err(http.StatusInternalServerError)
				return err
			}
		}
		if e.opts.AfterCreate != nil {
			err = e.opts.AfterCreate(c, tx, m)
			if err != nil {
				api.AddError(err).Log.Error("AfterCreate error", "error", err)
				api.Err(http.StatusInternalServerError)
				return err
			}
		}
		return nil
	})

	if err != nil {
		api.AddError(err).Log.ErrorContext(c, "Create error", "error", err)
		return
	}

	if CleanCacheFromTag != nil {
		_ = CleanCacheFromTag(c, m.TableName())
	}
	api.OK(m)
}

func (e *Control) update(c *gin.Context) {
	m := pkg.TablerDeepCopy(e.opts.Model)
	id := c.Param(e.opts.Key)
	api := response.Make(c)
	if id == "" {
		api.AddError(errors.New("id is empty"))
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	query := gormdb.DB.WithContext(context.WithValue(c, "gorm:cache:tag", m.TableName())).Where(e.opts.Key, id)
	if e.opts.Scope != nil {
		query = query.Clauses(dbresolver.Use(m.TableName())).Scopes(e.opts.Scope(c, m))
	}
	// find object
	err := query.First(m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.AddError(fmt.Errorf("%s(%s) record not found", e.opts.Key, id))
			api.Err(http.StatusNotFound)
			return
		}
		api.AddError(err).Log.ErrorContext(c, "Update error", "error", err.Error())
		api.Err(http.StatusInternalServerError)
		return
	}

	api = api.Bind(m)
	if api.Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if e.opts.BeforeUpdate != nil {
		err = e.opts.BeforeUpdate(c, gormdb.DB, m)
		if err != nil {
			api.AddError(err).Err(http.StatusInternalServerError)
			return
		}
	}
	query = gormdb.DB.WithContext(c)
	if e.opts.Scope != nil {
		query = query.Scopes(e.opts.Scope(c, m))
	}
	err = query.Save(m).Error
	if err != nil {
		api.AddError(err).Log.ErrorContext(c, "Update error", "error", err.Error())
		api.Err(http.StatusInternalServerError)
		return
	}
	if CleanCacheFromTag != nil {
		_ = CleanCacheFromTag(c, m.TableName())
	}
	if e.opts.AfterUpdate != nil {
		err = e.opts.AfterUpdate(c, query, m)
		if err != nil {
			api.AddError(err).Err(http.StatusInternalServerError)
			return
		}
	}
	api.OK(m)
}
