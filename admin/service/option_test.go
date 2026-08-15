package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

func setupOptionTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.Option{}, &models.OptionVersion{})
	assert.NoError(t, err)

	return db
}

func TestOptionModel_Fields(t *testing.T) {
	option := models.Option{
		Category:    "system",
		Name:        "status",
		DisplayName: "Status Options",
		Description: "Basic status options",
		Remark:      "System options",
		Status:      enum.Enabled,
		Version:     1,
		BuiltIn:     true,
	}

	assert.Equal(t, "system", option.Category)
	assert.Equal(t, "status", option.Name)
	assert.Equal(t, "Status Options", option.DisplayName)
	assert.Equal(t, "Basic status options", option.Description)
	assert.True(t, option.BuiltIn)
	assert.Equal(t, 1, option.Version)
}

func TestOptionVersionModel(t *testing.T) {
	version := models.OptionVersion{
		OptionID:   "opt-123",
		Version:    1,
		ChangedBy:  "user-456",
		ChangeNote: "Initial version",
		Status:     enum.Enabled,
	}

	assert.Equal(t, "opt-123", version.OptionID)
	assert.Equal(t, 1, version.Version)
	assert.Equal(t, "user-456", version.ChangedBy)
	assert.Equal(t, "Initial version", version.ChangeNote)
}

func TestOptionItem_JSONSerialization(t *testing.T) {
	items := &models.OptionItems{
		{ID: "1", Key: "enabled", Label: "Enabled", Value: "enabled", Color: "green", Sort: 1, Icon: "check", Extra: map[string]any{"description": "Active status"}},
		{ID: "2", Key: "disabled", Label: "Disabled", Value: "disabled", Color: "red", Sort: 2},
	}

	data, err := json.Marshal(items)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "enabled")
	assert.Contains(t, string(data), "check")
	assert.Contains(t, string(data), "Active status")

	var decodedItems models.OptionItems
	err = json.Unmarshal(data, &decodedItems)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(decodedItems))
	assert.Equal(t, "check", decodedItems[0].Icon)
	assert.Equal(t, "Active status", decodedItems[0].Extra["description"])
}

func TestOption_CRUD(t *testing.T) {
	db := setupOptionTestDB(t)

	option := &models.Option{
		Category:    "system",
		Name:        "status",
		DisplayName: "Status Options",
		Description: "Test description",
		Status:      enum.Enabled,
		Version:     1,
		BuiltIn:     true,
		Items: &models.OptionItems{
			{Key: "enabled", Label: "Enabled", Value: "enabled", Color: "green", Sort: 1},
			{Key: "disabled", Label: "Disabled", Value: "disabled", Color: "red", Sort: 2},
		},
	}

	result := db.Create(option)
	assert.NoError(t, result.Error)
	assert.NotEmpty(t, option.ID)

	var fetched models.Option
	err := db.Where("category = ? AND name = ?", "system", "status").First(&fetched).Error
	assert.NoError(t, err)
	assert.Equal(t, "system", fetched.Category)
	assert.Equal(t, "status", fetched.Name)
	assert.Equal(t, 2, len(*fetched.Items))
}

