package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/blueprint"
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
	return blueprint.Generate(ctx, blueprint.Options{
		FoundationRoot: s.Root,
		Blueprint:      blueprintName,
		Destination:    destination,
		Application: blueprint.Application{
			Name:        name,
			DisplayName: displayName,
			Module:      module,
			Repository:  repository,
		},
		Write: false,
	})
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
	foundationRoot, err := requiredString(arguments, "foundationRoot")
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
	plan, err := blueprint.Upgrade(ctx, blueprint.UpgradeOptions{
		ApplicationRoot: s.Root,
		FoundationRoot:  foundationRoot,
		ManifestPath:    manifestPath,
		Blueprint:       blueprintName,
		Application:     foundationUpgradeApplication(projectContext),
		Write:           write,
	})
	if err != nil {
		return plan, err
	}
	if write && plan.DryRun {
		return plan, fmt.Errorf("foundation upgrade did not enter write mode")
	}
	return plan, nil
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

func blueprintToolDefinitions() []Tool {
	readOnly := map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true}
	writeIdempotent := map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true}
	return []Tool{
		{
			Name:        "mss_plan_application",
			Title:       "Plan a downstream management application",
			Description: "Render a read-only deterministic plan for a complete downstream application. This MCP tool never writes the destination; use the CLI Skill for an approved write.",
			InputSchema: objectSchema(map[string]any{
				"name":        map[string]any{"type": "string", "description": "Lower-case kebab-case application name."},
				"displayName": map[string]any{"type": "string"},
				"module":      map[string]any{"type": "string", "description": "Target Go module."},
				"repository":  map[string]any{"type": "string", "description": "Target owner/name repository."},
				"blueprint":   map[string]any{"type": "string", "default": "management-system"},
				"destination": map[string]any{"type": "string", "description": "Optional destination used only for conflict inspection."},
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
			Title:       "Plan a three-way foundation upgrade",
			Description: "Compare the recorded old foundation, current downstream content, and a newer foundation checkout without writing files.",
			InputSchema: objectSchema(map[string]any{
				"foundationRoot": map[string]any{"type": "string", "description": "Path to the newer trusted foundation checkout."},
				"manifestPath":   map[string]any{"type": "string", "default": ".mss/blueprint-manifest.json"},
				"blueprint":      map[string]any{"type": "string"},
			}, []string{"foundationRoot"}),
			Annotations: readOnly,
		},
		{
			Name:        "mss_apply_foundation_upgrade",
			Title:       "Apply a conflict-free foundation upgrade",
			Description: "Apply a reviewed three-way upgrade. Requires confirm=true, refuses conflicts, preserves downstream-only files, and writes the new baseline last.",
			InputSchema: objectSchema(map[string]any{
				"foundationRoot": map[string]any{"type": "string", "description": "Path to the newer trusted foundation checkout."},
				"manifestPath":   map[string]any{"type": "string", "default": ".mss/blueprint-manifest.json"},
				"blueprint":      map[string]any{"type": "string"},
				"confirm":        map[string]any{"type": "boolean", "const": true},
			}, []string{"foundationRoot", "confirm"}),
			Annotations: writeIdempotent,
		},
	}
}
