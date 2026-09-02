package terminal

import (
	"fmt"
	"sort"
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
		{ID: "theme-cosca", Name: "Theme: Cosca", Description: "Premium dark theme (default)", Category: "Appearance"},
		{ID: "theme-opencode", Name: "Theme: OpenCode", Description: "OpenCode-inspired dark theme", Category: "Appearance"},
		{ID: "theme-tokyonight", Name: "Theme: TokyoNight", Description: "TokyoNight dark theme", Category: "Appearance"},
		{ID: "theme-petrol", Name: "Theme: Petrol", Description: "Cosca's petroleum & black theme", Category: "Appearance"},
		{ID: "toggle-hud", Name: "Toggle: Status Bar", Description: "Show/hide the bottom status bar", Category: "Appearance"},
		{ID: "panel-chat", Name: "Panel: Chat", Description: "Alt+1 — main conversation", Category: "Workspace"},
		{ID: "panel-files", Name: "Panel: Files", Description: "Alt+2 — changed files", Category: "Workspace"},
		{ID: "panel-agents", Name: "Panel: Agents", Description: "Alt+3 — agent hierarchy", Category: "Workspace"},
		{ID: "panel-operations", Name: "Panel: Operations", Description: "Alt+4 — execution tree", Category: "Workspace"},
		{ID: "panel-tasks", Name: "Panel: Tasks", Description: "Alt+5 — task list", Category: "Workspace"},
		{ID: "panel-memory", Name: "Panel: Memory", Description: "Alt+6 — memory (Fase 4)", Category: "Workspace"},
		{ID: "panel-git", Name: "Panel: Git", Description: "Alt+7 — git (Fase 5)", Category: "Workspace"},
		{ID: "panel-deploy", Name: "Panel: Deploy", Description: "Alt+8 — deploy (Fase 6)", Category: "Workspace"},
		{ID: "panel-graph", Name: "Panel: Graph", Description: "Alt+9 — graph (Fase 7)", Category: "Workspace"},
		{ID: "panel-diff", Name: "Panel: Diff", Description: "Ctrl+D — git diff", Category: "Workspace"},
		{ID: "mode-advanced", Name: "Mode: Advanced", Description: "Enable advanced mode", Category: "Session"},
		{ID: "mode-simple", Name: "Mode: Simple", Description: "Switch to simple mode", Category: "Session"},
		{ID: "clear-chat", Name: "Clear Chat", Description: "Clear all chat messages", Category: "Session"},
		{ID: "clear-context", Name: "Clear Context", Description: "Reset session context & agents", Category: "Session"},
		{ID: "session-info", Name: "Session Info", Description: "Show session summary", Category: "Session"},
		{ID: "status", Name: "Status", Description: "One-line session status", Category: "Session"},
		{ID: "quit", Name: "Quit", Description: "Exit terminal", Category: "Session"},
		{ID: "keyboard-shortcuts", Name: "Help: Keys", Description: "Show keyboard shortcuts", Category: "Help"},
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

	type scored struct {
		cmd   PaletteCommand
		idx   int
		score int
	}
	ql := strings.ToLower(q)
	var matched []scored
	for i, cmd := range p.commands {
		if s := fuzzyScore(ql, cmd); s > 0 {
			matched = append(matched, scored{cmd: cmd, idx: i, score: s})
		}
	}
	sort.SliceStable(matched, func(a, b int) bool {
		if matched[a].score != matched[b].score {
			return matched[a].score > matched[b].score
		}
		return matched[a].idx < matched[b].idx
	})

	p.filtered = make([]PaletteCommand, 0, len(matched))
	for _, sm := range matched {
		p.filtered = append(p.filtered, sm.cmd)
	}
	if p.cursor >= len(p.filtered) {
		p.cursor = 0
	}
	return p
}

// fuzzyScore ranks a command against a query without external libs. Exact
// prefix matches on the name dominate; then in-name, in-category and
// in-description matches; finally an ordered token-subsequence bonus.
func fuzzyScore(q string, cmd PaletteCommand) int {
	if q == "" {
		return 1
	}
	name := strings.ToLower(cmd.Name)
	cat := strings.ToLower(cmd.Category)
	desc := strings.ToLower(cmd.Description)

	score := 0
	if name == q {
		score += 1000
	}
	if strings.HasPrefix(name, q) {
		score += 500
	}
	if strings.Contains(name, q) {
		score += 200
	}
	if strings.Contains(cat, q) {
		score += 120
	}
	if strings.Contains(desc, q) {
		score += 60
	}
	// Ordered token subsequence across "name category".
	tokens := strings.Fields(q)
	score += fuzzySubsequence(tokens, name+" "+cat)
	return score
}

// fuzzySubsequence rewards tokens that appear in order in hay.
func fuzzySubsequence(tokens []string, hay string) int {
	bonus := 0
	pos := 0
	matchedAny := false
	for _, tok := range tokens {
		idx := strings.Index(hay[pos:], tok)
		if idx < 0 {
			return -1 // one missing token kills the subsequence bonus
		}
		pos += idx + len(tok)
		bonus += 25
		matchedAny = true
	}
	if !matchedAny {
		return 0
	}
	return bonus
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
	total := len(p.filtered)
	allTotal := len(p.commands)
	if p.query != "" && total != allTotal {
		b.WriteString(paletteHintStyle.Render(fmt.Sprintf("%d/%d results", total, allTotal)))
	} else {
		b.WriteString(paletteHintStyle.Render(fmt.Sprintf("%d commands", total)))
	}
	b.WriteString(" · ")
	b.WriteString(paletteHintStyle.Render("↑/↓ navigate · Enter select · Esc dismiss"))

	return paletteOverlayStyle.Render(b.String())
}
