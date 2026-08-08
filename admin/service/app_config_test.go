package service

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/dbresolver"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	cacheconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
)

type appConfigTestTenant struct {
	db *gorm.DB
}

func (e *appConfigTestTenant) Scope(_ *gin.Context, _ schema.Tabler) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB { return db }
}

func (e *appConfigTestTenant) GetTenant(_ *gin.Context) (center.TenantImp, error) {
	return e, nil
}

func (e *appConfigTestTenant) GetDB(_ *gin.Context, _ schema.Tabler) *gorm.DB {
	return e.db
}

func (*appConfigTestTenant) GetID() any       { return "test" }
func (*appConfigTestTenant) GetDefault() bool { return true }

type appConfigTestEnv struct {
	db     *gorm.DB
	redis  *miniredis.Miniredis
	client *redis.Client
	ctx    *gin.Context
}

func requireThemeProfileMeta(t *testing.T, raw any) map[string]any {
	t.Helper()
	switch value := raw.(type) {
	case gin.H:
		return map[string]any(value)
	case map[string]any:
		return value
	default:
		t.Fatalf("theme profile metadata has unexpected type %T", raw)
		return nil
	}
}

func setupAppConfigTestEnv(t *testing.T) appConfigTestEnv {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.AppConfig{}, &models.UserConfig{}, &models.ConfigRevision{}))
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX idx_app_config_test ON mss_boot_app_configs(name, "group")`,
	).Error)
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX idx_user_config_test ON mss_boot_user_configs(user_id, name, "group")`,
	).Error)

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cache, err := cacheconfig.NewRedis(client, nil)
	require.NoError(t, err)

	previousTenant := center.GetTenant()
	previousCache := center.GetCache()
	previousAppConfig := center.GetAppConfig()
	center.SetTenant(&appConfigTestTenant{db: db})
	center.SetCache(cache)
	center.SetAppConfig(&models.AppConfig{})
	t.Cleanup(func() {
		center.SetTenant(previousTenant)
		center.SetCache(previousCache)
		center.SetAppConfig(previousAppConfig)
		_ = cache.Close()
		_ = sqlDB.Close()
	})

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return appConfigTestEnv{db: db, redis: mr, client: client, ctx: ctx}
}

func TestAppConfigProfileNeverBroadensForAuthenticatedCallers(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.AppConfig{
		{Group: "base", Name: "websiteName", Value: "MSS", Auth: false},
		{Group: "security", Name: "githubEnabled", Value: "true", Auth: false},
		// The allowlist is authoritative, including for rows misclassified by an
		// older value-based writer in either direction.
		{Group: "theme", Name: "navTheme", Value: "realDark", Auth: true},
		{Group: "security", Name: "githubClientId", Value: "must-stay-private", Auth: false},
		{Group: "security", Name: "maintenanceMessage", Value: "staff only", Auth: true},
		// Simulate a row written by the old value-based classification bug.
		{Group: "security", Name: "githubClientSecret", Value: "leaked-before-fix", Auth: false},
		{Group: "base", Name: "harmlessUnknown", Value: "private by default", Auth: false},
	}).Error)

	svc := &AppConfig{}
	for _, order := range [][]bool{{false, true}, {true, false}} {
		env.redis.FlushAll()
		// Poison the legacy shared cache. The public projection must never read it.
		env.redis.SAdd("app-configs", "security")
		env.redis.HSet("app-configs:security", "githubClientSecret", "legacy-cache-secret")

		for _, authenticated := range order {
			profile, err := svc.Profile(env.ctx, authenticated)
			require.NoError(t, err)
			require.Equal(t, "MSS", profile["base"]["websiteName"])
			require.Equal(t, true, profile["security"]["githubEnabled"])
			require.Equal(t, "realDark", profile["theme"]["navTheme"])
			require.NotContains(t, profile["security"], "githubClientId")
			require.NotContains(t, profile["security"], "maintenanceMessage")
			require.NotContains(t, profile["security"], "githubClientSecret")
			require.NotContains(t, profile["base"], "harmlessUnknown")
		}

		cached, err := env.redis.Get(publicProfileCacheKey(0))
		require.NoError(t, err)
		require.NotContains(t, cached, "staff only")
		require.NotContains(t, cached, "leaked-before-fix")
	}
}

