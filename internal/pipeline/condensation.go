package pipeline

import (
	"fmt"
	"strings"
)

// ── Condensation (OpenHands Pattern #3) ──────────────────────────────────
//
// Condensation manages long event histories by summarizing older events
// when the total count exceeds a threshold. This prevents context overflow
// in long-running pipelines and multi-turn agent conversations.
//
// OpenHands Pattern:
//   Rolling-window: keep last N events in full, summarize older events
//   Dedicated subsystem: condensation is separate from main pipeline logic
//   Uses cheaper model for summarization (rule-based in MVP)

// CondensationEvent is a special event marking condensed history.
const CondensationEvent StepEventType = "condensation"

// CondenserConfig controls condensation behavior.
type CondenserConfig struct {
	// MaxEvents is the threshold at which condensation triggers.
	MaxEvents int
	// KeepEvents is the number of most recent events to keep in full.
	KeepEvents int
	// MinCondenseEvents is the minimum number of events to condense at once.
	MinCondenseEvents int
}

// DefaultCondenserConfig returns sensible defaults.
func DefaultCondenserConfig() CondenserConfig {
	return CondenserConfig{
		MaxEvents:         100,
		KeepEvents:        20,
		MinCondenseEvents: 30,
	}
}

// ContextCondenser manages event history condensation.
type ContextCondenser struct {
	config CondenserConfig
}

// NewContextCondenser creates a condenser with the given config.
func NewContextCondenser(config CondenserConfig) *ContextCondenser {
	if config.MaxEvents <= 0 {
		config.MaxEvents = 100
	}
	if config.KeepEvents <= 0 {
		config.KeepEvents = 20
	}
	if config.MinCondenseEvents <= 0 {
		config.MinCondenseEvents = 30
	}
	return &ContextCondenser{config: config}
}

// ShouldCondense returns true if the event count exceeds the threshold.
func (c *ContextCondenser) ShouldCondense(eventCount int) bool {
	return eventCount >= c.config.MaxEvents
}

// Condense summarizes a batch of events into a single condensation event.
// The output is deterministic (rule-based) — no LLM needed for MVP.
//
// Rules:
//   - Count completed, failed, started events per agent
//   - Extract key actions (first word of each action)
//   - Include total duration
func (c *ContextCondenser) Condense(planID string, events []StepEvent) (StepEvent, error) {
	if len(events) < c.config.MinCondenseEvents {
		return StepEvent{}, fmt.Errorf("condensation: need at least %d events, got %d",
			c.config.MinCondenseEvents, len(events))
	}

	// ── Summarize by counting ───────────────────────────────────────
	completed := 0
	failed := 0
	started := 0
	agents := make(map[string]int)
	totalDuration := int64(0)
	firstTime := events[0].Timestamp
	lastTime := events[len(events)-1].Timestamp

	for _, ev := range events {
		switch ev.Type {
		case StepEventCompleted, PlanCompleted:
			completed++
		case StepEventFailed, PlanFailed:
			failed++
		case StepEventStarted:
			started++
		}
		if ev.Agent != "" {
			agents[ev.Agent]++
		}
		totalDuration += ev.Duration
	}

	// Build agent summary
	agentParts := make([]string, 0, len(agents))
	for agent, count := range agents {
		agentParts = append(agentParts, fmt.Sprintf("%s:%d", agent, count))
	}

	summary := fmt.Sprintf(
		"[CONDENSED %d events → 1] %d completed, %d failed, %d started | "+
			"agents: %s | duration: %dms | span: %s → %s",
		len(events), completed, failed, started,
		strings.Join(agentParts, ", "),
		totalDuration,
		firstTime.Format("15:04:05"),
		lastTime.Format("15:04:05"),
	)

	return StepEvent{
		ID:        NewEventID(),
		Type:      CondensationEvent,
		PlanID:    planID,
		Output:    summary,
		Input:     fmt.Sprintf("%d events condensed", len(events)),
		Duration:  totalDuration,
		Timestamp: lastTime,
	}, nil
}

// Compact reduces the event list to KeepEvents + condensation summary.
// Returns the compacted event list and the number of events condensed.
func (c *ContextCondenser) Compact(planID string, events []StepEvent) ([]StepEvent, int, error) {
	if len(events) <= c.config.KeepEvents {
		return events, 0, nil
	}

	// Split: events to keep (most recent) + events to condense (older)
	split := len(events) - c.config.KeepEvents
	if split < c.config.MinCondenseEvents {
		return events, 0, nil
	}

	toCondense := events[:split]
	toKeep := events[split:]

	condensed, err := c.Condense(planID, toCondense)
	if err != nil {
		return events, 0, err
	}

	// Build compacted list: condensation + recent events
	compacted := make([]StepEvent, 0, len(toKeep)+1)
	compacted = append(compacted, condensed)
	compacted = append(compacted, toKeep...)

	return compacted, len(toCondense), nil
}

// ── Context Window Budget ────────────────────────────────────────────────

// ContextBudget tracks token usage and triggers condensation when needed.
type ContextBudget struct {
	MaxTokens    int
	TokensPerEvent int // estimated tokens per event
	Events       []StepEvent
}

// NewContextBudget creates a budget with the given token limit.
func NewContextBudget(maxTokens int) *ContextBudget {
	return &ContextBudget{
		MaxTokens:      maxTokens,
		TokensPerEvent: 200, // ~200 tokens per event (estimate)
	}
}

// AddEvent appends an event and returns whether condensation is needed.
func (b *ContextBudget) AddEvent(event StepEvent) bool {
	b.Events = append(b.Events, event)
	estimated := len(b.Events) * b.TokensPerEvent
	return estimated >= b.MaxTokens
}

// Usage returns the estimated token usage percentage.
func (b *ContextBudget) Usage() float64 {
	if b.MaxTokens == 0 {
		return 0
	}
	return float64(len(b.Events)*b.TokensPerEvent) / float64(b.MaxTokens) * 100
}
