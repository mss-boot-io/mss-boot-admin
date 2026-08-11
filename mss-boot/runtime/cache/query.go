package cache

import (
	"context"
	"errors"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/redisresource"
	"gorm.io/gorm"
)

// QueryLoader executes against the exact GORM handle supplied by QueryCache.
// A gorm.ErrRecordNotFound result is normalized into cacheable NotFound
// metadata and reproduced on cache hits.
type QueryLoader func(context.Context, *gorm.DB) (Result, error)

// QueryCache is an explicit opt-in adapter; it does not install global GORM
// callbacks and therefore cannot accidentally cache unspecified queries.
type QueryCache struct {
	derived *Derived
}

func NewQueryCache(scope *redisresource.Scope, policy Policy) (*QueryCache, error) {
	derived, err := NewDerived(scope, policy)
	if err != nil {
		return nil, err
	}
	return &QueryCache{derived: derived}, nil
}

func (q *QueryCache) Load(ctx context.Context, db *gorm.DB, target Target, loader QueryLoader) (Outcome, error) {
	if ctx == nil {
		return Outcome{}, validationError(ErrInvalidTarget, "context", "is required")
	}
	if err := ctx.Err(); err != nil {
		return Outcome{}, err
	}
	if q == nil || q.derived == nil {
		return Outcome{}, ErrClosed
	}
	if db == nil {
		return Outcome{}, validationError(ErrInvalidTarget, "database", "is required")
	}
	if loader == nil {
		return Outcome{}, validationError(ErrInvalidTarget, "loader", "is required")
	}
	if err := target.validate(); err != nil {
		return Outcome{}, err
	}

	load := func(loadContext context.Context) (Result, error) {
		result, err := loader(loadContext, db.WithContext(loadContext))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Result{NotFound: true}, nil
		}
		return result, err
	}

	if activeGORMTransaction(db) {
		outcome, err := q.derived.authorityBypass(ctx, load, StatusTransactionBypass)
		if err != nil {
			return outcome, err
		}
		return restoreNotFound(outcome)
	}

	outcome, err := q.derived.Load(ctx, target, load)
	if err != nil {
		return outcome, err
	}
	return restoreNotFound(outcome)
}

func (q *QueryCache) Invalidate(ctx context.Context, dataset Dataset) (Status, error) {
	if q == nil || q.derived == nil {
		return "", ErrClosed
	}
	return q.derived.Invalidate(ctx, dataset)
}

func (q *QueryCache) Close(ctx context.Context) error {
	if q == nil || q.derived == nil {
		return nil
	}
	return q.derived.Close(ctx)
}

func restoreNotFound(outcome Outcome) (Outcome, error) {
	if outcome.Result.NotFound {
		return outcome, gorm.ErrRecordNotFound
	}
	return outcome, nil
}

func activeGORMTransaction(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	if db.Statement != nil && isTransactionPool(db.Statement.ConnPool) {
		return true
	}
	return isTransactionPool(db.ConnPool)
}

func isTransactionPool(pool gorm.ConnPool) bool {
	if pool == nil {
		return false
	}
	_, ok := pool.(gorm.TxCommitter)
	return ok
}
