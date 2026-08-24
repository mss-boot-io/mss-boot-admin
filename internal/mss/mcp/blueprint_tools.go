package mcp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/blueprint"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func (s *Server) callBlueprintTool(ctx context.Context, name string, arguments map[string]any) (callToolResult, bool) {
	var value any
	var err error
	switch name {
	case "mss_plan_application":
		value, err = s.planApplication(ctx, arguments)
	case "mss_get_blueprint_status":
		value, err = s.blueprintStatus(arguments)
	case "mss_plan_foundation_upgrade":
		value, err = s.foundationUpgrade(ctx, arguments, false)
	case "mss_apply_foundation_upgrade":
		confirmed, confirmErr := optionalBool(arguments, "confirm", false)
		if confirmErr != nil {
			err = confirmErr
		} else if !confirmed {
			err = errors.New("confirm=true is required to apply a foundation upgrade")
		} else {
			value, err = s.foundationUpgrade(ctx, arguments, true)
		}
	default:
		return callToolResult{}, false
	}
	if err != nil {
		return toolError(err), true
	}
	return toolSuccess(value), true
}

func (s *Server) planApplication(ctx context.Context, arguments map[string]any) (any, error) {
	name, err := requiredString(arguments, "name")
	if err != nil {
		return nil, err
	}
	module, err := requiredString(arguments, "module")
	if err != nil {
		return nil, err
	}
	displayName, err := optionalString(arguments, "displayName")
	if err != nil {
		return nil, err
	}
	repository, err := optionalString(arguments, "repository")
	if err != nil {
		return nil, err
	}
	blueprintName, err := optionalString(arguments, "blueprint")
	if err != nil {
		return nil, err
	}
	destination, err := optionalString(arguments, "destination")
	if err != nil {
		return nil, err
	}
	destination, err = resolveApplicationPlanDestination(s.Root, destination)
	if err != nil {
		return nil, err
	}
	foundationRoot, err := optionalString(arguments, "foundationRoot")
	if err != nil {
		return nil, err
	}
	options := blueprint.Options{
		Blueprint:           blueprintName,
		Destination:         destination,
		FrontendRegistryURL: s.ContributorFrontendRegistryURL,
		Application: blueprint.Application{
			Name:        name,
			DisplayName: displayName,
			Module:      module,
			Repository:  repository,
		},
		Write: false,
	}
	if foundationRoot == "" {
		return blueprint.GenerateEmbedded(ctx, s.Root, options)
	}
	options.FoundationRoot, err = resolveContributorFoundationRoot(s.Root, foundationRoot)
	if err != nil {
		return nil, err
	}
	return blueprint.Generate(ctx, options)
}

func resolveApplicationPlanDestination(workingRoot, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", nil
	}
	clean := filepath.Clean(filepath.FromSlash(requested))
	if !filepath.IsAbs(clean) && (filepath.VolumeName(clean) != "" || strings.HasPrefix(clean, string(filepath.Separator))) {
		return "", errors.New("application plan destination escapes the MCP working root")
	}
	if !filepath.IsAbs(clean) && (clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator))) {
		return "", errors.New("application plan destination escapes the MCP working root")
	}
	candidate := clean
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workingRoot, candidate)
	}
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve application plan destination: %w", err)
	}
	root, err := filepath.Abs(workingRoot)
	if err != nil {
		return "", fmt.Errorf("resolve MCP working root: %w", err)
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("application plan destination escapes the MCP working root")
	}
	return absolute, nil
}

func (s *Server) blueprintStatus(arguments map[string]any) (any, error) {
	manifestPath, err := optionalString(arguments, "manifestPath")
	if err != nil {
		return nil, err
	}
	projectContext, err := project.Load(s.Root)
	if err != nil {
		return nil, err
	}
	status, err := blueprint.ReadSnapshotStatus(s.Root, manifestPath)
	if err != nil {
		return nil, err
	}
	if err := status.ValidateProjectIdentity(
		projectContext.Project.Metadata.Name,
		projectContext.Project.Metadata.Repository,
	); err != nil {
		return nil, err
	}
	return status, nil
}

