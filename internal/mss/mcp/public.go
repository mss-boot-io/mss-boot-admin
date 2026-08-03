package mcp

// Tools returns the deterministic MCP tool catalog. Callers must treat the
// returned schema maps as read-only.
func Tools() []Tool {
	return tools()
}