func TestAppConfigProfileFiltersSensitiveKeysFromPoisonedPublicCache(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	env.redis.Set(publicProfileCacheKey(0), `{
		"v":2,
		"profileRevision":0,
		"themeRevision":0,
		"profile":{
			"base":{"websiteName":"MSS","unknown":"poisoned-unknown"},
			"security":{"githubEnabled":true,"githubClientId":"client-id","githubClientSecret":"poisoned"},
			"storage":{"s3_access_key_id":"poisoned-too"}
		}
	}`)

	profile, err := (&AppConfig{}).Profile(env.ctx, true)
	require.NoError(t, err)
	require.Equal(t, "MSS", profile["base"]["websiteName"])
	require.NotContains(t, profile["base"], "unknown")
	require.Equal(t, true, profile["security"]["githubEnabled"])
	require.NotContains(t, profile["security"], "githubClientId")
	require.NotContains(t, profile["security"], "githubClientSecret")
	require.NotContains(t, profile, "storage")
}

func TestAppConfigProfileCachesVersionedEnvelopeWithTTLAndThemeMetadata(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.AppConfig{
		{Group: ThemeConfigGroup, Name: "fixedHeader", Value: "false", Auth: false},
		{Group: ThemeConfigGroup, Name: "pwa", Value: "true", Auth: false},
	}).Error)

	profile, err := (&AppConfig{}).Profile(env.ctx, false)
	require.NoError(t, err)
	require.Equal(t, false, profile[ThemeConfigGroup]["fixedHeader"])
	require.Equal(t, true, profile[ThemeConfigGroup]["pwa"])
	require.Equal(t, map[string]any{
		"v": themeResourceVersion, "scope": ThemeScopeApplication, "revision": "0",
	}, requireThemeProfileMeta(t, profile[ThemeConfigGroup]["_meta"]))
	require.Equal(t, appConfigPublicProfileCacheTTL, env.redis.TTL(publicProfileCacheKey(0)))

	// Metadata is intentionally projected out of the stored profile and then
	// reconstructed from the authoritative envelope revision on a cache hit.
	cached, err := env.redis.Get(publicProfileCacheKey(0))
	require.NoError(t, err)
	require.NotContains(t, cached, `"_meta"`)
	require.Contains(t, cached, `"pwa":true`)
	profile, err = (&AppConfig{}).Profile(env.ctx, false)
	require.NoError(t, err)
	meta := requireThemeProfileMeta(t, profile[ThemeConfigGroup]["_meta"])
	require.Equal(t, "0", meta["revision"])
	require.Equal(t, true, profile[ThemeConfigGroup]["pwa"], "cache hit must match the database miss projection")
}

func TestAppConfigProfileStableReadUsesWriter(t *testing.T) {
	dir := t.TempDir()
	writerPath := filepath.Join(dir, "writer.db")
	replicaPath := filepath.Join(dir, "replica.db")
	writer, err := gorm.Open(sqlite.Open(writerPath), &gorm.Config{})
	require.NoError(t, err)
	replica, err := gorm.Open(sqlite.Open(replicaPath), &gorm.Config{})
	require.NoError(t, err)
	for _, db := range []*gorm.DB{writer, replica} {
		require.NoError(t, db.AutoMigrate(&models.AppConfig{}, &models.ConfigRevision{}))
	}

	profileKey := applicationPublicProfileRevisionKey()
	themeKey := applicationThemeRevisionKey()
	require.NoError(t, writer.Create(&models.AppConfig{
		Group: "base", Name: "websiteName", Value: "writer-value", Auth: false,
	}).Error)
	require.NoError(t, replica.Create(&models.AppConfig{
		Group: "base", Name: "websiteName", Value: "replica-value", Auth: false,
	}).Error)
	require.NoError(t, writer.Create([]*models.ConfigRevision{
		{Scope: profileKey.scope, OwnerID: profileKey.ownerID, Resource: profileKey.resource, Revision: 7},
		{Scope: themeKey.scope, OwnerID: themeKey.ownerID, Resource: themeKey.resource, Revision: 5},
	}).Error)
	require.NoError(t, replica.Create([]*models.ConfigRevision{
		{Scope: profileKey.scope, OwnerID: profileKey.ownerID, Resource: profileKey.resource, Revision: 6},
		{Scope: themeKey.scope, OwnerID: themeKey.ownerID, Resource: themeKey.resource, Revision: 4},
	}).Error)
	require.NoError(t, writer.Use(dbresolver.Register(dbresolver.Config{
		Replicas: []gorm.Dialector{sqlite.Open(replicaPath)},
	})))

	var defaultRead models.AppConfig
	require.NoError(t, writer.Where(&models.AppConfig{Group: "base", Name: "websiteName"}).First(&defaultRead).Error)
	require.Equal(t, "replica-value", defaultRead.Value, "test setup must route ordinary reads to the replica")

	previousTenant := center.GetTenant()
	previousCache := center.GetCache()
	center.SetTenant(&appConfigTestTenant{db: writer})
	center.SetCache(nil)
	t.Cleanup(func() {
		center.SetTenant(previousTenant)
		center.SetCache(previousCache)
		if sqlDB, dbErr := writer.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		if sqlDB, dbErr := replica.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	profile, err := (&AppConfig{}).Profile(ctx, false)
	require.NoError(t, err)
	require.Equal(t, "writer-value", profile["base"]["websiteName"])
	require.Equal(t, "5", requireThemeProfileMeta(t, profile[ThemeConfigGroup]["_meta"])["revision"])
}

type appConfigCacheDeadlineHook struct{}

func (appConfigCacheDeadlineHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (appConfigCacheDeadlineHook) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, _ redis.Cmder) error {
		<-ctx.Done()
		return ctx.Err()
	}
}

func (appConfigCacheDeadlineHook) ProcessPipelineHook(redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, _ []redis.Cmder) error {
		<-ctx.Done()
		return ctx.Err()
	}
}

