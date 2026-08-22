package terminal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PaletteCommand struct {
	ID          string
	Name        string
	Description string
	Category    string
}

type PaletteModel struct {
	open     bool
	commands []PaletteCommand
	filtered []PaletteCommand
	cursor   int
	query    string
	input    textinput.Model
}

func NewPalette() PaletteModel {
	ti := textinput.New()
	ti.Placeholder = "Search commands..."
	ti.Prompt = "> "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorFg).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorTextMuted)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorFg)
	ti.CharLimit = 256

	return PaletteModel{
		open:     false,
		commands: defaultCommands(),
		filtered: defaultCommands(),
		cursor:   0,
		input:    ti,
	}
}

func defaultCommands() []PaletteCommand {
	return []PaletteCommand{
		{ID: "theme-petrol", Name: "Theme: Petrol", Description: "Switch to Cosca's petroleum and black theme", Category: "Appearance"},
		{ID: "theme-opencode", Name: "Theme: OpenCode", Description: "Switch to OpenCode theme (dark, modern)", Category: "Appearance"},
		{ID: "theme-tokyonight", Name: "Theme: TokyoNight", Description: "Switch to TokyoNight theme (legacy)", Category: "Appearance"},
		{ID: "mode-advanced", Name: "Mode: Advanced", Description: "Enable advanced mode", Category: "Session"},
		{ID: "mode-simple", Name: "Mode: Simple", Description: "Switch to simple mode", Category: "Session"},
		{ID: "clear-chat", Name: "Clear Chat", Description: "Clear all chat messages", Category: "Session"},
		{ID: "export-session", Name: "Export Session", Description: "Export session to file", Category: "Session"},
		{ID: "session-info", Name: "Session Info", Description: "Show session info", Category: "Session"},
		{ID: "keyboard-shortcuts", Name: "Keyboard Shortcuts", Description: "Show keyboard shortcuts", Category: "Help"},
		{ID: "panel-chat", Name: "Panel: Chat", Description: "Switch to Chat panel", Category: "Navigation"},
		{ID: "panel-tasks", Name: "Panel: Tasks", Description: "Switch to Tasks panel", Category: "Navigation"},
		{ID: "panel-files", Name: "Panel: Files", Description: "Switch to Files panel", Category: "Navigation"},
		{ID: "panel-diff", Name: "Panel: Diff", Description: "Show Git diff panel", Category: "Navigation"},
		{ID: "panel-system", Name: "Panel: System", Description: "Show system info panel", Category: "Navigation"},
		{ID: "model-selector", Name: "Model Selector", Description: "Select AI model", Category: "Model"},
		{ID: "agent-selector", Name: "Agent Selector", Description: "Select agent", Category: "Agent"},
		{ID: "quit", Name: "Quit", Description: "Exit terminal", Category: "Session"},
		{ID: "help-slash", Name: "Help: Slash Commands", Description: "List slash commands", Category: "Help"},
		{ID: "help-colon", Name: "Help: Colon Commands", Description: "List colon commands", Category: "Help"},
	}
}

func (p PaletteModel) Open() PaletteModel {
	p.open = true
	p.cursor = 0
	p.query = ""
	p.filtered = p.commands
	p.input.SetValue("")
	p.input.Focus()
	return p
}

func (p PaletteModel) Close() PaletteModel {
	p.open = false
	return p
}

func (p PaletteModel) IsOpen() bool {
	return p.open
}

func (p PaletteModel) filter(q string) PaletteModel {
	p.query = q
	if q == "" {
		p.filtered = p.commands
		p.cursor = 0
		return p
	}

	lower := strings.ToLower(q)
	var filtered []PaletteCommand
	for _, cmd := range p.commands {
		if strings.Contains(strings.ToLower(cmd.Name), lower) ||
			strings.Contains(strings.ToLower(cmd.Description), lower) ||
			strings.Contains(strings.ToLower(cmd.Category), lower) {
			filtered = append(filtered, cmd)
		}
	}

	p.filtered = filtered
	if p.cursor >= len(filtered) {
		p.cursor = 0
	}
	return p
}

func (p PaletteModel) Update(msg tea.Msg) (PaletteModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return p.Close(), nil

		case "ctrl+p":
			return p.Close(), nil

		case "up":
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil

		case "down":
			if p.cursor < len(p.filtered)-1 {
				p.cursor++
			}
			return p, nil
		}
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	cmds = append(cmds, cmd)

	p = p.filter(p.input.Value())

	return p, tea.Batch(cmds...)
}

func (p PaletteModel) SelectedCommand() *PaletteCommand {
	if len(p.filtered) == 0 || p.cursor >= len(p.filtered) {
		return nil
	}
	return &p.filtered[p.cursor]
}

func (p PaletteModel) View() string {
	if !p.open {
		return ""
	}

	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGold).
		Bold(true).
		Render(" Command Palette "))
	b.WriteString("\n")

	b.WriteString(paletteInputStyle.Render(p.input.View()))
	b.WriteString("\n\n")

	if len(p.filtered) == 0 {
		b.WriteString(paletteHintStyle.Render("No matching commands"))
	} else {
		visible := 12
		start := 0
		if p.cursor >= visible {
			start = p.cursor - visible + 1
		}
		end := start + visible
		if end > len(p.filtered) {
			end = len(p.filtered)
		}

		currentCategory := ""
		for i := start; i < end; i++ {
			cmd := p.filtered[i]

			if cmd.Category != currentCategory {
				currentCategory = cmd.Category
				b.WriteString(paletteCategoryStyle.Render(currentCategory))
				b.WriteString("\n")
			}

			if i == p.cursor {
				b.WriteString(paletteItemSelectedStyle.Render(
					fmt.Sprintf("  %-30s %s", cmd.Name, truncateStr(cmd.Description, 40)),
				))
			} else {
				b.WriteString(paletteItemStyle.Render(
					fmt.Sprintf("  %-30s %s", cmd.Name, truncateStr(cmd.Description, 40)),
				))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(paletteHintStyle.Render("Type to filter · ↑/↓ navigate · Enter select · Esc dismiss"))

	return paletteOverlayStyle.Render(b.String())
}
