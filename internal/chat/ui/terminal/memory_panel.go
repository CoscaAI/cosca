package terminal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Memory Explorer (P15) ────────────────────────────────────────────────────
//
// The Memory panel exposes the session's epistemology — the kinds of knowledge
// the terminal has accumulated and WHY it believes each item. This is the
// "killer feature" that OpenCode does not reproduce: a traceable chain of
// FACT → DECISION → INFERENCE → EVIDENCE.
//
// DATA SOURCE: the terminal does not (yet) have direct access to the runtime's
// memory DB. The panel is built over data the terminal ALREADY holds from the
// current session:
//
//	FACT      ← user messages (intent/requirements)
//	DECISION  ← agents delegated (usedAgents) + mode toggles
//	INFERENCE ← assistant results / tool outcomes
//	EVIDENCE  ← write/edit tool calls (what was actually changed)
//
// TODO(runtime): when the runtime exposes a memory API (e.g. memory.Engine
// Query), replace buildMemoryEntries with a real retrieval and hydrate the
// same MemoryEntry struct. The struct and rendering are already prepared for
// that swap.

// MemoryType is the epistemological class of a memory entry.
type MemoryType string

const (
	MemoryFact      MemoryType = "FACT"
	MemoryDecision  MemoryType = "DECISION"
	MemoryInference MemoryType = "INFERENCE"
	MemoryEvidence  MemoryType = "EVIDENCE"
)

// MemoryEntry is a single epistemologically-typed memory item.
type MemoryEntry struct {
	Type   MemoryType
	Text   string
	Source string // where it came from (e.g. "user message", "tool write_file")
	Detail string // expanded "why does cosca believe this?" chain
}

// memoryTypeStyle returns the badge style for a memory type.
func memoryTypeStyle(t MemoryType) lipgloss.Style {
	switch t {
	case MemoryFact:
		return lipgloss.NewStyle().Foreground(colorBlue).Bold(true)
	case MemoryDecision:
		return lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	case MemoryInference:
		return lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	default: // EVIDENCE
		return lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	}
}

// buildMemoryEntries derives the session's epistemological memory from the
// model's live state. It is the terminal-side derivation; see the TODO above
// for the runtime swap point.
func buildMemoryEntries(m *Model) []MemoryEntry {
	var entries []MemoryEntry

	// FACT: user messages express intent/requirements.
	for _, msg := range m.messages {
		if msg.Role == "user" && strings.TrimSpace(msg.Content) != "" {
			entries = append(entries, MemoryEntry{
				Type:   MemoryFact,
				Text:   truncateStr(msg.Content, 60),
				Source: "user message",
				Detail: fmt.Sprintf("user said: %q", truncateStr(msg.Content, 120)),
			})
		}
	}

	// DECISION: agents delegated in this session.
	for _, a := range m.usedAgents {
		entries = append(entries, MemoryEntry{
			Type:   MemoryDecision,
			Text:   "Agent delegated: " + a,
			Source: "usedAgents",
			Detail: fmt.Sprintf("decision: %s was delegated control in this session", a),
		})
	}
	if m.currentAgent != "" {
		entries = append(entries, MemoryEntry{
			Type:   MemoryDecision,
			Text:   "Active agent: " + m.currentAgent,
			Source: "currentAgent",
			Detail: fmt.Sprintf("decision: %s currently holds control", m.currentAgent),
		})
	}

	// EVIDENCE: write/edit tool calls (what was actually changed).
	for _, ev := range m.operations.entries {
		if ev.Action == ActionToolResult {
			risk := toolRiskLevel(ev.Details)
			if risk == "write" || risk == "destructive" {
				entries = append(entries, MemoryEntry{
					Type:   MemoryEvidence,
					Text:   truncateStr(ev.Details, 60),
					Source: "tool " + ev.Action,
					Detail: fmt.Sprintf("evidence: tool %s by %s → %s", ev.Action, ev.Actor, truncateStr(ev.Details, 120)),
				})
			}
		}
	}

	// INFERENCE: assistant results / tool outcomes.
	for _, ev := range m.operations.entries {
		if ev.Action == ActionToolResult && toolRiskLevel(ev.Details) == "read" {
			entries = append(entries, MemoryEntry{
				Type:   MemoryInference,
				Text:   truncateStr(ev.Details, 60),
				Source: "tool " + ev.Action,
				Detail: fmt.Sprintf("inference: %s observed %s", ev.Actor, truncateStr(ev.Details, 120)),
			})
		}
	}

	// Cap the list to keep the panel responsive.
	if len(entries) > 200 {
		entries = entries[:200]
	}
	return entries
}

