package system

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	configgormdb "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
)

const (
	themeSettingsMigrationMySQLDSNEnv      = "MSS_THEME_SETTINGS_TEST_MYSQL_DSN"
	themeSettingsMigrationPostgresDSNEnv   = "MSS_THEME_SETTINGS_TEST_POSTGRES_DSN"
	themeSettingsMigrationTestDatabaseName = "mss_theme_settings_test"
	themeSettingsMigrationVersion          = "20260807150000"
)

func TestIsThemeSettingsMigrationTestDatabaseName(t *testing.T) {
	tests := []struct {
		name         string
		databaseName string
		want         bool
	}{
		{name: "exact allowlisted name", databaseName: "mss_theme_settings_test", want: true},
		{name: "shared theme test database", databaseName: "mss_theme_test", want: false},
		{name: "production prefix", databaseName: "prod_mss_theme_settings_test", want: false},
		{name: "archive suffix", databaseName: "mss_theme_settings_test_archive", want: false},
		{name: "case variant", databaseName: "MSS_THEME_SETTINGS_TEST", want: false},
		{name: "surrounding whitespace", databaseName: " mss_theme_settings_test ", want: false},
		{name: "empty", databaseName: "", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isThemeSettingsMigrationTestDatabaseName(test.databaseName); got != test.want {
				t.Fatalf(
					"isThemeSettingsMigrationTestDatabaseName(%q) = %t, want %t",
					test.databaseName,
					got,
					test.want,
				)
			}
		})
	}
}

func isThemeSettingsMigrationTestDatabaseName(databaseName string) bool {
	return databaseName == themeSettingsMigrationTestDatabaseName
}

// The integration tests are opt-in and destructive inside one disposable
// database. openThemeSettingsMigrationIntegrationDB verifies the exact
// allowlisted database name before any table is dropped.
func TestThemeSettingsMigrationMySQLIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(themeSettingsMigrationMySQLDSNEnv))
	if dsn == "" {
		t.Skip(themeSettingsMigrationMySQLDSNEnv + " is not set")
	}

	db := openThemeSettingsMigrationIntegrationDB(t, "mysql", dsn)
	runThemeSettingsMigrationIntegrationContract(t, db)
}

func TestThemeSettingsMigrationPostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(themeSettingsMigrationPostgresDSNEnv))
	if dsn == "" {
		t.Skip(themeSettingsMigrationPostgresDSNEnv + " is not set")
	}

	db := openThemeSettingsMigrationIntegrationDB(t, "postgres", dsn)
	runThemeSettingsMigrationIntegrationContract(t, db)
}

func openThemeSettingsMigrationIntegrationDB(t *testing.T, dialect, dsn string) *gorm.DB {
	t.Helper()
	dialector, ok := configgormdb.Opens[dialect]
	if !ok {
		t.Fatalf("theme settings migration integration dialect %q is unavailable", dialect)
	}

	db, err := gorm.Open(dialector(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open %s theme settings migration integration database: %v", dialect, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("obtain %s theme settings migration integration connection: %v", dialect, err)
	}
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	db = db.WithContext(ctx)
	if err := sqlDB.PingContext(ctx); err != nil {
		cancel()
		_ = sqlDB.Close()
		t.Fatalf("ping %s theme settings migration integration database: %v", dialect, err)
	}

	databaseNameQuery := "SELECT current_database()"
	if dialect == "mysql" {
		databaseNameQuery = "SELECT DATABASE()"
	}
	var databaseName string
	if err := db.Raw(databaseNameQuery).Scan(&databaseName).Error; err != nil {
		cancel()
		_ = sqlDB.Close()
		t.Fatalf("verify %s theme settings migration integration database: %v", dialect, err)
	}
	if !isThemeSettingsMigrationTestDatabaseName(databaseName) {
		cancel()
		_ = sqlDB.Close()
		t.Fatalf(
			"refusing destructive %s theme settings migration test outside the allowlisted database %q",
			dialect,
			themeSettingsMigrationTestDatabaseName,
		)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		if err := dropThemeSettingsMigrationIntegrationTables(db.WithContext(cleanupCtx)); err != nil {
			t.Errorf("clean up %s theme settings migration integration tables: %v", dialect, err)
		}
		cancel()
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close %s theme settings migration integration database: %v", dialect, err)
		}
	})
	resetThemeSettingsMigrationIntegrationTables(t, db)
	return db
}