func TestAppConfigProfileCacheOperationsHaveBoundedLatency(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create(&models.AppConfig{
		Group: "base", Name: "websiteName", Value: "database-value", Auth: false,
	}).Error)
	env.client.AddHook(appConfigCacheDeadlineHook{})

	started := time.Now()
	profile, err := (&AppConfig{}).Profile(env.ctx, false)
	elapsed := time.Since(started)
	require.NoError(t, err)
	require.Equal(t, "database-value", profile["base"]["websiteName"])
	require.Less(t, elapsed, time.Second, "optional public-profile cache operations must be bounded")
}

func TestAppConfigCreateOrUpdateClassifiesByKeyAndInvalidatesPublicCache(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.AppConfig{
		// Simulate both directions of the old value-based classification bug.
		{Group: "base", Name: "websiteName", Value: "old-token-shaped-value", Auth: true},
		{Group: "security", Name: "githubEnabled", Value: "false", Auth: true},
		{Group: "security", Name: "githubClientSecret", Value: "old-ordinary-value", Auth: false},
	}).Error)
	env.redis.Set(publicProfileCacheKey(0), `{"v":2,"profileRevision":0,"themeRevision":0,"profile":{"base":{"websiteName":"old"}}}`)
	env.redis.HSet(legacyAppConfigCacheHash, "base:websiteName", "stale-legacy-value")
	env.redis.HSet(appConfigEntryCacheHash, "base:websiteName", "stale-versioned-value")

	svc := &AppConfig{}
	err := svc.CreateOrUpdate(env.ctx, "base", map[string]any{
		"websiteName": "value-with-token-and-secret-words",
	})
	require.NoError(t, err)
	err = svc.CreateOrUpdate(env.ctx, "security", map[string]any{
		"githubEnabled":      true,
		"githubClientId":     "ordinary-looking-public-id",
		"githubClientSecret": "ordinary-looking-value",
		"api_token":          "ordinary-looking-value",
	})
	require.NoError(t, err)
	require.False(t, env.redis.Exists(publicProfileCacheKey(0)))
	legacyExists, err := env.client.HExists(env.ctx, legacyAppConfigCacheHash, "base:websiteName").Result()
	require.NoError(t, err)
	require.False(t, legacyExists, "committed generic writes must invalidate the legacy field")
	currentExists, err := env.client.HExists(env.ctx, appConfigEntryCacheHash, "base:websiteName").Result()
	require.NoError(t, err)
	require.False(t, currentExists, "committed generic writes must invalidate the current field")
	legacyValue, ok := center.GetAppConfig().GetAppConfig(env.ctx, "base:websiteName")
	require.True(t, ok)
	require.Equal(t, "value-with-token-and-secret-words", legacyValue)

	var records []models.AppConfig
	require.NoError(t, env.db.Find(&records).Error)
	byName := make(map[string]models.AppConfig, len(records))
	for _, record := range records {
		byName[record.Name] = record
	}
	require.False(t, byName["websiteName"].Auth, "the value must not drive sensitivity")
	require.False(t, byName["githubEnabled"].Auth)
	require.True(t, byName["githubClientId"].Auth, "unknown keys are private by default")
	require.True(t, byName["githubClientSecret"].Auth)
	require.True(t, byName["api_token"].Auth)

	profile, err := svc.Profile(env.ctx, false)
	require.NoError(t, err)
	require.Equal(t, "value-with-token-and-secret-words", profile["base"]["websiteName"])
	require.Equal(t, true, profile["security"]["githubEnabled"])
	require.NotContains(t, profile["security"], "githubClientId")
	require.NotContains(t, profile["security"], "githubClientSecret")
	require.NotContains(t, profile["security"], "api_token")

	// The authorized group path remains the explicit privileged projection.
	group, err := svc.GroupWithSensitiveValues(env.ctx, "security", true)
	require.NoError(t, err)
	require.Equal(t, "ordinary-looking-public-id", group["githubClientId"])
	require.Equal(t, "ordinary-looking-value", group["githubClientSecret"])
}

