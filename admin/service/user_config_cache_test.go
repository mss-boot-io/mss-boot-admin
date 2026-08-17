package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
)

type userConfigStaleCacheHitHook struct {
	key     string
	reached chan struct{}
	release chan struct{}
	blocked atomic.Bool
}

func (*userConfigStaleCacheHitHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (hook *userConfigStaleCacheHitHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := next(ctx, cmd)
		if err != nil || cmd.Name() != "get" {
			return err
		}
		args := cmd.Args()
		if len(args) < 2 {
			return err
		}
		key, ok := args[1].(string)
		if !ok || key != hook.key || !hook.blocked.CompareAndSwap(false, true) {
			return err
		}
		close(hook.reached)
		select {
		case <-hook.release:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (*userConfigStaleCacheHitHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

type userConfigConcurrentReadResult struct {
	value string
	err   error
}

func assertUserConfigCacheHitRevalidatesRevision(
	t *testing.T,
	env appConfigTestEnv,
	cacheKey string,
	read func(*gin.Context) (string, error),
	mutate func(*gin.Context) error,
	want string,
) {
	t.Helper()
	require.True(t, env.redis.Exists(cacheKey), "test requires a populated cache entry")
	hook := &userConfigStaleCacheHitHook{
		key: cacheKey, reached: make(chan struct{}), release: make(chan struct{}),
	}
	env.client.AddHook(hook)

	resultCh := make(chan userConfigConcurrentReadResult, 1)
	readCtx := env.ctx.Copy()
	go func() {
		value, err := read(readCtx)
		resultCh <- userConfigConcurrentReadResult{value: value, err: err}
	}()

	select {
	case <-hook.reached:
	case <-time.After(2 * time.Second):
		t.Fatal("cache GET did not reach the deterministic concurrency barrier")
	}
	released := false
	defer func() {
		if !released {
			close(hook.release)
		}
	}()
	require.NoError(t, mutate(env.ctx.Copy()))
	close(hook.release)
	released = true

	select {
	case result := <-resultCh:
		require.NoError(t, result.err)
		require.Equal(t, want, result.value)
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent cache read did not finish")
	}
}

func TestUserConfigCacheHitsRevalidateRevisionAfterConcurrentMutation(t *testing.T) {
	t.Run("profile", func(t *testing.T) {
		env := setupAppConfigTestEnv(t)
		require.NoError(t, env.db.Create(&models.UserConfig{
			UserID: "user-a", Group: "notification", Name: "desktop", Value: "old",
		}).Error)
		svc := &UserConfig{}
		_, err := svc.Profile(env.ctx, "user-a")
		require.NoError(t, err)
		assertUserConfigCacheHitRevalidatesRevision(
			t,
			env,
			userConfigProfileCacheKey("user-a", 0),
			func(ctx *gin.Context) (string, error) {
				profile, readErr := svc.Profile(ctx, "user-a")
				if readErr != nil {
					return "", readErr
				}
				return fmt.Sprint(profile["notification"]["desktop"]), nil
			},
			func(ctx *gin.Context) error {
				return svc.CreateOrUpdate(ctx, "user-a", "notification", map[string]any{"desktop": "new"})
			},
			"new",
		)
	})

	t.Run("group", func(t *testing.T) {
		env := setupAppConfigTestEnv(t)
		require.NoError(t, env.db.Create(&models.UserConfig{
			UserID: "user-a", Group: "notification", Name: "desktop", Value: "old",
		}).Error)
		svc := &UserConfig{}
		_, err := svc.Group(env.ctx, "user-a", "notification")
		require.NoError(t, err)
		assertUserConfigCacheHitRevalidatesRevision(
			t,
			env,
			userConfigGroupCacheKey("user-a", "notification", 0),
			func(ctx *gin.Context) (string, error) {
				group, readErr := svc.Group(ctx, "user-a", "notification")
				if readErr != nil {
					return "", readErr
				}
				return fmt.Sprint(group["desktop"]), nil
			},
			func(ctx *gin.Context) error {
				return svc.CreateOrUpdate(ctx, "user-a", "notification", map[string]any{"desktop": "new"})
			},
			"new",
		)
	})

	t.Run("theme", func(t *testing.T) {
		env := setupAppConfigTestEnv(t)
		require.NoError(t, env.db.Create(&models.UserConfig{
			UserID: "user-a", Group: ThemeConfigGroup, Name: "navTheme", Value: "light",
		}).Error)
		svc := &Theme{}
		_, err := svc.UserResource(env.ctx, "user-a")
		require.NoError(t, err)
		assertUserConfigCacheHitRevalidatesRevision(
			t,
			env,
			userThemeCacheKey("user-a", 0),
			func(ctx *gin.Context) (string, error) {
				resource, readErr := svc.UserResource(ctx, "user-a")
				if readErr != nil {
					return "", readErr
				}
				if resource.NavTheme == nil {
					return "", fmt.Errorf("theme resource has nil navTheme")
				}
				return *resource.NavTheme, nil
			},
			func(ctx *gin.Context) error {
				revision := int64(0)
				_, patchErr := svc.PatchUserResource(ctx, "user-a", map[string]any{
					"navTheme": "realDark",
				}, revision)
				return patchErr
			},
			"realDark",
		)
	})
}

func TestUserConfigProfileAndGroupCachesAreVersionedOwnerIsolatedAndCacheEmptyGroups(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.UserConfig{
		{UserID: "user-a", Group: "notification", Name: "desktop", Value: "a-old"},
		{UserID: "user-b", Group: "notification", Name: "desktop", Value: "b-old"},
	}).Error)

	svc := &UserConfig{}
	profileA, err := svc.Profile(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "a-old", profileA["notification"]["desktop"])
	profileB, err := svc.Profile(env.ctx, "user-b")
	require.NoError(t, err)
	require.Equal(t, "b-old", profileB["notification"]["desktop"])
	groupA, err := svc.Group(env.ctx, "user-a", "notification")
	require.NoError(t, err)
	require.Equal(t, "a-old", groupA["desktop"])
	empty, err := svc.Group(env.ctx, "user-a", "empty")
	require.NoError(t, err)
	require.Empty(t, empty)

	profileKeyA := userConfigProfileCacheKey("user-a", 0)
	profileKeyB := userConfigProfileCacheKey("user-b", 0)
	groupKeyA := userConfigGroupCacheKey("user-a", "notification", 0)
	groupKeyB := userConfigGroupCacheKey("user-b", "notification", 0)
	emptyKeyA := userConfigGroupCacheKey("user-a", "empty", 0)
	require.NotEqual(t, profileKeyA, profileKeyB)
	require.NotEqual(t, groupKeyA, groupKeyB)
	for _, key := range []string{profileKeyA, profileKeyB, groupKeyA, emptyKeyA} {
		require.NotContains(t, key, "user-a")
		require.NotContains(t, key, "user-b")
		require.True(t, env.redis.Exists(key))
		require.Equal(t, userConfigCacheTTL, env.redis.TTL(key))
	}

	profilePayloadA, err := env.client.Get(env.ctx, profileKeyA).Bytes()
	require.NoError(t, err)
	var profileEnvelope userConfigProfileCacheEnvelope
	require.NoError(t, json.Unmarshal(profilePayloadA, &profileEnvelope))
	require.Equal(t, userConfigCacheVersion, profileEnvelope.Version)
	require.Equal(t, userConfigOwnerDigest("user-a"), profileEnvelope.OwnerDigest)
	require.Equal(t, int64(0), profileEnvelope.Revision)

	emptyPayload, err := env.client.Get(env.ctx, emptyKeyA).Bytes()
	require.NoError(t, err)
	var emptyEnvelope userConfigGroupCacheEnvelope
	require.NoError(t, json.Unmarshal(emptyPayload, &emptyEnvelope))
	require.NotNil(t, emptyEnvelope.Values, "an empty group is still a complete cacheable resource")
	require.Empty(t, emptyEnvelope.Values)

	// A payload copied under another owner's otherwise valid key must miss. The
	// owner digest in the envelope is defense in depth against poisoned Redis
	// entries and accidental key construction regressions.
	require.NoError(t, env.db.Model(&models.UserConfig{}).
		Where("user_id = ?", "user-b").Update("value", "b-new").Error)
	require.NoError(t, env.client.Set(env.ctx, profileKeyB, profilePayloadA, userConfigCacheTTL).Err())
	profileB, err = svc.Profile(env.ctx, "user-b")
	require.NoError(t, err)
	require.Equal(t, "b-new", profileB["notification"]["desktop"])

	groupPayloadA, err := env.client.Get(env.ctx, groupKeyA).Bytes()
	require.NoError(t, err)
	require.NoError(t, env.client.Set(env.ctx, groupKeyB, groupPayloadA, userConfigCacheTTL).Err())
	groupB, err := svc.Group(env.ctx, "user-b", "notification")
	require.NoError(t, err)
	require.Equal(t, "b-new", groupB["desktop"])
	otherKeyA := userConfigGroupCacheKey("user-a", "other", 0)
	require.NoError(t, env.client.Set(env.ctx, otherKeyA, groupPayloadA, userConfigCacheTTL).Err())
	other, err := svc.Group(env.ctx, "user-a", "other")
	require.NoError(t, err)
	require.Empty(t, other, "a payload for another group digest must not be accepted")

	// Cache hits still require the authoritative revision table, but not the
	// configuration value table itself.
	require.NoError(t, env.db.Migrator().DropTable(&models.UserConfig{}))
	profileA, err = svc.Profile(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "a-old", profileA["notification"]["desktop"])
	groupA, err = svc.Group(env.ctx, "user-a", "notification")
	require.NoError(t, err)
	require.Equal(t, "a-old", groupA["desktop"])
}

func TestUserThemeCacheIsRevisionKeyedVersionedAndOwnerDigested(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	revision0 := int64(0)
	resource, err := (&Theme{}).PatchUserResource(env.ctx, "user-a", map[string]any{
		"navTheme": "realDark",
	}, revision0)
	require.NoError(t, err)
	require.Equal(t, "1", resource.Meta.Revision)

	key := userThemeCacheKey("user-a", 1)
	require.True(t, env.redis.Exists(key))
	require.NotContains(t, key, "user-a")
	require.Equal(t, userConfigCacheTTL, env.redis.TTL(key))
	payload, err := env.client.Get(env.ctx, key).Bytes()
	require.NoError(t, err)
	var envelope userThemeCacheEnvelope
	require.NoError(t, json.Unmarshal(payload, &envelope))
	require.Equal(t, userConfigCacheVersion, envelope.Version)
	require.Equal(t, userConfigOwnerDigest("user-a"), envelope.OwnerDigest)
	require.Equal(t, int64(1), envelope.Revision)
	require.Equal(t, "realDark", *envelope.Resource.NavTheme)

	require.NoError(t, env.db.Migrator().DropTable(&models.UserConfig{}))
	cached, err := (&Theme{}).UserResource(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "1", cached.Meta.Revision)
	require.Equal(t, "realDark", *cached.NavTheme)
}

func TestGenericUserConfigMutationIsAtomicAndAdvancesProfileRevision(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Exec(`
		CREATE TRIGGER fail_user_config_z_insert
		BEFORE INSERT ON mss_boot_user_configs
		WHEN NEW.name = 'z'
		BEGIN
			SELECT RAISE(ABORT, 'forced grouped config write failure');
		END;
	`).Error)

	svc := &UserConfig{}
	err := svc.CreateOrUpdate(env.ctx, "user-a", "notification", map[string]any{
		"a": "first",
		"z": "second",
	})
	require.Error(t, err)
	var count int64
	require.NoError(t, env.db.Model(&models.UserConfig{}).Count(&count).Error)
	require.Zero(t, count, "the earlier sorted key must roll back with the later failure")
	require.NoError(t, env.db.Model(&models.ConfigRevision{}).Count(&count).Error)
	require.Zero(t, count, "the profile revision row must roll back with the values")

	require.NoError(t, env.db.Exec("DROP TRIGGER fail_user_config_z_insert").Error)
	require.NoError(t, svc.CreateOrUpdate(env.ctx, "user-a", "notification", map[string]any{
		"a": "first",
		"z": "second",
	}))
	profileRevision, err := readConfigRevision(env.db, userConfigProfileRevisionKey("user-a"))
	require.NoError(t, err)
	require.Equal(t, int64(1), profileRevision)
	themeRevision, err := readConfigRevision(env.db, userThemeRevisionKey("user-a"))
	require.NoError(t, err)
	require.Zero(t, themeRevision)

	profile, err := svc.Profile(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "first", profile["notification"]["a"])
	group, err := svc.Group(env.ctx, "user-a", "notification")
	require.NoError(t, err)
	require.Equal(t, "second", group["z"])
	profileKey1 := userConfigProfileCacheKey("user-a", 1)
	groupKey1 := userConfigGroupCacheKey("user-a", "notification", 1)
	require.True(t, env.redis.Exists(profileKey1))
	require.True(t, env.redis.Exists(groupKey1))

	require.NoError(t, svc.CreateOrUpdate(env.ctx, "user-a", "notification", map[string]any{"a": "updated"}))
	profileRevision, err = readConfigRevision(env.db, userConfigProfileRevisionKey("user-a"))
	require.NoError(t, err)
	require.Equal(t, int64(2), profileRevision)
	require.False(t, env.redis.Exists(profileKey1))
	require.False(t, env.redis.Exists(groupKey1))
	group, err = svc.Group(env.ctx, "user-a", "notification")
	require.NoError(t, err)
	require.Equal(t, "updated", group["a"])
}

func TestLegacyModelUserConfigSetterAdvancesRevisionsAndBypassesCachedSnapshots(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.UserConfig{
		{UserID: "user-a", Group: "notification", Name: "desktop", Value: "old"},
		{UserID: "user-a", Group: ThemeConfigGroup, Name: "navTheme", Value: "light"},
	}).Error)

	userConfig := &UserConfig{}
	profile, err := userConfig.Profile(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "old", profile["notification"]["desktop"])
	group, err := userConfig.Group(env.ctx, "user-a", "notification")
	require.NoError(t, err)
	require.Equal(t, "old", group["desktop"])
	theme, err := (&Theme{}).UserResource(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "light", *theme.NavTheme)
	require.True(t, env.redis.Exists(userConfigProfileCacheKey("user-a", 0)))
	require.True(t, env.redis.Exists(userConfigGroupCacheKey("user-a", "notification", 0)))
	require.True(t, env.redis.Exists(userThemeCacheKey("user-a", 0)))

	legacy := &models.UserConfig{}
	require.NoError(t, legacy.SetUserConfig(env.ctx, "user-a", "notification.desktop", "new"))
	requireUserConfigRevisions(t, env.db, "user-a", 1, 0)
	// The compatibility setter does not need to know cache key formats. Old
	// entries can remain until TTL because the database revision makes them
	// unreachable immediately.
	require.True(t, env.redis.Exists(userConfigProfileCacheKey("user-a", 0)))
	require.True(t, env.redis.Exists(userConfigGroupCacheKey("user-a", "notification", 0)))

	profile, err = userConfig.Profile(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "new", profile["notification"]["desktop"])
	group, err = userConfig.Group(env.ctx, "user-a", "notification")
	require.NoError(t, err)
	require.Equal(t, "new", group["desktop"])

	require.NoError(t, legacy.SetUserConfig(env.ctx, "user-a", "theme.navTheme", "realDark"))
	requireUserConfigRevisions(t, env.db, "user-a", 2, 1)
	require.True(t, env.redis.Exists(userConfigProfileCacheKey("user-a", 1)))
	require.True(t, env.redis.Exists(userThemeCacheKey("user-a", 0)))

	profile, err = userConfig.Profile(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "realDark", profile[ThemeConfigGroup]["navTheme"])
	theme, err = (&Theme{}).UserResource(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "1", theme.Meta.Revision)
	require.Equal(t, "realDark", *theme.NavTheme)
}

