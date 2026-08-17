package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
)

func TestThemeApplicationPatchSupportsSparseFalseAndNull(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.AppConfig{
		{Group: ThemeConfigGroup, Name: "layout", Value: "mix"},
		{Group: ThemeConfigGroup, Name: "fixedHeader", Value: "true"},
		// Historical keys outside the current theme contract must survive reset
		// but must never be projected to current clients.
		{Group: ThemeConfigGroup, Name: "pwa", Value: "true"},
	}).Error)

	svc := &Theme{}
	require.NoError(t, svc.PatchApplication(env.ctx, map[string]any{
		"fixedHeader":  false,
		"colorPrimary": "#A1b2C3",
	}, 0))

	overrides, err := svc.Application(env.ctx)
	require.NoError(t, err)
	require.NotNil(t, overrides.Layout)
	require.Equal(t, "mix", *overrides.Layout, "an omitted field must remain unchanged")
	require.NotNil(t, overrides.FixedHeader)
	require.False(t, *overrides.FixedHeader, "false is an explicit override")
	require.NotNil(t, overrides.ColorPrimary)
	require.Equal(t, "#a1b2c3", *overrides.ColorPrimary)

	require.NoError(t, svc.PatchApplication(env.ctx, map[string]any{"fixedHeader": nil}, 1))
	overrides, err = svc.Application(env.ctx)
	require.NoError(t, err)
	require.Nil(t, overrides.FixedHeader, "null must remove the override")
	require.Equal(t, "mix", *overrides.Layout)

	require.NoError(t, svc.ResetApplication(env.ctx, 2))
	overrides, err = svc.Application(env.ctx)
	require.NoError(t, err)
	require.Empty(t, themeOverridesMap(overrides))
	var historical models.AppConfig
	require.NoError(t, env.db.Where(&models.AppConfig{Group: ThemeConfigGroup, Name: "pwa"}).First(&historical).Error)

	// Reset uses a hard delete so the existing unique key cannot strand a
	// soft-deleted row and prevent a later override from being recreated.
	require.NoError(t, svc.PatchApplication(env.ctx, map[string]any{"layout": "side"}, 3))
	overrides, err = svc.Application(env.ctx)
	require.NoError(t, err)
	require.Equal(t, "side", *overrides.Layout)
}

func TestThemeReadIgnoresInvalidHistoricalOverrides(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.AppConfig{
		{Group: ThemeConfigGroup, Name: "layout", Value: "mix"},
		{Group: ThemeConfigGroup, Name: "navTheme", Value: "legacy-invalid"},
		{Group: ThemeConfigGroup, Name: "fixedHeader", Value: "not-a-boolean"},
		{Group: ThemeConfigGroup, Name: "colorPrimary", Value: "#ABCDEF"},
	}).Error)

	overrides, err := (&Theme{}).Application(env.ctx)
	require.NoError(t, err)
	require.Equal(t, "mix", *overrides.Layout)
	require.Equal(t, "#abcdef", *overrides.ColorPrimary)
	require.Nil(t, overrides.NavTheme)
	require.Nil(t, overrides.FixedHeader)
}

