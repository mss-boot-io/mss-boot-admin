package system

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"runtime"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// This is the SHA-256 fingerprint of the historical built-in
// clientID + NUL + clientSecret tuple. The credential itself must never be
// reintroduced into source, fixtures, logs, or migration diagnostics.
const legacyBuiltinOAuthTupleFingerprint = "768de56f0790a647823a1b3e72a9d6a81928eaa2c5623c8851b3d72ed9b641a1"

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260806220000SanitizeBuiltinOAuth)
}

func _20260806220000SanitizeBuiltinOAuth(db *gorm.DB, version string) error {
	sanitized, err := sanitizeBuiltinOAuthConfig(db, version, legacyBuiltinOAuthTupleFingerprint)
	if err != nil {
		return err
	}
	if sanitized > 0 {
		slog.Warn(
			"removed a historical credential from built-in OAuth configuration; provider-side revocation is still required",
			"sanitizedConfigs", sanitized,
		)
	}
	return nil
}

func sanitizeBuiltinOAuthConfig(db *gorm.DB, version, expectedFingerprint string) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("sanitize built-in OAuth configuration: database is nil")
	}
	var sanitized int64
	err := db.Transaction(func(tx *gorm.DB) error {
		var alreadyApplied int64
		if err := tx.Model(&migrationmodels.Migration{}).
			Where("version = ?", version).
			Count(&alreadyApplied).Error; err != nil {
			return fmt.Errorf("sanitize built-in OAuth configuration: check version: %w", err)
		}
		if alreadyApplied > 0 {
			return nil
		}

		var configs []models.SystemConfig
		if err := tx.Unscoped().
			Where("name = ? AND ext IN ? AND built_in = ?", "application", []source.Scheme{source.SchemeYaml, source.SchemeYml}, true).
			Find(&configs).Error; err != nil {
			return fmt.Errorf("sanitize built-in OAuth configuration: load built-in application profile: %w", err)
		}
		for i := range configs {
			content, changed, err := sanitizeLegacyOAuthYAML(configs[i].Content, expectedFingerprint)
			if err != nil {
				return fmt.Errorf("sanitize built-in OAuth configuration %q: %w", configs[i].ID, err)
			}
			if !changed {
				continue
			}
			result := tx.Unscoped().Model(&models.SystemConfig{}).
				Where("id = ?", configs[i].ID).
				Update("content", content)
			if result.Error != nil {
				return fmt.Errorf("sanitize built-in OAuth configuration %q: update content: %w", configs[i].ID, result.Error)
			}
			sanitized += result.RowsAffected
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error; err != nil {
			return fmt.Errorf("sanitize built-in OAuth configuration: record migration version: %w", err)
		}
		return nil
	})
	return sanitized, err
}

func sanitizeLegacyOAuthYAML(content, expectedFingerprint string) (string, bool, error) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		return "", false, fmt.Errorf("parse YAML: %w", err)
	}
	root := &document
	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		root = root.Content[0]
	}
	oauth, ok := yamlMappingValue(root, "oauth2")
	if !ok {
		return content, false, nil
	}
	clientID, hasClientID := yamlMappingValue(oauth, "clientID")
	clientSecret, hasClientSecret := yamlMappingValue(oauth, "clientSecret")
	if !hasClientID || !hasClientSecret || clientID.Kind != yaml.ScalarNode || clientSecret.Kind != yaml.ScalarNode {
		return content, false, nil
	}
	fingerprint := oauthTupleFingerprint(clientID.Value, clientSecret.Value)
	if subtle.ConstantTimeCompare([]byte(fingerprint), []byte(expectedFingerprint)) != 1 {
		return content, false, nil
	}

	clientID.Value = ""
	clientID.Tag = "!!str"
	clientID.Style = yaml.SingleQuotedStyle
	clientSecret.Value = ""
	clientSecret.Tag = "!!str"
	clientSecret.Style = yaml.SingleQuotedStyle
	scopes, hasScopes := yamlMappingValue(oauth, "scopes")
	if !hasScopes {
		oauth.Content = append(oauth.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "scopes"},
			&yaml.Node{},
		)
		scopes = oauth.Content[len(oauth.Content)-1]
	}
	scopes.Kind = yaml.SequenceNode
	scopes.Tag = "!!seq"
	scopes.Style = 0
	scopes.Value = ""
	scopes.Content = []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "read:user"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "user:email"},
	}

	updated, err := yaml.Marshal(&document)
	if err != nil {
		return "", false, fmt.Errorf("encode sanitized YAML: %w", err)
	}
	return string(updated), true, nil
}

func oauthTupleFingerprint(clientID, clientSecret string) string {
	sum := sha256.Sum256([]byte(clientID + "\x00" + clientSecret))
	return hex.EncodeToString(sum[:])
}

func yamlMappingValue(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1], true
		}
	}
	return nil, false
}
