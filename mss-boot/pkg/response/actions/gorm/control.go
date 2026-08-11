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
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// Control action
type Control struct {
	opts *Options
}

type controlOperationError struct {
	operation actions.WriteOperation
	err       error
}

func (e *controlOperationError) Error() string { return e.err.Error() }
func (e *controlOperationError) Unwrap() error { return e.err }

var errVerifyHandlerMissing = errors.New("verify handler is nil")

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

func (e *Control) writeError(c *gin.Context, api *response.API, operation actions.WriteOperation, err error) {
	if e.opts.WriteErrorMapper == nil {
		if operation == actions.WriteOperationCreator && errors.Is(err, errVerifyHandlerMissing) {
			api.Log.ErrorContext(c, "Control creator identity is unavailable")
			api.Err(http.StatusUnauthorized)
			return
		}
		api.AddError(err).Log.ErrorContext(c, "Control write error", "operation", operation, "error", err)
		api.Err(http.StatusInternalServerError)
		return
	}

	publicError, matched := e.opts.WriteErrorMapper(c, operation, err)
	if !matched || publicError.Status < http.StatusBadRequest || publicError.Status > 599 || publicError.Error == nil {
		publicError = actions.PublicWriteError{
			Status: http.StatusInternalServerError,
			Error:  response.NewError("WRITE_FAILED", "request could not be completed"),
		}
	}
	api.Error = publicError.Error
	api.Log.ErrorContext(c, "Control write failed", "operation", operation, "error_code", publicError.Error.ErrorCode())
	api.Err(publicError.Status)
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
			e.writeError(c, api, actions.WriteOperationBeforeCreate, err)
			return
		}
	}
	requestContext := c.Request.Context()
	// Keep a stable Gin context on the GORM statement. Existing model hooks use
	// its request-scoped identity and statistics data, while the copied context
	// is detached from Gin's pool and cannot be reset by a later request while
	// database/sql is still observing it.
	gormContext := c.Copy()
	query := gormdb.DB.WithContext(gormContext)
	if e.opts.Scope != nil {
		query = query.Clauses(dbresolver.Use(m.TableName())).Scopes(e.opts.Scope(c, e.opts.Model))
	}
	err := query.Transaction(func(tx *gorm.DB) error {
		err := tx.Create(m).Error
		if err != nil {
			return &controlOperationError{operation: actions.WriteOperationCreate, err: err}
		}
		if pkg.SupportCreator(m) {
			verify := response.VerifyHandler(c)
			if verify == nil {
				return &controlOperationError{operation: actions.WriteOperationCreator, err: errVerifyHandlerMissing}
			}
			err = tx.Model(m).Update(pkg.GetCreatorField(), verify.GetUserID()).Error
			if err != nil {
				return &controlOperationError{operation: actions.WriteOperationCreator, err: err}
			}
		}
		if e.opts.AfterCreate != nil {
			err = e.opts.AfterCreate(c, tx, m)
			if err != nil {
				return &controlOperationError{operation: actions.WriteOperationAfterCreate, err: err}
			}
		}
		return nil
	})

	if err != nil {
		operation := actions.WriteOperationCreate
		cause := err
		var operationError *controlOperationError
		if errors.As(err, &operationError) {
			operation = operationError.operation
			cause = operationError.err
		}
		e.writeError(c, api, operation, cause)
		return
	}
	if CleanCacheFromTag != nil {
		_ = CleanCacheFromTag(requestContext, m.TableName())
	}
	if e.opts.AfterCommitCreate != nil {
		if err := e.opts.AfterCommitCreate(c, query, m); err != nil {
			e.writeError(c, api, actions.WriteOperationAfterCommit, err)
			return
		}
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
	requestContext := c.Request.Context()
	gormContext := c.Copy()
	query := gormdb.DB.WithContext(context.WithValue(gormContext, "gorm:cache:tag", m.TableName())).Where(e.opts.Key, id)
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
		e.writeError(c, api, actions.WriteOperationLoad, err)
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
			e.writeError(c, api, actions.WriteOperationBeforeUpdate, err)
			return
		}
	}
	query = gormdb.DB.WithContext(gormContext)
	if e.opts.Scope != nil {
		query = query.Scopes(e.opts.Scope(c, m))
	}
	err = query.Save(m).Error
	if err != nil {
		e.writeError(c, api, actions.WriteOperationUpdate, err)
		return
	}
	if CleanCacheFromTag != nil {
		_ = CleanCacheFromTag(requestContext, m.TableName())
	}
	if e.opts.AfterUpdate != nil {
		err = e.opts.AfterUpdate(c, query, m)
		if err != nil {
			e.writeError(c, api, actions.WriteOperationAfterUpdate, err)
			return
		}
	}
	api.OK(m)
}
