package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/generator"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/skills"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/verify"
)

const (
	protocolLatest = "2026-07-28"
	serverName     = "mss-agent-foundation"
	serverVersion  = "0.1.0"
)

const serverInstructions = "Use these tools as the repository source of truth. Inspect project context and existing capabilities before planning changes. Module generation and validation execution default to dry-run; set write or execute only after reviewing the returned plan. Never pass production credentials or paths outside the repository root."

// Server exposes stable project packages through newline-delimited JSON-RPC.
type Server struct {
	Root   string
	Stderr io.Writer
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type initializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// Tool is one deterministic MCP tool definition.
type Tool struct {
	Name         string         `json:"name"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	Annotations  map[string]any `json:"annotations,omitempty"`
}

type listToolsResult struct {
	ResultType string `json:"resultType,omitempty"`
	Tools      []Tool `json:"tools"`
	TTLMS      int    `json:"ttlMs,omitempty"`
	CacheScope string `json:"cacheScope,omitempty"`
}

type callToolResult struct {
	ResultType        string        `json:"resultType,omitempty"`
	Content           []textContent `json:"content"`
	StructuredContent any           `json:"structuredContent,omitempty"`
	IsError           bool          `json:"isError,omitempty"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Serve runs the stdio transport until input closes or the context is cancelled.
func (s *Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	if strings.TrimSpace(s.Root) == "" {
		return errors.New("repository root is required")
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	s.Root = root
	if s.Stderr == nil {
		s.Stderr = io.Discard
	}

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var request rpcRequest
		if err := json.Unmarshal(line, &request); err != nil {
			if encodeErr := encoder.Encode(errorResponse(nil, -32700, "Parse error", err.Error())); encodeErr != nil {
				return encodeErr
			}
			continue
		}
		response, respond := s.handle(ctx, request)
		if !respond {
			continue
		}
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("write MCP response: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read MCP stdio request: %w", err)
	}
	return nil
}

func (s *Server) handle(ctx context.Context, request rpcRequest) (rpcResponse, bool) {
	if request.JSONRPC != "2.0" {
		if len(request.ID) == 0 {
			return rpcResponse{}, false
		}
		return errorResponse(request.ID, -32600, "Invalid Request", "jsonrpc must equal 2.0"), true
	}
	if strings.TrimSpace(request.Method) == "" {
		if len(request.ID) == 0 {
			return rpcResponse{}, false
		}
		return errorResponse(request.ID, -32600, "Invalid Request", "method is required"), true
	}

	// Notifications never receive responses.
	if len(request.ID) == 0 {
		s.handleNotification(request)
		return rpcResponse{}, false
	}

	switch request.Method {
	case "initialize":
		return successResponse(request.ID, s.initialize(request.Params)), true
	case "ping":
		return successResponse(request.ID, map[string]any{}), true
	case "tools/list":
		return successResponse(request.ID, listToolsResult{
			ResultType: "complete",
			Tools:      tools(),
			TTLMS:      300000,
			CacheScope: "public",
		}), true
	case "tools/call":
		var params callToolParams
		if err := decodeParams(request.Params, &params); err != nil {
			return errorResponse(request.ID, -32602, "Invalid params", err.Error()), true
		}
		if strings.TrimSpace(params.Name) == "" {
			return errorResponse(request.ID, -32602, "Invalid params", "tool name is required"), true
		}
		result, known := s.callTool(ctx, params.Name, params.Arguments)
		if !known {
			return errorResponse(request.ID, -32602, "Invalid params", "unknown tool "+params.Name), true
		}
		return successResponse(request.ID, result), true
	default:
		return errorResponse(request.ID, -32601, "Method not found", request.Method), true
	}
}

func (s *Server) handleNotification(request rpcRequest) {
	switch request.Method {
	case "notifications/initialized", "notifications/cancelled":
		return
	default:
		_, _ = fmt.Fprintf(s.Stderr, "mss-mcp: ignored notification %s\n", request.Method)
	}
}

func (s *Server) initialize(raw json.RawMessage) map[string]any {
	params := initializeParams{}
	_ = decodeParams(raw, &params)
	version := negotiateProtocol(params.ProtocolVersion)
	return map[string]any{
		"protocolVersion": version,
		"capabilities": map[string]any{
			"tools": map[string]any{"listChanged": false},
		},
		"serverInfo": map[string]any{
			"name":        serverName,
			"title":       "mss Agent-Native Foundation",
			"version":     serverVersion,
			"description": "Repository context, generation, and validation tools for mss-boot-admin.",
		},
		"instructions": serverInstructions,
	}
}

func (s *Server) callTool(ctx context.Context, name string, arguments map[string]any) (callToolResult, bool) {
	var value any
	var err error
	switch name {
	case "mss_get_project_context":
		value, err = project.Load(s.Root)
	case "mss_list_capabilities":
		value, err = s.listCapabilities(arguments)
	case "mss_list_skills":
		value, err = s.listSkills(arguments)
	case "mss_validate_module_spec":
		value, err = s.validateModule(arguments)
	case "mss_generate_module":
		value, err = s.generateModule(arguments)
	case "mss_get_validation_plan":
		value, err = s.validationPlan(arguments)
	case "mss_run_validation":
		value, err = s.runValidation(ctx, arguments)
	default:
		return callToolResult{}, false
	}
	if err != nil {
		return toolError(err), true
	}
	return toolSuccess(value), true
}

func (s *Server) listCapabilities(arguments map[string]any) (any, error) {
	contextDocument, err := project.Load(s.Root)
	if err != nil {
		return nil, err
	}
	status, err := optionalString(arguments, "status")
	if err != nil {
		return nil, err
	}
	capabilities := contextDocument.Capabilities.Spec.Capabilities
	if status != "" {
		filtered := make([]project.Capability, 0)
		for _, capability := range capabilities {
			if capability.Status == status {
				filtered = append(filtered, capability)
			}
		}
		capabilities = filtered
	}
	return map[string]any{
		"project":      contextDocument.Project.Metadata.Name,
		"statusFilter": status,
		"capabilities": capabilities,
	}, nil
}

func (s *Server) listSkills(arguments map[string]any) (any, error) {
	includeBody, err := optionalBool(arguments, "includeBody", false)
	if err != nil {
		return nil, err
	}
	report, discoverErr := skills.Discover(s.Root)
	if discoverErr != nil {
		return report, discoverErr
	}
	if !includeBody {
		for index := range report.Skills {
			report.Skills[index].Body = ""
		}
	}
	return report, nil
}

func (s *Server) validateModule(arguments map[string]any) (any, error) {
	path, err := requiredString(arguments, "path")
	if err != nil {
		return nil, err
	}
	absolute, relative, err := s.resolveExistingFile(path)
	if err != nil {
		return nil, err
	}
	module, err := spec.LoadModule(absolute)
	if err != nil {
		return nil, err
	}
	module.SourcePath = relative
	return map[string]any{
		"valid":  true,
		"path":   relative,
		"module": module,
	}, nil
}

func (s *Server) generateModule(arguments map[string]any) (any, error) {
	path, err := requiredString(arguments, "path")
	if err != nil {
		return nil, err
	}
	write, err := optionalBool(arguments, "write", false)
	if err != nil {
		return nil, err
	}
	check, err := optionalBool(arguments, "check", false)
	if err != nil {
		return nil, err
	}
	if write && check {
		return nil, errors.New("write and check cannot both be true")
	}
	absolute, relative, err := s.resolveExistingFile(path)
	if err != nil {
		return nil, err
	}
	module, err := spec.LoadModule(absolute)
	if err != nil {
		return nil, err
	}
	module.SourcePath = relative
	return generator.Generate(module, generator.Options{Root: s.Root, Write: write, Check: check})
}

func (s *Server) validationPlan(arguments map[string]any) (any, error) {
	contextDocument, options, err := s.validationOptions(arguments)
	if err != nil {
		return nil, err
	}
	return verify.PlanChecks(contextDocument, options)
}

func (s *Server) runValidation(ctx context.Context, arguments map[string]any) (any, error) {
	contextDocument, options, err := s.validationOptions(arguments)
	if err != nil {
		return nil, err
	}
	execute, err := optionalBool(arguments, "execute", false)
	if err != nil {
		return nil, err
	}
	options.PlanOnly = !execute
	return verify.Run(ctx, contextDocument, options)
}

func (s *Server) validationOptions(arguments map[string]any) (*project.Context, verify.Options, error) {
	contextDocument, err := project.Load(s.Root)
	if err != nil {
		return nil, verify.Options{}, err
	}
	mode, err := optionalString(arguments, "mode")
	if err != nil {
		return nil, verify.Options{}, err
	}
	if mode == "" {
		mode = string(verify.ModeChanged)
	}
	baseRef, err := optionalString(arguments, "baseRef")
	if err != nil {
		return nil, verify.Options{}, err
	}
	module, err := optionalString(arguments, "module")
	if err != nil {
		return nil, verify.Options{}, err
	}
	return contextDocument, verify.Options{
		Mode:    verify.Mode(mode),
		BaseRef: baseRef,
		Module:  module,
	}, nil
}

func (s *Server) resolveExistingFile(input string) (string, string, error) {
	if strings.TrimSpace(input) == "" {
		return "", "", errors.New("path is required")
	}
	if filepath.IsAbs(input) {
		return "", "", errors.New("absolute paths are not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(input))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", errors.New("path escapes repository root")
	}
	absolute := filepath.Join(s.Root, clean)
	info, err := os.Stat(absolute)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", errors.New("path must reference a regular file")
	}
	realPath, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", "", err
	}
	relative, err := filepath.Rel(s.Root, realPath)
	if err != nil {
		return "", "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", errors.New("resolved path escapes repository root")
	}
	return realPath, filepath.ToSlash(relative), nil
}

func tools() []Tool {
	readOnly := map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true}
	writeIdempotent := map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true}
	definitions := []Tool{
		{
			Name:        "mss_get_project_context",
			Title:       "Get mss project context",
			Description: "Return the normalized project, capability, and command contracts for the current repository.",
			InputSchema: emptyObjectSchema(),
			Annotations: readOnly,
		},
		{
			Name:        "mss_list_capabilities",
			Title:       "List existing project capabilities",
			Description: "List reusable and legacy capabilities, optionally filtered by lifecycle status, before planning new code.",
			InputSchema: objectSchema(map[string]any{
				"status": map[string]any{"type": "string", "enum": []string{"stable", "beta", "legacy", "planned"}},
			}, nil),
			Annotations: readOnly,
		},
		{
			Name:        "mss_list_skills",
			Title:       "List repository Agent Skills",
			Description: "Return validated repository-local Agent Skills. Bodies are omitted unless includeBody is true.",
			InputSchema: objectSchema(map[string]any{
				"includeBody": map[string]any{"type": "boolean", "default": false},
			}, nil),
			Annotations: readOnly,
		},
		{
			Name:        "mss_validate_module_spec",
			Title:       "Validate an AdminModule specification",
			Description: "Load, normalize, and semantically validate one repository-relative AdminModule YAML file.",
			InputSchema: objectSchema(map[string]any{
				"path": map[string]any{"type": "string", "description": "Repository-relative path to an AdminModule YAML file."},
			}, []string{"path"}),
			Annotations: readOnly,
		},
		{
			Name:        "mss_generate_module",
			Title:       "Plan or apply deterministic module generation",
			Description: "Generate a deterministic module plan. Defaults to dry-run; set write=true only after reviewing the plan. check=true verifies drift without writes.",
			InputSchema: objectSchema(map[string]any{
				"path":  map[string]any{"type": "string", "description": "Repository-relative AdminModule YAML path."},
				"write": map[string]any{"type": "boolean", "default": false},
				"check": map[string]any{"type": "boolean", "default": false},
			}, []string{"path"}),
			Annotations: writeIdempotent,
		},
		{
			Name:        "mss_get_validation_plan",
			Title:       "Get a change-aware validation plan",
			Description: "Return the deterministic minimum-sufficient validation plan without executing commands.",
			InputSchema: validationSchema(false),
			Annotations: readOnly,
		},
		{
			Name:        "mss_run_validation",
			Title:       "Plan or execute repository validation",
			Description: "Create a validation report. Defaults to plan-only; set execute=true to run the selected commands.",
			InputSchema: validationSchema(true),
			Annotations: writeIdempotent,
		},
	}
	sort.SliceStable(definitions, func(i, j int) bool { return definitions[i].Name < definitions[j].Name })
	return definitions
}

