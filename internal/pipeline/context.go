package pipeline

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/diagnostics"
)

// SessionContext is the shared state across all agents in a pipeline execution.
// Unlike the engine's ContextBuilder which rebuilds from scratch each turn,
// SessionContext accumulates state across agents.
type SessionContext struct {
	SessionID  string                        `json:"session_id"`
	Plan       *Plan                         `json:"plan,omitempty"`
	Handoffs   []*HandoffArtifact            `json:"handoffs"`
	ActiveTask *TaskNode                     `json:"active_task,omitempty"`
	Knowledge  []string                      `json:"knowledge"`
	Errors     []diagnostics.ClassifiedError `json:"errors"`
	Metrics    SessionMetrics                `json:"metrics"`
	mu         sync.RWMutex
}

// SessionMetrics tracks aggregate statistics across the pipeline session.
type SessionMetrics struct {
	StartedAt      time.Time     `json:"started_at"`
	TasksCompleted int           `json:"tasks_completed"`
	TasksFailed    int           `json:"tasks_failed"`
	BuildAttempts  int           `json:"build_attempts"`
	TestAttempts   int           `json:"test_attempts"`
	RetriesUsed    int           `json:"retries_used"`
	TokensUsed     TokenUsage    `json:"tokens_used"`
	TotalDuration  time.Duration `json:"total_duration"`
}

// NewSessionContext creates a context from a plan.
func NewSessionContext(plan *Plan) *SessionContext {
	return &SessionContext{
		SessionID: plan.ID,
		Plan:      plan,
		Handoffs:  make([]*HandoffArtifact, 0),
		Knowledge: make([]string, 0),
		Errors:    make([]diagnostics.ClassifiedError, 0),
		Metrics: SessionMetrics{
			StartedAt: time.Now().UTC(),
		},
	}
}

// RecordHandoff adds a handoff artifact to the session.
func (sc *SessionContext) RecordHandoff(artifact *HandoffArtifact) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Handoffs = append(sc.Handoffs, artifact)
}

// RecordError logs a classified error.
func (sc *SessionContext) RecordError(err diagnostics.ClassifiedError) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Errors = append(sc.Errors, err)
}

// IncrementTasksCompleted bumps the completed task count.
func (sc *SessionContext) IncrementTasksCompleted() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Metrics.TasksCompleted++
}

// IncrementTasksFailed bumps the failed task count.
func (sc *SessionContext) IncrementTasksFailed() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Metrics.TasksFailed++
}

// AddTokens accumulates token usage.
func (sc *SessionContext) AddTokens(input, output int) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Metrics.TokensUsed.Input += input
	sc.Metrics.TokensUsed.Output += output
}

// Summary returns a human-readable session summary.
func (sc *SessionContext) Summary() string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "Session: %s\n", sc.SessionID)
	fmt.Fprintf(&b, "Duration: %s\n", time.Since(sc.Metrics.StartedAt).Round(time.Millisecond))
	fmt.Fprintf(&b, "Tasks: %d completed, %d failed\n",
		sc.Metrics.TasksCompleted, sc.Metrics.TasksFailed)
	fmt.Fprintf(&b, "Handoffs: %d\n", len(sc.Handoffs))
	fmt.Fprintf(&b, "Errors: %d\n", len(sc.Errors))
	fmt.Fprintf(&b, "Tokens: %d in / %d out\n",
		sc.Metrics.TokensUsed.Input, sc.Metrics.TokensUsed.Output)
	fmt.Fprintf(&b, "Builds: %d | Tests: %d | Retries: %d\n",
		sc.Metrics.BuildAttempts, sc.Metrics.TestAttempts, sc.Metrics.RetriesUsed)
	if len(sc.Knowledge) > 0 {
		fmt.Fprintf(&b, "Knowledge used: %s\n", strings.Join(sc.Knowledge, ", "))
	}
	return b.String()
}
