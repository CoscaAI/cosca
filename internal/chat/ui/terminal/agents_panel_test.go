package terminal

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── P11: Agents panel ────────────────────────────────────────────────────────

func TestAgentsPanelRendersHierarchy(t *testing.T) {
	used := []string{"CTO", "Go Specialist"}
	view := AgentsPanelView(used, "CTO", 30, 20)

	// Root + kernel + chiefs should be present.
	for _, want := range []string{"DON", "KERNEL", "CTO", "Security", "QA"} {
		if !strings.Contains(view, want) {
			t.Fatalf("agents panel missing %q: %q", want, view)
		}
	}
	// Specialists under Backend should appear.
	if !strings.Contains(view, "Go Specialist") {
		t.Fatalf("agents panel missing specialist: %q", view)
	}
}

func TestAgentsPanelStatusGlyphs(t *testing.T) {
	// Active agent gets ●, done (used but not current) gets ✓, waiting gets ○.
	used := []string{"CTO"} // CTO worked but is not the current agent
	view := AgentsPanelView(used, "Security", 30, 30)

	if !strings.Contains(view, "●") {
		t.Fatalf("active agent should render ●: %q", view)
	}
	if !strings.Contains(view, "✓") {
		t.Fatalf("done agent should render ✓: %q", view)
	}
	if !strings.Contains(view, "○") {
		t.Fatalf("waiting agent should render ○: %q", view)
	}
}

func TestAgentsPanelFillsDimensions(t *testing.T) {
	view := AgentsPanelView(nil, "", 30, 10)
	if got := lipgloss.Width(view); got != 30 {
		t.Fatalf("agents panel width = %d, want 30", got)
	}
	if got := lipgloss.Height(view); got != 10 {
		t.Fatalf("agents panel height = %d, want 10", got)
	}
}

func TestAlt3SwitchesToRealAgentsPanel(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{CurrentAgent: "CTO"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	// Alt+3 → PanelAgents
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}, Alt: true})
	m = updated.(Model)
	if m.activePanel != PanelAgents {
		t.Fatalf("activePanel = %d, want PanelAgents", m.activePanel)
	}

	// The View should render the real agents hierarchy, not the placeholder.
	view := m.View()
	if strings.Contains(view, "Em construção") {
		t.Fatalf("Agents panel should not be a placeholder: %q", view)
	}
	if !strings.Contains(view, "DON") {
		t.Fatalf("Agents panel should render hierarchy: %q", view)
	}
}

// ─── P12: Task progress ───────────────────────────────────────────────────────

func TestTaskPanelShowsStageProgress(t *testing.T) {
	tasks := []TaskInfo{
		{ID: "1842", Name: "Build health endpoint", Agent: "CTO", Status: "running"},
	}
	view := TaskPanelView(tasks, 30, 20)

	if !strings.Contains(view, "TASK") {
		t.Fatalf("task panel missing TASK header: %q", view)
	}
	if !strings.Contains(view, "Planning") {
		t.Fatalf("task panel missing Planning stage: %q", view)
	}
	if !strings.Contains(view, "Implementation") {
		t.Fatalf("task panel missing Implementation stage: %q", view)
	}
	if !strings.Contains(view, "Tests") {
		t.Fatalf("task panel missing Tests stage: %q", view)
	}
	// Progress bar blocks.
	if !strings.Contains(view, "█") {
		t.Fatalf("task panel missing progress bar fill: %q", view)
	}
	if !strings.Contains(view, "░") {
		t.Fatalf("task panel missing progress bar track: %q", view)
	}
	// Per-agent delegation line.
	if !strings.Contains(view, "CTO") {
		t.Fatalf("task panel missing agent chip: %q", view)
	}
}

func TestTaskPanelCompletedShowsFullBars(t *testing.T) {
	tasks := []TaskInfo{
		{ID: "1", Name: "Done task", Agent: "CTO", Status: "completed"},
	}
	view := TaskPanelView(tasks, 30, 20)
	// Completed task should have full bars (no empty track visible for stages).
	if strings.Contains(view, "░") {
		t.Fatalf("completed task should have full progress bars: %q", view)
	}
}

func TestTaskPanelEmpty(t *testing.T) {
	view := TaskPanelView(nil, 30, 10)
	if !strings.Contains(view, "No active tasks") {
		t.Fatalf("empty task panel should show message: %q", view)
	}
}

func TestTaskStagesForStatus(t *testing.T) {
	running := taskStagesFor(TaskInfo{Status: "running"})
	if running[0].Pct != 1 || running[1].Pct != 1 {
		t.Fatalf("running task should have planning+research done, got %+v", running)
	}
	if running[2].Pct <= 0 || running[2].Pct >= 1 {
		t.Fatalf("running task implementation should be partial, got %f", running[2].Pct)
	}

	done := taskStagesFor(TaskInfo{Status: "completed"})
	for _, s := range done {
		if s.Pct != 1 {
			t.Fatalf("completed task stage %q should be full, got %f", s.Name, s.Pct)
		}
	}

	pending := taskStagesFor(TaskInfo{Status: "pending"})
	for _, s := range pending {
		if s.Pct != 0 {
			t.Fatalf("pending task stage %q should be empty, got %f", s.Name, s.Pct)
		}
	}
}

// ─── P13: Agent chain ─────────────────────────────────────────────────────────

func TestAgentChainBreadcrumb(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{CurrentAgent: "CTO"})
	chain := m.agentChain()
	if chain != "kernel → CTO" {
		t.Fatalf("agentChain = %q, want kernel → CTO", chain)
	}

	m2 := New(nil, nil, nil, nil, nil, ModelConfig{})
	if m2.agentChain() != "kernel" {
		t.Fatalf("agentChain with no current = %q, want kernel", m2.agentChain())
	}
}

func TestAppBarIncludesAgentChain(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{CurrentAgent: "CTO"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	view := m.View()
	if !strings.Contains(view, "kernel → CTO") {
		t.Fatalf("app bar should include agent chain: %q", view)
	}
}

func TestRememberAgentTracksOrchestration(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	m.rememberAgent("CTO")
	m.rememberAgent("Go Specialist")
	m.rememberAgent("CTO") // duplicate should be ignored

	if len(m.usedAgents) != 2 {
		t.Fatalf("usedAgents = %d, want 2 (dedup)", len(m.usedAgents))
	}
	if m.usedAgents[0] != "CTO" || m.usedAgents[1] != "Go Specialist" {
		t.Fatalf("usedAgents order wrong: %v", m.usedAgents)
	}
}
