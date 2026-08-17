package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/11 11:58:29
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/11 11:58:29
 */

type AppConfig struct {
	ModelGormTenant
	// Name 名称
	Name string `gorm:"column:name;size:128;index;default:'';not null" json:"name" binding:"required"`
	// Group 分组
	Group string `gorm:"column:group;size:128;index;default:'';not null" json:"group" binding:"required"`
	// Value 值
	Value string `gorm:"column:value;size:255;default:'';not null" json:"value"`
	// Auth 是否需要认证 如果为true，只有登录后才会返回
	Auth bool `gorm:"column:auth;default:false;not null" json:"auth"`
}

const (
	appConfigCacheHash            = "app-configs:{entry}:v1"
	appConfigCacheEnvelopeVersion = 1
	appConfigCacheStateFound      = "found"
	appConfigCacheStateMissing    = "missing"
	appConfigCacheTTL             = 5 * time.Minute
	appConfigNegativeCacheTTL     = 30 * time.Second
	appConfigCacheOperationLimit  = 100 * time.Millisecond
)

type appConfigCacheEnvelope struct {
	Version   int    `json:"v"`
	Group     string `json:"group"`
	Name      string `json:"name"`
	Auth      *bool  `json:"auth"`
	State     string `json:"state"`
	Value     string `json:"value"`
	ExpiresAt int64  `json:"expiresAt"`
}

var (
	ErrAppConfigIdentityMismatch = errors.New("app config key does not exactly match stored database identity")
	ErrAppConfigRevisionChanged  = errors.New("app config revision changed concurrently")
)

func (*AppConfig) TableName() string {
	return "mss_boot_app_configs"
}

func splitAppConfigKey(key string) (string, string) {
	separator := strings.IndexByte(key, ':')
	if separator == -1 {
		return "", key
	}
	return key[:separator], key[separator+1:]
}

func appConfigCacheField(group, name string) string {
	return fmt.Sprintf("%s:%s", group, name)
}

func appConfigCacheContext(ctx *gin.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), appConfigCacheOperationLimit)
	}
	return context.WithTimeout(ctx, appConfigCacheOperationLimit)
}

func deleteAppConfigCacheField(ctx *gin.Context, hash, group, name string) {
	cache := center.GetCache()
	if cache == nil {
		return
	}
	cacheCtx, cancel := appConfigCacheContext(ctx)
	defer cancel()
	_ = cache.HDel(cacheCtx, hash, appConfigCacheField(group, name)).Err()
}

// cachedAppConfig returns a nil config with hit=true for a valid negative cache entry.
func cachedAppConfig(ctx *gin.Context, group, name string, now time.Time) (*AppConfig, bool) {
	cache := center.GetCache()
	if cache == nil {
		return nil, false
	}

	cacheCtx, cancel := appConfigCacheContext(ctx)
	payload, err := cache.HGet(cacheCtx, appConfigCacheHash, appConfigCacheField(group, name)).Bytes()
	cancel()
	if err != nil {
		return nil, false
	}

	var envelope appConfigCacheEnvelope
	if err = json.Unmarshal(payload, &envelope); err != nil ||
		envelope.Version != appConfigCacheEnvelopeVersion ||
		envelope.Group != group ||
		envelope.Name != name ||
		envelope.Auth == nil ||
		*envelope.Auth ||
		envelope.ExpiresAt <= now.UnixMilli() {
		deleteAppConfigCacheField(ctx, appConfigCacheHash, group, name)
		return nil, false
	}

	switch envelope.State {
	case appConfigCacheStateFound:
		return &AppConfig{
			Group: group,
			Name:  name,
			Value: envelope.Value,
			Auth:  false,
		}, true
	case appConfigCacheStateMissing:
		if envelope.Value != "" {
			deleteAppConfigCacheField(ctx, appConfigCacheHash, group, name)
			return nil, false
		}
		return nil, true
	default:
		deleteAppConfigCacheField(ctx, appConfigCacheHash, group, name)
		return nil, false
	}
}

func cacheAppConfig(ctx *gin.Context, group, name, state, value string, ttl time.Duration) {
	cache := center.GetCache()
	if cache == nil {
		return
	}

	public := false
	payload, err := json.Marshal(appConfigCacheEnvelope{
		Version:   appConfigCacheEnvelopeVersion,
		Group:     group,
		Name:      name,
		Auth:      &public,
		State:     state,
		Value:     value,
		ExpiresAt: time.Now().Add(ttl).UnixMilli(),
	})
	if err != nil {
		return
	}
	cacheCtx, cancel := appConfigCacheContext(ctx)
	_ = cache.HSet(cacheCtx, appConfigCacheHash, appConfigCacheField(group, name), payload).Err()
	cancel()
}

