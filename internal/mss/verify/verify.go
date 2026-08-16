package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/command"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

// Mode selects verification scope.
type Mode string

const (
	ModeChanged Mode = "changed"
	ModeAll     Mode = "all"
	ModeModule  Mode = "module"
)

// Options controls planning, execution, and reporting.
type Options struct {
	Mode     Mode
	BaseRef  string
	Module   string
	PlanOnly bool
}

// Plan describes the exact checks selected for a change.
type Plan struct {
	Mode         Mode                `json:"mode"`
	BaseRef      string              `json:"baseRef,omitempty"`
	Module       string              `json:"module,omitempty"`
	ChangedFiles []string            `json:"changedFiles,omitempty"`
	Checks       []command.Spec      `json:"checks"`
	Reasons      map[string][]string `json:"reasons"`
}

// Report is the durable verification result consumed by agents and CI.
type Report struct {
	Project     string           `json:"project"`
	Root        string           `json:"root"`
	GeneratedAt time.Time        `json:"generatedAt"`
	Plan        Plan             `json:"plan"`
	PlanOnly    bool             `json:"planOnly"`
	Success     bool             `json:"success"`
	Results     []command.Result `json:"results,omitempty"`
}

// PlanChecks computes a deterministic minimum-sufficient validation plan.
func PlanChecks(ctx *project.Context, options Options) (Plan, error) {
	if options.Mode == "" {
		options.Mode = ModeChanged
	}
	plan := Plan{
		Mode:    options.Mode,
		BaseRef: options.BaseRef,
		Module:  options.Module,
		Reasons: make(map[string][]string),
	}

	switch options.Mode {
	case ModeAll:
		plan.ChangedFiles = nil
	case ModeModule:
		if !moduleNameValid(options.Module) {
			return Plan{}, fmt.Errorf("invalid module name %q", options.Module)
		}
		plan.ChangedFiles = []string{
			"modules/" + options.Module,
			"web/antd/src/modules/" + options.Module,
			"docs/docs/modules/" + options.Module + ".md",
		}
	case ModeChanged:
		files, base, err := changedFiles(ctx.Root, options.BaseRef)
		if err != nil {
			return Plan{}, err
		}
		plan.BaseRef = base
		plan.ChangedFiles = files
	default:
		return Plan{}, fmt.Errorf("unsupported verification mode %q", options.Mode)
	}

	checks := make(map[string]command.Spec)
	add := func(check command.Spec, reason string) {
		if _, exists := checks[check.ID]; !exists {
			checks[check.ID] = check
		}
		plan.Reasons[check.ID] = appendUnique(plan.Reasons[check.ID], reason)
	}

	add(diffCheck(ctx.Root), "all changes must pass Git whitespace and conflict-marker checks")

	if options.Mode == ModeAll {
		add(toolingTest(ctx.Root), "full verification includes agent infrastructure tests")
		add(frameworkTest(ctx.Root), "full verification includes reusable framework tests")
		add(backendTest(ctx.Root), "full verification includes backend tests")
		add(backendBuild(ctx.Root), "full verification includes backend build")
		add(frontendLint(ctx.Root), "full verification includes frontend lint and type checks")
		add(frontendTest(ctx.Root), "full verification includes frontend unit tests")
		add(frontendBuild(ctx.Root), "full verification includes frontend production build")
		if hasFrontendApplication(ctx, "web/antd-v6") {
			add(frontendV6Lint(ctx.Root), "full verification includes independent Ant Design 6 frontend lint and type checks")
			add(frontendV6Test(ctx.Root), "full verification includes independent Ant Design 6 frontend unit tests")
			add(frontendV6Build(ctx.Root), "full verification includes independent Ant Design 6 frontend production build")
		}
		add(docsBuild(ctx.Root), "full verification includes documentation build")
	} else {
		for _, changed := range plan.ChangedFiles {
			path := filepath.ToSlash(changed)
			switch {
			case isAgentInfrastructure(path):
				add(toolingTest(ctx.Root), path+" affects agent infrastructure contracts or tooling")
			case strings.HasPrefix(path, "mss-boot/"):
				add(frameworkTest(ctx.Root), path+" affects the reusable framework")
			case isFrontend(path):
				add(frontendLint(ctx.Root), path+" affects the frontend")
				add(frontendTest(ctx.Root), path+" affects frontend behavior")
				if frontendBuildSensitive(path) {
					add(frontendBuild(ctx.Root), path+" affects frontend build or routing configuration")
				}
			case isFrontendV6(path):
				add(frontendV6Lint(ctx.Root), path+" affects the independent Ant Design 6 frontend")
				add(frontendV6Test(ctx.Root), path+" affects Ant Design 6 frontend behavior")
				if frontendV6BuildSensitive(path) {
					add(frontendV6Build(ctx.Root), path+" affects Ant Design 6 frontend build or routing configuration")
				}
			case isDocs(path):
				add(docsBuild(ctx.Root), path+" affects the documentation site")
			case isBackend(path):
				add(backendTest(ctx.Root), path+" affects backend code")
				if backendBuildSensitive(path) {
					add(backendBuild(ctx.Root), path+" affects backend startup, modules, or dependencies")
				}
			}
		}
		if options.Mode == ModeModule {
			moduleDir := filepath.Join(ctx.Root, "modules", options.Module)
			add(command.Spec{
				ID:          "module-test:" + options.Module,
				Description: "run focused tests for module " + options.Module,
				Directory:   ctx.Root,
				Args:        []string{"go", "test", "./modules/" + options.Module + "/..."},
				Timeout:     10 * time.Minute,
			}, filepath.ToSlash(moduleDir)+" is the requested module scope")
		}
	}

	ids := make([]string, 0, len(checks))
	for id := range checks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	plan.Checks = make([]command.Spec, 0, len(ids))
	for _, id := range ids {
		plan.Checks = append(plan.Checks, checks[id])
		sort.Strings(plan.Reasons[id])
	}
	sort.Strings(plan.ChangedFiles)
	return plan, nil
}

