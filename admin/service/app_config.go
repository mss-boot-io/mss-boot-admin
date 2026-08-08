package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/11 22:01:11
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/11 22:01:11
 */

type AppConfig struct{}

var ErrAppConfigKeyCaseMismatch = errors.New("application config key casing does not match canonical public contract")
var ErrAppConfigKeyCollision = errors.New("application config canonical key collision")
var ErrLegacyThemeSnapshotMismatch = errors.New("legacy application theme snapshot does not match resource")

type AppConfigKeyCaseMismatchError struct {
	Group          string
	Name           string
	CanonicalGroup string
	CanonicalName  string
}

func (e *AppConfigKeyCaseMismatchError) Error() string {
	return fmt.Sprintf(
		"%s: got %s/%s, want %s/%s",
		ErrAppConfigKeyCaseMismatch,
		e.Group,
		e.Name,
		e.CanonicalGroup,
		e.CanonicalName,
	)
}

func (e *AppConfigKeyCaseMismatchError) Unwrap() error { return ErrAppConfigKeyCaseMismatch }

type AppConfigKeyCollisionError struct {
	Group      string
	Name       string
	Candidates int
}

func (e *AppConfigKeyCollisionError) Error() string {
	return fmt.Sprintf(
		"%s: key=%s/%s candidates=%d",
		ErrAppConfigKeyCollision,
		e.Group,
		e.Name,
		e.Candidates,
	)
}

func (e *AppConfigKeyCollisionError) Unwrap() error { return ErrAppConfigKeyCollision }

const (
	appConfigPublicProfileCacheKeyPrefix      = "app-configs:{profile:public}:v2:"
	appConfigPublicProfileLegacyCacheKey      = "app-configs:{profile:public}:payload"
	appConfigPublicProfileLegacyGenerationKey = "app-configs:{profile:public}:generation"
	appConfigPublicProfileMaxReadAttempts     = 3
	appConfigPublicProfileCacheTTL            = 15 * time.Minute
	appConfigPublicProfileCacheVersion        = 2
	appConfigEntryCacheHash                   = "app-configs:{entry}:v1"
	legacyAppConfigCacheHash                  = "appConfig"
	appConfigCacheOperationLimit              = 100 * time.Millisecond
)

type appConfigPublicKey struct {
	Group string
	Name  string
}

// sensitiveAppConfigKeys is the complete field-level secret contract for the
// application settings API. The persisted Auth flag is intentionally not an
// authorization source: historical rows may contain incorrect classifications
// and new private settings are not automatically credentials.
var sensitiveAppConfigKeys = []appConfigPublicKey{
	{Group: "email", Name: "password"},
	{Group: "security", Name: "githubClientSecret"},
	{Group: "security", Name: "larkAppSecret"},
	{Group: "storage", Name: "s3SecretAccessKey"},
}

var sensitiveAppConfigKeySet = func() map[appConfigPublicKey]struct{} {
	result := make(map[appConfigPublicKey]struct{}, len(sensitiveAppConfigKeys))
	for _, key := range sensitiveAppConfigKeys {
		result[key] = struct{}{}
	}
	return result
}()

// publicAppConfigKeys is the browser bootstrap contract. Additions require a
// review of the frontend consumer and the client-configuration security
// contract; every key not listed here is private by default.
var publicAppConfigKeys = []appConfigPublicKey{
	{Group: "base", Name: "websiteName"},
	{Group: "base", Name: "websiteDescription"},
	{Group: "base", Name: "websiteLogo"},
	{Group: "base", Name: "websiteRecordNumber"},
	{Group: "base", Name: "websiteCopyRight"},
	{Group: "security", Name: "registerEnabled"},
	{Group: "security", Name: "phoneEnabled"},
	{Group: "security", Name: "emailEnabled"},
	{Group: "security", Name: "githubEnabled"},
	{Group: "security", Name: "larkEnabled"},
	{Group: "theme", Name: "navTheme"},
	{Group: "theme", Name: "layout"},
	{Group: "theme", Name: "contentWidth"},
	{Group: "theme", Name: "fixedHeader"},
	{Group: "theme", Name: "fixSiderbar"},
	{Group: "theme", Name: "colorWeak"},
	{Group: "theme", Name: "colorPrimary"},
	{Group: "theme", Name: "pwa"},
}

