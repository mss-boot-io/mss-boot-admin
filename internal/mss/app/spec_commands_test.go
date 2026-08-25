package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

func TestSpecInitFeatureAcceptsExplicitPrimaryModule(t *testing.T) {
	rootOverride := repositoryRoot(t)
	command := newUnifiedSpecInitCommand(&rootOverride)
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"order-approval", "--kind", "feature", "--module", "orders"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute spec init: %v", err)
	}
	feature, err := spec.LoadFeature(writeFeatureFixtureForApp(t, stdout.String()))
	if err != nil {
		t.Fatalf("load rendered Feature: %v\n%s", err, stdout.String())
	}
	if len(feature.Spec.Modules) != 1 || feature.Spec.Modules[0].Name != "orders" {
		t.Fatalf("rendered modules = %#v", feature.Spec.Modules)
	}
}

func writeFeatureFixtureForApp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "feature.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write Feature fixture: %v", err)
	}
	return path
}

func TestSpecInitRejectsModuleFlagForAdminModule(t *testing.T) {
	rootOverride := repositoryRoot(t)
	command := newUnifiedSpecInitCommand(&rootOverride)
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"orders", "--kind", "module", "--module", "catalog"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "supported only for Feature") {
		t.Fatalf("error = %v", err)
	}
}

func TestSpecInitFeatureWritesRequestedPathWithoutOverwrite(t *testing.T) {
	sourceRoot := repositoryRoot(t)
	rootOverride := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootOverride, ".mss"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"project.yaml", "capabilities.yaml", "commands.yaml"} {
		data, err := os.ReadFile(filepath.Join(sourceRoot, ".mss", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(rootOverride, ".mss", name), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	arguments := []string{
		"order-approval",
		"--kind", "feature",
		"--module", "orders",
		"--output", ".mss/features/order-approval.yaml",
		"--write",
	}
	command := newUnifiedSpecInitCommand(&rootOverride)
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs(arguments)
	if err := command.Execute(); err != nil {
		t.Fatalf("execute spec init write: %v", err)
	}
	target := filepath.Join(rootOverride, ".mss", "features", "order-approval.yaml")
	feature, err := spec.LoadFeature(target)
	if err != nil {
		t.Fatalf("load written Feature: %v", err)
	}
	if len(feature.Spec.Modules) != 1 || feature.Spec.Modules[0].Name != "orders" {
		t.Fatalf("written modules = %#v", feature.Spec.Modules)
	}

	second := newUnifiedSpecInitCommand(&rootOverride)
	second.SetOut(&bytes.Buffer{})
	second.SetErr(&bytes.Buffer{})
	second.SetArgs(arguments)
	err = second.Execute()
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("second write error = %v", err)
	}
}