func TestUserThemePatchAndResetAdvanceProfileAndThemeRevisionsTogether(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	svc := &UserConfig{}
	_, err := svc.Profile(env.ctx, "user-a")
	require.NoError(t, err)
	_, err = (&Theme{}).UserResource(env.ctx, "user-a")
	require.NoError(t, err)
	require.True(t, env.redis.Exists(userConfigProfileCacheKey("user-a", 0)))
	require.True(t, env.redis.Exists(userThemeCacheKey("user-a", 0)))

	revision0 := int64(0)
	resource, err := (&Theme{}).PatchUserResource(env.ctx, "user-a", map[string]any{
		"colorWeak": false,
	}, revision0)
	require.NoError(t, err)
	require.Equal(t, "1", resource.Meta.Revision)
	require.False(t, env.redis.Exists(userConfigProfileCacheKey("user-a", 0)))
	require.False(t, env.redis.Exists(userThemeCacheKey("user-a", 0)))
	require.True(t, env.redis.Exists(userThemeCacheKey("user-a", 1)))
	requireUserConfigRevisions(t, env.db, "user-a", 1, 1)

	profile, err := svc.Profile(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, false, profile[ThemeConfigGroup]["colorWeak"])
	require.True(t, env.redis.Exists(userConfigProfileCacheKey("user-a", 1)))

	revision1 := int64(1)
	resource, err = (&Theme{}).ResetUserResource(env.ctx, "user-a", revision1)
	require.NoError(t, err)
	require.Equal(t, "2", resource.Meta.Revision)
	require.False(t, env.redis.Exists(userConfigProfileCacheKey("user-a", 1)))
	require.False(t, env.redis.Exists(userThemeCacheKey("user-a", 1)))
	require.True(t, env.redis.Exists(userThemeCacheKey("user-a", 2)))
	requireUserConfigRevisions(t, env.db, "user-a", 2, 2)
}

