package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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
	optionCacheKeyPrefix      = "options:v2"
	optionCacheTTL            = 5 * time.Minute
	optionCacheOperationLimit = 100 * time.Millisecond
	MaxOptionRecords          = 256
	maxOptionChangeNoteLength = 1024
)

type Option struct{}

var (
	ErrOptionVersionChanged  = errors.New("option version changed concurrently")
	ErrOptionNameUnavailable = errors.New("option name unavailable")
	ErrOptionCapacity        = errors.New("option capacity reached")
	ErrOptionBuiltIn         = errors.New("built-in option is protected")
	ErrOptionInUse           = errors.New("option is in use")
	ErrOptionDataInvalid     = errors.New("stored option data is invalid")
)

type OptionUpdateInput struct {
	Category        *string
	DisplayName     *string
	Description     *string
	Name            *string
	Remark          *string
	Items           *models.OptionItems
	Status          *enum.Status
	ExpectedVersion int
}

type OptionRevisionConflictError struct {
	Current *models.Option
}

func (e *OptionRevisionConflictError) Error() string {
	return ErrOptionVersionChanged.Error()
}

func (e *OptionRevisionConflictError) Unwrap() error {
	return ErrOptionVersionChanged
}

func (e *Option) GetOption(ctx context.Context, category, name string) (*models.Option, error) {
	category = strings.TrimSpace(category)
	name = strings.TrimSpace(name)
	cacheKey, cacheReady := optionCacheKey(ctx, category, name)

	if center.GetCache() != nil && cacheReady {
		cacheCtx, cancel := optionCacheContext(ctx)
		cachedData, err := center.GetCache().Get(cacheCtx, cacheKey).Result()
		cancel()
		if err == nil && cachedData != "" {
			var option models.Option
			if err := json.Unmarshal([]byte(cachedData), &option); err == nil &&
				option.Category == category && option.Name == name && option.Status == enum.Enabled &&
				option.ValidateStored() == nil {
				return &option, nil
			}
			slog.Warn("discard invalid cached option", "key", cacheKey)
			cacheCtx, cancel = optionCacheContext(ctx)
			_ = center.GetCache().Del(cacheCtx, cacheKey).Err()
			cancel()
		}
	}

	option, err := e.getOptionFromDB(ctx, category, name)
	if err != nil {
		return nil, err
	}

	if center.GetCache() != nil && option != nil && cacheReady {
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
	if err := option.ValidateStored(); err != nil {
		return nil, errors.Join(ErrOptionDataInvalid, err)
	}

	return &option, nil
}

func (e *Option) GetOptions(ctx context.Context, queries []struct{ Category, Name string }) ([]*models.Option, error) {
	results := make([]*models.Option, 0, len(queries))
	for _, query := range queries {
		option, err := e.GetOption(ctx, query.Category, query.Name)
		if err != nil {
			return nil, fmt.Errorf("get option %q/%q: %w", query.Category, query.Name, err)
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
	keys := []string{fmt.Sprintf("options:%s:%s", category, name)}
	if cacheKey, ok := optionCacheKey(ctx, category, name); ok {
		keys = append(keys, cacheKey)
	}
	cacheCtx, cancel := optionCacheContext(ctx)
	defer cancel()
	return center.GetCache().Del(cacheCtx, keys...).Err()
}

func (e *Option) CreateOption(
	ctx context.Context,
	option *models.Option,
	changedBy string,
	changeNote string,
) (*models.Option, error) {
	if option == nil {
		return nil, fmt.Errorf("%w: option is nil", models.ErrOptionInvalid)
	}
	next := *option
	next.ID = ""
	next.CreatedAt = time.Time{}
	next.UpdatedAt = time.Time{}
	next.DeletedAt = gorm.DeletedAt{}
	next.Version = 1
	next.BuiltIn = false
	next.Items = cloneOptionItems(nil, option.Items, true)
	if err := next.NormalizeAndValidate(); err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(changeNote)) > maxOptionChangeNoteLength {
		return nil, fmt.Errorf("%w: change note is too long", models.ErrOptionInvalid)
	}

	ginCtx := optionGinContext(ctx)
	db := center.GetDB(ginCtx, &models.Option{}).Clauses(dbresolver.Write)
	err := db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.Option{}).Count(&count).Error; err != nil {
			return err
		}
		if count >= MaxOptionRecords {
			return ErrOptionCapacity
		}
		var duplicate int64
		if err := tx.Model(&models.Option{}).Where("name = ?", next.Name).Count(&duplicate).Error; err != nil {
			return err
		}
		if duplicate > 0 {
			return ErrOptionNameUnavailable
		}
		return tx.Create(&next).Error
	})
	if err != nil {
		return nil, err
	}
	e.invalidateAfterCommit(ctx, next.Category, next.Name)
	return &next, nil
}

