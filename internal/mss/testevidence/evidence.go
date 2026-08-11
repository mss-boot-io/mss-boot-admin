// Package testevidence runs a focused Go test command and verifies that every
// required top-level test produced fresh, exact JSON evidence.
package testevidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/command"
)

const defaultTimeout = 30 * time.Minute

const (
	// GoWorkAuto inherits the caller's Go workspace selection.
	GoWorkAuto = "auto"
	// GoWorkOff forces independent module validation with GOWORK=off.
	GoWorkOff = "off"
)

// Options selects one exact Go test evidence run.
type Options struct {
	Root      string
	Directory string
	Package   string
	Run       string
	Count     int
	Race      bool
	GoWork    string
	Required  []string
}

var packageArgumentPattern = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

// Report is the machine-readable evidence emitted by the CLI.
type Report struct {
	GeneratedAt  time.Time         `json:"generatedAt"`
	Success      bool              `json:"success"`
	Directory    string            `json:"directory,omitempty"`
	Package      string            `json:"package"`
	RunPattern   string            `json:"runPattern"`
	Count        int               `json:"count"`
	Race         bool              `json:"race"`
	GoWork       string            `json:"goWork"`
	Required     []string          `json:"required"`
	Command      []string          `json:"command,omitempty"`
	Environment  map[string]string `json:"environment"`
	ExitCode     int               `json:"exitCode"`
	CommandError string            `json:"commandError,omitempty"`
	Stderr       string            `json:"stderr,omitempty"`
	ParseError   string            `json:"parseError,omitempty"`
	Cached       bool              `json:"cached"`
	Tests        []TestEvidence    `json:"tests"`
	Packages     []PackageEvidence `json:"packages"`
	Failures     []string          `json:"failures,omitempty"`
}

// TestEvidence contains all observed JSON actions for one test or subtest.
type TestEvidence struct {
	Package  string `json:"package"`
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Expected int    `json:"expected,omitempty"`
	Run      int    `json:"run"`
	Pass     int    `json:"pass"`
	Skip     int    `json:"skip"`
	Fail     int    `json:"fail"`
	Success  bool   `json:"success"`
}

// PackageEvidence contains terminal and cache evidence for one package.
type PackageEvidence struct {
	Name   string `json:"name"`
	Start  int    `json:"start"`
	Pass   int    `json:"pass"`
	Skip   int    `json:"skip"`
	Fail   int    `json:"fail"`
	Cached bool   `json:"cached"`
}

// JSON renders a stable, indented report for agents and CI.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

type executeCommand func(context.Context, command.Spec) command.Result

// Run executes go test directly, parses its JSON event stream, and requires
// exact run/pass counts for every explicitly required top-level test.
func Run(ctx context.Context, options Options) (Report, error) {
	return run(ctx, options, command.Run)
}

