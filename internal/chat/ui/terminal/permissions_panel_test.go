package terminal

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CoscaAI/cosca/internal/trace"
)

// ─── P19: Permission Center ───────────────────────────────────────────────────

func TestPermissionsPanelRendersVerdicts(t *testing.T) {
	rules := []PermissionRule{
		{Resource: "filesystem project/*", Verdict: VerdictAllow},
		{Resource: "shell rm", Verdict: VerdictAsk},
		{Resource: "deploy production", Verdict: VerdictApproval},
		{Resource: "secrets read", Verdict: VerdictDeny},
	}
	view := PermissionsPanelView(rules, 0, 30, 20)

	for _, want := range []string{"ALLOW", "ASK", "APPROVAL", "DENY"} {
		if !strings.Contains(view, want) {
			t.Fatalf("permissions panel missing verdict %q: %q", want, view)
		}
	}
}

func TestPermissionsPanelEmpty(t *testing.T) {
	view := PermissionsPanelView(nil, 0, 30, 10)
	if !strings.Contains(view, "No rules yet") {
		t.Fatalf("empty permissions panel should show message: %q", view)
	}
}

func TestPermissionsPanelFillsDimensions(t *testing.T) {
	rules := []PermissionRule{{Resource: "x", Verdict: VerdictAllow}}
	view := PermissionsPanelView(rules, 0, 30, 10)
	if got := lipgloss.Width(view); got != 30 {
		t.Fatalf("permissions panel width = %d, want 30", got)
	}
	if got := lipgloss.Height(view); got != 10 {
		t.Fatalf("permissions panel height = %d, want 10", got)
	}
}

func TestBuildPermissionRulesFromSession(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	// A write tool call → ALLOW; a destructive shell → APPROVAL.
	m.operations.AddEvent(trace.Event{
		Action: ActionToolResult,
		Actor:  "CTO",
		Result: "success",
		Details: "write_file internal/api/health.go",
	})
	m.operations.AddEvent(trace.Event{
		Action: ActionToolResult,
		Actor:  "CTO",
		Result: "success",
		Details: "rm -rf /tmp/cache",
	})

	rules := buildPermissionRules(&m)
	var hasAllow, hasApproval bool
	for _, r := range rules {
		switch r.Verdict {
		case VerdictAllow:
			hasAllow = true
		case VerdictApproval:
			hasApproval = true
		}
	}
	if !hasAllow {
		t.Fatalf("expected an ALLOW rule from write tool")
	}
	if !hasApproval {
		t.Fatalf("expected an APPROVAL rule from destructive tool")
	}
}

func TestAlt7SwitchesToRealPermissionsPanel(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	// Alt+7 → PanelPermissions
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'7'}, Alt: true})
	m = updated.(Model)
	if m.activePanel != PanelPermissions {
		t.Fatalf("activePanel = %d, want PanelPermissions", m.activePanel)
	}

	// The View should render the real permissions panel, not a placeholder.
	view := m.View()
	if strings.Contains(view, "Em construção") {
		t.Fatalf("Permissions panel should not be a placeholder: %q", view)
	}
	if !strings.Contains(view, "PERMISSIONS") {
		t.Fatalf("Permissions panel should render: %q", view)
	}
}

func TestPermissionsPanelNavigation(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	m.activePanel = PanelPermissions

	// Down moves selection.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.permissionSelected != 1 {
		t.Fatalf("permissionSelected = %d, want 1 after down", m.permissionSelected)
	}
	// Up moves back.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.permissionSelected != 0 {
		t.Fatalf("permissionSelected = %d, want 0 after up", m.permissionSelected)
	}
}

// ─── P20: Computer Mode ───────────────────────────────────────────────────────

func TestComputerModeViewRendersCapabilities(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	view := ComputerModeView(&m, 30)

	for _, want := range []string{"terminal", "filesystem", "browser", "clipboard", "processes", "network", "secrets"} {
		if !strings.Contains(view, want) {
			t.Fatalf("computer mode missing capability %q: %q", want, view)
		}
	}
	// States: ok (✓), approval (⚠), denied (✕).
	if !strings.Contains(view, "✓") {
		t.Fatalf("computer mode missing ok state: %q", view)
	}
	if !strings.Contains(view, "⚠") {
		t.Fatalf("computer mode missing approval state: %q", view)
	}
	if !strings.Contains(view, "✕") {
		t.Fatalf("computer mode missing denied state: %q", view)
	}
}

func TestBuildComputerCapabilitiesReflectsUsage(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	m.operations.AddEvent(trace.Event{
		Action: ActionToolResult,
		Actor:  "CTO",
		Result: "success",
		Details: "bash go test ./...",
	})

	caps := buildComputerCapabilities(&m)
	var terminalState string
	for _, c := range caps {
		if c.Name == "terminal" {
			terminalState = c.State
		}
	}
	if terminalState != "ok" {
		t.Fatalf("terminal capability state = %q, want ok (used)", terminalState)
	}
}

func TestComputerPaletteAction(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	var updated tea.Model
	updated, _ = m.handlePaletteAction("computer")
	m = updated.(Model)
	if m.activePanel != PanelPermissions {
		t.Fatalf("computer action should open Permissions panel, got %d", m.activePanel)
	}
}