func TestUserConfigRevisionKeysDoNotAcceptStalePayloads(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	svc := &UserConfig{}
	require.NoError(t, svc.CreateOrUpdate(env.ctx, "user-a", "notification", map[string]any{
		"desktop": "old",
	}))
	_, err := svc.Profile(env.ctx, "user-a")
	require.NoError(t, err)
	_, err = svc.Group(env.ctx, "user-a", "notification")
	require.NoError(t, err)
	profileKey1 := userConfigProfileCacheKey("user-a", 1)
	groupKey1 := userConfigGroupCacheKey("user-a", "notification", 1)
	profilePayload1, err := env.client.Get(env.ctx, profileKey1).Bytes()
	require.NoError(t, err)
	groupPayload1, err := env.client.Get(env.ctx, groupKey1).Bytes()
	require.NoError(t, err)

	require.NoError(t, svc.CreateOrUpdate(env.ctx, "user-a", "notification", map[string]any{
		"desktop": "new",
	}))
	// Model failed cleanup by putting the old revision payloads back. Reads first
	// consult the database revision and therefore never even address these keys.
	require.NoError(t, env.client.Set(env.ctx, profileKey1, profilePayload1, userConfigCacheTTL).Err())
	require.NoError(t, env.client.Set(env.ctx, groupKey1, groupPayload1, userConfigCacheTTL).Err())
	profile, err := svc.Profile(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "new", profile["notification"]["desktop"])
	group, err := svc.Group(env.ctx, "user-a", "notification")
	require.NoError(t, err)
	require.Equal(t, "new", group["desktop"])

	revision0 := int64(0)
	theme1, err := (&Theme{}).PatchUserResource(env.ctx, "user-b", map[string]any{
		"layout": "side",
	}, revision0)
	require.NoError(t, err)
	require.Equal(t, "1", theme1.Meta.Revision)
	themeKey1 := userThemeCacheKey("user-b", 1)
	themePayload1, err := env.client.Get(env.ctx, themeKey1).Bytes()
	require.NoError(t, err)
	revision1 := int64(1)
	_, err = (&Theme{}).PatchUserResource(env.ctx, "user-b", map[string]any{
		"layout": "mix",
	}, revision1)
	require.NoError(t, err)
	require.NoError(t, env.client.Set(env.ctx, themeKey1, themePayload1, userConfigCacheTTL).Err())
	require.NoError(t, env.client.Del(env.ctx, userThemeCacheKey("user-b", 2)).Err())
	current, err := (&Theme{}).UserResource(env.ctx, "user-b")
	require.NoError(t, err)
	require.Equal(t, "2", current.Meta.Revision)
	require.Equal(t, "mix", *current.Layout)
}

