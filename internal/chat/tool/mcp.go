// Package tool provides the tool registry for managing, discovering, and
// executing tools within the Cosca chat agent system.
package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/mcp"
	"github.com/CoscaAI/cosca/internal/contenttrust"
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
//
// I8/I6/I2 (mineração git-secrets): o conteúdo vindo do servidor MCP é EXTERNO.
// Passa pelo contenttrust.GuardSecrets — se vazar um segredo, é QUARENTENADO
// (I6) e mascarado (o valor cru não flui para contexto/persistência/ledger).
// O guard é determinístico (I1) e fail-closed (I2); nunca concede autoridade.
func (t *MCPTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	if t.client == nil {
		return nil, fmt.Errorf("MCP tool %q: no client connection available", t.name)
	}
	result, err := t.client.ExecuteTool(ctx, t.name, params)
	if err != nil {
		return nil, err
	}
	if result != nil {
		guarded := contenttrust.GuardSecrets(contenttrust.Item{
			Content:   result.Output,
			Origin:    contenttrust.OriginMCP,
			Source:    t.name,
			Authority: contenttrust.AuthorityExternal,
			Trust:     contenttrust.TrustUntrusted,
		})
		result.Output = guarded.Content
		// Se vazou segredo, o conteúdo já foi mascarado (e marcado como
		// quarantined internamente). Mantemos o output mascarado para contexto;
		// nunca elevamos autoridade (I8).
	}
	return result, nil
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
