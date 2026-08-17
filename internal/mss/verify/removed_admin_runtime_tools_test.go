package verify

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRemovedAdminRuntimeToolsStayAbsent(t *testing.T) {
	root := repositoryRoot(t)
	removed := []string{
		"admin/apis/_virtual_disabled.go",
		"admin/apis/model.go",
		"admin/apis/field.go",
		"admin/apis/template.go",
		"admin/apis/oauth_credential.go",
		"admin/apis/template_oauth_credential_test.go",
		"admin/apis/template_repository.go",
		"admin/apis/template_repository_test.go",
		"admin/apis/github.go",
		"admin/dto/model.go",
		"admin/dto/field.go",
		"admin/dto/template.go",
		"admin/dto/virtual.go",
		"admin/models/model.go",
		"admin/models/field.go",
		"admin/pkg/common.go",
		"admin/pkg/generator.go",
		"admin/pkg/generate_config.go",
		"admin/pkg/git.go",
		"admin/pkg/gist.go",
		"admin/pkg/github_client.go",
		"admin/pkg/generator_security_test.go",
		"admin/pkg/git_security_test.go",
		"admin/pkg/oauthcredential",
		"admin/pkg/file.go",
		"admin/pkg/base_rule.go",
		"admin/pkg/pack",
		"admin/pkg/parse.go",
		"mss-boot/virtual",
		"web/antd-v6/src/pages/Model",
		"web/antd-v6/src/pages/Field",
		"web/antd-v6/src/pages/Virtual",
		"web/antd-v6/src/pages/Generator",
		"web/antd-v6/src/pages/Generator/credentialLifecycle.ts",
		"web/antd-v6/src/pages/Generator/credentialLifecycle.test.ts",
		"web/antd-v6/src/services/admin/model.ts",
		"web/antd-v6/src/services/admin/field.ts",
		"web/antd-v6/src/services/admin/virtual.ts",
		"web/antd-v6/src/services/admin/generator.ts",
		"web/antd-v6/src/services/admin/generatorCredential.test.ts",
		"web/antd-v6/src/util/addOption.tsx",
		"docs/docs/vm",
	}
	for _, relative := range removed {
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			err = filepath.WalkDir(path, func(current string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if current != path && !entry.IsDir() {
					t.Errorf("retired Admin runtime tool file exists: %s", filepath.ToSlash(current))
				}
				return nil
			})
			if err != nil {
				t.Errorf("inspect retired directory %s: %v", relative, err)
			}
		} else if err == nil {
			t.Errorf("retired Admin runtime tool path exists: %s", relative)
		} else if !os.IsNotExist(err) {
			t.Errorf("inspect retired path %s: %v", relative, err)
		}
	}
}

func TestRemovedAdminRuntimeToolsStayOutOfActiveContracts(t *testing.T) {
	root := repositoryRoot(t)
	checks := []struct {
		path       string
		prohibited []string
	}{
		{
			path: "web/antd-v6/config/routes.ts",
			prohibited: []string{
				"path: '/generator'", "path: '/model'", "path: '/field/", "path: '/virtual/",
				"component: './Generator'", "component: './Model'", "component: './Field'", "component: './Virtual'",
			},
		},
		{
			path:       "web/antd-v6/src/services/admin/index.ts",
			prohibited: []string{"'./generator'", "'./model'", "'./field'", "'./virtual'"},
		},
		{
			path:       ".mss/capabilities.yaml",
			prohibited: []string{"legacy.dynamic-schema", "admin/models/model.go", "mss-boot/virtual"},
		},
		{
			path: "admin/apis/oauth.go",
			prohibited: []string{
				"oauthcredential", "IntentIntegration",
			},
		},
		{
			path: "admin/dto/github.go",
			prohibited: []string{
				"oneof=login binding integration",
				`json:"credential,omitempty"`,
				`json:"credentialExpiresAt,omitempty"`,
			},
		},
		{
			path:       "admin/pkg/oauthstate/store.go",
			prohibited: []string{"IntentIntegration"},
		},
		{
			path: "admin/router/security_contract.go",
			prohibited: []string{
				"/admin/api/:key", "/admin/api/models", "/admin/api/fields",
				"/admin/api/model/generate-data", "/admin/api/template/get-branches",
				"/admin/api/template/get-path", "/admin/api/template/get-params", "/admin/api/template/generate",
				"/admin/api/github/get-login-url",
			},
		},
		{
			path:       "web/antd-v6/src/locales/en-US.ts",
			prohibited: []string{"menu.develop", "menu.model", "menu.field", "menu.generator"},
		},
		{
			path:       "web/antd-v6/src/locales/zh-CN.ts",
			prohibited: []string{"menu.develop", "menu.model", "menu.field", "menu.generator"},
		},
		{
			path: "web/antd-v6/src/pages/User/Callback/$provider.tsx",
			prohibited: []string{
				"intent === 'integration'", "result.intent === 'integration'",
			},
		},
		{
			path: "web/antd-v6/src/services/admin/typings.d.ts",
			prohibited: []string{
				"'login' | 'binding' | 'integration'",
				"credentialExpiresAt?: string",
				"type TemplateGenerateReq",
				"type TemplateGetBranchesResp",
			},
		},
		{
			path: "web/antd-v6/src/utils/oauth.ts",
			prohibited: []string{
				"intent: 'integration'",
				"response.intent === 'integration'",
				"intent === 'integration'",
			},
		},
		{
			path: ".mss/features/admin-secret-lifecycle.yaml",
			prohibited: []string{
				"template-generator",
				"generator-credential-handle",
				"admin/pkg/oauthcredential",
				"pages/Generator/credentialLifecycle",
				"generatorCredential.test.ts",
			},
		},
		{
			path: "docs/docs/admin/token-oauth2-guide.md",
			prohibited: []string{
				"发起登录、绑定或集成授权",
				"Generator 的公开仓库",
				"真正执行 Generate 必须",
				"credential store 使用",
			},
		},
	}
	for _, check := range checks {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(check.path)))
		if err != nil {
			t.Fatalf("read active contract %s: %v", check.path, err)
		}
		for _, token := range check.prohibited {
			if strings.Contains(string(content), token) {
				t.Errorf("active contract %s still contains retired token %q", check.path, token)
			}
		}
	}
}

func TestOfflineDeterministicGeneratorRemainsAvailable(t *testing.T) {
	root := repositoryRoot(t)
	for _, relative := range []string{
		"cmd/mss/main.go",
		"internal/mss/generator/generator.go",
		"templates/module",
		".mss/schemas/admin-module.schema.json",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
			t.Errorf("required offline generator path %s is unavailable: %v", relative, err)
		}
	}
	content, err := os.ReadFile(filepath.Join(root, ".mss", "capabilities.yaml"))
	if err != nil {
		t.Fatalf("read capabilities: %v", err)
	}
	if !strings.Contains(string(content), "agent.module-generator") {
		t.Fatal("offline deterministic module generator capability was removed")
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
