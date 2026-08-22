package terminal

import (
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestThemeSelection(t *testing.T) {
	SetTheme("petrol")
	if ThemeName() != "petrol" {
		t.Fatalf("ThemeName() = %q, want petrol", ThemeName())
	}
	if th.Background != theme.Petrol.Background {
		t.Fatalf("selected background = %q, want %q", th.Background, theme.Petrol.Background)
	}
	if appStyle.GetBackground() != theme.Petrol.Background {
		t.Fatalf("app background = %v, want %v", appStyle.GetBackground(), theme.Petrol.Background)
	}
	if inputBoxStyle.GetBackground() != theme.Petrol.InputBackground {
		t.Fatalf("input background = %v, want %v", inputBoxStyle.GetBackground(), theme.Petrol.InputBackground)
	}

	SetTheme("opencode")
	if ThemeName() != "opencode" {
		t.Fatalf("ThemeName() = %q, want opencode", ThemeName())
	}

	SetTheme("tokyo")
	if ThemeName() != "tokyonight" {
		t.Fatalf("ThemeName() = %q, want tokyonight", ThemeName())
	}

	SetTheme("theme-petrol")
	if ThemeName() != "petrol" {
		t.Fatalf("ThemeName() = %q after alias, want petrol", ThemeName())
	}
}

func TestPetrolAppStyleEmitsTrueColor(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	SetTheme("petrol")
	lipgloss.SetColorProfile(termenv.TrueColor)
	rendered := appStyle.Render("petrol")

	sequence := "\x1b[38;2;243;247;247;48;2;9;47;51m" // #F4F7F7 on #092F33
	if !strings.Contains(rendered, sequence) {
		t.Fatalf("rendered style %q does not contain %q", rendered, sequence)
	}
}

func TestSidePanelsUseRequestedDimensions(t *testing.T) {
	SetTheme("petrol")
	if got := lipgloss.Width(TaskPanelView(nil, 30, 10)); got != 30 {
		t.Fatalf("empty task panel width = %d, want 30", got)
	}
	if got := lipgloss.Height(TaskPanelView(nil, 30, 10)); got != 10 {
		t.Fatalf("empty task panel height = %d, want 10", got)
	}
	if got := lipgloss.Width(FilesPanelView(nil, 30, 10, 0)); got != 30 {
		t.Fatalf("empty files panel width = %d, want 30", got)
	}
	if got := lipgloss.Height(FilesPanelView(nil, 30, 10, 0)); got != 10 {
		t.Fatalf("empty files panel height = %d, want 10", got)
	}
}

func TestViewFillsWindow(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	if got := lipgloss.Width(m.View()); got != 100 {
		t.Fatalf("view width = %d, want 100", got)
	}
	if got := lipgloss.Height(m.View()); got != 30 {
		t.Fatalf("view height = %d, want 30", got)
	}

	for _, panel := range []PanelID{PanelTasks, PanelFiles} {
		m.activePanel = panel
		if got := lipgloss.Width(m.View()); got != 100 {
			t.Fatalf("panel %d view width = %d, want 100", panel, got)
		}
		if got := lipgloss.Height(m.View()); got != 30 {
			t.Fatalf("panel %d view height = %d, want 30", panel, got)
		}
	}
}
