package pipeline

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

type CostTracker struct {
	sessions map[string]*SessionCost
	mu       sync.RWMutex
}

type SessionCost struct {
	SessionID   string
	Tasks       []TaskCost
	TotalTokens int64
	TotalCost   float64
	StartTime   time.Time
	ModelCosts  map[string]float64
}

type TaskCost struct {
	TaskID       string
	Model        string
	PromptTokens int64
	CompTokens   int64
	Cost         float64
	Duration     time.Duration
}

var ModelPricing = map[string]struct{ Input, Output float64 }{
	"gpt-4o":        {2.50, 10.00},
	"gpt-4o-mini":   {0.15, 0.60},
	"claude-sonnet": {3.00, 15.00},
	"deepseek-v4":   {0.14, 0.28},
	"deepseek-v4-pro": {0.14, 0.28},
	"ollama-local":  {0.00, 0.00},
}

func NewCostTracker() *CostTracker {
	return &CostTracker{
		sessions: make(map[string]*SessionCost),
	}
}

func (c *CostTracker) TrackTask(sessionID, taskID, model string, promptTokens, compTokens int64, d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, ok := c.sessions[sessionID]
	if !ok {
		session = &SessionCost{
			SessionID:  sessionID,
			StartTime:  time.Now().UTC(),
			ModelCosts: make(map[string]float64),
		}
		c.sessions[sessionID] = session
	}

	cost := c.computeCost(model, promptTokens, compTokens)

	tc := TaskCost{
		TaskID:       taskID,
		Model:        model,
		PromptTokens: promptTokens,
		CompTokens:   compTokens,
		Cost:         cost,
		Duration:     d,
	}

	session.Tasks = append(session.Tasks, tc)
	session.TotalTokens += promptTokens + compTokens
	session.TotalCost += cost
	session.ModelCosts[model] += cost
}

func (c *CostTracker) computeCost(model string, promptTokens, compTokens int64) float64 {
	pricing, ok := ModelPricing[model]
	if !ok {
		pricing = ModelPricing["deepseek-v4"]
	}
	inputCost := (float64(promptTokens) / 1_000_000) * pricing.Input
	outputCost := (float64(compTokens) / 1_000_000) * pricing.Output
	return round6(inputCost + outputCost)
}

func (c *CostTracker) SessionTotal(sessionID string) (tokens int64, cost float64, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	session, ok := c.sessions[sessionID]
	if !ok {
		return 0, 0, false
	}
	return session.TotalTokens, session.TotalCost, true
}

func (c *CostTracker) FormatReport(sessionID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	session, ok := c.sessions[sessionID]
	if !ok {
		return "No cost data for session " + sessionID
	}

	var b strings.Builder
	dur := time.Since(session.StartTime).Round(time.Second)

	b.WriteString(fmt.Sprintf("=== COST REPORT: %s ===\n", sessionID))
	b.WriteString(fmt.Sprintf("Session duration: %s\n", dur))
	b.WriteString(fmt.Sprintf("Total tokens:     %d\n", session.TotalTokens))
	b.WriteString(fmt.Sprintf("Total cost:       $%.6f\n", session.TotalCost))
	b.WriteString(fmt.Sprintf("Task count:       %d\n\n", len(session.Tasks)))

	b.WriteString("--- By Model ---\n")
	for model, cost := range session.ModelCosts {
		pct := 0.0
		if session.TotalCost > 0 {
			pct = (cost / session.TotalCost) * 100
		}
		b.WriteString(fmt.Sprintf("  %s: $%.6f (%.1f%%)\n", model, cost, pct))
	}

	b.WriteString("\n--- Task Details ---\n")
	for _, tc := range session.Tasks {
		b.WriteString(fmt.Sprintf("  %s | model=%s | prompt=%d comp=%d | cost=$%.6f | dur=%s\n",
			tc.TaskID, tc.Model, tc.PromptTokens, tc.CompTokens, tc.Cost, tc.Duration.Round(time.Millisecond)))
	}

	return b.String()
}

func (c *CostTracker) EstimateRemaining(sessionID string, tasksRemaining int) (estTokens int64, estCost float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	session, ok := c.sessions[sessionID]
	if !ok || len(session.Tasks) == 0 {
		return 0, 0
	}

	avgTokens := session.TotalTokens / int64(len(session.Tasks))
	avgCost := session.TotalCost / float64(len(session.Tasks))

	estTokens = avgTokens * int64(tasksRemaining)
	estCost = math.Round(avgCost*float64(tasksRemaining)*1e6) / 1e6
	return
}

func round6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}
