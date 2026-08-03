package mcp

import (
	"context"
	"path/filepath"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

func (s *Server) callSpecificationTool(_ context.Context, name string, arguments map[string]any) (callToolResult, bool) {
	if name != "mss_validate_spec" {
		return callToolResult{}, false
	}
	path, err := requiredString(arguments, "path")
	if err != nil {
		return toolError(err), true
	}
	absolute, relative, err := s.resolveExistingFile(path)
	if err != nil {
		return toolError(err), true
	}
	document, err := spec.ValidateFile(absolute)
	if err != nil {
		return toolError(err), true
	}
	document.Path = filepath.ToSlash(relative)
	return toolSuccess(document), true
}

func specificationToolDefinitions() []Tool {
	return []Tool{
		{
			Name:        "mss_validate_spec",
			Title:       "Validate an MSS specification",
			Description: "Auto-detect and semantically validate a repository-relative AdminModule or Feature specification, including cross-reference and executable acceptance rules.",
			InputSchema: objectSchema(map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Repository-relative path to an AdminModule or Feature YAML specification.",
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
