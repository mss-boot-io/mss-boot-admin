package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/testevidence"
)

func TestRootRegistersTestEvidenceHelp(t *testing.T) {
	command := NewAgentRootCommand()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"test", "evidence", "--help"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute test evidence help: %v", err)
	}
	for _, value := range []string{"--directory", "--package", "--run", "--count", "--race", "--go-work", "--require"} {
		if !strings.Contains(stdout.String(), value) {
			t.Fatalf("help does not contain %s:\n%s", value, stdout.String())
		}
	}
}

func TestEvidenceCommandPassesRepeatedRequirementsAndWritesJSON(t *testing.T) {
	rootOverride := repositoryRoot(t)
	var received testevidence.Options
	runner := func(_ context.Context, options testevidence.Options) (testevidence.Report, error) {
		received = options
		return testevidence.Report{
			Success:  true,
			ExitCode: 0,
			Required: append([]string(nil), options.Required...),
			Tests: []testevidence.TestEvidence{
				{Name: "TestAlpha", Required: true, Expected: 2, Run: 2, Pass: 2, Success: true},
				{Name: "TestBeta", Required: true, Expected: 2, Run: 2, Pass: 2, Success: true},
			},
			Packages: []testevidence.PackageEvidence{},
		}, nil
	}
	command := newTestCommandWithRunner(&rootOverride, runner)
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{
		"evidence",
		"--directory", "internal/mss/testevidence",
		"--package", ".",
		"--run", "^(TestAlpha|TestBeta)$",
		"--count", "2",
		"--race",
		"--go-work", "off",
		"--require", "TestAlpha",
		"--require", "TestBeta",
	})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute successful evidence command: %v", err)
	}
	if !reflect.DeepEqual(received.Required, []string{"TestAlpha", "TestBeta"}) || received.Count != 2 || !received.Race || received.GoWork != testevidence.GoWorkOff {
		t.Fatalf("unexpected runner options: %#v", received)
	}
	var report testevidence.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode command JSON: %v\n%s", err, stdout.String())
	}
	if !report.Success || len(report.Tests) != 2 {
		t.Fatalf("unexpected command report: %#v", report)
	}
}

func TestEvidenceCommandReturnsNonZeroForNegativeEvidence(t *testing.T) {
	tests := []struct {
		name   string
		report testevidence.Report
	}{
		{
			name: "missing",
			report: testevidence.Report{
				ExitCode: 0,
				Tests:    []testevidence.TestEvidence{{Name: "TestExact", Required: true, Expected: 1}},
				Failures: []string{"required test TestExact run actions = 0, want 1"},
			},
		},
		{
			name: "skip",
			report: testevidence.Report{
				ExitCode: 0,
				Tests:    []testevidence.TestEvidence{{Name: "TestExact", Required: true, Expected: 1, Run: 1, Skip: 1}},
				Failures: []string{"test TestExact emitted 1 skip action(s)"},
			},
		},
		{
			name: "cached-only",
			report: testevidence.Report{
				ExitCode: 0,
				Cached:   true,
				Tests:    []testevidence.TestEvidence{{Name: "TestExact", Required: true, Expected: 1, Run: 1, Pass: 1}},
				Packages: []testevidence.PackageEvidence{{Name: "example.com/fixture", Pass: 1, Cached: true}},
				Failures: []string{"package example.com/fixture used cached test evidence"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rootOverride := repositoryRoot(t)
			runner := func(context.Context, testevidence.Options) (testevidence.Report, error) {
				return test.report, errors.New("Go test exact evidence failed")
			}
			command := newTestCommandWithRunner(&rootOverride, runner)
			var stdout bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&bytes.Buffer{})
			command.SilenceUsage = true
			command.SilenceErrors = true
			command.SetArgs([]string{
				"evidence",
				"--directory", ".",
				"--package", "./internal/mss/testevidence",
				"--run", "^TestExact$",
				"--require", "TestExact",
			})
			if err := command.ExecuteContext(context.Background()); err == nil {
				t.Fatalf("%s evidence must return a non-zero CLI result", test.name)
			}
			var report testevidence.Report
			if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
				t.Fatalf("decode negative JSON: %v\n%s", err, stdout.String())
			}
			if report.Success {
				t.Fatalf("negative command reported success: %#v", report)
			}
		})
	}
}

