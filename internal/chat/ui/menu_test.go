package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestMenu_OpensOnSlash verifica que o menu abre quando o input começa com "/".
func TestMenu_OpensOnSlash(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.ready = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = mustModel(t, updated)

	if !m.menu.visible {
		t.Fatal("menu should open when input starts with /")
	}
	out := m.View()
	clean := cleanRendered(out)
	if !strings.Contains(clean, "/kernel") {
		t.Error("menu should list commands like /kernel")
	}
}

// TestMenu_Filter verifica o filtro live: "/k" mostra apenas comandos com k.
func TestMenu_Filter(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.ready = true

	m.menu.visible = true
	m.menu.query = "k"
	items := m.menu.filtered()
	if len(items) == 0 {
		t.Fatal("filter 'k' should match /kernel and /skills")
	}
	for _, it := range items {
		if !strings.HasPrefix(strings.ToLower(it.name), "/k") &&
			!strings.Contains(strings.ToLower(it.name), "k") {
			t.Errorf("filter 'k' matched unexpected item %q", it.name)
		}
	}
}

// TestMenu_EnterExecutes verifica que Enter executa o comando selecionado.
func TestMenu_EnterExecutes(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.ready = true

	m.menu.visible = true
	m.menu.query = "stat"
	m.menu.cursor = 0

	updated, _ := m.handleMenuKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	if m.menu.visible {
		t.Error("menu should close after enter")
	}
	found := false
	for _, msg := range m.messages {
		if msg.role == "slash" && strings.Contains(msg.content, "Engine") {
			found = true
		}
	}
	if !found {
		t.Error("/status should have been executed via menu")
	}
}

// TestMenu_EscapeCloses verifica que Esc fecha o menu.
func TestMenu_EscapeCloses(t *testing.T) {
	m := newTestModel()
	m.menu.visible = true
	m.menu.query = "help"

	updated, _ := m.handleMenuKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mustModel(t, updated)

	if m.menu.visible {
		t.Error("esc should close the menu")
	}
}

// TestMenu_TabCompletes verifica que Tab completa o comando no input.
func TestMenu_TabCompletes(t *testing.T) {
	m := newTestModel()
	m.menu.visible = true
	m.menu.query = "hel"
	m.menu.cursor = 0

	updated, _ := m.handleMenuKey(tea.KeyMsg{Type: tea.KeyTab})
	m = mustModel(t, updated)

	if m.input.Value() != "/help" {
		t.Errorf("tab should complete to /help, got %q", m.input.Value())
	}
}

// TestMenu_ArrowNav verifica a navegação com setas.
func TestMenu_ArrowNav(t *testing.T) {
	m := newTestModel()
	m.menu.visible = true
	m.menu.query = ""
	m.menu.cursor = 0

	updated, _ := m.handleMenuKey(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)

	if m.menu.cursor != 1 {
		t.Errorf("down should advance cursor, got %d", m.menu.cursor)
	}
}

// TestLogo_Renders verifica que o logo ASCII aparece na tela de boas-vindas.
func TestLogo_Renders(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 40
	m.ready = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = mustModel(t, updated)

	out := m.View()
	clean := cleanRendered(out)
	if !strings.Contains(clean, "████") {
		t.Error("logo should render block characters (ASCII art)")
	}
}