func invalidateAppConfigCache(ctx *gin.Context, group, name string) {
	deleteAppConfigCacheField(ctx, appConfigCacheHash, group, name)
}

func (e *AppConfig) SetAppConfig(ctx *gin.Context, key string, auth bool, value string) error {
	if key == "" {
		return nil
	}

	group, name := splitAppConfigKey(key)
	if name == "" {
		return fmt.Errorf("%w: name is empty", ErrAppConfigIdentityMismatch)
	}
	canonicalGroup := strings.ToLower(strings.TrimSpace(norm.NFKC.String(group)))
	if canonicalGroup == ConfigRevisionResourceTheme && group != ConfigRevisionResourceTheme {
		return fmt.Errorf("%w: theme group must use canonical casing", ErrAppConfigIdentityMismatch)
	}

	db := center.GetDB(ctx, e).Clauses(dbresolver.Write)
	err := db.Transaction(func(tx *gorm.DB) error {
		profileRevision, err := lockApplicationConfigRevision(tx, ConfigRevisionResourcePublicProfile)
		if err != nil {
			return err
		}
		var themeRevision int64
		if group == ConfigRevisionResourceTheme {
			themeRevision, err = lockApplicationConfigRevision(tx, ConfigRevisionResourceTheme)
			if err != nil {
				return err
			}
		}
		if err = upsertExactAppConfig(tx, group, name, auth, value); err != nil {
			return err
		}
		if _, err = advanceApplicationConfigRevision(
			tx,
			ConfigRevisionResourcePublicProfile,
			profileRevision,
		); err != nil {
			return err
		}
		if group == ConfigRevisionResourceTheme {
			if _, err = advanceApplicationConfigRevision(
				tx,
				ConfigRevisionResourceTheme,
				themeRevision,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// The database is authoritative. Cache invalidation happens only after the write
	// succeeds and is deliberately best effort so Redis cannot break configuration writes.
	invalidateAppConfigCache(ctx, group, name)
	return nil
}

func lockApplicationConfigRevision(tx *gorm.DB, resource string) (int64, error) {
	row := &ConfigRevision{
		Scope: ConfigRevisionScopeApplication, OwnerID: "", Resource: resource, UpdatedAt: time.Now(),
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
		return 0, err
	}
	row = &ConfigRevision{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"scope = ? AND owner_id = ? AND resource = ?",
		ConfigRevisionScopeApplication,
		"",
		resource,
	).Take(row).Error
	if err != nil {
		return 0, err
	}
	if row.Scope != ConfigRevisionScopeApplication || row.OwnerID != "" || row.Resource != resource {
		return 0, fmt.Errorf(
			"%w: requested scope=%q resource=%q, stored scope=%q resource=%q, ownerEmpty=%t",
			ErrAppConfigIdentityMismatch,
			ConfigRevisionScopeApplication,
			resource,
			row.Scope,
			row.Resource,
			row.OwnerID == "",
		)
	}
	if row.Revision < 0 {
		return 0, fmt.Errorf("app config revision is negative for resource %q", resource)
	}
	return row.Revision, nil
}

func advanceApplicationConfigRevision(tx *gorm.DB, resource string, current int64) (int64, error) {
	if current < 0 || current == int64(^uint64(0)>>1) {
		return 0, fmt.Errorf("app config revision cannot advance for resource %q", resource)
	}
	next := current + 1
	result := tx.Model(&ConfigRevision{}).
		Where(
			"scope = ? AND owner_id = ? AND resource = ? AND revision = ?",
			ConfigRevisionScopeApplication,
			"",
			resource,
			current,
		).
		Updates(map[string]any{"revision": next, "updated_at": time.Now()})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, ErrAppConfigRevisionChanged
	}
	return next, nil
}

func upsertExactAppConfig(tx *gorm.DB, group, name string, auth bool, value string) error {
	candidates, err := exactAppConfigCandidates(tx, group, name)
	if err != nil {
		return err
	}
	if len(candidates) > 1 {
		return fmt.Errorf(
			"%w: group=%q name=%q matched %d rows",
			ErrAppConfigIdentityMismatch,
			group,
			name,
			len(candidates),
		)
	}
	now := time.Now()
	if len(candidates) == 1 {
		candidate := candidates[0]
		if candidate.Group != group || candidate.Name != name {
			return fmt.Errorf(
				"%w: requested group=%q name=%q, stored group=%q name=%q",
				ErrAppConfigIdentityMismatch,
				group,
				name,
				candidate.Group,
				candidate.Name,
			)
		}
		result := tx.Unscoped().Model(&AppConfig{}).
			Where("id = ?", candidate.ID).
			Updates(map[string]any{
				"group": group, "name": name, "auth": auth, "value": value,
				"updated_at": now, "deleted_at": nil,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			var count int64
			if err := tx.Unscoped().Model(&AppConfig{}).
				Where("id = ?", candidate.ID).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return fmt.Errorf("update app config %q/%q: row disappeared", group, name)
			}
		}
		return nil
	}
	record := &AppConfig{Group: group, Name: name, Auth: auth, Value: value}
	record.UpdatedAt = now
	return tx.Create(record).Error
}

func exactAppConfigCandidates(tx *gorm.DB, group, name string) ([]AppConfig, error) {
	newQuery := func() *gorm.DB { return tx.Session(&gorm.Session{NewDB: true}).Unscoped() }
	var exact []AppConfig
	if err := newQuery().
		Where(clause.Eq{Column: clause.Column{Name: "group"}, Value: group}).
		Where("name = ?", name).
		Find(&exact).Error; err != nil {
		return nil, err
	}
	fallback := newQuery()
	quotedGroup := fallback.Statement.Quote("group")
	quotedName := fallback.Statement.Quote("name")
	var trailingAliases []AppConfig
	if err := fallback.Where(
		fmt.Sprintf("RTRIM(%s) = ? AND RTRIM(%s) = ?", quotedGroup, quotedName),
		group,
		name,
	).Find(&trailingAliases).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]AppConfig, len(exact)+len(trailingAliases))
	for _, row := range exact {
		byID[row.ID] = row
	}
	for _, row := range trailingAliases {
		byID[row.ID] = row
	}
	result := make([]AppConfig, 0, len(byID))
	for _, row := range byID {
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func getAppConfig(ctx *gin.Context, key string) (*AppConfig, error) {
	if key == "" {
		return nil, fmt.Errorf("key is empty")
	}

	group, name := splitAppConfigKey(key)
	if cached, hit := cachedAppConfig(ctx, group, name, time.Now()); hit {
		if cached == nil {
			return nil, gorm.ErrRecordNotFound
		}
		return cached, nil
	}

	c := &AppConfig{}
	err := center.GetDB(ctx, c).
		Clauses(dbresolver.Write).
		Model(&AppConfig{}).
		Where(clause.Eq{Column: clause.Column{Name: "group"}, Value: group}).
		Where("name = ?", name).
		First(c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cacheAppConfig(ctx, group, name, appConfigCacheStateMissing, "", appConfigNegativeCacheTTL)
		}
		return nil, err
	}
	if c.Group != group || c.Name != name {
		cacheAppConfig(ctx, group, name, appConfigCacheStateMissing, "", appConfigNegativeCacheTTL)
		return nil, gorm.ErrRecordNotFound
	}
	if c.Auth {
		// Authenticated configuration can contain credentials. Never retain it in the
		// shared cache, including invalid envelopes encountered above.
		invalidateAppConfigCache(ctx, c.Group, c.Name)
	} else {
		cacheAppConfig(ctx, c.Group, c.Name, appConfigCacheStateFound, c.Value, appConfigCacheTTL)
	}
	return c, nil
}

func (e *AppConfig) GetAppConfig(ctx *gin.Context, key string) (string, bool) {
	c, err := getAppConfig(ctx, key)
	if err != nil {
		return "", false
	}
	return c.Value, true
}

// GetAppConfigSnapshot reads a bounded set of keys in one database statement.
// It bypasses the per-key cache so a security policy cannot combine values from
// different revisions or mistake a cache/database failure for a missing key.
func (e *AppConfig) GetAppConfigSnapshot(ctx *gin.Context, keys ...string) (map[string]string, error) {
	if ctx == nil || len(keys) == 0 {
		return nil, errors.New("application config snapshot context and keys are required")
	}
	group := ""
	names := make([]string, 0, len(keys))
	wanted := make(map[string]string, len(keys))
	for _, key := range keys {
		keyGroup, name := splitAppConfigKey(key)
		if keyGroup == "" || name == "" {
			return nil, fmt.Errorf("invalid application config snapshot key %q", key)
		}
		if group == "" {
			group = keyGroup
		}
		if keyGroup != group {
			return nil, errors.New("application config snapshot keys must share one group")
		}
		if _, exists := wanted[name]; exists {
			return nil, fmt.Errorf("duplicate application config snapshot key %q", key)
		}
		wanted[name] = key
		names = append(names, name)
	}

	var rows []AppConfig
	err := center.GetDB(ctx, e).
		Clauses(dbresolver.Write).
		Model(&AppConfig{}).
		Where(clause.Eq{Column: clause.Column{Name: "group"}, Value: group}).
		Where("name IN ?", names).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rows))
	for i := range rows {
		key, expected := wanted[rows[i].Name]
		if !expected || rows[i].Group != group {
			return nil, ErrAppConfigIdentityMismatch
		}
		if _, duplicate := result[key]; duplicate {
			return nil, ErrAppConfigIdentityMismatch
		}
		result[key] = rows[i].Value
	}
	return result, nil
}
