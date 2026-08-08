package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
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
 * @Date: 2024/3/2 00:42:39
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/2 00:42:39
 */

type UserConfig struct{}

const (
	userConfigCacheVersion        = 1
	userConfigReadMaxAttempts     = 3
	userConfigCacheTTL            = 15 * time.Minute
	userConfigCacheOperationLimit = 100 * time.Millisecond
)

var (
	ErrUserConfigKeyCaseMismatch = errors.New("user config key casing does not match stored database key")
	ErrUserConfigUserRequired    = errors.New("user config user id is required")
	ErrInvalidUserConfigKey      = errors.New("invalid user config key")
)

type userConfigProfileCacheEnvelope struct {
	Version     int              `json:"v"`
	OwnerDigest string           `json:"ownerDigest"`
	Revision    int64            `json:"revision"`
	Profile     map[string]gin.H `json:"profile"`
}

type userConfigGroupCacheEnvelope struct {
	Version     int            `json:"v"`
	OwnerDigest string         `json:"ownerDigest"`
	GroupDigest string         `json:"groupDigest"`
	Revision    int64          `json:"revision"`
	Values      map[string]any `json:"values"`
}

type userThemeCacheEnvelope struct {
	Version     int                `json:"v"`
	OwnerDigest string             `json:"ownerDigest"`
	Revision    int64              `json:"revision"`
	Resource    *dto.ThemeResource `json:"resource"`
}

type UserConfigKeyCaseMismatchError struct {
	UserID       string
	Group        string
	Name         string
	StoredUserID string
	StoredGroup  string
	StoredName   string
}

func (e *UserConfigKeyCaseMismatchError) Error() string {
	return fmt.Sprintf(
		"%s: group=%q name=%q storedGroup=%q storedName=%q ownerMatch=%t",
		ErrUserConfigKeyCaseMismatch,
		e.Group,
		e.Name,
		e.StoredGroup,
		e.StoredName,
		e.UserID == e.StoredUserID,
	)
}

func (e *UserConfigKeyCaseMismatchError) Unwrap() error { return ErrUserConfigKeyCaseMismatch }

func (e *UserConfig) Profile(ctx *gin.Context, userID string) (map[string]gin.H, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrUserConfigUserRequired
	}
	db := center.GetDB(ctx, &models.UserConfig{})
	key := userConfigProfileRevisionKey(userID)
	for attempt := 0; attempt < userConfigReadMaxAttempts; attempt++ {
		before, err := readConfigRevision(db.Clauses(dbresolver.Write), key)
		if err != nil {
			return nil, err
		}
		cached, hit, cacheReady := getCachedUserConfigProfile(ctx, userID, before)
		if hit {
			after, err := readConfigRevision(db.Clauses(dbresolver.Write), key)
			if err != nil {
				return nil, err
			}
			if before == after {
				return cached, nil
			}
			continue
		}

		result, err := loadUserConfigProfile(db, userID)
		if err != nil {
			return nil, err
		}
		after, err := readConfigRevision(db.Clauses(dbresolver.Write), key)
		if err != nil {
			return nil, err
		}
		if before != after {
			continue
		}
		if cacheReady {
			if err = cacheUserConfigProfile(ctx, userID, after, result); err != nil {
				slog.Warn("set user config profile cache", "revision", after, "err", err)
			}
		}
		return result, nil
	}
	return nil, errors.New("user config profile changed during read")
}

func loadUserConfigProfile(db *gorm.DB, userID string) (map[string]gin.H, error) {
	list := make([]*models.UserConfig, 0)
	err := db.Clauses(dbresolver.Write).
		Where("user_id = ?", userID).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]gin.H)
	themeRecords := make([]themeRecord, 0, len(themeFieldNames))
	for i := range list {
		if list[i].UserID != userID {
			return nil, userConfigKeyMismatch(userID, list[i].Group, list[i].Name, *list[i])
		}
		if canonicalConfigIdentifierFold(list[i].Group) == ThemeConfigGroup {
			if list[i].Group != ThemeConfigGroup {
				return nil, userConfigKeyMismatch(userID, ThemeConfigGroup, list[i].Name, *list[i])
			}
			if isCanonicalThemeField(list[i].Name) {
				themeRecords = append(themeRecords, themeRecord{name: list[i].Name, value: list[i].Value})
			}
			continue
		}
		if _, ok := result[list[i].Group]; !ok {
			result[list[i].Group] = make(gin.H)
		}
		result[list[i].Group][list[i].Name] = list[i].Value
	}
	themeRevision, err := readConfigRevision(db.Clauses(dbresolver.Write), userThemeRevisionKey(userID))
	if err != nil {
		return nil, err
	}
	theme, err := decodeThemeRecords(themeRecords)
	if err != nil {
		return nil, err
	}
	result[ThemeConfigGroup] = gin.H(themeResourceMap(newThemeResource(ThemeScopeUser, themeRevision, theme)))
	return result, nil
}