func validationSchema(includeExecute bool) map[string]any {
	properties := map[string]any{
		"mode":    map[string]any{"type": "string", "enum": []string{"changed", "all", "module"}, "default": "changed"},
		"baseRef": map[string]any{"type": "string"},
		"module":  map[string]any{"type": "string"},
	}
	if includeExecute {
		properties["execute"] = map[string]any{"type": "boolean", "default": false}
	}
	return objectSchema(properties, nil)
}

func emptyObjectSchema() map[string]any {
	return map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"additionalProperties": false,
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := emptyObjectSchema()
	schema["properties"] = properties
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func toolSuccess(value any) callToolResult {
	text, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return toolError(err)
	}
	return callToolResult{
		ResultType:        "complete",
		Content:           []textContent{{Type: "text", Text: string(text)}},
		StructuredContent: value,
	}
}

func toolError(err error) callToolResult {
	return callToolResult{
		ResultType: "complete",
		Content:    []textContent{{Type: "text", Text: err.Error()}},
		StructuredContent: map[string]any{
			"error": err.Error(),
		},
		IsError: true,
	}
}

func successResponse(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: cloneRaw(id), Result: result}
}

func errorResponse(id json.RawMessage, code int, message string, data any) rpcResponse {
	return rpcResponse{
		JSONRPC: "2.0",
		ID:      cloneRaw(id),
		Error:   &rpcError{Code: code, Message: message, Data: data},
	}
}

