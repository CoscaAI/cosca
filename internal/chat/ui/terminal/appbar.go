package terminal

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── App Bar (R1) ─────────────────────────────────────────────────────────────
//
// The app bar is a SINGLE thin line of chrome at the top of the terminal that
// merges what used to be three separate rows (header, tab bar, breadcrumbs)
// into one OpenCode-style bar:
//
//	◉ COSCA TERMINAL | Chat Files Diff Tasks ... | kernel → cto | ● ready
//
// This frees vertical space for the actual content and looks clean.

// renderAppBar renders the single-line top chrome: brand + tabs + agent chain
// + status. It replaces renderHeader + renderTabBar + renderBreadcrumbs.
func (m Model) renderAppBar() string {
	bg := lipgloss.NewStyle().Background(th.BgAlt)

	// Brand.
	brand := lipgloss.NewStyle().
		Foreground(th.Accent).
		Bold(true).
		Background(th.BgAlt).
		Padding(0, 1).
		Render("◉ COSCA")

	// Tabs (compact).
	var tabs []string
	for _, p := range m.panels {
		name := panelNames[p]
		if p == PanelDiff && m.diffDirty {
			name = "•" + name
		}
		if p == m.activePanel {
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(th.Surface).
				Background(th.Primary).
				Bold(true).
				Padding(0, 1).
				Render(name))
		} else {
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(th.MutedLight).
				Background(th.BgAlt).
				Padding(0, 1).
				Render(name))
		}
	}
	tabStr := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	// Agent chain (right side).
	chain := m.agentChain()
	chainStr := lipgloss.NewStyle().
		Foreground(th.MutedLight).
		Background(th.BgAlt).
		Padding(0, 1).
		Render(chain)

	// Status dot.
	status := "ready"
	statusColor := th.Success
	if m.busy {
		status = "busy"
		statusColor = th.Warning
	} else if m.streaming {
		status = "streaming"
		statusColor = th.Accent2
	}
	statusStr := lipgloss.NewStyle().
		Foreground(statusColor).
		Background(th.BgAlt).
		Bold(true).
		Padding(0, 1).
		Render("● " + status)

	// Compose left (brand + tabs) and right (chain + status).
	left := lipgloss.JoinHorizontal(lipgloss.Top, brand, tabStr)
	right := lipgloss.JoinHorizontal(lipgloss.Top, chainStr, statusStr)

	// Fill the middle with the background so the bar spans the full width.
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	midW := m.width - leftW - rightW
	if midW < 0 {
		midW = 0
	}
	mid := bg.Render(strings.Repeat(" ", midW))

	full := lipgloss.JoinHorizontal(lipgloss.Top, left, mid, right)
	return lipgloss.NewStyle().Width(m.width).Background(th.BgAlt).Render(full)
}

// agentChain returns the current delegation chain as a compact breadcrumb.
func (m Model) agentChain() string {
	chain := []string{"kernel"}
	if m.currentAgent != "" {
		chain = append(chain, m.currentAgent)
	}
	return strings.Join(chain, " → ")
}