func runThemeSettingsMigrationIntegrationContract(t *testing.T, db *gorm.DB) {
	t.Helper()

	t.Run("fresh schema and idempotent runtime", func(t *testing.T) {
		resetThemeSettingsMigrationIntegrationTables(t, db)
		createThemeSettingsConfigurationTables(t, db)
		appSchemaBefore := themeSettingsTableSchema(t, db, &models.AppConfig{})
		userSchemaBefore := themeSettingsTableSchema(t, db, &models.UserConfig{})

		for attempt := 1; attempt <= 2; attempt++ {
			if err := migrateConfigRevisions(db, themeSettingsMigrationVersion); err != nil {
				t.Fatalf("fresh config revision migration attempt %d: %v", attempt, err)
			}
		}

		assertThemeSettingsConfigurationSchemaUnchanged(
			t,
			db,
			appSchemaBefore,
			userSchemaBefore,
		)
		assertThemeSettingsRevisionSchema(t, db)
		assertThemeSettingsMigrationVersionCount(t, db, 1)

		withThemeSettingsServiceDB(t, db, func(ctx *gin.Context) {
			zero := int64(0)
			application, err := (&service.Theme{}).PatchApplicationResource(ctx, map[string]any{
				"colorPrimary": "#ABCDEF",
				"fixedHeader":  false,
			}, &zero)
			if err != nil {
				t.Fatalf("write fresh application theme: %v", err)
			}
			if application.Meta.Revision != "1" || application.ColorPrimary == nil ||
				*application.ColorPrimary != "#abcdef" || application.FixedHeader == nil ||
				*application.FixedHeader {
				t.Fatalf("fresh application theme is not normalized: %#v", application)
			}

			user, err := (&service.Theme{}).PatchUserResource(ctx, "integration-user", map[string]any{
				"colorWeak":   true,
				"fixSiderbar": false,
			}, &zero)
			if err != nil {
				t.Fatalf("write fresh user theme: %v", err)
			}
			if user.Meta.Revision != "1" || user.ColorWeak == nil || !*user.ColorWeak ||
				user.FixSiderbar == nil || *user.FixSiderbar {
				t.Fatalf("fresh user theme is not normalized: %#v", user)
			}
		})

		if err := migrateConfigRevisions(db, themeSettingsMigrationVersion); err != nil {
			t.Fatalf("rerun config revision migration after writes: %v", err)
		}
		assertThemeSettingsMigrationVersionCount(t, db, 1)
		assertThemeSettingsRevision(t, db, "application", "", "theme", 1)
		assertThemeSettingsRevision(t, db, "user", "integration-user", "theme", 1)
	})

	t.Run("legacy upgrade preservation normalization and reset set", func(t *testing.T) {
		resetThemeSettingsMigrationIntegrationTables(t, db)
		createThemeSettingsConfigurationTables(t, db)
		insertLegacyThemeSettingsFixtures(t, db)

		appSchemaBefore := themeSettingsTableSchema(t, db, &models.AppConfig{})
		userSchemaBefore := themeSettingsTableSchema(t, db, &models.UserConfig{})
		appRowsBefore := themeSettingsApplicationRows(t, db)
		userRowsBefore := themeSettingsUserRows(t, db)

		for attempt := 1; attempt <= 2; attempt++ {
			if err := migrateConfigRevisions(db, themeSettingsMigrationVersion); err != nil {
				t.Fatalf("legacy config revision migration attempt %d: %v", attempt, err)
			}
		}

		assertThemeSettingsConfigurationSchemaUnchanged(
			t,
			db,
			appSchemaBefore,
			userSchemaBefore,
		)
		if got := themeSettingsApplicationRows(t, db); !reflect.DeepEqual(got, appRowsBefore) {
			t.Fatalf("application rows changed during additive migration:\n got: %#v\nwant: %#v", got, appRowsBefore)
		}
		if got := themeSettingsUserRows(t, db); !reflect.DeepEqual(got, userRowsBefore) {
			t.Fatalf("user rows changed during additive migration:\n got: %#v\nwant: %#v", got, userRowsBefore)
		}
		assertThemeSettingsRevisionSchema(t, db)
		assertThemeSettingsMigrationVersionCount(t, db, 1)

		withThemeSettingsServiceDB(t, db, func(ctx *gin.Context) {
			application, err := (&service.Theme{}).ApplicationResource(ctx)
			if err != nil {
				t.Fatalf("read upgraded application theme: %v", err)
			}
			if application.Meta.Revision != "0" || application.FixedHeader == nil ||
				*application.FixedHeader || application.Layout == nil || *application.Layout != "mix" ||
				application.ColorPrimary == nil || *application.ColorPrimary != "#abcdef" ||
				application.NavTheme != nil {
				t.Fatalf("upgraded application theme normalization mismatch: %#v", application)
			}

			user, err := (&service.Theme{}).UserResource(ctx, "integration-user")
			if err != nil {
				t.Fatalf("read upgraded user theme: %v", err)
			}
			if user.Meta.Revision != "0" || user.ColorWeak == nil || !*user.ColorWeak ||
				user.FixSiderbar == nil || *user.FixSiderbar || user.ContentWidth != nil {
				t.Fatalf("upgraded user theme normalization mismatch: %#v", user)
			}

			zero := int64(0)
			application, err = (&service.Theme{}).ResetApplicationResource(ctx, &zero)
			if err != nil {
				t.Fatalf("reset upgraded application theme: %v", err)
			}
			if application.Meta.Revision != "1" || !themeSettingsResourceIsEmpty(application) {
				t.Fatalf("application reset returned a non-empty resource: %#v", application)
			}
			user, err = (&service.Theme{}).ResetUserResource(ctx, "integration-user", &zero)
			if err != nil {
				t.Fatalf("reset upgraded user theme: %v", err)
			}
			if user.Meta.Revision != "1" || !themeSettingsResourceIsEmpty(user) {
				t.Fatalf("user reset returned a non-empty resource: %#v", user)
			}

			assertThemeSettingsPreservedRow(t, db, &models.AppConfig{}, "app-unknown-theme")
			assertThemeSettingsPreservedRow(t, db, &models.AppConfig{}, "app-pwa")
			assertThemeSettingsPreservedRow(t, db, &models.AppConfig{}, "app-custom")
			assertThemeSettingsPreservedRow(t, db, &models.UserConfig{}, "user-unknown-theme")
			assertThemeSettingsPreservedRow(t, db, &models.UserConfig{}, "user-other-owner")

			one := int64(1)
			application, err = (&service.Theme{}).PatchApplicationResource(ctx, map[string]any{
				"fixedHeader": false,
			}, &one)
			if err != nil {
				t.Fatalf("set application theme after reset: %v", err)
			}
			if application.Meta.Revision != "2" || application.FixedHeader == nil || *application.FixedHeader {
				t.Fatalf("application reset-then-set mismatch: %#v", application)
			}

			user, err = (&service.Theme{}).PatchUserResource(ctx, "integration-user", map[string]any{
				"colorWeak": false,
			}, &one)
			if err != nil {
				t.Fatalf("set user theme after reset: %v", err)
			}
			if user.Meta.Revision != "2" || user.ColorWeak == nil || *user.ColorWeak {
				t.Fatalf("user reset-then-set mismatch: %#v", user)
			}
		})

		assertThemeSettingsSingleStoredValue(t, db, "application", "", "fixedHeader", "false")
		assertThemeSettingsSingleStoredValue(t, db, "user", "integration-user", "colorWeak", "false")
		if err := migrateConfigRevisions(db, themeSettingsMigrationVersion); err != nil {
			t.Fatalf("rerun config revision migration after reset/set: %v", err)
		}
		assertThemeSettingsMigrationVersionCount(t, db, 1)
		assertThemeSettingsRevision(t, db, "application", "", "theme", 2)
		assertThemeSettingsRevision(t, db, "user", "integration-user", "theme", 2)
	})
}

func createThemeSettingsConfigurationTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&models.AppConfig{}, &models.UserConfig{}); err != nil {
		t.Fatalf("create theme settings configuration tables: %v", err)
	}
	groupColumn := `"group"`
	if db.Dialector.Name() == "mysql" {
		groupColumn = "`group`"
	}
	if err := db.Exec(fmt.Sprintf(
		"CREATE UNIQUE INDEX ux_theme_settings_app_config ON mss_boot_app_configs(name, %s)",
		groupColumn,
	)).Error; err != nil {
		t.Fatalf("create application configuration unique index: %v", err)
	}
	if err := db.Exec(fmt.Sprintf(
		"CREATE UNIQUE INDEX ux_theme_settings_user_config ON mss_boot_user_configs(user_id, name, %s)",
		groupColumn,
	)).Error; err != nil {
		t.Fatalf("create user configuration unique index: %v", err)
	}
}

func insertLegacyThemeSettingsFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	appRows := []models.AppConfig{
		newThemeSettingsAppConfig("app-fixed-header", "theme", "fixedHeader", "false", false, now),
		newThemeSettingsAppConfig("app-layout", "theme", "layout", "mix", false, now),
		newThemeSettingsAppConfig("app-color", "theme", "colorPrimary", "#ABCDEF", false, now),
		newThemeSettingsAppConfig("app-invalid", "theme", "navTheme", "legacy-invalid", false, now),
		newThemeSettingsAppConfig("app-unknown-theme", "theme", "futureDensity", "compact", true, now),
		newThemeSettingsAppConfig("app-pwa", "theme", "pwa", "true", false, now),
		newThemeSettingsAppConfig("app-custom", "custom", "releaseChannel", "edge", true, now),
	}
	if err := db.Create(&appRows).Error; err != nil {
		t.Fatalf("insert legacy application theme fixtures: %v", err)
	}

	userRows := []models.UserConfig{
		newThemeSettingsUserConfig("user-color-weak", "integration-user", "theme", "colorWeak", "true", now),
		newThemeSettingsUserConfig("user-fix-siderbar", "integration-user", "theme", "fixSiderbar", "false", now),
		newThemeSettingsUserConfig("user-invalid", "integration-user", "theme", "contentWidth", "Wide", now),
		newThemeSettingsUserConfig("user-unknown-theme", "integration-user", "theme", "futureDensity", "comfortable", now),
		newThemeSettingsUserConfig("user-other-owner", "other-user", "theme", "colorWeak", "false", now),
	}
	if err := db.Create(&userRows).Error; err != nil {
		t.Fatalf("insert legacy user theme fixtures: %v", err)
	}
}