func decodeParams(raw json.RawMessage, target any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func requiredString(arguments map[string]any, key string) (string, error) {
	value, exists := arguments[key]
	if !exists {
		return "", fmt.Errorf("argument %s is required", key)
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("argument %s must be a non-empty string", key)
	}
	return strings.TrimSpace(text), nil
}

func optionalString(arguments map[string]any, key string) (string, error) {
	value, exists := arguments[key]
	if !exists || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("argument %s must be a string", key)
	}
	return strings.TrimSpace(text), nil
}

func optionalBool(arguments map[string]any, key string, fallback bool) (bool, error) {
	value, exists := arguments[key]
	if !exists || value == nil {
		return fallback, nil
	}
	boolean, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("argument %s must be a boolean", key)
	}
	return boolean, nil
}

func negotiateProtocol(requested string) string {
	supported := map[string]bool{
		"2024-11-05":   true,
		"2025-03-26":   true,
		"2025-06-18":   true,
		"2025-11-25":   true,
		protocolLatest: true,
	}
	if supported[requested] {
		return requested
	}
	// A legacy client that sends an unknown version still receives the newest
	// session-based revision; 2026-07-28 clients do not issue initialize.
	return "2025-11-25"
}

func cloneRaw(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage("null")
	}
	return append(json.RawMessage(nil), value...)
}
