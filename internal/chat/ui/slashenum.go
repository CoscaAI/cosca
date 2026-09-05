package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Slash Command Menu (estilo opencode) ─────────────────────────────────────
// Ao digitar "/", abre um menu de comandos filtrado por prefixo. Navegação:
// ↑/↓ para mover, Enter executa, Esc fecha, Tab completa. Filtro live: digitar
// "/k" mostra apenas os comandos começando com k.

// slashCommand descreve um comando disponível no menu.
type slashCommand struct {
	name        string
	description string
}

// slashCommands é o catálogo completo do menu.
var slashCommands = []slashCommand{
	{"/kernel", "identity, laws and constitution of the Kernel"},
	{"/status", "engine state, session and streaming"},
	{"/agents", "capos of the family (automatic routing)"},
	{"/skills", "available skills arsenal"},
	{"/workflows", "available workflows"},
	{"/memory <q>", "semantic memory query"},
	{"/ver <img>", "view an image (local OCR)"},
	{"/model", "model selector (Ctrl+P also opens)"},
	{"/clear", "clear visual history"},
	{"/help", "full help panel"},
	{"/exit", "exit chat"},
}

// slashMenu é o estado do menu de autocomplete.
type slashMenu struct {
	visible  bool
	query    string // texto digitado após o "/" (filtro live)
	cursor   int    // item selecionado
	hasFocus bool   // o filtro está ativo (digitação continua filtrando)
}

// ─── Filtro ───────────────────────────────────────────────────────────────────

// filtered retorna os comandos que casam com a consulta atual.
func (sm slashMenu) filtered() []slashCommand {
	q := strings.ToLower(sm.query)
	out := make([]slashCommand, 0, len(slashCommands))
	for _, c := range slashCommands {
		if q == "" || strings.HasPrefix(strings.ToLower(c.name), q) ||
			strings.Contains(strings.ToLower(c.name), q) {
			out = append(out, c)
		}
	}
	return out
}

// clampCursor mantém o cursor dentro dos limites da lista.
func (sm slashMenu) clampCursor() slashMenu {
	if n := len(sm.filtered()); n == 0 {
		sm.cursor = 0
	} else if sm.cursor >= n {
		sm.cursor = n - 1
	} else if sm.cursor < 0 {
		sm.cursor = 0
	}
	return sm
}

// ─── Ações ────────────────────────────────────────────────────────────────────

// applyFilter atualiza o filtro (chamado a cada tecla durante o menu).
func (sm slashMenu) applyFilter(q string) slashMenu {
	sm.query = q
	sm.cursor = 0
	sm.hasFocus = true
	return sm
}

// completeTo preenche a query com o comando selecionado (Tab).
func (sm slashMenu) completeTo() string {
	items := sm.filtered()
	if len(items) == 0 {
		return ""
	}
	if sm.cursor >= len(items) {
		sm.cursor = len(items) - 1
	}
	return items[sm.cursor].name
}

// ─── View ─────────────────────────────────────────────────────────────────────

// slashMenuStyle é a moldura do menu.
var slashMenuStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorBorder).
	Background(colorBackgroundAlt).
	Padding(0, 1)

// slashMenuTitleStyle é o cabeçalho do menu.
var slashMenuTitleStyle = lipgloss.NewStyle().
	Foreground(colorGold).
	Bold(true)

// slashMenuItemStyle é um item não selecionado.
var slashMenuItemStyle = lipgloss.NewStyle().
	Foreground(colorForeground)

// slashMenuSelectedStyle é o item selecionado.
var slashMenuSelectedStyle = lipgloss.NewStyle().
	Foreground(colorBackground).
	Background(colorGold).
	Bold(true).
	Padding(0, 1)

// slashMenuDimStyle é um item filtrável fora do foco.
var slashMenuDimStyle = lipgloss.NewStyle().
	Foreground(colorGray)

// View renderiza o menu (ou vazio quando oculto).
func (sm slashMenu) View(width int) string {
	if !sm.visible {
		return ""
	}
	items := sm.filtered()

	var b strings.Builder
	b.WriteString(slashMenuTitleStyle.Render("  /" + sm.query))
	b.WriteString("\n")

	if len(items) == 0 {
		b.WriteString(slashMenuDimStyle.Render("  nenhum comando encontrado"))
		b.WriteString("\n")
	} else {
		for i, c := range items {
			prefix := " "
			line := fmt.Sprintf(" %s %s", prefix, c.name)
			if i == sm.cursor {
				b.WriteString(slashMenuSelectedStyle.Render(line))
				b.WriteString(slashMenuDimStyle.Render("  " + c.description))
			} else {
				b.WriteString(slashMenuItemStyle.Render(line))
				b.WriteString(slashMenuDimStyle.Render("  " + c.description))
			}
			b.WriteString("\n")
		}
	}

	// Dica de navegação.
	b.WriteString(slashMenuDimStyle.Render("  ↑/↓ mover · Enter executar · Esc fechar · Tab completar"))

	// Limita a largura para não estourar a tela.
	menu := slashMenuStyle.Render(strings.TrimSuffix(b.String(), "\n"))
	if width > 4 {
		menu = lipgloss.NewStyle().MaxWidth(width - 4).Render(menu)
	}
	return menu
}
