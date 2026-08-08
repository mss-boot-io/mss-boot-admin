package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"
)

const (
	optionCacheKeyPrefix      = "options"
	optionCacheTTL            = 5 * time.Minute
	optionCacheOperationLimit = 100 * time.Millisecond
)

type Option struct{}

var errOptionVersionChanged = errors.New("option version changed concurrently")

func (e *Option) GetOption(ctx context.Context, category, name string) (*models.Option, error) {
	cacheKey := fmt.Sprintf("%s:%s:%s", optionCacheKeyPrefix, category, name)

	if center.GetCache() != nil {
		cacheCtx, cancel := optionCacheContext(ctx)
		cachedData, err := center.GetCache().Get(cacheCtx, cacheKey).Result()
		cancel()
		if err == nil && cachedData != "" {
			var option models.Option
			if err := json.Unmarshal([]byte(cachedData), &option); err == nil {
				return &option, nil
			}
			slog.Error("unmarshal cached option error", "key", cacheKey, "err", err)
		}
	}

	option, err := e.getOptionFromDB(ctx, category, name)
	if err != nil {
		return nil, err
	}

	if center.GetCache() != nil && option != nil {
		data, err := json.Marshal(option)
		if err != nil {
			slog.Error("marshal option error", "err", err)
		} else {
			cacheCtx, cancel := optionCacheContext(ctx)
			err = center.GetCache().Set(cacheCtx, cacheKey, string(data), optionCacheTTL).Err()
			cancel()
			if err != nil {
				slog.Error("set option cache error", "key", cacheKey, "err", err)
			}
		}
	}

	return option, nil
}

func (e *Option) getOptionFromDB(ctx context.Context, category, name string) (*models.Option, error) {
	ginCtx, ok := ctx.(*gin.Context)
	if !ok {
		ginCtx = &gin.Context{}
	}

	var option models.Option
	// A cache miss must not republish a lagging replica value after a committed
	// update invalidates Redis. Pin the authoritative refill to the writer.
	err := center.GetDB(ginCtx, &models.Option{}).
		Clauses(dbresolver.Write).
		Where("category = ? AND name = ? AND status = ?", category, name, enum.Enabled).
		First(&option).Error
	if err != nil {
		return nil, err
	}

	return &option, nil
}

func (e *Option) GetOptions(ctx context.Context, queries []struct{ Category, Name string }) ([]*models.Option, error) {
	results := make([]*models.Option, 0, len(queries))
	for _, query := range queries {
		option, err := e.GetOption(ctx, query.Category, query.Name)
		if err != nil {
			slog.Error("get option error", "category", query.Category, "name", query.Name, "err", err)
			continue
		}
		if option != nil {
			results = append(results, option)
		}
	}
	return results, nil
}

func (e *Option) InvalidateCache(ctx context.Context, category, name string) error {
	if center.GetCache() == nil {
		return nil
	}
	cacheKey := fmt.Sprintf("%s:%s:%s", optionCacheKeyPrefix, category, name)
	cacheCtx, cancel := optionCacheContext(ctx)
	defer cancel()
	return center.GetCache().Del(cacheCtx, cacheKey).Err()
}

func (e *Option) UpdateOption(ctx context.Context, id string, items *models.OptionItems, changedBy, changeNote string) error {
	ginCtx, ok := ctx.(*gin.Context)
	if !ok {
		ginCtx = &gin.Context{}
	}

	var option models.Option
	db := center.GetDB(ginCtx, &models.Option{}).Clauses(dbresolver.Write)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).
			First(&option).Error; err != nil {
			return err
		}

		versionSnapshot := &models.OptionVersion{
			OptionID:   option.ID,
			Version:    option.Version,
			Items:      option.Items,
			ChangedBy:  changedBy,
			ChangeNote: changeNote,
			Status:     enum.Enabled,
		}
		if err := tx.Create(versionSnapshot).Error; err != nil {
			return err
		}

		nextVersion := option.Version + 1
		result := tx.Model(&models.Option{}).
			Where("id = ? AND version = ?", option.ID, option.Version).
			Updates(map[string]any{"items": items, "version": nextVersion})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errOptionVersionChanged
		}
		option.Items = items
		option.Version = nextVersion
		return nil
	})
	if err != nil {
		return err
	}

	// The database commit is authoritative. Redis invalidation is derived
	// maintenance and must not turn a successful option update into an error
	// that invites the client to repeat the mutation and advance the version
	// again.
	if err := e.InvalidateCache(ctx, option.Category, option.Name); err != nil {
		slog.Warn("invalidate option cache after commit", "err", err)
	}
	return nil
}

func NewOption() *Option {
	return &Option{}
}

func optionCacheContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, optionCacheOperationLimit)
}
