package feature

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/generator"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

// Options controls one read-only Feature implementation plan.
type Options struct {
	Root        string
	FeaturePath string
}

// Plan combines Feature intent, validated modules, generation impact, and acceptance evidence.
type Plan struct {
	Feature       FeatureSummary          `json:"feature"`
	Root          string                  `json:"root"`
	FeaturePath   string                  `json:"featurePath"`
	Success       bool                    `json:"success"`
	Goals         []string                `json:"goals"`
	NonGoals      []string                `json:"nonGoals"`
	Modules       []ModulePlan            `json:"modules"`
	Requirements  []RequirementPlan       `json:"requirements"`
	Constraints   []spec.FeatureConstraint `json:"constraints"`
	Acceptance    []AcceptancePlan        `json:"acceptance"`
	Risks         []spec.FeatureRisk      `json:"risks,omitempty"`
	Validation    spec.FeatureValidation  `json:"validation"`
	Rollout       spec.FeatureRollout     `json:"rollout"`
	Issues        []string                `json:"issues,omitempty"`
}

// FeatureSummary is the stable identity used by Agent handoffs.
type FeatureSummary struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description,omitempty"`
	Owner       string            `json:"owner"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// ModulePlan is one validated vertical-module impact.
type ModulePlan struct {
	Name             string         `json:"name"`
	Operation        string         `json:"operation"`
	Description      string         `json:"description,omitempty"`
	SpecPath         string         `json:"specPath,omitempty"`
	SpecValid        bool           `json:"specValid"`
	SpecName         string         `json:"specName,omitempty"`
	GenerationDryRun bool           `json:"generationDryRun,omitempty"`
	GeneratedOutputs int            `json:"generatedOutputs,omitempty"`
	GenerationActions map[string]int `json:"generationActions,omitempty"`
	Issue            string         `json:"issue,omitempty"`
}

// RequirementPlan keeps actor/module/permission/rule context together.
type RequirementPlan struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Actor       string   `json:"actor"`
	Module      string   `json:"module"`
	Permission  string   `json:"permission,omitempty"`
	Rules       []string `json:"rules"`
	Acceptance  []string `json:"acceptance"`
}

// AcceptancePlan presents required evidence without executing arbitrary commands from a spec.
type AcceptancePlan struct {
	ID          string                    `json:"id"`
	Requirement string                    `json:"requirement,omitempty"`
	Statement   string                    `json:"statement"`
	Level       string                    `json:"level"`
	Required    bool                      `json:"required"`
	Evidence    []spec.AcceptanceEvidence `json:"evidence"`
}

// Build creates a read-only implementation plan and never executes Feature evidence commands.
func Build(options Options) (Plan, error) {
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve repository root: %w", err)
	}
	featurePath, relative, err := resolveFile(root, options.FeaturePath)
	if err != nil {
		return Plan{}, err
	}
	feature, err := spec.LoadFeature(featurePath)
	if err != nil {
		return Plan{}, err
	}
	feature.SourcePath = relative
	plan := Plan{
		Feature: FeatureSummary{
			Name:        feature.Metadata.Name,
			DisplayName: feature.Metadata.DisplayName,
			Description: feature.Metadata.Description,
			Owner:       feature.Metadata.Owner,
			Labels:      cloneLabels(feature.Metadata.Labels),
		},
		Root:        root,
		FeaturePath: relative,
		Success:     true,
		Goals:       append([]string(nil), feature.Spec.Goals...),
		NonGoals:    append([]string(nil), feature.Spec.NonGoals...),
		Constraints: append([]spec.FeatureConstraint(nil), feature.Spec.Constraints...),
		Risks:       append([]spec.FeatureRisk(nil), feature.Spec.Risks...),
		Validation:  feature.Spec.Validation,
		Rollout:     feature.Spec.Rollout,
		Modules:     make([]ModulePlan, 0, len(feature.Spec.Modules)),
		Requirements: make([]RequirementPlan, 0, len(feature.Spec.Requirements)),
		Acceptance:  make([]AcceptancePlan, 0, len(feature.Spec.Acceptance)),
	}

	for _, module := range feature.Spec.Modules {
		modulePlan := buildModulePlan(root, module)
		plan.Modules = append(plan.Modules, modulePlan)
		if modulePlan.Issue != "" {
			plan.Success = false
			plan.Issues = append(plan.Issues, module.Name+": "+modulePlan.Issue)
		}
	}

	acceptanceByRequirement := make(map[string][]string)
	for _, criterion := range feature.Spec.Acceptance {
		plan.Acceptance = append(plan.Acceptance, AcceptancePlan{
			ID:          criterion.ID,
			Requirement: criterion.Requirement,
			Statement:   criterion.Statement,
			Level:       criterion.Level,
			Required:    criterion.Required,
			Evidence:    append([]spec.AcceptanceEvidence(nil), criterion.Evidence...),
		})
		if criterion.Requirement != "" {
			acceptanceByRequirement[criterion.Requirement] = append(acceptanceByRequirement[criterion.Requirement], criterion.ID)
		}
	}
	for _, requirement := range feature.Spec.Requirements {
		acceptance := append([]string(nil), acceptanceByRequirement[requirement.ID]...)
		sort.Strings(acceptance)
		plan.Requirements = append(plan.Requirements, RequirementPlan{
			ID:          requirement.ID,
			Title:       requirement.Title,
			Description: requirement.Description,
			Priority:    requirement.Priority,
			Actor:       requirement.Actor,
			Module:      requirement.Module,
			Permission:  requirement.Permission,
			Rules:       append([]string(nil), requirement.Rules...),
			Acceptance:  acceptance,
		})
	}

	sort.Strings(plan.Issues)
	sort.SliceStable(plan.Modules, func(i, j int) bool { return plan.Modules[i].Name < plan.Modules[j].Name })
	sort.SliceStable(plan.Requirements, func(i, j int) bool { return plan.Requirements[i].ID < plan.Requirements[j].ID })
	sort.SliceStable(plan.Acceptance, func(i, j int) bool { return plan.Acceptance[i].ID < plan.Acceptance[j].ID })
	if !plan.Success {
		return plan, errors.New("Feature implementation plan contains invalid or missing module contracts")
	}
	return plan, nil
}

func buildModulePlan(root string, module spec.FeatureModule) ModulePlan {
	plan := ModulePlan{
		Name:        module.Name,
		Operation:   module.Operation,
		Description: module.Description,
		SpecPath:    module.SpecPath,
	}
	if module.SpecPath == "" {
		if module.Operation == "create" || module.Operation == "extend" {
			plan.Issue = "create/extend operations require specPath"
		}
		return plan
	}
	path, relative, err := resolveFile(root, module.SpecPath)
	if err != nil {
		plan.Issue = err.Error()
		return plan
	}
	plan.SpecPath = relative
	document, err := spec.ValidateFile(path)
	if err != nil {
		plan.Issue = err.Error()
		return plan
	}
	if document.Kind != "AdminModule" {
		plan.Issue = fmt.Sprintf("specification kind %s is not AdminModule", document.Kind)
		return plan
	}
	if document.Name != module.Name {
		plan.Issue = fmt.Sprintf("Feature module name %s does not match AdminModule name %s", module.Name, document.Name)
		return plan
	}
	moduleSpec, ok := document.Document.(*spec.ModuleSpec)
	if !ok {
		plan.Issue = "validated AdminModule has an unexpected Go representation"
		return plan
	}
	moduleSpec.SourcePath = relative
	plan.SpecValid = true
	plan.SpecName = moduleSpec.Metadata.Name
	if module.Operation != "create" && module.Operation != "extend" {
		return plan
	}
	generation, err := generator.Generate(moduleSpec, generator.Options{Root: root})
	if err != nil {
		plan.Issue = err.Error()
		return plan
	}
	plan.GenerationDryRun = generation.DryRun
	plan.GeneratedOutputs = len(generation.Changes)
	plan.GenerationActions = make(map[string]int)
	for _, change := range generation.Changes {
		plan.GenerationActions[string(change.Action)]++
	}
	return plan
}

// JSON returns stable indented JSON.
func (p Plan) JSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// Text renders a concise implementation handoff.
func (p Plan) Text() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Feature: %s (%s)\n", p.Feature.DisplayName, p.Feature.Name)
	fmt.Fprintf(&builder, "owner: %s\n", p.Feature.Owner)
	fmt.Fprintf(&builder, "path: %s\n", p.FeaturePath)
	fmt.Fprintf(&builder, "success: %t\n", p.Success)
	fmt.Fprintf(&builder, "modules: %d\n", len(p.Modules))
	fmt.Fprintf(&builder, "requirements: %d\n", len(p.Requirements))
	fmt.Fprintf(&builder, "acceptance: %d\n", len(p.Acceptance))
	fmt.Fprintf(&builder, "rollout: %s\n\n", p.Rollout.Strategy)
	for _, module := range p.Modules {
		fmt.Fprintf(&builder, "- module %s [%s] spec=%s outputs=%d", module.Name, module.Operation, module.SpecPath, module.GeneratedOutputs)
		if module.Issue != "" {
			fmt.Fprintf(&builder, " issue=%s", module.Issue)
		}
		builder.WriteByte('\n')
	}
	for _, requirement := range p.Requirements {
		fmt.Fprintf(&builder, "- requirement %s [%s] actor=%s module=%s", requirement.ID, requirement.Priority, requirement.Actor, requirement.Module)
		if requirement.Permission != "" {
			fmt.Fprintf(&builder, " permission=%s", requirement.Permission)
		}
		fmt.Fprintf(&builder, " acceptance=%s\n", strings.Join(requirement.Acceptance, ","))
	}
	for _, issue := range p.Issues {
		fmt.Fprintf(&builder, "- issue: %s\n", issue)
	}
	return builder.String()
}

func resolveFile(root, input string) (string, string, error) {
	if strings.TrimSpace(input) == "" {
		return "", "", errors.New("Feature specification path is required")
	}
	if filepath.IsAbs(input) {
		return "", "", errors.New("absolute Feature paths are not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(input))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", errors.New("Feature path escapes repository root")
	}
	absolute := filepath.Join(root, clean)
	info, err := os.Stat(absolute)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", errors.New("Feature path must reference a regular file")
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
		return "", "", errors.New("resolved Feature path escapes repository root")
	}
	return realPath, filepath.ToSlash(relative), nil
}

func cloneLabels(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