func (e *Option) UpdateOptionResource(
	ctx context.Context,
	id string,
	input OptionUpdateInput,
	changedBy string,
	changeNote string,
) (*models.Option, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 64 {
		return nil, fmt.Errorf("%w: option identifier is invalid", models.ErrOptionInvalid)
	}
	changeNote = strings.TrimSpace(changeNote)
	if len(changeNote) > maxOptionChangeNoteLength {
		return nil, fmt.Errorf("%w: change note is too long", models.ErrOptionInvalid)
	}
	ginCtx, ok := ctx.(*gin.Context)
	if !ok {
		ginCtx = &gin.Context{}
	}

	var current models.Option
	var updated models.Option
	db := center.GetDB(ginCtx, &models.Option{}).Clauses(dbresolver.Write)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).
			First(&current).Error; err != nil {
			return err
		}
		if err := current.ValidateStored(); err != nil {
			return errors.Join(ErrOptionDataInvalid, err)
		}
		if current.Version != input.ExpectedVersion {
			return &OptionRevisionConflictError{Current: cloneOption(&current)}
		}

		next := *cloneOption(&current)
		if input.Category != nil {
			next.Category = *input.Category
		}
		if input.DisplayName != nil {
			next.DisplayName = *input.DisplayName
		}
		if input.Description != nil {
			next.Description = *input.Description
		}
		if input.Name != nil {
			next.Name = *input.Name
		}
		if input.Remark != nil {
			next.Remark = *input.Remark
		}
		if input.Status != nil {
			next.Status = *input.Status
		}
		if input.Items != nil {
			next.Items = cloneOptionItems(current.Items, input.Items, false)
		}
		next.Version = current.Version + 1
		if current.BuiltIn &&
			(next.Category != current.Category || next.Name != current.Name || next.Status != current.Status) {
			return ErrOptionBuiltIn
		}
		if err := next.NormalizeAndValidate(); err != nil {
			return err
		}
		var duplicate int64
		if err := tx.Model(&models.Option{}).
			Where("name = ? AND id <> ?", next.Name, current.ID).
			Count(&duplicate).Error; err != nil {
			return err
		}
		if duplicate > 0 {
			return ErrOptionNameUnavailable
		}

		versionSnapshot := optionVersionSnapshot(&current, changedBy, changeNote)
		if err := tx.Create(versionSnapshot).Error; err != nil {
			return err
		}

		result := tx.Model(&models.Option{}).
			Where("id = ? AND version = ?", current.ID, current.Version).
			Updates(map[string]any{
				"category":     next.Category,
				"display_name": next.DisplayName,
				"description":  next.Description,
				"name":         next.Name,
				"remark":       next.Remark,
				"items":        next.Items,
				"status":       next.Status,
				"version":      next.Version,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			var latest models.Option
			if err := tx.Clauses(dbresolver.Write).First(&latest, "id = ?", current.ID).Error; err != nil {
				return err
			}
			return &OptionRevisionConflictError{Current: cloneOption(&latest)}
		}
		return tx.Clauses(dbresolver.Write).First(&updated, "id = ?", current.ID).Error
	})
	if err != nil {
		return nil, err
	}
	e.invalidateAfterCommit(ctx, current.Category, current.Name)
	if current.Category != updated.Category || current.Name != updated.Name {
		e.invalidateAfterCommit(ctx, updated.Category, updated.Name)
	}
	return &updated, nil
}

func (e *Option) DeleteOption(
	ctx context.Context,
	id string,
	expectedVersion int,
	changedBy string,
	changeNote string,
) (*models.Option, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 64 {
		return nil, fmt.Errorf("%w: option identifier is invalid", models.ErrOptionInvalid)
	}
	changeNote = strings.TrimSpace(changeNote)
	if len(changeNote) > maxOptionChangeNoteLength {
		return nil, fmt.Errorf("%w: change note is too long", models.ErrOptionInvalid)
	}
	ginCtx := optionGinContext(ctx)
	db := center.GetDB(ginCtx, &models.Option{}).Clauses(dbresolver.Write)
	var current models.Option
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", id).Error; err != nil {
			return err
		}
		if err := current.ValidateStored(); err != nil {
			return errors.Join(ErrOptionDataInvalid, err)
		}
		if current.Version != expectedVersion {
			return &OptionRevisionConflictError{Current: cloneOption(&current)}
		}
		if current.BuiltIn {
			return ErrOptionBuiltIn
		}
		var usageCount int64
		if err := tx.Model(&models.OptionUsage{}).
			Where("option_id = ? AND status = ?", current.ID, enum.Enabled).
			Count(&usageCount).Error; err != nil {
			return err
		}
		if usageCount > 0 {
			return ErrOptionInUse
		}
		if err := tx.Create(optionVersionSnapshot(&current, changedBy, changeNote)).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND version = ?", current.ID, current.Version).Delete(&models.Option{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			var latest models.Option
			if err := tx.Clauses(dbresolver.Write).First(&latest, "id = ?", current.ID).Error; err != nil {
				return err
			}
			return &OptionRevisionConflictError{Current: cloneOption(&latest)}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	e.invalidateAfterCommit(ctx, current.Category, current.Name)
	return &current, nil
}

