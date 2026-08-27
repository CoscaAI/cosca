// Package executor provides the Tool Executor — the central orchestrator that
// combines the ToolRegistry with the Sandbox Gate to execute tool calls with
// proper sandbox enforcement.
//
// The Executor sits between the Response Parser (which extracts ToolCall
// requests from LLM output) and the tool implementations. It validates calls,
// enforces sandbox policies, executes tools with context-based timeouts, and
// returns structured ToolResult values.
//
// Architecture (from next-gen-cli-design.md):
//
//	Response Parser → Tool Executor → Sandbox Gate → Tool Implementations
//
// The executor is also responsible for concurrent batch execution with a
// configurable concurrency limit, partial-failure tolerance, and full thread
// safety for concurrent access patterns.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/sandbox"
	"github.com/CoscaAI/cosca/internal/level"
	"github.com/CoscaAI/cosca/internal/policy"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// ToolCall represents a parsed request to invoke a specific tool. Unlike the
// LLM-facing chat.ToolCall (which carries raw JSON arguments), this type has
// already been decoded into a structured input map for convenient use by the
// executor and downstream consumers.
type ToolCall struct {
	// ID is a unique identifier for this tool call, typically provided by
	// the LLM response for correlating results with requests.
	ID string `json:"id"`

	// Name is the registered name of the tool to invoke (e.g. "read", "write").
	Name string `json:"name"`

	// Input is the parsed set of parameters for the tool, keyed by parameter
	// name. These are validated against the tool's JSON Schema before execution.
	Input map[string]interface{} `json:"input"`
}

// ToolResult contains the structured outcome of a single tool execution.
// It carries the correlation ID, a machine-readable status, the output payload
// (if successful), an error description (if failed), and the wall-clock
// duration in milliseconds.
type ToolResult struct {
	// ToolCallID correlates this result with the original ToolCall request.
	ToolCallID string `json:"tool_call_id"`

	// Status is one of "success", "error", or "timeout".
	Status string `json:"status"`

	// Output is the result payload produced by the tool on success.
	// The type and structure depend on the specific tool.
	Output interface{} `json:"output,omitempty"`

	// Error describes what went wrong. Empty on success.
	Error string `json:"error,omitempty"`

	// DurationMs is the wall-clock execution time in milliseconds.
	DurationMs int64 `json:"duration_ms"`
}

// ─── Interfaces ──────────────────────────────────────────────────────────────

// ToolRegistry defines the subset of tool.Registry operations that the executor
// depends on. This interface allows the executor to work with any registry
// implementation (the concrete *tool.Registry or a test double).
type ToolRegistry interface {
	// Register adds a tool to the registry.
	Register(t chat.Tool)

	// Get returns a tool by name, or nil if not found.
	Get(name string) chat.Tool

	// List returns all registered tool names sorted alphabetically.
	List() []string

	// GetAll returns all registered tools.
	GetAll() []chat.Tool

	// Execute finds a tool by name, validates its parameters, executes it,
	// and returns the result.
	Execute(ctx context.Context, name string, params json.RawMessage) *chat.ToolResult

	// Definitions returns all registered tools as ToolDefinition values
	// suitable for LLM function calling.
	Definitions() []chat.ToolDefinition
}

// SandboxGate defines the sandbox operations the executor needs. The concrete
// implementation is typically *sandbox.Gate, optionally wrapped to add the
// IsAvailable check. If no sandbox gate is provided (nil), the executor runs
// tools without sandbox-level enforcement, relying on each tool's own
// path-validation via rails.
type SandboxGate interface {
	// Execute runs a command within the sandbox under the given mode and
	// returns the result.
	Execute(ctx context.Context, cmd chat.Command, mode chat.SandboxMode) (*chat.SandboxResult, error)

	// IsAvailable reports whether the sandbox is operational (e.g. bwrap
	// binary found, workspace accessible). When false, the executor skips
	// sandbox-level enforcement for this execution.
	IsAvailable() bool
}

// ─── Batch Options ───────────────────────────────────────────────────────────

// batchConfig holds configuration for ExecuteBatch.
type batchConfig struct {
	concurrency int
}

// BatchOption is a functional option for configuring batch execution behaviour.
type BatchOption func(*batchConfig)

// WithConcurrency sets the maximum number of concurrent tool executions in a
// batch. Must be greater than zero; if not specified, a default of 3 is used.
func WithConcurrency(n int) BatchOption {
	return func(c *batchConfig) {
		if n > 0 {
			c.concurrency = n
		}
	}
}

// ─── Status Constants ────────────────────────────────────────────────────────

const (
	StatusSuccess string = "success"
	StatusError   string = "error"
	StatusTimeout string = "timeout"
)

