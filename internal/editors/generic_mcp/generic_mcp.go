// Package genericmcp provides a generic MCP (Model Context Protocol) adapter
// that wraps Cosca capabilities as MCP tools, supporting both stdio
// and HTTP transport.
package genericmcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

// =============================================================================
// Constants
// =============================================================================

// MCPVersion is the MCP protocol version.
const MCPVersion = "2024-11-05"

// Transport types.
const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
)

// =============================================================================
// Adapter
// =============================================================================

// Adapter provides a generic MCP (Model Context Protocol) integration
// that wraps Cosca capabilities as MCP tools. It supports both
// stdio-based and HTTP-based transport protocols.
type Adapter struct {
	types.BaseEditor
}

// NewAdapter creates a new Generic MCP editor adapter.
func NewAdapter() *Adapter {
	return &Adapter{
		BaseEditor: types.BaseEditor{
			NameValue: "generic_mcp",
			CapabilitiesVal: types.EditorCapabilities{
				SupportsContext:        true,
				SupportsSearch:         true,
				SupportsExecute:        true,
				SupportsWatch:          false,
				SupportsMCP:            true,
				SupportsCustomCommands: false,
				SupportsKeybindings:    false,
			},
		},
	}
}

// =============================================================================
// MCP Type Definitions
// =============================================================================

// MCPToolSchema defines the JSON Schema for a tool's parameters.
type MCPToolSchema struct {
	Type       string                       `json:"type"`
	Properties map[string]MCPPropertySchema `json:"properties,omitempty"`
	Required   []string                     `json:"required,omitempty"`
}

// MCPPropertySchema defines the schema for a single tool parameter.
type MCPPropertySchema struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// MCPToolDefinition defines a tool exposed via MCP. Command and Args are
// optional: when set they describe how the tool is executed (the executable
// and its arguments), mirroring the editor task conventions used elsewhere
// in Cosca.
type MCPToolDefinition struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	InputSchema MCPToolSchema `json:"inputSchema"`
	Command     string        `json:"command,omitempty"`
	Args        []string      `json:"args,omitempty"`
}

// MCPResourceDefinition defines a resource exposed via MCP.
type MCPResourceDefinition struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// MCPServerDefinition defines the complete MCP server configuration.
type MCPServerDefinition struct {
	ProtocolVersion string                  `json:"protocolVersion"`
	ServerInfo      MCPServerInfo           `json:"serverInfo"`
	Tools           []MCPToolDefinition     `json:"tools"`
	Resources       []MCPResourceDefinition `json:"resources,omitempty"`
	Capabilities    MCPServerCapabilities   `json:"capabilities"`
}

// MCPServerInfo provides metadata about the MCP server.
type MCPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPServerCapabilities describes the MCP server's capabilities.
type MCPServerCapabilities struct {
	Tools     *struct{} `json:"tools,omitempty"`
	Resources *struct{} `json:"resources,omitempty"`
}

// MCPConfig represents a client's MCP server configuration entry.
type MCPConfig struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// =============================================================================
// Cosca Tool Definitions
// =============================================================================