func (s *Server) foundationUpgrade(ctx context.Context, arguments map[string]any, write bool) (any, error) {
	foundationRoot, err := optionalString(arguments, "foundationRoot")
	if err != nil {
		return nil, err
	}
	manifestPath, err := optionalString(arguments, "manifestPath")
	if err != nil {
		return nil, err
	}
	blueprintName, err := optionalString(arguments, "blueprint")
	if err != nil {
		return nil, err
	}
	projectContext, err := project.Load(s.Root)
	if err != nil {
		return nil, err
	}
	options := blueprint.UpgradeOptions{
		ApplicationRoot:          s.Root,
		ManifestPath:             manifestPath,
		Blueprint:                blueprintName,
		Application:              foundationUpgradeApplication(projectContext),
		ModuleSpecificationsPath: foundationUpgradeModuleSpecificationsPath(projectContext),
		PreservedBusinessPaths:   foundationUpgradePreservedBusinessPaths(projectContext),
		ValidationCommands:       foundationUpgradeValidationCommands(projectContext),
		FrontendRegistryURL:      s.ContributorFrontendRegistryURL,
		Write:                    write,
	}
	var plan blueprint.UpgradePlan
	if foundationRoot == "" {
		options.RequestedDistributionVersion, err = embeddedReleaseDistributionVersion()
		if err != nil {
			return nil, err
		}
		plan, err = blueprint.UpgradeEmbedded(ctx, options)
	} else {
		options.FoundationRoot, err = resolveContributorFoundationRoot(s.Root, foundationRoot)
		if err != nil {
			return nil, err
		}
		plan, err = blueprint.Upgrade(ctx, options)
	}
	if err != nil {
		return plan, err
	}
	if write && plan.DryRun {
		return plan, fmt.Errorf("foundation upgrade did not enter write mode")
	}
	return plan, nil
}

func resolveContributorFoundationRoot(workingRoot, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", errors.New("contributor Foundation root is required")
	}
	if !filepath.IsAbs(requested) {
		requested = filepath.Join(workingRoot, filepath.FromSlash(requested))
	}
	absolute, err := filepath.Abs(requested)
	if err != nil {
		return "", fmt.Errorf("resolve contributor Foundation root: %w", err)
	}
	return absolute, nil
}

func embeddedReleaseDistributionVersion() (string, error) {
	provenance, err := buildinfo.ReleaseProvenance()
	if err != nil {
		return "", fmt.Errorf("embedded Admin Distribution upgrade is unavailable: %w; install an official release-built mss-mcp binary or use foundationRoot only from a clean Foundation contributor checkout", err)
	}
	return "v" + strings.TrimPrefix(strings.TrimSpace(provenance.Version), "v"), nil
}

// foundationUpgradeApplication leaves Module empty because the signed-in
// downstream snapshot owns the root module identity. project.yaml's
// spec.backend.module may instead name a nested deployable Admin module.
func foundationUpgradeApplication(projectContext *project.Context) blueprint.Application {
	if projectContext == nil {
		return blueprint.Application{}
	}
	return blueprint.Application{
		Name:        projectContext.Project.Metadata.Name,
		DisplayName: projectContext.Project.Metadata.DisplayName,
		Repository:  projectContext.Project.Metadata.Repository,
	}
}

func foundationUpgradeModuleSpecificationsPath(projectContext *project.Context) string {
	specifications := ".mss"
	if projectContext != nil {
		if configured := strings.TrimSpace(projectContext.Project.Spec.RepositoryLayout["specifications"]); configured != "" {
			specifications = configured
		}
	}
	return filepath.ToSlash(filepath.Join(filepath.FromSlash(specifications), "modules"))
}

func foundationUpgradePreservedBusinessPaths(projectContext *project.Context) []string {
	if projectContext == nil {
		return nil
	}
	layout := projectContext.Project.Spec.RepositoryLayout
	specifications := strings.TrimSpace(layout["specifications"])
	if specifications == "" {
		specifications = ".mss"
	}
	paths := []string{
		strings.TrimSpace(layout["modules"]),
		filepath.ToSlash(filepath.Join(filepath.FromSlash(specifications), "modules")),
		filepath.ToSlash(filepath.Join(filepath.FromSlash(specifications), "features")),
	}
	if frontend := strings.TrimSpace(layout["frontend"]); frontend != "" {
		paths = append(paths, filepath.ToSlash(filepath.Join(filepath.FromSlash(frontend), "src", "business")))
	}
	if documentation := strings.TrimSpace(layout["documentation"]); documentation != "" {
		if projectContext.LayoutKind() == "foundation" {
			paths = append(paths, filepath.ToSlash(filepath.Join(filepath.FromSlash(documentation), "docs", "modules")))
		} else {
			paths = append(paths, filepath.ToSlash(filepath.Join(filepath.FromSlash(documentation), "modules")))
		}
	}
	result := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
	}
	return result
}

