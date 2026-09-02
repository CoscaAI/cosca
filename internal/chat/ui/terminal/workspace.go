package terminal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PlaceholderPanelView renders the "workspace coming soon" panel for panels
// that belong to later roadmap phases (Agents → Fase 3, Memory → Fase 4,
// Git → Fase 5, Deploy → Fase 6, Graph → Fase 7). It follows the same framed
// look as the Files/Diff side panels so the full workspace feels reachable.
func PlaceholderPanelView(title, phase string, width, height int) string {
	panelW := width
	if panelW < 20 {
		panelW = 40
	}
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.BorderSubtle).
		Background(th.BackgroundPanel).
		Foreground(th.Text).
		Padding(0, 1)
	styleWidth := panelW - borderStyle.GetHorizontalBorderSize()
	contentWidth := styleWidth - borderStyle.GetHorizontalPadding()
	if contentWidth < 8 {
		contentWidth = 8
	}
	borderStyle = borderStyle.Width(styleWidth)
	contentHeight := height - borderStyle.GetVerticalFrameSize()
	if contentHeight < 3 {
		contentHeight = 3
	}

	var b strings.Builder
	b.WriteString(sidePanelBadge(strings.ToUpper(title), phase, false))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGray).
		Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n\n")

	b.WriteString(lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true).
		Render("◈"))
	b.WriteString(" ")
	b.WriteString(lipgloss.NewStyle().
		Foreground(th.Text).
		Render(fmt.Sprintf("%s workspace", title)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGrayLight).
		Italic(true).
		Render("Em construção — chega na " + phase + "."))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGray).
		Render("Alt+1..9 alterna os painéis."))

	return borderStyle.Height(contentHeight).Render(b.String())
}

// renderAgentsRail is the thin left column shown on very wide terminals
// (>= wideRailWidth): the agents that participated in the current session,
// with the active agent highlighted.
func renderAgentsRail(agents []string, current string, width, height int) string {
	if width < 8 {
		width = 8
	}
	contentW := width - 2
	if contentW < 4 {
		contentW = 4
	}
	railStyle := lipgloss.NewStyle().
		Background(th.BgAlt).
		Width(width).
		Height(height).
		Padding(0, 1)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().
		Foreground(th.TextMuted).
		Bold(true).
		Render("AGENTS"))
	b.WriteString("\n")

	if len(agents) == 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(th.Muted).
			Render(truncateStr("idle", contentW)))
		return railStyle.Render(b.String())
	}

	for _, a := range agents {
		dot := "○"
		dotStyle := lipgloss.NewStyle().Foreground(th.Muted)
		nameStyle := lipgloss.NewStyle().Foreground(th.TextMuted)
		if a == current {
			dot = "●"
			dotStyle = lipgloss.NewStyle().Foreground(th.Accent)
			nameStyle = lipgloss.NewStyle().Foreground(th.Text).Bold(true)
		}
		line := dotStyle.Render(dot) + " " + nameStyle.Render(truncateStr(a, contentW-2))
		b.WriteString(line)
		b.WriteString("\n")
	}

	return railStyle.Render(strings.TrimRight(b.String(), "\n"))
}