var publicAppConfigKeySet = func() map[appConfigPublicKey]struct{} {
	result := make(map[appConfigPublicKey]struct{}, len(publicAppConfigKeys))
	for _, key := range publicAppConfigKeys {
		result[key] = struct{}{}
	}
	return result
}()

type publicAppConfigCacheEnvelope struct {
	Version         int              `json:"v"`
	ProfileRevision int64            `json:"profileRevision"`
	ThemeRevision   int64            `json:"themeRevision"`
	Profile         map[string]gin.H `json:"profile"`
}

// Profile returns the public application bootstrap projection. The auth
// argument is retained for source compatibility, but caller identity must
// never widen this endpoint to include authenticated-only or secret values.
func (e *AppConfig) Profile(ctx *gin.Context, _ bool) (map[string]gin.H, error) {
	// The revision/data/revision snapshot must come from the same authoritative
	// writer pool; a lagging replica must never populate a current revision key.
	db := center.GetDB(ctx, &models.AppConfig{})
	for attempt := 0; attempt < appConfigPublicProfileMaxReadAttempts; attempt++ {
		revisions, err := readPublicProfileRevisions(db)
		if err != nil {
			return nil, err
		}
		if cached, ok := getCachedPublicProfile(ctx, revisions.profile, revisions.theme); ok {
			afterCache, err := readPublicProfileRevisions(db)
			if err != nil {
				return nil, err
			}
			if revisions == afterCache {
				return cached, nil
			}
			continue
		}

		result, err := loadPublicProfile(db.Clauses(dbresolver.Write), revisions.theme)
		if err != nil {
			return nil, err
		}
		after, err := readPublicProfileRevisions(db)
		if err != nil {
			return nil, err
		}
		if revisions != after {
			continue
		}
		if err = cachePublicProfile(ctx, revisions.profile, revisions.theme, result); err != nil {
			slog.Warn("set public app config profile cache", "revision", revisions.profile, "err", err)
		}
		return result, nil
	}

	return nil, errors.New("app config profile changed during read")
}

type publicProfileRevisions struct {
	profile int64
	theme   int64
}

func readPublicProfileRevisions(db *gorm.DB) (publicProfileRevisions, error) {
	profileKey := applicationPublicProfileRevisionKey()
	profileRevision, err := readConfigRevision(db.Clauses(dbresolver.Write), profileKey)
	if err != nil {
		return publicProfileRevisions{}, err
	}
	themeKey := applicationThemeRevisionKey()
	themeRevision, err := readConfigRevision(db.Clauses(dbresolver.Write), themeKey)
	if err != nil {
		return publicProfileRevisions{}, err
	}
	return publicProfileRevisions{
		profile: profileRevision,
		theme:   themeRevision,
	}, nil
}

func loadPublicProfile(db *gorm.DB, themeRevision int64) (map[string]gin.H, error) {
	list := make([]*models.AppConfig, 0, len(publicAppConfigKeys))
	query := db.Where("1 = 0")
	for _, key := range publicAppConfigKeys {
		query = query.Or(&models.AppConfig{Group: key.Group, Name: key.Name})
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, err
	}

	result := make(map[string]gin.H)
	themeRecords := make([]themeRecord, 0, len(themeFieldNames))
	var legacyPWA *bool
	for i := range list {
		if !isPublicAppConfigKey(list[i].Group, list[i].Name) {
			continue
		}
		if list[i].Group == ThemeConfigGroup {
			if list[i].Name == "pwa" {
				if value, ok := normalizeLegacyPWAValue(list[i].Value); ok {
					legacyPWA = &value
				}
				continue
			}
			themeRecords = append(themeRecords, themeRecord{name: list[i].Name, value: list[i].Value})
			continue
		}
		if result[list[i].Group] == nil {
			result[list[i].Group] = make(gin.H)
		}
		result[list[i].Group][list[i].Name] = transferValue(list[i].Value)
	}
	theme, err := decodeThemeRecords(themeRecords)
	if err != nil {
		return nil, err
	}
	result[ThemeConfigGroup] = gin.H(themeResourceMap(newThemeResource(
		ThemeScopeApplication,
		themeRevision,
		theme,
	)))
	if legacyPWA != nil {
		result[ThemeConfigGroup]["pwa"] = *legacyPWA
	}
	return result, nil
}

