package setup

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func TestPlanUsesOnlyThinHostDependencySurfaces(t *testing.T) {
	root := t.TempDir()
	ctx := &project.Context{Root: root}
	ctx.Project.Spec.RepositoryLayout = map[string]string{
		"kind":     "thin-host",
		"backend":  ".",
		"frontend": "web",
	}
	ctx.Project.Spec.Frontend.PackageManagerVersion = "10.34.5"
	ctx.Project.Spec.Frontend.DefaultApplication = "admin-web"
	ctx.Project.Spec.Frontend.Applications = []project.FrontendApplicationSpec{{
		ID:                    "admin-web",
		Path:                  "web",
		PackageManager:        "pnpm",
		PackageManagerVersion: "10.34.5",
	}}

	steps := Plan(ctx, Options{})
	if len(steps) != 3 {
		t.Fatalf("Thin Host setup steps = %#v", steps)
	}
	if got, want := steps[0].ID, "go-backend-dependencies"; got != want {
		t.Fatalf("backend setup ID = %q, want %q", got, want)
	}
	if steps[0].Directory != filepath.Clean(root) || !reflect.DeepEqual(steps[0].Args, []string{"go", "mod", "download"}) || steps[0].Environment["GOWORK"] != "off" || steps[0].Environment["GOFLAGS"] != "-mod=readonly" {
		t.Fatalf("backend setup = %#v", steps[0])
	}
	if got, want := steps[1].ID, "go-backend-migrate"; got != want {
		t.Fatalf("migration setup ID = %q, want %q", got, want)
	}
	if steps[1].Directory != filepath.Clean(root) || !reflect.DeepEqual(steps[1].Args, []string{"go", "run", "-mod=readonly", "./cmd/server", "migrate", "--config-provider", "fs"}) || steps[1].Environment["GOWORK"] != "off" || steps[1].Environment["CONFIG_PROVIDER"] != "fs" {
		t.Fatalf("migration setup = %#v", steps[1])
	}
	for _, argument := range steps[1].Args {
		if strings.Contains(strings.ToLower(argument), "password") {
			t.Fatalf("migration setup leaked a password argument: %#v", steps[1])
		}
	}
	if got, want := steps[2].ID, "frontend-dependencies"; got != want {
		t.Fatalf("frontend setup ID = %q, want %q", got, want)
	}
	if steps[2].Directory != filepath.Join(root, "web") || !reflect.DeepEqual(steps[2].Args, []string{"corepack", "pnpm@10.34.5", "install", "--frozen-lockfile"}) {
		t.Fatalf("frontend setup = %#v", steps[2])
	}
	for _, step := range steps {
		if step.ID == "go-framework-dependencies" || step.ID == "docs-dependencies" {
			t.Fatalf("Thin Host setup contains Foundation-only step %#v", step)
		}
		if !reflect.DeepEqual(step.UnsetEnvironment, []string{initialAdminPasswordEnvironment}) {
			t.Fatalf("setup step does not remove inherited initial password: %#v", step)
		}
	}
}

func TestRunScopesInheritedInitialPasswordToMigrationAndRedactsIt(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	toolDirectory := t.TempDir()
	writeSetupTestExecutable(t, toolDirectory, "go", `#!/bin/sh
case " $* " in
  *" ./cmd/server migrate "*)
    if [ -z "${MSS_ADMIN_INITIAL_PASSWORD:-}" ]; then
	      echo "initial administrator credentials are required" >&2
      exit 42
    fi
    if [ "${MSS_ADMIN_INITIAL_PASSWORD}" != "PackageFirstAdmin2026" ]; then
      echo "unexpected migration password" >&2
      exit 43
    fi
    echo "migration received ${MSS_ADMIN_INITIAL_PASSWORD}" >&2
    ;;
  *)
    if [ -n "${MSS_ADMIN_INITIAL_PASSWORD:-}" ]; then
      echo "dependency step leaked ${MSS_ADMIN_INITIAL_PASSWORD}" >&2
      exit 44
    fi
    ;;
esac
exit 0
`)
	writeSetupTestExecutable(t, toolDirectory, "corepack", `#!/bin/sh
if [ -n "${MSS_ADMIN_INITIAL_PASSWORD:-}" ]; then
  echo "frontend step leaked ${MSS_ADMIN_INITIAL_PASSWORD}" >&2
  exit 45
fi
exit 0
`)
	t.Setenv("PATH", toolDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx := thinHostSetupContext(root)
	t.Setenv("MSS_ADMIN_INITIAL_PASSWORD", "")
	missing := Run(context.Background(), ctx, Options{})
	if missing.Success || len(missing.Results) != 2 || missing.Results[1].ID != "go-backend-migrate" || missing.Results[1].ExitCode != 42 {
		t.Fatalf("setup without inherited initial password = %#v", missing)
	}

	const initialPassword = "PackageFirstAdmin2026"
	t.Setenv("MSS_ADMIN_INITIAL_PASSWORD", initialPassword)
	supplied := Run(context.Background(), ctx, Options{})
	if !supplied.Success || len(supplied.Results) != 3 {
		t.Fatalf("setup with inherited initial password = %#v", supplied)
	}
	serialized, err := json.Marshal(supplied)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), initialPassword) {
		t.Fatal("setup report exposed the inherited initial password")
	}
	if !strings.Contains(string(serialized), "[REDACTED]") {
		t.Fatal("setup report did not redact inherited password echoed by migration")
	}
}