func TestGenericUserConfigRejectsCaseInsensitiveOwnerAndKeyAliases(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Migrator().DropTable(&models.UserConfig{}))
	require.NoError(t, env.db.Exec(`
		CREATE TABLE mss_boot_user_configs (
			id varchar(64) PRIMARY KEY,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime,
			user_id varchar(64) COLLATE NOCASE NOT NULL,
			name varchar(128) COLLATE NOCASE NOT NULL,
			"group" varchar(128) COLLATE NOCASE NOT NULL,
			value varchar(255) NOT NULL DEFAULT ''
		)
	`).Error)
	require.NoError(t, env.db.Exec(`
		CREATE UNIQUE INDEX idx_user_config_nocase_test
		ON mss_boot_user_configs(user_id, name, "group")
	`).Error)
	legacy := &models.UserConfig{
		UserID: "User-A", Group: "Notification", Name: "Desktop", Value: "old",
	}
	require.NoError(t, env.db.Create(legacy).Error)

	svc := &UserConfig{}
	err := svc.CreateOrUpdate(env.ctx, "user-a", "notification", map[string]any{"desktop": "new"})
	require.ErrorIs(t, err, ErrUserConfigKeyCaseMismatch)
	require.NotContains(t, err.Error(), "user-a")
	require.NotContains(t, err.Error(), "User-A")
	_, err = svc.Group(env.ctx, "user-a", "notification")
	require.ErrorIs(t, err, ErrUserConfigKeyCaseMismatch)
	require.NotContains(t, err.Error(), "user-a")
	require.NotContains(t, err.Error(), "User-A")
	err = (&models.UserConfig{}).SetUserConfig(env.ctx, "user-a", "notification.desktop", "model-new")
	require.ErrorIs(t, err, models.ErrUserConfigIdentityMismatch)
	require.NotContains(t, err.Error(), "user-a")
	require.NotContains(t, err.Error(), "User-A")

	var persisted models.UserConfig
	require.NoError(t, env.db.First(&persisted, "id = ?", legacy.ID).Error)
	require.Equal(t, "User-A", persisted.UserID)
	require.Equal(t, "Notification", persisted.Group)
	require.Equal(t, "Desktop", persisted.Name)
	require.Equal(t, "old", persisted.Value)
	var revisionCount int64
	require.NoError(t, env.db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
	require.Zero(t, revisionCount, "a rejected alias must roll back its provisional revision row")

	require.NoError(t, svc.CreateOrUpdate(env.ctx, "User-A", "Notification", map[string]any{"Desktop": "exact"}))
	require.NoError(t, env.db.First(&persisted, "id = ?", legacy.ID).Error)
	require.Equal(t, "exact", persisted.Value)
}