func publicProfileCacheKey(revision int64) string {
	return appConfigPublicProfileCacheKeyPrefix + strconv.FormatInt(revision, 10)
}

func appConfigCacheContext(ctx *gin.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), appConfigCacheOperationLimit)
	}
	return context.WithTimeout(ctx, appConfigCacheOperationLimit)
}

func getCachedPublicProfile(ctx *gin.Context, profileRevision, themeRevision int64) (map[string]gin.H, bool) {
	cache := center.GetCache()
	if cache == nil {
		return nil, false
	}
	cacheCtx, cancel := appConfigCacheContext(ctx)
	defer cancel()
	payload, err := cache.Get(cacheCtx, publicProfileCacheKey(profileRevision)).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			slog.Warn("get public app config profile cache", "revision", profileRevision, "err", err)
		}
		return nil, false
	}
	envelope := &publicAppConfigCacheEnvelope{}
	if err = json.Unmarshal(payload, envelope); err != nil {
		slog.Warn("decode public app config profile cache", "err", err)
		return nil, false
	}
	if envelope.Version != appConfigPublicProfileCacheVersion ||
		envelope.ProfileRevision != profileRevision ||
		envelope.ThemeRevision != themeRevision ||
		envelope.Profile == nil {
		return nil, false
	}
	profile := publicProfileProjection(envelope.Profile)
	ensureApplicationThemeMeta(profile, themeRevision)
	return profile, true
}

func cachePublicProfile(
	ctx *gin.Context,
	profileRevision int64,
	themeRevision int64,
	profile map[string]gin.H,
) error {
	cache := center.GetCache()
	if cache == nil {
		return nil
	}
	payload, err := json.Marshal(&publicAppConfigCacheEnvelope{
		Version:         appConfigPublicProfileCacheVersion,
		ProfileRevision: profileRevision,
		ThemeRevision:   themeRevision,
		Profile:         publicProfileProjection(profile),
	})
	if err != nil {
		return err
	}
	cacheCtx, cancel := appConfigCacheContext(ctx)
	defer cancel()
	return cache.Set(
		cacheCtx,
		publicProfileCacheKey(profileRevision),
		payload,
		appConfigPublicProfileCacheTTL,
	).Err()
}

func publicProfileProjection(profile map[string]gin.H) map[string]gin.H {
	result := make(map[string]gin.H, len(profile))
	for group, values := range profile {
		for name, value := range values {
			if !isPublicAppConfigKey(group, name) {
				continue
			}
			if group == ThemeConfigGroup {
				if name == "pwa" {
					normalized, ok := normalizeLegacyPWAValue(value)
					if !ok {
						continue
					}
					value = normalized
				} else {
					normalized, ok := normalizeThemeProfileValue(name, value)
					if !ok {
						continue
					}
					value = normalized
				}
			}
			if result[group] == nil {
				result[group] = make(gin.H)
			}
			result[group][name] = value
		}
	}
	return result
}

func ensureApplicationThemeMeta(profile map[string]gin.H, revision int64) {
	if profile[ThemeConfigGroup] == nil {
		profile[ThemeConfigGroup] = make(gin.H)
	}
	profile[ThemeConfigGroup]["_meta"] = gin.H{
		"v":        themeResourceVersion,
		"scope":    ThemeScopeApplication,
		"revision": strconv.FormatInt(revision, 10),
	}
}

func normalizeLegacyPWAValue(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		if typed == "true" {
			return true, true
		}
		if typed == "false" {
			return false, true
		}
	}
	return false, false
}