func foundationUpgradeValidationCommands(projectContext *project.Context) []string {
	commands := []string{"mss doctor --strict --format json"}
	if projectContext == nil {
		return append(commands, "mss verify --all")
	}
	for _, name := range []string{"backend-test", "backend-build", "frontend-lint", "frontend-test", "frontend-build", "verify"} {
		entry, ok := projectContext.Commands.Spec.Commands[name]
		if !ok || strings.TrimSpace(entry.Command) == "" {
			continue
		}
		commands = append(commands, strings.TrimSpace(entry.Command))
	}
	if len(commands) == 1 {
		commands = append(commands, "mss verify --all")
	}
	return commands
}

func blueprintToolDefinitions() []Tool {
	readOnly := map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true}
	writeIdempotent := map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true}
	return []Tool{
		{
			Name:        "mss_plan_application",
			Title:       "Plan a downstream management application",
			Description: "Render a read-only deterministic Thin Host plan from the Blueprint embedded in the matching release-built mss-mcp binary. No Foundation checkout is needed. Contributors may explicitly select a clean checkout with foundationRoot. This tool never writes the destination.",
			InputSchema: objectSchema(map[string]any{
				"name":        map[string]any{"type": "string", "description": "Lower-case kebab-case application name."},
				"displayName": map[string]any{"type": "string"},
				"module":      map[string]any{"type": "string", "description": "Target Go module."},
				"repository":  map[string]any{"type": "string", "description": "Target owner/name repository."},
				"blueprint":   map[string]any{"type": "string", "default": "management-system"},
				"destination": map[string]any{"type": "string", "description": "Optional destination confined to the MCP working root and used only for conflict inspection."},
				"foundationRoot": map[string]any{
					"type":        "string",
					"description": "Optional clean Foundation checkout override for contributor development; ordinary package consumers omit it.",
				},
			}, []string{"name", "module"}),
			Annotations: readOnly,
		},
		{
			Name:        "mss_get_blueprint_status",
			Title:       "Get downstream foundation baseline",
			Description: "Read and cross-validate the downstream lock and manifest, returning compatibility metadata plus the four independent identities.",
			InputSchema: objectSchema(map[string]any{
				"manifestPath": map[string]any{"type": "string", "default": ".mss/blueprint-manifest.json"},
			}, nil),
			Annotations: readOnly,
		},
		{
			Name:        "mss_plan_foundation_upgrade",
			Title:       "Plan a three-way Admin Distribution upgrade",
			Description: "Compare the recorded baseline, current Thin Host, and the Distribution embedded in this release-built mss-mcp binary without writing files. The embedded target is bound to the binary release identity; contributors may explicitly override it with a clean Foundation checkout.",
			InputSchema: objectSchema(map[string]any{
				"foundationRoot": map[string]any{"type": "string", "description": "Optional clean Foundation checkout override for contributor development; ordinary package consumers omit it."},
				"manifestPath":   map[string]any{"type": "string", "default": ".mss/blueprint-manifest.json"},
				"blueprint":      map[string]any{"type": "string"},
			}, nil),
			Annotations: readOnly,
		},
		{
			Name:        "mss_apply_foundation_upgrade",
			Title:       "Apply a conflict-free Admin Distribution upgrade",
			Description: "Apply a reviewed three-way Thin Host upgrade from the Distribution embedded in this release-built mss-mcp binary. Requires confirm=true, refuses conflicts, preserves business-owned files, and writes the new baseline last. Contributors may explicitly override the source with a clean Foundation checkout.",
			InputSchema: objectSchema(map[string]any{
				"foundationRoot": map[string]any{"type": "string", "description": "Optional clean Foundation checkout override for contributor development; ordinary package consumers omit it."},
				"manifestPath":   map[string]any{"type": "string", "default": ".mss/blueprint-manifest.json"},
				"blueprint":      map[string]any{"type": "string"},
				"confirm":        map[string]any{"type": "boolean", "const": true},
			}, []string{"confirm"}),
			Annotations: writeIdempotent,
		},
	}
}