type blockingFirstLegacyInvalidationHook struct {
	field   string
	blocked chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (h *blockingFirstLegacyInvalidationHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *blockingFirstLegacyInvalidationHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		args := cmd.Args()
		matches := cmd.Name() == "hdel" && len(args) > 2 && args[1] == legacyAppConfigCacheHash
		if matches {
			matches = false
			for _, arg := range args[2:] {
				if arg == h.field {
					matches = true
					break
				}
			}
		}
		if matches && h.calls.Add(1) == 1 {
			close(h.blocked)
			<-h.release
		}
		return next(ctx, cmd)
	}
}

func (h *blockingFirstLegacyInvalidationHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestLegacyAppConfigInvalidationCannotRegressWhenPostCommitEffectsReorder(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create(&models.AppConfig{
		Group: "base", Name: "websiteName", Value: "old", Auth: false,
	}).Error)
	env.redis.HSet(legacyAppConfigCacheHash, "base:websiteName", "old")
	env.redis.HSet(appConfigEntryCacheHash, "base:websiteName", "old")

	hook := &blockingFirstLegacyInvalidationHook{
		field:   "base:websiteName",
		blocked: make(chan struct{}),
		release: make(chan struct{}),
	}
	env.client.AddHook(hook)
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(hook.release) }) }
	defer release()

	firstResult := make(chan error, 1)
	firstCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	go func() {
		firstResult <- (&AppConfig{}).CreateOrUpdate(firstCtx, "base", map[string]any{
			"websiteName": "first",
		})
	}()

	select {
	case <-hook.blocked:
	case <-time.After(2 * time.Second):
		t.Fatal("first committed write did not reach legacy invalidation")
	}

	secondCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, (&AppConfig{}).CreateOrUpdate(secondCtx, "base", map[string]any{
		"websiteName": "second",
	}))
	release()
	select {
	case err := <-firstResult:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("first write did not finish after releasing invalidation")
	}

	legacyExists, err := env.client.HExists(env.ctx, legacyAppConfigCacheHash, "base:websiteName").Result()
	require.NoError(t, err)
	require.False(t, legacyExists, "reordered post-commit invalidations must converge to an empty field")
	currentExists, err := env.client.HExists(env.ctx, appConfigEntryCacheHash, "base:websiteName").Result()
	require.NoError(t, err)
	require.False(t, currentExists, "reordered invalidations must also clear the current field")
	value, ok := center.GetAppConfig().GetAppConfig(env.ctx, "base:websiteName")
	require.True(t, ok)
	require.Equal(t, "second", value)
}

func TestPublicAppConfigKeyAllowlistIsExact(t *testing.T) {
	require.Len(t, publicAppConfigKeys, 18)
	for _, key := range publicAppConfigKeys {
		t.Run(key.Group+"/"+key.Name, func(t *testing.T) {
			require.True(t, isPublicAppConfigKey(key.Group, key.Name))
		})
	}
	for _, key := range []appConfigPublicKey{
		{Group: "security", Name: "githubClientId"},
		{Group: "security", Name: "githubClientSecret"},
		{Group: "email", Name: "host"},
		{Group: "storage", Name: "region"},
		{Group: "base", Name: "harmlessUnknown"},
		{Group: "base", Name: "WebsiteName"},
		{Group: "base", Name: " websiteName"},
	} {
		t.Run("private/"+key.Group+"/"+key.Name, func(t *testing.T) {
			require.False(t, isPublicAppConfigKey(key.Group, key.Name))
		})
	}
}

