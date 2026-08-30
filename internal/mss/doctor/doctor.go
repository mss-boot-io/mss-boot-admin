package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/blueprint"
	devcmd "github.com/mss-boot-io/mss-boot-admin/internal/mss/dev"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/verify"
)

// Status is the outcome of one environment check.
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
	StatusInfo Status = "info"
)

// Component identifies one independently validated monorepo component.
type Component string

const (
	ComponentAll       Component = "all"
	ComponentBackend   Component = "backend"
	ComponentFramework Component = "framework"
	ComponentFrontend  Component = "frontend"
	ComponentDocs      Component = "docs"
	ComponentAgent     Component = "agent"
)

var componentOrder = []Component{
	ComponentBackend,
	ComponentFramework,
	ComponentFrontend,
	ComponentDocs,
	ComponentAgent,
}

// Options configures the readiness surface checked by Run.
type Options struct {
	Components []Component
}

// Option mutates doctor Options.
type Option func(*Options)

// WithComponents limits checks to the supplied monorepo components.
func WithComponents(components ...Component) Option {
	return func(options *Options) {
		options.Components = append([]Component(nil), components...)
	}
}

// ParseComponents validates CLI component names and returns stable, deduplicated values.
func ParseComponents(values []string) ([]Component, error) {
	if len(values) == 0 {
		return nil, nil
	}
	selected := make(map[Component]bool, len(values))
	for _, value := range values {
		component := Component(strings.ToLower(strings.TrimSpace(value)))
		switch component {
		case ComponentAll:
			return []Component{ComponentAll}, nil
		case ComponentBackend, ComponentFramework, ComponentFrontend, ComponentDocs, ComponentAgent:
			selected[component] = true
		default:
			return nil, fmt.Errorf("unsupported doctor component %q", value)
		}
	}
	components := make([]Component, 0, len(selected))
	for _, component := range componentOrder {
		if selected[component] {
			components = append(components, component)
		}
	}
	return components, nil
}

// Check records one deterministic doctor result.
type Check struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Status      Status                    `json:"status"`
	Required    bool                      `json:"required"`
	Detail      string                    `json:"detail,omitempty"`
	Remediation string                    `json:"remediation,omitempty"`
	Snapshot    *blueprint.SnapshotStatus `json:"snapshot,omitempty"`
}

// Report is emitted in text or JSON form for humans and agents.
type Report struct {
	Project     string      `json:"project"`
	Root        string      `json:"root"`
	GeneratedAt time.Time   `json:"generatedAt"`
	Platform    string      `json:"platform"`
	Components  []Component `json:"components"`
	Ready       bool        `json:"ready"`
	Checks      []Check     `json:"checks"`
}