func transferValue(value string) any {
	switch value {
	case "true":
		return true
	case "false":
		return false
	default:
		return value
	}
}

// Group returns the least-privileged group projection. Callers must opt in to
// sensitive values through GroupWithSensitiveValues after enforcing the
// dedicated field-level permission.
func (e *AppConfig) Group(ctx *gin.Context, group string) (map[string]any, error) {
	return e.GroupWithSensitiveValues(ctx, group, false)
}

func (e *AppConfig) GroupWithSensitiveValues(
	ctx *gin.Context,
	group string,
	includeSensitive bool,
) (map[string]any, error) {
	if err := validateAppConfigGroupCasing(group); err != nil {
		return nil, err
	}
	if group == ThemeConfigGroup {
		return e.LegacyThemeGroup(ctx, nil)
	}
	list := make([]*models.AppConfig, 0)
	db := center.GetDB(ctx, &models.AppConfig{})
	err := db.
		Where(&models.AppConfig{Group: group}).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]any)
	for i := range list {
		// Use the canonical request group and reject historical casing aliases
		// as secrets too. Case-insensitive database collations can otherwise
		// return a legacy `Security/githubClientSecret` row for `security`.
		if !includeSensitive && isSensitiveAppConfigKeyAlias(group, list[i].Name) {
			continue
		}
		result[list[i].Name] = list[i].Value
	}
	return result, nil
}

// AppConfigGroupContainsSensitiveValues reports whether a canonical group can
// contain one of the fixed credential fields. It is used to avoid consulting
// the field-level policy engine for groups that cannot expose credentials.
func AppConfigGroupContainsSensitiveValues(group string) bool {
	for _, key := range sensitiveAppConfigKeys {
		if group == key.Group {
			return true
		}
	}
	return false
}

// AppConfigMutationContainsSensitiveValues reports whether a request changes
// at least one fixed credential field. Mixed requests are authorized as one
// unit before the transaction starts.
func AppConfigMutationContainsSensitiveValues(group string, data map[string]any) bool {
	for name := range data {
		if isSensitiveAppConfigKey(group, name) {
			return true
		}
	}
	return false
}

func (e *AppConfig) LegacyThemeGroup(
	ctx *gin.Context,
	resource *dto.ThemeResource,
) (map[string]any, error) {
	snapshot, result, err := e.LegacyThemeResource(ctx)
	if err != nil {
		return nil, err
	}
	if resource != nil && !reflect.DeepEqual(resource, snapshot) {
		return nil, ErrLegacyThemeSnapshotMismatch
	}
	return result, nil
}

func (e *AppConfig) LegacyThemeResource(
	ctx *gin.Context,
) (*dto.ThemeResource, map[string]any, error) {
	return (&Theme{}).LegacyApplicationSnapshot(ctx)
}

func (e *AppConfig) ThemeResource(ctx *gin.Context) (*dto.ThemeResource, error) {
	return (&Theme{}).ApplicationResource(ctx)
}

func (e *AppConfig) UpdateTheme(
	ctx *gin.Context,
	data map[string]any,
	expectedRevision *int64,
) (*dto.ThemeResource, error) {
	return (&Theme{}).PatchApplicationResource(ctx, data, expectedRevision)
}

func (e *AppConfig) UpdateLegacyTheme(
	ctx *gin.Context,
	data map[string]any,
	expectedRevision *int64,
) (*dto.ThemeResource, error) {
	return (&Theme{}).PatchLegacyApplicationResource(ctx, data, expectedRevision)
}

func (e *AppConfig) UpdateLegacyThemeSnapshot(
	ctx *gin.Context,
	data map[string]any,
	expectedRevision *int64,
) (*dto.ThemeResource, map[string]any, error) {
	return (&Theme{}).PatchLegacyApplicationSnapshot(ctx, data, expectedRevision)
}

func (e *AppConfig) ResetTheme(
	ctx *gin.Context,
	expectedRevision *int64,
) (*dto.ThemeResource, error) {
	return (&Theme{}).ResetApplicationResource(ctx, expectedRevision)
}

