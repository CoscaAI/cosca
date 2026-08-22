// Package tool provides the tool registry for managing, discovering, and
// executing tools within the Cosca chat agent system.
package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/mcp"
)

// MCPTool wraps a tool discovered from an MCP server as a chat.Tool.
// It holds a reference to the MCP client so that Execute delegates to
// the server via JSON-RPC tools/call.
type MCPTool struct {
	name        string
	description string
	schema      json.RawMessage
	client      *mcp.Client
}

// Compile-time interface check.
var _ chat.Tool = (*MCPTool)(nil)

// NewMCPTool creates a new MCPTool from an MCP tool definition and client.
func NewMCPTool(info mcp.ToolInfo, client *mcp.Client) *MCPTool {
	return &MCPTool{
		name:        info.Name,
		description: info.Description,
		schema:      info.InputSchema,
		client:      client,
	}
}

// Name returns the tool's unique identifier (e.g. "cosca_knowledge_search").
func (t *MCPTool) Name() string { return t.name }

// Description returns a human-readable description of the tool.
func (t *MCPTool) Description() string { return t.description }

// Schema returns the JSON Schema that describes the tool's parameters.
func (t *MCPTool) Schema() json.RawMessage { return t.schema }

// Execute runs the tool on the MCP server via JSON-RPC tools/call. It
// marshals the params and delegates to the MCP client's ExecuteTool method.
// The result's Output contains the text content returned by the server.
func (t *MCPTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	if t.client == nil {
		return nil, fmt.Errorf("MCP tool %q: no client connection available", t.name)
	}
	return t.client.ExecuteTool(ctx, t.name, params)
}

// Validate checks whether the given JSON-encoded parameters are valid JSON.
// MCP servers are responsible for schema-level validation; this adapter
// performs only a basic well-formedness check.
func (t *MCPTool) Validate(params json.RawMessage) error {
	if !json.Valid(params) {
		return fmt.Errorf("invalid JSON parameters for tool %q", t.name)
	}
	return nil
}