func (e *UserConfig) ThemeResource(ctx *gin.Context, userID string) (*dto.ThemeResource, error) {
	return (&Theme{}).UserResource(ctx, userID)
}

func (e *UserConfig) LegacyThemeGroup(
	ctx *gin.Context,
	userID string,
	resource *dto.ThemeResource,
) (map[string]any, error) {
	if resource == nil {
		var err error
		resource, err = (&Theme{}).UserResource(ctx, userID)
		if err != nil {
			return nil, err
		}
	}
	return themeOverridesMap(&resource.ThemeOverrides), nil
}

func (e *UserConfig) UpdateTheme(
	ctx *gin.Context,
	userID string,
	data map[string]any,
	expectedRevision *int64,
) (*dto.ThemeResource, error) {
	return (&Theme{}).PatchUserResource(ctx, userID, data, expectedRevision)
}

func (e *UserConfig) ResetTheme(
	ctx *gin.Context,
	userID string,
	expectedRevision *int64,
) (*dto.ThemeResource, error) {
	return (&Theme{}).ResetUserResource(ctx, userID, expectedRevision)
}

func (e *UserConfig) Group(ctx *gin.Context, userID, group string) (map[string]any, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrUserConfigUserRequired
	}
	if err := rejectNonCanonicalThemeGroup(group); err != nil {
		return nil, err
	}
	if group == ThemeConfigGroup {
		return e.LegacyThemeGroup(ctx, userID, nil)
	}
	db := center.GetDB(ctx, &models.UserConfig{})
	key := userConfigProfileRevisionKey(userID)
	for attempt := 0; attempt < userConfigReadMaxAttempts; attempt++ {
		before, err := readConfigRevision(db.Clauses(dbresolver.Write), key)
		if err != nil {
			return nil, err
		}
		cached, hit, cacheReady := getCachedUserConfigGroup(ctx, userID, group, before)
		if hit {
			after, err := readConfigRevision(db.Clauses(dbresolver.Write), key)
			if err != nil {
				return nil, err
			}
			if before == after {
				return cached, nil
			}
			continue
		}
		list, err := userConfigGroupCandidates(db, userID, group)
		if err != nil {
			return nil, err
		}
		result := make(map[string]any)
		seenNames := make(map[string]string, len(list))
		for i := range list {
			if list[i].UserID != userID || list[i].Group != group {
				return nil, userConfigKeyMismatch(userID, group, list[i].Name, list[i])
			}
			foldedName := canonicalConfigIdentifierFold(list[i].Name)
			if stored, ok := seenNames[foldedName]; ok && stored != list[i].Name {
				return nil, &UserConfigKeyCaseMismatchError{
					UserID: userID, Group: group, Name: stored,
					StoredUserID: list[i].UserID, StoredGroup: list[i].Group, StoredName: list[i].Name,
				}
			}
			seenNames[foldedName] = list[i].Name
			result[list[i].Name] = list[i].Value
		}
		after, err := readConfigRevision(db.Clauses(dbresolver.Write), key)
		if err != nil {
			return nil, err
		}
		if before != after {
			continue
		}
		if cacheReady {
			if err = cacheUserConfigGroup(ctx, userID, group, after, result); err != nil {
				slog.Warn("set user config group cache", "revision", after, "err", err)
			}
		}
		return result, nil
	}
	return nil, errors.New("user config group changed during read")
}

