package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// FeatureSpec is a cross-module product, delivery, and executable acceptance contract.
type FeatureSpec struct {
	APIVersion string          `yaml:"apiVersion" json:"apiVersion"`
	Kind       string          `yaml:"kind" json:"kind"`
	Metadata   FeatureMetadata `yaml:"metadata" json:"metadata"`
	Spec       FeatureBody     `yaml:"spec" json:"spec"`
	SourcePath string          `yaml:"-" json:"sourcePath,omitempty"`
}

// FeatureMetadata identifies ownership and project labels.
type FeatureMetadata struct {
	Name        string            `yaml:"name" json:"name"`
	DisplayName string            `yaml:"displayName" json:"displayName"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Owner       string            `yaml:"owner" json:"owner"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// FeatureBody describes intent, actors, modules, constraints, acceptance, and rollout.
type FeatureBody struct {
	Problem      string                `yaml:"problem" json:"problem"`
	Goals        []string              `yaml:"goals" json:"goals"`
	NonGoals     []string              `yaml:"nonGoals" json:"nonGoals"`
	Actors       []FeatureActor        `yaml:"actors" json:"actors"`
	Modules      []FeatureModule       `yaml:"modules" json:"modules"`
	Requirements []FeatureRequirement  `yaml:"requirements" json:"requirements"`
	Constraints  []FeatureConstraint   `yaml:"constraints" json:"constraints"`
	Acceptance   []AcceptanceCriterion `yaml:"acceptance" json:"acceptance"`
	Risks        []FeatureRisk         `yaml:"risks,omitempty" json:"risks,omitempty"`
	Validation   FeatureValidation     `yaml:"validation" json:"validation"`
	Rollout      FeatureRollout        `yaml:"rollout" json:"rollout"`
}

// FeatureActor is a user or system role referenced by requirements.
type FeatureActor struct {
	ID          string `yaml:"id" json:"id"`
	DisplayName string `yaml:"displayName" json:"displayName"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// FeatureModule identifies a vertical module created, extended, deprecated, or removed.
type FeatureModule struct {
	Name        string            `yaml:"name" json:"name"`
	Kind        FeatureModuleKind `yaml:"kind,omitempty" json:"kind"`
	Operation   string            `yaml:"operation" json:"operation"`
	SpecPath    string            `yaml:"specPath,omitempty" json:"specPath,omitempty"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
}

// FeatureModuleKind distinguishes generated AdminModule contracts from
// cross-cutting infrastructure work that has no AdminModule specification.
type FeatureModuleKind string

const (
	FeatureModuleKindAdminModule    FeatureModuleKind = "admin-module"
	FeatureModuleKindInfrastructure FeatureModuleKind = "infrastructure"
)

// EffectiveKind preserves historical Feature contracts: a module with a
// specPath is an AdminModule, while one without a specPath is infrastructure.
func (m FeatureModule) EffectiveKind() FeatureModuleKind {
	kind := FeatureModuleKind(strings.TrimSpace(string(m.Kind)))
	if kind != "" {
		return kind
	}
	if strings.TrimSpace(m.SpecPath) != "" {
		return FeatureModuleKindAdminModule
	}
	return FeatureModuleKindInfrastructure
}

// Valid reports whether kind is one of the supported Feature module contracts.
func (k FeatureModuleKind) Valid() bool {
	return k == FeatureModuleKindAdminModule || k == FeatureModuleKindInfrastructure
}

// FeatureRequirement is one actor- and module-scoped requirement.
type FeatureRequirement struct {
	ID          string   `yaml:"id" json:"id"`
	Title       string   `yaml:"title" json:"title"`
	Description string   `yaml:"description" json:"description"`
	Priority    string   `yaml:"priority" json:"priority"`
	Actor       string   `yaml:"actor" json:"actor"`
	Module      string   `yaml:"module" json:"module"`
	Permission  string   `yaml:"permission,omitempty" json:"permission,omitempty"`
	Rules       []string `yaml:"rules" json:"rules"`
}

// FeatureConstraint is a non-negotiable implementation boundary.
type FeatureConstraint struct {
	ID        string `yaml:"id" json:"id"`
	Type      string `yaml:"type" json:"type"`
	Statement string `yaml:"statement" json:"statement"`
}

// AcceptanceCriterion connects a verifiable statement to evidence.
type AcceptanceCriterion struct {
	ID          string               `yaml:"id" json:"id"`
	Requirement string               `yaml:"requirement,omitempty" json:"requirement,omitempty"`
	Statement   string               `yaml:"statement" json:"statement"`
	Level       string               `yaml:"level" json:"level"`
	Phase       AcceptancePhase      `yaml:"phase" json:"phase"`
	Required    bool                 `yaml:"required" json:"required"`
	Evidence    []AcceptanceEvidence `yaml:"evidence" json:"evidence"`
}

// AcceptancePhase scopes evidence to one delivery or publication transition.
// A later phase is never an implicit prerequisite for an earlier transition.
type AcceptancePhase string

const (
	AcceptancePhaseCheckpoint      AcceptancePhase = "checkpoint"
	AcceptancePhaseFeatureFreeze   AcceptancePhase = "feature-freeze"
	AcceptancePhasePreFramework    AcceptancePhase = "pre-framework"
	AcceptancePhasePreRoot         AcceptancePhase = "pre-root"
	AcceptancePhasePostPublication AcceptancePhase = "post-publication"
)

var orderedAcceptancePhases = []AcceptancePhase{
	AcceptancePhaseCheckpoint,
	AcceptancePhaseFeatureFreeze,
	AcceptancePhasePreFramework,
	AcceptancePhasePreRoot,
	AcceptancePhasePostPublication,
}

// AcceptancePhases returns the supported phases in lifecycle order.
func AcceptancePhases() []AcceptancePhase {
	return append([]AcceptancePhase(nil), orderedAcceptancePhases...)
}

// Valid reports whether phase is one of the machine-supported transitions.
func (p AcceptancePhase) Valid() bool {
	switch p {
	case AcceptancePhaseCheckpoint,
		AcceptancePhaseFeatureFreeze,
		AcceptancePhasePreFramework,
		AcceptancePhasePreRoot,
		AcceptancePhasePostPublication:
		return true
	default:
		return false
	}
}

// EffectivePhase returns checkpoint for a legacy omitted or blank phase.
// New contracts and generated templates still serialize phase explicitly.
func (a AcceptanceCriterion) EffectivePhase() AcceptancePhase {
	phase := AcceptancePhase(strings.TrimSpace(string(a.Phase)))
	if phase == "" {
		return AcceptancePhaseCheckpoint
	}
	return phase
}

// AcceptancePhaseSummary is a phase-local acceptance count. Required evidence
// from another phase is deliberately excluded.
type AcceptancePhaseSummary struct {
	Phase      AcceptancePhase `json:"phase"`
	Acceptance int             `json:"acceptance"`
	Required   int             `json:"required"`
	Levels     map[string]int  `json:"levels,omitempty"`
}

// AcceptanceEvidence points to a command, test, path, report, or manual proof.
type AcceptanceEvidence struct {
	Type  string `yaml:"type" json:"type"`
	Value string `yaml:"value" json:"value"`
}

// FeatureRisk records severity and mitigation for one delivery risk.
type FeatureRisk struct {
	ID          string `yaml:"id" json:"id"`
	Description string `yaml:"description" json:"description"`
	Severity    string `yaml:"severity" json:"severity"`
	Mitigation  string `yaml:"mitigation" json:"mitigation"`
}

// FeatureValidation defines canonical changed/all/custom verification commands.
type FeatureValidation struct {
	Changed string   `yaml:"changed" json:"changed"`
	All     string   `yaml:"all" json:"all"`
	Custom  []string `yaml:"custom,omitempty" json:"custom,omitempty"`
}

// FeatureRollout makes deployment and rollback expectations explicit.
type FeatureRollout struct {
	Strategy  string `yaml:"strategy" json:"strategy"`
	Migration string `yaml:"migration" json:"migration"`
	Rollback  string `yaml:"rollback" json:"rollback"`
}

// LoadFeature reads, normalizes, and validates a Feature specification.
func LoadFeature(path string) (*FeatureSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read feature specification: %w", err)
	}
	feature := &FeatureSpec{}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(feature); err != nil {
		return nil, fmt.Errorf("parse feature specification: %w", err)
	}
	feature.SourcePath = filepath.ToSlash(path)
	feature.Normalize()
	if err := feature.Validate(); err != nil {
		return nil, err
	}
	return feature, nil
}