func TestThemeCaseVariantsAreIgnoredAndPreservedByReadDeleteAndReset(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	applicationRows := []models.AppConfig{
		{Group: ThemeConfigGroup, Name: "NavTheme", Value: "realDark"},
		{Group: "Theme", Name: "layout", Value: "side"},
		{Group: ThemeConfigGroup, Name: "pwa", Value: "true"},
		{Group: ThemeConfigGroup, Name: "FixedHeader", Value: "true"},
	}
	userRows := []models.UserConfig{
		{UserID: "user-a", Group: ThemeConfigGroup, Name: "ColorWeak", Value: "true"},
		{UserID: "user-a", Group: "Theme", Name: "layout", Value: "mix"},
		{UserID: "User-A", Group: ThemeConfigGroup, Name: "navTheme", Value: "light"},
	}
	require.NoError(t, env.db.Create(&applicationRows).Error)
	require.NoError(t, env.db.Create(&userRows).Error)

	application, err := (&Theme{}).ApplicationResource(env.ctx)
	require.NoError(t, err)
	require.Empty(t, themeOverridesMap(&application.ThemeOverrides))
	user, err := (&Theme{}).UserResource(env.ctx, "user-a")
	require.NoError(t, err)
	require.Empty(t, themeOverridesMap(&user.ThemeOverrides))

	// A null patch must delete only an exact canonical key, even when the
	// database collation would compare FixedHeader/ColorWeak as equal.
	require.NoError(t, (&Theme{}).PatchApplication(env.ctx, map[string]any{"fixedHeader": nil}, 0))
	require.NoError(t, (&Theme{}).PatchUser(env.ctx, "user-a", map[string]any{"colorWeak": nil}, 0))
	require.NoError(t, (&Theme{}).ResetApplication(env.ctx, 1))
	require.NoError(t, (&Theme{}).ResetUser(env.ctx, "user-a", 1))

	for i := range applicationRows {
		var row models.AppConfig
		require.NoError(t, env.db.Unscoped().First(&row, "id = ?", applicationRows[i].ID).Error)
		require.False(t, row.DeletedAt.Valid, "non-canonical application row %q must survive", row.Name)
	}
	for i := range userRows {
		var row models.UserConfig
		require.NoError(t, env.db.Unscoped().First(&row, "id = ?", userRows[i].ID).Error)
		require.False(t, row.DeletedAt.Valid, "non-canonical user row %q must survive", row.Name)
	}
}

func TestThemePutCreatesCanonicalResourceWithoutMutatingSQLiteCaseVariant(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	applicationLegacy := &models.AppConfig{
		Group: "Theme", Name: "NavTheme", Value: "realDark", Auth: true,
	}
	userLegacy := &models.UserConfig{
		UserID: "user-a", Group: "Theme", Name: "ColorWeak", Value: "true",
	}
	require.NoError(t, env.db.Create(applicationLegacy).Error)
	require.NoError(t, env.db.Delete(applicationLegacy).Error, "soft-deleted legacy rows must also be canonicalizable")
	require.NoError(t, env.db.Create(userLegacy).Error)

	revision0 := int64(0)
	application, err := (&Theme{}).PatchApplicationResource(env.ctx, map[string]any{
		"navTheme": "light",
	}, revision0)
	require.NoError(t, err)
	require.Equal(t, "1", application.Meta.Revision)
	require.Equal(t, "light", *application.NavTheme)

	user, err := (&Theme{}).PatchUserResource(env.ctx, "user-a", map[string]any{
		"colorWeak": false,
	}, revision0)
	require.NoError(t, err)
	require.Equal(t, "1", user.Meta.Revision)
	require.NotNil(t, user.ColorWeak)
	require.False(t, *user.ColorWeak)

	var canonicalApplication models.AppConfig
	require.NoError(t, env.db.Unscoped().First(&canonicalApplication, "id = ?", applicationLegacy.ID).Error)
	require.Equal(t, "Theme", canonicalApplication.Group)
	require.Equal(t, "NavTheme", canonicalApplication.Name)
	require.True(t, canonicalApplication.DeletedAt.Valid, "case-sensitive databases keep the invalid legacy row isolated")
	var insertedApplication models.AppConfig
	require.NoError(t, env.db.First(
		&insertedApplication,
		`"group" = ? AND name = ?`,
		ThemeConfigGroup,
		"navTheme",
	).Error)
	require.NotEqual(t, applicationLegacy.ID, insertedApplication.ID)
	require.Equal(t, "light", insertedApplication.Value)
	require.False(t, insertedApplication.Auth)

	var canonicalUser models.UserConfig
	require.NoError(t, env.db.Unscoped().First(&canonicalUser, "id = ?", userLegacy.ID).Error)
	require.Equal(t, "Theme", canonicalUser.Group)
	require.Equal(t, "ColorWeak", canonicalUser.Name)
	require.Equal(t, "true", canonicalUser.Value)
	var insertedUser models.UserConfig
	require.NoError(t, env.db.First(
		&insertedUser,
		"user_id = ? AND \"group\" = ? AND name = ?",
		"user-a",
		ThemeConfigGroup,
		"colorWeak",
	).Error)
	require.NotEqual(t, userLegacy.ID, insertedUser.ID)
	require.Equal(t, "false", insertedUser.Value)

	var applicationCount int64
	require.NoError(t, env.db.Unscoped().Model(&models.AppConfig{}).
		Where("LOWER(name) = ?", "navtheme").Count(&applicationCount).Error)
	require.Equal(t, int64(2), applicationCount)
	var userCount int64
	require.NoError(t, env.db.Unscoped().Model(&models.UserConfig{}).
		Where("LOWER(user_id) = ? AND LOWER(name) = ?", "user-a", "colorweak").Count(&userCount).Error)
	require.Equal(t, int64(2), userCount)
}