func (e *AppConfig) ResetLegacyThemeSnapshot(
	ctx *gin.Context,
	expectedRevision *int64,
) (*dto.ThemeResource, map[string]any, error) {
	return (&Theme{}).ResetLegacyApplicationSnapshot(ctx, expectedRevision)
}

func (e *AppConfig) CreateOrUpdate(ctx *gin.Context, group string, data map[string]any) error {
	if err := validateAppConfigGroupCasing(group); err != nil {
		return err
	}
	for name := range data {
		if err := validatePublicAppConfigKeyCasing(group, name); err != nil {
			return err
		}
	}
	if group == ThemeConfigGroup {
		_, err := (&Theme{}).PatchLegacyApplicationResource(ctx, data, nil)
		return err
	}
	if len(data) == 0 {
		return nil
	}

	keys := make([]string, 0, len(data))
	publicMutation := false
	for name := range data {
		keys = append(keys, name)
		publicMutation = publicMutation || isPublicAppConfigKey(group, name)
	}
	sort.Strings(keys)

	db := center.GetDB(ctx, &models.AppConfig{})
	var staleProfileRevision int64
	err := db.Transaction(func(tx *gorm.DB) error {
		var profileRevision int64
		if publicMutation {
			var err error
			profileRevision, err = lockConfigRevision(tx, applicationPublicProfileRevisionKey())
			if err != nil {
				return err
			}
		}
		for _, name := range keys {
			if err := upsertApplicationConfigValue(
				tx,
				group,
				name,
				cast.ToString(data[name]),
				!isPublicAppConfigKey(group, name),
			); err != nil {
				return err
			}
		}
		if publicMutation {
			if _, err := advanceConfigRevision(tx, applicationPublicProfileRevisionKey(), profileRevision); err != nil {
				return err
			}
			staleProfileRevision = profileRevision
		}
		return nil
	})
	if err != nil {
		return err
	}

	invalidateAppConfigEntryCaches(ctx, group, keys)
	if publicMutation {
		cleanupPublicProfileCache(ctx, staleProfileRevision)
	}
	return nil
}

func (e *AppConfig) Reset(ctx *gin.Context, group string) error {
	if err := rejectNonCanonicalThemeGroup(group); err != nil {
		return err
	}
	if group != ThemeConfigGroup {
		return ErrThemeGroupOnly
	}
	return (&Theme{}).ResetApplication(ctx)
}

func upsertApplicationConfigValue(tx *gorm.DB, group, name, value string, auth bool) error {
	if !auth && isPublicAppConfigKey(group, name) {
		return upsertCanonicalPublicAppConfigValue(tx, group, name, value)
	}
	record := &models.AppConfig{Group: group, Name: name, Value: value, Auth: auth}
	record.UpdatedAt = time.Now()
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}, {Name: "group"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"auth", "value", "updated_at", "deleted_at",
		}),
	}).Create(record).Error
}

func upsertCanonicalPublicAppConfigValue(tx *gorm.DB, group, name, value string) error {
	candidates, err := appConfigCollationCandidates(tx, group, name, true)
	if err != nil {
		return err
	}
	if len(candidates) > 1 {
		return &AppConfigKeyCollisionError{Group: group, Name: name, Candidates: len(candidates)}
	}
	now := time.Now()
	if len(candidates) == 1 {
		result := tx.Unscoped().Model(&models.AppConfig{}).
			Where("id = ?", candidates[0].ID).
			Updates(map[string]any{
				"group":      group,
				"name":       name,
				"value":      value,
				"auth":       false,
				"updated_at": now,
				"deleted_at": nil,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var count int64
			if err := tx.Unscoped().Model(&models.AppConfig{}).
				Where("id = ?", candidates[0].ID).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return fmt.Errorf("canonicalize public app config %s/%s: row disappeared", group, name)
			}
		}
		return nil
	}
	record := &models.AppConfig{Group: group, Name: name, Value: value, Auth: false}
	record.UpdatedAt = now
	return tx.Create(record).Error
}

