package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openOperationalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Task{}, &models.Notice{}, &models.SystemConfig{}))
	return db
}

func TestPrepareTaskCreateOwnsIdentityAndRejectsUnsafeDefinitions(t *testing.T) {
	db := openOperationalTestDB(t)
	definition := &models.Task{
		Name:     " health check ",
		Provider: models.TaskProviderDefault,
		Spec:     "*/5 * * * * *",
		Protocol: "HTTPS",
		Endpoint: "example.com/health",
		Method:   "get",
		Status:   enum.Enabled,
		EntryID:  42,
		Once:     true,
		Python:   "alert(document.cookie)",
		Command:  `["sh"]`,
		Args:     `["-c"]`,
	}
	definition.ID = "client-controlled"
	definition.CreatorID = "forged"

	require.NoError(t, PrepareTaskCreate(context.Background(), db, definition))
	require.Empty(t, definition.ID)
	require.Empty(t, definition.CreatorID)
	require.Equal(t, 0, definition.EntryID)
	require.Equal(t, enum.Disabled, definition.Status)
	require.False(t, definition.Once)
	require.Equal(t, "health check", definition.Name)
	require.Equal(t, "https", definition.Protocol)
	require.Equal(t, "GET", definition.Method)
	require.Empty(t, definition.Python)
	require.Equal(t, models.JsonRawMessage("[]"), definition.Command)
	require.Equal(t, models.JsonRawMessage("[]"), definition.Args)

	unsafe := []models.Task{
		{Name: "bad cron", Spec: "* * *", Provider: models.TaskProviderDefault, Protocol: "https", Endpoint: "example.com", Method: "GET"},
		{Name: "script", Spec: "* * * * * *", Provider: "script", Python: "print(1)"},
		{Name: "credential URL", Spec: "* * * * * *", Provider: models.TaskProviderDefault, Protocol: "https", Endpoint: "user:pass@example.com", Method: "GET"},
		{Name: "large body", Spec: "* * * * * *", Provider: models.TaskProviderDefault, Protocol: "https", Endpoint: "example.com", Method: "POST", Body: strings.Repeat("x", maxTaskBodyBytes+1)},
	}
	for index := range unsafe {
		require.ErrorIs(t, PrepareTaskCreate(context.Background(), db, &unsafe[index]), ErrOperationalPayloadInvalid)
	}

	kubernetes := &models.Task{
		Name:      "cluster cleanup",
		Provider:  models.TaskProviderK8S,
		Spec:      "*/5 * * * *",
		Cluster:   "primary",
		Namespace: "default",
		Image:     "example.test/cleanup:1",
		Command:   `["cleanup"]`,
		Args:      `[]`,
	}
	require.NoError(t, PrepareTaskCreate(context.Background(), db, kubernetes))
	kubernetes.Spec = "0 */5 * * * *"
	require.ErrorIs(
		t,
		PrepareTaskCreate(context.Background(), db, kubernetes),
		ErrOperationalPayloadInvalid,
	)
}

func TestPrepareTaskUpdatePreservesServerStateAndDeleteRequiresDisabled(t *testing.T) {
	db := openOperationalTestDB(t)
	current := &models.Task{
		Name:     "registered",
		Provider: models.TaskProviderFunc,
		Spec:     "0 * * * * *",
		Method:   "test",
		Args:     `["before"]`,
		Command:  "[]",
		Status:   enum.Enabled,
		EntryID:  17,
	}
	current.CreatorID = "creator"
	require.NoError(t, db.Create(current).Error)

	incoming := &models.Task{
		Name:     " updated ",
		Provider: models.TaskProviderFunc,
		Spec:     "30 * * * * *",
		Method:   "test",
		Args:     `[" one ","two"]`,
		Status:   enum.Disabled,
		EntryID:  999,
	}
	incoming.ID = "attacker-id"
	incoming.CreatorID = "attacker"
	require.NoError(t, PrepareTaskUpdate(context.Background(), db, current.ID, incoming))
	require.Equal(t, current.ID, incoming.ID)
	require.Equal(t, current.CreatorID, incoming.CreatorID)
	require.Equal(t, current.EntryID, incoming.EntryID)
	require.Equal(t, current.Status, incoming.Status)
	require.Equal(t, models.JsonRawMessage(`["one","two"]`), incoming.Args)
	providerChange := *incoming
	providerChange.Provider = models.TaskProviderDefault
	providerChange.Protocol = "https"
	providerChange.Endpoint = "example.test/health"
	providerChange.Method = "GET"
	require.ErrorIs(
		t,
		PrepareTaskUpdate(context.Background(), db, current.ID, &providerChange),
		ErrOperationalConflict,
	)
	require.ErrorIs(t, ValidateTaskDelete(context.Background(), db, []string{current.ID}), ErrOperationalConflict)

	require.NoError(t, db.Model(current).Update("status", enum.Disabled).Error)
	require.NoError(t, ValidateTaskDelete(context.Background(), db, []string{current.ID}))
	require.ErrorIs(t, ValidateTaskDelete(context.Background(), db, []string{"missing"}), gorm.ErrRecordNotFound)
}

