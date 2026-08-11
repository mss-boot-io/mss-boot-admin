package testevidence

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/command"
)

func TestRunRejectsMissingRequiredTestEvidence(t *testing.T) {
	options := validOptions(t, 1, false, "TestMissing")
	result := successfulResult(packageEvent("start"), packageEvent("pass"))

	report, err := run(context.Background(), options, returning(result))
	if err == nil {
		t.Fatal("missing required test evidence must fail")
	}
	if report.ExitCode != 0 {
		t.Fatalf("go test exit code = %d, want 0 to cover the native zero-hit false green", report.ExitCode)
	}
	test := findTest(t, report, "TestMissing")
	if test.Run != 0 || test.Pass != 0 || test.Success {
		t.Fatalf("unexpected missing-test evidence: %#v", test)
	}
	assertFailureContains(t, report, "required test TestMissing run actions = 0, want 1")
}

func TestRunRejectsSkippedTest(t *testing.T) {
	options := validOptions(t, 1, false, "TestSkipped")
	result := successfulResult(
		packageEvent("start"),
		testEvent("run", "TestSkipped"),
		testEvent("skip", "TestSkipped"),
		packageEvent("pass"),
	)

	report, err := run(context.Background(), options, returning(result))
	if err == nil {
		t.Fatal("skipped required test must fail evidence")
	}
	test := findTest(t, report, "TestSkipped")
	if test.Skip != 1 || test.Success {
		t.Fatalf("unexpected skipped-test evidence: %#v", test)
	}
	assertFailureContains(t, report, "test TestSkipped emitted 1 skip action(s)")
}

func TestRunRejectsFailedTestAndPackage(t *testing.T) {
	options := validOptions(t, 1, false, "TestFailed")
	result := command.Result{
		ExitCode: 1,
		Error:    "exit status 1",
		Stdout: events(
			packageEvent("start"),
			testEvent("run", "TestFailed"),
			testEvent("fail", "TestFailed"),
			packageEvent("fail"),
		),
	}

	report, err := run(context.Background(), options, returning(result))
	if err == nil {
		t.Fatal("failed test and package must fail evidence")
	}
	if report.ExitCode != 1 || findTest(t, report, "TestFailed").Fail != 1 {
		t.Fatalf("unexpected failed evidence: %#v", report)
	}
	assertFailureContains(t, report, "package example.com/evidence emitted 1 fail action(s)")
}

func TestRunCountMatrixRequiresExactActions(t *testing.T) {
	for _, count := range []int{1, 2, 5, 20} {
		t.Run(fmt.Sprintf("count-%d", count), func(t *testing.T) {
			options := validOptions(t, count, count%2 == 0, "TestExact")
			eventList := []goTestEvent{packageEvent("start")}
			for index := 0; index < count; index++ {
				eventList = append(eventList, testEvent("run", "TestExact"), testEvent("pass", "TestExact"))
			}
			eventList = append(eventList, packageEvent("pass"))
			var captured command.Spec
			execute := func(_ context.Context, spec command.Spec) command.Result {
				if isGoListCommand(spec) {
					return successfulGoListResult(spec.Directory, "example.com/evidence")
				}
				captured = spec
				return successfulResult(eventList...)
			}

			report, err := run(context.Background(), options, execute)
			if err != nil {
				t.Fatalf("exact evidence failed: %v; report = %#v", err, report)
			}
			test := findTest(t, report, "TestExact")
			if !report.Success || !test.Success || test.Run != count || test.Pass != count {
				t.Fatalf("unexpected count evidence: %#v", report)
			}
			wantArgs := []string{"go", "test", "-json", fmt.Sprintf("-count=%d", count), "-run", "^TestExact$"}
			if options.Race {
				wantArgs = append(wantArgs, "-race")
			}
			wantArgs = append(wantArgs, "./fixture")
			if !reflect.DeepEqual(captured.Args, wantArgs) {
				t.Fatalf("command args = %#v, want %#v", captured.Args, wantArgs)
			}
			if captured.Directory != options.Root {
				t.Fatalf("command directory = %q, want %q", captured.Directory, options.Root)
			}
		})
	}
}