// MemoryPanelView renders the memory explorer in the right side panel.
func MemoryPanelView(entries []MemoryEntry, selected int, width, height int) string {
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
	b.WriteString(sidePanelBadge("MEMORY", "[session]", false))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGray).
		Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n")

	if len(entries) == 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(colorGrayLight).
			Italic(true).
			Render("No memory yet — run a task."))
		return borderStyle.Height(contentHeight).Render(b.String())
	}

	displayCount := contentHeight - 3
	if displayCount < 1 {
		displayCount = 1
	}
	for i, e := range entries {
		if i >= displayCount {
			b.WriteString(lipgloss.NewStyle().
				Foreground(colorGrayLight).
				Italic(true).
				Render(fmt.Sprintf("  ... %d more", len(entries)-displayCount)))
			break
		}
		row := renderMemoryRow(e, contentWidth)
		if i == selected {
			row = lipgloss.NewStyle().
				Background(th.Selection).
				Foreground(colorFg).
				Render(row)
		}
		b.WriteString(row)
		b.WriteString("\n")

		// Expanded detail for the selected entry (P15: WHY DOES COSCA BELIEVE THIS?).
		if i == selected && e.Detail != "" {
			detail := lipgloss.NewStyle().
				Foreground(colorTextMuted).
				Italic(true).
				Render("  ↳ " + truncateStr(e.Detail, contentWidth-4))
			b.WriteString(detail)
			b.WriteString("\n")
		}
	}

	return borderStyle.Height(contentHeight).Render(b.String())
}

// renderMemoryRow renders a single memory entry with its type badge.
func renderMemoryRow(e MemoryEntry, width int) string {
	badge := memoryTypeStyle(e.Type).Render(" " + string(e.Type) + " ")
	text := lipgloss.NewStyle().Foreground(th.Text).Render(truncateStr(e.Text, width-14))
	return fmt.Sprintf(" %s %s", badge, text)
}

// countMemoryByType returns a breakdown of memory entries by epistemological type.
func countMemoryByType(entries []MemoryEntry) map[MemoryType]int {
	counts := map[MemoryType]int{}
	for _, e := range entries {
		counts[e.Type]++
	}
	return counts
}

// ─── Context Inspector (P16) ──────────────────────────────────────────────────
//
// The Context Inspector overlay answers "what entered the agent's context and
// why". It is derived from the terminal's live state: messages (Direct),
// usedAgents (Agents), operations events (Memory/Knowledge), and token usage.

// ContextSummary is the data shown by the Context Inspector overlay.
type ContextSummary struct {
	DirectFiles  int
	Messages     int
	MemoryItems  int
	Knowledge    int
	Agents       []string
	CurrentAgent string
	TokensIn     int
	TokensOut    int
	TokenLimit   int
	ActiveOps    int
}

// buildContextSummary derives the context summary from the model's live state.
func buildContextSummary(m *Model) ContextSummary {
	// Direct: count distinct file references in messages + file entries.
	direct := len(m.fileEntries)
	for _, msg := range m.messages {
		if refs := resolveFileReference(msg.Content); len(refs) > 0 {
			direct += len(refs)
		}
	}

	// Memory/Knowledge: derived from recall/search tool calls in operations.
	memItems := 0
	knowledge := 0
	for _, ev := range m.operations.entries {
		lower := strings.ToLower(ev.Action + " " + ev.Details)
		if strings.Contains(lower, "recall") || strings.Contains(lower, "memory") {
			memItems++
		}
		if strings.Contains(lower, "knowledge") || strings.Contains(lower, "search") {
			knowledge++
		}
	}

	return ContextSummary{
		DirectFiles:  direct,
		Messages:     len(m.messages),
		MemoryItems:  memItems,
		Knowledge:    knowledge,
		Agents:       m.usedAgents,
		CurrentAgent: m.currentAgent,
		TokensIn:     m.hud.Tokens.Input,
		TokensOut:    m.hud.Tokens.Output,
		TokenLimit:   128 * 1024,
		ActiveOps:    m.operations.active,
	}
}

// ContextInspectorView renders the centered overlay for the Context Inspector.
func ContextInspectorView(m *Model) string {
	cs := buildContextSummary(m)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGold).
		Bold(true).
		Render(" CONTEXT INSPECTOR "))
	b.WriteString("\n\n")

	row := func(label, value string) {
		b.WriteString(lipgloss.NewStyle().
			Foreground(colorGrayLight).
			Render(fmt.Sprintf("  %-10s", label)))
		b.WriteString(lipgloss.NewStyle().
			Foreground(th.Text).
			Render(value))
		b.WriteString("\n")
	}

	row("Direct:", fmt.Sprintf("%d files", cs.DirectFiles))
	row("Messages:", fmt.Sprintf("%d", cs.Messages))
	row("Memory:", fmt.Sprintf("%d relevant", cs.MemoryItems))
	row("Knowledge:", fmt.Sprintf("%d chunks", cs.Knowledge))

	// Agents chain.
	agents := "kernel"
	if len(cs.Agents) > 0 {
		agents = strings.Join(cs.Agents, " → ")
	}
	if cs.CurrentAgent != "" {
		agents += " → " + cs.CurrentAgent
	}
	row("Agents:", agents)

	// Tokens.
	total := cs.TokensIn + cs.TokensOut
	row("Tokens:", fmt.Sprintf("%.1fk / %.1fk (in/out)", float64(cs.TokensIn)/1024, float64(cs.TokensOut)/1024))

	// Context usage vs limit.
	limit := cs.TokenLimit
	if limit <= 0 {
		limit = 128 * 1024
	}
	pct := float64(total) / float64(limit)
	if pct > 1 {
		pct = 1
	}
	bar := renderTaskBar(pct, 20)
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGrayLight).
		Render("  Context:  "))
	b.WriteString(bar)
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGrayLight).
		Render(fmt.Sprintf(" %d / %dk", total, limit/1024)))
	b.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGray).
		Italic(true).
		Render("  Esc to close · Alt+6 for the Memory Explorer"))

	return paletteOverlayStyle.Render(b.String())
}