// Normalize removes insignificant whitespace and makes collection output deterministic.
func (f *FeatureSpec) Normalize() {
	f.Metadata.Name = normalizeIdentifier(f.Metadata.Name)
	f.Metadata.DisplayName = strings.TrimSpace(f.Metadata.DisplayName)
	f.Metadata.Description = strings.TrimSpace(f.Metadata.Description)
	f.Metadata.Owner = strings.TrimSpace(f.Metadata.Owner)
	f.Spec.Problem = strings.TrimSpace(f.Spec.Problem)
	f.Spec.Goals = sortedStrings(f.Spec.Goals)
	f.Spec.NonGoals = sortedStrings(f.Spec.NonGoals)
	f.Spec.Validation.Changed = strings.TrimSpace(f.Spec.Validation.Changed)
	f.Spec.Validation.All = strings.TrimSpace(f.Spec.Validation.All)
	f.Spec.Validation.Custom = sortedStrings(f.Spec.Validation.Custom)
	f.Spec.Rollout.Strategy = strings.TrimSpace(f.Spec.Rollout.Strategy)
	f.Spec.Rollout.Migration = strings.TrimSpace(f.Spec.Rollout.Migration)
	f.Spec.Rollout.Rollback = strings.TrimSpace(f.Spec.Rollout.Rollback)

	for index := range f.Spec.Actors {
		actor := &f.Spec.Actors[index]
		actor.ID = normalizeIdentifier(actor.ID)
		actor.DisplayName = strings.TrimSpace(actor.DisplayName)
		actor.Description = strings.TrimSpace(actor.Description)
	}
	for index := range f.Spec.Modules {
		module := &f.Spec.Modules[index]
		module.Name = normalizeIdentifier(module.Name)
		module.Operation = strings.TrimSpace(module.Operation)
		module.SpecPath = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(module.SpecPath))))
		if module.SpecPath == "." {
			module.SpecPath = ""
		}
		module.Kind = module.EffectiveKind()
		module.Description = strings.TrimSpace(module.Description)
	}
	for index := range f.Spec.Requirements {
		requirement := &f.Spec.Requirements[index]
		requirement.ID = normalizeIdentifier(requirement.ID)
		requirement.Title = strings.TrimSpace(requirement.Title)
		requirement.Description = strings.TrimSpace(requirement.Description)
		requirement.Priority = strings.TrimSpace(requirement.Priority)
		requirement.Actor = normalizeIdentifier(requirement.Actor)
		requirement.Module = normalizeIdentifier(requirement.Module)
		requirement.Permission = strings.TrimSpace(requirement.Permission)
		requirement.Rules = sortedStrings(requirement.Rules)
	}
	for index := range f.Spec.Constraints {
		constraint := &f.Spec.Constraints[index]
		constraint.ID = normalizeIdentifier(constraint.ID)
		constraint.Type = strings.TrimSpace(constraint.Type)
		constraint.Statement = strings.TrimSpace(constraint.Statement)
	}
	for index := range f.Spec.Acceptance {
		acceptance := &f.Spec.Acceptance[index]
		acceptance.ID = normalizeIdentifier(acceptance.ID)
		acceptance.Requirement = normalizeIdentifier(acceptance.Requirement)
		acceptance.Statement = strings.TrimSpace(acceptance.Statement)
		acceptance.Level = strings.TrimSpace(acceptance.Level)
		acceptance.Phase = acceptance.EffectivePhase()
		for evidenceIndex := range acceptance.Evidence {
			evidence := &acceptance.Evidence[evidenceIndex]
			evidence.Type = strings.TrimSpace(evidence.Type)
			evidence.Value = strings.TrimSpace(evidence.Value)
		}
		sort.SliceStable(acceptance.Evidence, func(i, j int) bool {
			if acceptance.Evidence[i].Type == acceptance.Evidence[j].Type {
				return acceptance.Evidence[i].Value < acceptance.Evidence[j].Value
			}
			return acceptance.Evidence[i].Type < acceptance.Evidence[j].Type
		})
	}
	for index := range f.Spec.Risks {
		risk := &f.Spec.Risks[index]
		risk.ID = normalizeIdentifier(risk.ID)
		risk.Description = strings.TrimSpace(risk.Description)
		risk.Severity = strings.TrimSpace(risk.Severity)
		risk.Mitigation = strings.TrimSpace(risk.Mitigation)
	}

	sort.SliceStable(f.Spec.Actors, func(i, j int) bool { return f.Spec.Actors[i].ID < f.Spec.Actors[j].ID })
	sort.SliceStable(f.Spec.Modules, func(i, j int) bool { return f.Spec.Modules[i].Name < f.Spec.Modules[j].Name })
	sort.SliceStable(f.Spec.Requirements, func(i, j int) bool { return f.Spec.Requirements[i].ID < f.Spec.Requirements[j].ID })
	sort.SliceStable(f.Spec.Constraints, func(i, j int) bool { return f.Spec.Constraints[i].ID < f.Spec.Constraints[j].ID })
	sort.SliceStable(f.Spec.Acceptance, func(i, j int) bool { return f.Spec.Acceptance[i].ID < f.Spec.Acceptance[j].ID })
	sort.SliceStable(f.Spec.Risks, func(i, j int) bool { return f.Spec.Risks[i].ID < f.Spec.Risks[j].ID })
}

