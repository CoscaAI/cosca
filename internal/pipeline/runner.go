package pipeline

import (
	"context"
	"time"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

type Runner interface {
	Run(ctx context.Context, req RunRequest) (*RunResult, error)
	RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error)
}

type RunRequest struct {
	Prompt  string
	Agent   string
	History []Message
	Options RunOptions

	// IntentType é a intenção da task (detectTaskType), transportada do
	// TaskNode até o orquestrador. Decide, pelo DADO, se a task é de ação
	// (vai ao LLM com tools) ou de consulta (pode ser determinística).
	IntentType string
}

type RunOptions struct {
	MaxTurns    int
	Temperature float64
	EnableBuild bool
	EnableTest  bool
	Timeout     time.Duration
	Planner     *Planner
}

type RunResult struct {
	Response    string
	Agent       string
	TokenUsage  TokenUsage
	TurnCount   int
	BuildResult *BuildResult
	TestResult  *TestResult
	TraceID     string
	MemoryID    string
	SkillsUsed  []string

	// ToolExecutions lista as tools realmente executadas (write_file/build/etc).
	// Alimenta a evidência de artefato do ADR-031 quando o pipeline não tem fase
	// explícita de build/test (build roda via tool execute_command do agente).
	ToolExecutions []ToolCallResult
}

type RunEvent struct {
	Type RunEventType
	Data any
}

type RunEventType string

const (
	EventContent     RunEventType = "content"
	EventToolStart   RunEventType = "tool_start"
	EventToolResult  RunEventType = "tool_result"
	EventBuildStart  RunEventType = "build_start"
	EventBuildEnd    RunEventType = "build_end"
	EventTestStart   RunEventType = "test_start"
	EventTestEnd     RunEventType = "test_end"
	EventError       RunEventType = "error"
	EventDone        RunEventType = "done"
	// EventCancelled is ADDITIVE: it signals that the run was cancelled (client
	// disconnect / explicit cancel / request-context cancelled), a FINAL state
	// DISTINCT from EventError (provider failure). Consumers that already branch
	// on the known types are unaffected — a new type falls to their default
	// case (no output, no panic).
	EventCancelled RunEventType = "cancelled"
)

type Message struct {
	Role    string
	Content string
}

type ToolCallResult = orchestration.ToolCallResult

type TokenUsage struct {
	Input  int
	Output int

	// Decomposição (ADR-031 Fase 0.1): 0 quando o provider não expõe o detalhe.
	CachedTokens    int `json:"cached_tokens,omitempty"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

type BuildResult struct {
	Success    bool
	Output     string
	DurationMs int64
}

type TestResult struct {
	Success    bool
	Passed     int
	Failed     int
	Output     string
	DurationMs int64
}