func newThemeSettingsAppConfig(
	id, group, name, value string,
	auth bool,
	now time.Time,
) models.AppConfig {
	row := models.AppConfig{Group: group, Name: name, Value: value, Auth: auth}
	row.ID = id
	row.CreatedAt = now
	row.UpdatedAt = now
	return row
}

func newThemeSettingsUserConfig(
	id, userID, group, name, value string,
	now time.Time,
) models.UserConfig {
	row := models.UserConfig{UserID: userID, Group: group, Name: name, Value: value}
	row.ID = id
	row.CreatedAt = now
	row.UpdatedAt = now
	return row
}

type themeSettingsIntegrationTenant struct {
	db *gorm.DB
}

func (e *themeSettingsIntegrationTenant) Scope(
	_ *gin.Context,
	_ schema.Tabler,
) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB { return db }
}

func (e *themeSettingsIntegrationTenant) GetTenant(*gin.Context) (center.TenantImp, error) {
	return e, nil
}

func (e *themeSettingsIntegrationTenant) GetDB(*gin.Context, schema.Tabler) *gorm.DB {
	return e.db.Session(&gorm.Session{NewDB: true})
}

func (*themeSettingsIntegrationTenant) GetID() any       { return "theme-settings-integration" }
func (*themeSettingsIntegrationTenant) GetDefault() bool { return true }

func withThemeSettingsServiceDB(t *testing.T, db *gorm.DB, run func(*gin.Context)) {
	t.Helper()
	previousTenant := center.GetTenant()
	previousCache := center.GetCache()
	center.SetTenant(&themeSettingsIntegrationTenant{db: db})
	center.SetCache(nil)
	defer func() {
		center.SetTenant(previousTenant)
		center.SetCache(previousCache)
	}()

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	run(ctx)
}

