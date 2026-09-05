package pipeline

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type TerminalMetrics struct {
	CurrentModel    string
	CurrentAgent    string
	TokensUsed      int64
	EstimatedCost   float64
	Uptime          time.Duration
	TasksCompleted  int
	TasksTotal      int
	ProgressPercent float64
	ActiveTool      string
	LastError       string

	mu   sync.RWMutex
	born time.Time
}

func NewTerminalMetrics() *TerminalMetrics {
	return &TerminalMetrics{
		born: time.Now().UTC(),
	}
}

func (t *TerminalMetrics) SetModel(model string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.CurrentModel = model
}

func (t *TerminalMetrics) SetAgent(agent string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.CurrentAgent = agent
}

func (t *TerminalMetrics) AddTokens(tokens int64, model string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.TokensUsed += tokens

	pricing, ok := ModelPricing[model]
	if !ok {
		pricing = ModelPricing["deepseek-v4"]
	}
	avgRate := (pricing.Input + pricing.Output) / 2
	t.EstimatedCost += (float64(tokens) / 1_000_000) * avgRate
}

func (t *TerminalMetrics) SetProgress(completed, total int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.TasksCompleted = completed
	t.TasksTotal = total
	if total > 0 {
		t.ProgressPercent = float64(completed) / float64(total) * 100
	} else {
		t.ProgressPercent = 0
	}
}

func (t *TerminalMetrics) IncrementCompleted() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.TasksCompleted++
	if t.TasksTotal > 0 {
		t.ProgressPercent = float64(t.TasksCompleted) / float64(t.TasksTotal) * 100
	}
}

func (t *TerminalMetrics) SetTool(tool string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ActiveTool = tool
}

func (t *TerminalMetrics) ClearTool() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ActiveTool = ""
}

func (t *TerminalMetrics) SetError(err string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.LastError = err
}

func (t *TerminalMetrics) ClearError() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.LastError = ""
}

func (t *TerminalMetrics) Refresh() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Uptime = time.Since(t.born)
}

func (t *TerminalMetrics) HUDLine() string {
	t.Refresh()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[%s] ", t.Uptime.Round(time.Second)))

	if t.CurrentAgent != "" {
		b.WriteString(fmt.Sprintf("agent:%s ", t.CurrentAgent))
	}
	if t.CurrentModel != "" {
		b.WriteString(fmt.Sprintf("model:%s ", t.CurrentModel))
	}

	if t.TasksTotal > 0 {
		b.WriteString(fmt.Sprintf("tasks:%d/%d ", t.TasksCompleted, t.TasksTotal))
	}

	b.WriteString(fmt.Sprintf("tokens:%d ", t.TokensUsed))
	b.WriteString(fmt.Sprintf("cost:$%.4f ", t.EstimatedCost))

	if t.ActiveTool != "" {
		b.WriteString(fmt.Sprintf("tool:%s ", t.ActiveTool))
	}

	if t.LastError != "" {
		b.WriteString(fmt.Sprintf("err:%s", truncate(t.LastError, 40)))
	}

	return strings.TrimSpace(b.String())
}

func (t *TerminalMetrics) HUDDetailed() string {
	t.Refresh()

	var b strings.Builder
	b.WriteString("┌─ COSCA — LIVE ──────────────────────────────────┐\n")
	b.WriteString(fmt.Sprintf("│ Uptime:     %-38s │\n", t.Uptime.Round(time.Second)))
	b.WriteString(fmt.Sprintf("│ Agent:      %-38s │\n", t.CurrentAgent))
	b.WriteString(fmt.Sprintf("│ Model:      %-38s │\n", t.CurrentModel))
	b.WriteString(fmt.Sprintf("│ Tokens:     %-38d │\n", t.TokensUsed))
	b.WriteString(fmt.Sprintf("│ Cost:       $%-37.4f │\n", t.EstimatedCost))

	if t.TasksTotal > 0 {
		bar := progressBar(int(t.ProgressPercent), 38)
		b.WriteString(fmt.Sprintf("│ Progress:   %s │\n", bar))
		b.WriteString(fmt.Sprintf("│ Tasks:      %d / %-34d │\n", t.TasksCompleted, t.TasksTotal))
	}

	if t.ActiveTool != "" {
		b.WriteString(fmt.Sprintf("│ Tool:       %-38s │\n", t.ActiveTool))
	}

	if t.LastError != "" {
		err := truncate(t.LastError, 38)
		b.WriteString(fmt.Sprintf("│ Error:      %-38s │\n", err))
	}

	b.WriteString("└──────────────────────────────────────────────────┘")
	return b.String()
}

func progressBar(pct, width int) string {
	if width < 2 {
		return ""
	}
	filled := pct * width / 100
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return fmt.Sprintf("%s%s %d%%",
		strings.Repeat("█", filled),
		strings.Repeat("░", width-filled),
		pct)
}