// Run validates structured contracts, executes the plan, and writes reports.
func Run(parent context.Context, ctx *project.Context, options Options) (Report, error) {
	plan, err := PlanChecks(ctx, options)
	if err != nil {
		return Report{}, err
	}
	if err := os.MkdirAll(filepath.Join(ctx.Root, ".mss", "reports"), 0o755); err != nil {
		return Report{}, fmt.Errorf("create verification report directory: %w", err)
	}
	report := Report{
		Project:     ctx.Project.Metadata.Name,
		Root:        ctx.Root,
		GeneratedAt: time.Now().UTC(),
		Plan:        plan,
		PlanOnly:    options.PlanOnly,
		Success:     true,
	}

	contractResult := validateContracts(ctx.Root)
	report.Results = append(report.Results, contractResult)
	if contractResult.ExitCode != 0 {
		report.Success = false
	}

	if !options.PlanOnly && report.Success {
		for _, check := range plan.Checks {
			result := command.Run(parent, check)
			result.Stdout = truncate(result.Stdout, 128*1024)
			result.Stderr = truncate(result.Stderr, 128*1024)
			report.Results = append(report.Results, result)
			if result.ExitCode != 0 {
				report.Success = false
				break
			}
		}
	}

	if writeErr := WriteReports(ctx, report); writeErr != nil {
		return report, writeErr
	}
	if !report.Success {
		return report, errors.New("verification failed; see .mss/reports/verify.md")
	}
	return report, nil
}

// WriteReports persists stable JSON and Markdown summaries.
func WriteReports(ctx *project.Context, report Report) error {
	reportDir := filepath.Join(ctx.Root, ".mss", "reports")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return fmt.Errorf("create verification report directory: %w", err)
	}
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal verification report: %w", err)
	}
	jsonData = append(jsonData, '\n')
	if err := writeAtomic(filepath.Join(reportDir, "verify.json"), jsonData); err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(reportDir, "verify.md"), []byte(report.Markdown())); err != nil {
		return err
	}
	return nil
}

