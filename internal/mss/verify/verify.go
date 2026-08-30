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
	thinHost := ctx.LayoutKind() == "thin-host"

	switch options.Mode {
	case ModeAll:
		plan.ChangedFiles = nil
	case ModeModule:
		if !moduleNameValid(options.Module) {
			return Plan{}, fmt.Errorf("invalid module name %q", options.Module)
		}
		plan.ChangedFiles = moduleScopePaths(ctx, options.Module)
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

	for _, check := range diffChecks(ctx.Root) {
		add(check, "all changes must pass Git whitespace and conflict-marker checks")
	}

	if options.Mode == ModeAll {
		if thinHost {
			add(thinHostBackendTest(ctx), "full Thin Host verification includes downstream backend tests with GOWORK disabled")
			add(thinHostBackendBuild(ctx), "full Thin Host verification builds the composed Admin and business modules")
			add(thinHostFrontendLint(ctx), "full Thin Host verification includes host frontend lint and type checks")
			add(thinHostFrontendTest(ctx), "full Thin Host verification includes host frontend tests")
			add(thinHostFrontendBuild(ctx), "full Thin Host verification builds the single composed Umi application")
		} else {
			add(agentReleaseTest(ctx.Root), "full local verification runs the complete Agent module test target with GOWORK disabled")
			add(agentReleaseBuild(ctx.Root), "full local verification builds both Agent executables independently")
			add(strictAgentDoctor(ctx.Root), "full local verification validates the Agent environment contract before release preparation")
			add(strictBackendDoctor(ctx.Root), "full local verification validates the Admin environment contract before release preparation")
			add(skillContractValidation(ctx.Root), "full local verification validates every checked-in Agent skill")
			add(foundationCompatibility(ctx.Root), "full local verification qualifies standalone package-first generation and upgrade behavior")
			add(frameworkReleaseQualification(ctx.Root), "full local verification includes Framework race, coverage, vet, tidy, and independent-module checks")
			add(backendReleaseQualification(ctx.Root), "full local verification includes Admin race, coverage, vet, module metadata, external consumer, and build checks")
			add(presentationThinHostContract(ctx.Root), "full verification qualifies the fixed core-plus-business presentation contract through external Go and npm consumers")
			add(releaseContractTest(ctx.Root), "full local verification validates release policy and workflow contracts before candidate packaging")
			if hasFrontendApplication(ctx, "web/antd-v6") {
				add(frontendQualification(ctx.Root), "full local verification includes dependency policy, lint, unit, release build, delivery, and browser qualification")
			}
			add(docsBuild(ctx.Root), "full verification includes documentation build")
		}
	} else {
		for _, changed := range plan.ChangedFiles {
			path := filepath.ToSlash(changed)
			if thinHost {
				switch {
				case isThinHostFrontend(ctx, path):
					add(thinHostFrontendLint(ctx), path+" affects the Thin Host frontend")
					add(thinHostFrontendTest(ctx), path+" affects Thin Host frontend behavior")
					if thinHostFrontendBuildSensitive(ctx, path) {
						add(thinHostFrontendBuild(ctx), path+" affects Thin Host routing, generated code, or dependencies")
					}
				case isThinHostBackend(ctx, path):
					add(thinHostBackendTest(ctx), path+" affects the Thin Host backend")
					modulePrefix := normalizedLayoutPrefix(ctx.Project.Spec.RepositoryLayout["modules"])
					if backendBuildSensitive(path) || (modulePrefix != "" && strings.HasPrefix(path, modulePrefix)) {
						add(thinHostBackendBuild(ctx), path+" affects Thin Host startup, modules, or dependencies")
					}
				}
				continue
			}
			if releaseWorkflowContractSensitive(path) {
				add(releaseWorkflowContractTest(ctx.Root), path+" affects release policy or workflow contracts")
			}
			if presentationThinHostContractSensitive(path) {
				add(
					presentationThinHostContract(ctx.Root),
					path+" affects the packaged core-plus-business presentation Thin Host contract",
				)
			}
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
			focused, moduleDir, focusedErr := focusedModuleTest(ctx, options.Module)
			if focusedErr != nil {
				return Plan{}, focusedErr
			}
			add(focused, filepath.ToSlash(moduleDir)+" is the requested module scope")
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

	contractResult := validateContracts(ctx.Root, ctx.Project.Spec.RepositoryLayout["modules"])
	report.Results = append(report.Results, contractResult)
	if contractResult.ExitCode != 0 {
		report.Success = false
	}
	if ctx.LayoutKind() == "thin-host" {
		thinHostResult := validateThinHostStructure(ctx)
		report.Results = append(report.Results, thinHostResult)
		if thinHostResult.ExitCode != 0 {
			report.Success = false
		}
		generatedResult := validateThinHostGeneratedModules(ctx)
		report.Results = append(report.Results, generatedResult)
		if generatedResult.ExitCode != 0 {
			report.Success = false
		}
	}
	untrackedResult := validateUntrackedWorkspaceText(ctx.Root)
	report.Results = append(report.Results, untrackedResult)
	if untrackedResult.ExitCode != 0 {
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

func validateContracts(root string, moduleDirectories ...string) command.Result {
	started := time.Now()
	result := command.Result{
		ID:          "contract-validation",
		Description: "validate module and feature contracts",
		Directory:   root,
		StartedAt:   started.UTC(),
		ExitCode:    0,
	}
	modulesDirectory := "modules"
	if len(moduleDirectories) > 0 && strings.TrimSpace(moduleDirectories[0]) != "" {
		modulesDirectory = strings.TrimSpace(moduleDirectories[0])
	}
	if !confinedRepositoryPath(modulesDirectory) {
		result.ExitCode = 1
		result.Error = "project modules directory must be repository-relative and confined"
		result.Duration = time.Since(started)
		return result
	}
	patterns := []string{
		filepath.Join(root, ".mss", "modules", "*.yaml"),
		filepath.Join(root, filepath.FromSlash(modulesDirectory), "*", "module.yaml"),
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
	return diffChecks(root)[0]
}

func diffChecks(root string) []command.Spec {
	if _, err := runGit(root, "rev-parse", "--verify", "HEAD^{commit}"); err != nil {
		return []command.Spec{
			{
				ID:          "git-diff-check",
				Description: "check staged whitespace errors and conflict markers before the first commit",
				Directory:   root,
				Args:        []string{"git", "diff", "--cached", "--check", "--"},
				Timeout:     2 * time.Minute,
			},
			{
				ID:          "git-worktree-check",
				Description: "check unstaged whitespace errors and conflict markers before the first commit",
				Directory:   root,
				Args:        []string{"git", "diff", "--check", "--"},
				Timeout:     2 * time.Minute,
			},
		}
	}
	return []command.Spec{{
		ID:          "git-diff-check",
		Description: "check whitespace errors and conflict markers",
		Directory:   root,
		Args:        []string{"git", "diff", "--check", "HEAD", "--"},
		Timeout:     2 * time.Minute,
	}}
}

func validateUntrackedWorkspaceText(root string) command.Result {
	started := time.Now()
	result := command.Result{
		ID:          "untracked-text-check",
		Description: "check untracked text files for whitespace errors and conflict markers",
		Directory:   root,
		StartedAt:   started.UTC(),
		ExitCode:    0,
	}
	output, err := runGit(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		result.ExitCode = 1
		result.Error = err.Error()
		result.Duration = time.Since(started)
		return result
	}
	var problems []string
	for _, raw := range bytes.Split(output, []byte{0}) {
		relative := filepath.ToSlash(string(raw))
		if relative == "" {
			continue
		}
		if !confinedRepositoryPath(relative) {
			problems = append(problems, relative+": untracked path is not repository-confined")
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, statErr := os.Lstat(path)
		if statErr != nil {
			problems = append(problems, relative+": inspect untracked file: "+statErr.Error())
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			problems = append(problems, relative+": read untracked file: "+readErr.Error())
			continue
		}
		if bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		for index, line := range bytes.Split(data, []byte{'\n'}) {
			line = bytes.TrimSuffix(line, []byte{'\r'})
			if len(line) > 0 && (line[len(line)-1] == ' ' || line[len(line)-1] == '\t') {
				problems = append(problems, fmt.Sprintf("%s:%d: trailing whitespace", relative, index+1))
			}
			if bytes.HasPrefix(line, []byte("<<<<<<< ")) || bytes.Equal(line, []byte("=======")) || bytes.HasPrefix(line, []byte(">>>>>>> ")) {
				problems = append(problems, fmt.Sprintf("%s:%d: unresolved conflict marker", relative, index+1))
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		result.ExitCode = 1
		result.Error = strings.Join(problems, "; ")
	} else {
		result.Stdout = "untracked text files contain no trailing whitespace or conflict markers\n"
	}
	result.Duration = time.Since(started)
	return result
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
		Environment: map[string]string{
			"GOFLAGS": "-mod=readonly",
			"GOWORK":  filepath.Join(root, "go.work"),
		},
		Timeout: 10 * time.Minute,
	}
}

func agentReleaseTest(root string) command.Spec {
	return command.Spec{
		ID:          "agent-release-test",
		Description: "run the complete Agent module test target independently",
		Directory:   root,
		Args:        []string{"make", "test-agent"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     30 * time.Minute,
	}
}

func agentReleaseBuild(root string) command.Spec {
	return command.Spec{
		ID:          "agent-build",
		Description: "build the mss and mss-mcp Agent executables independently",
		Directory:   root,
		Args:        []string{"make", "build-agent"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     20 * time.Minute,
	}
}

func strictAgentDoctor(root string) command.Spec {
	return command.Spec{
		ID:          "agent-doctor-strict",
		Description: "validate the Agent environment contract in strict mode",
		Directory:   root,
		Args:        []string{"go", "run", "./cmd/mss", "doctor", "--strict", "--component", "agent", "--format", "json"},
		Environment: map[string]string{"GOFLAGS": "-mod=readonly", "GOWORK": filepath.Join(root, "go.work")},
		Timeout:     10 * time.Minute,
	}
}

func strictBackendDoctor(root string) command.Spec {
	return command.Spec{
		ID:          "backend-doctor-strict",
		Description: "validate the Admin environment contract in strict mode",
		Directory:   root,
		Args:        []string{"go", "run", "./cmd/mss", "doctor", "--strict", "--component", "backend", "--format", "json"},
		Environment: map[string]string{"GOFLAGS": "-mod=readonly", "GOWORK": filepath.Join(root, "go.work")},
		Timeout:     10 * time.Minute,
	}
}

func skillContractValidation(root string) command.Spec {
	return command.Spec{
		ID:          "agent-skills-validation",
		Description: "validate checked-in Agent skill contracts",
		Directory:   root,
		Args:        []string{"go", "run", "./cmd/mss", "skills", "validate", "--format", "json"},
		Environment: map[string]string{"GOFLAGS": "-mod=readonly", "GOWORK": filepath.Join(root, "go.work")},
		Timeout:     10 * time.Minute,
	}
}

func foundationCompatibility(root string) command.Spec {
	return command.Spec{
		ID:          "foundation-compatibility",
		Description: "qualify standalone package-first generation and upgrade behavior",
		Directory:   root,
		Args:        []string{"bash", "tools/compatibility/test-standalone-mss-consumer.sh", "--upgrade"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     60 * time.Minute,
	}
}

func releaseContractTest(root string) command.Spec {
	return command.Spec{
		ID:          "release-contract-test",
		Description: "validate release policy and workflow contracts",
		Directory:   root,
		Args:        []string{"python3", "-m", "unittest", "discover", "-s", "tools/release", "-p", "test_*.py"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     20 * time.Minute,
	}
}

func releaseWorkflowContractTest(root string) command.Spec {
	return command.Spec{
		ID:          "release-workflow-contract-test",
		Description: "validate changed release workflow and policy contracts without requiring a clean feature-freeze commit",
		Directory:   root,
		Args: []string{
			"python3", "-m", "unittest",
			"tools.release.test_root_release_workflow",
			"tools.release.test_container_workflow",
			"tools.release.test_workflow_governance",
			"tools.release.test_check_release_policy",
		},
		Environment: map[string]string{"CI": "true"},
		Timeout:     20 * time.Minute,
	}
}

func frameworkTest(root string) command.Spec {
	return command.Spec{
		ID:          "framework-test",
		Description: "test the reusable mss-boot module independently",
		Directory:   filepath.Join(root, "mss-boot"),
		Args:        []string{"go", "test", "./..."},
		Environment: map[string]string{"GOFLAGS": "-mod=readonly", "GOWORK": "off"},
		Timeout:     20 * time.Minute,
	}
}

func frameworkReleaseQualification(root string) command.Spec {
	return command.Spec{
		ID:          "framework-release-qualification",
		Description: "run Framework race, coverage, vet, tidy, and independent-module qualification",
		Directory:   root,
		Args:        []string{"make", "verify-framework"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     60 * time.Minute,
	}
}

func presentationThinHostContract(root string) command.Spec {
	return command.Spec{
		ID:          "presentation-thin-host-contract",
		Description: "qualify the fixed Foundation presentation inventory plus generated business composition through external Go and npm consumers",
		Directory:   root,
		Args:        []string{"bash", "tools/compatibility/test-presentation-thin-host-contract.sh"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     30 * time.Minute,
	}
}

func backendTest(root string) command.Spec {
	return command.Spec{
		ID:          "backend-test",
		Description: "run the Admin application tests in the repository workspace",
		Directory:   filepath.Join(root, "admin"),
		Args: []string{
			"go", "test",
			"-coverprofile=" + filepath.Join(root, ".mss", "reports", "admin-coverage.out"),
			"./...",
		},
		Environment: map[string]string{
			"GOFLAGS": "-mod=readonly",
			"GOWORK":  filepath.Join(root, "go.work"),
		},
		Timeout: 20 * time.Minute,
	}
}

func backendBuild(root string) command.Spec {
	return command.Spec{
		ID:          "backend-build",
		Description: "build the admin backend",
		Directory:   filepath.Join(root, "admin"),
		Args:        []string{"go", "build", "./..."},
		Environment: map[string]string{
			"CGO_ENABLED": "0",
			"GOFLAGS":     "-mod=readonly",
			"GOWORK":      filepath.Join(root, "go.work"),
		},
		Timeout: 10 * time.Minute,
	}
}

func backendReleaseQualification(root string) command.Spec {
	return command.Spec{
		ID:          "backend-release-qualification",
		Description: "run Admin race, coverage, vet, module metadata, external consumer, and build qualification",
		Directory:   root,
		Args:        []string{"make", "verify-admin-preview"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     60 * time.Minute,
	}
}

func frontendLint(root string) command.Spec {
	return command.Spec{
		ID:          "frontend-lint",
		Description: "run the Ant Design 6 frontend lint and TypeScript checks",
		Directory:   filepath.Join(root, "web", "antd-v6"),
		Args:        []string{"corepack", "pnpm@10.34.5", "lint"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     15 * time.Minute,
	}
}

func frontendTest(root string) command.Spec {
	return command.Spec{
		ID:          "frontend-test",
		Description: "run the Ant Design 6 frontend unit tests",
		Directory:   filepath.Join(root, "web", "antd-v6"),
		Args:        []string{"corepack", "pnpm@10.34.5", "test:ci"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     20 * time.Minute,
	}
}

func frontendBuild(root string) command.Spec {
	return command.Spec{
		ID:          "frontend-build",
		Description: "build the Ant Design 6 release frontend",
		Directory:   filepath.Join(root, "web", "antd-v6"),
		Args:        []string{"corepack", "pnpm@10.34.5", "build:release"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     20 * time.Minute,
	}
}

func frontendQualification(root string) command.Spec {
	return command.Spec{
		ID:          "frontend-qualification",
		Description: "qualify dependency policy, lint, unit behavior, release build, delivery, and browser behavior for Ant Design 6",
		Directory:   root,
		Args:        []string{"make", "web-v6-qualify"},
		Environment: map[string]string{"CI": "true"},
		Timeout:     60 * time.Minute,
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

func thinHostBackendTest(ctx *project.Context) command.Spec {
	return command.Spec{
		ID:          "backend-test",
		Description: "test the Thin Host backend and composed business modules independently",
		Directory:   layoutDirectory(ctx.Root, ctx.Project.Spec.RepositoryLayout["backend"]),
		Args: []string{
			"go", "test",
			"-coverprofile=" + filepath.Join(ctx.Root, ".mss", "reports", "thin-host-coverage.out"),
			"./...",
		},
		Environment: map[string]string{"GOFLAGS": "-mod=readonly", "GOWORK": "off"},
		Timeout:     20 * time.Minute,
	}
}

func thinHostBackendBuild(ctx *project.Context) command.Spec {
	return command.Spec{
		ID:          "backend-build",
		Description: "build the Thin Host Admin and business composition",
		Directory:   layoutDirectory(ctx.Root, ctx.Project.Spec.RepositoryLayout["backend"]),
		Args:        []string{"go", "build", "./..."},
		Environment: map[string]string{"CGO_ENABLED": "0", "GOFLAGS": "-mod=readonly", "GOWORK": "off"},
		Timeout:     10 * time.Minute,
	}
}

func thinHostFrontendLint(ctx *project.Context) command.Spec {
	return thinHostFrontendCommand(ctx, "frontend-lint", "lint", "run Thin Host frontend lint and TypeScript checks", 15*time.Minute)
}

func thinHostFrontendTest(ctx *project.Context) command.Spec {
	return thinHostFrontendCommand(ctx, "frontend-test", "test", "run Thin Host frontend unit tests", 20*time.Minute)
}

func thinHostFrontendBuild(ctx *project.Context) command.Spec {
	return thinHostFrontendCommand(ctx, "frontend-build", "build", "build the single composed Thin Host Umi application", 20*time.Minute)
}

func thinHostFrontendCommand(ctx *project.Context, id, script, description string, timeout time.Duration) command.Spec {
	version := strings.TrimSpace(ctx.Project.Spec.Frontend.PackageManagerVersion)
	if version == "" {
		version = "10.34.5"
	}
	return command.Spec{
		ID:          id,
		Description: description,
		Directory:   layoutDirectory(ctx.Root, ctx.Project.Spec.RepositoryLayout["frontend"]),
		Args:        []string{"corepack", "pnpm@" + version, "run", script},
		Environment: map[string]string{"CI": "true"},
		Timeout:     timeout,
	}
}

func layoutDirectory(root, relative string) string {
	relative = strings.TrimSpace(relative)
	if relative == "" || relative == "." {
		return root
	}
	return filepath.Join(root, filepath.FromSlash(relative))
}

func moduleScopePaths(ctx *project.Context, module string) []string {
	layout := ctx.Project.Spec.RepositoryLayout
	modules := strings.TrimSpace(layout["modules"])
	if modules == "" {
		modules = "modules"
	}
	paths := []string{filepath.ToSlash(filepath.Join(filepath.FromSlash(modules), module))}
	generated := strings.TrimSpace(layout["generated"])
	if generated != "" {
		paths = append(paths,
			filepath.ToSlash(filepath.Join(filepath.FromSlash(generated), "modules", module)),
			filepath.ToSlash(filepath.Join(filepath.FromSlash(layout["frontend"]), "src", "pages", "generated")),
		)
	}
	documentation := strings.TrimSpace(layout["documentation"])
	if documentation != "" {
		if ctx.LayoutKind() == "foundation" {
			paths = append(paths, filepath.ToSlash(filepath.Join(filepath.FromSlash(documentation), "docs", "modules", module+".md")))
		} else {
			paths = append(paths, filepath.ToSlash(filepath.Join(filepath.FromSlash(documentation), "modules", module+".md")))
		}
	}
	return paths
}

func focusedModuleTest(ctx *project.Context, module string) (command.Spec, string, error) {
	backendDirectory := layoutDirectory(ctx.Root, ctx.Project.Spec.RepositoryLayout["backend"])
	modules := strings.TrimSpace(ctx.Project.Spec.RepositoryLayout["modules"])
	if modules == "" {
		modules = "modules"
	}
	moduleDirectory := filepath.Join(ctx.Root, filepath.FromSlash(modules), module)
	relative, err := filepath.Rel(backendDirectory, moduleDirectory)
	if err != nil {
		return command.Spec{}, "", fmt.Errorf("resolve module verification path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return command.Spec{}, "", errors.New("project modules directory must be inside the backend module root")
	}
	packagePattern := "./" + strings.TrimPrefix(filepath.ToSlash(relative), "./") + "/..."
	environment := map[string]string{
		"GOFLAGS": "-mod=readonly",
		"GOWORK":  filepath.Join(ctx.Root, "go.work"),
	}
	if ctx.LayoutKind() != "foundation" {
		environment["GOWORK"] = "off"
	}
	return command.Spec{
		ID:          "module-test:" + module,
		Description: "run focused tests for module " + module,
		Directory:   backendDirectory,
		Args:        []string{"go", "test", packagePattern},
		Environment: environment,
		Timeout:     10 * time.Minute,
	}, moduleDirectory, nil
}

func normalizedLayoutPrefix(relative string) string {
	relative = strings.Trim(filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(relative)))), "/")
	if relative == "" || relative == "." {
		return ""
	}
	return relative + "/"
}

func isThinHostFrontend(ctx *project.Context, path string) bool {
	prefix := normalizedLayoutPrefix(ctx.Project.Spec.RepositoryLayout["frontend"])
	return prefix != "" && strings.HasPrefix(path, prefix)
}

func thinHostFrontendBuildSensitive(ctx *project.Context, path string) bool {
	frontend := strings.TrimSuffix(normalizedLayoutPrefix(ctx.Project.Spec.RepositoryLayout["frontend"]), "/")
	generated := strings.TrimSuffix(normalizedLayoutPrefix(ctx.Project.Spec.RepositoryLayout["generated"]), "/")
	return strings.HasPrefix(path, frontend+"/config/") ||
		(generated != "" && strings.HasPrefix(path, generated+"/")) ||
		strings.HasPrefix(path, frontend+"/src/pages/") ||
		path == frontend+"/package.json" ||
		path == frontend+"/pnpm-lock.yaml" ||
		path == frontend+"/tsconfig.json"
}

func isThinHostBackend(ctx *project.Context, path string) bool {
	if isThinHostFrontend(ctx, path) || strings.HasPrefix(path, ".mss/") || strings.HasPrefix(path, "docs/") {
		return false
	}
	modulesPrefix := normalizedLayoutPrefix(ctx.Project.Spec.RepositoryLayout["modules"])
	if modulesPrefix != "" && strings.HasPrefix(path, modulesPrefix) {
		return true
	}
	backendPrefix := normalizedLayoutPrefix(ctx.Project.Spec.RepositoryLayout["backend"])
	if backendPrefix != "" && strings.HasPrefix(path, backendPrefix) {
		return strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "go.mod") || strings.HasSuffix(path, "go.sum")
	}
	return path == "go.mod" || path == "go.sum" || strings.HasPrefix(path, "cmd/") || strings.HasSuffix(path, ".go")
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

func presentationThinHostContractSensitive(path string) bool {
	for _, exact := range []string{
		".github/workflows/admin-distribution-compatibility.yml",
		"go.mod",
		"go.sum",
		"go.work",
		"go.work.sum",
		"admin/go.mod",
		"admin/go.sum",
		"tools/compatibility/test-presentation-thin-host-contract.sh",
		"web/antd-v6/.npmignore",
		"web/antd-v6/package.json",
		"web/antd-v6/pnpm-lock.yaml",
		"web/antd-v6/src/.npmignore",
	} {
		if path == exact {
			return true
		}
	}
	for _, prefix := range []string{
		".mss/core-pages/",
		".mss/modules/",
		"admin/app/",
		"admin/business/",
		"admin/modules/",
		"admin/presentation/",
		"cmd/mss/",
		"internal/mss/generator/",
		"internal/mss/spec/admin_presentation",
		"internal/mss/spec/core_presentation",
		"internal/mss/spec/presentation",
		"templates/application/",
		"templates/module/",
		"web/antd-v6/package/",
		"web/antd-v6/src/generated/core-presentation-registry.generated.ts",
		"web/antd-v6/src/generated/modules/",
		"web/antd-v6/src/generated/presentation-registry.generated.ts",
		"web/antd-v6/src/modules/",
		"web/antd-v6/src/pages/",
		"web/antd-v6/src/shared/presentation/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func releaseWorkflowContractSensitive(path string) bool {
	return strings.HasPrefix(path, ".github/workflows/") ||
		strings.HasPrefix(path, "tools/release/") ||
		path == ".mss/release-policy.yaml" ||
		path == ".mss/release-qualification.json" ||
		path == ".mss/commands.yaml" ||
		path == ".agents/skills/mss-release/SKILL.md" ||
		path == "Makefile"
}

func isFrontend(path string) bool {
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
	if err := temporary.Chmod(0o600); err != nil {
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
