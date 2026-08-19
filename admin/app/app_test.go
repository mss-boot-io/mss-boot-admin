package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/modules/supplier"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
)

func TestApplicationsConstructWithoutRegistryPollution(t *testing.T) {
	coreClone, err := migration.Migrate.CloneRegistrations()
	if err != nil {
		t.Fatalf("clone core migrations before construction: %v", err)
	}
	if err := supplier.RegisterMigration(coreClone); err != nil {
		t.Fatalf("Supplier was already hidden in global migration state: %v", err)
	}

	first, err := New(WithBusinessModules(supplier.Module()))
	if err != nil {
		t.Fatalf("construct first Application: %v", err)
	}
	second, err := New(WithBusinessModules(supplier.Module()))
	if err != nil {
		t.Fatalf("construct second Application: %v", err)
	}
	for name, application := range map[string]*Application{"first": first, "second": second} {
		descriptors := application.BusinessDescriptors()
		if len(descriptors) != 1 || descriptors[0].Name != "supplier" {
			t.Fatalf("%s descriptors = %#v", name, descriptors)
		}
	}
	afterConstruction, err := migration.Migrate.CloneRegistrations()
	if err != nil {
		t.Fatalf("clone core migrations after construction: %v", err)
	}
	if err := supplier.RegisterMigration(afterConstruction); err != nil {
		t.Fatalf("Application construction polluted global migrations: %v", err)
	}
}

func TestCommandTreesKeepHelpAndFlagsIsolated(t *testing.T) {
	application, err := New(WithBusinessModules(supplier.Module()))
	if err != nil {
		t.Fatalf("construct Application: %v", err)
	}
	for _, args := range [][]string{{"--help"}, {"server", "--help"}, {"migrate", "--help"}} {
		command, commandErr := application.Command()
		if commandErr != nil {
			t.Fatalf("construct command %v: %v", args, commandErr)
		}
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&output)
		command.SetArgs(args)
		if err := command.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("execute help %v: %v", args, err)
		}
		if !strings.Contains(output.String(), "Usage:") {
			t.Fatalf("help %v output = %q", args, output.String())
		}
	}
}

func TestApplicationExecutionIsSingleUseAndReturnsErrors(t *testing.T) {
	application, err := New()
	if err != nil {
		t.Fatalf("construct Application: %v", err)
	}
	if err := application.ExecuteArgsContext(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("execute Application help: %v", err)
	}
	if err := application.ExecuteArgsContext(context.Background(), []string{"--help"}); !errors.Is(err, ErrApplicationExecuted) {
		t.Fatalf("second execution error = %v, want already executed", err)
	}

	blocked, err := New()
	if err != nil {
		t.Fatalf("construct blocked Application: %v", err)
	}
	applicationRunning.Store(true)
	if err := blocked.ExecuteArgsContext(context.Background(), []string{"--help"}); !errors.Is(err, ErrApplicationRunning) {
		t.Fatalf("concurrent execution error = %v, want already running", err)
	}
	applicationRunning.Store(false)
	if err := blocked.ExecuteArgsContext(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("blocked Application was consumed by concurrency rejection: %v", err)
	}
}