func TestThemePutRejectsAmbiguousCaseVariantCollisionAndRollsBack(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Exec("DROP INDEX idx_app_config_test").Error)
	rows := []models.AppConfig{
		{Group: ThemeConfigGroup, Name: "navTheme", Value: "light"},
		{Group: ThemeConfigGroup, Name: "navTheme", Value: "realDark"},
	}
	require.NoError(t, env.db.Create(&rows).Error)

	revision0 := int64(0)
	_, err := (&Theme{}).PatchApplicationResource(env.ctx, map[string]any{
		"navTheme": "realDark",
	}, revision0)
	require.ErrorIs(t, err, ErrThemeKeyCollision)
	var collision *ThemeKeyCollisionError
	require.ErrorAs(t, err, &collision)
	require.Equal(t, ThemeScopeApplication, collision.Scope)
	require.Equal(t, "navTheme", collision.Key)
	require.Equal(t, 2, collision.Candidates)

	var persisted []models.AppConfig
	require.NoError(t, env.db.Order("name").Find(&persisted).Error)
	require.Len(t, persisted, 2)
	values := []string{persisted[0].Value, persisted[1].Value}
	require.ElementsMatch(t, []string{"light", "realDark"}, values)
	var revisionCount int64
	require.NoError(t, env.db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
	require.Zero(t, revisionCount, "collision must roll back revision rows and data changes")
}

func TestThemeKeyCollisionErrorDoesNotExposeRawOwner(t *testing.T) {
	const ownerID = "personally-identifying-user"
	collision := &ThemeKeyCollisionError{
		Scope: ThemeScopeUser, OwnerID: ownerID, Key: "navTheme", Candidates: 2,
	}

	require.ErrorIs(t, collision, ErrThemeKeyCollision)
	require.Equal(t, ownerID, collision.OwnerID, "structured owner remains available to trusted callers")
	require.NotContains(t, collision.Error(), ownerID)
	require.Contains(t, collision.Error(), "ownerPresent=true")
}

func TestUserThemePutKeepsSQLiteCaseVariantOwnerIsolated(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	legacy := &models.UserConfig{
		UserID: "User-A", Group: ThemeConfigGroup, Name: "navTheme", Value: "light",
	}
	require.NoError(t, env.db.Create(legacy).Error)

	revision0 := int64(0)
	resource, err := (&Theme{}).PatchUserResource(env.ctx, "user-a", map[string]any{
		"navTheme": "realDark",
	}, revision0)
	require.NoError(t, err)
	require.Equal(t, "realDark", *resource.NavTheme)

	var persisted models.UserConfig
	require.NoError(t, env.db.First(&persisted, "id = ?", legacy.ID).Error)
	require.Equal(t, "User-A", persisted.UserID)
	require.Equal(t, "light", persisted.Value)
	var canonical models.UserConfig
	require.NoError(t, env.db.First(
		&canonical,
		"user_id = ? AND \"group\" = ? AND name = ?",
		"user-a",
		ThemeConfigGroup,
		"navTheme",
	).Error)
	require.Equal(t, "realDark", canonical.Value)
}

