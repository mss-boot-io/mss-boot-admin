package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/blueprint"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/doctor"
	featurecmd "github.com/mss-boot-io/mss-boot-admin/internal/mss/feature"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/generator"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/mcp"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/skills"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/verify"
)

const catalogPath = ".mss/evals/catalog.yaml"

var caseIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// Catalog is the machine-readable set of foundation evaluation cases.
type Catalog struct {
	APIVersion string          `yaml:"apiVersion" json:"apiVersion"`
	Kind       string          `yaml:"kind" json:"kind"`
	Metadata   CatalogMetadata `yaml:"metadata" json:"metadata"`
	Spec       CatalogSpec     `yaml:"spec" json:"spec"`
}

// CatalogMetadata identifies the project and evaluation version.
type CatalogMetadata struct {
	Project string `yaml:"project" json:"project"`
	Version string `yaml:"version" json:"version"`
}

// CatalogSpec contains evaluation cases.
type CatalogSpec struct {
	Cases []Case `yaml:"cases" json:"cases"`
}

// Case describes one user-visible Agent capability scenario.
type Case struct {
	ID          string      `yaml:"id" json:"id"`
	Title       string      `yaml:"title" json:"title"`
	Description string      `yaml:"description" json:"description"`
	Tags        []string    `yaml:"tags,omitempty" json:"tags,omitempty"`
	Checks      []CheckSpec `yaml:"checks" json:"checks"`
}

// CheckSpec selects one deterministic built-in evaluator.
type CheckSpec struct {
	Type    string `yaml:"type" json:"type"`
	Path    string `yaml:"path,omitempty" json:"path,omitempty"`
	Mode    string `yaml:"mode,omitempty" json:"mode,omitempty"`
	Minimum int    `yaml:"minimum,omitempty" json:"minimum,omitempty"`
	Maximum int    `yaml:"maximum,omitempty" json:"maximum,omitempty"`
}

// Report is persisted for agents, CI, and release review.
type Report struct {
	Project        string       `json:"project"`
	CatalogVersion string       `json:"catalogVersion"`
	Root           string       `json:"root"`
	GeneratedAt    time.Time    `json:"generatedAt"`
	Success        bool         `json:"success"`
	Cases          []CaseResult `json:"cases"`
}

// RunOptions selects evaluation cases and qualification-only dependency seams.
// An empty ContributorFrontendRegistryURL keeps the normal public npm lookup.
// The Blueprint resolver remains responsible for accepting only an explicit
// loopback HTTP override.
type RunOptions struct {
	CaseIDs                        []string
	ContributorFrontendRegistryURL string
}

// CaseResult is the outcome of one evaluation scenario.
type CaseResult struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Success  bool          `json:"success"`
	Duration time.Duration `json:"duration"`
	Checks   []CheckResult `json:"checks"`
}

// CheckResult contains stable diagnostics and structured evidence.
type CheckResult struct {
	Type     string         `json:"type"`
	Success  bool           `json:"success"`
	Duration time.Duration  `json:"duration"`
	Message  string         `json:"message"`
	Details  map[string]any `json:"details,omitempty"`
}

// Load reads and validates the evaluation catalog.
func Load(root string) (*Catalog, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(absoluteRoot, filepath.FromSlash(catalogPath)))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", catalogPath, err)
	}
	catalog := &Catalog{}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(catalog); err != nil {
		return nil, fmt.Errorf("parse %s: %w", catalogPath, err)
	}
	if err := catalog.Validate(); err != nil {
		return nil, err
	}
	catalog.Sort()
	return catalog, nil
}