// JSON returns stable indented JSON.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// Markdown renders an agent-friendly verification handoff.
func (r Report) Markdown() string {
	var builder strings.Builder
	builder.WriteString("# mss verification report\n\n")
	fmt.Fprintf(&builder, "- Project: `%s`\n", r.Project)
	fmt.Fprintf(&builder, "- Mode: `%s`\n", r.Plan.Mode)
	if r.Plan.BaseRef != "" {
		fmt.Fprintf(&builder, "- Base ref: `%s`\n", r.Plan.BaseRef)
	}
	if r.Plan.Module != "" {
		fmt.Fprintf(&builder, "- Module: `%s`\n", r.Plan.Module)
	}
	fmt.Fprintf(&builder, "- Plan only: `%t`\n", r.PlanOnly)
	fmt.Fprintf(&builder, "- Success: `%t`\n", r.Success)
	fmt.Fprintf(&builder, "- Generated at: `%s`\n\n", r.GeneratedAt.Format(time.RFC3339))

	builder.WriteString("## Changed files\n\n")
	if len(r.Plan.ChangedFiles) == 0 {
		builder.WriteString("No explicit changed-file set; full validation scope was selected.\n\n")
	} else {
		for _, path := range r.Plan.ChangedFiles {
			fmt.Fprintf(&builder, "- `%s`\n", path)
		}
		builder.WriteByte('\n')
	}

	builder.WriteString("## Planned checks\n\n")
	for _, check := range r.Plan.Checks {
		fmt.Fprintf(&builder, "- `%s`: `%s`\n", check.ID, command.Display(check.Args))
		for _, reason := range r.Plan.Reasons[check.ID] {
			fmt.Fprintf(&builder, "  - %s\n", reason)
		}
	}
	builder.WriteByte('\n')

	builder.WriteString("## Results\n\n")
	for _, result := range r.Results {
		status := "PASS"
		if result.ExitCode != 0 {
			status = "FAIL"
		}
		fmt.Fprintf(&builder, "### %s — %s\n\n", status, result.ID)
		if len(result.Args) > 0 {
			fmt.Fprintf(&builder, "Command: `%s`\n\n", command.Display(result.Args))
		}
		fmt.Fprintf(&builder, "Exit code: `%d`; duration: `%s`\n\n", result.ExitCode, result.Duration.Round(time.Millisecond))
		if result.Error != "" {
			fmt.Fprintf(&builder, "Error: `%s`\n\n", strings.ReplaceAll(result.Error, "`", "'"))
		}
		if strings.TrimSpace(result.Stdout) != "" {
			builder.WriteString("<details><summary>stdout</summary>\n\n```text\n")
			builder.WriteString(result.Stdout)
			builder.WriteString("\n```\n</details>\n\n")
		}
		if strings.TrimSpace(result.Stderr) != "" {
			builder.WriteString("<details><summary>stderr</summary>\n\n```text\n")
			builder.WriteString(result.Stderr)
			builder.WriteString("\n```\n</details>\n\n")
		}
	}
	return builder.String()
}

func validateContracts(root string) command.Result {
	started := time.Now()
	result := command.Result{
		ID:          "contract-validation",
		Description: "validate module and feature contracts",
		Directory:   root,
		StartedAt:   started.UTC(),
		ExitCode:    0,
	}
	patterns := []string{
		filepath.Join(root, ".mss", "modules", "*.yaml"),
		filepath.Join(root, "modules", "*", "module.yaml"),
	}
	var paths []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			result.ExitCode = 1
			result.Error = err.Error()
			result.Duration = time.Since(started)
			return result
		}
		paths = append(paths, matches...)
	}
	sort.Strings(paths)
	var output strings.Builder
	for _, path := range paths {
		module, err := spec.LoadModule(path)
		if err != nil {
			result.ExitCode = 1
			result.Error = err.Error()
			break
		}
		fmt.Fprintf(&output, "validated %s (%s)\n", filepath.ToSlash(path), module.Metadata.Name)
	}
	if result.ExitCode == 0 {
		featurePaths, err := filepath.Glob(filepath.Join(root, ".mss", "features", "*.yaml"))
		if err != nil {
			result.ExitCode = 1
			result.Error = err.Error()
		} else {
			sort.Strings(featurePaths)
			for _, path := range featurePaths {
				feature, loadErr := spec.LoadFeature(path)
				if loadErr != nil {
					result.ExitCode = 1
					result.Error = loadErr.Error()
					break
				}
				fmt.Fprintf(&output, "validated %s (%s)\n", filepath.ToSlash(path), feature.Metadata.Name)
			}
		}
	}
	result.Stdout = output.String()
	result.Duration = time.Since(started)
	return result
}