func TestThemeUserScopeIsOwnedAndResetIndependently(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	svc := &Theme{}

	require.NoError(t, svc.PatchUser(env.ctx, "user-a", map[string]any{
		"navTheme":    "realDark",
		"fixSiderbar": false,
	}, 0))
	require.NoError(t, svc.PatchUser(env.ctx, "user-b", map[string]any{
		"navTheme": "light",
	}, 0))

	userA, err := svc.User(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "realDark", *userA.NavTheme)
	require.NotNil(t, userA.FixSiderbar)
	require.False(t, *userA.FixSiderbar)
	userB, err := svc.User(env.ctx, "user-b")
	require.NoError(t, err)
	require.Equal(t, "light", *userB.NavTheme)

	require.NoError(t, svc.ResetUser(env.ctx, "user-a", 1))
	userA, err = svc.User(env.ctx, "user-a")
	require.NoError(t, err)
	require.Empty(t, themeOverridesMap(userA))
	userB, err = svc.User(env.ctx, "user-b")
	require.NoError(t, err)
	require.Equal(t, "light", *userB.NavTheme)

	require.ErrorIs(t, svc.PatchUser(env.ctx, "", map[string]any{"layout": "mix"}, 0), ErrThemeUserRequired)
	require.ErrorIs(t, svc.ResetUser(env.ctx, "", 0), ErrThemeUserRequired)
}

func TestThemePatchRejectsUnknownAndIllTypedValuesBeforeWriting(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	svc := &Theme{}

	tests := []struct {
		name  string
		patch map[string]any
	}{
		{name: "unknown field", patch: map[string]any{"unknownThemeSetting": true}},
		{name: "legacy pwa invalid type", patch: map[string]any{"pwa": "true"}},
		{name: "invalid nav theme", patch: map[string]any{"navTheme": "dark"}},
		{name: "invalid layout", patch: map[string]any{"layout": "middle"}},
		{name: "invalid content width", patch: map[string]any{"contentWidth": "fluid"}},
		{name: "boolean encoded as string", patch: map[string]any{"fixedHeader": "false"}},
		{name: "invalid color", patch: map[string]any{"colorPrimary": "rgb(0,0,0)"}},
		{name: "null patch object", patch: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := svc.PatchApplication(env.ctx, test.patch, 0)
			require.ErrorIs(t, err, ErrInvalidThemePatch)
		})
	}

	var count int64
	require.NoError(t, env.db.Model(&models.AppConfig{}).
		Where(&models.AppConfig{Group: ThemeConfigGroup}).Count(&count).Error)
	require.Zero(t, count, "validation must finish before the transaction writes anything")
}