type blockingPublicProfileCacheHook struct {
	blocked   chan struct{}
	release   chan struct{}
	blockOnce sync.Once
}

type blockingPublicProfileReadHook struct {
	key       string
	payload   string
	blocked   chan struct{}
	release   chan struct{}
	blockOnce sync.Once
}

func (h *blockingPublicProfileReadHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *blockingPublicProfileReadHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		args := cmd.Args()
		if cmd.Name() == "get" && len(args) > 1 && args[1] == h.key {
			h.blockOnce.Do(func() {
				close(h.blocked)
				<-h.release
			})
			if stringCmd, ok := cmd.(*redis.StringCmd); ok {
				stringCmd.SetVal(h.payload)
				return nil
			}
		}
		return next(ctx, cmd)
	}
}

func (h *blockingPublicProfileReadHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestAppConfigProfileRechecksRevisionAfterCacheHit(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create(&models.AppConfig{
		Group: "base", Name: "websiteName", Value: "old", Auth: false,
	}).Error)
	initial, err := (&AppConfig{}).Profile(env.ctx, false)
	require.NoError(t, err)
	require.Equal(t, "old", initial["base"]["websiteName"])
	payload, err := env.redis.Get(publicProfileCacheKey(0))
	require.NoError(t, err)

	hook := &blockingPublicProfileReadHook{
		key:     publicProfileCacheKey(0),
		payload: payload,
		blocked: make(chan struct{}),
		release: make(chan struct{}),
	}
	env.client.AddHook(hook)
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(hook.release) }) }
	defer release()

	type profileResult struct {
		profile map[string]gin.H
		err     error
	}
	result := make(chan profileResult, 1)
	profileCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	go func() {
		profile, profileErr := (&AppConfig{}).Profile(profileCtx, false)
		result <- profileResult{profile: profile, err: profileErr}
	}()

	select {
	case <-hook.blocked:
	case <-time.After(2 * time.Second):
		t.Fatal("profile read did not reach the cached revision")
	}
	require.NoError(t, env.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.AppConfig{}).
			Where("`group` = ? AND name = ?", "base", "websiteName").
			Update("value", "new").Error; err != nil {
			return err
		}
		key := applicationPublicProfileRevisionKey()
		return tx.Create(&models.ConfigRevision{
			Scope: key.scope, OwnerID: key.ownerID, Resource: key.resource, Revision: 1,
		}).Error
	}))
	release()

	select {
	case got := <-result:
		require.NoError(t, got.err)
		require.Equal(t, "new", got.profile["base"]["websiteName"])
	case <-time.After(2 * time.Second):
		t.Fatal("profile read did not finish after releasing the cache hit")
	}
}

func (h *blockingPublicProfileCacheHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *blockingPublicProfileCacheHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		args := cmd.Args()
		if cmd.Name() == "set" && len(args) > 1 {
			if key, ok := args[1].(string); ok && key == publicProfileCacheKey(0) {
				h.blockOnce.Do(func() {
					close(h.blocked)
					<-h.release
				})
			}
		}
		return next(ctx, cmd)
	}
}

func (h *blockingPublicProfileCacheHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestAppConfigProfileVersionedCacheCannotPoisonNewRevisionAfterConcurrentUpdate(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create(&models.AppConfig{
		Group: "base", Name: "websiteName", Value: "old", Auth: false,
	}).Error)

	hook := &blockingPublicProfileCacheHook{
		blocked: make(chan struct{}),
		release: make(chan struct{}),
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(hook.release) }) }
	defer release()
	env.client.AddHook(hook)

	type profileResult struct {
		profile map[string]gin.H
		err     error
	}
	result := make(chan profileResult, 1)
	profileCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	go func() {
		profile, err := (&AppConfig{}).Profile(profileCtx, false)
		result <- profileResult{profile: profile, err: err}
	}()

	select {
	case <-hook.blocked:
	case <-time.After(2 * time.Second):
		t.Fatal("profile read did not reach the guarded cache write")
	}

	updateCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, (&AppConfig{}).CreateOrUpdate(updateCtx, "base", map[string]any{
		"websiteName": "new",
	}))
	release()

	select {
	case got := <-result:
		require.NoError(t, got.err)
		// This request linearized before the update and may return the old
		// snapshot, but it can only populate the now-obsolete revision-0 key.
		require.Equal(t, "old", got.profile["base"]["websiteName"])
	case <-time.After(2 * time.Second):
		t.Fatal("profile read did not complete after the revision changed")
	}

	profile, err := (&AppConfig{}).Profile(env.ctx, false)
	require.NoError(t, err)
	require.Equal(t, "new", profile["base"]["websiteName"])
	cached, err := env.redis.Get(publicProfileCacheKey(1))
	require.NoError(t, err)
	require.Contains(t, cached, `"websiteName":"new"`)
	require.NotContains(t, cached, `"websiteName":"old"`)
}