// Run executes environment and repository checks without mutating the workspace.
// Without options it preserves the historical full-repository readiness check.
func Run(ctx context.Context, projectContext *project.Context, options ...Option) Report {
	configured := Options{Components: []Component{ComponentAll}}
	for _, option := range options {
		if option != nil {
			option(&configured)
		}
	}
	components := normalizeComponents(configured.Components)
	selected := componentSet(components)

	report := Report{
		Project:     projectContext.Project.Metadata.Name,
		Root:        projectContext.Root,
		GeneratedAt: time.Now().UTC(),
		Platform:    runtime.GOOS + "/" + runtime.GOARCH,
		Components:  components,
		Ready:       true,
	}

	if selected(ComponentAgent) {
		report.Checks = append(report.Checks,
			fileCheck(projectContext.Root, ".mss/project.yaml", true),
			fileCheck(projectContext.Root, ".mss/capabilities.yaml", true),
			fileCheck(projectContext.Root, ".mss/commands.yaml", true),
			foundationSnapshotCheck(projectContext),
		)
		if projectContext.LayoutKind() == "thin-host" {
			report.Checks = append(report.Checks,
				fileCheck(projectContext.Root, ".mss/lock.yaml", true),
				fileCheck(projectContext.Root, ".mss/blueprint-manifest.json", true),
				fileCheck(projectContext.Root, ".mss/dev.yaml", true),
				fileCheck(projectContext.Root, "README.md", true),
				developmentConfigCheck(projectContext),
				thinHostContractCheck(projectContext),
			)
		}
	}
	if selected(ComponentBackend) {
		if projectContext.LayoutKind() == "thin-host" {
			backendPath := strings.TrimSpace(projectContext.Project.Spec.RepositoryLayout["backend"])
			modulesPath := strings.TrimSpace(projectContext.Project.Spec.RepositoryLayout["modules"])
			report.Checks = append(report.Checks,
				fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(backendPath, "go.mod")), true),
				fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(backendPath, "go.sum")), true),
				fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(backendPath, "cmd", "server", "main.go")), true),
				fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(modulesPath, "all", "generated.go")), true),
			)
		} else {
			report.Checks = append(report.Checks,
				fileCheck(projectContext.Root, "admin/go.mod", true),
				fileCheck(projectContext.Root, "admin/go.sum", true),
				fileCheck(projectContext.Root, "go.work", true),
			)
		}
	}
	if selected(ComponentFramework) {
		if projectContext.LayoutKind() != "thin-host" {
			report.Checks = append(report.Checks,
				fileCheck(projectContext.Root, "mss-boot/go.mod", true),
				fileCheck(projectContext.Root, "mss-boot/go.sum", true),
			)
		}
	}
	if selected(ComponentFrontend) {
		if projectContext.LayoutKind() == "thin-host" {
			frontendPath := strings.TrimSpace(projectContext.Project.Spec.RepositoryLayout["frontend"])
			report.Checks = append(report.Checks,
				fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(frontendPath, "package.json")), true),
				fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(frontendPath, "pnpm-lock.yaml")), true),
				fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(frontendPath, "tsconfig.json")), true),
				fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(frontendPath, "config", "config.ts")), true),
				fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(frontendPath, "mss-admin.config.ts")), true),
				fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(frontendPath, "src", "app.tsx")), true),
				fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(frontendPath, "src", "access.ts")), true),
			)
		} else {
			applications := projectContext.Project.Spec.Frontend.Applications
			if len(applications) == 0 {
				frontendPath := strings.TrimSpace(projectContext.Project.Spec.RepositoryLayout["frontend"])
				if frontendPath == "" {
					frontendPath = "web/antd-v6"
				}
				report.Checks = append(report.Checks,
					fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(frontendPath, "pnpm-lock.yaml")), true),
				)
			} else {
				for _, application := range applications {
					report.Checks = append(report.Checks,
						fileCheck(projectContext.Root, filepath.ToSlash(filepath.Join(application.Path, "pnpm-lock.yaml")), true),
					)
				}
			}
		}
	}
	if selected(ComponentDocs) && projectContext.LayoutKind() != "thin-host" {
		report.Checks = append(report.Checks,
			fileCheck(projectContext.Root, "docs/pnpm-lock.yaml", true),
		)
	}
	if selected(ComponentAgent) || selected(ComponentBackend) || selected(ComponentFramework) || selected(ComponentFrontend) {
		report.Checks = append(report.Checks, adminDistributionCheck(projectContext))
	}

	report.Checks = append(report.Checks, toolCheck(ctx, "git", true, "git", "--version"))
	if selected(ComponentBackend) || selected(ComponentFramework) || selected(ComponentAgent) {
		minimumGoVersion := strings.TrimSpace(projectContext.Project.Spec.Backend.GoVersion)
		if minimumGoVersion != "" {
			minimumGoVersion = ">=" + minimumGoVersion
		}
		report.Checks = append(report.Checks, toolVersionCheck(
			ctx,
			"go",
			true,
			minimumGoVersion,
			"go",
			"version",
		))
	}
	if selected(ComponentFrontend) || selected(ComponentDocs) {
		report.Checks = append(report.Checks,
			toolVersionCheck(ctx, "node", true, projectContext.Project.Spec.Frontend.NodeVersion, "node", "--version"),
		)
		if projectContext.LayoutKind() == "thin-host" {
			report.Checks = append(report.Checks, toolCheck(ctx, "corepack", true, "corepack", "--version"))
		}
		report.Checks = append(report.Checks, packageManagerToolCheck(ctx, projectContext))
	}
	if selected(ComponentBackend) {
		report.Checks = append(report.Checks, toolCheck(ctx, "docker", false, "docker", "--version"))
	}

	if selected(ComponentBackend) {
		report.Checks = append(report.Checks,
			portCheck("backend-port", 8080),
			portCheck("redis-port", 6379),
		)
	}
	if selected(ComponentFrontend) {
		applications := projectContext.Project.Spec.Frontend.Applications
		if len(applications) == 0 {
			report.Checks = append(report.Checks, portCheck("frontend-port", 8001))
		} else {
			defaultApplication, hasDefault := projectContext.DefaultFrontendApplication()
			for _, application := range applications {
				id := application.ID + "-port"
				if hasDefault && application.ID == defaultApplication.ID {
					id = "frontend-port"
				}
				report.Checks = append(report.Checks, portCheck(id, application.DevelopmentPort))
			}
		}
	}

	for _, check := range report.Checks {
		if check.Required && check.Status == StatusFail {
			report.Ready = false
			break
		}
	}

	sort.SliceStable(report.Checks, func(i, j int) bool {
		return report.Checks[i].ID < report.Checks[j].ID
	})
	return report
}