// CoscaToolDefinitions returns the MCP tool definitions for Cosca capabilities.
func CoscaToolDefinitions() []MCPToolDefinition {
	return []MCPToolDefinition{
		{
			Name:        "cosca_search",
			Description: "Perform semantic search across the codebase using Cosca. Returns relevant code snippets, files, and their relevance scores.",
			InputSchema: MCPToolSchema{
				Type: "object",
				Properties: map[string]MCPPropertySchema{
					"query": {
						Type:        "string",
						Description: "The search query (natural language or code pattern)",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results to return (default: 10)",
						Default:     10,
					},
					"file_pattern": {
						Type:        "string",
						Description: "Optional file pattern to filter results (e.g., '*.go')",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "cosca_index",
			Description: "Index the current codebase with Cosca. Must be run after significant code changes to keep search results relevant.",
			InputSchema: MCPToolSchema{
				Type: "object",
				Properties: map[string]MCPPropertySchema{
					"path": {
						Type:        "string",
						Description: "Optional path to index (defaults to project root)",
					},
					"force": {
						Type:        "boolean",
						Description: "Force re-index even if already indexed",
						Default:     false,
					},
				},
			},
		},
		{
			Name:        "cosca_context",
			Description: "Get Cosca-generated context for a specific file. Returns relevant symbols, types, and documentation.",
			InputSchema: MCPToolSchema{
				Type: "object",
				Properties: map[string]MCPPropertySchema{
					"file": {
						Type:        "string",
						Description: "Path to the file to get context for",
					},
				},
				Required: []string{"file"},
			},
		},
		{
			Name:        "cosca_status",
			Description: "Check Cosca system status including index health, last indexed, and plugin status.",
			InputSchema: MCPToolSchema{
				Type: "object",
				Properties: map[string]MCPPropertySchema{
					"verbose": {
						Type:        "boolean",
						Description: "Show detailed status information",
						Default:     false,
					},
				},
			},
		},
		{
			Name:        "cosca_memory",
			Description: "Store or retrieve information from Cosca memory system for cross-session context.",
			InputSchema: MCPToolSchema{
				Type: "object",
				Properties: map[string]MCPPropertySchema{
					"action": {
						Type:        "string",
						Description: "Action to perform: 'store', 'retrieve', or 'search'",
						Enum:        []string{"store", "retrieve", "search"},
					},
					"key": {
						Type:        "string",
						Description: "Memory key (for store/retrieve actions)",
					},
					"value": {
						Type:        "string",
						Description: "Memory value (for store action)",
					},
					"query": {
						Type:        "string",
						Description: "Search query (for search action)",
					},
				},
				Required: []string{"action"},
			},
		},
		{
			Name:        "cosca_kernel_identity",
			Description: "Carregar o Cosca Kernel — identity, leis, constituição",
			InputSchema: MCPToolSchema{
				Type:       "object",
				Properties: map[string]MCPPropertySchema{},
			},
			Command: "cosca",
			Args:    []string{"kernel", "identity"},
		},
	}
}

// CoscaResourceDefinitions returns MCP resource definitions for Cosca.
func CoscaResourceDefinitions() []MCPResourceDefinition {
	return []MCPResourceDefinition{
		{
			URI:         "cosca://status",
			Name:        "Cosca Status",
			Description: "Current Cosca system status and health information",
			MimeType:    "application/json",
		},
		{
			URI:         "cosca://index/stats",
			Name:        "Cosca Index Statistics",
			Description: "Codebase index statistics including file count and last indexed time",
			MimeType:    "application/json",
		},
	}
}

// =============================================================================
// MCP Server Generation
// =============================================================================

// GenerateMCPServerDefinition creates a complete MCP server definition.
func GenerateMCPServerDefinition() *MCPServerDefinition {
	return &MCPServerDefinition{
		ProtocolVersion: MCPVersion,
		ServerInfo: MCPServerInfo{
			Name:    "cosca",
			Version: "1.0.0",
		},
		Tools:     CoscaToolDefinitions(),
		Resources: CoscaResourceDefinitions(),
		Capabilities: MCPServerCapabilities{
			Tools:     &struct{}{},
			Resources: &struct{}{},
		},
	}
}

// GenerateMCPConfig generates an MCP client configuration entry.
func GenerateMCPConfig(coscaBinPath string) *MCPConfig {
	if coscaBinPath == "" {
		coscaBinPath = "cosca"
	}

	return &MCPConfig{
		Command: coscaBinPath,
		Args:    []string{"mcp", "serve"},
		Env: map[string]string{
			"COSCA_MCP_TRANSPORT": TransportStdio,
		},
	}
}

// GenerateMCPHTTPServerConfig generates an MCP client configuration for HTTP.
func GenerateMCPHTTPServerConfig(host string, port int) *MCPConfig {
	return &MCPConfig{
		URL: fmt.Sprintf("http://%s:%d/mcp", host, port),
	}
}

// =============================================================================
// Adapter Implementation
// =============================================================================

// Version returns the MCP protocol version.
func (a *Adapter) Version() (string, error) {
	return MCPVersion, nil
}

// Detect checks if any MCP-compatible editor is present.
func (a *Adapter) Detect() (bool, error) {
	homeDir, _ := os.UserHomeDir()
	checkPaths := []string{
		filepath.Join(homeDir, ".config", "mcp"),
		filepath.Join(homeDir, ".mcp"),
		".mcp",
		filepath.Join(homeDir, ".cursor", "mcp.json"),
	}

	for _, path := range checkPaths {
		if _, err := os.Stat(path); err == nil {
			return true, nil
		}
	}

	if os.Getenv("MCP_ENABLED") == "true" {
		return true, nil
	}

	mcpConfigs := []string{
		filepath.Join(homeDir, ".config", "windsurf", "mcp_config.json"),
		filepath.Join(homeDir, ".config", "claude", "mcp_config.json"),
	}
	for _, path := range mcpConfigs {
		if _, err := os.Stat(path); err == nil {
			return true, nil
		}
	}

	return false, nil
}

// Setup creates MCP configuration files for Cosca integration.
func (a *Adapter) Setup(config types.EditorConfig) error {
	mcpDir := filepath.Join(config.ProjectDir, ".mcp")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		return fmt.Errorf("create .mcp directory: %w", err)
	}

	serverDef := GenerateMCPServerDefinition()
	serverDefJSON, err := json.MarshalIndent(serverDef, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal MCP server definition: %w", err)
	}

	serverDefPath := filepath.Join(mcpDir, "cosca-server.json")
	if err := os.WriteFile(serverDefPath, serverDefJSON, 0o644); err != nil {
		return fmt.Errorf("write MCP server definition: %w", err)
	}

	mcpConfig := GenerateMCPConfig(config.CoscaBinPath)
	mcpConfigJSON, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal MCP config: %w", err)
	}

	mcpConfigPath := filepath.Join(mcpDir, "mcp.json")
	if err := os.WriteFile(mcpConfigPath, mcpConfigJSON, 0o644); err != nil {
		return fmt.Errorf("write MCP config: %w", err)
	}

	readmeContent := fmt.Sprintf(`# Cosca MCP Integration

This directory contains MCP (Model Context Protocol) configuration for Cosca.

## Files

- %s - MCP server definition with tool/resource schemas
- %s - MCP client configuration for connecting to the Cosca MCP server

## Tools Available

%s

## Usage

### stdio Transport (default)
The Cosca MCP server runs as a subprocess:
  %s mcp serve

### HTTP Transport
Start the Cosca MCP server in HTTP mode:
  COSCA_MCP_TRANSPORT=http %s mcp serve --http :8370
`,
		"cosca-server.json",
		"mcp.json",
		formatToolList(serverDef.Tools),
		config.CoscaBinPath,
		config.CoscaBinPath,
	)

	readmePath := filepath.Join(mcpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0o644); err != nil {
		return fmt.Errorf("write MCP README: %w", err)
	}

	log.Info().Str("dir", mcpDir).Msg("Cosca MCP integration configured")
	return nil
}

// Validate checks that MCP configuration files exist.
func (a *Adapter) Validate() error {
	mcpDir := filepath.Join(".", ".mcp")

	if info, err := os.Stat(mcpDir); err != nil || !info.IsDir() {
		return fmt.Errorf(".mcp directory not found: %w", types.ErrEditorNotConfigured)
	}

	requiredFiles := []string{
		filepath.Join(mcpDir, "cosca-server.json"),
		filepath.Join(mcpDir, "mcp.json"),
	}

	for _, f := range requiredFiles {
		if _, err := os.Stat(f); err != nil {
			return fmt.Errorf("required MCP file %q not found: %w", f, types.ErrEditorNotConfigured)
		}
	}

	for _, f := range requiredFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %q: %w", f, err)
		}
		if !json.Valid(data) {
			return fmt.Errorf("%q is not valid JSON", f)
		}
	}

	return nil
}

