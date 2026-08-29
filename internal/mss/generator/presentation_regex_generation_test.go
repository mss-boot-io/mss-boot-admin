package generator

import (
	"strings"
	"testing"
)

func TestGeneratePresentationUsesPortableRegexRuntimeInEveryFrontendConsumer(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, repositoryRoot+"/templates/module", root+"/templates/module")
	module := loadPresentationTestModule(t, repositoryRoot)

	if _, err := Generate(module, Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(presentation regex consumers) error = %v", err)
	}

	for _, relative := range []string{
		"web/antd-v6/src/generated/modules/supplier/contract.ts",
		"web/antd-v6/src/generated/modules/supplier/SupplierPage.tsx",
		"web/antd-v6/e2e/generated/supplier.spec.ts",
	} {
		generated := string(readGeneratedTestFile(t, root, relative))
		if !strings.Contains(generated, `compilePortablePresentationPattern("^[A-Z0-9_-]+$")`) {
			t.Errorf("generated presentation consumer %s omitted the portable pattern compiler", relative)
		}
		if !strings.Contains(generated, "@mss-boot-io/admin-web/runtime/presentation") {
			t.Errorf("generated presentation consumer %s omitted the stable runtime subpath", relative)
		}
		if strings.Contains(generated, `new RegExp("^[A-Z0-9_-]+$")`) {
			t.Errorf("generated presentation consumer %s retained a bare RegExp compiler", relative)
		}
	}
}

func TestGenerateLegacyModuleKeepsExistingRegexOutputWithoutPresentationRuntimeImport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, repositoryRoot+"/templates/module", root+"/templates/module")
	module := loadPresentationTestModule(t, repositoryRoot)
	module.Spec.Presentation = nil

	if _, err := Generate(module, Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(legacy regex consumers) error = %v", err)
	}

	for _, relative := range []string{
		"web/antd-v6/src/generated/modules/supplier/contract.ts",
		"web/antd-v6/src/generated/modules/supplier/SupplierPage.tsx",
		"web/antd-v6/e2e/generated/supplier.spec.ts",
	} {
		generated := string(readGeneratedTestFile(t, root, relative))
		if !strings.Contains(generated, `new RegExp("^[A-Z0-9_-]+$")`) {
			t.Errorf("legacy consumer %s no longer uses its existing regex output", relative)
		}
		if strings.Contains(generated, "runtime/presentation") || strings.Contains(generated, "compilePortablePresentationPattern(") {
			t.Errorf("legacy consumer %s unexpectedly depends on the presentation runtime", relative)
		}
	}
}