func TestOptionVersion_Tracking(t *testing.T) {
	db := setupOptionTestDB(t)

	option := &models.Option{
		Category:    "system",
		Name:        "status",
		DisplayName: "Status Options",
		Status:      enum.Enabled,
		Version:     1,
		BuiltIn:     true,
		Items: &models.OptionItems{
			{Key: "enabled", Label: "Enabled", Value: "enabled", Color: "green", Sort: 1},
		},
	}
	result := db.Create(option)
	assert.NoError(t, result.Error)

	versionSnapshot := &models.OptionVersion{
		OptionID:   option.ID,
		Version:    option.Version,
		Items:      option.Items,
		ChangedBy:  "user-123",
		ChangeNote: "Initial version",
		Status:     enum.Enabled,
	}
	err := db.Create(versionSnapshot).Error
	assert.NoError(t, err)

	option.Version = 2
	option.Items = &models.OptionItems{
		{Key: "enabled", Label: "Enabled", Value: "enabled", Color: "green", Sort: 1},
		{Key: "disabled", Label: "Disabled", Value: "disabled", Color: "red", Sort: 2},
	}
	err = db.Save(option).Error
	assert.NoError(t, err)

	var updatedOption models.Option
	db.First(&updatedOption, "id = ?", option.ID)
	assert.Equal(t, 2, updatedOption.Version)
	assert.Equal(t, 2, len(*updatedOption.Items))

	var version models.OptionVersion
	db.First(&version, "option_id = ?", option.ID)
	assert.Equal(t, 1, version.Version)
	assert.NotNil(t, version.Items)
}

func TestOptionItem_Validation(t *testing.T) {
	item := models.OptionItem{
		Key:   "test",
		Label: "Test Option",
		Value: "test",
		Color: "blue",
		Sort:  1,
	}

	assert.Equal(t, "test", item.Key)
	assert.Equal(t, "Test Option", item.Label)
	assert.Equal(t, "test", item.Value)
	assert.Equal(t, "blue", item.Color)
	assert.Equal(t, 1, item.Sort)
}

func TestOptionItems_Value_Scan(t *testing.T) {
	items := &models.OptionItems{
		{ID: "1", Key: "a", Label: "A", Value: "a", Sort: 1},
		{ID: "2", Key: "b", Label: "B", Value: "b", Sort: 2},
	}

	value, err := items.Value()
	assert.NoError(t, err)
	assert.NotNil(t, value)

	var scannedItems models.OptionItems
	err = scannedItems.Scan(value)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(scannedItems))
}

func TestUpdateOptionCacheFailureDoesNotFailCommittedMutation(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.AutoMigrate(&models.Option{}, &models.OptionVersion{}))
	option := &models.Option{
		Category: "system",
		Name:     "status",
		Status:   enum.Enabled,
		Version:  1,
		Items:    &models.OptionItems{{Key: "old", Label: "Old", Value: "old"}},
	}
	require.NoError(t, env.db.Create(option).Error)

	// Keep the configured cache adapter but make Redis unavailable. The
	// mutation must still report the authoritative database result.
	env.redis.Close()
	updated := &models.OptionItems{{Key: "new", Label: "New", Value: "new"}}
	require.NoError(t, NewOption().UpdateOption(env.ctx, option.ID, updated, "tester", "cache fault"))

	var persisted models.Option
	require.NoError(t, env.db.First(&persisted, "id = ?", option.ID).Error)
	require.Equal(t, 2, persisted.Version)
	require.Equal(t, "new", (*persisted.Items)[0].Value)
}

func TestUpdateOptionSnapshotFailureRollsBackMutation(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.AutoMigrate(&models.Option{}, &models.OptionVersion{}))
	option := &models.Option{
		Category: "system",
		Name:     "status",
		Status:   enum.Enabled,
		Version:  1,
		Items:    &models.OptionItems{{Key: "old", Label: "Old", Value: "old"}},
	}
	require.NoError(t, env.db.Create(option).Error)
	require.NoError(t, env.client.Set(env.ctx, "options:system:status", "cached-before-failure", optionCacheTTL).Err())
	require.NoError(t, env.db.Exec(`
		CREATE TRIGGER fail_option_version_insert
		BEFORE INSERT ON mss_boot_option_versions
		BEGIN
			SELECT RAISE(ABORT, 'forced option snapshot failure');
		END;
	`).Error)

	updated := &models.OptionItems{{Key: "new", Label: "New", Value: "new"}}
	err := NewOption().UpdateOption(env.ctx, option.ID, updated, "tester", "must roll back")
	require.Error(t, err)

	var persisted models.Option
	require.NoError(t, env.db.First(&persisted, "id = ?", option.ID).Error)
	require.Equal(t, 1, persisted.Version)
	require.Equal(t, "old", (*persisted.Items)[0].Value)
	var snapshotCount int64
	require.NoError(t, env.db.Model(&models.OptionVersion{}).Count(&snapshotCount).Error)
	require.Zero(t, snapshotCount)
	cached, err := env.redis.Get("options:system:status")
	require.NoError(t, err)
	require.Equal(t, "cached-before-failure", cached)
}

