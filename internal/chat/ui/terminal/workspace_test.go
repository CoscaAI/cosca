package terminal

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestWorkspaceKeysMapToPanels(t *testing.T) {
	cases := []struct {
		key  string
		want PanelID
	}{
		{"alt+1", PanelChat},
		{"alt+2", PanelFiles},
		{"alt+3", PanelAgents},
		{"alt+4", PanelOperations},
		{"alt+5", PanelTasks},
		{"alt+6", PanelMemory},
		{"alt+7", PanelGit},
		{"alt+8", PanelDeploy},
		{"alt+9", PanelGraph},
	}
	for _, c := range cases {
		got, ok := workspaceKeys[c.key]
		if !ok {
			t.Fatalf("workspaceKeys missing %q", c.key)
		}
		if got != c.want {
			t.Fatalf("workspaceKeys[%q] = %d, want %d", c.key, got, c.want)
		}
	}
}

func TestWorkspaceKeysSwitchPanel(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	// Alt+3 → Agents placeholder
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}, Alt: true})
	m = updated.(Model)
	if m.activePanel != PanelAgents {
		t.Fatalf("activePanel = %d, want PanelAgents", m.activePanel)
	}

	// Alt+6 → Memory placeholder
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}, Alt: true})
	m = updated.(Model)
	if m.activePanel != PanelMemory {
		t.Fatalf("activePanel = %d, want PanelMemory", m.activePanel)
	}

	// Alt+1 → back to Chat
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true})
	m = updated.(Model)
	if m.activePanel != PanelChat {
		t.Fatalf("activePanel = %d, want PanelChat", m.activePanel)
	}
}

func TestPanelPhaseMapping(t *testing.T) {
	cases := []struct {
		p    PanelID
		want string
	}{
		{PanelMemory, "Fase 4"},
		{PanelGit, "Fase 5"},
		{PanelDeploy, "Fase 6"},
		{PanelGraph, "Fase 7"},
		{PanelChat, ""},
		{PanelFiles, ""},
		{PanelTasks, ""},
		{PanelAgents, ""}, // Agents is real since Fase 3
	}
	for _, c := range cases {
		if got := panelPhase(c.p); got != c.want {
			t.Fatalf("panelPhase(%d) = %q, want %q", c.p, got, c.want)
		}
	}
}

func TestPlaceholderPanelViewRenders(t *testing.T) {
	view := PlaceholderPanelView("Agents", "Fase 3", 30, 10)
	if !strings.Contains(view, "Agents") {
		t.Fatalf("placeholder missing title: %q", view)
	}
	if !strings.Contains(view, "Fase 3") {
		t.Fatalf("placeholder missing phase: %q", view)
	}
	if got := lipgloss.Width(view); got != 30 {
		t.Fatalf("placeholder width = %d, want 30", got)
	}
}

func TestPaletteIncludesPhase2Commands(t *testing.T) {
	p := NewPalette()
	ids := map[string]bool{}
	for _, c := range p.commands {
		ids[c.ID] = true
	}
	for _, want := range []string{
		"toggle-hud", "clear-context", "panel-agents", "panel-memory",
		"panel-git", "panel-deploy", "panel-graph", "status",
		"theme-cosca", "theme-petrol", "theme-tokyonight", "theme-opencode",
	} {
		if !ids[want] {
			t.Fatalf("palette missing command %q", want)
		}
	}
}

func TestPaletteFuzzyFilter(t *testing.T) {
	p := NewPalette()
	p = p.Open()

	// "panel" should surface all workspace panel commands
	p = p.filter("panel")
	if len(p.filtered) == 0 {
		t.Fatalf("filter 'panel' returned no results")
	}
	for _, c := range p.filtered {
		if !strings.Contains(strings.ToLower(c.Name), "panel") {
			t.Fatalf("filter 'panel' matched non-panel command %q", c.Name)
		}
	}

	// "theme cosca" should rank the cosca theme first
	p = p.filter("theme cosca")
	if len(p.filtered) == 0 {
		t.Fatalf("filter 'theme cosca' returned no results")
	}
	if p.filtered[0].ID != "theme-cosca" {
		t.Fatalf("top result = %q, want theme-cosca", p.filtered[0].ID)
	}
}

func TestToggleHudAction(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	if !m.showHUD {
		t.Fatalf("showHUD should default to true")
	}

	var updated tea.Model
	updated, _ = m.handlePaletteAction("toggle-hud")
	m = updated.(Model)
	if m.showHUD {
		t.Fatalf("showHUD should be false after toggle")
	}

	updated, _ = m.handlePaletteAction("toggle-hud")
	m = updated.(Model)
	if !m.showHUD {
		t.Fatalf("showHUD should be true after second toggle")
	}
}

func TestClearContextAction(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{CurrentAgent: "test"})
	m.appendMessage("user", "hello")
	m.pendingPrompts = []string{"queued"}
	m.usedAgents = []string{"test"}

	var updated tea.Model
	updated, _ = m.handlePaletteAction("clear-context")
	m = updated.(Model)

	if len(m.messages) != 1 {
		t.Fatalf("messages = %d, want 1 (the info confirmation)", len(m.messages))
	}
	if len(m.pendingPrompts) != 0 {
		t.Fatalf("pendingPrompts = %d, want 0", len(m.pendingPrompts))
	}
	if len(m.usedAgents) != 0 {
		t.Fatalf("usedAgents = %d, want 0", len(m.usedAgents))
	}
}

func TestStatusLine(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{
		CurrentAgent: "kernel",
		CurrentModel: "test-model",
	})
	line := m.statusLine()
	if !strings.Contains(line, "status:") {
		t.Fatalf("statusLine missing status: %q", line)
	}
	if !strings.Contains(line, "model:test-model") {
		t.Fatalf("statusLine missing model: %q", line)
	}
	if !strings.Contains(line, "theme:") {
		t.Fatalf("statusLine missing theme: %q", line)
	}
}
