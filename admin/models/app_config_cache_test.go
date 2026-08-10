package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
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
	cacheconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
)

const (
	appConfigCacheTestHash       = "app-configs:{entry}:v1"
	legacyAppConfigCacheTestHash = "appConfig"
)

type appConfigCacheTestTenant struct {
	db *gorm.DB
}

func (e *appConfigCacheTestTenant) Scope(_ *gin.Context, _ schema.Tabler) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB { return db }
}

func (e *appConfigCacheTestTenant) GetTenant(_ *gin.Context) (center.TenantImp, error) {
	return e, nil
}

func (e *appConfigCacheTestTenant) GetDB(_ *gin.Context, _ schema.Tabler) *gorm.DB {
	return e.db
}

func (*appConfigCacheTestTenant) GetID() any       { return "cache-test" }
func (*appConfigCacheTestTenant) GetDefault() bool { return true }

type appConfigCacheTestEnv struct {
	db     *gorm.DB
	redis  *miniredis.Miniredis
	client *redis.Client
	ctx    *gin.Context
}

func setupAppConfigCacheTestEnv(t *testing.T) appConfigCacheTestEnv {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&AppConfig{}, &ConfigRevision{}))
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX idx_app_config_cache_test ON mss_boot_app_configs(name, "group")`,
	).Error)

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cache, err := cacheconfig.NewRedis(client, nil)
	require.NoError(t, err)

	previousTenant := center.GetTenant()
	previousCache := center.GetCache()
	center.SetTenant(&appConfigCacheTestTenant{db: db})
	center.SetCache(cache)
	t.Cleanup(func() {
		center.SetTenant(previousTenant)
		center.SetCache(previousCache)
		_ = cache.Close()
		_ = sqlDB.Close()
	})

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return appConfigCacheTestEnv{db: db, redis: mr, client: client, ctx: ctx}
}

func appConfigCacheTestField(group, name string) string {
	return fmt.Sprintf("%s:%s", group, name)
}

func TestAppConfigReadThroughCacheSupportsEmptyAndMissingValues(t *testing.T) {
	t.Run("empty value", func(t *testing.T) {
		env := setupAppConfigCacheTestEnv(t)
		require.NoError(t, (&AppConfig{}).SetAppConfig(env.ctx, "base:empty", false, ""))

		var queries atomic.Int32
		require.NoError(t, env.db.Callback().Query().Before("gorm:query").Register(
			"test:count-app-config-empty-queries",
			func(*gorm.DB) { queries.Add(1) },
		))

		value, ok := (&AppConfig{}).GetAppConfig(env.ctx, "base:empty")
		require.True(t, ok)
		require.Empty(t, value)
		value, ok = (&AppConfig{}).GetAppConfig(env.ctx, "base:empty")
		require.True(t, ok)
		require.Empty(t, value)
		require.Equal(t, int32(1), queries.Load(), "the second empty-value read must use the cache")

		payload, err := env.client.HGet(
			env.ctx,
			appConfigCacheTestHash,
			appConfigCacheTestField("base", "empty"),
		).Bytes()
		require.NoError(t, err)
		var envelope map[string]any
		require.NoError(t, json.Unmarshal(payload, &envelope))
		require.Equal(t, "found", envelope["state"])
		require.Equal(t, "", envelope["value"])
	})

	t.Run("missing value", func(t *testing.T) {
		env := setupAppConfigCacheTestEnv(t)

		var queries atomic.Int32
		require.NoError(t, env.db.Callback().Query().Before("gorm:query").Register(
			"test:count-app-config-missing-queries",
			func(*gorm.DB) { queries.Add(1) },
		))

		value, ok := (&AppConfig{}).GetAppConfig(env.ctx, "base:missing")
		require.False(t, ok)
		require.Empty(t, value)
		value, ok = (&AppConfig{}).GetAppConfig(env.ctx, "base:missing")
		require.False(t, ok)
		require.Empty(t, value)
		require.Equal(t, int32(1), queries.Load(), "the second missing-value read must use negative cache")

		payload, err := env.client.HGet(
			env.ctx,
			appConfigCacheTestHash,
			appConfigCacheTestField("base", "missing"),
		).Bytes()
		require.NoError(t, err)
		var envelope map[string]any
		require.NoError(t, json.Unmarshal(payload, &envelope))
		require.Equal(t, "missing", envelope["state"])
	})
}

func TestSetAppConfigKeepsUngroupedKeysDistinct(t *testing.T) {
	env := setupAppConfigCacheTestEnv(t)
	require.NoError(t, env.db.Create(&AppConfig{
		Group: "grouped", Name: "shared-name", Value: "grouped-value",
	}).Error)

	store := &AppConfig{}
	require.NoError(t, store.SetAppConfig(env.ctx, "shared-name", false, "ungrouped-value"))
	value, ok := store.GetAppConfig(env.ctx, "shared-name")
	require.True(t, ok)
	require.Equal(t, "ungrouped-value", value)
	value, ok = store.GetAppConfig(env.ctx, "grouped:shared-name")
	require.True(t, ok)
	require.Equal(t, "grouped-value", value)
}

func TestAppConfigReadThroughCacheRejectsLegacyAndInvalidEnvelopes(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "legacy raw value", payload: "stale-raw-value"},
		{
			name: "unknown version",
			payload: fmt.Sprintf(
				`{"v":99,"group":"base","name":"websiteName","auth":false,"state":"found","value":"stale","expiresAt":%d}`,
				time.Now().Add(time.Minute).UnixMilli(),
			),
		},
		{
			name: "mismatched field",
			payload: fmt.Sprintf(
				`{"v":1,"group":"security","name":"websiteName","auth":false,"state":"found","value":"stale","expiresAt":%d}`,
				time.Now().Add(time.Minute).UnixMilli(),
			),
		},
		{
			name: "expired",
			payload: fmt.Sprintf(
				`{"v":1,"group":"base","name":"websiteName","auth":false,"state":"found","value":"stale","expiresAt":%d}`,
				time.Now().Add(-time.Second).UnixMilli(),
			),
		},
		{
			name: "invalid state",
			payload: fmt.Sprintf(
				`{"v":1,"group":"base","name":"websiteName","auth":false,"state":"unknown","value":"stale","expiresAt":%d}`,
				time.Now().Add(time.Minute).UnixMilli(),
			),
		},
		{
			name: "authenticated envelope",
			payload: fmt.Sprintf(
				`{"v":1,"group":"base","name":"websiteName","auth":true,"state":"found","value":"stale","expiresAt":%d}`,
				time.Now().Add(time.Minute).UnixMilli(),
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := setupAppConfigCacheTestEnv(t)
			require.NoError(t, env.db.Create(&AppConfig{
				Group: "base", Name: "websiteName", Value: "database-value",
			}).Error)
			env.redis.HSet(
				appConfigCacheTestHash,
				appConfigCacheTestField("base", "websiteName"),
				test.payload,
			)

			value, ok := (&AppConfig{}).GetAppConfig(env.ctx, "base:websiteName")
			require.True(t, ok)
			require.Equal(t, "database-value", value)
			cached, err := env.client.HGet(
				env.ctx,
				appConfigCacheTestHash,
				appConfigCacheTestField("base", "websiteName"),
			).Result()
			require.NoError(t, err)
			require.NotEqual(t, test.payload, cached, "a cache miss must be replaced by a fresh envelope")
		})
	}
}

func TestAppConfigReadThroughIgnoresAndRemovesLegacyNamespace(t *testing.T) {
	env := setupAppConfigCacheTestEnv(t)
	require.NoError(t, env.db.Create(&AppConfig{
		Group: "base", Name: "websiteName", Value: "database-value",
	}).Error)
	field := appConfigCacheTestField("base", "websiteName")
	env.redis.HSet(legacyAppConfigCacheTestHash, field, "stale-legacy-value")

	value, ok := (&AppConfig{}).GetAppConfig(env.ctx, "base:websiteName")
	require.True(t, ok)
	require.Equal(t, "database-value", value)

	legacyExists, err := env.client.HExists(env.ctx, legacyAppConfigCacheTestHash, field).Result()
	require.NoError(t, err)
	require.False(t, legacyExists)
	currentExists, err := env.client.HExists(env.ctx, appConfigCacheTestHash, field).Result()
	require.NoError(t, err)
	require.True(t, currentExists)
}

func TestAppConfigReadThroughRemovesInvalidEntryWhenDatabaseFails(t *testing.T) {
	env := setupAppConfigCacheTestEnv(t)
	field := appConfigCacheTestField("base", "websiteName")
	env.redis.HSet(appConfigCacheTestHash, field, "malformed")
	require.NoError(t, env.db.Migrator().DropTable(&AppConfig{}))

	value, ok := (&AppConfig{}).GetAppConfig(env.ctx, "base:websiteName")
	require.False(t, ok)
	require.Empty(t, value)
	cached, err := env.client.HExists(env.ctx, appConfigCacheTestHash, field).Result()
	require.NoError(t, err)
	require.False(t, cached, "invalid entries must not survive a failed database fallback")
}

func TestAppConfigDatabaseMatchIsCaseSensitive(t *testing.T) {
	env := setupAppConfigCacheTestEnv(t)
	require.NoError(t, env.db.Migrator().DropTable(&AppConfig{}))
	require.NoError(t, env.db.Exec(`
		CREATE TABLE mss_boot_app_configs (
			id varchar(64) PRIMARY KEY NOT NULL,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime,
			name varchar(128) COLLATE NOCASE NOT NULL DEFAULT '',
			"group" varchar(128) COLLATE NOCASE NOT NULL DEFAULT '',
			value varchar(255) NOT NULL DEFAULT '',
			auth numeric NOT NULL DEFAULT 0
		)
	`).Error)
	require.NoError(t, env.db.Create(&AppConfig{
		Group: "Base", Name: "WebsiteName", Value: "alias-value",
	}).Error)

	value, ok := (&AppConfig{}).GetAppConfig(env.ctx, "base:websiteName")
	require.False(t, ok, "a case-insensitive database match must not satisfy an exact config key")
	require.Empty(t, value)
	err := (&AppConfig{}).SetAppConfig(env.ctx, "base:websiteName", false, "must-not-overwrite")
	require.ErrorIs(t, err, ErrAppConfigIdentityMismatch)
	var stored AppConfig
	require.NoError(t, env.db.Where("value = ?", "alias-value").First(&stored).Error)
	require.Equal(t, "Base", stored.Group)
	require.Equal(t, "WebsiteName", stored.Name)
	require.Equal(t, "alias-value", stored.Value)

	payload, err := env.client.HGet(
		env.ctx,
		appConfigCacheTestHash,
		appConfigCacheTestField("base", "websiteName"),
	).Bytes()
	require.NoError(t, err)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(payload, &envelope))
	require.Equal(t, "missing", envelope["state"])
}

func TestAppConfigDatabaseFallbackUsesWriter(t *testing.T) {
	dir := t.TempDir()
	writerPath := filepath.Join(dir, "writer.db")
	replicaPath := filepath.Join(dir, "replica.db")
	writer, err := gorm.Open(sqlite.Open(writerPath), &gorm.Config{})
	require.NoError(t, err)
	replica, err := gorm.Open(sqlite.Open(replicaPath), &gorm.Config{})
	require.NoError(t, err)
	for _, db := range []*gorm.DB{writer, replica} {
		require.NoError(t, db.AutoMigrate(&AppConfig{}))
	}
	require.NoError(t, writer.Create(&AppConfig{
		Group: "base", Name: "websiteName", Value: "writer-value",
	}).Error)
	require.NoError(t, replica.Create(&AppConfig{
		Group: "base", Name: "websiteName", Value: "replica-value",
	}).Error)
	require.NoError(t, writer.Use(dbresolver.Register(dbresolver.Config{
		Replicas: []gorm.Dialector{sqlite.Open(replicaPath)},
	})))

	var defaultRead AppConfig
	require.NoError(t, writer.Where(&AppConfig{Group: "base", Name: "websiteName"}).First(&defaultRead).Error)
	require.Equal(t, "replica-value", defaultRead.Value, "test setup must route ordinary reads to the replica")

	previousTenant := center.GetTenant()
	previousCache := center.GetCache()
	center.SetTenant(&appConfigCacheTestTenant{db: writer})
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

	value, ok := (&AppConfig{}).GetAppConfig(ctx, "base:websiteName")
	require.True(t, ok)
	require.Equal(t, "writer-value", value)
}

func TestAppConfigReadThroughDoesNotCacheAuthenticatedValues(t *testing.T) {
	env := setupAppConfigCacheTestEnv(t)
	require.NoError(t, env.db.Create(&AppConfig{
		Group: "email", Name: "password", Value: "database-secret", Auth: true,
	}).Error)

	var queries atomic.Int32
	require.NoError(t, env.db.Callback().Query().Before("gorm:query").Register(
		"test:count-authenticated-app-config-queries",
		func(*gorm.DB) { queries.Add(1) },
	))

	field := appConfigCacheTestField("email", "password")
	env.redis.HSet(
		appConfigCacheTestHash,
		field,
		fmt.Sprintf(
			`{"v":1,"group":"email","name":"password","auth":true,"state":"found","value":"cached-secret","expiresAt":%d}`,
			time.Now().Add(time.Minute).UnixMilli(),
		),
	)

	store := &AppConfig{}
	value, ok := store.GetAppConfig(env.ctx, "email:password")
	require.True(t, ok)
	require.Equal(t, "database-secret", value)
	value, ok = store.GetAppConfig(env.ctx, "email:password")
	require.True(t, ok)
	require.Equal(t, "database-secret", value)
	require.Equal(t, int32(2), queries.Load(), "authenticated values must always be read from the database")

	cached, err := env.client.HExists(env.ctx, appConfigCacheTestHash, field).Result()
	require.NoError(t, err)
	require.False(t, cached, "authenticated values must not remain in the shared cache")
}

func TestSetAppConfigWritesDatabaseBeforeInvalidatingCache(t *testing.T) {
	env := setupAppConfigCacheTestEnv(t)
	store := &AppConfig{}
	require.NoError(t, store.SetAppConfig(env.ctx, "base:websiteName", false, "old"))
	value, ok := store.GetAppConfig(env.ctx, "base:websiteName")
	require.True(t, ok)
	require.Equal(t, "old", value)
	field := appConfigCacheTestField("base", "websiteName")
	before, err := env.client.HGet(env.ctx, appConfigCacheTestHash, field).Result()
	require.NoError(t, err)

	require.NoError(t, env.db.Exec(`
		CREATE TRIGGER fail_app_config_cache_update
		BEFORE UPDATE ON mss_boot_app_configs
		WHEN NEW.name = 'websiteName'
		BEGIN
			SELECT RAISE(ABORT, 'forced app config update failure');
		END;
	`).Error)
	err = store.SetAppConfig(env.ctx, "base:websiteName", false, "new")
	require.Error(t, err)
	after, cacheErr := env.client.HGet(env.ctx, appConfigCacheTestHash, field).Result()
	require.NoError(t, cacheErr)
	require.Equal(t, before, after, "a failed database write must not publish or invalidate cache state")

	value, ok = store.GetAppConfig(env.ctx, "base:websiteName")
	require.True(t, ok)
	require.Equal(t, "old", value)
}

func TestSetAppConfigRollsBackValueAndRevisionsWhenThemeRevisionAdvanceFails(t *testing.T) {
	env := setupAppConfigCacheTestEnv(t)
	require.NoError(t, env.db.Create(&AppConfig{
		Group: ConfigRevisionResourceTheme,
		Name:  "navTheme",
		Value: "light",
		Auth:  false,
	}).Error)
	require.NoError(t, env.db.Create([]*ConfigRevision{
		{
			Scope: ConfigRevisionScopeApplication, OwnerID: "",
			Resource: ConfigRevisionResourcePublicProfile, Revision: 7,
		},
		{
			Scope: ConfigRevisionScopeApplication, OwnerID: "",
			Resource: ConfigRevisionResourceTheme, Revision: 4,
		},
	}).Error)
	require.NoError(t, env.db.Exec(`
		CREATE TRIGGER fail_theme_revision_advance
		BEFORE UPDATE OF revision ON mss_boot_config_revisions
		WHEN OLD.scope = 'application'
			AND OLD.owner_id = ''
			AND OLD.resource = 'theme'
		BEGIN
			SELECT RAISE(ABORT, 'forced theme revision advance failure');
		END;
	`).Error)

	err := (&AppConfig{}).SetAppConfig(env.ctx, "theme:navTheme", false, "realDark")
	require.ErrorContains(t, err, "forced theme revision advance failure")

	var stored AppConfig
	require.NoError(t, env.db.Clauses(dbresolver.Write).
		Where(&AppConfig{Group: ConfigRevisionResourceTheme, Name: "navTheme"}).
		Take(&stored).Error)
	require.Equal(t, "light", stored.Value, "the app config update must roll back")

	var revisions []ConfigRevision
	require.NoError(t, env.db.Clauses(dbresolver.Write).
		Where(
			"scope = ? AND owner_id = ? AND resource IN ?",
			ConfigRevisionScopeApplication,
			"",
			[]string{ConfigRevisionResourcePublicProfile, ConfigRevisionResourceTheme},
		).
		Find(&revisions).Error)
	require.Len(t, revisions, 2)
	byResource := make(map[string]int64, len(revisions))
	for _, revision := range revisions {
		byResource[revision.Resource] = revision.Revision
	}
	require.Equal(t, int64(7), byResource[ConfigRevisionResourcePublicProfile],
		"the earlier profile revision advance must roll back")
	require.Equal(t, int64(4), byResource[ConfigRevisionResourceTheme],
		"the failed theme revision must remain unchanged")
}

func TestSetAppConfigSuccessInvalidatesAndRefillsFromDatabase(t *testing.T) {
	env := setupAppConfigCacheTestEnv(t)
	store := &AppConfig{}
	field := appConfigCacheTestField("base", "websiteName")

	require.NoError(t, store.SetAppConfig(env.ctx, "base:websiteName", false, "old"))
	value, ok := store.GetAppConfig(env.ctx, "base:websiteName")
	require.True(t, ok)
	require.Equal(t, "old", value)
	cached, err := env.client.HExists(env.ctx, appConfigCacheTestHash, field).Result()
	require.NoError(t, err)
	require.True(t, cached)

	require.NoError(t, store.SetAppConfig(env.ctx, "base:websiteName", false, "new"))
	cached, err = env.client.HExists(env.ctx, appConfigCacheTestHash, field).Result()
	require.NoError(t, err)
	require.False(t, cached)
	value, ok = store.GetAppConfig(env.ctx, "base:websiteName")
	require.True(t, ok)
	require.Equal(t, "new", value)
	cached, err = env.client.HExists(env.ctx, appConfigCacheTestHash, field).Result()
	require.NoError(t, err)
	require.True(t, cached)
}

type appConfigCacheFailureHook struct {
	err error
}

func (h appConfigCacheFailureHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h appConfigCacheFailureHook) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return func(context.Context, redis.Cmder) error { return h.err }
}

func (h appConfigCacheFailureHook) ProcessPipelineHook(redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(context.Context, []redis.Cmder) error { return h.err }
}

func TestAppConfigRedisFailuresDoNotAffectDatabaseReadsOrWrites(t *testing.T) {
	env := setupAppConfigCacheTestEnv(t)
	env.client.AddHook(appConfigCacheFailureHook{err: errors.New("cache unavailable")})

	store := &AppConfig{}
	require.NoError(t, store.SetAppConfig(env.ctx, "base:websiteName", false, "database-value"))
	value, ok := store.GetAppConfig(env.ctx, "base:websiteName")
	require.True(t, ok)
	require.Equal(t, "database-value", value)
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

func TestAppConfigCacheOperationsHaveBoundedLatency(t *testing.T) {
	env := setupAppConfigCacheTestEnv(t)
	require.NoError(t, env.db.Create(&AppConfig{
		Group: "base", Name: "websiteName", Value: "database-value",
	}).Error)
	env.client.AddHook(appConfigCacheDeadlineHook{})

	started := time.Now()
	value, ok := (&AppConfig{}).GetAppConfig(env.ctx, "base:websiteName")
	elapsed := time.Since(started)
	require.True(t, ok)
	require.Equal(t, "database-value", value)
	require.Less(t, elapsed, time.Second, "optional cache operations must not inherit a long Redis timeout")
}

func TestAppConfigSnapshotPropagatesDatabaseFailure(t *testing.T) {
	env := setupAppConfigCacheTestEnv(t)
	require.NoError(t, env.db.Create([]*AppConfig{
		{Group: "storage", Name: "maxSize", Value: "1024"},
		{Group: "storage", Name: "allowedTypes", Value: "text/plain"},
	}).Error)

	values, err := (&AppConfig{}).GetAppConfigSnapshot(
		env.ctx,
		"storage:maxSize",
		"storage:allowedTypes",
	)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"storage:maxSize":      "1024",
		"storage:allowedTypes": "text/plain",
	}, values)

	sqlDB, err := env.db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = (&AppConfig{}).GetAppConfigSnapshot(env.ctx, "storage:maxSize", "storage:allowedTypes")
	require.Error(t, err)
}
