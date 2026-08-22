package pipeline

import (
	"strings"
	"testing"
)

func TestTerminalMetricsBasics(t *testing.T) {
	m := NewTerminalMetrics()
	if m.born.IsZero() {
		t.Fatal("born must be set")
	}
	m.SetModel("gpt-4o")
	m.SetAgent("cosca-backend")
	if m.CurrentModel != "gpt-4o" || m.CurrentAgent != "cosca-backend" {
		t.Fatalf("model/agent: %+v", m)
	}
}

func TestTerminalMetricsTokensAndCost(t *testing.T) {
	m := NewTerminalMetrics()
	// gpt-4o: input 2.50, output 10.00 → avg 6.25/M.
	m.AddTokens(1_000_000, "gpt-4o")
	if m.TokensUsed != 1_000_000 {
		t.Fatalf("tokens = %d", m.TokensUsed)
	}
	if m.EstimatedCost != 6.25 {
		t.Fatalf("cost = %v, want 6.25", m.EstimatedCost)
	}

	// Unknown model → deepseek-v4 fallback (avg 0.21/M).
	m.AddTokens(1_000_000, "weird-model")
	if m.EstimatedCost <= 6.25 {
		t.Fatalf("cost after unknown = %v", m.EstimatedCost)
	}
}

func TestTerminalMetricsProgress(t *testing.T) {
	m := NewTerminalMetrics()
	m.SetProgress(2, 4)
	if m.ProgressPercent != 50.0 {
		t.Fatalf("progress = %v", m.ProgressPercent)
	}
	m.IncrementCompleted()
	if m.TasksCompleted != 3 || m.ProgressPercent != 75.0 {
		t.Fatalf("after increment: %+v", m)
	}
	// total=0 → progress 0.
	m2 := NewTerminalMetrics()
	m2.SetProgress(5, 0)
	if m2.ProgressPercent != 0 {
		t.Fatalf("zero-total progress = %v", m2.ProgressPercent)
	}
}

func TestTerminalMetricsToolAndError(t *testing.T) {
	m := NewTerminalMetrics()
	m.SetTool("read_file")
	if m.ActiveTool != "read_file" {
		t.Fatal("tool not set")
	}
	m.ClearTool()
	if m.ActiveTool != "" {
		t.Fatal("tool not cleared")
	}
	m.SetError("something broke")
	if !strings.Contains(m.LastError, "broke") {
		t.Fatal("error not set")
	}
	m.ClearError()
	if m.LastError != "" {
		t.Fatal("error not cleared")
	}
}

func TestTerminalMetricsHUDLine(t *testing.T) {
	m := NewTerminalMetrics()
	m.SetAgent("cosca-kernel")
	m.SetModel("gpt-4o")
	m.SetProgress(1, 2)
	m.AddTokens(1000, "deepseek-v4")
	m.SetTool("execute_command")

	out := m.HUDLine()
	for _, want := range []string{"agent:cosca-kernel", "model:gpt-4o", "tasks:1/2", "tokens:1000", "cost:$", "tool:execute_command"} {
		if !strings.Contains(out, want) {
			t.Fatalf("HUDLine missing %q: %q", want, out)
		}
	}

	// Error truncated to 40 chars.
	m.SetError(strings.Repeat("e", 100))
	out = m.HUDLine()
	if !strings.Contains(out, "...") {
		t.Fatalf("error not truncated: %q", out)
	}
}

func TestTerminalMetricsHUDDetailed(t *testing.T) {
	m := NewTerminalMetrics()
	m.SetAgent("cosca-backend")
	m.SetProgress(3, 4)
	m.AddTokens(500, "gpt-4o-mini")

	out := m.HUDDetailed()
	for _, want := range []string{"COSCA — LIVE", "Uptime:", "Agent:", "Progress:", "75"} {
		if !strings.Contains(out, want) {
			t.Fatalf("HUDDetailed missing %q:\n%s", want, out)
		}
	}
}

func TestProgressBar(t *testing.T) {
	if progressBar(50, 38) == "" {
		t.Fatal("normal bar empty")
	}
	// Width < 2 → empty.
	if progressBar(50, 1) != "" {
		t.Fatal("tiny width must be empty")
	}
	// Clamps.
	bar := progressBar(150, 10)
	if !strings.Contains(bar, "██████████") {
		t.Fatalf("over-100 bar: %q", bar)
	}
	bar2 := progressBar(-10, 10)
	if !strings.Contains(bar2, "0%") {
		t.Fatalf("negative bar: %q", bar2)
	}
	// Exact count of filled cells.
	bar3 := progressBar(50, 10)
	if strings.Count(bar3, "█") != 5 || strings.Count(bar3, "░") != 5 {
		t.Fatalf("half bar: %q", bar3)
	}
}
