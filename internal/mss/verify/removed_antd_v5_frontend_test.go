package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredAntDesignV5FrontendStaysAbsent(t *testing.T) {
	root := repositoryRoot(t)
	for _, relative := range []string{
		"web/antd",
		"templates/module/frontend",
		".github/workflows/frontend-ci.yml",
		".github/workflows/frontend-deploy.yml",
		".github/workflows/frontend-release.yml",
		"scripts/remediate_web_deps.py",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err == nil {
			t.Errorf("retired V5 frontend path exists: %s", relative)
		} else if !os.IsNotExist(err) {
			t.Errorf("inspect retired V5 frontend path %s: %v", relative, err)
		}
	}
}

func TestActiveFrontendContractsSelectOnlyAntDesignV6(t *testing.T) {
	root := repositoryRoot(t)
	for _, relative := range []string{
		".mss/project.yaml",
		".mss/dev.yaml",
		".mss/release-policy.yaml",
		".mss/schemas/admin-module.schema.json",
		".mss/blueprints/management-system.yaml",
		"Makefile",
		"internal/mss/app/app.go",
		"internal/mss/dev/config.go",
		"internal/mss/generator/generator.go",
		"internal/mss/mcp/server.go",
		"internal/mss/setup/setup.go",
		"internal/mss/spec/module.go",
		"internal/mss/verify/verify.go",
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read active frontend contract %s: %v", relative, err)
		}
		for _, retired := range []string{"antd-v5", "frontend-v5", "web/antd/"} {
			if strings.Contains(string(content), retired) {
				t.Errorf("active frontend contract %s contains retired token %q", relative, retired)
			}
		}
	}
}
