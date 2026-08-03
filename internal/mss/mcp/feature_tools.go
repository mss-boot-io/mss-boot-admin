package mcp

import (
	"context"

	featurecmd "github.com/mss-boot-io/mss-boot-admin/internal/mss/feature"
)

func (s *Server) callFeatureTool(_ context.Context, name string, arguments map[string]any) (callToolResult, bool) {
	if name != "mss_plan_feature" {
		return callToolResult{}, false
	}
	path, err := requiredString(arguments, "path")
	if err != nil {
		return toolError(err), true
	}
	plan, err := featurecmd.Build(featurecmd.Options{
		Root:        s.Root,
		FeaturePath: path,
	})
	if err != nil {
		return toolErrorWithStructured(err, plan), true
	}
	return toolSuccess(plan), true
}

func featureToolDefinitions() []Tool {
	return []Tool{
		{
			Name:        "mss_plan_feature",
			Title:       "Plan a cross-module Feature implementation",
			Description: "Validate a Feature and all referenced AdminModule contracts, then return generation impact, requirements, permissions, acceptance evidence, risks, validation, rollout, and rollback without executing arbitrary evidence commands.",
			InputSchema: objectSchema(map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Repository-relative path to a Feature YAML specification.",
				},
			}, []string{"path"}),
			Annotations: map[string]any{
				"readOnlyHint":    true,
				"destructiveHint": false,
				"idempotentHint":  true,
			},
		},
	}
}

func toolErrorWithStructured(err error, value any) callToolResult {
	result := toolError(err)
	result.StructuredContent = value
	return result
}