func developmentConfigCheck(projectContext *project.Context) Check {
	check := Check{
		ID:          "contract:development-topology",
		Name:        "Thin Host development topology",
		Required:    true,
		Remediation: "restore .mss/dev.yaml from the matching Distribution Blueprint",
	}
	config, err := devcmd.Load(projectContext.Root)
	if err != nil {
		check.Status = StatusFail
		check.Detail = err.Error()
		return check
	}
	if config.Document.Metadata.Project != projectContext.Project.Metadata.Name {
		check.Status = StatusFail
		check.Detail = fmt.Sprintf("development project %q does not match project contract %q", config.Document.Metadata.Project, projectContext.Project.Metadata.Name)
		return check
	}
	ids := config.ServiceIDs()
	if len(ids) == 0 {
		check.Status = StatusFail
		check.Detail = "development topology has no services"
		return check
	}
	if application, ok := projectContext.DefaultFrontendApplication(); ok {
		expectedVersion := strings.TrimSpace(application.PackageManagerVersion)
		if expectedVersion == "" {
			expectedVersion = strings.TrimSpace(projectContext.Project.Spec.Frontend.PackageManagerVersion)
		}
		if declared := strings.TrimSpace(projectContext.Project.Spec.Frontend.PackageManagerVersion); declared != "" && expectedVersion != declared {
			check.Status = StatusFail
			check.Detail = fmt.Sprintf("frontend application package manager version %q does not match project version %q", expectedVersion, declared)
			return check
		}
		if expectedVersion != "" {
			service, exists := config.Service(application.ID)
			expectedCommand := "pnpm@" + expectedVersion
			if !exists || len(service.Command) < 2 || service.Command[0] != "corepack" || service.Command[1] != expectedCommand {
				check.Status = StatusFail
				check.Detail = fmt.Sprintf("development service %s must invoke corepack %s", application.ID, expectedCommand)
				return check
			}
		}
	}
	check.Status = StatusPass
	check.Detail = "services: " + strings.Join(ids, ", ")
	return check
}

func thinHostContractCheck(projectContext *project.Context) Check {
	check := Check{
		ID:          "contract:thin-host",
		Name:        "Thin Host frozen package contract",
		Required:    true,
		Remediation: "restore managed Thin Host files from the matching Distribution Blueprint and run mss setup",
	}
	if err := verify.ValidateThinHostStructure(projectContext); err != nil {
		check.Status = StatusFail
		check.Detail = err.Error()
		return check
	}
	check.Status = StatusPass
	check.Detail = "managed layout, exact Go sums, frozen pnpm lock, and package pins are aligned"
	return check
}