func run(ctx context.Context, options Options, execute executeCommand) (Report, error) {
	if options.GoWork == "" {
		options.GoWork = GoWorkAuto
	}
	report := Report{
		GeneratedAt: time.Now().UTC(),
		Package:     options.Package,
		RunPattern:  options.Run,
		Count:       options.Count,
		Race:        options.Race,
		GoWork:      options.GoWork,
		Required:    append([]string(nil), options.Required...),
		Environment: map[string]string{},
		ExitCode:    -1,
		Tests:       []TestEvidence{},
		Packages:    []PackageEvidence{},
	}

	reportDirectory, workingDirectory, failures := validateOptions(options)
	report.Directory = reportDirectory
	report.Failures = append(report.Failures, failures...)
	redactReport(&report, options.Root)
	if len(report.Failures) > 0 {
		normalizeReport(&report)
		return report, errors.New("Go test evidence options are invalid")
	}
	if options.GoWork == GoWorkOff {
		report.Environment["GOWORK"] = "off"
	}

	selection, listResult, err := resolveSinglePackage(ctx, workingDirectory, options.Package, report.Environment, execute)
	if err != nil {
		report.Command = append([]string(nil), listResult.Args...)
		report.ExitCode = listResult.ExitCode
		report.CommandError = listResult.Error
		report.Stderr = truncate(listResult.Stderr, 64*1024)
		report.Failures = append(report.Failures, err.Error())
		redactReport(&report, options.Root)
		normalizeReport(&report)
		return report, errors.New("Go test evidence package resolution failed")
	}

	args := []string{
		"go",
		"test",
		"-json",
		fmt.Sprintf("-count=%d", options.Count),
		"-run",
		options.Run,
	}
	if options.Race {
		args = append(args, "-race")
	}
	args = append(args, options.Package)
	report.Command = append([]string(nil), args...)

	result := execute(ctx, command.Spec{
		ID:          "go-test-exact-evidence",
		Description: "run a focused Go test suite and require exact JSON evidence",
		Directory:   workingDirectory,
		Args:        args,
		Environment: cloneEnvironment(report.Environment),
		Timeout:     defaultTimeout,
	})
	report.ExitCode = result.ExitCode
	report.CommandError = redactRepositoryPaths(result.Error, options.Root)
	report.Stderr = truncate(redactRepositoryPaths(result.Stderr, options.Root), 64*1024)

	tests, packages, parseErr := parseEvents(strings.NewReader(result.Stdout), selection.ImportPath, options.Required, options.Count)
	report.Tests = tests
	report.Packages = packages
	if parseErr != nil {
		report.ParseError = parseErr.Error()
		report.Failures = append(report.Failures, "go test stdout is not a complete JSON event stream: "+parseErr.Error())
	}
	if result.ExitCode != 0 {
		if report.CommandError != "" {
			report.Failures = append(report.Failures, fmt.Sprintf("go test command failed with exit code %d: %s", result.ExitCode, report.CommandError))
		} else {
			report.Failures = append(report.Failures, fmt.Sprintf("go test command failed with exit code %d", result.ExitCode))
		}
	}
	if report.CommandError != "" && result.ExitCode == 0 {
		report.Failures = append(report.Failures, "go test command reported an execution error: "+report.CommandError)
	}

	for index := range report.Tests {
		test := &report.Tests[index]
		if test.Required {
			test.Success = test.Run == test.Expected && test.Pass == test.Expected && test.Skip == 0 && test.Fail == 0
			if test.Run != test.Expected {
				report.Failures = append(report.Failures, fmt.Sprintf("required test %s run actions = %d, want %d", test.Name, test.Run, test.Expected))
			}
			if test.Pass != test.Expected {
				report.Failures = append(report.Failures, fmt.Sprintf("required test %s pass actions = %d, want %d", test.Name, test.Pass, test.Expected))
			}
		} else {
			test.Success = test.Skip == 0 && test.Fail == 0
		}
		if test.Skip > 0 {
			report.Failures = append(report.Failures, fmt.Sprintf("test %s emitted %d skip action(s)", test.Name, test.Skip))
		}
		if test.Fail > 0 {
			report.Failures = append(report.Failures, fmt.Sprintf("test %s emitted %d fail action(s)", test.Name, test.Fail))
		}
	}

	if len(report.Packages) != 1 {
		report.Failures = append(report.Failures, fmt.Sprintf("go test emitted events for %d packages, want exactly 1", len(report.Packages)))
	} else if report.Packages[0].Name != selection.ImportPath {
		report.Failures = append(report.Failures, fmt.Sprintf("go test emitted package %s, want resolved package %s", report.Packages[0].Name, selection.ImportPath))
	}
	for _, pkg := range report.Packages {
		if pkg.Start != 1 {
			report.Failures = append(report.Failures, fmt.Sprintf("package %s start actions = %d, want 1", pkg.Name, pkg.Start))
		}
		if pkg.Cached {
			report.Cached = true
			report.Failures = append(report.Failures, fmt.Sprintf("package %s used cached test evidence", pkg.Name))
		}
		if pkg.Fail > 0 {
			report.Failures = append(report.Failures, fmt.Sprintf("package %s emitted %d fail action(s)", pkg.Name, pkg.Fail))
		}
		if pkg.Skip > 0 {
			report.Failures = append(report.Failures, fmt.Sprintf("package %s emitted %d skip action(s)", pkg.Name, pkg.Skip))
		}
		if pkg.Pass != 1 {
			report.Failures = append(report.Failures, fmt.Sprintf("package %s pass actions = %d, want 1", pkg.Name, pkg.Pass))
		}
	}

	redactReport(&report, options.Root)
	normalizeReport(&report)
	report.Success = len(report.Failures) == 0
	if !report.Success {
		return report, errors.New("Go test exact evidence failed")
	}
	return report, nil
}

