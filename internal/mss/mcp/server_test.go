package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestServeSupportsLegacyInitializeAndStatelessTools(t *testing.T) {
	root := writeMCPFixture(t)
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"mss_get_project_context","arguments":{},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`,
	}
	var output bytes.Buffer
	server := &Server{Root: root}
	if err := server.Serve(context.Background(), bytes.NewBufferString(joinLines(requests)), &output); err != nil {
		t.Fatalf("serve MCP requests: %v", err)
	}

	responses := decodeResponses(t, output.Bytes())
	if len(responses) != 3 {
		t.Fatalf("response count = %d, want 3; output=%s", len(responses), output.String())
	}

	initialize := responses[0]
	result := objectResult(t, initialize)
	if result["protocolVersion"] != "2025-11-25" {
		t.Fatalf("protocolVersion = %#v", result["protocolVersion"])
	}

	list := objectResult(t, responses[1])
	if list["resultType"] != "complete" {
		t.Fatalf("tools/list resultType = %#v", list["resultType"])
	}
	tools, ok := list["tools"].([]any)
	if !ok || len(tools) < 7 {
		t.Fatalf("tools/list tools = %#v", list["tools"])
	}
	var names []string
	for _, raw := range tools {
		definition, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("invalid tool definition %#v", raw)
		}
		names = append(names, definition["name"].(string))
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("tool definitions are not sorted: %#v", names)
	}

	call := objectResult(t, responses[2])
	if call["isError"] == true {
		t.Fatalf("project context tool failed: %#v", call)
	}
	structured, ok := call["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("missing structuredContent: %#v", call)
	}
	projectDocument, ok := structured["project"].(map[string]any)
	if !ok {
		t.Fatalf("missing project document: %#v", structured)
	}
	metadata := projectDocument["metadata"].(map[string]any)
	if metadata["name"] != "fixture" {
		t.Fatalf("project name = %#v", metadata["name"])
	}
}

func TestServeReturnsToolErrorForEscapingPath(t *testing.T) {
	root := writeMCPFixture(t)
	request := `{"jsonrpc":"2.0","id":"escape","method":"tools/call","params":{"name":"mss_validate_module_spec","arguments":{"path":"../outside.yaml"}}}`
	var output bytes.Buffer
	server := &Server{Root: root}
	if err := server.Serve(context.Background(), bytes.NewBufferString(request+"\n"), &output); err != nil {
		t.Fatalf("serve MCP request: %v", err)
	}
	responses := decodeResponses(t, output.Bytes())
	result := objectResult(t, responses[0])
	if result["isError"] != true {
		t.Fatalf("expected tool error result, got %#v", result)
	}
}

func TestServeReturnsProtocolErrorForUnknownTool(t *testing.T) {
	root := writeMCPFixture(t)
	request := `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"missing","arguments":{}}}`
	var output bytes.Buffer
	server := &Server{Root: root}
	if err := server.Serve(context.Background(), bytes.NewBufferString(request+"\n"), &output); err != nil {
		t.Fatalf("serve MCP request: %v", err)
	}
	responses := decodeResponses(t, output.Bytes())
	if responses[0].Error == nil || responses[0].Error.Code != -32602 {
		t.Fatalf("unexpected response: %#v", responses[0])
	}
}

func TestServeNeverRespondsToNotifications(t *testing.T) {
	root := writeMCPFixture(t)
	input := "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n"
	var output bytes.Buffer
	server := &Server{Root: root}
	if err := server.Serve(context.Background(), bytes.NewBufferString(input), &output); err != nil {
		t.Fatalf("serve notification: %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("notification produced output %q", output.String())
	}
}

func writeMCPFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".mss/project.yaml": `apiVersion: mss.io/v1alpha1
kind: Project
metadata:
  name: fixture
spec:
  repositoryLayout:
    backend: .
    framework: framework
    frontend: web
    documentation: docs
    specifications: .mss
  backend:
    module: example.com/fixture
`,
		".mss/capabilities.yaml": `apiVersion: mss.io/v1alpha1
kind: CapabilityCatalog
metadata:
  project: fixture
spec:
  statuses:
    stable: stable
  capabilities:
    - id: identity.authentication
      displayName: Authentication
      status: stable
      owners: [backend]
      guidance: Reuse it.
`,
		".mss/commands.yaml": `apiVersion: mss.io/v1alpha1
kind: CommandCatalog
metadata:
  project: fixture
spec:
  commands:
    context:
      command: go run ./cmd/mss context
      description: Context
      category: agent
`,
		".agents/skills/fixture-skill/SKILL.md": `---
name: fixture-skill
description: Fixture skill for MCP tests.
---

# Fixture

Use the project contracts.
`,
	}
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", relative, err)
		}
	}
	return root
}

func decodeResponses(t *testing.T, data []byte) []rpcResponse {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	var responses []rpcResponse
	for decoder.More() {
		var response rpcResponse
		if err := decoder.Decode(&response); err != nil {
			t.Fatalf("decode response: %v; data=%s", err, string(data))
		}
		responses = append(responses, response)
	}
	return responses
}

func objectResult(t *testing.T, response rpcResponse) map[string]any {
	t.Helper()
	if response.Error != nil {
		t.Fatalf("unexpected RPC error: %#v", response.Error)
	}
	data, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatalf("marshal response result: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode object result: %v", err)
	}
	return result
}

func joinLines(lines []string) string {
	var buffer bytes.Buffer
	for _, line := range lines {
		buffer.WriteString(line)
		buffer.WriteByte('\n')
	}
	return buffer.String()
}