func changedFiles(root, requestedBase string) ([]string, string, error) {
	files := make(map[string]struct{})
	statusOutput, err := runGit(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, "", err
	}
	for _, entry := range bytes.Split(statusOutput, []byte{0}) {
		if len(entry) < 4 {
			continue
		}
		path := strings.TrimSpace(string(entry[3:]))
		if arrow := strings.LastIndex(path, " -> "); arrow >= 0 {
			path = path[arrow+4:]
		}
		if path != "" {
			files[filepath.ToSlash(path)] = struct{}{}
		}
	}

	base := resolveBaseRef(root, requestedBase)
	if base != "" {
		diffOutput, diffErr := runGit(root, "diff", "--name-only", "-z", base+"...HEAD")
		if diffErr != nil {
			return nil, "", diffErr
		}
		for _, entry := range bytes.Split(diffOutput, []byte{0}) {
			path := strings.TrimSpace(string(entry))
			if path != "" {
				files[filepath.ToSlash(path)] = struct{}{}
			}
		}
	}

	result := make([]string, 0, len(files))
	for path := range files {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, base, nil
}

func resolveBaseRef(root, requested string) string {
	candidates := []string{requested, os.Getenv("MSS_VERIFY_BASE")}
	if githubBase := os.Getenv("GITHUB_BASE_REF"); githubBase != "" {
		candidates = append(candidates, "origin/"+githubBase, githubBase)
	}
	candidates = append(candidates, "origin/main", "main", "HEAD~1")
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, err := runGit(root, "rev-parse", "--verify", candidate+"^{commit}"); err == nil {
			return candidate
		}
	}
	return ""
}

func runGit(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitError.Stderr)))
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func diffCheck(root string) command.Spec {
	return command.Spec{
		ID:          "git-diff-check",
		Description: "check whitespace errors and conflict markers",
		Directory:   root,
		Args:        []string{"git", "diff", "--check", "HEAD"},
		Timeout:     2 * time.Minute,
	}
}

func toolingTest(root string) command.Spec {
	args := []string{
		"go",
		"test",
		"./internal/mss/...",
		"./cmd/mss/...",
		"./admin/modules/runtime/...",
	}
	// The vertical module runtime is optional on branches that have not added
	// that layer yet. Omit only a definitely absent path; malformed or
	// unreadable paths stay in the command so verification fails visibly.
	optionalRuntime := filepath.Join(root, "modules", "runtime")
	if _, err := os.Stat(optionalRuntime); err == nil || !os.IsNotExist(err) {
		args = append(args, "./modules/runtime/...")
	}
	return command.Spec{
		ID:          "agent-tooling-test",
		Description: "test mss CLI, contracts, generator, verifier, Admin module registry, and available vertical module runtime",
		Directory:   root,
		Args:        args,
		Timeout:     10 * time.Minute,
	}
}

func frameworkTest(root string) command.Spec {
	return command.Spec{
		ID:          "framework-test",
		Description: "test the reusable mss-boot module independently",
		Directory:   filepath.Join(root, "mss-boot"),
		Args:        []string{"go", "test", "./..."},
		Environment: map[string]string{"GOWORK": "off"},
		Timeout:     20 * time.Minute,
	}
}

func backendTest(root string) command.Spec {
	return command.Spec{
		ID:          "backend-test",
		Description: "run the Admin application tests independently",
		Directory:   filepath.Join(root, "admin"),
		Args: []string{
			"go", "test",
			"-coverprofile=" + filepath.Join(root, ".mss", "reports", "admin-coverage.out"),
			"./...",
		},
		Environment: map[string]string{"GOWORK": "off"},
		Timeout:     20 * time.Minute,
	}
}

func backendBuild(root string) command.Spec {
	return command.Spec{
		ID:          "backend-build",
		Description: "build the admin backend",
		Directory:   filepath.Join(root, "admin"),
		Args:        []string{"go", "build", "./..."},
		Environment: map[string]string{"CGO_ENABLED": "0", "GOWORK": "off"},
		Timeout:     10 * time.Minute,
	}
}