func TestNoticePreparationAlwaysUsesVerifiedOwner(t *testing.T) {
	db := openOperationalTestDB(t)
	notice := &models.Notice{
		UserID:      "victim",
		Title:       " hello ",
		Read:        true,
		Type:        models.NoticeTypeMessage,
		Description: "safe",
	}
	notice.ID = "client-id"
	require.NoError(t, PrepareNoticeCreate(context.Background(), db, "owner", notice))
	require.Empty(t, notice.ID)
	require.Equal(t, "owner", notice.UserID)
	require.False(t, notice.Read)
	require.Equal(t, "hello", notice.Title)
	require.NoError(t, db.Create(notice).Error)

	incoming := &models.Notice{
		UserID: "victim",
		Title:  "updated",
		Read:   true,
		Type:   models.NoticeTypeNotification,
	}
	incoming.ID = "attacker-id"
	require.NoError(t, PrepareNoticeUpdate(context.Background(), db, "owner", notice.ID, incoming))
	require.Equal(t, notice.ID, incoming.ID)
	require.Equal(t, "owner", incoming.UserID)
	require.False(t, incoming.Read)

	oversized := &models.Notice{Title: "large", Description: strings.Repeat("x", maxNoticeDescriptionBytes+1)}
	require.ErrorIs(t, PrepareNoticeCreate(context.Background(), db, "owner", oversized), ErrOperationalPayloadInvalid)
	require.ErrorIs(t, PrepareNoticeUpdate(context.Background(), db, "other", notice.ID, incoming), gorm.ErrRecordNotFound)
}

func TestSystemConfigPreparationValidatesFormatAndProtectsBuiltIns(t *testing.T) {
	db := openOperationalTestDB(t)
	config := &models.SystemConfig{
		Name:    " private ",
		Ext:     source.SchemeJSOM,
		Content: `{"token":"stored-only-in-detail"}`,
		BuiltIn: true,
	}
	config.ID = "client-id"
	require.NoError(t, PrepareSystemConfigCreate(context.Background(), db, config))
	require.Empty(t, config.ID)
	require.False(t, config.BuiltIn)
	require.Equal(t, "private", config.Name)
	require.NoError(t, db.Create(config).Error)

	duplicate := &models.SystemConfig{Name: "private", Ext: source.SchemeYaml, Content: "enabled: true"}
	require.ErrorIs(t, PrepareSystemConfigCreate(context.Background(), db, duplicate), ErrOperationalConflict)
	invalid := &models.SystemConfig{Name: "invalid", Ext: source.SchemeJSOM, Content: "{"}
	require.ErrorIs(t, PrepareSystemConfigCreate(context.Background(), db, invalid), ErrOperationalPayloadInvalid)

	require.NoError(t, db.Exec(
		"UPDATE mss_boot_system_configs SET built_in = ? WHERE id = ?",
		true,
		config.ID,
	).Error)
	incoming := &models.SystemConfig{
		Name:    "renamed",
		Ext:     source.SchemeYaml,
		Content: `{"enabled":true}`,
	}
	require.NoError(t, PrepareSystemConfigUpdate(context.Background(), db, config.ID, incoming))
	require.Equal(t, config.ID, incoming.ID)
	require.Equal(t, config.Name, incoming.Name)
	require.Equal(t, config.Ext, incoming.Ext)
	require.True(t, incoming.BuiltIn)
	require.ErrorIs(t, ValidateSystemConfigDelete(context.Background(), db, []string{config.ID}), ErrOperationalBuiltIn)
	require.ErrorIs(t, ValidateSystemConfigDelete(context.Background(), db, []string{"missing"}), gorm.ErrRecordNotFound)

	tooLarge := &models.SystemConfig{Name: "large", Ext: source.SchemeYaml, Content: strings.Repeat("x", maxSystemConfigBytes+1)}
	require.True(t, errors.Is(PrepareSystemConfigCreate(context.Background(), db, tooLarge), ErrOperationalPayloadInvalid))
}
