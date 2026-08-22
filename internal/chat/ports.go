// Package chat provides the core ports (interfaces) and supporting types for the
// Cosca Chat agent system. This file defines the hexagonal-architecture boundary:
// the "input" ports that the core engine depends on, with adapters implemented in
// sub-packages (agent/, tool/, provider/, memory/, sandbox/).
//
// Design principles:
//   - MCP-first tool system — all tools are discoverable, schema-driven, sandboxed
//   - Multi-provider LLM access — any provider (OpenAI, Anthropic, DeepSeek, Ollama)
//     with a uniform streaming interface
//   - MAG memory pattern — semantic memory with FTS5 + vector search
//   - OS-enforced sandbox — bwrap + seccomp for safe code execution
//   - Agent hierarchy — composable agents with system identity, capabilities, and
//     subagent spawning
package chat

import (
	"context"
	"encoding/json"
	"time"
)

// ─── Agent ───────────────────────────────────────────────────────────────────

// Agent represents a Cosca agent with identity, capabilities, and the ability
// to process requests. Agents form a hierarchy (Kernel → CEO → CTO → Chiefs)
// and can be spawned as subagents with isolated context.
type Agent interface {
	// Name returns the agent's logical name (e.g. "cosca-architecture").
	Name() string

	// SystemPrompt returns the agent's system-level instruction prompt that
	// defines its identity, role, and behavioral guardrails.
	SystemPrompt() string

	// Capabilities returns the list of capability names this agent supports.
	// Used for routing and skill discovery.
	Capabilities() []string

	// Run executes an agent request within the given context and returns the
	// agent's response including content, tool calls, and token usage.
	Run(ctx context.Context, req AgentRequest) (*AgentResponse, error)
}

// ─── Tool ────────────────────────────────────────────────────────────────────

// Tool represents a tool that an LLM can invoke. Tools are MCP-first: every
// tool has a JSON Schema describing its parameters, and execution goes through
// the sandbox gate. Built-in tools (filesystem, shell, search, git, etc.) and
// external MCP servers all implement this interface.
type Tool interface {
	// Name returns the tool's unique identifier (e.g. "filesystem_read").
	Name() string

	// Description returns a human-readable description of what the tool does.
	// This is included in the LLM's tool definition for function calling.
	Description() string

	// Schema returns the JSON Schema that describes the tool's parameters.
	// Used by the LLM to understand how to call the tool correctly.
	Schema() json.RawMessage

	// Execute runs the tool with the given JSON-encoded parameters and returns
	// the result. Execution is subject to sandbox enforcement and permission
	// classification.
	Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error)

	// Validate checks whether the given JSON-encoded parameters conform to the
	// tool's schema. Returns an error describing the violation if invalid.
	Validate(params json.RawMessage) error
}

// ─── Provider ────────────────────────────────────────────────────────────────

// Provider represents an LLM provider (OpenAI, Anthropic, DeepSeek, Ollama, etc.)
// that can generate chat completions. All providers expose a unified streaming
// interface: Chat returns a channel of events that delivers delta tokens, the
// final completion, or an error.
type Provider interface {
	// Name returns the provider's logical name (e.g. "openai", "anthropic").
	Name() string

	// Chat sends a chat completion request and returns a channel of events for
	// streaming consumption. When streaming is enabled via ChatRequest.Stream,
	// the channel delivers delta tokens in real time. The channel is closed
	// after a Done event or on error.
	Chat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error)

	// Models returns the list of model identifiers available from this provider
	// (e.g. ["gpt-4o", "gpt-4o-mini"]).
	Models() []string

	// IsAvailable reports whether the provider is reachable and configured
	// (e.g. API key present, endpoint reachable).
	IsAvailable() bool
}

// ─── Memory ──────────────────────────────────────────────────────────────────

// Memory represents the semantic memory store following the MAG (Memory-Augmented
// Generation) pattern. It supports storing, searching (semantic + FTS5), and
// forgetting entries, enabling cross-session and cross-agent knowledge retention.
type Memory interface {
	// Store persists a memory entry into the store.
	Store(ctx context.Context, entry MemoryEntry) error

	// Search performs a combined semantic + FTS5 search for entries matching the
	// given query string. Results are ranked by relevance score. Limit caps the
	// maximum number of returned entries.
	Search(ctx context.Context, query string, limit int) ([]MemoryEntry, error)

	// Forget removes a memory entry by its unique identifier.
	Forget(ctx context.Context, id string) error

	// Stats returns the total number of stored memory entries.
	Stats() (int, error)
}

// ─── Sandbox ─────────────────────────────────────────────────────────────────

// Sandbox provides OS-enforced execution isolation using bwrap + seccomp on Linux.
// It controls what files, network, and system calls a command can access based on
// the selected SandboxMode.
type Sandbox interface {
	// Execute runs a command within the sandbox under the given mode and returns
	// the result including stdout, stderr, exit code, and duration.
	Execute(ctx context.Context, cmd Command, mode SandboxMode) (*SandboxResult, error)

	// Mode returns the current sandbox mode (read-only, workspace, or full).
	Mode() SandboxMode

	// ValidatePath checks whether the given path is accessible under the current
	// sandbox mode. Returns an error if the path is outside the allowed scope.
	ValidatePath(path string) error
}