func TestUpdateOptionConcurrentWritersDoNotLoseVersions(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.AutoMigrate(&models.Option{}, &models.OptionVersion{}))
	option := &models.Option{
		Category: "system",
		Name:     "status",
		Status:   enum.Enabled,
		Version:  1,
		Items:    &models.OptionItems{{Key: "old", Label: "Old", Value: "old"}},
	}
	require.NoError(t, env.db.Create(option).Error)

	start := make(chan struct{})
	errorsCh := make(chan error, 2)
	for _, value := range []string{"alpha", "beta"} {
		value := value
		go func() {
			<-start
			items := &models.OptionItems{{Key: value, Label: value, Value: value}}
			errorsCh <- NewOption().UpdateOption(env.ctx.Copy(), option.ID, items, value, "concurrent")
		}()
	}
	close(start)
	for range 2 {
		require.NoError(t, <-errorsCh)
	}

	var persisted models.Option
	require.NoError(t, env.db.First(&persisted, "id = ?", option.ID).Error)
	require.Equal(t, 3, persisted.Version)
	require.NotNil(t, persisted.Items)
	require.Len(t, *persisted.Items, 1)

	var snapshots []models.OptionVersion
	require.NoError(t, env.db.Where("option_id = ?", option.ID).Order("version").Find(&snapshots).Error)
	require.Len(t, snapshots, 2)
	require.Equal(t, []int{1, 2}, []int{snapshots[0].Version, snapshots[1].Version})
	require.Equal(t, "old", (*snapshots[0].Items)[0].Value)
	require.ElementsMatch(
		t,
		[]string{"alpha", "beta"},
		[]string{(*snapshots[1].Items)[0].Value, (*persisted.Items)[0].Value},
	)
}

func TestOptionCacheOperationsHaveBoundedLatency(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.AutoMigrate(&models.Option{}, &models.OptionVersion{}))
	option := &models.Option{
		Category: "system",
		Name:     "status",
		Status:   enum.Enabled,
		Version:  1,
		Items:    &models.OptionItems{{Key: "old", Label: "Old", Value: "old"}},
	}
	require.NoError(t, env.db.Create(option).Error)
	env.client.AddHook(appConfigCacheDeadlineHook{})

	started := time.Now()
	loaded, err := NewOption().GetOption(env.ctx, option.Category, option.Name)
	require.NoError(t, err)
	require.Equal(t, option.ID, loaded.ID)
	require.Less(t, time.Since(started), 750*time.Millisecond, "GET and SET cache failures must be bounded")

	started = time.Now()
	updated := &models.OptionItems{{Key: "new", Label: "New", Value: "new"}}
	require.NoError(t, NewOption().UpdateOption(env.ctx, option.ID, updated, "tester", "cache timeout"))
	require.Less(t, time.Since(started), 500*time.Millisecond, "DEL timeout must not dominate committed response")
	var persisted models.Option
	require.NoError(t, env.db.First(&persisted, "id = ?", option.ID).Error)
	require.Equal(t, 2, persisted.Version)
	require.Equal(t, "new", (*persisted.Items)[0].Value)
}