func frontendLint(root string) command.Spec {
	return command.Spec{
		ID:          "frontend-lint",
		Description: "run frontend lint and TypeScript checks",
		Directory:   filepath.Join(root, "web", "antd"),
		Args:        []string{"corepack", "pnpm", "lint"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     15 * time.Minute,
	}
}

func frontendTest(root string) command.Spec {
	return command.Spec{
		ID:          "frontend-test",
		Description: "run frontend unit tests",
		Directory:   filepath.Join(root, "web", "antd"),
		Args:        []string{"corepack", "pnpm", "test", "--", "--runInBand"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     20 * time.Minute,
	}
}

func frontendBuild(root string) command.Spec {
	return command.Spec{
		ID:          "frontend-build",
		Description: "build the portable release frontend profile",
		Directory:   filepath.Join(root, "web", "antd"),
		Args:        []string{"corepack", "pnpm", "build:release"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     20 * time.Minute,
	}
}

func frontendV6Lint(root string) command.Spec {
	return command.Spec{
		ID:          "frontend-v6-lint",
		Description: "run independent Ant Design 6 frontend lint and TypeScript checks",
		Directory:   filepath.Join(root, "web", "antd-v6"),
		Args:        []string{"corepack", "pnpm@10.34.5", "lint"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     15 * time.Minute,
	}
}

func frontendV6Test(root string) command.Spec {
	return command.Spec{
		ID:          "frontend-v6-test",
		Description: "run independent Ant Design 6 frontend unit tests",
		Directory:   filepath.Join(root, "web", "antd-v6"),
		Args:        []string{"corepack", "pnpm@10.34.5", "test:ci"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     20 * time.Minute,
	}
}

func frontendV6Build(root string) command.Spec {
	return command.Spec{
		ID:          "frontend-v6-build",
		Description: "build the independent Ant Design 6 release frontend",
		Directory:   filepath.Join(root, "web", "antd-v6"),
		Args:        []string{"corepack", "pnpm@10.34.5", "build:release"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     20 * time.Minute,
	}
}

func docsBuild(root string) command.Spec {
	return command.Spec{
		ID:          "docs-build",
		Description: "build the documentation site",
		Directory:   filepath.Join(root, "docs"),
		Args:        []string{"corepack", "pnpm", "build"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     20 * time.Minute,
	}
}

func isAgentInfrastructure(path string) bool {
	return strings.HasPrefix(path, ".mss/") ||
		strings.HasPrefix(path, ".agents/") ||
		strings.HasPrefix(path, "cmd/mss/") ||
		strings.HasPrefix(path, "internal/mss/") ||
		strings.HasPrefix(path, "templates/module/") ||
		strings.HasPrefix(path, "modules/runtime/") ||
		strings.HasPrefix(path, "modules/all/") ||
		path == "AGENTS.md"
}

func isFrontend(path string) bool {
	return strings.HasPrefix(path, "web/antd/")
}

func isFrontendV6(path string) bool {
	return strings.HasPrefix(path, "web/antd-v6/")
}

func hasFrontendApplication(ctx *project.Context, applicationPath string) bool {
	for _, application := range ctx.Project.Spec.Frontend.Applications {
		if filepath.ToSlash(filepath.Clean(application.Path)) == applicationPath {
			return true
		}
	}
	return false
}

func isDocs(path string) bool {
	return strings.HasPrefix(path, "docs/")
}

func isBackend(path string) bool {
	if path == "go.mod" || path == "go.sum" || path == "go.work" || path == "main.go" {
		return true
	}
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	return !strings.HasPrefix(path, "mss-boot/") && !strings.HasPrefix(path, "web/") && !strings.HasPrefix(path, "docs/")
}

func frontendBuildSensitive(path string) bool {
	return strings.HasPrefix(path, "web/antd/config/") ||
		strings.HasPrefix(path, "web/antd/src/modules/") ||
		strings.HasPrefix(path, "web/antd/src/pages/") ||
		strings.HasSuffix(path, "package.json") ||
		strings.HasSuffix(path, "pnpm-lock.yaml") ||
		strings.HasSuffix(path, "tsconfig.json")
}

func frontendV6BuildSensitive(path string) bool {
	return strings.HasPrefix(path, "web/antd-v6/config/") ||
		strings.HasPrefix(path, "web/antd-v6/src/modules/") ||
		strings.HasPrefix(path, "web/antd-v6/src/pages/") ||
		strings.HasSuffix(path, "package.json") ||
		strings.HasSuffix(path, "pnpm-lock.yaml") ||
		strings.HasSuffix(path, "tsconfig.json")
}

func backendBuildSensitive(path string) bool {
	return path == "main.go" || path == "go.mod" || path == "go.sum" || path == "go.work" ||
		strings.HasPrefix(path, "cmd/") || strings.HasPrefix(path, "config/") || strings.HasPrefix(path, "modules/")
}

func moduleNameValid(value string) bool {
	if value == "" || strings.HasPrefix(value, "-") || strings.HasSuffix(value, "-") {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			continue
		}
		return false
	}
	return true
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "\n... output truncated by mss verify ...\n"
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".mss-report-*")
	if err != nil {
		return fmt.Errorf("create temporary report: %w", err)
	}
	name := temporary.Name()
	cleanup := func() {
		_ = temporary.Close()
		_ = os.Remove(name)
	}
	if _, err := temporary.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temporary report: %w", err)
	}
	if err := temporary.Chmod(0o644); err != nil {
		cleanup()
		return fmt.Errorf("chmod temporary report: %w", err)
	}
	if err := temporary.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temporary report: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		cleanup()
		return fmt.Errorf("replace report %s: %w", path, err)
	}
	return nil
}
