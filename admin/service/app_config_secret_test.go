package service

import (
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/stretchr/testify/require"
)

func TestAppConfigGroupOmitsFixedSensitiveValuesByDefault(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.AppConfig{
		{Group: "email", Name: "password", Value: "smtp-secret", Auth: true},
		{Group: "email", Name: "smtpHost", Value: "smtp.example.test", Auth: true},
		{Group: "security", Name: "githubBrowserSessionClientSecret", Value: "github-browser-secret", Auth: false},
		{Group: "security", Name: "larkBrowserSessionAppSecret", Value: "lark-browser-secret", Auth: false},
		{Group: "security", Name: "githubBrowserSessionClientId", Value: "client-id", Auth: true},
	}).Error)

	tests := []struct {
		group     string
		secretKey string
		visible   string
	}{
		{group: "email", secretKey: "password", visible: "smtpHost"},
		{group: "security", secretKey: "githubBrowserSessionClientSecret", visible: "githubBrowserSessionClientId"},
		{group: "security", secretKey: "larkBrowserSessionAppSecret", visible: "githubBrowserSessionClientId"},
	}

	svc := &AppConfig{}
	for _, test := range tests {
		t.Run(test.group+"/"+test.secretKey, func(t *testing.T) {
			leastPrivilege, err := svc.Group(env.ctx, test.group)
			require.NoError(t, err)
			require.NotContains(t, leastPrivilege, test.secretKey)
			require.Contains(t, leastPrivilege, test.visible)

			privileged, err := svc.GroupWithSensitiveValues(env.ctx, test.group, true)
			require.NoError(t, err)
			require.Equal(t, map[string]string{
				"password":                         "smtp-secret",
				"githubBrowserSessionClientSecret": "github-browser-secret",
				"larkBrowserSessionAppSecret":      "lark-browser-secret",
			}[test.secretKey], privileged[test.secretKey])
		})
	}
}

func TestAppConfigSensitiveKeyContractIsExactAndRejectsCaseBypass(t *testing.T) {
	tests := []struct {
		group string
		name  string
	}{
		{group: "email", name: "password"},
		{group: "security", name: "githubBrowserSessionClientSecret"},
		{group: "security", name: "larkBrowserSessionAppSecret"},
	}
	for _, test := range tests {
		t.Run(test.group+"/"+test.name, func(t *testing.T) {
			require.True(t, AppConfigGroupContainsSensitiveValues(test.group))
			require.True(t, AppConfigMutationContainsSensitiveValues(
				test.group,
				map[string]any{test.name: "secret"},
			))
		})
	}

	require.False(t, AppConfigGroupContainsSensitiveValues("base"))
	require.False(t, AppConfigGroupContainsSensitiveValues("storage"))
	require.False(t, AppConfigMutationContainsSensitiveValues(
		"storage",
		map[string]any{"s3SecretAccessKey": "removed-from-app-config"},
	))
	require.False(t, AppConfigMutationContainsSensitiveValues(
		"security",
		map[string]any{"githubBrowserSessionClientId": "public-id"},
	))

	env := setupAppConfigTestEnv(t)
	err := (&AppConfig{}).CreateOrUpdate(
		env.ctx,
		"security",
		map[string]any{"GithubBrowserSessionClientSecret": "case-bypass"},
	)
	require.ErrorIs(t, err, ErrAppConfigKeyCaseMismatch)
	err = (&AppConfig{}).CreateOrUpdate(
		env.ctx,
		"Security",
		map[string]any{"githubBrowserSessionClientSecret": "group-case-bypass"},
	)
	require.ErrorIs(t, err, ErrAppConfigKeyCaseMismatch)

	var count int64
	require.NoError(t, env.db.Model(&models.AppConfig{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestAppConfigRejectsRetiredBrowserOAuthKeys(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	for _, name := range []string{
		"githubClientId",
		"githubClientSecret",
		"githubRedirectURI",
		"githubRedirectUrl",
		"githubRedirectURL",
		"githubScope",
		"larkAppId",
		"larkAppSecret",
		"larkRedirectURI",
		"larkRedirectUrl",
	} {
		err := (&AppConfig{}).CreateOrUpdate(env.ctx, "security", map[string]any{name: "retired"})
		require.ErrorIs(t, err, ErrAppConfigKeyNotAllowed, name)
	}

	var count int64
	require.NoError(t, env.db.Model(&models.AppConfig{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestStorageAppConfigAllowsOnlyUploadAdmissionPolicy(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.AppConfig{
		{Group: "storage", Name: "maxSize", Value: "1024", Auth: true},
		{Group: "storage", Name: "allowedTypes", Value: "image/png,image/*", Auth: true},
		{Group: "storage", Name: "type", Value: "s3", Auth: true},
		{Group: "storage", Name: "endpoint", Value: "https://legacy.invalid", Auth: true},
		{Group: "storage", Name: "s3Bucket", Value: "legacy-bucket", Auth: true},
		{Group: "storage", Name: "s3AccessKeyID", Value: "legacy-access", Auth: true},
		{Group: "storage", Name: "s3SecretAccessKey", Value: "legacy-secret", Auth: false},
	}).Error)

	svc := &AppConfig{}
	for _, includeSensitive := range []bool{false, true} {
		projection, err := svc.GroupWithSensitiveValues(env.ctx, "storage", includeSensitive)
		require.NoError(t, err)
		require.Equal(t, map[string]any{
			"allowedTypes": "image/png,image/*",
			"maxSize":      "1024",
		}, projection)
	}

	require.NoError(t, svc.CreateOrUpdate(env.ctx, "storage", map[string]any{
		"allowedTypes": "image/png",
		"maxSize":      "2048",
	}))
	projection, err := svc.Group(env.ctx, "storage")
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"allowedTypes": "image/png",
		"maxSize":      "2048",
	}, projection)

	removedKeys := []string{
		"type",
		"endpoint",
		"s3Provider",
		"s3Endpoint",
		"s3Region",
		"s3Bucket",
		"s3AccessKeyID",
		"s3SecretAccessKey",
		"s3SigningMethod",
	}
	for _, name := range removedKeys {
		t.Run("rejects/"+name, func(t *testing.T) {
			err := svc.CreateOrUpdate(env.ctx, "storage", map[string]any{
				"maxSize": "4096",
				name:      "removed-from-app-config",
			})
			require.ErrorIs(t, err, ErrAppConfigKeyNotAllowed)
			var typed *AppConfigKeyNotAllowedError
			require.ErrorAs(t, err, &typed)
			require.Equal(t, "storage", typed.Group)
			require.Equal(t, name, typed.Name)

			current, readErr := svc.Group(env.ctx, "storage")
			require.NoError(t, readErr)
			require.Equal(t, "2048", current["maxSize"])
		})
	}
}

func TestAppConfigGroupOmitsHistoricalSensitiveKeyCasingAliases(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create(&models.AppConfig{
		Group: "security", Name: "GithubClientSecret", Value: "legacy-secret", Auth: false,
	}).Error)

	group, err := (&AppConfig{}).Group(env.ctx, "security")
	require.NoError(t, err)
	require.NotContains(t, group, "GithubClientSecret")
}
