package gorm

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/3/4 01:36:12
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/3/4 01:36:12
 */

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	gormpkg "gorm.io/gorm"
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
			api := response.Make(c).Bind(&ids, binding.JSON)
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

func (e *Delete) writeError(
	c *gin.Context,
	api *response.API,
	operation actions.WriteOperation,
	err error,
) {
	if e.opts.WriteErrorMapper == nil {
		api.AddError(err).Log.ErrorContext(c, "Delete error", "operation", operation, "error", err)
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
	api.Log.ErrorContext(
		c,
		"Delete failed",
		"operation",
		operation,
		"error_code",
		publicError.Error.ErrorCode(),
	)
	api.Err(publicError.Status)
}

func (e *Delete) delete(c *gin.Context, ids ...string) {
	api := response.Make(c)
	if len(ids) == 0 {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	c.Set("ids", append([]string(nil), ids...))
	query := gormdb.DB.WithContext(c.Copy())
	err := query.Transaction(func(tx *gormpkg.DB) error {
		if e.opts.BeforeDelete != nil {
			if err := e.opts.BeforeDelete(c, tx, e.opts.Model); err != nil {
				return &controlOperationError{
					operation: actions.WriteOperationBeforeDelete,
					err:       err,
				}
			}
		}
		deleteQuery := tx.Where(fmt.Sprintf("%s IN ?", e.opts.Key), ids)
		if e.opts.Scope != nil {
			deleteQuery = deleteQuery.Clauses(dbresolver.Use(e.opts.Model.TableName())).Scopes(e.opts.Scope(c, e.opts.Model))
		}
		if err := deleteQuery.Delete(e.opts.Model).Error; err != nil {
			return &controlOperationError{operation: actions.WriteOperationDelete, err: err}
		}
		return nil
	})
	if err != nil {
		operation := actions.WriteOperationDelete
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
		_ = CleanCacheFromTag(c, e.opts.Model.TableName())
	}
	if e.opts.AfterDelete != nil {
		if err = e.opts.AfterDelete(c, gormdb.DB, e.opts.Model); err != nil {
			e.writeError(c, api, actions.WriteOperationAfterDelete, err)
			return
		}
	}
	api.OK(nil)
}