func validateOptions(options Options) (string, string, []string) {
	var failures []string
	reportDirectory, workingDirectory, err := resolveDirectory(options.Root, options.Directory)
	if err != nil {
		failures = append(failures, err.Error())
	}
	if err := validatePackageArgument(options.Package); err != nil {
		failures = append(failures, err.Error())
	}
	if options.Count < 1 {
		failures = append(failures, "count must be greater than zero")
	}
	if options.GoWork != GoWorkAuto && options.GoWork != GoWorkOff {
		failures = append(failures, fmt.Sprintf("go-work must be %q or %q", GoWorkAuto, GoWorkOff))
	}

	compiled, err := regexp.Compile(options.Run)
	if strings.TrimSpace(options.Run) == "" {
		failures = append(failures, "run pattern is required")
	} else if err != nil {
		failures = append(failures, "invalid run pattern: "+err.Error())
	} else if !fullyAnchored(options.Run) {
		failures = append(failures, "run pattern must anchor every alternative at the beginning and end of text")
	}

	if len(options.Required) == 0 {
		failures = append(failures, "at least one required top-level test name is required")
	}
	seen := make(map[string]bool, len(options.Required))
	for _, name := range options.Required {
		if !isTopLevelTestName(name) {
			failures = append(failures, fmt.Sprintf("invalid top-level test name %q", name))
		}
		if seen[name] {
			failures = append(failures, fmt.Sprintf("duplicate required test name %q", name))
		}
		seen[name] = true
		if compiled != nil && !compiled.MatchString(name) {
			failures = append(failures, fmt.Sprintf("run pattern does not select required test %q", name))
		}
	}
	sort.Strings(failures)
	return reportDirectory, workingDirectory, failures
}

