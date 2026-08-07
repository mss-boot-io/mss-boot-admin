package system

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const sanitizeBuiltinOAuthTestVersion = "2026080622000"

func TestSanitizeBuiltinOAuthConfigOnlyMatchesHistoricalBuiltinTuple(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sanitize-oauth.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	if err := db.AutoMigrate(&models.SystemConfig{}, &migrationmodels.Migration{}); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	legacyContent := testOAuthConfig("fixture-client", "fixture-secret", []string{"repo", "user"})
	matching := createOAuthSystemConfig(t, db, "matching", legacyContent, true)
	customizedContent := testOAuthConfig("fixture-client", "rotated-secret", []string{"repo"})
	customized := createOAuthSystemConfig(t, db, "customized", customizedContent, true)
	nonBuiltin := createOAuthSystemConfig(t, db, "non-builtin", legacyContent, false)

	sanitized, err := sanitizeBuiltinOAuthConfig(
		db,
		sanitizeBuiltinOAuthTestVersion,
		oauthTupleFingerprint("fixture-client", "fixture-secret"),
	)
	if err != nil {
		t.Fatalf("sanitize built-in OAuth configuration: %v", err)
	}
	if sanitized != 1 {
		t.Fatalf("sanitized configs = %d, want 1", sanitized)
	}

	assertSanitizedOAuthConfig(t, db, matching.ID)
	assertOAuthConfigContent(t, db, customized.ID, customizedContent)
	assertOAuthConfigContent(t, db, nonBuiltin.ID, legacyContent)
	var versionCount int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", sanitizeBuiltinOAuthTestVersion).
		Count(&versionCount).Error; err != nil {
		t.Fatalf("count migration version: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("migration version count = %d, want 1", versionCount)
	}

	rerunSanitized, err := sanitizeBuiltinOAuthConfig(
		db,
		sanitizeBuiltinOAuthTestVersion,
		oauthTupleFingerprint("fixture-client", "fixture-secret"),
	)
	if err != nil {
		t.Fatalf("rerun sanitizer: %v", err)
	}
	if rerunSanitized != 0 {
		t.Fatalf("rerun sanitized configs = %d, want 0", rerunSanitized)
	}
}

func testOAuthConfig(clientID, clientSecret string, scopes []string) string {
	content := map[string]any{
		"server": map[string]any{"addr": "0.0.0.0:8080"},
		"oauth2": map[string]any{
			"clientID":     clientID,
			"clientSecret": clientSecret,
			"scopes":       scopes,
		},
	}
	data, err := yaml.Marshal(content)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func createOAuthSystemConfig(
	t *testing.T,
	db *gorm.DB,
	remark string,
	content string,
	builtIn bool,
) models.SystemConfig {
	t.Helper()
	config := models.SystemConfig{
		Name:    "application",
		Ext:     source.SchemeYaml,
		Content: content,
		Remark:  remark,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("create %s system config: %v", remark, err)
	}
	result := db.Table(config.TableName()).
		Where("id = ?", config.ID).
		Update("built_in", builtIn)
	if result.Error != nil {
		t.Fatalf("set %s built-in flag: %v", remark, result.Error)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("set %s built-in flag affected %d rows, want 1", remark, result.RowsAffected)
	}
	var storedBuiltIn bool
	if err := db.Table(config.TableName()).
		Select("built_in").
		Where("id = ?", config.ID).
		Scan(&storedBuiltIn).Error; err != nil {
		t.Fatalf("read back %s built-in flag: %v", remark, err)
	}
	if storedBuiltIn != builtIn {
		t.Fatalf("stored %s built-in flag = %v, want %v", remark, storedBuiltIn, builtIn)
	}
	return config
}

func assertSanitizedOAuthConfig(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	var config models.SystemConfig
	if err := db.Unscoped().First(&config, "id = ?", id).Error; err != nil {
		t.Fatalf("load sanitized system config: %v", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(config.Content), &document); err != nil {
		t.Fatalf("parse sanitized YAML: %v", err)
	}
	root := document.Content[0]
	oauth, ok := yamlMappingValue(root, "oauth2")
	if !ok {
		t.Fatal("sanitized YAML has no oauth2 mapping")
	}
	clientID, _ := yamlMappingValue(oauth, "clientID")
	clientSecret, _ := yamlMappingValue(oauth, "clientSecret")
	if clientID.Value != "" || clientSecret.Value != "" {
		t.Fatal("sanitized OAuth credentials are not empty")
	}
	scopeNode, _ := yamlMappingValue(oauth, "scopes")
	scopes := make([]string, 0, len(scopeNode.Content))
	for _, item := range scopeNode.Content {
		scopes = append(scopes, item.Value)
	}
	if want := []string{"read:user", "user:email"}; !reflect.DeepEqual(scopes, want) {
		t.Fatalf("sanitized scopes = %#v, want %#v", scopes, want)
	}
}

func assertOAuthConfigContent(t *testing.T, db *gorm.DB, id, want string) {
	t.Helper()
	var config models.SystemConfig
	if err := db.Unscoped().First(&config, "id = ?", id).Error; err != nil {
		t.Fatalf("load unchanged system config: %v", err)
	}
	if config.Content != want {
		t.Fatalf("customized system config %q was changed", id)
	}
}