// ─── Executor ────────────────────────────────────────────────────────────────

// Executor orchestrates tool execution with sandbox enforcement. It combines
// a ToolRegistry (for tool lookup and discovery) with a SandboxGate (for
// execution isolation) and workspace rails (for path validation).
//
// The executor is thread-safe: all exported methods coordinate access via an
// internal RWMutex, and concurrent ExecuteBatch calls are isolated by a
// semaphore-based concurrency limiter.
type Executor struct {
	registry  ToolRegistry
	sandbox   SandboxGate
	rails     *sandbox.Rails
	workspace string
	// policy é o guard determinístico (GOVERNANCE_PROTOCOL, L366): nível 2 da
	// hierarquia de autoridade, entre a validação e o sandbox. Nil = sem
	// guard (comportamento histórico).
	policy *policy.Engine
	// levelGate é o sistema de NÍVEIS de capacidade (decisão do Don 2026-08-25):
	// enforcement por código que limita o que o agente pode fazer conforme o
	// nível atual (L1 inicial / L2 operacional / L3 soberano). Avaliado ANTES do
	// policy — é a primeira barreira de soberania. Nil = nível não gerenciado.
	levelGate *level.Gate
	// execObserver é o hook chamado após cada execução (para o watchdog de
	// auto-regulação observar o resultado e descer o nível quando houver loop).
	// Nil = sem observação.
	execObserver func(result *ToolResult, progressed bool)
	mu           sync.RWMutex
}

// New creates a new Executor with the given dependencies.
//
//   - registry: the tool registry (typically *tool.Registry) to look up tools.
//   - sandboxGate: the sandbox gate for execution enforcement. May be nil,
//     in which case sandbox-level enforcement is disabled and tools run
//     with only their own internal validation.
//   - workspace: the absolute path to the workspace root. Used for path
//     validation via sandbox.Rails.
func New(registry ToolRegistry, sandboxGate SandboxGate, workspace string) *Executor {
	return &Executor{
		registry:  registry,
		sandbox:   sandboxGate,
		rails:     sandbox.NewRails(workspace),
		workspace: workspace,
	}
}

// SetPolicy anexa o guard determinístico ao executor (nil desativa).
func (e *Executor) SetPolicy(p *policy.Engine) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.policy = p
}

// SetLevelGate anexa o sistema de níveis de capacidade ao executor (nil
// desativa). O gate é a primeira barreira de soberania: decide pela matriz
// nível × dimensão se a ação é permitida no nível atual.
func (e *Executor) SetLevelGate(g *level.Gate) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.levelGate = g
}

// SetExecObserver registra o hook de observação de resultados (para o watchdog
// de auto-regulação). Chamado após cada execução com o resultado e se houve
// progresso. Nil desativa.
func (e *Executor) SetExecObserver(fn func(result *ToolResult, progressed bool)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.execObserver = fn
}

// ─── Core Methods ────────────────────────────────────────────────────────────