// Validate checks identity, references, evidence, security boundaries, and rollout completeness.
func (f *FeatureSpec) Validate() error {
	var problems []string
	if f.APIVersion != "mss.io/v1alpha1" {
		problems = append(problems, "apiVersion must equal mss.io/v1alpha1")
	}
	if f.Kind != "Feature" {
		problems = append(problems, "kind must equal Feature")
	}
	if !identifierPattern.MatchString(f.Metadata.Name) {
		problems = append(problems, "metadata.name must be lower-case kebab-case")
	}
	if f.Metadata.DisplayName == "" {
		problems = append(problems, "metadata.displayName is required")
	}
	if f.Metadata.Owner == "" {
		problems = append(problems, "metadata.owner is required")
	}
	if len(f.Spec.Problem) < 20 {
		problems = append(problems, "spec.problem must describe the user or engineering problem")
	}
	if len(f.Spec.Goals) == 0 {
		problems = append(problems, "spec.goals must contain at least one goal")
	}
	if f.Spec.NonGoals == nil {
		problems = append(problems, "spec.nonGoals must be explicit, even when empty")
	}
	if len(f.Spec.Actors) == 0 {
		problems = append(problems, "spec.actors must contain at least one actor")
	}
	if len(f.Spec.Modules) == 0 {
		problems = append(problems, "spec.modules must contain at least one module")
	}
	if len(f.Spec.Requirements) == 0 {
		problems = append(problems, "spec.requirements must contain at least one requirement")
	}
	if len(f.Spec.Constraints) == 0 {
		problems = append(problems, "spec.constraints must contain at least one constraint")
	}
	if len(f.Spec.Acceptance) == 0 {
		problems = append(problems, "spec.acceptance must contain at least one criterion")
	}

	actors := make(map[string]bool, len(f.Spec.Actors))
	modules := make(map[string]bool, len(f.Spec.Modules))
	requirements := make(map[string]bool, len(f.Spec.Requirements))
	allIDs := make(map[string]string)
	registerID := func(kind, id string) {
		if !identifierPattern.MatchString(id) {
			problems = append(problems, fmt.Sprintf("%s id %q must be lower-case kebab-case", kind, id))
			return
		}
		if previous, exists := allIDs[id]; exists {
			problems = append(problems, fmt.Sprintf("id %q is shared by %s and %s", id, previous, kind))
			return
		}
		allIDs[id] = kind
	}

	for index, actor := range f.Spec.Actors {
		registerID(fmt.Sprintf("actors[%d]", index), actor.ID)
		if actor.DisplayName == "" {
			problems = append(problems, fmt.Sprintf("actors[%d].displayName is required", index))
		}
		actors[actor.ID] = true
	}
	allowedModuleOperations := map[string]bool{"create": true, "extend": true, "deprecate": true, "remove": true}
	for index, module := range f.Spec.Modules {
		registerID(fmt.Sprintf("modules[%d]", index), module.Name)
		kind := module.EffectiveKind()
		if !kind.Valid() {
			problems = append(problems, fmt.Sprintf("modules[%d].kind %q is unsupported", index, kind))
		}
		if !allowedModuleOperations[module.Operation] {
			problems = append(problems, fmt.Sprintf("modules[%d].operation %q is unsupported", index, module.Operation))
		}
		if module.SpecPath != "" {
			if filepath.IsAbs(module.SpecPath) || module.SpecPath == ".." || strings.HasPrefix(module.SpecPath, "../") {
				problems = append(problems, fmt.Sprintf("modules[%d].specPath must be repository-relative", index))
			}
			if module.Operation == "create" && !strings.HasSuffix(module.SpecPath, ".yaml") && !strings.HasSuffix(module.SpecPath, ".yml") {
				problems = append(problems, fmt.Sprintf("modules[%d].specPath must reference YAML", index))
			}
		}
		modules[module.Name] = true
	}
	allowedPriorities := map[string]bool{"must": true, "should": true, "could": true}
	for index, requirement := range f.Spec.Requirements {
		registerID(fmt.Sprintf("requirements[%d]", index), requirement.ID)
		requirements[requirement.ID] = true
		if requirement.Title == "" || len(requirement.Description) < 10 {
			problems = append(problems, fmt.Sprintf("requirements[%d] needs a title and descriptive statement", index))
		}
		if !allowedPriorities[requirement.Priority] {
			problems = append(problems, fmt.Sprintf("requirements[%d].priority %q is unsupported", index, requirement.Priority))
		}
		if !actors[requirement.Actor] {
			problems = append(problems, fmt.Sprintf("requirements[%d].actor %q is not declared", index, requirement.Actor))
		}
		if !modules[requirement.Module] {
			problems = append(problems, fmt.Sprintf("requirements[%d].module %q is not declared", index, requirement.Module))
		}
		if requirement.Permission != "" {
			parts := strings.Split(requirement.Permission, ":")
			if len(parts) != 2 || !identifierPattern.MatchString(parts[0]) || !identifierPattern.MatchString(parts[1]) {
				problems = append(problems, fmt.Sprintf("requirements[%d].permission must use resource:action form", index))
			}
		}
		if len(requirement.Rules) == 0 {
			problems = append(problems, fmt.Sprintf("requirements[%d].rules must not be empty", index))
		}
	}

	allowedConstraintTypes := map[string]bool{
		"compatibility": true, "security": true, "privacy": true, "performance": true,
		"data": true, "operations": true, "ux": true, "dependency": true,
	}
	for index, constraint := range f.Spec.Constraints {
		registerID(fmt.Sprintf("constraints[%d]", index), constraint.ID)
		if !allowedConstraintTypes[constraint.Type] {
			problems = append(problems, fmt.Sprintf("constraints[%d].type %q is unsupported", index, constraint.Type))
		}
		if len(constraint.Statement) < 10 {
			problems = append(problems, fmt.Sprintf("constraints[%d].statement is too short", index))
		}
	}

	allowedLevels := map[string]bool{"unit": true, "contract": true, "integration": true, "e2e": true, "security": true, "migration": true, "manual": true}
	allowedEvidence := map[string]bool{"command": true, "test": true, "path": true, "report": true, "manual": true}
	acceptedRequirements := make(map[string]bool)
	for index, acceptance := range f.Spec.Acceptance {
		registerID(fmt.Sprintf("acceptance[%d]", index), acceptance.ID)
		if acceptance.Requirement != "" {
			if !requirements[acceptance.Requirement] {
				problems = append(problems, fmt.Sprintf("acceptance[%d].requirement %q is not declared", index, acceptance.Requirement))
			} else {
				acceptedRequirements[acceptance.Requirement] = true
			}
		}
		if len(acceptance.Statement) < 10 {
			problems = append(problems, fmt.Sprintf("acceptance[%d].statement is too short", index))
		}
		if !allowedLevels[acceptance.Level] {
			problems = append(problems, fmt.Sprintf("acceptance[%d].level %q is unsupported", index, acceptance.Level))
		}
		phase := acceptance.EffectivePhase()
		if !phase.Valid() {
			problems = append(problems, fmt.Sprintf("acceptance[%d].phase %q is unsupported", index, phase))
		}
		if len(acceptance.Evidence) == 0 {
			problems = append(problems, fmt.Sprintf("acceptance[%d].evidence must not be empty", index))
		}
		for evidenceIndex, evidence := range acceptance.Evidence {
			if !allowedEvidence[evidence.Type] {
				problems = append(problems, fmt.Sprintf("acceptance[%d].evidence[%d].type %q is unsupported", index, evidenceIndex, evidence.Type))
			}
			if evidence.Value == "" {
				problems = append(problems, fmt.Sprintf("acceptance[%d].evidence[%d].value is required", index, evidenceIndex))
			}
		}
	}
	for _, requirement := range f.Spec.Requirements {
		if requirement.Priority == "must" && !acceptedRequirements[requirement.ID] {
			problems = append(problems, fmt.Sprintf("must requirement %q needs at least one linked acceptance criterion", requirement.ID))
		}
	}

	allowedSeverities := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
	for index, risk := range f.Spec.Risks {
		registerID(fmt.Sprintf("risks[%d]", index), risk.ID)
		if !allowedSeverities[risk.Severity] {
			problems = append(problems, fmt.Sprintf("risks[%d].severity %q is unsupported", index, risk.Severity))
		}
		if len(risk.Description) < 10 || len(risk.Mitigation) < 10 {
			problems = append(problems, fmt.Sprintf("risks[%d] needs a description and mitigation", index))
		}
	}
	if f.Spec.Validation.Changed == "" || f.Spec.Validation.All == "" {
		problems = append(problems, "spec.validation.changed and spec.validation.all are required")
	}
	allowedStrategies := map[string]bool{"direct": true, "phased": true, "feature-flag": true, "shadow": true}
	if !allowedStrategies[f.Spec.Rollout.Strategy] {
		problems = append(problems, "spec.rollout.strategy is unsupported")
	}
	if len(f.Spec.Rollout.Migration) < 5 || len(f.Spec.Rollout.Rollback) < 5 {
		problems = append(problems, "spec.rollout.migration and spec.rollout.rollback are required")
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return validationError{Problems: problems}
	}
	return nil
}