func TestUserThemeRejectsLegacyPWAWithoutWritingOrAdvancingRevision(t *testing.T) {
	env := setupAppConfigTestEnv(t)

	_, err := (&Theme{}).PatchUserResource(env.ctx, "user-a", map[string]any{
		"pwa": true,
	}, 0)
	require.ErrorIs(t, err, ErrInvalidThemePatch)

	var configCount int64
	require.NoError(t, env.db.Model(&models.UserConfig{}).Count(&configCount).Error)
	require.Zero(t, configCount)
	var revisionCount int64
	require.NoError(t, env.db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
	require.Zero(t, revisionCount)
}

func TestReservedThemeAndPublicKeyVariantsCannotPoisonEmptyDatabase(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	app := &AppConfig{}
	user := &UserConfig{}

	for _, group := range []string{"Theme", "thème", "theme "} {
		err := app.CreateOrUpdate(env.ctx, group, map[string]any{"layout": "side"})
		require.Error(t, err, "application group %q must be rejected", group)
		err = user.CreateOrUpdate(env.ctx, "user-a", group, map[string]any{"layout": "side"})
		require.Error(t, err, "user group %q must be rejected", group)
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
		err := app.CreateOrUpdate(env.ctx, test.group, map[string]any{test.name: "poison"})
		require.Error(t, err, "%s/%s must be rejected", test.group, test.name)
	}

	var appCount int64
	require.NoError(t, env.db.Model(&models.AppConfig{}).Count(&appCount).Error)
	require.Zero(t, appCount)
	var userCount int64
	require.NoError(t, env.db.Model(&models.UserConfig{}).Count(&userCount).Error)
	require.Zero(t, userCount)
	var revisionCount int64
	require.NoError(t, env.db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
	require.Zero(t, revisionCount)

	// Rejected variants must not poison the subsequent canonical writes.
	_, err := (&Theme{}).PatchApplicationResource(env.ctx, map[string]any{"layout": "side"}, 0)
	require.NoError(t, err)
	require.NoError(t, app.CreateOrUpdate(env.ctx, "base", map[string]any{"websiteName": "canonical"}))
	_, err = (&Theme{}).PatchUserResource(env.ctx, "user-a", map[string]any{"layout": "mix"}, 0)
	require.NoError(t, err)
}

func TestThemeGroupVariantReadsAreRejected(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, (&Theme{}).PatchApplication(env.ctx, map[string]any{"layout": "side"}, 0))
	require.NoError(t, (&Theme{}).PatchUser(env.ctx, "user-a", map[string]any{"layout": "mix"}, 0))

	for _, group := range []string{"Theme", "thème", "theme "} {
		_, err := (&AppConfig{}).Group(env.ctx, group)
		require.Error(t, err, "application group %q must not read canonical theme rows", group)
		_, err = (&UserConfig{}).Group(env.ctx, "user-a", group)
		require.Error(t, err, "user group %q must not read canonical theme rows", group)
	}
}

func TestNonReservedUnicodeConfigIdentifiersRemainSupported(t *testing.T) {
	env := setupAppConfigTestEnv(t)

	require.NoError(t, (&AppConfig{}).CreateOrUpdate(env.ctx, "自定义", map[string]any{
		"显示名称": "应用值",
	}))
	application, err := (&AppConfig{}).Group(env.ctx, "自定义")
	require.NoError(t, err)
	require.Equal(t, "应用值", application["显示名称"])

	require.NoError(t, (&UserConfig{}).CreateOrUpdate(env.ctx, "user-a", "偏好", map[string]any{
		"首页": "工作台",
	}))
	user, err := (&UserConfig{}).Group(env.ctx, "user-a", "偏好")
	require.NoError(t, err)
	require.Equal(t, "工作台", user["首页"])
}

func TestUserConfigModelReadKeepsEmptyGroupInPredicate(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Create([]*models.UserConfig{
		{UserID: "user-a", Group: "", Name: "landing", Value: "ungrouped"},
		{UserID: "user-a", Group: "theme", Name: "landing", Value: "grouped"},
	}).Error)

	value, ok := (&models.UserConfig{}).GetUserConfig(env.ctx, "user-a", "landing")
	require.True(t, ok)
	require.Equal(t, "ungrouped", value)
}

func TestThemeApplicationPatchRollsBackOnDatabaseFailure(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	require.NoError(t, env.db.Exec(`
		CREATE TRIGGER fail_theme_layout_insert
		BEFORE INSERT ON mss_boot_app_configs
		WHEN NEW.name = 'layout'
		BEGIN
			SELECT RAISE(ABORT, 'forced theme write failure');
		END;
	`).Error)

	err := (&Theme{}).PatchApplication(env.ctx, map[string]any{
		"colorPrimary": "#123456",
		"layout":       "side",
	}, 0)
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrInvalidThemePatch), "database failures are server errors, not validation errors")

	var count int64
	require.NoError(t, env.db.Model(&models.AppConfig{}).
		Where(&models.AppConfig{Group: ThemeConfigGroup}).Count(&count).Error)
	require.Zero(t, count, "the earlier key in the sorted patch must roll back")
	require.NoError(t, env.db.Model(&models.ConfigRevision{}).Count(&count).Error)
	require.Zero(t, count, "revision rows must roll back with the failed theme write")
}

