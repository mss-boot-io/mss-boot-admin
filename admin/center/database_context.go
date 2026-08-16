package center

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type requestDatabaseContextKey struct{}

// BindRequestDatabase pins a leased database handle to one HTTP request. The
// caller owns the lease and must keep it alive until every downstream handler
// has returned.
func BindRequestDatabase(ctx *gin.Context, db *gorm.DB) error {
	if ctx == nil || ctx.Request == nil {
		return errors.New("request context is required")
	}
	if db == nil {
		return errors.New("request database is required")
	}
	requestContext := context.WithValue(ctx.Request.Context(), requestDatabaseContextKey{}, db)
	ctx.Request = ctx.Request.WithContext(requestContext)
	return nil
}

// RequestDatabase returns the database pinned to the current request without
// exposing the context key to application packages.
func RequestDatabase(ctx context.Context) (*gorm.DB, bool) {
	if ctx == nil {
		return nil, false
	}
	db, ok := ctx.Value(requestDatabaseContextKey{}).(*gorm.DB)
	return db, ok && db != nil
}