// Teardown removes MCP configuration files.
func (a *Adapter) Teardown() error {
	mcpDir := filepath.Join(".", ".mcp")

	if _, err := os.Stat(mcpDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := os.RemoveAll(mcpDir); err != nil {
		return fmt.Errorf("remove .mcp directory: %w", err)
	}

	log.Info().Str("dir", mcpDir).Msg("Cosca MCP integration removed")
	return nil
}

// Info returns information about the Generic MCP integration.
func (a *Adapter) Info() (types.EditorInfo, error) {
	detected := false
	mcpDir := filepath.Join(".", ".mcp")

	if info, err := os.Stat(mcpDir); err == nil && info.IsDir() {
		detected = true
	}

	return types.EditorInfo{
		Name:         "generic_mcp",
		Version:      MCPVersion,
		Path:         mcpDir,
		Capabilities: a.Capabilities(),
		Detected:     detected,
	}, nil
}

// =============================================================================
// Serialization Helpers
// =============================================================================

// SerializeMCPServerDefinition returns the JSON representation of the Cosca MCP server definition.
func SerializeMCPServerDefinition() (string, error) {
	def := GenerateMCPServerDefinition()
	data, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serialize MCP definition: %w", err)
	}
	return string(data), nil
}

// SerializeMCPConfig returns the JSON representation of the MCP client configuration.
func SerializeMCPConfig(coscaBinPath string) (string, error) {
	cfg := GenerateMCPConfig(coscaBinPath)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serialize MCP config: %w", err)
	}
	return string(data), nil
}

// =============================================================================
// Internal Helpers
// =============================================================================

// formatToolList creates a readable list of tools from MCP tool definitions.
func formatToolList(tools []MCPToolDefinition) string {
	lines := make([]string, 0, len(tools))
	for _, t := range tools {
		lines = append(lines, fmt.Sprintf("- **%s**: %s", t.Name, t.Description))
	}
	return strings.Join(lines, "\n")
}
