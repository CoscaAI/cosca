package terminal

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CoscaAI/cosca/internal/pipeline"
	"github.com/CoscaAI/cosca/internal/trace"
)

// ─── P15: Memory Explorer ─────────────────────────────────────────────────────

func TestMemoryPanelRendersEpistemicTypes(t *testing.T) {
	entries := []MemoryEntry{
		{Type: MemoryFact, Text: "user wants a health endpoint", Source: "user message"},
		{Type: MemoryDecision, Text: "Agent delegated: CTO", Source: "usedAgents"},
		{Type: MemoryEvidence, Text: "wrote internal/api/health.go", Source: "tool TOOL_RESULT"},
		{Type: MemoryInference, Text: "read config", Source: "tool TOOL_RESULT"},
	}
	view := MemoryPanelView(entries, 0, 30, 20)

	for _, want := range []string{"FACT", "DECISION", "EVIDENCE", "INFERENCE"} {
		if !strings.Contains(view, want) {
			t.Fatalf("memory panel missing type %q: %q", want, view)
		}
	}
}

func TestMemoryPanelEmpty(t *testing.T) {
	view := MemoryPanelView(nil, 0, 30, 10)
	if !strings.Contains(view, "No memory yet") {
		t.Fatalf("empty memory panel should show message: %q", view)
	}
}

func TestMemoryPanelFillsDimensions(t *testing.T) {
	entries := []MemoryEntry{{Type: MemoryFact, Text: "x", Source: "s"}}
	view := MemoryPanelView(entries, 0, 30, 10)
	if got := lipgloss.Width(view); got != 30 {
		t.Fatalf("memory panel width = %d, want 30", got)
	}
	if got := lipgloss.Height(view); got != 10 {
		t.Fatalf("memory panel height = %d, want 10", got)
	}
}

func TestBuildMemoryEntriesFromSession(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{CurrentAgent: "CTO"})
	m.appendMessage("user", "build a health endpoint")
	m.rememberAgent("CTO")
	m.operations.AddEvent(trace.Event{
		Action: ActionToolResult,
		Actor:  "CTO",
		Result: "success",
		Details: "write_file internal/api/health.go",
	})

	entries := buildMemoryEntries(&m)
	var hasFact, hasDecision, hasEvidence bool
	for _, e := range entries {
		switch e.Type {
		case MemoryFact:
			hasFact = true
		case MemoryDecision:
			hasDecision = true
		case MemoryEvidence:
			hasEvidence = true
		}
	}
	if !hasFact {
		t.Fatalf("expected a FACT entry from user message")
	}
	if !hasDecision {
		t.Fatalf("expected a DECISION entry from agent delegation")
	}
	if !hasEvidence {
		t.Fatalf("expected an EVIDENCE entry from write tool")
	}
}

func TestAlt6SwitchesToRealMemoryPanel(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	// Alt+6 → PanelMemory
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}, Alt: true})
	m = updated.(Model)
	if m.activePanel != PanelMemory {
		t.Fatalf("activePanel = %d, want PanelMemory", m.activePanel)
	}

	// The View should render the real memory explorer, not a placeholder.
	view := m.View()
	if strings.Contains(view, "Em construção") {
		t.Fatalf("Memory panel should not be a placeholder: %q", view)
	}
	if !strings.Contains(view, "MEMORY") {
		t.Fatalf("Memory panel should render: %q", view)
	}
}

func TestMemoryPanelNavigation(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	m.appendMessage("user", "first")
	m.appendMessage("user", "second")
	m.appendMessage("user", "third")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	m.activePanel = PanelMemory

	// Down moves selection.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.memorySelected != 1 {
		t.Fatalf("memorySelected = %d, want 1 after down", m.memorySelected)
	}
	// Up moves back.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.memorySelected != 0 {
		t.Fatalf("memorySelected = %d, want 0 after up", m.memorySelected)
	}
}

// ─── P16: Context Inspector ───────────────────────────────────────────────────

func TestAltIOpensContextInspector(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{CurrentAgent: "CTO"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	if m.inspectorOpen {
		t.Fatalf("inspector should start closed")
	}

	// Alt+I opens.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}, Alt: true})
	m = updated.(Model)
	if !m.inspectorOpen {
		t.Fatalf("inspector should be open after Alt+I")
	}

	// Esc closes.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.inspectorOpen {
		t.Fatalf("inspector should close on Esc")
	}
}

func TestContextInspectorViewRenders(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{CurrentAgent: "CTO"})
	m.appendMessage("user", "build endpoint")
	m.rememberAgent("CTO")
	m.hud.Tokens = pipeline.TokenUsage{Input: 18432, Output: 512}

	view := ContextInspectorView(&m)
	for _, want := range []string{"CONTEXT", "Direct:", "Messages:", "Memory:", "Knowledge:", "Agents:", "Tokens:", "Context:"} {
		if !strings.Contains(view, want) {
			t.Fatalf("context inspector missing %q: %q", want, view)
		}
	}
	// Agent chain should be present.
	if !strings.Contains(view, "CTO") {
		t.Fatalf("context inspector missing agent: %q", view)
	}
}

func TestBuildContextSummary(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{CurrentAgent: "CTO"})
	m.appendMessage("user", "build endpoint")
	m.rememberAgent("CTO")
	m.hud.Tokens = pipeline.TokenUsage{Input: 1024, Output: 256}

	cs := buildContextSummary(&m)
	if cs.Messages != 1 {
		t.Fatalf("Messages = %d, want 1", cs.Messages)
	}
	if len(cs.Agents) != 1 || cs.Agents[0] != "CTO" {
		t.Fatalf("Agents = %v, want [CTO]", cs.Agents)
	}
	if cs.TokensIn != 1024 || cs.TokensOut != 256 {
		t.Fatalf("Tokens = %d/%d, want 1024/256", cs.TokensIn, cs.TokensOut)
	}
	if cs.TokenLimit != 128*1024 {
		t.Fatalf("TokenLimit = %d, want 131072", cs.TokenLimit)
	}
}

func TestPaletteIncludesContextCommand(t *testing.T) {
	p := NewPalette()
	ids := map[string]bool{}
	for _, c := range p.commands {
		ids[c.ID] = true
	}
	if !ids["inspect-context"] {
		t.Fatalf("palette missing inspect-context command")
	}
}

func TestInspectContextPaletteAction(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	var updated tea.Model
	updated, _ = m.handlePaletteAction("inspect-context")
	m = updated.(Model)
	if !m.inspectorOpen {
		t.Fatalf("inspect-context action should open the inspector")
	}
}
