package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
)

const (
	optionCacheKeyPrefix = "options"
	optionCacheTTL       = 5 * time.Minute
)

type Option struct{}

func (e *Option) GetOption(ctx context.Context, category, name string) (*models.Option, error) {
	cacheKey := fmt.Sprintf("%s:%s:%s", optionCacheKeyPrefix, category, name)

	if center.GetCache() != nil {
	 cachedData, err := center.GetCache().Get(ctx, cacheKey).Result()
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
		 err = center.GetCache().Set(ctx, cacheKey, string(data), optionCacheTTL).Err()
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
	err := center.GetDB(ginCtx, &models.Option{}).
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
	return center.GetCache().Del(ctx, cacheKey).Err()
}

func (e *Option) UpdateOption(ctx context.Context, id string, items *models.OptionItems, changedBy, changeNote string) error {
	ginCtx, ok := ctx.(*gin.Context)
	if !ok {
	 ginCtx = &gin.Context{}
	}

	var option models.Option
	err := center.GetDB(ginCtx, &models.Option{}).Where("id = ?", id).First(&option).Error
	if err != nil {
	 return err
	}

	versionSnapshot := &models.OptionVersion{
	 OptionID:   id,
	 Version:    option.Version,
	 Items:      option.Items,
	 ChangedBy:  changedBy,
	 ChangeNote: changeNote,
	 Status:     enum.Enabled,
	}

	err = center.GetDB(ginCtx, &models.OptionVersion{}).Create(versionSnapshot).Error
	if err != nil {
	 slog.Error("create option version error", "err", err)
	}

	option.Items = items
	option.Version = option.Version + 1

	err = center.GetDB(ginCtx, &models.Option{}).Save(&option).Error
	if err != nil {
	 return err
	}

	return e.InvalidateCache(ctx, option.Category, option.Name)
}

func NewOption() *Option {
 return &Option{}
}