func TestConfigRevisionRejectsCaseInsensitiveOwnerAlias(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Migrator().DropTable(&models.ConfigRevision{}))
	require.NoError(t, env.db.Exec(`
		CREATE TABLE mss_boot_config_revisions (
			scope varchar(16) COLLATE NOCASE NOT NULL,
			owner_id varchar(64) COLLATE NOCASE NOT NULL DEFAULT '',
			resource varchar(32) COLLATE NOCASE NOT NULL,
			revision bigint NOT NULL DEFAULT 0,
			updated_at datetime NOT NULL,
			PRIMARY KEY (scope, owner_id, resource)
		)
	`).Error)
	require.NoError(t, env.db.Create(&models.ConfigRevision{
		Scope: ThemeScopeUser, OwnerID: "User-A", Resource: configRevisionResourceUserProfile,
		Revision: 9,
	}).Error)

	_, err := readConfigRevision(env.db, userConfigProfileRevisionKey("user-a"))
	require.ErrorIs(t, err, errConfigRevisionKeyMismatch)
	require.NotContains(t, err.Error(), "user-a")
	require.NotContains(t, err.Error(), "User-A")
	err = env.db.Transaction(func(tx *gorm.DB) error {
		_, lockErr := lockConfigRevision(tx, userConfigProfileRevisionKey("user-a"))
		return lockErr
	})
	require.ErrorIs(t, err, errConfigRevisionKeyMismatch)
	require.NotContains(t, err.Error(), "user-a")
	require.NotContains(t, err.Error(), "User-A")
	var persisted models.ConfigRevision
	require.NoError(t, env.db.First(&persisted).Error)
	require.Equal(t, "User-A", persisted.OwnerID)
	require.Equal(t, int64(9), persisted.Revision)
}