func (e *UserConfig) CreateOrUpdate(ctx *gin.Context, userID, group string, data map[string]any) error {
	if strings.TrimSpace(userID) == "" {
		return ErrUserConfigUserRequired
	}
	if group == "" {
		return fmt.Errorf("%w: group is empty", ErrInvalidUserConfigKey)
	}
	if err := rejectNonCanonicalThemeGroup(group); err != nil {
		return err
	}
	if group == ThemeConfigGroup {
		return (&Theme{}).PatchUser(ctx, userID, data)
	}
	keys := make([]string, 0, len(data))
	for name := range data {
		if name == "" {
			return fmt.Errorf("%w: name is empty", ErrInvalidUserConfigKey)
		}
		keys = append(keys, name)
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	db := center.GetDB(ctx, &models.UserConfig{}).Clauses(dbresolver.Write)
	var staleProfileRevision int64
	err := db.Transaction(func(tx *gorm.DB) error {
		profileRevision, err := lockConfigRevision(tx, userConfigProfileRevisionKey(userID))
		if err != nil {
			return err
		}
		for _, name := range keys {
			if err := upsertUserConfigValue(tx, userID, group, name, cast.ToString(data[name])); err != nil {
				return err
			}
		}
		if _, err := advanceConfigRevision(tx, userConfigProfileRevisionKey(userID), profileRevision); err != nil {
			return err
		}
		staleProfileRevision = profileRevision
		return nil
	})
	if err != nil {
		return err
	}
	cleanupUserConfigProfileCache(ctx, userID, staleProfileRevision)
	cleanupUserConfigGroupCache(ctx, userID, group, staleProfileRevision)
	return nil
}

func (e *UserConfig) Reset(ctx *gin.Context, userID, group string) error {
	if err := rejectNonCanonicalThemeGroup(group); err != nil {
		return err
	}
	if group != ThemeConfigGroup {
		return ErrThemeGroupOnly
	}
	return (&Theme{}).ResetUser(ctx, userID)
}

func userConfigProfileRevisionKey(userID string) configRevisionKey {
	return configRevisionKey{scope: ThemeScopeUser, ownerID: userID, resource: configRevisionResourceUserProfile}
}

func upsertUserConfigValue(tx *gorm.DB, userID, group, name, value string) error {
	candidates, err := userConfigKeyCandidates(tx, userID, group, name, true)
	if err != nil {
		return err
	}
	if len(candidates) > 1 {
		return userConfigKeyMismatch(userID, group, name, candidates[0])
	}
	now := time.Now()
	if len(candidates) == 1 {
		candidate := candidates[0]
		if candidate.UserID != userID || candidate.Group != group || candidate.Name != name {
			return userConfigKeyMismatch(userID, group, name, candidate)
		}
		result := tx.Unscoped().Model(&models.UserConfig{}).
			Where("id = ?", candidate.ID).
			Updates(map[string]any{
				"user_id": userID, "group": group, "name": name, "value": value,
				"updated_at": now, "deleted_at": nil,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var count int64
			if err := tx.Unscoped().Model(&models.UserConfig{}).Where("id = ?", candidate.ID).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return fmt.Errorf("update user config %q/%q: row disappeared", group, name)
			}
		}
		return nil
	}
	record := &models.UserConfig{UserID: userID, Group: group, Name: name, Value: value}
	record.UpdatedAt = now
	return tx.Create(record).Error
}

func userConfigKeyCandidates(
	db *gorm.DB,
	userID, group, name string,
	includeDeleted bool,
) ([]models.UserConfig, error) {
	newQuery := func() *gorm.DB {
		query := db.Session(&gorm.Session{NewDB: true})
		if includeDeleted {
			query = query.Unscoped()
		}
		return query
	}
	var rows []models.UserConfig
	if err := newQuery().
		Where("user_id = ?", userID).
		Where(clause.Eq{Column: clause.Column{Name: "group"}, Value: group}).
		Where("name = ?", name).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	fallback := newQuery()
	quotedUserID := fallback.Statement.Quote("user_id")
	quotedGroup := fallback.Statement.Quote("group")
	quotedName := fallback.Statement.Quote("name")
	var trailingAliases []models.UserConfig
	if err := fallback.Where(
		fmt.Sprintf("RTRIM(%s) = ? AND RTRIM(%s) = ? AND RTRIM(%s) = ?", quotedUserID, quotedGroup, quotedName),
		userID,
		group,
		name,
	).Find(&trailingAliases).Error; err != nil {
		return nil, err
	}
	result := mergeUserConfigCandidates(rows, trailingAliases)
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func userConfigGroupCandidates(db *gorm.DB, userID, group string) ([]models.UserConfig, error) {
	newQuery := func() *gorm.DB { return db.Clauses(dbresolver.Write) }
	var rows []models.UserConfig
	if err := newQuery().
		Where("user_id = ?", userID).
		Where(clause.Eq{Column: clause.Column{Name: "group"}, Value: group}).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	fallback := newQuery()
	quotedUserID := fallback.Statement.Quote("user_id")
	quotedGroup := fallback.Statement.Quote("group")
	var trailingAliases []models.UserConfig
	if err := fallback.Where(
		fmt.Sprintf("RTRIM(%s) = ? AND RTRIM(%s) = ?", quotedUserID, quotedGroup),
		userID,
		group,
	).Find(&trailingAliases).Error; err != nil {
		return nil, err
	}
	result := mergeUserConfigCandidates(rows, trailingAliases)
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func userConfigKeyMismatch(userID, group, name string, stored models.UserConfig) error {
	return &UserConfigKeyCaseMismatchError{
		UserID: userID, Group: group, Name: name,
		StoredUserID: stored.UserID, StoredGroup: stored.Group, StoredName: stored.Name,
	}
}

func userConfigOwnerDigest(userID string) string {
	sum := sha256.Sum256([]byte("mss:user-config-owner:v1\x00" + userID))
	return hex.EncodeToString(sum[:])
}

func userConfigProfileCacheKey(userID string, revision int64) string {
	return fmt.Sprintf(
		"user-configs:{%s}:profile:v%d:%s",
		userConfigOwnerDigest(userID),
		userConfigCacheVersion,
		strconv.FormatInt(revision, 10),
	)
}

func userThemeCacheKey(userID string, revision int64) string {
	return fmt.Sprintf(
		"user-configs:{%s}:theme:v%d:%s",
		userConfigOwnerDigest(userID),
		userConfigCacheVersion,
		strconv.FormatInt(revision, 10),
	)
}

func userConfigGroupDigest(group string) string {
	sum := sha256.Sum256([]byte("mss:user-config-group:v1\x00" + group))
	return hex.EncodeToString(sum[:])
}

func userConfigGroupCacheKey(userID, group string, revision int64) string {
	return fmt.Sprintf(
		"user-configs:{%s}:group:%s:v%d:%s",
		userConfigOwnerDigest(userID),
		userConfigGroupDigest(group),
		userConfigCacheVersion,
		strconv.FormatInt(revision, 10),
	)
}

func getCachedUserConfigProfile(
	ctx *gin.Context,
	userID string,
	revision int64,
) (map[string]gin.H, bool, bool) {
	cache := center.GetCache()
	if cache == nil {
		return nil, false, false
	}
	cacheCtx, cancel := boundedUserConfigCacheContext(ctx)
	defer cancel()
	payload, err := cache.Get(cacheCtx, userConfigProfileCacheKey(userID, revision)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, true
		}
		slog.Warn("get user config profile cache", "revision", revision, "err", err)
		return nil, false, false
	}
	envelope := &userConfigProfileCacheEnvelope{}
	if err = json.Unmarshal(payload, envelope); err != nil {
		slog.Warn("decode user config profile cache", "revision", revision, "err", err)
		return nil, false, true
	}
	if envelope.Version != userConfigCacheVersion ||
		envelope.OwnerDigest != userConfigOwnerDigest(userID) ||
		envelope.Revision != revision ||
		envelope.Profile == nil {
		return nil, false, true
	}
	return envelope.Profile, true, true
}

func cacheUserConfigProfile(
	ctx *gin.Context,
	userID string,
	revision int64,
	profile map[string]gin.H,
) error {
	cache := center.GetCache()
	if cache == nil {
		return nil
	}
	payload, err := json.Marshal(&userConfigProfileCacheEnvelope{
		Version: userConfigCacheVersion, OwnerDigest: userConfigOwnerDigest(userID),
		Revision: revision, Profile: profile,
	})
	if err != nil {
		return err
	}
	cacheCtx, cancel := boundedUserConfigCacheContext(ctx)
	defer cancel()
	return cache.Set(cacheCtx, userConfigProfileCacheKey(userID, revision), payload, userConfigCacheTTL).Err()
}

func getCachedUserConfigGroup(
	ctx *gin.Context,
	userID, group string,
	revision int64,
) (map[string]any, bool, bool) {
	cache := center.GetCache()
	if cache == nil {
		return nil, false, false
	}
	cacheCtx, cancel := boundedUserConfigCacheContext(ctx)
	defer cancel()
	payload, err := cache.Get(cacheCtx, userConfigGroupCacheKey(userID, group, revision)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, true
		}
		slog.Warn("get user config group cache", "revision", revision, "err", err)
		return nil, false, false
	}
	envelope := &userConfigGroupCacheEnvelope{}
	if err = json.Unmarshal(payload, envelope); err != nil {
		slog.Warn("decode user config group cache", "revision", revision, "err", err)
		return nil, false, true
	}
	if envelope.Version != userConfigCacheVersion ||
		envelope.OwnerDigest != userConfigOwnerDigest(userID) ||
		envelope.GroupDigest != userConfigGroupDigest(group) ||
		envelope.Revision != revision ||
		envelope.Values == nil {
		return nil, false, true
	}
	return envelope.Values, true, true
}

func cacheUserConfigGroup(
	ctx *gin.Context,
	userID, group string,
	revision int64,
	values map[string]any,
) error {
	cache := center.GetCache()
	if cache == nil {
		return nil
	}
	payload, err := json.Marshal(&userConfigGroupCacheEnvelope{
		Version: userConfigCacheVersion, OwnerDigest: userConfigOwnerDigest(userID),
		GroupDigest: userConfigGroupDigest(group), Revision: revision, Values: values,
	})
	if err != nil {
		return err
	}
	cacheCtx, cancel := boundedUserConfigCacheContext(ctx)
	defer cancel()
	return cache.Set(cacheCtx, userConfigGroupCacheKey(userID, group, revision), payload, userConfigCacheTTL).Err()
}

func getCachedUserTheme(
	ctx *gin.Context,
	userID string,
	revision int64,
) (*dto.ThemeResource, bool, bool) {
	cache := center.GetCache()
	if cache == nil {
		return nil, false, false
	}
	cacheCtx, cancel := boundedUserConfigCacheContext(ctx)
	defer cancel()
	payload, err := cache.Get(cacheCtx, userThemeCacheKey(userID, revision)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, true
		}
		slog.Warn("get user theme cache", "revision", revision, "err", err)
		return nil, false, false
	}
	envelope := &userThemeCacheEnvelope{}
	if err = json.Unmarshal(payload, envelope); err != nil {
		slog.Warn("decode user theme cache", "revision", revision, "err", err)
		return nil, false, true
	}
	if envelope.Version != userConfigCacheVersion ||
		envelope.OwnerDigest != userConfigOwnerDigest(userID) ||
		envelope.Revision != revision ||
		envelope.Resource == nil ||
		envelope.Resource.Meta.Version != themeResourceVersion ||
		envelope.Resource.Meta.Scope != ThemeScopeUser ||
		envelope.Resource.Meta.Revision != strconv.FormatInt(revision, 10) {
		return nil, false, true
	}
	return envelope.Resource, true, true
}

