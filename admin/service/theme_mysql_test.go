package service

import (
	"context"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
)

const themeMySQLTestDSNEnv = "MSS_THEME_TEST_MYSQL_DSN"

// TestThemeMySQLAccentInsensitiveUpgradePath is opt-in because it requires a
// disposable MySQL database. Every data mutation runs inside an outer
// transaction that is rolled back; schema creation may remain in that test
// database. The DSN database must use a utf8mb4 accent-insensitive collation.
func TestThemeMySQLAccentInsensitiveUpgradePath(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(themeMySQLTestDSNEnv))
	if dsn == "" {
		t.Skip(themeMySQLTestDSNEnv + " is not set")
	}

	handle, err := (&gormdb.Database{
		Driver:       "mysql",
		Source:       dsn,
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	}).Open(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, handle.Close()) })

	db := handle.DB.Set(
		"gorm:table_options",
		"ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
	)
	require.NoError(t, db.AutoMigrate(
		&models.AppConfig{},
		&models.UserConfig{},
		&models.ConfigRevision{},
	))
	requireMySQLAccentInsensitiveColumn(t, db, (&models.AppConfig{}).TableName(), "group")
	requireMySQLAccentInsensitiveColumn(t, db, (&models.AppConfig{}).TableName(), "name")
	requireMySQLAccentInsensitiveColumn(t, db, (&models.UserConfig{}).TableName(), "user_id")

	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { _ = tx.Rollback().Error })
	cleanDB := func() *gorm.DB {
		return tx.Session(&gorm.Session{NewDB: true})
	}
	for _, table := range []string{
		(&models.ConfigRevision{}).TableName(),
		(&models.UserConfig{}).TableName(),
		(&models.AppConfig{}).TableName(),
	} {
		require.NoError(t, tx.Exec("DELETE FROM "+table).Error)
	}

	previousTenant := center.GetTenant()
	previousCache := center.GetCache()
	previousAppConfig := center.GetAppConfig()
	// Keep the service handle on the same SQL transaction while clearing any
	// GORM statement state accumulated by the test's setup/count queries.
	serviceDB := cleanDB()
	center.SetTenant(&appConfigTestTenant{db: serviceDB})
	center.SetCache(nil)
	center.SetAppConfig(&models.AppConfig{})
	t.Cleanup(func() {
		center.SetTenant(previousTenant)
		center.SetCache(previousCache)
		center.SetAppConfig(previousAppConfig)
	})
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	app := &AppConfig{}
	user := &UserConfig{}
	for _, group := range []string{"Theme", "thème", "theme "} {
		require.ErrorIs(t, app.CreateOrUpdate(ctx, group, map[string]any{"layout": "side"}), ErrThemeGroupCaseMismatch)
		require.ErrorIs(t, user.CreateOrUpdate(ctx, "user-a", group, map[string]any{"layout": "side"}), ErrThemeGroupCaseMismatch)
	}
	for _, test := range []struct {
		group string
		name  string
	}{
		{group: "Base", name: "websiteName"},
		{group: "báse", name: "websiteName"},
		{group: "base ", name: "websiteName"},
		{group: "base", name: "WebsiteName"},
		{group: "base", name: "wéBSITEName"},
		{group: "base", name: "websiteName "},
	} {
		require.ErrorIs(
			t,
			app.CreateOrUpdate(ctx, test.group, map[string]any{test.name: "poison"}),
			ErrAppConfigKeyCaseMismatch,
		)
	}
	var count int64
	require.NoError(t, cleanDB().Model(&models.AppConfig{}).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, cleanDB().Model(&models.UserConfig{}).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, cleanDB().Model(&models.ConfigRevision{}).Count(&count).Error)
	require.Zero(t, count)

	// Rejected aliases cannot poison the first canonical writes.
	_, err = (&Theme{}).PatchApplicationResource(ctx, map[string]any{"layout": "side"}, 0)
	require.NoError(t, err)
	require.NoError(t, app.CreateOrUpdate(ctx, "base", map[string]any{"websiteName": "canonical"}))
	_, err = (&Theme{}).PatchUserResource(ctx, "user-a", map[string]any{"layout": "mix"}, 0)
	require.NoError(t, err)

	applicationAliases := []models.AppConfig{
		{Group: "Theme", Name: "NavTheme", Value: "light", Auth: true},
		{Group: "thème", Name: "contentWidth", Value: "Fluid", Auth: true},
		{Group: "theme ", Name: "colorWeak", Value: "true", Auth: true},
		{Group: ThemeConfigGroup, Name: "fixedHéader", Value: "true", Auth: true},
		{Group: ThemeConfigGroup, Name: "fixSiderbar ", Value: "true", Auth: true},
		{Group: ThemeConfigGroup, Name: "colorPrímary", Value: "#112233", Auth: true},
	}
	require.NoError(t, cleanDB().Create(&applicationAliases).Error)
	resource, err := (&Theme{}).PatchApplicationResource(ctx, map[string]any{
		"navTheme":     "realDark",
		"contentWidth": "Fixed",
		"colorWeak":    false,
		"fixedHeader":  false,
		"fixSiderbar":  false,
		"colorPrimary": "#A1B2C3",
	}, 1)
	require.NoError(t, err)
	require.Equal(t, "realDark", *resource.NavTheme)
	require.Equal(t, "Fixed", *resource.ContentWidth)
	require.False(t, *resource.ColorWeak)
	require.False(t, *resource.FixedHeader)
	require.False(t, *resource.FixSiderbar)
	require.Equal(t, "#a1b2c3", *resource.ColorPrimary)
	for i, name := range []string{
		"navTheme", "contentWidth", "colorWeak", "fixedHeader", "fixSiderbar", "colorPrimary",
	} {
		var persisted models.AppConfig
		require.NoError(t, cleanDB().Unscoped().First(&persisted, "id = ?", applicationAliases[i].ID).Error)
		require.Equal(t, ThemeConfigGroup, persisted.Group)
		require.Equal(t, name, persisted.Name)
		require.False(t, persisted.Auth)
	}

	publicAliases := []models.AppConfig{
		{Group: "Base", Name: "WebsiteDescription", Value: "old", Auth: true},
		{Group: "báse", Name: "websiteLogo ", Value: "old-logo", Auth: true},
	}
	require.NoError(t, cleanDB().Create(&publicAliases).Error)
	require.NoError(t, app.CreateOrUpdate(ctx, "base", map[string]any{
		"websiteDescription": "canonical description",
		"websiteLogo":        "canonical logo",
	}))
	for i, name := range []string{"websiteDescription", "websiteLogo"} {
		var persisted models.AppConfig
		require.NoError(t, cleanDB().Unscoped().First(&persisted, "id = ?", publicAliases[i].ID).Error)
		require.Equal(t, "base", persisted.Group)
		require.Equal(t, name, persisted.Name)
		require.False(t, persisted.Auth)
	}

	theme, err := app.Group(ctx, ThemeConfigGroup)
	require.NoError(t, err)
	require.Equal(t, "realDark", theme["navTheme"])
	profile, err := app.Profile(ctx)
	require.NoError(t, err)
	require.Equal(t, "canonical", profile["base"]["websiteName"])
	require.Equal(t, "canonical description", profile["base"]["websiteDescription"])
	require.Equal(t, "canonical logo", profile["base"]["websiteLogo"])
	require.Equal(t, "realDark", profile[ThemeConfigGroup]["navTheme"])

	userAlias := &models.UserConfig{
		UserID: "user-a", Group: "thème", Name: "NavTheme", Value: "light",
	}
	require.NoError(t, cleanDB().Create(userAlias).Error)
	userResource, err := (&Theme{}).PatchUserResource(ctx, "user-a", map[string]any{
		"navTheme": "realDark",
	}, 1)
	require.NoError(t, err)
	require.Equal(t, "realDark", *userResource.NavTheme)
	var persistedUserAlias models.UserConfig
	require.NoError(t, cleanDB().Unscoped().First(&persistedUserAlias, "id = ?", userAlias.ID).Error)
	require.Equal(t, "user-a", persistedUserAlias.UserID)
	require.Equal(t, ThemeConfigGroup, persistedUserAlias.Group)
	require.Equal(t, "navTheme", persistedUserAlias.Name)

	crossOwner := &models.UserConfig{
		UserID: "User-A", Group: "thème", Name: "ColorWeak", Value: "true",
	}
	require.NoError(t, cleanDB().Create(crossOwner).Error)
	revisionBefore, err := strconv.ParseInt(userResource.Meta.Revision, 10, 64)
	require.NoError(t, err)
	_, err = (&Theme{}).PatchUserResource(ctx, "user-a", map[string]any{"colorWeak": false}, revisionBefore)
	require.ErrorIs(t, err, ErrThemeKeyCollision)
	userResource, err = (&Theme{}).UserResource(ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, strconv.FormatInt(revisionBefore, 10), userResource.Meta.Revision)
	require.Nil(t, userResource.ColorWeak)
	var persistedCrossOwner models.UserConfig
	require.NoError(t, cleanDB().First(&persistedCrossOwner, "id = ?", crossOwner.ID).Error)
	require.Equal(t, "User-A", persistedCrossOwner.UserID)
	require.Equal(t, "thème", persistedCrossOwner.Group)
	require.Equal(t, "ColorWeak", persistedCrossOwner.Name)
	require.Equal(t, "true", persistedCrossOwner.Value)
}

func requireMySQLAccentInsensitiveColumn(t *testing.T, db *gorm.DB, table, column string) {
	t.Helper()
	var collation string
	require.NoError(t, db.Raw(`
		SELECT COLLATION_NAME
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?
	`, table, column).Scan(&collation).Error)
	require.Contains(t, strings.ToLower(collation), "_ai_ci", "%s.%s must use an accent-insensitive collation", table, column)
}