func TestUserConfigCachesFailOpenToAuthoritativeDatabase(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.UserConfig{
		{UserID: "user-a", Group: "notification", Name: "desktop", Value: "enabled"},
		{UserID: "user-a", Group: ThemeConfigGroup, Name: "navTheme", Value: "realDark"},
	}).Error)
	env.redis.Close()

	started := time.Now()
	profile, err := (&UserConfig{}).Profile(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "enabled", profile["notification"]["desktop"])
	group, err := (&UserConfig{}).Group(env.ctx, "user-a", "notification")
	require.NoError(t, err)
	require.Equal(t, "enabled", group["desktop"])
	theme, err := (&Theme{}).UserResource(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "realDark", *theme.NavTheme)
	require.Less(t, time.Since(started), 3*time.Second, "cache failure must be bounded and fall back")
}

func TestUserConfigAuthoritativeReadsBypassLaggingReplica(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "source.db")
	replicaPath := filepath.Join(directory, "replica.db")
	seedUserConfigDatabase(t, sourcePath, "source", 2, 5, "realDark")
	seedUserConfigDatabase(t, replicaPath, "replica", 1, 4, "light")

	db, err := gorm.Open(sqlite.Open(sourcePath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Use(dbresolver.Register(dbresolver.Config{
		Replicas: []gorm.Dialector{sqlite.Open(replicaPath)},
	})))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	previousTenant := center.GetTenant()
	previousCache := center.GetCache()
	center.SetTenant(&appConfigTestTenant{db: db})
	center.SetCache(nil)
	t.Cleanup(func() {
		center.SetTenant(previousTenant)
		center.SetCache(previousCache)
	})
	ctx, _ := gin.CreateTestContext(nil)

	profile, err := (&UserConfig{}).Profile(ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "source", profile["notification"]["desktop"])
	themeMeta := requireThemeProfileMeta(t, profile[ThemeConfigGroup]["_meta"])
	require.Equal(t, "5", themeMeta["revision"])
	require.Equal(t, "realDark", profile[ThemeConfigGroup]["navTheme"])
	group, err := (&UserConfig{}).Group(ctx, "user-a", "notification")
	require.NoError(t, err)
	require.Equal(t, "source", group["desktop"])
	theme, err := (&Theme{}).UserResource(ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "5", theme.Meta.Revision)
	require.Equal(t, "realDark", *theme.NavTheme)
	application, err := (&Theme{}).ApplicationResource(ctx)
	require.NoError(t, err)
	require.Equal(t, "8", application.Meta.Revision)
	require.Equal(t, "mix", *application.Layout)
	applicationGroup, err := (&AppConfig{}).Group(ctx, ThemeConfigGroup)
	require.NoError(t, err)
	require.Equal(t, "8", requireThemeProfileMeta(t, applicationGroup["_meta"])["revision"])
	require.Equal(t, "mix", applicationGroup["layout"])
	require.NotContains(t, applicationGroup, "pwa")
}