// Summary returns a compact validation result for CLI, MCP, and Evals.
func (f *FeatureSpec) Summary() map[string]any {
	levels := make(map[string]int)
	required := 0
	for _, acceptance := range f.Spec.Acceptance {
		levels[acceptance.Level]++
		if acceptance.Required {
			required++
		}
	}
	priorities := make(map[string]int)
	for _, requirement := range f.Spec.Requirements {
		priorities[requirement.Priority]++
	}
	severities := make(map[string]int)
	for _, risk := range f.Spec.Risks {
		severities[risk.Severity]++
	}
	return map[string]any{
		"name":                  f.Metadata.Name,
		"displayName":           f.Metadata.DisplayName,
		"owner":                 f.Metadata.Owner,
		"actors":                len(f.Spec.Actors),
		"modules":               len(f.Spec.Modules),
		"requirements":          len(f.Spec.Requirements),
		"requirementPriorities": priorities,
		"constraints":           len(f.Spec.Constraints),
		"acceptance":            len(f.Spec.Acceptance),
		"requiredAcceptance":    required,
		"acceptanceLevels":      levels,
		"acceptancePhases":      f.AcceptancePhaseSummaries(),
		"risks":                 len(f.Spec.Risks),
		"riskSeverities":        severities,
		"rolloutStrategy":       f.Spec.Rollout.Strategy,
	}
}

