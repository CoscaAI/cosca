// Package engine implements the core Agent Engine — the central loop that
// orchestrates context building, LLM routing, tool execution, subagent spawning,
// session management, and auto-compaction for the Cosca Chat CLI.
//
// Architecture (from next-gen-cli-design.md):
//
//	User Input → ContextBuilder → Router → LLM Call → ResponseParser → Tool Executor → Verify → Loop
//	                                        ↕                              ↕
//	                                  SessionManager                  SubagentSpawner
//	                                        ↕
//	                                  AutoCompaction
package engine

import (
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/modlink"
)

// ─── Session ──────────────────────────────────────────────────────────────────

// Session represents a persistent chat session. Sessions are saved as JSONL
// in .cosca/sessions/ and support resume, fork, and search operations.
type Session struct {
	// ID is a unique identifier for this session (UUID v4).
	ID string `json:"id"`

	// Model is the model identifier used for this session (e.g. "gpt-4o").
	Model string `json:"model"`

	// Agent is the primary agent name for this session.
	Agent string `json:"agent,omitempty"`

	// Messages is the full conversation history.
	Messages []chat.Message `json:"messages"`

	// CreatedAt is when the session was first created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the session was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// TokenUsage tracks cumulative token consumption for the session.
	TokenUsage chat.Usage `json:"token_usage,omitempty"`

	// Metadata carries arbitrary key-value pairs (e.g. project path, git branch).
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ─── Engine Config ────────────────────────────────────────────────────────────

// EngineConfig configures the Agent Engine for a single run or session.
type EngineConfig struct {
	// Model is the LLM model identifier (e.g. "gpt-4o", "claude-sonnet-5").
	Model string `json:"model"`

	// Agent is the agent name to use (e.g. "cosca-kernel"). Empty means auto-route.
	Agent string `json:"agent,omitempty"`

	// MaxTokens is the maximum tokens per LLM call. 0 = model default.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness (0.0–2.0). Default 0.7.
	Temperature float64 `json:"temperature,omitempty"`

	// MaxTurns limits the number of agent loop iterations. 0 = unlimited.
	MaxTurns int `json:"max_turns,omitempty"`

	// Stream enables token-by-token streaming to the output.
	Stream bool `json:"stream,omitempty"`

	// Ephemeral when true skips session persistence.
	Ephemeral bool `json:"ephemeral,omitempty"`

	// Budget is the cognitive budget that gates LLM calls (nil = disabled,
	// current behavior unchanged). When set, the engine checks CanCall before
	// each model call and records tokens/time/cost after each one.
	Budget *CognitiveBudget `json:"budget,omitempty"`

	// HaltChecker é o kill-switch do kernel (interface mínima IsHalted).
	// Quando não-nil, o engine verifica ANTES de cada chamada LLM — se o
	// kernel foi haltado, a chamada é bloqueada com erro claro (o botão de
	// emergência protege o caminho que mais gasta tokens). Nil = sem check
	// (comportamento atual).
	HaltChecker HaltChecker `json:"-"`
}

// HaltChecker é a interface mínima do kill-switch. O kernel.EmergencyManager
// a satisfaz — o engine conhece apenas a interface, sem depender do kernel.
type HaltChecker interface {
	IsHalted() bool
}

// ─── Engine Result ────────────────────────────────────────────────────────────

// EngineResult is the final result produced by the Agent Engine after completing
// all turns of the agent loop (or stopping on error/limit).
type EngineResult struct {
	// Content is the final text response to the user.
	Content string `json:"content"`

	// Messages is the complete conversation history including all turns.
	Messages []chat.Message `json:"messages"`

	// TokenUsage reports cumulative token consumption.
	TokenUsage chat.Usage `json:"token_usage"`

	// TurnCount is the number of agent loop iterations executed.
	TurnCount int `json:"turn_count"`

	// SessionID links this result to a persisted session, if applicable.
	SessionID string `json:"session_id,omitempty"`

	// Error describes any terminal error that stopped the engine. Empty on success.
	Error string `json:"error,omitempty"`

	// Turns records per-turn telemetry (turn number, tool calls, tokens, duration).
	Turns []TurnRecord `json:"turns,omitempty"`

	// Budget reports the cognitive budget spent by this run, when a budget
	// was configured (nil otherwise).
	Budget *BudgetSpent `json:"budget,omitempty"`
}

// ─── Built Context ────────────────────────────────────────────────────────────

// BuiltContext is the fully assembled context for an LLM call, produced by
// ContextBuilder. It contains everything the LLM needs: system identity,
// project instructions, relevant memories, knowledge, skills, tools, and
// conversation history.
type BuiltContext struct {
	// SystemPrompt is the assembled system instruction for this turn.
	SystemPrompt string `json:"system_prompt"`

	// Messages is the conversation history including the user's input.
	Messages []chat.Message `json:"messages"`

	// Tools is the set of tools available for this turn.
	Tools []chat.Tool `json:"tools,omitempty"`

	// ToolDefinitions is the OpenAI-compatible tool definitions for the LLM.
	ToolDefinitions []chat.ToolDefinition `json:"tool_definitions,omitempty"`

	// TokenEstimate is the approximate token count of the built context.
	TokenEstimate int `json:"token_estimate,omitempty"`

	// ContextLimit is the model's context window limit in tokens.
	ContextLimit int `json:"context_limit,omitempty"`

	// UsagePct is TokenEstimate / ContextLimit * 100.
	UsagePct float64 `json:"usage_pct,omitempty"`

	// KnowledgeNoRoute (FASE 1 routing/scope) sinaliza que a pesquisa de
	// conhecimento entrou em estado NO_ROUTE no modo modular: o roteador
	// determinístico não encontrou um espaço semântico confiável para a consulta,
	// o retrieval foi 0 e nenhum full-scan foi feito. True mantém `knowledge`
	// vazio e injeta uma linha curta de escopo no SystemPrompt.
	KnowledgeNoRoute bool `json:"knowledge_no_route,omitempty"`

	// ScopeInfo, quando não-nil, carrega o *modlink.SearchScope decidido pelo
	// roteador (módulos/capacidades/NoRoute) — o "onde" da busca confinada.
	// Opcional: nil quando a busca não roteou (legacy) ou o sinal não foi
	// propagado pelo adapter.
	ScopeInfo *modlink.SearchScope `json:"scope_info,omitempty"`
}

// ─── Subagent Request / Result ────────────────────────────────────────────────

// SubagentRequest encapsulates the inputs for spawning a subagent.
type SubagentRequest struct {
	// Agent is the registered agent name to spawn (e.g. "cosca-database").
	Agent string `json:"agent"`

	// Task is the specific task or question for the subagent.
	Task string `json:"task"`

	// Context carries relevant context from the parent agent (files, schema, etc.).
	Context map[string]string `json:"context,omitempty"`

	// MaxTokens limits the subagent's response length.
	MaxTokens int `json:"max_tokens,omitempty"`
}

// SubagentResult is the summary returned by a subagent after completing its task.
type SubagentResult struct {
	// Summary is the condensed result of the subagent's work.
	Summary string `json:"summary"`

	// TokenUsage reports the subagent's token consumption.
	TokenUsage chat.Usage `json:"token_usage"`

	// Error is non-empty if the subagent failed.
	Error string `json:"error,omitempty"`

	// ErrorCode is a stable, non-sensitive category for Error.
	ErrorCode string `json:"error_code,omitempty"`
}

// ─── Compaction Result ────────────────────────────────────────────────────────

// CompactionResult describes the outcome of an auto-compaction operation.
type CompactionResult struct {
	// Compacted when true means messages were summarised/truncated.
	Compacted bool `json:"compacted"`

	// Summary is the generated summary of the compacted portion.
	Summary string `json:"summary,omitempty"`

	// MessagesRemaining is the count of messages kept after compaction.
	MessagesRemaining int `json:"messages_remaining"`

	// TokensSaved is the estimated number of tokens freed by compaction.
	TokensSaved int `json:"tokens_saved,omitempty"`
}

// ─── Route Result ─────────────────────────────────────────────────────────────

// RouteResult describes the outcome of routing a user request to an agent.
type RouteResult struct {
	// Agent is the selected agent name.
	Agent string `json:"agent"`

	// Confidence is the routing confidence score (0.0–1.0).
	Confidence float64 `json:"confidence"`

	// Method describes how the route was determined (keyword, semantic, fallback).
	Method string `json:"method"`
}

// ─── Turn Record ──────────────────────────────────────────────────────────────

// TurnRecord captures a single turn in the agent loop for logging and debugging.
type TurnRecord struct {
	// TurnNumber is the sequential turn index (1-based).
	TurnNumber int `json:"turn_number"`

	// ToolCallsMade is the number of tool calls in this turn.
	ToolCallsMade int `json:"tool_calls_made"`

	// TokensUsed is the token consumption for this turn.
	TokensUsed int `json:"tokens_used"`

	// DurationMs is how long this turn took.
	DurationMs int64 `json:"duration_ms"`
}
