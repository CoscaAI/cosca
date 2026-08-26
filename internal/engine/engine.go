package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/executor"
	"github.com/CoscaAI/cosca/internal/contenttrust"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/plugins"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/safeerror"
	"github.com/rs/zerolog/log"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Cross-package interfaces (DI boundaries)
// ═══════════════════════════════════════════════════════════════════════════════

// ProviderChat is the subset of chat.ChatProvider that the engine requires.
type ProviderChat interface {
	Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error)
	ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error)
	Name() string
}

// ToolExecutor is the subset of executor.Executor that the engine requires.
type ToolExecutor interface {
	Execute(ctx context.Context, toolCall executor.ToolCall) (*executor.ToolResult, error)
	ExecuteBatch(ctx context.Context, toolCalls []executor.ToolCall, opts ...executor.BatchOption) []*executor.ToolResult
	ListTools(ctx context.Context) ([]chat.ToolDefinition, error)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Constants
// ═══════════════════════════════════════════════════════════════════════════════

// SubagentSpawnToolName is the tool name that triggers subagent spawning.
// When the LLM issues a tool call with this name, the engine intercepts it
// and spawns a subagent instead of routing it to the ToolExecutor.
const SubagentSpawnToolName = "spawn_agent"

// DefaultTemperature is the default LLM temperature when not specified in config.
const DefaultTemperature = 0.7

// CompactionThresholdPct is the context usage percentage above which
// auto-compaction is triggered.
const CompactionThresholdPct = 80.0

// ═══════════════════════════════════════════════════════════════════════════════
// Engine Event Types (streaming output)
// ═══════════════════════════════════════════════════════════════════════════════

// EngineEventType categorises a streaming engine event.
type EngineEventType string

const (
	// EngineEventContent carries a text delta from the LLM.
	EngineEventContent EngineEventType = "content"
	// EngineEventToolStart signals that a tool is about to execute.
	EngineEventToolStart EngineEventType = "tool_start"
	// EngineEventToolResult carries the result of a tool execution.
	EngineEventToolResult EngineEventType = "tool_result"
	// EngineEventSubagentStart signals that a subagent is being spawned.
	EngineEventSubagentStart EngineEventType = "subagent_start"
	// EngineEventSubagentResult carries the subagent's summary.
	EngineEventSubagentResult EngineEventType = "subagent_result"
	// EngineEventTurnEnd signals the end of a single agent loop turn.
	EngineEventTurnEnd EngineEventType = "turn_end"
	// EngineEventDone signals that the engine has finished processing.
	EngineEventDone EngineEventType = "done"
	// EngineEventError signals a terminal error.
	EngineEventError EngineEventType = "error"
)

// EngineEvent represents a single event emitted by RunStream.
type EngineEvent struct {
	Type           EngineEventType
	Content        string
	ToolCall       *executor.ToolCall
	ToolResult     *executor.ToolResult
	SubagentResult *SubagentResult
	TurnRecord     *TurnRecord
	Error          error
}

// ═══════════════════════════════════════════════════════════════════════════════
// AgentEngine
// ═══════════════════════════════════════════════════════════════════════════════

// AgentEngine implements the core agent loop as described in the architecture:
//
//	User Input → ContextBuilder → Router → LLM Call → ResponseParser →
//	Tool Executor / SubagentSpawner → Verify → Loop
//
// The engine is fully DI-friendly: all dependencies are injected via the
// constructor. Concrete engine-internal types (*AgentRegistry, *ContextBuilder,
// *Router, *SessionManager) are used directly since they live in the same
// package. Cross-package dependencies use interfaces (ProviderChat, ToolExecutor).
type AgentEngine struct {
	registry        *AgentRegistry
	contextBldr     *ContextBuilder
	router          *Router
	parser          *ResponseParser
	spawner         *SubagentSpawner
	provider        ProviderChat
	executor        ToolExecutor
	sessionMgr      *SessionManager // optional
	config          EngineConfig
	memoryRetriever MemoryRetriever     // nil = skip memory retrieval
	memoryStorer    MemoryStorer        // nil = skip memory storage
	knowledge       KnowledgeSearcher   // nil = skip knowledge search
	hooks           plugins.HookManager // optional hook system
	autoCompaction  *AutoCompaction     // context-window auto-compaction
	onClose         []func()            // resource cleanup hooks (Close)
}

// NewAgentEngine creates a new AgentEngine with the given dependencies.
//
// Parameters:
//   - registry: agent registry for lookups and routing
//   - contextBldr: builds LLM context (system prompt + history + tools)
//   - router: routes user input to the appropriate agent
//   - provider: LLM chat provider
//   - executor: tool executor for running tool calls
//   - config: engine configuration (model, max turns, etc.)
//   - memoryRetriever: optional memory retriever for pre-turn memory search (nil to skip)
//   - memoryStorer: optional memory storer for post-turn memory persistence (nil to skip)
//   - knowledge: optional knowledge searcher for pre-turn knowledge retrieval (nil to skip)
//
// The parser and spawner are created internally. SessionManager is nil initially
// and can be set via WithSessionManager.
func NewAgentEngine(
	registry *AgentRegistry,
	contextBldr *ContextBuilder,
	router *Router,
	provider ProviderChat,
	executor ToolExecutor,
	config EngineConfig,
	memoryRetriever MemoryRetriever,
	memoryStorer MemoryStorer,
	knowledge KnowledgeSearcher,
) *AgentEngine {
	maxTokens := DefaultMaxTokens
	if contextBldr != nil && contextBldr.maxTokens > 0 {
		maxTokens = contextBldr.maxTokens
	}
	return &AgentEngine{
		registry:        registry,
		contextBldr:     contextBldr,
		router:          router,
		parser:          NewResponseParser(),
		spawner:         NewSubagentSpawner(registry, provider, executor),
		provider:        provider,
		executor:        executor,
		sessionMgr:      nil,
		config:          config,
		memoryRetriever: memoryRetriever,
		memoryStorer:    memoryStorer,
		knowledge:       knowledge,
		autoCompaction:  NewAutoCompaction(maxTokens),
	}
}

// WithAutoCompaction attaches a custom AutoCompaction instance, overriding the
// default one created by NewAgentEngine. Passing nil disables auto-compaction.
func (e *AgentEngine) WithAutoCompaction(ac *AutoCompaction) *AgentEngine {
	e.autoCompaction = ac
	return e
}

// WithSessionManager attaches an optional SessionManager for persistence.
func (e *AgentEngine) WithSessionManager(sm *SessionManager) *AgentEngine {
	e.sessionMgr = sm
	return e
}

// WithPlugins attaches an optional plugin hook manager. When set, the engine
// triggers pre_tool_use and post_tool_use hooks around every tool execution.
func (e *AgentEngine) WithPlugins(hooks plugins.HookManager) *AgentEngine {
	e.hooks = hooks
	return e
}

// WithConfigMaxTurns overrides the maximum number of agent loop turns.
// If n <= 0 the existing value is kept.
func (e *AgentEngine) WithConfigMaxTurns(n int) *AgentEngine {
	if n > 0 {
		e.config.MaxTurns = n
	}
	return e
}

// OnClose registers a cleanup hook (ex: closing an underlying memory/knowledge
// engine) to be run when Close is called. Multiple hooks run in reverse order.
func (e *AgentEngine) OnClose(fn func()) *AgentEngine {
	e.onClose = append(e.onClose, fn)
	return e
}

// Close releases resources registered via OnClose (reverse order). Idempotent
// and safe to call multiple times; no-op when no hooks are registered.
func (e *AgentEngine) Close() {
	for i := len(e.onClose) - 1; i >= 0; i-- {
		e.onClose[i]()
	}
	e.onClose = nil
}

// WithAgent pins the engine to a specific agent (bypassing the router).
// An empty name keeps the default auto-routing behaviour.
func (e *AgentEngine) WithAgent(agentName string) *AgentEngine {
	e.config.Agent = agentName
	return e
}

// SetProvider swaps the LLM provider at runtime (e.g. from the model picker
// in the TUI). It updates both the engine and the subagent spawner, so the
// next Run/RunStream call uses the new provider. The engine's config model
// is also updated to reflect the new provider's default model.
func (e *AgentEngine) SetProvider(p ProviderChat) {
	if p == nil {
		return
	}
	e.provider = p
	if e.spawner != nil {
		e.spawner.SetProvider(p)
	}
	if model := p.Name(); model != "" {
		e.config.Model = model
	}
}

// Provider returns the currently active LLM provider.
func (e *AgentEngine) Provider() ProviderChat {
	return e.provider
}

// ─── Run (synchronous) ─────────────────────────────────────────────────────────

// Run executes the agent loop synchronously and returns the final result.
//
// The loop:
//  1. Routes user input to the appropriate agent (if not pre-configured).
//  2. Builds context via ContextBuilder.
//  3. Calls the LLM via provider.
//  4. Parses the response for content and tool calls.
//  5. If tool calls → executes them (or spawns subagents) and feeds results back.
//  6. Repeats until no more tool calls, max turns reached, or error.
//  7. Auto-compacts the conversation history when context usage exceeds the threshold.
//  8. Saves session if SessionManager is available.
func (e *AgentEngine) Run(ctx context.Context, userInput string, history []chat.Message) (*EngineResult, error) {
	startTime := time.Now()

	// Cognitive budget (opt-in): when EngineConfig.Budget is nil the tracker
	// is nil too and the loop below behaves exactly as before (no regression).
	var budgetTracker *BudgetTracker
	if e.config.Budget != nil {
		budgetTracker = NewBudgetTracker(*e.config.Budget)
	}

	// ── 1. Determine agent ──────────────────────────────────────────────
	agentName := e.config.Agent
	if agentName == "" {
		routeResult := e.router.Route(ctx, userInput, history)
		agentName = routeResult.Agent
	}

	// ── 2a. Pre-turn: Retrieve memories and knowledge ────────────────────
	var memories []string
	var knowledge []string
	var knowledgeNoRoute bool
	var knowledgeScope *modlink.SearchScope

	if e.memoryRetriever != nil {
		results, err := e.memoryRetriever.Search(ctx, userInput, MemorySearchOptions{
			Limit:  5,
			Layers: []string{"session", "workspace"},
		})
		if err == nil && len(results) > 0 {
			for _, r := range results {
				memories = append(memories, r.Content)
			}
		}
	}

	if e.knowledge != nil {
		results, err := e.knowledge.Search(ctx, KnowledgeSearchParams{
			Query: userInput,
			Limit: 3,
		})
		if err == nil {
			if results.NoRoute {
				// FASE 1: NO_ROUTE no modo modular — semantic retrieval é 0
				// (sem full-scan silencioso); o sinal é propagado para o contexto.
				knowledgeNoRoute = true
				knowledgeScope = results.Scope
			} else if len(results.Results) > 0 {
				for _, r := range results.Results {
					knowledge = append(knowledge, r.Content)
				}
			}
		}
	}

	// ── 2b. Build context ─────────────────────────────────────────────
	// Prepend user input to history for context builder.
	msgs := make([]chat.Message, len(history), len(history)+1)
	copy(msgs, history)
	msgs = append(msgs, chat.Message{
		Role:    chat.RoleUser,
		Content: userInput,
	})

	builtCtx := e.contextBldr.Build(ctx, agentName, msgs, nil, memories, knowledge)
	builtCtx = applyKnowledgeNoRoute(builtCtx, knowledgeNoRoute, knowledgeScope)

	// ── 3. Session management ───────────────────────────────────────────
	var sessionID string
	if e.sessionMgr != nil && !e.config.Ephemeral {
		if len(history) == 0 {
			session := e.sessionMgr.CreateSession(e.config.Model, agentName)
			sessionID = session.ID
		}
	}

	// ── 4. Agent loop ──────────────────────────────────────────────────
	// ── Inject system prompt (identity + memory + knowledge + skills) ──
	llmMessages := withSystemPromptPrepended(builtCtx)

	turnCount := 0
	totalUsage := chat.Usage{}
	var lastContent string
	var turns []TurnRecord
	compactedThisRun := false

	for {
		select {
		case <-ctx.Done():
			return e.buildResult(lastContent, llmMessages, totalUsage, turnCount, sessionID, safeerror.Message("cancelled"), budgetTracker, turns...), nil
		default:
		}

		// Max turns check.
		if e.config.MaxTurns > 0 && turnCount >= e.config.MaxTurns {
			break
		}
		turnCount++

		turnStart := time.Now()

		// Build options for this turn.
		opts := e.buildChatOptions(builtCtx.ToolDefinitions)

		// Cognitive budget guard: if a budget is configured and the estimated
		// next call would exceed it, block the LLM call entirely and return a
		// clear error without invoking the provider.
		if budgetTracker != nil && !budgetTracker.CanCall() {
			return e.buildResult(lastContent, llmMessages, totalUsage, turnCount, sessionID,
				"budget cognitivo estourado (tokens/tempo/custo)", budgetTracker, turns...), nil
		}

		// LLM call.
		resp, err := e.provider.Chat(ctx, llmMessages, opts)
		if err != nil {
			return nil, safeerror.Error("llm_call_failed", err)
		}

		// Parse response.
		content, toolCalls, parseErr := e.parser.ParseResponse(resp)
		if parseErr != nil {
			return e.buildResult(lastContent, llmMessages, totalUsage, turnCount, sessionID,
				safeerror.Message("parse_failed"), budgetTracker, turns...), nil
		}

		// Track usage.
		if resp.Usage.TotalTokens > 0 {
			totalUsage.PromptTokens += resp.Usage.PromptTokens
			totalUsage.CompletionTokens += resp.Usage.CompletionTokens
			totalUsage.TotalTokens += resp.Usage.TotalTokens
		}

		// Record cognitive budget consumption for this LLM call. Cost stays 0
		// by default — a pricing hook (not implemented) would fill it in.
		if budgetTracker != nil {
			budgetTracker.Record(resp.Usage.TotalTokens, time.Since(turnStart), 0)
		}

		// Build assistant message.
		assistantMsg := chat.Message{
			Role:      chat.RoleAssistant,
			Content:   content,
			ToolCalls: toolCalls,
		}
		llmMessages = append(llmMessages, assistantMsg)

		// Post-turn: Store the interaction as a memory record.
		if e.memoryStorer != nil && content != "" {
			_, storeErr := e.memoryStorer.Store(ctx, MemoryRecord{
				Type:    "decision",
				Layer:   "session",
				Content: fmt.Sprintf("User: %s\nAssistant: %s", userInput, content),
				Metadata: map[string]string{
					"agent":   agentName,
					"session": sessionID,
					"model":   e.config.Model,
				},
				Priority: 1,
			})
			if storeErr != nil {
				// Log the failure but don't break the loop.
				info := safeerror.Inspect("memory_store_failed", storeErr)
				log.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Str("agent", agentName).Str("session", sessionID).Msg("memory store failed")
			}
		}

		// Persist assistant message.
		if e.sessionMgr != nil && sessionID != "" {
			_ = e.sessionMgr.AppendMessage(sessionID, assistantMsg)
		}

		// Track the last text content for the final result.
		if content != "" {
			lastContent = content
		}

		// If no tool calls, we're done with the loop.
		if len(toolCalls) == 0 {
			break
		}

		// Execute tool calls.
		turnToolCalls := 0
		for _, tc := range toolCalls {
			turnToolCalls++

			// Trigger pre_tool_use hook if plugin hooks are configured.
			if e.hooks != nil {
				_ = e.hooks.ExecuteHooks(plugins.HookPreToolUse, tc)
			}

			// Check for subagent spawn request.
			if e.isSubagentSpawn(tc) {
				subReq := e.parseSubagentRequest(tc)
				subResult, err := e.spawner.Spawn(ctx, subReq)
				if err != nil {
					llmMessages = append(llmMessages, chat.Message{
						Role:       chat.RoleTool,
						ToolCallID: tc.ID,
						Content:    contenttrust.Envelope(contenttrust.Default(contenttrust.OriginTool, safeerror.Message("subagent_spawn_failed"), tc.Function.Name)),
					})
					continue
				}

				// Build tool result message.
				resultContent := ""
				if subResult.Error != "" {
					resultContent = safeerror.ToolMessage(errors.New(subResult.Error))
				} else {
					resultContent = subResult.Summary
				}

				// Accumulate subagent token usage.
				if subResult.TokenUsage.TotalTokens > 0 {
					totalUsage.PromptTokens += subResult.TokenUsage.PromptTokens
					totalUsage.CompletionTokens += subResult.TokenUsage.CompletionTokens
					totalUsage.TotalTokens += subResult.TokenUsage.TotalTokens
				}

				llmMessages = append(llmMessages, chat.Message{
					Role:       chat.RoleTool,
					ToolCallID: tc.ID,
					Content:    contenttrust.Envelope(contenttrust.Default(contenttrust.OriginTool, resultContent, tc.Function.Name)),
				})
			} else {
				// Regular tool call — execute via executor.
				execTC, convErr := e.chatToolCallToExec(tc)
				if convErr != nil {
					llmMessages = append(llmMessages, chat.Message{
						Role:       chat.RoleTool,
						ToolCallID: tc.ID,
						Content:    contenttrust.Envelope(contenttrust.Default(contenttrust.OriginTool, safeerror.ToolMessage(convErr), tc.Function.Name)),
					})
					continue
				}

				result, execErr := e.executor.Execute(ctx, execTC)
				if execErr != nil {
					// Truly exceptional error (not tool failure, but infrastructure).
					return e.buildResult(lastContent, llmMessages, totalUsage, turnCount, sessionID,
						safeerror.Message("tool_execution_failed"), budgetTracker, turns...), nil
				}

				resultContent := contenttrust.Envelope(contenttrust.Default(contenttrust.OriginTool, e.formatExecResult(result), execTC.Name))
				llmMessages = append(llmMessages, chat.Message{
					Role:       chat.RoleTool,
					ToolCallID: tc.ID,
					Content:    resultContent,
				})
			}

			// Trigger post_tool_use hook if plugin hooks are configured.
			if e.hooks != nil {
				lastMsg := llmMessages[len(llmMessages)-1]
				_ = e.hooks.ExecuteHooks(plugins.HookPostToolUse, lastMsg)
			}
		}

		// Record turn.
		turns = append(turns, TurnRecord{
			TurnNumber:    turnCount,
			ToolCallsMade: turnToolCalls,
			TokensUsed:    totalUsage.TotalTokens,
			DurationMs:    time.Since(turnStart).Milliseconds(),
		})

		// Auto-compaction: when context usage exceeds the threshold, shorten the
		// conversation history before feeding tool results back to the LLM.
		if !compactedThisRun && builtCtx.UsagePct > CompactionThresholdPct {
			if newMsgs, ok := e.compactMessages(llmMessages); ok {
				before := len(llmMessages)
				llmMessages = newMsgs
				compactedThisRun = true
				log.Info().Int("turns_before", before).Int("turns_after", len(llmMessages)).Msg("context compacted")
			}
		}

		// Continue loop to feed tool results back to the LLM.
	}

	// ── 5. Finalise ─────────────────────────────────────────────────────
	if e.sessionMgr != nil && sessionID != "" && !e.config.Ephemeral {
		_ = e.sessionMgr.SaveSession(sessionID)
	}

	result := e.buildResult(lastContent, llmMessages, totalUsage, turnCount, sessionID, "", budgetTracker, turns...)
	_ = startTime // used in future metrics
	return result, nil
}

// ─── RunStream (streaming) ─────────────────────────────────────────────────────

// RunStream executes the agent loop with streaming output. Events are emitted
// on the returned channel as they occur: content deltas, tool starts/results,
// subagent results, and the final done/error signal.
//
// The caller must read from the channel until it is closed.
func (e *AgentEngine) RunStream(ctx context.Context, userInput string, history []chat.Message) (<-chan EngineEvent, error) {
	events := make(chan EngineEvent, 16)

	// ── 1. Determine agent ──────────────────────────────────────────────
	agentName := e.config.Agent
	if agentName == "" {
		routeResult := e.router.Route(ctx, userInput, history)
		agentName = routeResult.Agent
	}

		// ── 2a. Pre-turn: Retrieve memories and knowledge ────────────────────
		var memories []string
		var knowledge []string
		var knowledgeNoRoute bool
		var knowledgeScope *modlink.SearchScope

		if e.memoryRetriever != nil {
			results, err := e.memoryRetriever.Search(ctx, userInput, MemorySearchOptions{
				Limit:  5,
				Layers: []string{"session", "workspace"},
			})
			if err == nil && len(results) > 0 {
				for _, r := range results {
					memories = append(memories, r.Content)
				}
			}
		}

		if e.knowledge != nil {
			results, err := e.knowledge.Search(ctx, KnowledgeSearchParams{
				Query: userInput,
				Limit: 3,
			})
			if err == nil {
				if results.NoRoute {
					// FASE 1: NO_ROUTE no modo modular — semantic retrieval é 0
					// (sem full-scan silencioso); o sinal é propagado ao contexto.
					knowledgeNoRoute = true
					knowledgeScope = results.Scope
				} else if len(results.Results) > 0 {
					for _, r := range results.Results {
						knowledge = append(knowledge, r.Content)
					}
				}
			}
		}

		// ── 2b. Build context ─────────────────────────────────────────────
		msgs := make([]chat.Message, len(history), len(history)+1)
		copy(msgs, history)
		msgs = append(msgs, chat.Message{
			Role:    chat.RoleUser,
			Content: userInput,
		})

		builtCtx := e.contextBldr.Build(ctx, agentName, msgs, nil, memories, knowledge)
		builtCtx = applyKnowledgeNoRoute(builtCtx, knowledgeNoRoute, knowledgeScope)

	// ── 3. Session management ───────────────────────────────────────────
	var sessionID string
	if e.sessionMgr != nil && !e.config.Ephemeral {
		if len(history) == 0 {
			session := e.sessionMgr.CreateSession(e.config.Model, agentName)
			sessionID = session.ID
		}
	}

	// ── 4. Launch streaming goroutine ──────────────────────────────────
	go func() {
		defer close(events)
		emit := func(event EngineEvent) bool {
			if ctx.Err() != nil {
				// Preserve one terminal cancellation event for consumers that
				// start draining after cancellation; the bounded buffer makes
				// this non-blocking and never creates a sender goroutine.
				select {
				case events <- event:
					return true
				default:
					return false
				}
			}
			select {
			case events <- event:
				return true
			case <-ctx.Done():
				return false
			}
		}

		// Cognitive budget (opt-in): nil when EngineConfig.Budget is nil.
		var budgetTracker *BudgetTracker
		if e.config.Budget != nil {
			budgetTracker = NewBudgetTracker(*e.config.Budget)
		}

		// ── Inject system prompt (identity + memory + knowledge + skills) ──
		llmMessages := withSystemPromptPrepended(builtCtx)

		turnCount := 0
		totalUsage := chat.Usage{}
		compactedThisRun := false
		for {
			select {
			case <-ctx.Done():
				// Best-effort terminal notification. emit still selects ctx.Done,
				// so this cannot block or enqueue after cancellation when no
				// consumer is ready.
				emit(EngineEvent{Type: EngineEventError, Error: safeerror.Error("cancelled", ctx.Err())})
				return
			default:
			}

			// Max turns check.
			if e.config.MaxTurns > 0 && turnCount >= e.config.MaxTurns {
				if !emit(EngineEvent{Type: EngineEventDone}) {
					return
				}
				return
			}
			turnCount++

			turnStart := time.Now()

			// ── LLM streaming call ─────────────────────────────────
			opts := e.buildChatOptions(builtCtx.ToolDefinitions)
			opts.Stream = true

			// Cognitive budget guard: block the streaming call before it
			// starts if the estimated next call would exceed the budget.
			if budgetTracker != nil && !budgetTracker.CanCall() {
				if !emit(EngineEvent{
					Type:  EngineEventError,
					Error: errors.New("budget cognitivo estourado (tokens/tempo/custo)"),
				}) {
					return
				}
				return
			}

			stream, err := e.provider.ChatStream(ctx, llmMessages, opts)
			if err != nil {
				if !emit(EngineEvent{
					Type:  EngineEventError,
					Error: safeerror.Error("stream_start_failed", err),
				}) {
					return
				}
				return
			}

			// ── Parse stream ─────────────────────────────────────
			parseEvents, parseErr := e.parser.ParseStream(ctx, stream, builtCtx.ToolDefinitions)
			if parseErr != nil {
				if !emit(EngineEvent{
					Type:  EngineEventError,
					Error: safeerror.Error("parse_stream_failed", parseErr),
				}) {
					return
				}
				return
			}

			var contentBuilder strings.Builder
			var toolCalls []chat.ToolCall

			for pe := range parseEvents {
				switch pe.Type {
				case ParseEventContent:
					contentBuilder.WriteString(pe.Content)
					if !emit(EngineEvent{
						Type:    EngineEventContent,
						Content: pe.Content,
					}) {
						return
					}

				case ParseEventToolCall:
					if pe.ToolCall != nil {
						toolCalls = append(toolCalls, *pe.ToolCall)
					}

				case ParseEventDone:
					// Stream finished normally.

				case ParseEventError:
					if !emit(EngineEvent{
						Type:  EngineEventError,
						Error: safeerror.Error("stream_parse_failed", pe.Error),
					}) {
						return
					}
					return
				}
			}

			// Build assistant message.
			content := contentBuilder.String()

			// Record cognitive budget consumption for this streaming turn.
			// Streaming responses do not report token usage, so tokens are
			// estimated from the generated content; cost stays 0 by default.
			if budgetTracker != nil {
				budgetTracker.Record(providers.EstimateTokens([]string{content}), time.Since(turnStart), 0)
			}

			assistantMsg := chat.Message{
				Role:      chat.RoleAssistant,
				Content:   content,
				ToolCalls: toolCalls,
			}
			llmMessages = append(llmMessages, assistantMsg)

			// Post-turn: Store the interaction as a memory record.
			if e.memoryStorer != nil && content != "" {
				_, storeErr := e.memoryStorer.Store(ctx, MemoryRecord{
					Type:    "decision",
					Layer:   "session",
					Content: fmt.Sprintf("User: %s\nAssistant: %s", userInput, content),
					Metadata: map[string]string{
						"agent":   agentName,
						"session": sessionID,
						"model":   e.config.Model,
					},
					Priority: 1,
				})
				if storeErr != nil {
					// Log the failure but don't break the stream.
					info := safeerror.Inspect("memory_store_failed", storeErr)
					log.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Str("agent", agentName).Str("session", sessionID).Msg("memory store failed")
				}
			}

			// Persist.
			if e.sessionMgr != nil && sessionID != "" {
				_ = e.sessionMgr.AppendMessage(sessionID, assistantMsg)
			}

			// No tool calls → done.
			if len(toolCalls) == 0 {
				if !emit(EngineEvent{Type: EngineEventDone}) {
					return
				}
				return
			}

			// ── Execute tool calls ─────────────────────────────────
			for _, tc := range toolCalls {
				if e.hooks != nil {
					_ = e.hooks.ExecuteHooks(plugins.HookPreToolUse, tc)
				}

				if e.isSubagentSpawn(tc) {
					if !emit(EngineEvent{
						Type: EngineEventSubagentStart,
						ToolCall: &executor.ToolCall{
							ID:   tc.ID,
							Name: tc.Function.Name,
						},
					}) {
						return
					}

					subReq := e.parseSubagentRequest(tc)
					subResult, err := e.spawner.Spawn(ctx, subReq)
					if err != nil {
						llmMessages = append(llmMessages, chat.Message{
							Role:       chat.RoleTool,
							ToolCallID: tc.ID,
							Content:    contenttrust.Envelope(contenttrust.Default(contenttrust.OriginTool, safeerror.Message("subagent_spawn_failed"), tc.Function.Name)),
						})
						continue
					}

					if subResult.TokenUsage.TotalTokens > 0 {
						totalUsage.PromptTokens += subResult.TokenUsage.PromptTokens
						totalUsage.CompletionTokens += subResult.TokenUsage.CompletionTokens
						totalUsage.TotalTokens += subResult.TokenUsage.TotalTokens
					}

					if !emit(EngineEvent{
						Type:           EngineEventSubagentResult,
						SubagentResult: subResult,
					}) {
						return
					}

					resultContent := ""
					if subResult.Error != "" {
						resultContent = safeerror.ToolMessage(errors.New(subResult.Error))
					} else {
						resultContent = subResult.Summary
					}

					llmMessages = append(llmMessages, chat.Message{
						Role:       chat.RoleTool,
						ToolCallID: tc.ID,
						Content:    contenttrust.Envelope(contenttrust.Default(contenttrust.OriginTool, resultContent, tc.Function.Name)),
					})
				} else {
					execTC, convErr := e.chatToolCallToExec(tc)
					if convErr != nil {
						if !emit(EngineEvent{
							Type:  EngineEventError,
							Error: safeerror.Error("invalid_tool_arguments", convErr),
						}) {
							return
						}
						return
					}

					if !emit(EngineEvent{
						Type:     EngineEventToolStart,
						ToolCall: &execTC,
					}) {
						return
					}

					result, execErr := e.executor.Execute(ctx, execTC)
					if execErr != nil {
						if !emit(EngineEvent{
							Type:  EngineEventError,
							Error: safeerror.Error("tool_execution_failed", execErr),
						}) {
							return
						}
						return
					}

					if !emit(EngineEvent{
						Type:       EngineEventToolResult,
						ToolResult: safeToolResult(result),
					}) {
						return
					}

					resultContent := contenttrust.Envelope(contenttrust.Default(contenttrust.OriginTool, e.formatExecResult(result), execTC.Name))
					llmMessages = append(llmMessages, chat.Message{
						Role:       chat.RoleTool,
						ToolCallID: tc.ID,
						Content:    resultContent,
					})
				}

				if e.hooks != nil {
					lastMsg := llmMessages[len(llmMessages)-1]
					_ = e.hooks.ExecuteHooks(plugins.HookPostToolUse, lastMsg)
				}
			}

			// ── End of turn ──────────────────────────────────────
			if !emit(EngineEvent{
				Type: EngineEventTurnEnd,
				TurnRecord: &TurnRecord{
					TurnNumber:    turnCount,
					ToolCallsMade: len(toolCalls),
					TokensUsed:    totalUsage.TotalTokens,
					DurationMs:    time.Since(turnStart).Milliseconds(),
				},
			}) {
				return
			}

			// Auto-compaction: when context usage exceeds the threshold, shorten
			// the conversation history before feeding tool results back.
			if !compactedThisRun && builtCtx.UsagePct > CompactionThresholdPct {
				if newMsgs, ok := e.compactMessages(llmMessages); ok {
					before := len(llmMessages)
					llmMessages = newMsgs
					compactedThisRun = true
					log.Info().Int("turns_before", before).Int("turns_after", len(llmMessages)).Msg("context compacted")
				}
			}

			// Continue loop to feed tool results back to LLM.
		}
	}()

	return events, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Internal helpers
// ═══════════════════════════════════════════════════════════════════════════════

// buildChatOptions constructs ChatOptions from the engine config and tool defs.
func (e *AgentEngine) buildChatOptions(toolDefs []chat.ToolDefinition) chat.ChatOptions {
	return chat.ChatOptions{
		Temperature: e.config.Temperature,
		MaxTokens:   e.config.MaxTokens,
		Tools:       toolDefs,
	}
}

// withSystemPromptPrepended returns the conversation messages with the built
// system prompt (identity + memory + knowledge + skills) injected as the first
// message. The prompt is skipped when it is empty (e.g. unknown/unregistered
// agent) or when the conversation already opens with a system message, in
// which case injecting it again would duplicate the agent identity.
func withSystemPromptPrepended(builtCtx *BuiltContext) []chat.Message {
	llmMessages := make([]chat.Message, 0, len(builtCtx.Messages)+1)
	if strings.TrimSpace(builtCtx.SystemPrompt) != "" && !startsWithSystemMessage(builtCtx.Messages) {
		llmMessages = append(llmMessages, chat.Message{Role: chat.RoleSystem, Content: builtCtx.SystemPrompt})
	}
	return append(llmMessages, builtCtx.Messages...)
}

// startsWithSystemMessage reports whether the first conversation message is a
// system message, indicating the agent identity is already present in history.
func startsWithSystemMessage(messages []chat.Message) bool {
	return len(messages) > 0 && messages[0].Role == chat.RoleSystem
}

// compactMessages shortens the conversation history in llmMessages using the
// AutoCompaction algorithm (tool-output stripping + old-turn summarisation). The
// leading system prompt(s) are always preserved verbatim, and the most recent
// turns — including the last assistant response — are kept. It returns the
// compacted message list and whether a compaction actually happened. When
// auto-compaction is disabled (nil) or the history is too small to compact, the
// original list is returned unchanged.
func (e *AgentEngine) compactMessages(llmMessages []chat.Message) ([]chat.Message, bool) {
	if e.autoCompaction == nil || len(llmMessages) == 0 {
		return llmMessages, false
	}

	// Split off the leading system prompt(s) so the agent identity is never
	// summarised away by the compaction algorithm.
	idx := 0
	for idx < len(llmMessages) && llmMessages[idx].Role == chat.RoleSystem {
		idx++
	}
	head := llmMessages[:idx]
	conversation := llmMessages[idx:]

	if len(conversation) == 0 {
		return llmMessages, false
	}

	compacted := e.autoCompaction.compactMessages(conversation)
	if len(compacted) >= len(conversation) {
		return llmMessages, false
	}

	result := make([]chat.Message, 0, len(head)+len(compacted))
	result = append(result, head...)
	result = append(result, compacted...)
	return result, true
}

// isSubagentSpawn returns true if the tool call is a subagent spawn request.
func (e *AgentEngine) isSubagentSpawn(tc chat.ToolCall) bool {
	return tc.Function.Name == SubagentSpawnToolName
}

// subagentSpawnInput is the JSON structure for subagent spawn tool arguments.
type subagentSpawnInput struct {
	Agent     string            `json:"agent"`
	Task      string            `json:"task"`
	Context   map[string]string `json:"context,omitempty"`
	MaxTokens int               `json:"max_tokens,omitempty"`
}

// parseSubagentRequest extracts a SubagentRequest from a tool call.
// Returns a minimal SubagentRequest if parsing fails.
func (e *AgentEngine) parseSubagentRequest(tc chat.ToolCall) SubagentRequest {
	req := SubagentRequest{
		Agent: tc.Function.Name,
	}
	if tc.Function.Arguments == "" {
		return req
	}

	var input subagentSpawnInput
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
		return req
	}

	if input.Agent != "" {
		req.Agent = input.Agent
	}
	req.Task = input.Task
	req.Context = input.Context
	req.MaxTokens = input.MaxTokens
	return req
}

// chatToolCallToExec converts a chat.ToolCall to an executor.ToolCall.
func (e *AgentEngine) chatToolCallToExec(tc chat.ToolCall) (executor.ToolCall, error) {
	var input map[string]any
	if tc.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
			return executor.ToolCall{}, fmt.Errorf("parsing arguments for %q: %w", tc.Function.Name, err)
		}
	}
	return executor.ToolCall{
		ID:    tc.ID,
		Name:  tc.Function.Name,
		Input: input,
	}, nil
}