type themeSettingsColumn struct {
	Name             string
	DatabaseTypeName string
	Nullable         bool
	PrimaryKey       bool
	Unique           bool
}

func themeSettingsTableSchema(t *testing.T, db *gorm.DB, model any) []themeSettingsColumn {
	t.Helper()
	columnTypes, err := db.Migrator().ColumnTypes(model)
	if err != nil {
		t.Fatalf("inspect theme settings configuration schema: %v", err)
	}
	result := make([]themeSettingsColumn, 0, len(columnTypes))
	for _, columnType := range columnTypes {
		nullable, _ := columnType.Nullable()
		primaryKey, _ := columnType.PrimaryKey()
		unique, _ := columnType.Unique()
		result = append(result, themeSettingsColumn{
			Name:             columnType.Name(),
			DatabaseTypeName: strings.ToLower(columnType.DatabaseTypeName()),
			Nullable:         nullable,
			PrimaryKey:       primaryKey,
			Unique:           unique,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func assertThemeSettingsConfigurationSchemaUnchanged(
	t *testing.T,
	db *gorm.DB,
	appBefore, userBefore []themeSettingsColumn,
) {
	t.Helper()
	if got := themeSettingsTableSchema(t, db, &models.AppConfig{}); !reflect.DeepEqual(got, appBefore) {
		t.Fatalf("application configuration schema changed:\n got: %#v\nwant: %#v", got, appBefore)
	}
	if got := themeSettingsTableSchema(t, db, &models.UserConfig{}); !reflect.DeepEqual(got, userBefore) {
		t.Fatalf("user configuration schema changed:\n got: %#v\nwant: %#v", got, userBefore)
	}
}

type themeSettingsApplicationRow struct {
	ID      string
	Group   string
	Name    string
	Value   string
	Auth    bool
	Deleted bool
}

type themeSettingsUserRow struct {
	ID      string
	UserID  string
	Group   string
	Name    string
	Value   string
	Deleted bool
}

func themeSettingsApplicationRows(t *testing.T, db *gorm.DB) []themeSettingsApplicationRow {
	t.Helper()
	var rows []models.AppConfig
	if err := db.Unscoped().Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("read application configuration rows: %v", err)
	}
	result := make([]themeSettingsApplicationRow, 0, len(rows))
	for i := range rows {
		result = append(result, themeSettingsApplicationRow{
			ID: rows[i].ID, Group: rows[i].Group, Name: rows[i].Name,
			Value: rows[i].Value, Auth: rows[i].Auth, Deleted: rows[i].DeletedAt.Valid,
		})
	}
	return result
}

func themeSettingsUserRows(t *testing.T, db *gorm.DB) []themeSettingsUserRow {
	t.Helper()
	var rows []models.UserConfig
	if err := db.Unscoped().Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("read user configuration rows: %v", err)
	}
	result := make([]themeSettingsUserRow, 0, len(rows))
	for i := range rows {
		result = append(result, themeSettingsUserRow{
			ID: rows[i].ID, UserID: rows[i].UserID, Group: rows[i].Group,
			Name: rows[i].Name, Value: rows[i].Value, Deleted: rows[i].DeletedAt.Valid,
		})
	}
	return result
}

func assertThemeSettingsRevisionSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if !db.Migrator().HasTable(&models.ConfigRevision{}) {
		t.Fatal("config revision table was not created")
	}
	for _, column := range []string{"scope", "owner_id", "resource", "revision", "updated_at"} {
		if !db.Migrator().HasColumn(&models.ConfigRevision{}, column) {
			t.Fatalf("config revision table is missing column %q", column)
		}
	}

	rows := []models.ConfigRevision{
		{Scope: "integration", OwnerID: "one", Resource: "theme", Revision: 3},
		{Scope: "integration", OwnerID: "two", Resource: "theme", Revision: 4},
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
		t.Fatalf("create composite revision sentinels: %v", err)
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.ConfigRevision{
		Scope: "integration", OwnerID: "one", Resource: "theme", Revision: 99,
	}).Error; err != nil {
		t.Fatalf("insert duplicate composite revision sentinel: %v", err)
	}
	var count int64
	if err := db.Model(&models.ConfigRevision{}).Where("scope = ?", "integration").Count(&count).Error; err != nil {
		t.Fatalf("count composite revision sentinels: %v", err)
	}
	if count != 2 {
		t.Fatalf("composite revision sentinel count = %d, want 2", count)
	}
	assertThemeSettingsRevision(t, db, "integration", "one", "theme", 3)
	if err := db.Where("scope = ?", "integration").Delete(&models.ConfigRevision{}).Error; err != nil {
		t.Fatalf("remove composite revision sentinels: %v", err)
	}
}

func assertThemeSettingsMigrationVersionCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", themeSettingsMigrationVersion).
		Count(&count).Error; err != nil {
		t.Fatalf("count theme settings migration versions: %v", err)
	}
	if count != want {
		t.Fatalf("theme settings migration version count = %d, want %d", count, want)
	}
}