func TestRunPromptsOnceAndRetriesOnlyTheMigration(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	toolDirectory := t.TempDir()
	const initialPassword = "PackageFirst!2026"
	writeSetupTestExecutable(t, toolDirectory, "go", `#!/bin/sh
case " $* " in
  *" ./cmd/server migrate "*)
	    if [ "${MSS_ADMIN_INITIAL_PASSWORD:-}" != "PackageFirst!2026" ]; then
      echo "initial administrator credentials are required" >&2
      exit 42
    fi
    echo "migration received ${MSS_ADMIN_INITIAL_PASSWORD}" >&2
	    ;;
	  *)
	    if [ -n "${MSS_ADMIN_INITIAL_PASSWORD:-}" ]; then
	      echo "dependency step leaked ${MSS_ADMIN_INITIAL_PASSWORD}" >&2
	      exit 44
	    fi
    ;;
esac
exit 0
`)
	writeSetupTestExecutable(t, toolDirectory, "corepack", `#!/bin/sh
if [ -n "${MSS_ADMIN_INITIAL_PASSWORD:-}" ]; then
  echo "frontend step leaked ${MSS_ADMIN_INITIAL_PASSWORD}" >&2
  exit 45
fi
exit 0
`)
	t.Setenv("PATH", toolDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MSS_ADMIN_INITIAL_PASSWORD", "")

	promptCount := 0
	secret := []byte(initialPassword)
	report := Run(context.Background(), thinHostSetupContext(root), Options{
		PromptInitialAdminPassword: func() ([]byte, error) {
			promptCount++
			return secret, nil
		},
	})
	if !report.Success || len(report.Results) != 3 {
		t.Fatalf("prompted setup = %#v", report)
	}
	if promptCount != 1 {
		t.Fatalf("password prompt count = %d, want 1", promptCount)
	}
	if got := string(secret); got != strings.Repeat("\x00", len(initialPassword)) {
		t.Fatalf("prompted password bytes were not cleared: %q", got)
	}
	serialized, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), initialPassword) {
		t.Fatal("setup report exposed the prompted initial password")
	}
	if !strings.Contains(string(serialized), "[REDACTED]") {
		t.Fatal("setup report did not redact a child process that echoed the prompted password")
	}
	for _, step := range report.Steps {
		if _, exists := step.Environment[initialAdminPasswordEnvironment]; exists {
			t.Fatalf("setup plan persisted prompted password environment: %#v", step)
		}
	}
}

func TestRunDoesNotPromptForUnrelatedMigrationFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	toolDirectory := t.TempDir()
	writeSetupTestExecutable(t, toolDirectory, "go", `#!/bin/sh
case " $* " in
  *" ./cmd/server migrate "*)
    echo "database is unavailable" >&2
    exit 43
    ;;
esac
exit 0
`)
	writeSetupTestExecutable(t, toolDirectory, "corepack", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", toolDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MSS_ADMIN_INITIAL_PASSWORD", "")

	prompted := false
	report := Run(context.Background(), thinHostSetupContext(root), Options{
		PromptInitialAdminPassword: func() ([]byte, error) {
			prompted = true
			return []byte("Unused!2026"), nil
		},
	})
	if report.Success || len(report.Results) != 2 || report.Results[1].ExitCode != 43 {
		t.Fatalf("unrelated migration failure = %#v", report)
	}
	if prompted {
		t.Fatal("setup prompted for an unrelated migration failure")
	}
}

func thinHostSetupContext(root string) *project.Context {
	ctx := &project.Context{Root: root}
	ctx.Project.Spec.RepositoryLayout = map[string]string{
		"kind":     "thin-host",
		"backend":  ".",
		"frontend": "web",
	}
	ctx.Project.Spec.Frontend.PackageManagerVersion = "10.34.5"
	ctx.Project.Spec.Frontend.DefaultApplication = "admin-web"
	ctx.Project.Spec.Frontend.Applications = []project.FrontendApplicationSpec{{
		ID:                    "admin-web",
		Path:                  "web",
		PackageManager:        "pnpm",
		PackageManagerVersion: "10.34.5",
	}}
	return ctx
}

func writeSetupTestExecutable(t *testing.T, directory, name, content string) {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