func TestOptionCacheMissRefillsFromWriterInsteadOfLaggingReplica(t *testing.T) {
	dir := t.TempDir()
	writerPath := filepath.Join(dir, "option-writer.db")
	replicaPath := filepath.Join(dir, "option-replica.db")
	writer, err := gorm.Open(sqlite.Open(writerPath), &gorm.Config{})
	require.NoError(t, err)
	replica, err := gorm.Open(sqlite.Open(replicaPath), &gorm.Config{})
	require.NoError(t, err)
	for _, db := range []*gorm.DB{writer, replica} {
		require.NoError(t, db.AutoMigrate(&models.Option{}))
	}

	writerOption := &models.Option{
		Category: "system", Name: "status", Status: enum.Enabled, Version: 2,
		Items: &models.OptionItems{{Key: "new", Label: "New", Value: "new"}},
	}
	require.NoError(t, writer.Create(writerOption).Error)
	replicaOption := &models.Option{
		Category: "system", Name: "status", Status: enum.Enabled, Version: 1,
		Items: &models.OptionItems{{Key: "old", Label: "Old", Value: "old"}},
	}
	replicaOption.ID = writerOption.ID
	require.NoError(t, replica.Create(replicaOption).Error)
	require.NoError(t, writer.Use(dbresolver.Register(dbresolver.Config{
		Replicas: []gorm.Dialector{sqlite.Open(replicaPath)},
	})))

	var replicaRead models.Option
	require.NoError(t, writer.Where("id = ?", writerOption.ID).First(&replicaRead).Error)
	require.Equal(t, 1, replicaRead.Version, "test setup must route an ordinary read to the replica")

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

	loaded, err := NewOption().GetOption(ctx, "system", "status")
	require.NoError(t, err)
	require.Equal(t, 2, loaded.Version)
	require.Equal(t, "new", (*loaded.Items)[0].Value)
}

func TestOptionResourceMutationsSnapshotFullStateAndRejectStaleWriters(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.AutoMigrate(&models.Option{}, &models.OptionVersion{}, &models.OptionUsage{}))
	items := &models.OptionItems{{Key: "old", Label: "Old", Value: "old", Extra: map[string]any{"source": "seed"}}}
	option := &models.Option{
		Category:    "system",
		Name:        "status",
		DisplayName: "Status",
		Description: "before",
		Remark:      "seed",
		Status:      enum.Enabled,
		Version:     1,
		Items:       items,
	}
	require.NoError(t, env.db.Create(option).Error)
	originalItemID := (*option.Items)[0].ID

	displayName := "Status values"
	description := "after"
	requestedItems := &models.OptionItems{
		{ID: originalItemID, Key: "old", Label: "Updated", Value: "old", Extra: map[string]any{"source": "updated"}},
		{ID: "foreign-client-id", Key: "new", Label: "New", Value: "new"},
	}
	expected := 1
	updated, err := NewOption().UpdateOptionResource(env.ctx, option.ID, OptionUpdateInput{
		DisplayName:     &displayName,
		Description:     &description,
		Items:           requestedItems,
		ExpectedVersion: &expected,
	}, "actor", "full snapshot")
	require.NoError(t, err)
	require.Equal(t, 2, updated.Version)
	require.Equal(t, originalItemID, (*updated.Items)[0].ID)
	require.NotEqual(t, "foreign-client-id", (*updated.Items)[1].ID)
	require.NotEmpty(t, (*updated.Items)[1].ID)

	var snapshot models.OptionVersion
	require.NoError(t, env.db.Where("option_id = ? AND version = ?", option.ID, 1).First(&snapshot).Error)
	require.Equal(t, "system", snapshot.Category)
	require.Equal(t, "status", snapshot.Name)
	require.Equal(t, "Status", snapshot.DisplayName)
	require.Equal(t, "before", snapshot.Description)
	require.Equal(t, "seed", snapshot.Remark)
	require.Equal(t, enum.Enabled, snapshot.OptionStatus)
	require.False(t, snapshot.BuiltIn)
	require.Equal(t, "old", (*snapshot.Items)[0].Value)
	require.Equal(t, "actor", snapshot.ChangedBy)

	_, err = NewOption().UpdateOptionResource(env.ctx, option.ID, OptionUpdateInput{
		DisplayName:     &displayName,
		ExpectedVersion: &expected,
	}, "stale", "must fail")
	require.ErrorIs(t, err, ErrOptionVersionChanged)
	var conflict *OptionRevisionConflictError
	require.True(t, errors.As(err, &conflict))
	require.Equal(t, 2, conflict.Current.Version)
	var snapshotCount int64
	require.NoError(t, env.db.Model(&models.OptionVersion{}).Count(&snapshotCount).Error)
	require.EqualValues(t, 1, snapshotCount)
}