func TestRunAcceptsSuccessfulExactSuite(t *testing.T) {
	options := validOptions(t, 2, true, "TestAlpha", "TestBeta")
	options.Run = "^(TestAlpha|TestBeta)$"
	result := successfulResult(
		packageEvent("start"),
		testEvent("run", "TestAlpha"), testEvent("pass", "TestAlpha"),
		testEvent("run", "TestBeta"), testEvent("pass", "TestBeta"),
		testEvent("run", "TestAlpha"), testEvent("pass", "TestAlpha"),
		testEvent("run", "TestBeta"), testEvent("pass", "TestBeta"),
		packageEvent("pass"),
	)

	report, err := run(context.Background(), options, returning(result))
	if err != nil {
		t.Fatalf("run exact suite: %v; report = %#v", err, report)
	}
	if !report.Success || report.Cached || len(report.Failures) != 0 || len(report.Tests) != 2 {
		t.Fatalf("unexpected successful report: %#v", report)
	}
	if report.Directory != "." {
		t.Fatalf("report directory = %q, want repository-relative dot", report.Directory)
	}
	data, err := report.JSON()
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	for _, field := range []string{`"command"`, `"exitCode"`, `"run"`, `"pass"`, `"skip"`, `"fail"`} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("JSON report does not contain %s: %s", field, data)
		}
	}
	if strings.Contains(string(data), options.Root) {
		t.Fatalf("JSON report leaked absolute repository path: %s", data)
	}
}

func TestRunGoWorkOffUsesOnlyRestrictedOverride(t *testing.T) {
	options := validOptions(t, 1, false, "TestExact")
	options.GoWork = GoWorkOff
	result := successfulResult(
		packageEvent("start"),
		testEvent("run", "TestExact"),
		testEvent("pass", "TestExact"),
		packageEvent("pass"),
	)
	var captured command.Spec
	report, err := run(context.Background(), options, func(_ context.Context, spec command.Spec) command.Result {
		if isGoListCommand(spec) {
			if !reflect.DeepEqual(spec.Environment, map[string]string{"GOWORK": "off"}) {
				t.Fatalf("go list GOWORK override = %#v", spec.Environment)
			}
			return successfulGoListResult(spec.Directory, "example.com/evidence")
		}
		captured = spec
		return result
	})
	if err != nil {
		t.Fatalf("run GOWORK=off evidence: %v; report = %#v", err, report)
	}
	want := map[string]string{"GOWORK": "off"}
	if !reflect.DeepEqual(captured.Environment, want) || !reflect.DeepEqual(report.Environment, want) {
		t.Fatalf("restricted environment = spec %#v report %#v, want %#v", captured.Environment, report.Environment, want)
	}
	if report.GoWork != GoWorkOff {
		t.Fatalf("goWork = %q, want %q", report.GoWork, GoWorkOff)
	}
}

func TestRunRejectsMalformedJSON(t *testing.T) {
	options := validOptions(t, 1, false, "TestExact")
	result := successfulResult(packageEvent("start"), testEvent("run", "TestExact"))
	result.Stdout += "{not-json\n"

	report, err := run(context.Background(), options, returning(result))
	if err == nil {
		t.Fatal("malformed go test JSON must fail evidence")
	}
	if report.ParseError == "" {
		t.Fatalf("parse error was not reported: %#v", report)
	}
}

func TestRunRejectsCommandFailure(t *testing.T) {
	options := validOptions(t, 1, false, "TestExact")
	result := command.Result{ExitCode: -1, Error: "executable file not found", Stderr: "go unavailable"}

	report, err := run(context.Background(), options, returning(result))
	if err == nil {
		t.Fatal("command execution failure must fail evidence")
	}
	if report.ExitCode != -1 || report.CommandError != result.Error || report.Stderr != result.Stderr {
		t.Fatalf("unexpected command failure report: %#v", report)
	}
}

func TestRunRejectsCachedEvidence(t *testing.T) {
	options := validOptions(t, 1, false, "TestExact")
	result := successfulResult(
		packageEvent("start"),
		testEvent("run", "TestExact"),
		testEvent("pass", "TestExact"),
		goTestEvent{Action: "output", Package: "example.com/evidence", Output: "ok  \texample.com/evidence\t(cached)\n"},
		packageEvent("pass"),
	)

	report, err := run(context.Background(), options, returning(result))
	if err == nil {
		t.Fatal("cached evidence must fail")
	}
	if !report.Cached || !report.Packages[0].Cached {
		t.Fatalf("cached package was not reported: %#v", report)
	}
}