func foundationSnapshotCheck(projectContext *project.Context) Check {
	check := Check{
		ID:       "snapshot:foundation",
		Name:     "Foundation snapshot",
		Required: true,
	}
	inspection, err := blueprint.InspectSnapshot(
		projectContext.Root,
		".mss/blueprint-manifest.json",
		projectContext.Project.Metadata.Name,
		projectContext.Project.Metadata.Repository,
	)
	if err != nil {
		check.Status = StatusFail
		check.Detail = err.Error()
		check.Remediation = "restore both generated snapshot records or the strict Foundation source development lock"
		return check
	}

	switch inspection.Role {
	case blueprint.SnapshotRoleFoundationSource:
		if inspection.Source == nil {
			check.Status = StatusFail
			check.Detail = "Foundation source inspection omitted its identity"
			return check
		}
		check.Status = StatusInfo
		check.Required = false
		check.Detail = fmt.Sprintf(
			"Foundation source sentinel: %s@%s, Blueprint %s@%s, generator mss@%s",
			inspection.Source.FoundationRepository,
			inspection.Source.FoundationVersion,
			inspection.Source.Blueprint,
			inspection.Source.BlueprintVersion,
			inspection.Source.GeneratorVersion,
		)
	case blueprint.SnapshotRoleGenerated:
		if inspection.Status == nil {
			check.Status = StatusFail
			check.Detail = "generated snapshot inspection omitted its verified status"
			return check
		}
		check.Status = StatusPass
		check.Required = true
		check.Snapshot = inspection.Status
		check.Detail = fmt.Sprintf(
			"Foundation %s@%s, Blueprint %s@%s, generator %s@%s, snapshot %s",
			inspection.Status.Identities.Foundation.Repository,
			inspection.Status.Identities.Foundation.Version,
			inspection.Status.Identities.Blueprint.Name,
			inspection.Status.Identities.Blueprint.Version,
			inspection.Status.Identities.Generator.Tool,
			inspection.Status.Identities.Generator.Version,
			inspection.Status.Identities.Snapshot.SHA256,
		)
	default:
		check.Status = StatusFail
		check.Detail = "snapshot inspection returned an unsupported repository role"
		check.Remediation = "restore a supported Foundation source or generated downstream snapshot"
	}
	return check
}

func adminDistributionCheck(projectContext *project.Context) Check {
	distribution := projectContext.Project.Spec.Distribution
	legacyContract := distribution.Empty() && strings.TrimSpace(projectContext.Project.Spec.RepositoryLayout["kind"]) == ""
	check := Check{
		ID:          "contract:admin-distribution",
		Name:        "Admin Distribution",
		Required:    true,
		Remediation: "align project spec.distribution with the Foundation lock and manifest, then regenerate or upgrade the Thin Host",
	}
	if !distribution.Empty() {
		if problems := distribution.Validate(); len(problems) > 0 {
			check.Status = StatusFail
			check.Detail = strings.Join(problems, "; ")
			return check
		}
	} else if !legacyContract {
		check.Status = StatusFail
		check.Detail = "explicit project layout must pin a coordinated Admin Distribution"
		return check
	}

	inspection, err := blueprint.InspectSnapshot(
		projectContext.Root,
		".mss/blueprint-manifest.json",
		projectContext.Project.Metadata.Name,
		projectContext.Project.Metadata.Repository,
	)
	if err != nil {
		if legacyContract {
			check.Status = StatusInfo
			check.Required = false
			check.Detail = "legacy project contract does not pin a coordinated Admin Distribution"
			return check
		}
		check.Status = StatusFail
		check.Detail = "cannot verify Distribution snapshot: " + err.Error()
		return check
	}
	var recorded project.DistributionSpec
	switch inspection.Role {
	case blueprint.SnapshotRoleFoundationSource:
		if inspection.Source == nil {
			check.Status = StatusFail
			check.Detail = "Foundation source snapshot omitted its Distribution"
			return check
		}
		recorded = inspection.Source.Distribution
	case blueprint.SnapshotRoleGenerated:
		if inspection.Status == nil {
			check.Status = StatusFail
			check.Detail = "generated snapshot omitted its Distribution"
			return check
		}
		recorded = inspection.Status.Distribution
	default:
		check.Status = StatusFail
		check.Detail = "snapshot inspection returned an unsupported repository role"
		return check
	}
	if recorded != distribution {
		check.Status = StatusFail
		check.Detail = fmt.Sprintf(
			"project pins %s but snapshot pins %s",
			distributionSummary(distribution),
			distributionSummary(recorded),
		)
		return check
	}
	if legacyContract {
		check.Status = StatusInfo
		check.Required = false
		check.Detail = "legacy project contract and snapshot do not pin a coordinated Admin Distribution"
		return check
	}
	check.Status = StatusPass
	check.Detail = distributionSummary(distribution) + "; backend and frontend versions exactly match; snapshot aligned"
	return check
}