func TestThemeResourcesAdvanceScopedRevisionsAndRejectStaleWriters(t *testing.T) {
	env := setupAppConfigTestEnv(t)
	svc := &Theme{}

	application, err := svc.ApplicationResource(env.ctx)
	require.NoError(t, err)
	require.Equal(t, ThemeScopeApplication, application.Meta.Scope)
	require.Equal(t, "0", application.Meta.Revision)

	revision0 := int64(0)
	application, err = svc.PatchApplicationResource(env.ctx, map[string]any{
		"colorPrimary": "#A1B2C3",
	}, revision0)
	require.NoError(t, err)
	require.Equal(t, "1", application.Meta.Revision)
	require.Equal(t, "#a1b2c3", *application.ColorPrimary)

	_, err = svc.PatchApplicationResource(env.ctx, map[string]any{
		"layout": "side",
	}, revision0)
	require.ErrorIs(t, err, ErrThemeRevisionConflict)
	var conflict *ThemeRevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, int64(0), conflict.Expected)
	require.Equal(t, int64(1), conflict.Actual)
	require.Equal(t, "1", conflict.Current.Meta.Revision)
	require.Nil(t, conflict.Current.Layout, "the stale mutation must be rolled back")
	require.Equal(t, "#a1b2c3", *conflict.Current.ColorPrimary)

	// A writer holding the current strong revision advances the resource.
	application, err = svc.PatchApplicationResource(env.ctx, map[string]any{
		"layout": "mix",
	}, 1)
	require.NoError(t, err)
	require.Equal(t, "2", application.Meta.Revision)

	revision2 := int64(2)
	application, err = svc.ResetApplicationResource(env.ctx, revision2)
	require.NoError(t, err)
	require.Equal(t, "3", application.Meta.Revision)
	require.Empty(t, themeOverridesMap(&application.ThemeOverrides))

	userA, err := svc.PatchUserResource(env.ctx, "user-a", map[string]any{
		"navTheme": "realDark",
	}, revision0)
	require.NoError(t, err)
	require.Equal(t, ThemeScopeUser, userA.Meta.Scope)
	require.Equal(t, "1", userA.Meta.Revision)

	userB, err := svc.PatchUserResource(env.ctx, "user-b", map[string]any{
		"navTheme": "light",
	}, revision0)
	require.NoError(t, err)
	require.Equal(t, "1", userB.Meta.Revision, "user revisions must be isolated by owner")

	_, err = svc.ResetUserResource(env.ctx, "user-a", revision0)
	require.ErrorIs(t, err, ErrThemeRevisionConflict)
	userA, err = svc.UserResource(env.ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "1", userA.Meta.Revision)
	require.Equal(t, "realDark", *userA.NavTheme)

	var rows []models.ConfigRevision
	require.NoError(t, env.db.Order("scope, owner_id, resource").Find(&rows).Error)
	revisions := make(map[string]int64, len(rows))
	for _, row := range rows {
		key := row.Scope + "/" + row.OwnerID + "/" + row.Resource
		revisions[key] = row.Revision
	}
	require.Equal(t, int64(3), revisions["application//theme"])
	require.Equal(t, int64(3), revisions["application//public-profile"])
	require.Equal(t, int64(1), revisions["user/user-a/theme"])
	require.Equal(t, int64(1), revisions["user/user-b/theme"])
}

func TestGenericConfigServicesRejectThemeWrites(t *testing.T) {
	env := setupAppConfigTestEnv(t)

	require.ErrorIs(t, (&AppConfig{}).CreateOrUpdate(env.ctx, ThemeConfigGroup, map[string]any{
		"fixedHeader": false,
	}), ErrThemeGroupOnly)
	require.ErrorIs(t, (&UserConfig{}).CreateOrUpdate(env.ctx, "user-a", ThemeConfigGroup, map[string]any{
		"colorWeak": false,
	}), ErrThemeGroupOnly)

	application, err := (&AppConfig{}).Group(env.ctx, ThemeConfigGroup)
	require.NoError(t, err)
	require.Equal(t, "0", requireThemeProfileMeta(t, application["_meta"])["revision"])
	user, err := (&UserConfig{}).Group(env.ctx, "user-a", ThemeConfigGroup)
	require.NoError(t, err)
	require.Equal(t, "0", requireThemeProfileMeta(t, user["_meta"])["revision"])
}
