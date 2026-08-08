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
		{Group: "security", Name: "githubClientSecret", Value: "github-secret", Auth: false},
		{Group: "security", Name: "larkAppSecret", Value: "lark-secret", Auth: true},
		{Group: "security", Name: "githubClientId", Value: "client-id", Auth: true},
		{Group: "storage", Name: "s3SecretAccessKey", Value: "storage-secret", Auth: false},
		{Group: "storage", Name: "s3Bucket", Value: "bucket", Auth: true},
	}).Error)

	tests := []struct {
		group     string
		secretKey string
		visible   string
	}{
		{group: "email", secretKey: "password", visible: "smtpHost"},
		{group: "security", secretKey: "githubClientSecret", visible: "githubClientId"},
		{group: "security", secretKey: "larkAppSecret", visible: "githubClientId"},
		{group: "storage", secretKey: "s3SecretAccessKey", visible: "s3Bucket"},
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
				"password":           "smtp-secret",
				"githubClientSecret": "github-secret",
				"larkAppSecret":      "lark-secret",
				"s3SecretAccessKey":  "storage-secret",
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
		{group: "security", name: "githubClientSecret"},
		{group: "security", name: "larkAppSecret"},
		{group: "storage", name: "s3SecretAccessKey"},
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
	require.False(t, AppConfigMutationContainsSensitiveValues(
		"security",
		map[string]any{"githubClientId": "public-id"},
	))

	env := setupAppConfigTestEnv(t)
	err := (&AppConfig{}).CreateOrUpdate(
		env.ctx,
		"security",
		map[string]any{"GithubClientSecret": "case-bypass"},
	)
	require.ErrorIs(t, err, ErrAppConfigKeyCaseMismatch)
	err = (&AppConfig{}).CreateOrUpdate(
		env.ctx,
		"Security",
		map[string]any{"githubClientSecret": "group-case-bypass"},
	)
	require.ErrorIs(t, err, ErrAppConfigKeyCaseMismatch)

	var count int64
	require.NoError(t, env.db.Model(&models.AppConfig{}).Count(&count).Error)
	require.Zero(t, count)
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