func requireUserConfigRevisions(t *testing.T, db *gorm.DB, userID string, profile, theme int64) {
	t.Helper()
	profileRevision, err := readConfigRevision(db, userConfigProfileRevisionKey(userID))
	require.NoError(t, err)
	themeRevision, err := readConfigRevision(db, userThemeRevisionKey(userID))
	require.NoError(t, err)
	require.Equal(t, profile, profileRevision)
	require.Equal(t, theme, themeRevision)
}

func seedUserConfigDatabase(
	t *testing.T,
	path, notification string,
	profileRevision, themeRevision int64,
	navTheme string,
) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AppConfig{}, &models.UserConfig{}, &models.ConfigRevision{}))
	require.NoError(t, db.Create([]*models.UserConfig{
		{UserID: "user-a", Group: "notification", Name: "desktop", Value: notification},
		{UserID: "user-a", Group: ThemeConfigGroup, Name: "navTheme", Value: navTheme},
	}).Error)
	applicationLayout := "side"
	legacyPWA := "false"
	if notification == "source" {
		applicationLayout = "mix"
		legacyPWA = "true"
	}
	require.NoError(t, db.Create([]*models.AppConfig{
		{Group: ThemeConfigGroup, Name: "layout", Value: applicationLayout},
		{Group: ThemeConfigGroup, Name: "pwa", Value: legacyPWA},
	}).Error)
	require.NoError(t, db.Create([]*models.ConfigRevision{
		{Scope: ThemeScopeUser, OwnerID: "user-a", Resource: configRevisionResourceUserProfile, Revision: profileRevision},
		{Scope: ThemeScopeUser, OwnerID: "user-a", Resource: configRevisionResourceTheme, Revision: themeRevision},
		{Scope: ThemeScopeApplication, Resource: configRevisionResourceTheme, Revision: themeRevision + 3},
	}).Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

func TestUserConfigCacheKeysNeverContainRawOwnerOrGroup(t *testing.T) {
	owner := "personally-identifying-user"
	group := "sensitive-preference-group"
	for _, key := range []string{
		userConfigProfileCacheKey(owner, 7),
		userThemeCacheKey(owner, 7),
		userConfigGroupCacheKey(owner, group, 7),
	} {
		require.False(t, strings.Contains(key, owner))
		require.False(t, strings.Contains(key, group))
	}
}
