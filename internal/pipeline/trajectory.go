package pipeline

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Trajectory records the full execution path of an agent through a task.
// It implements event sourcing: every step (thought, action, observation)
// is appended as an immutable event, enabling replay and debugging.
type Trajectory struct {
	mu       sync.RWMutex
	ID       string            `json:"id"`
	TaskID   string            `json:"task_id"`
	Agent    string            `json:"agent"`
	Events   []TrajectoryEvent `json:"events"`
	Started  time.Time         `json:"started"`
	Finished time.Time         `json:"finished,omitempty"`
	Outcome  string            `json:"outcome,omitempty"` // success, failed, timeout
}

// TrajectoryEvent is a single step in the agent's execution.
type TrajectoryEvent struct {
	Step       int       `json:"step"`
	Type       EventType `json:"type"`
	Content    string    `json:"content"`
	ToolCall   string    `json:"tool_call,omitempty"`
	ToolResult string    `json:"tool_result,omitempty"`
	TokensIn   int       `json:"tokens_in,omitempty"`
	TokensOut  int       `json:"tokens_out,omitempty"`
	DurationMs int64     `json:"duration_ms,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// EventType classifies trajectory events.
type EventType string

const (
	TrajThought     EventType = "thought"     // agent reasoning
	TrajAction      EventType = "action"      // tool call
	TrajObservation EventType = "observation" // tool result
	TrajPlan        EventType = "plan"        // planner output
	TrajDoD         EventType = "dod"         // definition of done check
	TrajRecovery    EventType = "recovery"    // recovery loop attempt
	TrajError       EventType = "error"       // execution error
	TrajComplete    EventType = "complete"    // task finished
)

// NewTrajectory creates a new trajectory for a task.
func NewTrajectory(taskID, agent string) *Trajectory {
	return &Trajectory{
		ID:      fmt.Sprintf("TRJ-%s-%s", time.Now().Format("20060102"), randomHex(8)),
		TaskID:  taskID,
		Agent:   agent,
		Events:  make([]TrajectoryEvent, 0),
		Started: time.Now(),
	}
}

// Record adds an event to the trajectory.
func (t *Trajectory) Record(eventType EventType, content string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Events = append(t.Events, TrajectoryEvent{
		Step:      len(t.Events) + 1,
		Type:      eventType,
		Content:   content,
		Timestamp: time.Now(),
	})
}

// RecordTool records a tool call with its result.
func (t *Trajectory) RecordTool(toolName, input, result string, durationMs int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	step := len(t.Events) + 1
	now := time.Now()

	// Record the action (tool call).
	t.Events = append(t.Events, TrajectoryEvent{
		Step:       step,
		Type:       TrajAction,
		ToolCall:   toolName + ": " + truncate(input, 200),
		DurationMs: durationMs,
		Timestamp:  now,
	})

	// Record the observation (tool result).
	t.Events = append(t.Events, TrajectoryEvent{
		Step:       step + 1,
		Type:       TrajObservation,
		ToolResult: truncate(result, 500),
		DurationMs: durationMs,
		Timestamp:  now,
	})
}

// Complete marks the trajectory as finished.
func (t *Trajectory) Complete(outcome string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Finished = time.Now()
	t.Outcome = outcome
	t.Events = append(t.Events, TrajectoryEvent{
		Step:      len(t.Events) + 1,
		Type:      TrajComplete,
		Content:   outcome,
		Timestamp: time.Now(),
	})
}

// Duration returns the total execution time.
func (t *Trajectory) Duration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.Finished.IsZero() {
		return time.Since(t.Started)
	}
	return t.Finished.Sub(t.Started)
}

// StepCount returns the number of events recorded.
func (t *Trajectory) StepCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.Events)
}

// Summary returns a human-readable trajectory summary.
func (t *Trajectory) Summary() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "Trajectory: %s\n", t.ID)
	fmt.Fprintf(&b, "Task:       %s\n", t.TaskID)
	fmt.Fprintf(&b, "Agent:      %s\n", t.Agent)
	fmt.Fprintf(&b, "Steps:      %d\n", len(t.Events))
	fmt.Fprintf(&b, "Duration:   %s\n", t.Duration().Round(time.Millisecond))
	fmt.Fprintf(&b, "Outcome:    %s\n", t.Outcome)
	fmt.Fprintf(&b, "\nTimeline:\n")

	for _, e := range t.Events {
		elapsed := e.Timestamp.Sub(t.Started).Round(time.Millisecond)
		switch e.Type {
		case TrajThought:
			fmt.Fprintf(&b, "  [%s] 💭 %s\n", elapsed, truncate(e.Content, 80))
		case TrajAction:
			fmt.Fprintf(&b, "  [%s] 🔧 %s\n", elapsed, e.ToolCall)
		case TrajObservation:
			fmt.Fprintf(&b, "  [%s] 👁  %s\n", elapsed, truncate(e.ToolResult, 80))
		case TrajPlan:
			fmt.Fprintf(&b, "  [%s] 📋 %s\n", elapsed, truncate(e.Content, 80))
		case TrajRecovery:
			fmt.Fprintf(&b, "  [%s] 🔄 RECOVERY: %s\n", elapsed, truncate(e.Content, 80))
		case TrajError:
			fmt.Fprintf(&b, "  [%s] ❌ %s\n", elapsed, truncate(e.Content, 80))
		case TrajComplete:
			fmt.Fprintf(&b, "  [%s] ✅ %s\n", elapsed, e.Content)
		default:
			fmt.Fprintf(&b, "  [%s] %s: %s\n", elapsed, e.Type, truncate(e.Content, 80))
		}
	}

	return b.String()
}

// ── Helpers ─────────────────────────────────────────────────────────────

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func randomHex(n int) string {
	const hexChars = "0123456789ABCDEF"
	b := make([]byte, n)
	for i := range b {
		b[i] = hexChars[time.Now().UnixNano()%16]
		time.Sleep(1) // ensure different values
	}
	return string(b)
}