func TestEvidenceCommandRejectsRealMissingAndSkippedTests(t *testing.T) {
	tests := []struct {
		name     string
		testName string
		failure  string
	}{
		{
			name:     "missing",
			testName: "TestMissing",
			failure:  "run actions = 0, want 1",
		},
		{
			name:     "skipped",
			testName: "TestSkip",
			failure:  "emitted 1 skip action(s)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rootOverride := repositoryRoot(t)
			command := newTestCommandWithRunner(&rootOverride, testevidence.Run)
			var stdout bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&bytes.Buffer{})
			command.SilenceUsage = true
			command.SilenceErrors = true
			command.SetArgs([]string{
				"evidence",
				"--directory", "internal/mss/testevidence",
				"--package", "./testdata/fixture",
				"--run", "^" + test.testName + "$",
				"--require", test.testName,
			})
			if err := command.ExecuteContext(context.Background()); err == nil {
				t.Fatalf("real %s test selection must return a non-zero CLI result", test.name)
			}
			var report testevidence.Report
			if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
				t.Fatalf("decode real negative JSON: %v\n%s", err, stdout.String())
			}
			if report.Success || report.ExitCode != 0 {
				t.Fatalf("real negative must reject a native go test zero exit: %#v", report)
			}
			found := false
			for _, failure := range report.Failures {
				if strings.Contains(failure, test.failure) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("real negative failures do not contain %q: %#v", test.failure, report.Failures)
			}
		})
	}
}

func TestEvidenceCommandAcceptsRealSinglePackageEvidence(t *testing.T) {
	rootOverride := repositoryRoot(t)
	command := newTestCommandWithRunner(&rootOverride, testevidence.Run)
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&bytes.Buffer{})
	command.SilenceUsage = true
	command.SilenceErrors = true
	command.SetArgs([]string{
		"evidence",
		"--directory", "internal/mss/testevidence",
		"--package", "./testdata/fixture",
		"--run", "^TestPass$",
		"--count", "2",
		"--go-work", "off",
		"--require", "TestPass",
	})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute real single-package evidence: %v\n%s", err, stdout.String())
	}
	var report testevidence.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode real positive JSON: %v\n%s", err, stdout.String())
	}
	if !report.Success || len(report.Packages) != 1 || len(report.Tests) != 1 {
		t.Fatalf("unexpected real single-package report: %#v", report)
	}
	if report.Tests[0].Package != report.Packages[0].Name || report.Tests[0].Run != 2 || report.Tests[0].Pass != 2 {
		t.Fatalf("real evidence was not bound to its package: %#v", report)
	}
}

func TestEvidenceCommandRejectsRealOutOfScopePackageSelections(t *testing.T) {
	rootOverride := repositoryRoot(t)
	tests := []struct {
		name    string
		pkg     string
		failure string
	}{
		{
			name:    "absolute",
			pkg:     filepath.Join(rootOverride, "internal", "mss", "testevidence", "testdata", "fixture"),
			failure: "must be repository-relative",
		},
		{
			name:    "external import",
			pkg:     "github.com/spf13/cobra",
			failure: "external import paths are not allowed",
		},
		{
			name:    "multi-package pattern",
			pkg:     "./...",
			failure: "ellipsis patterns are not allowed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := newTestCommandWithRunner(&rootOverride, testevidence.Run)
			var stdout bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&bytes.Buffer{})
			command.SilenceUsage = true
			command.SilenceErrors = true
			command.SetArgs([]string{
				"evidence",
				"--directory", "internal/mss/testevidence",
				"--package", test.pkg,
				"--run", "^TestPass$",
				"--go-work", "off",
				"--require", "TestPass",
			})
			if err := command.ExecuteContext(context.Background()); err == nil {
				t.Fatalf("out-of-scope package selection %q must fail", test.pkg)
			}
			var report testevidence.Report
			if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
				t.Fatalf("decode real package-selection failure: %v\n%s", err, stdout.String())
			}
			if report.Success {
				t.Fatalf("out-of-scope package selection reported success: %#v", report)
			}
			found := false
			for _, failure := range report.Failures {
				if strings.Contains(failure, test.failure) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("failures do not contain %q: %#v", test.failure, report.Failures)
			}
			if strings.Contains(stdout.String(), rootOverride) {
				t.Fatalf("negative report leaked repository root: %s", stdout.String())
			}
		})
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	projectContext, err := project.Load(".")
	if err != nil {
		t.Fatalf("load repository root: %v", err)
	}
	return projectContext.Root
}