func TestOptionBuiltInAndUsageDeletionBoundaries(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.AutoMigrate(&models.Option{}, &models.OptionVersion{}, &models.OptionUsage{}))
	builtIn := &models.Option{
		Category: "system", Name: "status", Status: enum.Enabled, Version: 1, BuiltIn: true,
		Items: &models.OptionItems{{Key: "enabled", Label: "Enabled", Value: "enabled"}},
	}
	require.NoError(t, env.db.Create(builtIn).Error)

	items := &models.OptionItems{{ID: (*builtIn.Items)[0].ID, Key: "enabled", Label: "Active", Value: "enabled"}}
	expected := 1
	updated, err := NewOption().UpdateOptionResource(env.ctx, builtIn.ID, OptionUpdateInput{
		Items: items, ExpectedVersion: &expected,
	}, "actor", "allowed item edit")
	require.NoError(t, err)
	require.Equal(t, 2, updated.Version)

	disabled := enum.Disabled
	expected = 2
	_, err = NewOption().UpdateOptionResource(env.ctx, builtIn.ID, OptionUpdateInput{
		Status: &disabled, ExpectedVersion: &expected,
	}, "actor", "forbidden identity mutation")
	require.ErrorIs(t, err, ErrOptionBuiltIn)
	_, err = NewOption().DeleteOption(env.ctx, builtIn.ID, &expected, "actor", "forbidden delete")
	require.ErrorIs(t, err, ErrOptionBuiltIn)

	custom := &models.Option{
		Category: "custom", Name: "priority", Status: enum.Enabled, Version: 1,
		Items: &models.OptionItems{{Key: "high", Label: "High", Value: "high"}},
	}
	require.NoError(t, env.db.Create(custom).Error)
	usage := &models.OptionUsage{OptionID: custom.ID, UsedBy: "orders", Status: enum.Enabled}
	require.NoError(t, env.db.Create(usage).Error)
	expected = 1
	_, err = NewOption().DeleteOption(env.ctx, custom.ID, &expected, "actor", "still used")
	require.ErrorIs(t, err, ErrOptionInUse)
	require.NoError(t, env.db.Model(usage).Update("status", enum.Disabled).Error)
	deleted, err := NewOption().DeleteOption(env.ctx, custom.ID, &expected, "actor", "retire")
	require.NoError(t, err)
	require.Equal(t, custom.ID, deleted.ID)
	var remaining int64
	require.NoError(t, env.db.Model(&models.Option{}).Where("id = ?", custom.ID).Count(&remaining).Error)
	require.Zero(t, remaining)
}

func TestOptionCacheKeyNamespacesAndEncodesTenantAndIdentity(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	key, ready := optionCacheKey(env.ctx, "system:admin", "status/value")
	require.True(t, ready)
	require.NotContains(t, key, "test")
	require.NotContains(t, key, "system:admin")
	require.NotContains(t, key, "status/value")
	parts := strings.Split(key, ":")
	require.Len(t, parts, 5)
	require.Equal(t, []string{"options", "v2"}, parts[:2])
	decoded := make([]string, 0, 3)
	for _, part := range parts[2:] {
		value, err := base64.RawURLEncoding.DecodeString(part)
		require.NoError(t, err)
		decoded = append(decoded, string(value))
	}
	require.Equal(t, []string{"test", "system:admin", "status/value"}, decoded)
}
