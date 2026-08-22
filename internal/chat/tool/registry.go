// Package tool provides the tool registry for managing, discovering, and
// executing tools within the Cosca chat agent system. It implements the
// MCP-first tool system where all tools are discoverable, schema-driven,
// and sandboxed.
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
)

// Registry manages tool registration, discovery, and execution. It is
// thread-safe and supports concurrent read/write operations using a
// sync.RWMutex.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]chat.Tool
}

// NewRegistry creates and returns a new empty Registry ready for use.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]chat.Tool),
	}
}

// Register adds a tool to the registry. If a tool with the same name
// already exists, it is silently replaced. This operation is thread-safe.
func (r *Registry) Register(t chat.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Get returns a tool by name. Returns nil if no tool is found with the
// given name. This operation is thread-safe.
func (r *Registry) Get(name string) chat.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List returns all registered tool names sorted alphabetically. Returns
// an empty slice if no tools are registered. This operation is thread-safe.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetAll returns all registered tools. The order of the returned slice is
// non-deterministic. Returns an empty slice if no tools are registered.
// This operation is thread-safe.
func (r *Registry) GetAll() []chat.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]chat.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		all = append(all, t)
	}
	return all
}

// Execute finds a tool by name, validates its parameters, and executes it.
// Returns a ToolResult containing the output, any error, and the execution
// duration. If the tool is not found, the result's Error field is set to
// "tool not found: {name}". This operation is thread-safe for lookups;
// the actual tool execution happens outside the lock.
func (r *Registry) Execute(ctx context.Context, name string, params json.RawMessage) *chat.ToolResult {
	tool := r.Get(name)
	if tool == nil {
		return &chat.ToolResult{
			Error: fmt.Sprintf("tool not found: %s", name),
		}
	}

	start := time.Now()
	result, err := tool.Execute(ctx, params)
	elapsed := time.Since(start)

	if err != nil {
		return &chat.ToolResult{
			Error:    err.Error(),
			Duration: elapsed,
		}
	}

	return &chat.ToolResult{
		Output:   result.Output,
		Duration: elapsed,
	}
}

// Definitions returns all registered tools as a slice of ToolDefinition
// in the format required for LLM function calling (OpenAI-compatible).
// Each definition includes the tool name, description, and JSON Schema
// parameters. If a tool's schema cannot be unmarshalled, an empty parameter
// map is used instead. This operation is thread-safe.
func (r *Registry) Definitions() []chat.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Sort by name for deterministic output
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	defs := make([]chat.ToolDefinition, 0, len(r.tools))
	for _, name := range names {
		t := r.tools[name]
		var params map[string]any
		if err := json.Unmarshal(t.Schema(), &params); err != nil {
			params = make(map[string]any)
		}

		defs = append(defs, chat.ToolDefinition{
			Type: "function",
			Function: chat.FunctionDef{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  params,
			},
		})
	}
	return defs
}

// Remove unregisters a tool by name. If the tool does not exist, this
// operation is a no-op. This operation is thread-safe.
func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// Size returns the number of registered tools. This operation is thread-safe.
func (r *Registry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