// AcceptancePhaseSummaries returns deterministic, phase-local counts for CLI,
// plan, report, and release consumers. Empty phases remain explicit so callers
// never infer a default or accidentally include a later transition.
func (f *FeatureSpec) AcceptancePhaseSummaries() []AcceptancePhaseSummary {
	summaries := make([]AcceptancePhaseSummary, 0, len(orderedAcceptancePhases))
	byPhase := make(map[AcceptancePhase]*AcceptancePhaseSummary, len(orderedAcceptancePhases))
	for _, phase := range orderedAcceptancePhases {
		summary := AcceptancePhaseSummary{
			Phase:  phase,
			Levels: make(map[string]int),
		}
		summaries = append(summaries, summary)
		byPhase[phase] = &summaries[len(summaries)-1]
	}
	for _, acceptance := range f.Spec.Acceptance {
		summary := byPhase[acceptance.EffectivePhase()]
		if summary == nil {
			continue
		}
		summary.Acceptance++
		summary.Levels[acceptance.Level]++
		if acceptance.Required {
			summary.Required++
		}
	}
	return summaries
}

// AcceptanceForPhase returns only criteria assigned to phase. It does not
// include earlier or later phases; transition orchestration composes phases
// explicitly instead of relying on hidden cumulative behavior.
func (f *FeatureSpec) AcceptanceForPhase(phase AcceptancePhase) []AcceptanceCriterion {
	criteria := make([]AcceptanceCriterion, 0)
	for _, acceptance := range f.Spec.Acceptance {
		if acceptance.EffectivePhase() == phase {
			criteria = append(criteria, acceptance)
		}
	}
	return criteria
}
