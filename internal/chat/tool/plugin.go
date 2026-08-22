// Package tool provides the tool registry and individual tool implementations
// for the Cosca chat agent system.
package tool

import (
	"context"
	"encoding/json"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/plugins"
)

// PluginTool wraps a plugins.Plugin as a chat.Tool, enabling WASM plugins
// (and other plugin runtimes) to be registered and invoked through the
// standard tool registry and executor.
type PluginTool struct {
	plugin plugins.Plugin
	schema json.RawMessage
}

// NewPluginTool creates a new PluginTool that adapts the given plugin to the
// chat.Tool interface. The default schema accepts any JSON object. Plugin
// implementations that define a ConfigSchema in their manifest may override
// this by calling SetSchema after construction.
func NewPluginTool(plugin plugins.Plugin) *PluginTool {
	return &PluginTool{
		plugin: plugin,
		schema: json.RawMessage(`{"type":"object","properties":{}}`),
	}
}

// SetSchema overrides the JSON Schema returned by Schema(). This is useful
// when a plugin provides a parameter schema in its manifest.
func (pt *PluginTool) SetSchema(schema json.RawMessage) {
	pt.schema = schema
}

// Name returns the plugin's human-readable name, which serves as the tool
// identifier for LLM function calling.
func (pt *PluginTool) Name() string { return pt.plugin.Name() }

// Description returns the plugin description from its manifest.
func (pt *PluginTool) Description() string { return pt.plugin.Description() }

// Schema returns the JSON Schema describing the plugin tool's parameters.
// By default this is a minimal object schema; call SetSchema to supply a
// richer schema derived from the plugin manifest.
func (pt *PluginTool) Schema() json.RawMessage { return pt.schema }

// Execute delegates to the underlying plugin's Run method. The JSON-encoded
// params are passed as a string to the plugin. The plugin's string result is
// returned in ToolResult.Output.
//
// Following the convention used throughout the tool package, execution errors
// are returned via ToolResult.Error with a nil error return, reserving the
// error return for infrastructure failures.
func (pt *PluginTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	output, err := pt.plugin.Run(ctx, string(params))
	if err != nil {
		return &chat.ToolResult{Error: err.Error()}, nil
	}
	return &chat.ToolResult{Output: output}, nil
}

// Validate is a no-op that always passes. Parameter validation is delegated
// to the plugin's own validation logic during Run. Tools that require strict
// schema validation should override this method.
func (pt *PluginTool) Validate(_ json.RawMessage) error {
	return nil
}

// compile-time interface checks
var _ chat.Tool = (*PluginTool)(nil)