func TestRunRejectsInvalidSelectionsBeforeExecution(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Options)
		failure string
	}{
		{
			name: "no require",
			mutate: func(options *Options) {
				options.Required = nil
			},
			failure: "at least one required top-level test name is required",
		},
		{
			name: "duplicate require",
			mutate: func(options *Options) {
				options.Required = []string{"TestExact", "TestExact"}
			},
			failure: `duplicate required test name "TestExact"`,
		},
		{
			name: "invalid test name",
			mutate: func(options *Options) {
				options.Required = []string{"Testlower"}
				options.Run = "^Testlower$"
			},
			failure: `invalid top-level test name "Testlower"`,
		},
		{
			name: "unanchored expression",
			mutate: func(options *Options) {
				options.Run = "TestExact"
			},
			failure: "run pattern must anchor every alternative",
		},
		{
			name: "partially anchored alternative",
			mutate: func(options *Options) {
				options.Run = "^TestExact$|TestOther$"
			},
			failure: "run pattern must anchor every alternative",
		},
		{
			name: "pattern excludes required",
			mutate: func(options *Options) {
				options.Run = "^TestOther$"
			},
			failure: `run pattern does not select required test "TestExact"`,
		},
		{
			name: "absolute directory",
			mutate: func(options *Options) {
				options.Directory = options.Root
			},
			failure: "directory must be repository-relative",
		},
		{
			name: "unclean directory",
			mutate: func(options *Options) {
				options.Directory = "./"
			},
			failure: "is not a clean relative path",
		},
		{
			name: "escaping directory",
			mutate: func(options *Options) {
				options.Directory = ".."
			},
			failure: "escapes repository root",
		},
		{
			name: "missing directory",
			mutate: func(options *Options) {
				options.Directory = "missing"
			},
			failure: "resolve test directory",
		},
		{
			name: "package shell metacharacter",
			mutate: func(options *Options) {
				options.Package = "./fixture;echo"
			},
			failure: `invalid package argument "./fixture;echo"`,
		},
		{
			name: "package parent segment",
			mutate: func(options *Options) {
				options.Package = "../fixture"
			},
			failure: `must be "." or start with "./"`,
		},
		{
			name: "absolute package",
			mutate: func(options *Options) {
				options.Package = filepath.Join(options.Root, "fixture")
			},
			failure: "must be repository-relative",
		},
		{
			name: "external import package",
			mutate: func(options *Options) {
				options.Package = "github.com/spf13/cobra"
			},
			failure: "external import paths are not allowed",
		},
		{
			name: "multi-package ellipsis",
			mutate: func(options *Options) {
				options.Package = "./fixture/..."
			},
			failure: "ellipsis patterns are not allowed",
		},
		{
			name: "invalid go work mode",
			mutate: func(options *Options) {
				options.GoWork = "custom"
			},
			failure: `go-work must be "auto" or "off"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := validOptions(t, 1, false, "TestExact")
			test.mutate(&options)
			called := false
			report, err := run(context.Background(), options, func(context.Context, command.Spec) command.Result {
				called = true
				return command.Result{}
			})
			if err == nil {
				t.Fatal("invalid selection must fail")
			}
			if called {
				t.Fatal("invalid selection executed an external command")
			}
			assertFailureContains(t, report, test.failure)
			for _, failure := range report.Failures {
				if strings.Contains(failure, options.Root) {
					t.Fatalf("validation failure leaked absolute repository path: %q", failure)
				}
			}
		})
	}
}

func TestRunRejectsMultipleResolvedPackagesBeforeTesting(t *testing.T) {
	options := validOptions(t, 1, false, "TestExact")
	testCalled := false
	execute := func(_ context.Context, spec command.Spec) command.Result {
		if !isGoListCommand(spec) {
			testCalled = true
			return command.Result{}
		}
		var builder strings.Builder
		encoder := json.NewEncoder(&builder)
		for _, pkg := range []goListPackage{
			{Dir: options.Root, ImportPath: "example.com/first"},
			{Dir: options.Root, ImportPath: "example.com/second"},
		} {
			if err := encoder.Encode(pkg); err != nil {
				t.Fatalf("encode go list package: %v", err)
			}
		}
		return command.Result{ExitCode: 0, Stdout: builder.String()}
	}

	report, err := run(context.Background(), options, execute)
	if err == nil {
		t.Fatal("multiple resolved packages must fail evidence")
	}
	if testCalled {
		t.Fatal("multiple resolved packages reached go test")
	}
	assertFailureContains(t, report, "resolved 2 packages, want exactly 1")
}

func TestRunRejectsResolvedPackageOutsideWorkingDirectory(t *testing.T) {
	options := validOptions(t, 1, false, "TestExact")
	outside := t.TempDir()
	testCalled := false
	execute := func(_ context.Context, spec command.Spec) command.Result {
		if !isGoListCommand(spec) {
			testCalled = true
			return command.Result{}
		}
		return successfulGoListResult(outside, "example.com/outside")
	}

	report, err := run(context.Background(), options, execute)
	if err == nil {
		t.Fatal("package outside the working directory must fail evidence")
	}
	if testCalled {
		t.Fatal("out-of-scope package reached go test")
	}
	assertFailureContains(t, report, "resolved outside the selected repository working directory")
}

func TestRunBindsRequiredEvidenceToResolvedPackage(t *testing.T) {
	options := validOptions(t, 1, false, "TestExact")
	execute := func(_ context.Context, spec command.Spec) command.Result {
		if isGoListCommand(spec) {
			return successfulGoListResult(spec.Directory, "example.com/target")
		}
		return successfulResult(
			goTestEvent{Action: "start", Package: "example.com/other"},
			goTestEvent{Action: "run", Package: "example.com/other", Test: "TestExact"},
			goTestEvent{Action: "pass", Package: "example.com/other", Test: "TestExact"},
			goTestEvent{Action: "pass", Package: "example.com/other"},
		)
	}

	report, err := run(context.Background(), options, execute)
	if err == nil {
		t.Fatal("same-named test from a different package must not satisfy required evidence")
	}
	test := findRequiredTest(t, report, "TestExact")
	if test.Package != "example.com/target" || test.Run != 0 || test.Pass != 0 || test.Success {
		t.Fatalf("wrong-package evidence satisfied the requirement: %#v", test)
	}
	assertFailureContains(t, report, "go test emitted package example.com/other, want resolved package example.com/target")
}

func TestRunRedactsAllReportStringFields(t *testing.T) {
	options := validOptions(t, 1, false, "TestExact")
	options.Run = "^(" + regexp.QuoteMeta(options.Root) + "|TestExact)$"
	result := successfulResult(
		packageEvent("start"),
		testEvent("run", "TestExact"),
		testEvent("pass", "TestExact"),
		packageEvent("pass"),
	)
	report, err := run(context.Background(), options, returning(result))
	if err != nil {
		t.Fatalf("run redaction evidence: %v; report = %#v", err, report)
	}
	data, err := report.JSON()
	if err != nil {
		t.Fatalf("marshal redacted report: %v", err)
	}
	if strings.Contains(string(data), options.Root) {
		t.Fatalf("report leaked repository root: %s", data)
	}
	if !strings.Contains(string(data), "$REPO") {
		t.Fatalf("report did not contain the repository redaction marker: %s", data)
	}
}

func validOptions(t *testing.T, count int, race bool, required ...string) Options {
	t.Helper()
	root := t.TempDir()
	pattern := "^(" + strings.Join(required, "|") + ")$"
	if len(required) == 1 {
		pattern = "^" + required[0] + "$"
	}
	return Options{
		Root:      root,
		Directory: ".",
		Package:   "./fixture",
		Run:       pattern,
		Count:     count,
		Race:      race,
		Required:  append([]string(nil), required...),
	}
}

func returning(result command.Result) executeCommand {
	return func(_ context.Context, spec command.Spec) command.Result {
		if isGoListCommand(spec) {
			return successfulGoListResult(spec.Directory, "example.com/evidence")
		}
		return result
	}
}

func isGoListCommand(spec command.Spec) bool {
	return len(spec.Args) >= 2 && spec.Args[0] == "go" && spec.Args[1] == "list"
}

func successfulGoListResult(directory, importPath string) command.Result {
	var builder strings.Builder
	if err := json.NewEncoder(&builder).Encode(goListPackage{Dir: directory, ImportPath: importPath}); err != nil {
		panic(err)
	}
	return command.Result{ExitCode: 0, Stdout: builder.String()}
}

func successfulResult(eventList ...goTestEvent) command.Result {
	return command.Result{ExitCode: 0, Stdout: events(eventList...)}
}

func events(eventList ...goTestEvent) string {
	var builder strings.Builder
	encoder := json.NewEncoder(&builder)
	for _, event := range eventList {
		if err := encoder.Encode(event); err != nil {
			panic(err)
		}
	}
	return builder.String()
}

func packageEvent(action string) goTestEvent {
	return goTestEvent{Action: action, Package: "example.com/evidence"}
}

func testEvent(action, name string) goTestEvent {
	return goTestEvent{Action: action, Package: "example.com/evidence", Test: name}
}

func findTest(t *testing.T, report Report, name string) TestEvidence {
	t.Helper()
	for _, evidence := range report.Tests {
		if evidence.Name == name {
			return evidence
		}
	}
	t.Fatalf("test %q not found in report: %#v", name, report.Tests)
	return TestEvidence{}
}

func findRequiredTest(t *testing.T, report Report, name string) TestEvidence {
	t.Helper()
	for _, evidence := range report.Tests {
		if evidence.Name == name && evidence.Required {
			return evidence
		}
	}
	t.Fatalf("required test %q not found in report: %#v", name, report.Tests)
	return TestEvidence{}
}

func assertFailureContains(t *testing.T, report Report, want string) {
	t.Helper()
	for _, failure := range report.Failures {
		if strings.Contains(failure, want) {
			return
		}
	}
	t.Fatalf("report failures do not contain %q: %#v", want, report.Failures)
}