func NewOption() *Option {
	return &Option{}
}

func (e *Option) invalidateAfterCommit(ctx context.Context, category, name string) {
	// The database commit is authoritative. Redis invalidation is derived
	// maintenance and must not turn a successful mutation into an error that
	// invites a client retry and an extra version transition.
	if err := e.InvalidateCache(ctx, category, name); err != nil {
		slog.Warn("invalidate option cache after commit", "category", category, "name", name, "err", err)
	}
}

func optionVersionSnapshot(option *models.Option, changedBy, changeNote string) *models.OptionVersion {
	return &models.OptionVersion{
		OptionID:     option.ID,
		Version:      option.Version,
		Category:     option.Category,
		Name:         option.Name,
		DisplayName:  option.DisplayName,
		Description:  option.Description,
		Remark:       option.Remark,
		Items:        cloneOptionItems(nil, option.Items, false),
		OptionStatus: option.Status,
		BuiltIn:      option.BuiltIn,
		ChangedBy:    strings.TrimSpace(changedBy),
		ChangeNote:   strings.TrimSpace(changeNote),
		Status:       enum.Enabled,
	}
}

func cloneOption(option *models.Option) *models.Option {
	if option == nil {
		return nil
	}
	clone := *option
	clone.Items = cloneOptionItems(nil, option.Items, false)
	return &clone
}

func cloneOptionItems(
	current *models.OptionItems,
	requested *models.OptionItems,
	resetAllIDs bool,
) *models.OptionItems {
	if requested == nil {
		return nil
	}
	currentByID := make(map[string]*models.OptionItem)
	if current != nil {
		for _, item := range *current {
			if item != nil && strings.TrimSpace(item.ID) != "" {
				currentByID[strings.TrimSpace(item.ID)] = item
			}
		}
	}
	clone := make(models.OptionItems, 0, len(*requested))
	for _, item := range *requested {
		if item == nil {
			clone = append(clone, nil)
			continue
		}
		copyItem := *item
		copyItem.Extra = cloneOptionExtra(item.Extra)
		owned := currentByID[strings.TrimSpace(copyItem.ID)]
		if resetAllIDs || owned == nil {
			copyItem.ID = ""
		} else if copyItem.Extra == nil {
			copyItem.Extra = cloneOptionExtra(owned.Extra)
		}
		clone = append(clone, &copyItem)
	}
	return &clone
}

func cloneOptionExtra(extra map[string]any) map[string]any {
	if extra == nil {
		return nil
	}
	payload, err := json.Marshal(extra)
	if err != nil {
		// Keep the original value so the later bounded JSON validation fails
		// closed instead of silently dropping an unsupported client field.
		return extra
	}
	var clone map[string]any
	if err := json.Unmarshal(payload, &clone); err != nil {
		return extra
	}
	return clone
}

func optionGinContext(ctx context.Context) *gin.Context {
	if ginCtx, ok := ctx.(*gin.Context); ok && ginCtx != nil {
		return ginCtx
	}
	return &gin.Context{}
}

func optionCacheKey(ctx context.Context, category, name string) (string, bool) {
	tenantProvider := center.GetTenant()
	ginCtx, isGin := ctx.(*gin.Context)
	if tenantProvider == nil || !isGin || ginCtx == nil {
		return "", false
	}
	tenant, err := tenantProvider.GetTenant(ginCtx)
	if err != nil || tenant == nil {
		return "", false
	}
	tenantID := strings.TrimSpace(fmt.Sprint(tenant.GetID()))
	if tenantID == "" || category == "" || name == "" {
		return "", false
	}
	encode := base64.RawURLEncoding.EncodeToString
	return fmt.Sprintf(
		"%s:%s:%s:%s",
		optionCacheKeyPrefix,
		encode([]byte(tenantID)),
		encode([]byte(category)),
		encode([]byte(name)),
	), true
}

func optionCacheContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, optionCacheOperationLimit)
}
