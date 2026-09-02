package terminal

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CoscaAI/cosca/internal/trace"
)

// ─── P22: Mission Control ─────────────────────────────────────────────────────

func TestMissionControlRendersMetrics(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{CurrentAgent: "CTO"})
	m.rememberAgent("CTO")
	m.tasks = []TaskInfo{{ID: "1", Name: "Build endpoint", Agent: "CTO", Status: "running"}}
	m.activeTaskName = "Build endpoint"

	view := MissionControlView(&m, 30, 20)
	for _, want := range []string{"MISSION CONTROL", "KERNEL", "TASKS", "TOOLS", "MEMORY", "KNOWLEDGE", "VERIFICATION", "ACTIVE MISSION"} {
		if !strings.Contains(view, want) {
			t.Fatalf("mission control missing %q: %q", want, view)
		}
	}
}

func TestMissionControlFillsDimensions(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	view := MissionControlView(&m, 30, 10)
	if got := lipgloss.Width(view); got != 30 {
		t.Fatalf("mission control width = %d, want 30", got)
	}
	if got := lipgloss.Height(view); got != 10 {
		t.Fatalf("mission control height = %d, want 10", got)
	}
}

func TestBuildMissionMetrics(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{CurrentAgent: "CTO"})
	m.rememberAgent("CTO")
	m.tasks = []TaskInfo{
		{ID: "1", Name: "a", Status: "running"},
		{ID: "2", Name: "b", Status: "completed"},
	}
	m.activeTaskName = "a"

	mm := buildMissionMetrics(&m)
	if mm.AgentsTotal != 1 {
		t.Fatalf("AgentsTotal = %d, want 1", mm.AgentsTotal)
	}
	if mm.TasksRunning != 1 { // 1 running task; m.busy is false in this test
		t.Fatalf("TasksRunning = %d, want 1", mm.TasksRunning)
	}
	if mm.TasksDone != 1 {
		t.Fatalf("TasksDone = %d, want 1", mm.TasksDone)
	}
	if mm.MissionName != "a" {
		t.Fatalf("MissionName = %q, want 'a'", mm.MissionName)
	}
}

func TestAlt8SwitchesToRealMissionControl(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	// Alt+8 → PanelMissionControl
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'8'}, Alt: true})
	m = updated.(Model)
	if m.activePanel != PanelMissionControl {
		t.Fatalf("activePanel = %d, want PanelMissionControl", m.activePanel)
	}

	view := m.View()
	if strings.Contains(view, "Em construção") {
		t.Fatalf("Mission Control should not be a placeholder: %q", view)
	}
	if !strings.Contains(view, "MISSION CONTROL") {
		t.Fatalf("Mission Control should render: %q", view)
	}
}

// ─── P23: Graph Mode ──────────────────────────────────────────────────────────

func TestGraphPanelRendersRelations(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{CurrentAgent: "CTO"})
	m.rememberAgent("CTO")
	// No active task name → the root node renders the literal "TASK" label.

	view := GraphPanelView(&m, 30, 20)
	for _, want := range []string{"GRAPH", "TASK", "AGENTS", "FILE", "MEM"} {
		if !strings.Contains(view, want) {
			t.Fatalf("graph panel missing %q: %q", want, view)
		}
	}
	// Box-drawing characters.
	if !strings.Contains(view, "┌") || !strings.Contains(view, "└") || !strings.Contains(view, "│") {
		t.Fatalf("graph panel missing box-drawing chars: %q", view)
	}
}

func TestGraphPanelFillsDimensions(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	view := GraphPanelView(&m, 30, 10)
	if got := lipgloss.Width(view); got != 30 {
		t.Fatalf("graph panel width = %d, want 30", got)
	}
	if got := lipgloss.Height(view); got != 10 {
		t.Fatalf("graph panel height = %d, want 10", got)
	}
}

func TestAlt9SwitchesToRealGraph(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	// Alt+9 → PanelGraph
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}, Alt: true})
	m = updated.(Model)
	if m.activePanel != PanelGraph {
		t.Fatalf("activePanel = %d, want PanelGraph", m.activePanel)
	}

	view := m.View()
	if strings.Contains(view, "Em construção") {
		t.Fatalf("Graph should not be a placeholder: %q", view)
	}
	if !strings.Contains(view, "GRAPH") {
		t.Fatalf("Graph should render: %q", view)
	}
}

// ─── P24: Verification Mode ───────────────────────────────────────────────────

func TestVerificationViewRendersResult(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	// Simulate build + test + write activity.
	m.operations.AddEvent(trace.Event{Action: ActionBuildStarted, Actor: "CTO", Result: "running", Details: "go build"})
	m.operations.AddEvent(trace.Event{Action: ActionTestStarted, Actor: "CTO", Result: "running", Details: "go test"})
	m.operations.AddEvent(trace.Event{Action: ActionToolResult, Actor: "CTO", Result: "success", Details: "write_file x.go"})

	view := VerificationView(&m)
	for _, want := range []string{"VERIFICATION", "Build", "Unit", "Integration", "Git Diff", "Policy", "RESULT"} {
		if !strings.Contains(view, want) {
			t.Fatalf("verification missing %q: %q", want, view)
		}
	}
}

func TestBuildVerificationResult(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	m.operations.AddEvent(trace.Event{Action: ActionBuildStarted, Actor: "CTO", Result: "running", Details: "go build"})
	m.operations.AddEvent(trace.Event{Action: ActionTestStarted, Actor: "CTO", Result: "running", Details: "go test"})
	m.operations.AddEvent(trace.Event{Action: ActionToolResult, Actor: "CTO", Result: "success", Details: "write_file x.go"})

	vr := buildVerificationResult(&m)
	if !vr.Build {
		t.Fatalf("Build should be true")
	}
	if !vr.Unit || !vr.Integration {
		t.Fatalf("Unit/Integration should be true")
	}
	if !vr.GitDiff {
		t.Fatalf("GitDiff should be true (write tool)")
	}
	if !vr.Policy {
		t.Fatalf("Policy should be true (no denies)")
	}
	if vr.Overall != "VERIFIED" {
		t.Fatalf("Overall = %q, want VERIFIED", vr.Overall)
	}
}

func TestAltVTogglesVerification(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	if m.verificationOpen {
		t.Fatalf("verification should start closed")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}, Alt: true})
	m = updated.(Model)
	if !m.verificationOpen {
		t.Fatalf("verification should open on Alt+V")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.verificationOpen {
		t.Fatalf("verification should close on Esc")
	}
}

func TestVerifyPaletteAction(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	var updated tea.Model
	updated, _ = m.handlePaletteAction("verify")
	m = updated.(Model)
	if !m.verificationOpen {
		t.Fatalf("verify action should open the verification overlay")
	}
}
