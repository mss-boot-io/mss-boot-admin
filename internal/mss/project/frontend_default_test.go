package project_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/dev"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/setup"
)

func TestRepositorySupportsOnlyTheIndependentV6Frontend(t *testing.T) {
	ctx, err := project.Load(".")
	if err != nil {
		t.Fatalf("load repository project contract: %v", err)
	}
	primary, ok := ctx.DefaultFrontendApplication()
	if !ok {
		t.Fatal("project contract did not resolve a default frontend application")
	}
	if primary.ID != "antd-v6" || primary.Path != "web/antd-v6" || primary.Role != "primary" {
		t.Fatalf("primary frontend = %#v, want the independent V6 application", primary)
	}
	if got := ctx.Project.Spec.RepositoryLayout["frontend"]; got != primary.Path {
		t.Fatalf("repositoryLayout.frontend = %q, want %q", got, primary.Path)
	}

	if applications := ctx.Project.Spec.Frontend.Applications; len(applications) != 1 || applications[0] != primary {
		t.Fatalf("frontend applications = %#v, want only V6", applications)
	}

	steps := setup.Plan(ctx, setup.Options{})
	var frontendDirectory string
	var frontendArgs []string
	for _, step := range steps {
		if step.ID == "frontend-dependencies" {
			frontendDirectory = step.Directory
			frontendArgs = step.Args
			break
		}
	}
	if frontendDirectory != filepath.Join(ctx.Root, filepath.FromSlash(primary.Path)) {
		t.Fatalf("setup frontend directory = %q, want primary V6 directory", frontendDirectory)
	}
	wantArgs := []string{"corepack", "pnpm@10.34.5", "install", "--frozen-lockfile"}
	if !reflect.DeepEqual(frontendArgs, wantArgs) {
		t.Fatalf("setup frontend command = %#v, want %#v", frontendArgs, wantArgs)
	}

	development, err := dev.Load(ctx.Root)
	if err != nil {
		t.Fatalf("load development contract: %v", err)
	}
	services, err := development.StartServices(nil)
	if err != nil {
		t.Fatalf("resolve default development services: %v", err)
	}
	if len(services) != 2 || services[0].ID != "backend" || services[1].ID != "frontend" {
		t.Fatalf("default development services = %#v, want backend then primary frontend", services)
	}
	if services[1].Directory != primary.Path || services[1].Health == nil ||
		services[1].Health.URL != "http://127.0.0.1:8001/" {
		t.Fatalf("default frontend development service = %#v", services[1])
	}
	if _, err := development.StartServices([]string{"frontend-v5"}); err == nil {
		t.Fatal("retired V5 development service is still selectable")
	}

	composePath := filepath.Join(ctx.Root, "compose", "admin", "docker-compose.yml")
	composeData, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read primary frontend compose contract: %v", err)
	}
	compose := string(composeData)
	for _, required := range []string{
		"MSS_FRONTEND_V6_IMAGE",
		"../../web/antd-v6",
		"MSS_FRONTEND_V6_PORT:-8001",
	} {
		if !strings.Contains(compose, required) {
			t.Errorf("primary frontend compose contract is missing %q", required)
		}
	}
	if strings.Contains(compose, "../../web/antd\n") || strings.Contains(compose, "mss-boot-admin-antd:local") {
		t.Fatal("primary frontend compose contract still selects the V5 application")
	}
}