func TestPublicConfigWritesCommitWhenRedisIsUnavailable(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	env.redis.Close()

	theme, err := (&Theme{}).PatchApplicationResource(env.ctx, map[string]any{
		"navTheme": "realDark",
	}, nil)
	require.NoError(t, err, "cache cleanup is best effort after the database commit")
	require.Equal(t, "1", theme.Meta.Revision)

	require.NoError(t, (&AppConfig{}).CreateOrUpdate(env.ctx, "base", map[string]any{
		"websiteName": "Redis-independent",
	}))

	profile, err := (&AppConfig{}).Profile(env.ctx, false)
	require.NoError(t, err, "a cache read failure must fall back to authoritative database state")
	require.Equal(t, "Redis-independent", profile["base"]["websiteName"])
	require.Equal(t, "realDark", profile[ThemeConfigGroup]["navTheme"])
	require.Equal(t, "1", requireThemeProfileMeta(t, profile[ThemeConfigGroup]["_meta"])["revision"])

	var rows []models.ConfigRevision
	require.NoError(t, env.db.Order("resource").Find(&rows).Error)
	require.Len(t, rows, 2)
	revisions := make(map[string]int64, len(rows))
	for _, row := range rows {
		revisions[row.Resource] = row.Revision
	}
	require.Equal(t, int64(1), revisions[configRevisionResourceTheme])
	require.Equal(t, int64(2), revisions[configRevisionResourcePublicProfile])
}

func TestLegacySetAppConfigAdvancesProfileAndThemeRevisions(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.AppConfig{
		{Group: "base", Name: "websiteName", Value: "old-name", Auth: false},
		{Group: ThemeConfigGroup, Name: "navTheme", Value: "light", Auth: false},
	}).Error)

	svc := &AppConfig{}
	profile, err := svc.Profile(env.ctx, false)
	require.NoError(t, err)
	require.Equal(t, "old-name", profile["base"]["websiteName"])
	require.Equal(t, "light", profile[ThemeConfigGroup]["navTheme"])
	require.True(t, env.redis.Exists(publicProfileCacheKey(0)))

	store := center.GetAppConfig()
	require.NotNil(t, store)
	require.NoError(t, store.SetAppConfig(env.ctx, "theme:navTheme", false, "realDark"))
	profile, err = svc.Profile(env.ctx, false)
	require.NoError(t, err)
	require.Equal(t, "realDark", profile[ThemeConfigGroup]["navTheme"])
	themeMeta := requireThemeProfileMeta(t, profile[ThemeConfigGroup]["_meta"])
	require.Equal(t, "1", themeMeta["revision"])

	require.NoError(t, store.SetAppConfig(env.ctx, "base:websiteName", false, "new-name"))
	profile, err = svc.Profile(env.ctx, false)
	require.NoError(t, err)
	require.Equal(t, "new-name", profile["base"]["websiteName"])
	require.Equal(t, "realDark", profile[ThemeConfigGroup]["navTheme"])

	var profileRevision models.ConfigRevision
	require.NoError(t, env.db.Where(
		"scope = ? AND owner_id = ? AND resource = ?",
		models.ConfigRevisionScopeApplication,
		"",
		models.ConfigRevisionResourcePublicProfile,
	).Take(&profileRevision).Error)
	require.Equal(t, int64(2), profileRevision.Revision)
	var themeRevision models.ConfigRevision
	require.NoError(t, env.db.Where(
		"scope = ? AND owner_id = ? AND resource = ?",
		models.ConfigRevisionScopeApplication,
		"",
		models.ConfigRevisionResourceTheme,
	).Take(&themeRevision).Error)
	require.Equal(t, int64(1), themeRevision.Revision)
}