// Execute runs a single tool call with sandbox enforcement and context-based
// timeout support.
//
// The execution flow:
//  1. Validate the tool call (name, input schema, path constraints).
//  2. Look up the tool in the registry.
//  3. Enforce sandbox policy if the sandbox gate is available.
//  4. Marshal the input map to JSON and delegate to the tool's Execute method.
//  5. Check for context cancellation / deadline exceeded.
//  6. Return a structured ToolResult with status, output, error, and duration.
//
// The returned error is reserved for truly exceptional conditions (e.g. a
// misconfigured executor). Most execution failures are reported inside
// ToolResult with Status "error" and a non-empty Error field.
func (e *Executor) Execute(ctx context.Context, toolCall ToolCall) (res *ToolResult, eR error) {
	start := time.Now()

	// 0. Observer de auto-regulação (watchdog): alimenta o sistema de níveis após
	// cada execução, em TODOS os caminhos (defer captura o retorno nomeado). Se o
	// watchdog detectar o padrão de loop (L434), desce o nível automaticamente.
	defer func() {
		e.mu.RLock()
		obs := e.execObserver
		e.mu.RUnlock()
		if obs != nil {
			progressed := res != nil && res.Status == StatusSuccess
			obs(res, progressed)
		}
	}()

	// 0. Anti-hallucination of tool parameters (see normalize.go): normalize
	// the input map up front so BOTH validation (ValidateToolCall) and the
	// actual tool execution (step 4, json.Marshal) operate on the canonical
	// key set. If we only normalized inside ValidateToolCall, that step's
	// struct copy would rewrite its own Input, but the marshal here at step 4
	// would still see the raw hallucinated keys. This reassigns the local
	// toolCall so the whole execution path sees canonical keys.
	normalizeToolCall(&toolCall)

	// 1. Validate the tool call.
	if err := e.ValidateToolCall(toolCall); err != nil {
		return &ToolResult{
			ToolCallID: toolCall.ID,
			Status:     StatusError,
			Error:      err.Error(),
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	// 2. Look up the tool.
	tool := e.registry.Get(toolCall.Name)
	if tool == nil {
		return &ToolResult{
			ToolCallID: toolCall.ID,
			Status:     StatusError,
			Error:      fmt.Sprintf("tool not found: %s", toolCall.Name),
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	// 2.5 Level gate (sistema de níveis de capacidade — decisão do Don 2026-08-25).
	// A primeira barreira de soberania: decide pela matriz nível × dimensão se a
	// ação é permitida no nível atual, ANTES do policy determinístico. Um veredito
	// LEVEL-DENY bloqueia a execução por código (independe do LLM).
	e.mu.RLock()
	lg := e.levelGate
	e.mu.RUnlock()
	if lg != nil {
		lv := lg.Check(level.Action{
			Tool:       toolCall.Name,
			RawCommand: strArg(toolCall.Input, "command", "cmd"),
			TargetPath: strArg(toolCall.Input, "path", "filePath", "file"),
		})
		if lv == level.VDeny || lv == level.VNeedApproval {
			return &ToolResult{
				ToolCallID: toolCall.ID,
				Status:     StatusError,
				Error:      fmt.Sprintf("level %s: ação bloqueada pela soberania do nível — %s", lg.Current(), lv),
				DurationMs: time.Since(start).Milliseconds(),
			}, nil
		}
	}

	// 2.6 Policy enforcement (guard determinístico — L366). O veredito DENY
	// e CONFIRM bloqueiam a execução: a decisão é por código, independente
	// do LLM (GOVERNANCE_PROTOCOL §1).
	e.mu.RLock()
	pol := e.policy
	e.mu.RUnlock()
	if pol != nil {
		ev := pol.Evaluate(policy.NewAction(toolCall.Name, toolCall.Input))
		if ev.Decision == policy.Deny || ev.Decision == policy.Confirm || ev.Decision == policy.Escalate {
			return &ToolResult{
				ToolCallID: toolCall.ID,
				Status:     StatusError,
				Error:      fmt.Sprintf("policy %s: %s", ev.Decision, ev.Reason),
				DurationMs: time.Since(start).Milliseconds(),
			}, nil
		}
	}

	// 3. Sandbox enforcement.
	if e.sandbox != nil && e.sandbox.IsAvailable() {
		if err := e.enforceSandbox(toolCall); err != nil {
			return &ToolResult{
				ToolCallID: toolCall.ID,
				Status:     StatusError,
				Error:      fmt.Sprintf("sandbox denied: %s", err.Error()),
				DurationMs: time.Since(start).Milliseconds(),
			}, nil
		}
	}

	// 4. Marshal input and execute.
	params, err := json.Marshal(toolCall.Input)
	if err != nil {
		return &ToolResult{
			ToolCallID: toolCall.ID,
			Status:     StatusError,
			Error:      fmt.Sprintf("invalid input: %s", err.Error()),
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	result, execErr := tool.Execute(ctx, params)
	durationMs := time.Since(start).Milliseconds()

	// 5. Check for context cancellation / deadline exceeded.
	if execErr != nil {
		if errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execErr, context.Canceled) {
			return &ToolResult{
				ToolCallID: toolCall.ID,
				Status:     StatusTimeout,
				Error:      execErr.Error(),
				DurationMs: durationMs,
			}, nil
		}
		return &ToolResult{
			ToolCallID: toolCall.ID,
			Status:     StatusError,
			Error:      execErr.Error(),
			DurationMs: durationMs,
		}, nil
	}

	// Tools may also encode errors in the result (e.g. filesystem.go's
	// errorResult helper returns nil error + non-empty Error field).
	if result != nil && result.Error != "" {
		return &ToolResult{
			ToolCallID: toolCall.ID,
			Status:     StatusError,
			Error:      result.Error,
			DurationMs: durationMs,
		}, nil
	}

	// 6. Success.
	output := ""
	if result != nil {
		output = result.Output
	}

	return &ToolResult{
		ToolCallID: toolCall.ID,
		Status:     StatusSuccess,
		Output:     output,
		DurationMs: durationMs,
	}, nil
}

// ExecuteBatch executes multiple tool calls concurrently with a configurable
// concurrency limit (default 3). Results are returned in the same order as the
// input tool calls.
//
// Partial failure is tolerated: one tool failing does not cancel the execution
// of other tools in the batch. Each call produces a separate ToolResult, so
// callers can inspect individual results for success or failure.
//
// The context passed to ExecuteBatch is propagated to each individual Execute
// call. Cancelling the context cancels all in-flight executions.
func (e *Executor) ExecuteBatch(ctx context.Context, toolCalls []ToolCall, opts ...BatchOption) []*ToolResult {
	// Apply batch options.
	cfg := &batchConfig{concurrency: 3}
	for _, opt := range opts {
		opt(cfg)
	}

	results := make([]*ToolResult, len(toolCalls))
	if len(toolCalls) == 0 {
		return results
	}

	// Semaphore-based concurrency limiter.
	sem := make(chan struct{}, cfg.concurrency)
	var wg sync.WaitGroup

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, call ToolCall) {
			defer wg.Done()

			// Acquire semaphore (respects context cancellation).
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[idx] = &ToolResult{
					ToolCallID: call.ID,
					Status:     StatusTimeout,
					Error:      ctx.Err().Error(),
				}
				return
			}
			defer func() { <-sem }()

			result, _ := e.Execute(ctx, call)
			results[idx] = result
		}(i, tc)
	}

	wg.Wait()
	return results
}

// ValidateToolCall validates a tool call against three criteria:
//  1. Tool name exists in the registry (and is non-empty).
//  2. Input parameters conform to the tool's JSON Schema (basic type checking
//     via the tool's Validate method).
//  3. If the input contains a "path" field, it passes workspace path
//     constraints via rails (prevents workspace escape and blocked directories).
func (e *Executor) ValidateToolCall(toolCall ToolCall) error {
	if toolCall.Name == "" {
		return fmt.Errorf("tool name is required")
	}

	tool := e.registry.Get(toolCall.Name)
	if tool == nil {
		return fmt.Errorf("tool not found: %s", toolCall.Name)
	}

	if toolCall.Input == nil {
		return fmt.Errorf("tool %q input is required", toolCall.Name)
	}

	// Anti-hallucination of tool parameters (see normalize.go): rewrite known
	// hallucinated keys back to their canonical key BEFORE schema validation.
	// LLMs often echo the prose of a parameter description instead of the
	// literal schema key (e.g. "userQuery" instead of "query"), which would
	// otherwise make the JSON Schema validation fail and reject the call.
	// The normalization is a no-op when the canonical key is already present
	// or no alias matches.
	normalizeToolCall(&toolCall)

	// Validate input against the tool's JSON Schema.
	params, err := json.Marshal(toolCall.Input)
	if err != nil {
		return fmt.Errorf("tool %q invalid input: %w", toolCall.Name, err)
	}
	if err := tool.Validate(params); err != nil {
		return fmt.Errorf("tool %q validation failed: %w", toolCall.Name, err)
	}

	// Validate workspace path constraints via rails if a "path" field is
	// present. This is a secondary check; individual tools also perform
	// their own path validation.
	if pathRaw, ok := toolCall.Input["path"]; ok {
		pathStr, ok := pathRaw.(string)
		if !ok {
			return fmt.Errorf("tool %q path must be a string", toolCall.Name)
		}
		if err := e.rails.Validate(pathStr); err != nil {
			return fmt.Errorf("tool %q path validation: %w", toolCall.Name, err)
		}
	}

	return nil
}

// strArg extrai a primeira chave string presente em args (para montar a Action
// do level gate — comando e alvo). Mesmo padrão do helper do policy.
func strArg(args map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := args[k].(string); ok {
			return v
		}
	}
	return ""
}

// ListTools returns all registered tool definitions in the format required for
// LLM function calling (OpenAI-compatible). Each definition includes the tool
// name, description, and JSON Schema parameters.
//
// This is the primary discovery mechanism: the LLM calls this method to learn
// which tools are available and how to call them.
func (e *Executor) ListTools(ctx context.Context) ([]chat.ToolDefinition, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.registry.Definitions(), nil
}

// ─── Internal Helpers ────────────────────────────────────────────────────────

// enforceSandbox checks whether the sandbox gate permits this tool call.
// Currently it validates that any paths in the input are within workspace
// bounds. Future extensions may include mode-based gating (e.g. blocking
// write tools in read-only mode).
func (e *Executor) enforceSandbox(toolCall ToolCall) error {
	// Check for path fields in the input and validate them against rails.
	for key, val := range toolCall.Input {
		if !isPathField(key) {
			continue
		}
		pathStr, ok := val.(string)
		if !ok {
			continue
		}
		if err := e.rails.Validate(pathStr); err != nil {
			return err
		}
	}
	return nil
}

// isPathField returns true if the given input key is likely a file path
// that should be validated against workspace rails.
func isPathField(key string) bool {
	switch key {
	case "path", "pattern", "directory", "dir", "file":
		return true
	default:
		return false
	}
}
