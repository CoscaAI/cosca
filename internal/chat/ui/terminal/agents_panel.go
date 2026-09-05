package terminal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Agent hierarchy (P11) ────────────────────────────────────────────────────
//
// The Agents panel renders the DON → KERNEL → CHIEFS → specialists delegation
// chain of the current session. The runtime does not expose a full live
// hierarchy, so the tree is derived from the agents actually used in the
// session (usedAgents) plus the current agent, laid out over the canonical
// Cosca org structure. Each agent carries a status:
//
//	ACTIVE  (● accent)  — the current agent
//	DONE    (✓ success) — an agent that already worked in this session
//	WAITING (○ muted)   — not used yet (future delegation)
//
// The tree is purely informational (visual), matching the "Mission Control"
// vision without requiring interactivity.

// AgentStatus is the display status of an agent in the hierarchy.
type AgentStatus string

const (
	AgentActive  AgentStatus = "active"
	AgentDone    AgentStatus = "done"
	AgentWaiting AgentStatus = "waiting"
)

// AgentNode is a single row in the agents hierarchy tree.
type AgentNode struct {
	Name     string
	Status   AgentStatus
	Children []*AgentNode
}

// agentStatusGlyph returns the glyph + style for an agent status.
func agentStatusGlyph(s AgentStatus) (string, lipgloss.Style) {
	switch s {
	case AgentActive:
		return "●", lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	case AgentDone:
		return "✓", lipgloss.NewStyle().Foreground(th.Success)
	default:
		return "○", lipgloss.NewStyle().Foreground(th.Muted)
	}
}

// buildAgentHierarchy derives the DON → KERNEL → CHIEFS → specialists tree from
// the agents used in the session. `used` is the ordered list of agents that
// worked (usedAgents), `current` is the active agent. The canonical org is
// seeded with the framework chiefs; agents actually used are marked DONE, the
// current one ACTIVE, and the rest WAITING.
func buildAgentHierarchy(used []string, current string) []*AgentNode {
	statusOf := func(name string) AgentStatus {
		if name == current {
			return AgentActive
		}
		for _, u := range used {
			if u == name {
				return AgentDone
			}
		}
		return AgentWaiting
	}

	// Canonical Cosca org: DON (root) → KERNEL → chiefs → specialists.
	// Chiefs are the framework's department heads that can be delegated to.
	chiefs := []string{
		"CTO", "Security", "Verification", "QA", "Backend",
		"Frontend", "Database", "DevOps", "Architecture",
	}

	// Specialists that report to the relevant chiefs.
	backendSpecs := []string{"Go Specialist", "API Specialist", "Service Specialist"}
	qaSpecs := []string{"Unit Test", "Integration Test", "E2E Test"}
	dbSpecs := []string{"SQL Specialist"}

	// Build the chiefs node list, marking status from session usage.
	var chiefNodes []*AgentNode
	for _, c := range chiefs {
		chiefNodes = append(chiefNodes, &AgentNode{Name: c, Status: statusOf(c)})
	}

	// Attach specialists under their chiefs.
	for _, cn := range chiefNodes {
		switch cn.Name {
		case "Backend":
			for _, s := range backendSpecs {
				cn.Children = append(cn.Children, &AgentNode{Name: s, Status: statusOf(s)})
			}
		case "QA":
			for _, s := range qaSpecs {
				cn.Children = append(cn.Children, &AgentNode{Name: s, Status: statusOf(s)})
			}
		case "Database":
			for _, s := range dbSpecs {
				cn.Children = append(cn.Children, &AgentNode{Name: s, Status: statusOf(s)})
			}
		}
	}

	kernel := &AgentNode{Name: "KERNEL", Status: statusOf("KERNEL"), Children: chiefNodes}

	// DON is the root orchestrator.
	don := &AgentNode{Name: "DON", Status: statusOf("DON"), Children: []*AgentNode{kernel}}

	return []*AgentNode{don}
}

// AgentsPanelView renders the agents hierarchy in the right side panel.
func AgentsPanelView(used []string, current string, width, height int) string {
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
	b.WriteString(sidePanelBadge("AGENTS", "[session]", false))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGray).
		Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n")

	// Render the hierarchy tree.
	tree := buildAgentHierarchy(used, current)
	rows := flattenAgentNodes(tree, 0)
	displayCount := contentHeight - 3
	if displayCount < 1 {
		displayCount = 1
	}
	for i, row := range rows {
		if i >= displayCount {
			b.WriteString(lipgloss.NewStyle().
				Foreground(colorGrayLight).
				Italic(true).
				Render(fmt.Sprintf("  ... %d more", len(rows)-displayCount)))
			break
		}
		b.WriteString(row)
		b.WriteString("\n")
	}

	return borderStyle.Height(contentHeight).Render(b.String())
}

// flattenAgentNodes renders the tree nodes into display rows with indentation.
func flattenAgentNodes(nodes []*AgentNode, depth int) []string {
	var rows []string
	for _, n := range nodes {
		glyph, style := agentStatusGlyph(n.Status)
		indent := strings.Repeat("  ", depth)
		connector := ""
		if depth > 0 {
			connector = "└── "
		}
		nameStyle := lipgloss.NewStyle().Foreground(th.Text)
		if n.Status == AgentActive {
			nameStyle = lipgloss.NewStyle().Foreground(th.Text).Bold(true)
		} else if n.Status == AgentWaiting {
			nameStyle = lipgloss.NewStyle().Foreground(th.TextMuted)
		}
		rows = append(rows, fmt.Sprintf("%s%s%s %s",
			indent, connector, style.Render(glyph), nameStyle.Render(n.Name)))
		if len(n.Children) > 0 {
			rows = append(rows, flattenAgentNodes(n.Children, depth+1)...)
		}
	}
	return rows
}