func distributionSummary(distribution project.DistributionSpec) string {
	if distribution.Empty() {
		return "no Admin Distribution"
	}
	return fmt.Sprintf(
		"%s@%s (backend %s@%s, frontend %s@%s)",
		distribution.Name,
		distribution.Version,
		distribution.Backend.Module,
		distribution.Backend.Version,
		distribution.Frontend.Package,
		distribution.Frontend.Version,
	)
}

// JSON returns stable indented JSON.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// Text returns a compact human-readable report.
func (r Report) Text() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "mss doctor: %s\n", r.Project)
	fmt.Fprintf(&builder, "root: %s\n", r.Root)
	fmt.Fprintf(&builder, "platform: %s\n", r.Platform)
	fmt.Fprintf(&builder, "components: %s\n", joinComponents(r.Components))
	fmt.Fprintf(&builder, "ready: %t\n\n", r.Ready)
	for _, check := range r.Checks {
		required := "optional"
		if check.Required {
			required = "required"
		}
		fmt.Fprintf(&builder, "[%s] %-24s (%s)", strings.ToUpper(string(check.Status)), check.Name, required)
		if check.Detail != "" {
			fmt.Fprintf(&builder, ": %s", check.Detail)
		}
		builder.WriteByte('\n')
		if check.Remediation != "" && check.Status != StatusPass {
			fmt.Fprintf(&builder, "       remediation: %s\n", check.Remediation)
		}
	}
	return builder.String()
}

func normalizeComponents(components []Component) []Component {
	if len(components) == 0 {
		return []Component{ComponentAll}
	}
	selected := make(map[Component]bool, len(components))
	for _, component := range components {
		if component == ComponentAll {
			return []Component{ComponentAll}
		}
		selected[component] = true
	}
	normalized := make([]Component, 0, len(selected))
	for _, component := range componentOrder {
		if selected[component] {
			normalized = append(normalized, component)
		}
	}
	if len(normalized) == 0 {
		return []Component{ComponentAll}
	}
	return normalized
}

func componentSet(components []Component) func(Component) bool {
	all := len(components) == 1 && components[0] == ComponentAll
	selected := make(map[Component]bool, len(components))
	for _, component := range components {
		selected[component] = true
	}
	return func(component Component) bool {
		return all || selected[component]
	}
}

func joinComponents(components []Component) string {
	values := make([]string, len(components))
	for i, component := range components {
		values[i] = string(component)
	}
	return strings.Join(values, ",")
}

func fileCheck(root, relative string, required bool) Check {
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Stat(path)
	check := Check{
		ID:       "file:" + relative,
		Name:     relative,
		Required: required,
	}
	if err != nil {
		check.Status = StatusFail
		check.Detail = err.Error()
		check.Remediation = "restore the tracked repository file"
		return check
	}
	if !info.Mode().IsRegular() {
		check.Status = StatusFail
		check.Detail = "path exists but is not a regular file"
		check.Remediation = "replace the path with the expected tracked file"
		return check
	}
	check.Status = StatusPass
	check.Detail = fmt.Sprintf("%d bytes", info.Size())
	return check
}

func toolCheck(parent context.Context, id string, required bool, executable string, args ...string) Check {
	check := Check{
		ID:       "tool:" + id,
		Name:     executable,
		Required: required,
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		if required {
			check.Status = StatusFail
		} else {
			check.Status = StatusWarn
		}
		check.Detail = "not found in PATH"
		check.Remediation = "install " + executable + " or add it to PATH"
		return check
	}

	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, args...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		check.Status = StatusFail
		check.Detail = "version check timed out"
		check.Remediation = "verify the executable can start without interaction"
		return check
	}
	if err != nil {
		if required {
			check.Status = StatusFail
		} else {
			check.Status = StatusWarn
		}
		check.Detail = strings.TrimSpace(string(output))
		if check.Detail == "" {
			check.Detail = err.Error()
		}
		check.Remediation = "repair or reinstall " + executable
		return check
	}

	check.Status = StatusPass
	check.Detail = firstLine(strings.TrimSpace(string(output)))
	if check.Detail == "" {
		check.Detail = path
	}
	return check
}

var semanticVersionPattern = regexp.MustCompile(`(?:^|[^0-9])v?(\d+)(?:\.(\d+))?(?:\.(\d+))?`)