// Validate checks catalog identity, unique cases, and supported check types.
func (c *Catalog) Validate() error {
	var problems []string
	if c.APIVersion != "mss.io/v1alpha1" {
		problems = append(problems, "apiVersion must equal mss.io/v1alpha1")
	}
	if c.Kind != "AgentEvaluationCatalog" {
		problems = append(problems, "kind must equal AgentEvaluationCatalog")
	}
	if strings.TrimSpace(c.Metadata.Project) == "" {
		problems = append(problems, "metadata.project is required")
	}
	if strings.TrimSpace(c.Metadata.Version) == "" {
		problems = append(problems, "metadata.version is required")
	}
	if len(c.Spec.Cases) == 0 {
		problems = append(problems, "spec.cases must contain at least one evaluation")
	}
	knownChecks := map[string]bool{
		"project-context":            true,
		"skills-contract":            true,
		"doctor-required":            true,
		"module-spec":                true,
		"module-generation-plan":     true,
		"application-blueprint-plan": true,
		"feature-spec":               true,
		"feature-plan":               true,
		"validation-plan":            true,
		"mcp-tools":                  true,
	}
	seen := make(map[string]bool)
	for index, evaluation := range c.Spec.Cases {
		prefix := fmt.Sprintf("spec.cases[%d]", index)
		if !caseIDPattern.MatchString(evaluation.ID) {
			problems = append(problems, prefix+".id must be lower-case kebab-case")
		}
		if seen[evaluation.ID] {
			problems = append(problems, prefix+".id is duplicated")
		}
		seen[evaluation.ID] = true
		if strings.TrimSpace(evaluation.Title) == "" {
			problems = append(problems, prefix+".title is required")
		}
		if len(evaluation.Checks) == 0 {
			problems = append(problems, prefix+".checks must not be empty")
		}
		for checkIndex, check := range evaluation.Checks {
			checkPrefix := fmt.Sprintf("%s.checks[%d]", prefix, checkIndex)
			if !knownChecks[check.Type] {
				problems = append(problems, checkPrefix+".type is unsupported: "+check.Type)
			}
			if (check.Type == "module-spec" ||
				check.Type == "module-generation-plan" ||
				check.Type == "feature-spec" ||
				check.Type == "feature-plan") && strings.TrimSpace(check.Path) == "" {
				problems = append(problems, checkPrefix+".path is required")
			}
			if check.Minimum < 0 {
				problems = append(problems, checkPrefix+".minimum must be non-negative")
			}
			if check.Maximum < 0 {
				problems = append(problems, checkPrefix+".maximum must be non-negative")
			}
			if check.Maximum > 0 && check.Type != "application-blueprint-plan" {
				problems = append(problems, checkPrefix+".maximum is supported only for application-blueprint-plan")
			}
			if check.Maximum > 0 && check.Minimum > check.Maximum {
				problems = append(problems, checkPrefix+".minimum must not exceed maximum")
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// Sort normalizes catalog ordering for stable output and cacheability.
func (c *Catalog) Sort() {
	for index := range c.Spec.Cases {
		sort.Strings(c.Spec.Cases[index].Tags)
	}
	sort.SliceStable(c.Spec.Cases, func(i, j int) bool {
		return c.Spec.Cases[i].ID < c.Spec.Cases[j].ID
	})
}

// List returns evaluation cases, optionally filtered by exact IDs.
func (c *Catalog) List(ids []string) ([]Case, error) {
	if len(ids) == 0 {
		return append([]Case(nil), c.Spec.Cases...), nil
	}
	requested := make(map[string]bool, len(ids))
	for _, id := range ids {
		requested[strings.TrimSpace(id)] = true
	}
	result := make([]Case, 0, len(ids))
	for _, evaluation := range c.Spec.Cases {
		if requested[evaluation.ID] {
			result = append(result, evaluation)
			delete(requested, evaluation.ID)
		}
	}
	if len(requested) > 0 {
		missing := make([]string, 0, len(requested))
		for id := range requested {
			missing = append(missing, id)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("unknown evaluation case(s): %s", strings.Join(missing, ", "))
	}
	return result, nil
}

// Run executes selected cases. An empty CaseIDs set executes the complete catalog.
func Run(ctx context.Context, root string, options RunOptions) (Report, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	catalog, err := Load(absoluteRoot)
	if err != nil {
		return Report{}, err
	}
	projectContext, err := project.Load(absoluteRoot)
	if err != nil {
		return Report{}, err
	}
	cases, err := catalog.List(options.CaseIDs)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		Project:        catalog.Metadata.Project,
		CatalogVersion: catalog.Metadata.Version,
		Root:           absoluteRoot,
		GeneratedAt:    time.Now().UTC(),
		Success:        true,
		Cases:          make([]CaseResult, 0, len(cases)),
	}
	for _, evaluation := range cases {
		caseResult := runCase(ctx, absoluteRoot, projectContext, evaluation, options)
		report.Cases = append(report.Cases, caseResult)
		if !caseResult.Success {
			report.Success = false
		}
	}
	if err := WriteReport(absoluteRoot, report); err != nil {
		return report, err
	}
	if !report.Success {
		return report, errors.New("one or more Agent evaluation cases failed; see .mss/reports/evals/latest.md")
	}
	return report, nil
}

func runCase(ctx context.Context, root string, projectContext *project.Context, evaluation Case, options RunOptions) CaseResult {
	started := time.Now()
	result := CaseResult{
		ID:      evaluation.ID,
		Title:   evaluation.Title,
		Success: true,
		Checks:  make([]CheckResult, 0, len(evaluation.Checks)),
	}
	for _, check := range evaluation.Checks {
		checkResult := runCheck(ctx, root, projectContext, check, options)
		result.Checks = append(result.Checks, checkResult)
		if !checkResult.Success {
			result.Success = false
		}
	}
	result.Duration = time.Since(started)
	return result
}

func runCheck(ctx context.Context, root string, projectContext *project.Context, check CheckSpec, options RunOptions) CheckResult {
	started := time.Now()
	result := CheckResult{Type: check.Type, Success: true}
	var value any
	var err error

	switch check.Type {
	case "project-context":
		value, err = checkProjectContext(projectContext)
	case "skills-contract":
		value, err = checkSkills(root, check.Minimum)
	case "doctor-required":
		value, err = checkDoctor(ctx, projectContext)
	case "module-spec":
		value, err = checkModuleSpec(root, check.Path)
	case "module-generation-plan":
		value, err = checkModuleGeneration(root, check.Path, check.Minimum)
	case "application-blueprint-plan":
		value, err = checkApplicationBlueprint(ctx, root, check.Minimum, check.Maximum, options.ContributorFrontendRegistryURL)
	case "feature-spec":
		value, err = checkFeatureSpec(root, check.Path)
	case "feature-plan":
		value, err = checkFeaturePlan(root, check.Path, check.Minimum)
	case "validation-plan":
		value, err = checkValidationPlan(projectContext, check.Mode, check.Minimum)
	case "mcp-tools":
		value, err = checkMCPTools(check.Minimum)
	default:
		err = fmt.Errorf("unsupported evaluation check %q", check.Type)
	}

	result.Duration = time.Since(started)
	if err != nil {
		result.Success = false
		result.Message = err.Error()
		return result
	}
	result.Message = "passed"
	if details, ok := value.(map[string]any); ok {
		result.Details = details
	} else if value != nil {
		result.Details = map[string]any{"value": value}
	}
	return result
}

func checkProjectContext(ctx *project.Context) (map[string]any, error) {
	if ctx == nil {
		return nil, errors.New("project context is nil")
	}
	if err := ctx.Validate(); err != nil {
		return nil, err
	}
	return map[string]any{
		"project":      ctx.Project.Metadata.Name,
		"capabilities": len(ctx.Capabilities.Spec.Capabilities),
		"commands":     len(ctx.Commands.Spec.Commands),
	}, nil
}

func checkSkills(root string, minimum int) (map[string]any, error) {
	report, err := skills.Discover(root)
	if err != nil {
		return map[string]any{"skills": len(report.Skills), "issues": report.Issues}, err
	}
	if len(report.Skills) < minimum {
		return nil, fmt.Errorf("discovered %d skills, expected at least %d", len(report.Skills), minimum)
	}
	return map[string]any{"skills": len(report.Skills), "valid": report.Valid}, nil
}

func checkDoctor(ctx context.Context, projectContext *project.Context) (map[string]any, error) {
	report := doctor.Run(ctx, projectContext, doctor.WithComponents(doctor.ComponentAgent))
	failedRequired := make([]string, 0)
	for _, check := range report.Checks {
		if check.Required && check.Status == doctor.StatusFail {
			failedRequired = append(failedRequired, check.ID)
		}
	}
	if len(failedRequired) > 0 {
		return map[string]any{"failedRequired": failedRequired}, fmt.Errorf("required doctor checks failed: %s", strings.Join(failedRequired, ", "))
	}
	return map[string]any{
		"ready":      report.Ready,
		"checks":     len(report.Checks),
		"components": report.Components,
	}, nil
}

func checkModuleSpec(root, inputPath string) (map[string]any, error) {
	absolute, relative, err := resolveFile(root, inputPath)
	if err != nil {
		return nil, err
	}
	module, err := spec.LoadModule(absolute)
	if err != nil {
		return nil, err
	}
	module.SourcePath = relative
	return map[string]any{
		"module":      module.Metadata.Name,
		"fields":      len(module.Spec.Entity.Fields),
		"permissions": len(module.Spec.Permissions),
		"path":        relative,
	}, nil
}

func checkModuleGeneration(root, inputPath string, minimum int) (map[string]any, error) {
	absolute, relative, err := resolveFile(root, inputPath)
	if err != nil {
		return nil, err
	}
	module, err := spec.LoadModule(absolute)
	if err != nil {
		return nil, err
	}
	module.SourcePath = relative
	plan, err := generator.Generate(module, generator.Options{Root: root})
	if err != nil {
		return nil, err
	}
	if len(plan.Changes) < minimum {
		return nil, fmt.Errorf("generation planned %d outputs, expected at least %d", len(plan.Changes), minimum)
	}
	actions := make(map[string]int)
	for _, change := range plan.Changes {
		actions[string(change.Action)]++
		if filepath.IsAbs(change.Path) || change.Path == ".." || strings.HasPrefix(filepath.Clean(change.Path), ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("generated path escapes repository root: %s", change.Path)
		}
	}
	return map[string]any{
		"module":  module.Metadata.Name,
		"outputs": len(plan.Changes),
		"actions": actions,
		"dryRun":  plan.DryRun,
	}, nil
}

func checkFeatureSpec(root, inputPath string) (map[string]any, error) {
	absolute, relative, err := resolveFile(root, inputPath)
	if err != nil {
		return nil, err
	}
	feature, err := spec.LoadFeature(absolute)
	if err != nil {
		return nil, err
	}
	feature.SourcePath = relative
	summary := feature.Summary()
	summary["path"] = relative
	return summary, nil
}

func checkFeaturePlan(root, inputPath string, minimum int) (map[string]any, error) {
	plan, err := featurecmd.Build(featurecmd.Options{Root: root, FeaturePath: inputPath})
	if err != nil {
		return nil, err
	}
	outputs := 0
	for _, module := range plan.Modules {
		outputs += module.GeneratedOutputs
	}
	if outputs < minimum {
		return nil, fmt.Errorf("Feature plan contains %d generated outputs, expected at least %d", outputs, minimum)
	}
	return map[string]any{
		"feature":      plan.Feature.Name,
		"modules":      len(plan.Modules),
		"requirements": len(plan.Requirements),
		"acceptance":   len(plan.Acceptance),
		"outputs":      outputs,
		"rollout":      plan.Rollout.Strategy,
	}, nil
}

func checkApplicationBlueprint(ctx context.Context, root string, minimum, maximum int, frontendRegistryURL string) (map[string]any, error) {
	plan, err := blueprint.Generate(ctx, applicationBlueprintOptions(root, frontendRegistryURL))
	if err != nil {
		return nil, err
	}
	if !plan.DryRun || !plan.Success {
		return nil, errors.New("application Blueprint plan must be a successful dry-run")
	}
	if err := validateApplicationBlueprintSize(plan.TotalFiles, minimum, maximum); err != nil {
		return nil, err
	}
	if plan.FoundationCommit == "" {
		return nil, errors.New("application Blueprint plan does not record a foundation commit")
	}
	actions := make(map[string]int)
	for _, change := range plan.Changes {
		actions[string(change.Action)]++
	}
	return map[string]any{
		"blueprint":        plan.Blueprint,
		"version":          plan.BlueprintVersion,
		"foundationCommit": plan.FoundationCommit,
		"files":            plan.TotalFiles,
		"bytes":            plan.TotalBytes,
		"actions":          actions,
		"minimumFiles":     minimum,
		"maximumFiles":     maximum,
	}, nil
}

func applicationBlueprintOptions(root, frontendRegistryURL string) blueprint.Options {
	return blueprint.Options{
		FoundationRoot:      root,
		FrontendRegistryURL: frontendRegistryURL,
		Application: blueprint.Application{
			Name:        "eval-admin",
			DisplayName: "Evaluation Administration",
			Module:      "github.com/example/eval-admin",
			Repository:  "example/eval-admin",
		},
	}
}

func validateApplicationBlueprintSize(totalFiles, minimum, maximum int) error {
	if totalFiles < minimum {
		return fmt.Errorf("application Blueprint planned %d files, expected at least %d", totalFiles, minimum)
	}
	if maximum > 0 && totalFiles > maximum {
		return fmt.Errorf("application Blueprint planned %d files, expected at most %d", totalFiles, maximum)
	}
	return nil
}

func checkValidationPlan(projectContext *project.Context, mode string, minimum int) (map[string]any, error) {
	if mode == "" {
		mode = string(verify.ModeAll)
	}
	plan, err := verify.PlanChecks(projectContext, verify.Options{Mode: verify.Mode(mode)})
	if err != nil {
		return nil, err
	}
	if len(plan.Checks) < minimum {
		return nil, fmt.Errorf("validation plan contains %d checks, expected at least %d", len(plan.Checks), minimum)
	}
	ids := make([]string, 0, len(plan.Checks))
	for _, check := range plan.Checks {
		ids = append(ids, check.ID)
	}
	return map[string]any{"mode": plan.Mode, "checks": ids}, nil
}

func checkMCPTools(minimum int) (map[string]any, error) {
	definitions := mcp.Tools()
	if len(definitions) < minimum {
		return nil, fmt.Errorf("MCP exposes %d tools, expected at least %d", len(definitions), minimum)
	}
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		if definition.Name == "" || len(definition.InputSchema) == 0 {
			return nil, errors.New("MCP tool definition is missing a name or input schema")
		}
		names = append(names, definition.Name)
	}
	if !sort.StringsAreSorted(names) {
		return nil, errors.New("MCP tool definitions are not deterministically sorted")
	}
	return map[string]any{"tools": names}, nil
}

// WriteReport stores the latest structured and human-readable evaluation report.
func WriteReport(root string, report Report) error {
	directory := filepath.Join(root, ".mss", "reports", "evals")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create evaluation report directory: %w", err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := writeAtomic(filepath.Join(directory, "latest.json"), data); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(directory, "latest.md"), []byte(report.Markdown()))
}

// ReadLatest reads the latest evaluation report.
func ReadLatest(root string) (Report, error) {
	data, err := os.ReadFile(filepath.Join(root, ".mss", "reports", "evals", "latest.json"))
	if err != nil {
		return Report{}, err
	}
	report := Report{}
	if err := json.Unmarshal(data, &report); err != nil {
		return Report{}, err
	}
	return report, nil
}

// JSON returns indented JSON.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// Markdown returns a concise Agent evaluation handoff.
func (r Report) Markdown() string {
	var builder strings.Builder
	builder.WriteString("# mss Agent evaluation report\n\n")
	fmt.Fprintf(&builder, "- Project: `%s`\n", r.Project)
	fmt.Fprintf(&builder, "- Catalog version: `%s`\n", r.CatalogVersion)
	fmt.Fprintf(&builder, "- Success: `%t`\n", r.Success)
	fmt.Fprintf(&builder, "- Generated at: `%s`\n\n", r.GeneratedAt.Format(time.RFC3339))
	for _, evaluation := range r.Cases {
		status := "PASS"
		if !evaluation.Success {
			status = "FAIL"
		}
		fmt.Fprintf(&builder, "## [%s] %s — %s\n\n", status, evaluation.ID, evaluation.Title)
		for _, check := range evaluation.Checks {
			checkStatus := "PASS"
			if !check.Success {
				checkStatus = "FAIL"
			}
			fmt.Fprintf(&builder, "- `%s` **%s**: %s\n", check.Type, checkStatus, check.Message)
		}
		builder.WriteByte('\n')
	}
	return builder.String()
}

// ListText renders the catalog for CLI users and agents.
func ListText(cases []Case) string {
	var builder strings.Builder
	for _, evaluation := range cases {
		fmt.Fprintf(&builder, "%s\t%s\n", evaluation.ID, evaluation.Title)
		if evaluation.Description != "" {
			fmt.Fprintf(&builder, "  %s\n", evaluation.Description)
		}
	}
	return builder.String()
}

func resolveFile(root, input string) (string, string, error) {
	if strings.TrimSpace(input) == "" {
		return "", "", errors.New("evaluation file path is required")
	}
	if filepath.IsAbs(input) {
		return "", "", errors.New("absolute evaluation paths are not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(input))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", errors.New("evaluation path escapes repository root")
	}
	absolute := filepath.Join(root, clean)
	info, err := os.Stat(absolute)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", errors.New("evaluation path must reference a regular file")
	}
	realPath, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", "", err
	}
	relative, err := filepath.Rel(root, realPath)
	if err != nil {
		return "", "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", errors.New("resolved evaluation path escapes repository root")
	}
	return realPath, filepath.ToSlash(relative), nil
}

func writeAtomic(path string, data []byte) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