func cacheUserTheme(ctx *gin.Context, userID string, revision int64, resource *dto.ThemeResource) error {
	cache := center.GetCache()
	if cache == nil || resource == nil {
		return nil
	}
	payload, err := json.Marshal(&userThemeCacheEnvelope{
		Version: userConfigCacheVersion, OwnerDigest: userConfigOwnerDigest(userID),
		Revision: revision, Resource: resource,
	})
	if err != nil {
		return err
	}
	cacheCtx, cancel := boundedUserConfigCacheContext(ctx)
	defer cancel()
	return cache.Set(cacheCtx, userThemeCacheKey(userID, revision), payload, userConfigCacheTTL).Err()
}

func cleanupUserConfigProfileCache(ctx *gin.Context, userID string, staleRevision int64) {
	cleanupUserConfigCacheKeys(ctx, userConfigProfileCacheKey(userID, staleRevision))
}

func cleanupUserConfigGroupCache(ctx *gin.Context, userID, group string, staleRevision int64) {
	cleanupUserConfigCacheKeys(ctx, userConfigGroupCacheKey(userID, group, staleRevision))
}

func cleanupUserThemeCache(ctx *gin.Context, userID string, staleRevision int64) {
	cleanupUserConfigCacheKeys(ctx, userThemeCacheKey(userID, staleRevision))
}

func cleanupUserConfigCacheKeys(ctx *gin.Context, keys ...string) {
	cache := center.GetCache()
	if cache == nil || len(keys) == 0 {
		return
	}
	cacheCtx, cancel := boundedUserConfigCacheContext(ctx)
	defer cancel()
	if err := cache.Del(cacheCtx, keys...).Err(); err != nil {
		slog.Warn("clean up user config cache after commit", "err", err)
	}
}

func boundedUserConfigCacheContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, userConfigCacheOperationLimit)
}