type semanticVersion struct {
	major int
	minor int
	patch int
}

func toolVersionCheck(parent context.Context, id string, required bool, constraint string, executable string, args ...string) Check {
	check := toolCheck(parent, id, required, executable, args...)
	constraint = strings.TrimSpace(constraint)
	if check.Status != StatusPass || constraint == "" {
		return check
	}
	version, ok := parseSemanticVersion(check.Detail)
	if !ok {
		check.Status = StatusFail
		check.Detail = fmt.Sprintf("cannot parse version from %q; project requires %s", check.Detail, constraint)
		check.Remediation = "install the project-declared " + id + " version"
		return check
	}
	if !satisfiesVersionConstraint(version, constraint) {
		check.Status = StatusFail
		check.Detail = fmt.Sprintf("found %s but project requires %s", formatSemanticVersion(version), constraint)
		check.Remediation = "install the project-declared " + id + " version"
		return check
	}
	check.Detail = fmt.Sprintf("%s (required %s)", check.Detail, constraint)
	return check
}

func packageManagerToolCheck(parent context.Context, projectContext *project.Context) Check {
	manager := strings.TrimSpace(projectContext.Project.Spec.Frontend.PackageManager)
	if manager == "" {
		manager = "pnpm"
	}
	version := strings.TrimSpace(projectContext.Project.Spec.Frontend.PackageManagerVersion)
	if projectContext.LayoutKind() != "thin-host" {
		return toolVersionCheck(parent, manager, true, version, manager, "--version")
	}
	pinnedManager := manager
	if version != "" {
		pinnedManager += "@" + version
	}
	return toolVersionCheck(parent, manager, true, version, "corepack", pinnedManager, "--version")
}

func parseSemanticVersion(value string) (semanticVersion, bool) {
	match := semanticVersionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) != 4 {
		return semanticVersion{}, false
	}
	parts := [3]int{}
	for index := range parts {
		if match[index+1] == "" {
			continue
		}
		parsed, err := strconv.Atoi(match[index+1])
		if err != nil {
			return semanticVersion{}, false
		}
		parts[index] = parsed
	}
	return semanticVersion{major: parts[0], minor: parts[1], patch: parts[2]}, true
}

func satisfiesVersionConstraint(actual semanticVersion, constraint string) bool {
	terms := strings.Fields(constraint)
	if len(terms) == 0 {
		return true
	}
	for _, term := range terms {
		operator := "="
		value := term
		for _, candidate := range []string{">=", "<=", ">", "<", "="} {
			if strings.HasPrefix(term, candidate) {
				operator = candidate
				value = strings.TrimPrefix(term, candidate)
				break
			}
		}
		required, ok := parseSemanticVersion(value)
		if !ok {
			return false
		}
		comparison := compareSemanticVersions(actual, required)
		switch operator {
		case "=":
			if comparison != 0 {
				return false
			}
		case ">=":
			if comparison < 0 {
				return false
			}
		case "<=":
			if comparison > 0 {
				return false
			}
		case ">":
			if comparison <= 0 {
				return false
			}
		case "<":
			if comparison >= 0 {
				return false
			}
		}
	}
	return true
}

func compareSemanticVersions(left, right semanticVersion) int {
	leftParts := [...]int{left.major, left.minor, left.patch}
	rightParts := [...]int{right.major, right.minor, right.patch}
	for index := range leftParts {
		if leftParts[index] < rightParts[index] {
			return -1
		}
		if leftParts[index] > rightParts[index] {
			return 1
		}
	}
	return 0
}

func formatSemanticVersion(version semanticVersion) string {
	return fmt.Sprintf("%d.%d.%d", version.major, version.minor, version.patch)
}

func portCheck(id string, port int) Check {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return Check{
			ID:       "port:" + id,
			Name:     address,
			Status:   StatusInfo,
			Required: false,
			Detail:   "already in use or unavailable: " + err.Error(),
		}
	}
	_ = listener.Close()
	return Check{
		ID:       "port:" + id,
		Name:     address,
		Status:   StatusInfo,
		Required: false,
		Detail:   "available",
	}
}

func firstLine(value string) string {
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return strings.TrimSpace(value)
}