func assertThemeSettingsRevision(
	t *testing.T,
	db *gorm.DB,
	scope, ownerID, resource string,
	want int64,
) {
	t.Helper()
	var row models.ConfigRevision
	if err := db.First(
		&row,
		"scope = ? AND owner_id = ? AND resource = ?",
		scope,
		ownerID,
		resource,
	).Error; err != nil {
		t.Fatalf("load revision %s/%s/%s: %v", scope, ownerID, resource, err)
	}
	if row.Revision != want {
		t.Fatalf("revision %s/%s/%s = %d, want %d", scope, ownerID, resource, row.Revision, want)
	}
}

func themeSettingsResourceIsEmpty(resource *dto.ThemeResource) bool {
	return resource != nil &&
		resource.NavTheme == nil &&
		resource.ColorPrimary == nil &&
		resource.Layout == nil &&
		resource.ContentWidth == nil &&
		resource.FixedHeader == nil &&
		resource.FixSiderbar == nil &&
		resource.ColorWeak == nil
}

func assertThemeSettingsPreservedRow(t *testing.T, db *gorm.DB, model any, id string) {
	t.Helper()
	result := db.Unscoped().First(model, "id = ?", id)
	if result.Error != nil {
		t.Fatalf("preserved configuration row %q is missing: %v", id, result.Error)
	}
	reflected := reflect.Indirect(reflect.ValueOf(model))
	deletedAt := reflected.FieldByName("DeletedAt")
	if deletedAt.IsValid() && deletedAt.FieldByName("Valid").Bool() {
		t.Fatalf("preserved configuration row %q was soft deleted", id)
	}
}

func assertThemeSettingsSingleStoredValue(
	t *testing.T,
	db *gorm.DB,
	scope, ownerID, name, wantValue string,
) {
	t.Helper()
	if scope == "application" {
		var rows []models.AppConfig
		if err := db.Unscoped().Where(&models.AppConfig{Group: service.ThemeConfigGroup, Name: name}).Find(&rows).Error; err != nil {
			t.Fatalf("read stored application theme value %q: %v", name, err)
		}
		if len(rows) != 1 || rows[0].Value != wantValue || rows[0].DeletedAt.Valid {
			t.Fatalf("stored application theme value %q = %#v, want one active %q row", name, rows, wantValue)
		}
		return
	}

	var rows []models.UserConfig
	if err := db.Unscoped().Where(&models.UserConfig{
		UserID: ownerID,
		Group:  service.ThemeConfigGroup,
		Name:   name,
	}).Find(&rows).Error; err != nil {
		t.Fatalf("read stored user theme value %q: %v", name, err)
	}
	if len(rows) != 1 || rows[0].Value != wantValue || rows[0].DeletedAt.Valid {
		t.Fatalf("stored user theme value %q = %#v, want one active %q row", name, rows, wantValue)
	}
}

func resetThemeSettingsMigrationIntegrationTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := dropThemeSettingsMigrationIntegrationTables(db); err != nil {
		t.Fatalf("reset theme settings migration integration tables: %v", err)
	}
}

func dropThemeSettingsMigrationIntegrationTables(db *gorm.DB) error {
	for _, table := range []string{
		"mss_boot_config_revisions",
		"mss_boot_user_configs",
		"mss_boot_app_configs",
		"mss_boot_migration",
	} {
		if err := db.Exec("DROP TABLE IF EXISTS " + table).Error; err != nil {
			return fmt.Errorf("drop %s: %w", table, err)
		}
	}
	return nil
}