// formatExecResult converts an executor.ToolResult to a string suitable for
// inclusion as a tool result message.
func (e *AgentEngine) formatExecResult(result *executor.ToolResult) string {
	if result == nil {
		return "No result returned."
	}
	if result.Error != "" {
		return safeerror.ToolMessage(errors.New(result.Error))
	}
	if result.Output == nil {
		return "OK"
	}
	switch v := result.Output.(type) {
	case string:
		return v
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(bytes)
	}
}

func safeToolResult(result *executor.ToolResult) *executor.ToolResult {
	if result == nil {
		return nil
	}
	copy := *result
	if copy.Error != "" {
		copy.Error = safeerror.ToolMessage(errors.New(copy.Error))
	}
	return &copy
}

// buildResult constructs an EngineResult from the accumulated state. When a
// budgetTracker is active (non-nil), a snapshot of the cognitive budget spent
// is attached to the result.
func (e *AgentEngine) buildResult(
	content string,
	messages []chat.Message,
	usage chat.Usage,
	turnCount int,
	sessionID string,
	errMsg string,
	budget *BudgetTracker,
	turns ...TurnRecord,
) *EngineResult {
	result := &EngineResult{
		Content:    content,
		Messages:   messages,
		TokenUsage: usage,
		TurnCount:  turnCount,
		SessionID:  sessionID,
		Error:      errMsg,
		Turns:      turns,
	}
	if budget != nil {
		spent := budget.Spent()
		result.Budget = &spent
	}
	return result
}

// Ensure sync is used (placeholder for future concurrent operations).
var _ = sync.Mutex{}