func validatePackageArgument(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("package is required")
	}
	if value != strings.TrimSpace(value) || strings.HasPrefix(value, "-") || !packageArgumentPattern.MatchString(value) {
		return fmt.Errorf("invalid package argument %q", value)
	}
	if filepath.IsAbs(value) {
		return fmt.Errorf("package argument %q must be repository-relative", value)
	}
	if strings.Contains(value, `\`) {
		return fmt.Errorf("package argument %q must use slash-separated repository-relative form", value)
	}
	if value != "." && !strings.HasPrefix(value, "./") {
		return fmt.Errorf("package argument %q must be %q or start with %q; external import paths are not allowed", value, ".", "./")
	}
	if strings.Contains(value, "...") {
		return fmt.Errorf("package argument %q must resolve one package; ellipsis patterns are not allowed", value)
	}
	if hasParentSegment(value) {
		return fmt.Errorf("package argument %q must not contain a parent segment", value)
	}
	if value == "." {
		return nil
	}
	tail := strings.TrimPrefix(value, "./")
	cleanTail := filepath.ToSlash(filepath.Clean(filepath.FromSlash(tail)))
	canonical := "."
	if cleanTail != "." {
		canonical = "./" + cleanTail
	}
	if value != canonical {
		return fmt.Errorf("package argument %q is not a clean relative path; use %q", value, canonical)
	}
	return nil
}

type goListPackage struct {
	Dir        string `json:"Dir"`
	ImportPath string `json:"ImportPath"`
}

func resolveSinglePackage(
	ctx context.Context,
	workingDirectory string,
	packageArgument string,
	environment map[string]string,
	execute executeCommand,
) (goListPackage, command.Result, error) {
	args := []string{"go", "list", "-json", packageArgument}
	result := execute(ctx, command.Spec{
		ID:          "go-list-evidence-package",
		Description: "resolve one repository-local Go package for exact test evidence",
		Directory:   workingDirectory,
		Args:        args,
		Environment: cloneEnvironment(environment),
		Timeout:     defaultTimeout,
	})
	if result.ExitCode != 0 {
		return goListPackage{}, result, fmt.Errorf("go list package resolution failed with exit code %d", result.ExitCode)
	}
	if result.Error != "" {
		return goListPackage{}, result, fmt.Errorf("go list package resolution reported an execution error: %s", result.Error)
	}

	decoder := json.NewDecoder(strings.NewReader(result.Stdout))
	var packages []goListPackage
	for {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return goListPackage{}, result, fmt.Errorf("go list stdout is not a complete JSON stream: %w", err)
		}
		packages = append(packages, pkg)
	}
	if len(packages) != 1 {
		return goListPackage{}, result, fmt.Errorf("package argument %q resolved %d packages, want exactly 1", packageArgument, len(packages))
	}
	selection := packages[0]
	if strings.TrimSpace(selection.ImportPath) == "" || selection.ImportPath == "command-line-arguments" {
		return goListPackage{}, result, fmt.Errorf("package argument %q did not resolve a stable package import path", packageArgument)
	}
	if strings.TrimSpace(selection.Dir) == "" {
		return goListPackage{}, result, fmt.Errorf("package argument %q resolved without a package directory", packageArgument)
	}
	if !filepath.IsAbs(selection.Dir) {
		return goListPackage{}, result, fmt.Errorf("package argument %q resolved a non-absolute package directory", packageArgument)
	}
	resolvedPackageDirectory, err := filepath.EvalSymlinks(selection.Dir)
	if err != nil {
		return goListPackage{}, result, fmt.Errorf("resolve selected package directory: %w", err)
	}
	resolvedPackageDirectory, err = filepath.Abs(resolvedPackageDirectory)
	if err != nil {
		return goListPackage{}, result, fmt.Errorf("resolve absolute selected package directory: %w", err)
	}
	relative, err := filepath.Rel(workingDirectory, resolvedPackageDirectory)
	if err != nil {
		return goListPackage{}, result, fmt.Errorf("compare selected package directory to working directory: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return goListPackage{}, result, fmt.Errorf("package argument %q resolved outside the selected repository working directory", packageArgument)
	}
	selection.Dir = resolvedPackageDirectory
	return selection, result, nil
}

func resolveDirectory(root, directory string) (string, string, error) {
	if strings.TrimSpace(root) == "" {
		return "", "", errors.New("repository root is required")
	}
	if strings.TrimSpace(directory) == "" {
		return "", "", errors.New("directory is required")
	}
	if filepath.IsAbs(directory) {
		return "", "", errors.New("directory must be repository-relative")
	}
	if strings.Contains(directory, `\`) {
		return "", "", errors.New("directory must use repository-relative slash-separated form")
	}
	cleanDirectory := filepath.ToSlash(filepath.Clean(filepath.FromSlash(directory)))
	if directory != cleanDirectory {
		return "", "", fmt.Errorf("directory %q is not a clean relative path; use %q", directory, cleanDirectory)
	}
	if cleanDirectory == ".." || strings.HasPrefix(cleanDirectory, "../") {
		return "", "", fmt.Errorf("test directory %q escapes repository root", directory)
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository root: %w", err)
	}
	candidate := filepath.Join(resolvedRoot, filepath.FromSlash(cleanDirectory))
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", "", fmt.Errorf("resolve test directory: %w", err)
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", "", fmt.Errorf("resolve test directory: %w", err)
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	if err != nil {
		return "", "", fmt.Errorf("compare test directory to repository root: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("test directory %q escapes repository root", directory)
	}
	info, err := os.Stat(resolvedCandidate)
	if err != nil {
		return "", "", fmt.Errorf("inspect test directory: %w", err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("test directory %q is not a directory", directory)
	}
	return filepath.ToSlash(relative), resolvedCandidate, nil
}

func hasParentSegment(value string) bool {
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func isTopLevelTestName(name string) bool {
	if !token.IsIdentifier(name) || !strings.HasPrefix(name, "Test") {
		return false
	}
	if len(name) == len("Test") {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[len("Test"):])
	return !unicode.IsLower(r)
}

func fullyAnchored(expression string) bool {
	parsed, err := syntax.Parse(expression, syntax.Perl)
	if err != nil {
		return false
	}
	return beginsAtText(parsed) && endsAtText(parsed)
}

func beginsAtText(expression *syntax.Regexp) bool {
	switch expression.Op {
	case syntax.OpBeginText:
		return true
	case syntax.OpCapture:
		return len(expression.Sub) == 1 && beginsAtText(expression.Sub[0])
	case syntax.OpConcat:
		return len(expression.Sub) > 0 && beginsAtText(expression.Sub[0])
	case syntax.OpAlternate:
		if len(expression.Sub) == 0 {
			return false
		}
		for _, alternative := range expression.Sub {
			if !beginsAtText(alternative) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func endsAtText(expression *syntax.Regexp) bool {
	switch expression.Op {
	case syntax.OpEndText:
		return true
	case syntax.OpCapture:
		return len(expression.Sub) == 1 && endsAtText(expression.Sub[0])
	case syntax.OpConcat:
		return len(expression.Sub) > 0 && endsAtText(expression.Sub[len(expression.Sub)-1])
	case syntax.OpAlternate:
		if len(expression.Sub) == 0 {
			return false
		}
		for _, alternative := range expression.Sub {
			if !endsAtText(alternative) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

type goTestEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
}

func parseEvents(reader io.Reader, requiredPackage string, required []string, expected int) ([]TestEvidence, []PackageEvidence, error) {
	tests := make(map[string]*TestEvidence, len(required))
	for _, name := range required {
		key := testEvidenceKey(requiredPackage, name)
		tests[key] = &TestEvidence{Package: requiredPackage, Name: name, Required: true, Expected: expected}
	}
	packages := make(map[string]*PackageEvidence)
	decoder := json.NewDecoder(reader)
	for {
		var event goTestEvent
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return flattenTests(tests), flattenPackages(packages), err
		}
		if event.Package != "" {
			if packages[event.Package] == nil {
				packages[event.Package] = &PackageEvidence{Name: event.Package}
			}
			pkg := packages[event.Package]
			if event.Test == "" {
				switch event.Action {
				case "start":
					pkg.Start++
				case "pass":
					pkg.Pass++
				case "skip":
					pkg.Skip++
				case "fail":
					pkg.Fail++
				case "output":
					if isCachedPackageOutput(event.Output) {
						pkg.Cached = true
					}
				}
			}
		}
		if event.Test == "" {
			continue
		}
		key := testEvidenceKey(event.Package, event.Test)
		if tests[key] == nil {
			tests[key] = &TestEvidence{Package: event.Package, Name: event.Test}
		}
		test := tests[key]
		switch event.Action {
		case "run":
			test.Run++
		case "pass":
			test.Pass++
		case "skip":
			test.Skip++
		case "fail":
			test.Fail++
		}
	}
	return flattenTests(tests), flattenPackages(packages), nil
}

func testEvidenceKey(packageName, testName string) string {
	return packageName + "\x00" + testName
}

func isCachedPackageOutput(output string) bool {
	fields := strings.Fields(output)
	return len(fields) >= 3 && fields[0] == "ok" && fields[len(fields)-1] == "(cached)"
}

func flattenTests(values map[string]*TestEvidence) []TestEvidence {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]TestEvidence, 0, len(keys))
	for _, key := range keys {
		result = append(result, *values[key])
	}
	return result
}

func flattenPackages(values map[string]*PackageEvidence) []PackageEvidence {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]PackageEvidence, 0, len(names))
	for _, name := range names {
		result = append(result, *values[name])
	}
	return result
}

func normalizeReport(report *Report) {
	sort.Strings(report.Required)
	sort.Strings(report.Failures)
	if report.Tests == nil {
		report.Tests = []TestEvidence{}
	}
	if report.Packages == nil {
		report.Packages = []PackageEvidence{}
	}
	if report.Environment == nil {
		report.Environment = map[string]string{}
	}
}

func cloneEnvironment(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func redactReport(report *Report, root string) {
	report.Directory = redactRepositoryPaths(report.Directory, root)
	report.Package = redactRepositoryPaths(report.Package, root)
	report.RunPattern = redactRepositoryPaths(report.RunPattern, root)
	report.GoWork = redactRepositoryPaths(report.GoWork, root)
	report.CommandError = redactRepositoryPaths(report.CommandError, root)
	report.Stderr = redactRepositoryPaths(report.Stderr, root)
	report.ParseError = redactRepositoryPaths(report.ParseError, root)
	for index := range report.Required {
		report.Required[index] = redactRepositoryPaths(report.Required[index], root)
	}
	for index := range report.Command {
		report.Command[index] = redactRepositoryPaths(report.Command[index], root)
	}
	redactedEnvironment := make(map[string]string, len(report.Environment))
	for key, value := range report.Environment {
		redactedKey := redactRepositoryPaths(key, root)
		redactedEnvironment[redactedKey] = redactRepositoryPaths(value, root)
	}
	report.Environment = redactedEnvironment
	for index := range report.Tests {
		report.Tests[index].Package = redactRepositoryPaths(report.Tests[index].Package, root)
		report.Tests[index].Name = redactRepositoryPaths(report.Tests[index].Name, root)
	}
	for index := range report.Packages {
		report.Packages[index].Name = redactRepositoryPaths(report.Packages[index].Name, root)
	}
	for index := range report.Failures {
		report.Failures[index] = redactRepositoryPaths(report.Failures[index], root)
	}
}

func redactRepositoryPaths(value, root string) string {
	if value == "" || strings.TrimSpace(root) == "" {
		return value
	}
	candidates := make(map[string]bool)
	if filepath.IsAbs(root) {
		candidates[filepath.Clean(root)] = true
	}
	if absolute, err := filepath.Abs(root); err == nil {
		candidates[filepath.Clean(absolute)] = true
		if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
			candidates[filepath.Clean(resolved)] = true
		}
	}
	paths := make([]string, 0, len(candidates)*2)
	for candidate := range candidates {
		if candidate == "" || candidate == string(filepath.Separator) {
			continue
		}
		paths = append(paths, candidate)
		if slash := filepath.ToSlash(candidate); slash != candidate {
			paths = append(paths, slash)
		}
	}
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i]) > len(paths[j])
	})
	for _, candidate := range paths {
		value = strings.ReplaceAll(value, candidate, "$REPO")
	}
	return value
}

func truncate(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	return value[:maximum] + "\n... truncated ...\n"
}