func invalidateAppConfigEntryCaches(ctx *gin.Context, group string, keys []string) {
	cache := center.GetCache()
	if cache == nil {
		return
	}
	fields := make([]string, 0, len(keys))
	for _, name := range keys {
		fields = append(fields, fmt.Sprintf("%s:%s", group, name))
	}
	if len(fields) == 0 {
		return
	}
	for _, hash := range []string{appConfigEntryCacheHash, legacyAppConfigCacheHash} {
		cacheCtx, cancel := appConfigCacheContext(ctx)
		err := cache.HDel(cacheCtx, hash, fields...).Err()
		cancel()
		if err != nil {
			slog.Warn(
				"invalidate app config cache after commit",
				"group", group,
				"hash", hash,
				"err", err,
			)
		}
	}
}

func cleanupPublicProfileCache(ctx *gin.Context, staleRevision int64) {
	cache := center.GetCache()
	if cache == nil {
		return
	}
	cacheCtx, cancel := appConfigCacheContext(ctx)
	defer cancel()
	if err := cache.Del(
		cacheCtx,
		publicProfileCacheKey(staleRevision),
		appConfigPublicProfileLegacyCacheKey,
		appConfigPublicProfileLegacyGenerationKey,
	).Err(); err != nil {
		slog.Warn("clean up public app config profile cache after commit", "revision", staleRevision, "err", err)
	}
}

func isPublicAppConfigKey(group, name string) bool {
	_, ok := publicAppConfigKeySet[appConfigPublicKey{Group: group, Name: name}]
	return ok
}

func isSensitiveAppConfigKey(group, name string) bool {
	_, ok := sensitiveAppConfigKeySet[appConfigPublicKey{Group: group, Name: name}]
	return ok
}

func isSensitiveAppConfigKeyAlias(group, name string) bool {
	for _, key := range sensitiveAppConfigKeys {
		if canonicalConfigIdentifierFold(group) == canonicalConfigIdentifierFold(key.Group) &&
			canonicalConfigIdentifierFold(name) == canonicalConfigIdentifierFold(key.Name) {
			return true
		}
	}
	return false
}

func validateAppConfigGroupCasing(group string) error {
	if err := rejectNonCanonicalThemeGroup(group); err != nil {
		return err
	}
	for _, key := range publicAppConfigKeys {
		if group != key.Group && canonicalConfigIdentifierFold(group) == canonicalConfigIdentifierFold(key.Group) {
			return &AppConfigKeyCaseMismatchError{
				Group: group, CanonicalGroup: key.Group,
			}
		}
	}
	for _, key := range sensitiveAppConfigKeys {
		if group != key.Group && canonicalConfigIdentifierFold(group) == canonicalConfigIdentifierFold(key.Group) {
			return &AppConfigKeyCaseMismatchError{
				Group: group, CanonicalGroup: key.Group,
			}
		}
	}
	return nil
}

func validatePublicAppConfigKeyCasing(group, name string) error {
	for _, key := range publicAppConfigKeys {
		if canonicalConfigIdentifierFold(group) == canonicalConfigIdentifierFold(key.Group) &&
			canonicalConfigIdentifierFold(name) == canonicalConfigIdentifierFold(key.Name) &&
			(group != key.Group || name != key.Name) {
			return &AppConfigKeyCaseMismatchError{
				Group:          group,
				Name:           name,
				CanonicalGroup: key.Group,
				CanonicalName:  key.Name,
			}
		}
	}
	for _, key := range sensitiveAppConfigKeys {
		if canonicalConfigIdentifierFold(group) == canonicalConfigIdentifierFold(key.Group) &&
			canonicalConfigIdentifierFold(name) == canonicalConfigIdentifierFold(key.Name) &&
			(group != key.Group || name != key.Name) {
			return &AppConfigKeyCaseMismatchError{
				Group:          group,
				Name:           name,
				CanonicalGroup: key.Group,
				CanonicalName:  key.Name,
			}
		}
	}
	return nil
}
