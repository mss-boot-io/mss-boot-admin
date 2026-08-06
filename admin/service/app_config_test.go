package service

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

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

func setupAppConfigTestEnv(t *testing.T) appConfigTestEnv {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.AppConfig{}))
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX idx_app_config_test ON mss_boot_app_configs(name, "group")`,
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

		cached, err := env.redis.Get(appConfigPublicProfileCacheKey)
		require.NoError(t, err)
		require.NotContains(t, cached, "staff only")
		require.NotContains(t, cached, "leaked-before-fix")
	}
}

func TestAppConfigProfileFiltersSensitiveKeysFromPoisonedPublicCache(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	env.redis.Set(appConfigPublicProfileCacheKey, `{
		"generation":0,
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

func TestAppConfigCreateOrUpdateClassifiesByKeyAndInvalidatesPublicCache(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.AppConfig{
		// Simulate both directions of the old value-based classification bug.
		{Group: "base", Name: "websiteName", Value: "old-token-shaped-value", Auth: true},
		{Group: "security", Name: "githubEnabled", Value: "false", Auth: true},
		{Group: "security", Name: "githubClientSecret", Value: "old-ordinary-value", Auth: false},
	}).Error)
	env.redis.Set(appConfigPublicProfileCacheKey, `{"generation":0,"profile":{"base":{"websiteName":"old"}}}`)

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
	require.False(t, env.redis.Exists(appConfigPublicProfileCacheKey))

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
	group, err := svc.Group(env.ctx, "security")
	require.NoError(t, err)
	require.Equal(t, "ordinary-looking-public-id", group["githubClientId"])
	require.Equal(t, "ordinary-looking-value", group["githubClientSecret"])
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

func (h *blockingPublicProfileCacheHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *blockingPublicProfileCacheHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		args := cmd.Args()
		if cmd.Name() == "eval" && len(args) > 1 {
			if script, ok := args[1].(string); ok && script == cachePublicProfileIfCurrentScript {
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

func TestAppConfigProfileDoesNotRepopulateStaleCacheAfterConcurrentUpdate(t *testing.T) {
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
		require.Equal(t, "new", got.profile["base"]["websiteName"])
	case <-time.After(2 * time.Second):
		t.Fatal("profile read did not retry after the generation changed")
	}

	profile, err := (&AppConfig{}).Profile(env.ctx, false)
	require.NoError(t, err)
	require.Equal(t, "new", profile["base"]["websiteName"])
	cached, err := env.redis.Get(appConfigPublicProfileCacheKey)
	require.NoError(t, err)
	require.Contains(t, cached, `"websiteName":"new"`)
	require.NotContains(t, cached, `"websiteName":"old"`)
}