// ─── Supporting Types ────────────────────────────────────────────────────────

// AgentRequest encapsulates the inputs for an agent execution.
type AgentRequest struct {
	// Messages is the conversation history including the user's latest input.
	Messages []Message

	// Context carries additional contextual data (project config, knowledge,
	// agent hierarchy, session state, etc.).
	Context map[string]any

	// Tools is the set of tools available to the agent for this request.
	Tools []Tool
}

// AgentResponse encapsulates the output of an agent execution.
type AgentResponse struct {
	// Content is the text response generated by the agent.
	Content string

	// ToolCalls lists any tool invocations the agent requested.
	ToolCalls []ToolCall

	// Usage reports the token consumption for this agent execution.
	Usage Usage
}

// ChatRequest encapsulates the inputs for an LLM provider chat completion.
type ChatRequest struct {
	// Model identifies the model to use (e.g. "gpt-4o", "claude-sonnet-5").
	Model string

	// Messages is the conversation history to send to the model.
	Messages []Message

	// Tools defines the set of tools available for function calling.
	Tools []Tool

	// Stream enables token-by-token streaming via the returned event channel.
	Stream bool
}

// ChatEventType categorises a streaming chat event.
type ChatEventType string

const (
	// ChatEventDelta carries a partial delta token from the stream.
	ChatEventDelta ChatEventType = "delta"
	// ChatEventDone signals that the stream has completed successfully.
	ChatEventDone ChatEventType = "done"
	// ChatEventError signals that the stream encountered an error.
	ChatEventError ChatEventType = "error"
)

// ChatEvent represents a single event in a streaming chat response. The Type
// field discriminates the event kind:
//   - delta: Delta carries the incremental text token.
//   - done:  Usage carries the final token counts; no more events follow.
//   - error: Error carries the failure; no more events follow.
type ChatEvent struct {
	Type  ChatEventType `json:"type"`
	Delta string        `json:"delta,omitempty"`
	Error error         `json:"error,omitempty"`
	Usage *Usage        `json:"usage,omitempty"`
}

// ToolResult contains the outcome of a tool execution.
type ToolResult struct {
	// Output is the textual output produced by the tool.
	Output string `json:"output"`

	// Error describes any error that occurred during execution. Empty on success.
	Error string `json:"error,omitempty"`

	// Duration is the wall-clock time the tool took to execute.
	Duration time.Duration `json:"duration"`
}

// MemoryEntry represents a single entry in the semantic memory store.
type MemoryEntry struct {
	// ID is the unique identifier for this memory entry.
	ID string `json:"id"`

	// Content is the textual content of the memory.
	Content string `json:"content"`

	// Metadata carries arbitrary key-value pairs associated with this entry
	// (e.g. source agent, session ID, timestamp, tags).
	Metadata map[string]any `json:"metadata,omitempty"`

	// Score is the relevance score (0.0–1.0) from the most recent search.
	Score float64 `json:"score,omitempty"`
}

// Command describes an OS command to execute inside the sandbox.
type Command struct {
	// Args is the command and its arguments (e.g. ["ls", "-la"]).
	Args []string `json:"args"`

	// Env is a set of environment variables to set for the command.
	Env map[string]string `json:"env,omitempty"`

	// WorkDir is the working directory for the command. If empty, defaults to
	// the workspace root.
	WorkDir string `json:"work_dir,omitempty"`
}

// SandboxMode defines the level of sandbox isolation for command execution.
type SandboxMode int

const (
	// SandboxReadOnly restricts access to read-only operations within the
	// workspace. Network is off, shell execution is disabled.
	SandboxReadOnly SandboxMode = iota

	// SandboxWorkspace allows read/write access within the workspace (except
	// .git/). Network is off by default (opt-in). Shell is allowed but confined
	// to the workspace. This is the default mode.
	SandboxWorkspace

	// SandboxFull grants unrestricted access: any file, network on, shell
	// unrestricted. Requires explicit user approval.
	SandboxFull
)

// String returns a human-readable name for the sandbox mode.
func (m SandboxMode) String() string {
	switch m {
	case SandboxReadOnly:
		return "read-only"
	case SandboxWorkspace:
		return "workspace"
	case SandboxFull:
		return "full"
	default:
		return "unknown"
	}
}

// SandboxResult contains the outcome of a sandboxed command execution.
type SandboxResult struct {
	// Stdout is the standard output produced by the command.
	Stdout string `json:"stdout"`

	// Stderr is the standard error output produced by the command.
	Stderr string `json:"stderr"`

	// ExitCode is the process exit code. Zero indicates success.
	ExitCode int `json:"exit_code"`

	// Duration is the wall-clock time the command took to execute.
	Duration time.Duration `json:"duration"`
}
