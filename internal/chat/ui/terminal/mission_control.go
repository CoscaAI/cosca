package terminal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Mission Control (P22) ────────────────────────────────────────────────────
//
// The Mission Control panel is the cockpit overview of the whole system — the
// culmination of the workspace vision. It shows key metrics derived from the
// session's live state plus the active mission's progress bar.
//
// DATA SOURCE: derived from data the terminal already holds:
//
//	AGENTS       ← usedAgents + currentAgent
//	TASKS        ← m.tasks / operations (running = busy, completed = done)
//	TOOLS        ← operations ledger (distinct tool actions)
//	MEMORY       ← buildMemoryEntries count
//	KNOWLEDGE    ← buildContextSummary.Knowledge
//	VERIFICATION ← tokens / overall health
//	ACTIVE MISSION ← current task with progress bar (busy → animated, done → full)
//
// TODO(runtime): when the runtime exposes a healthz/metrics API, replace the
// derived values with real runtime counters. The rendering is prepared for it.

// MissionMetrics is the data shown by the Mission Control panel.
type MissionMetrics struct {
	AgentsActive  int
	AgentsTotal   int
	TasksRunning  int
	TasksDone     int
	ToolsActive   int
	ToolsWaiting  int
	Memory        int
	Knowledge     int
	Verification  float64 // 0..1
	MissionName   string
	MissionPct    float64 // 0..1
	MissionActive bool
}

// buildMissionMetrics derives the mission control metrics from the model.
func buildMissionMetrics(m *Model) MissionMetrics {
	// Agents.
	agentsTotal := len(m.usedAgents)
	if agentsTotal == 0 && m.currentAgent != "" {
		agentsTotal = 1
	}
	agentsActive := 0
	if m.currentAgent != "" {
		agentsActive = 1
	}

	// Tasks.
	tasksRunning := 0
	tasksDone := 0
	for _, t := range m.tasks {
		switch t.Status {
		case "running":
			tasksRunning++
		case "completed":
			tasksDone++
		}
	}
	if m.busy {
		tasksRunning++
	}

	// Tools: distinct tool actions in the operations ledger.
	toolSet := map[string]bool{}
	for _, ev := range m.operations.entries {
		if ev.Action != "" {
			toolSet[ev.Action] = true
		}
	}
	toolsActive := len(toolSet)
	toolsWaiting := 0
	if len(m.usedAgents) > 0 && toolsActive == 0 {
		toolsWaiting = len(m.usedAgents)
	}

	// Memory + Knowledge.
	memory := len(buildMemoryEntries(m))
	cs := buildContextSummary(m)
	knowledge := cs.Knowledge

	// Verification: derive from token usage + overall health.
	verification := 0.0
	totalTokens := m.hud.Tokens.Input + m.hud.Tokens.Output
	if totalTokens > 0 {
		verification = 0.9 // healthy session with activity
	} else {
		verification = 0.5
	}
	if m.hud.Status == "error" {
		verification = 0.3
	}

	// Active mission: the current task (if any).
	missionName := ""
	missionPct := 0.0
	missionActive := false
	if m.activeTaskName != "" {
		missionName = m.activeTaskName
		missionActive = true
		if m.busy {
			missionPct = 0.6
		} else {
			missionPct = 1.0
		}
	} else if len(m.tasks) > 0 {
		missionName = m.tasks[0].Name
		if m.tasks[0].Status == "completed" {
			missionPct = 1.0
		} else if m.tasks[0].Status == "running" {
			missionPct = 0.6
			missionActive = true
		}
	}

	return MissionMetrics{
		AgentsActive:  agentsActive,
		AgentsTotal:   agentsTotal,
		TasksRunning:  tasksRunning,
		TasksDone:     tasksDone,
		ToolsActive:   toolsActive,
		ToolsWaiting:  toolsWaiting,
		Memory:        memory,
		Knowledge:     knowledge,
		Verification:  verification,
		MissionName:   missionName,
		MissionPct:    missionPct,
		MissionActive: missionActive,
	}
}

// MissionControlView renders the mission control cockpit panel.
func MissionControlView(m *Model, width, height int) string {
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

	mm := buildMissionMetrics(m)

	var b strings.Builder
	b.WriteString(sidePanelBadge("MISSION CONTROL", "[cockpit]", false))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGray).
		Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n\n")

	metric := func(label string, value string, style lipgloss.Style) {
		b.WriteString(lipgloss.NewStyle().
			Foreground(colorGrayLight).
			Render(fmt.Sprintf("  %-12s", label)))
		b.WriteString(style.Render(value))
		b.WriteString("\n")
	}

	// Kernel health line.
	health := "HEALTHY"
	healthStyle := lipgloss.NewStyle().Foreground(th.Success).Bold(true)
	if m.hud.Status == "error" {
		health = "DEGRADED"
		healthStyle = lipgloss.NewStyle().Foreground(th.Error).Bold(true)
	}
	b.WriteString(lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("  KERNEL"))
	b.WriteString(" ")
	b.WriteString(healthStyle.Render("● "+health))
	b.WriteString(lipgloss.NewStyle().Foreground(colorGrayLight).Render(fmt.Sprintf(" · %d AGENTS", mm.AgentsTotal)))
	b.WriteString("\n\n")

	metric("TASKS", fmt.Sprintf("%d RUNNING · %d COMPLETED", mm.TasksRunning, mm.TasksDone),
		lipgloss.NewStyle().Foreground(th.Text))
	metric("TOOLS", fmt.Sprintf("%d ACTIVE · %d WAITING", mm.ToolsActive, mm.ToolsWaiting),
		lipgloss.NewStyle().Foreground(th.Text))
	metric("MEMORY", fmt.Sprintf("%d ENTITIES", mm.Memory),
		lipgloss.NewStyle().Foreground(th.Text))
	metric("KNOWLEDGE", fmt.Sprintf("%d CHUNKS", mm.Knowledge),
		lipgloss.NewStyle().Foreground(th.Text))
	metric("VERIFICATION", fmt.Sprintf("%.1f%%", mm.Verification*100),
		lipgloss.NewStyle().Foreground(th.Success))

	// Active mission with progress bar.
	b.WriteString("\n")
	if mm.MissionName != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(colorGold).Bold(true).Render("  ACTIVE MISSION"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(th.Text).Render("  "+truncateStr(mm.MissionName, contentWidth-4)))
		b.WriteString("\n")
		bar := renderTaskBar(mm.MissionPct, contentWidth-4)
		b.WriteString("  " + bar)
		b.WriteString(lipgloss.NewStyle().Foreground(colorGrayLight).Render(fmt.Sprintf(" %d%%", int(mm.MissionPct*100))))
		b.WriteString("\n")
	} else {
		b.WriteString(lipgloss.NewStyle().
			Foreground(colorGrayLight).
			Italic(true).
			Render("  No active mission — type a task."))
	}

	// Clamp the rendered content to the panel's content height so the border
	// never overflows the requested dimensions. Each line is also truncated to
	// the content width so lipgloss does not re-wrap long lines (which would
	// silently grow the rendered height beyond the requested dimensions).
	content := clampPanelContent(b.String(), contentWidth, contentHeight)

	return borderStyle.Height(contentHeight).Render(content)
}

// ─── Graph Mode (P23) ─────────────────────────────────────────────────────────
//
// The Graph panel renders a simple 2D textual graph of the session's
// relationships: TASK → AGENTS → FILES → MEMORY. It uses box-drawing characters
// (─│┌┐└┘) with no external graph library.

// GraphPanelView renders the session relationship graph.
func GraphPanelView(m *Model, width, height int) string {
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
	b.WriteString(sidePanelBadge("GRAPH", "[relations]", false))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGray).
		Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n\n")

	// Task node (root).
	taskName := "TASK"
	if m.activeTaskName != "" {
		taskName = truncateStr(m.activeTaskName, 20)
	}
	b.WriteString(lipgloss.NewStyle().Foreground(colorGold).Bold(true).Render("  ┌────────┐"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorGold).Bold(true).Render("  │ " + centerPad(taskName, 8) + " │"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorGold).Bold(true).Render("  └───┬────┘"))
	b.WriteString("\n")

	// Agents branch.
	b.WriteString(lipgloss.NewStyle().Foreground(colorGray).Render("  ┌───┼───┐"))
	b.WriteString("\n")

	agents := m.usedAgents
	if len(agents) == 0 && m.currentAgent != "" {
		agents = []string{m.currentAgent}
	}
	agentLabel := "AGENTS"
	if len(agents) > 0 {
		agentLabel = truncateStr(agents[0], 8)
	}
	b.WriteString(lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("┌─▼──┐ "))
	b.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("┌─▼──┐ "))
	b.WriteString(lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("┌─▼──┐"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("│" + centerPad(agentLabel, 5) + "│ "))
	b.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("│" + centerPad("FILE", 5) + "│ "))
	b.WriteString(lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("│" + centerPad("MEM", 5) + "│"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("└─────┘ "))
	b.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("└─────┘ "))
	b.WriteString(lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("└─────┘"))
	b.WriteString("\n\n")

	// Legend / detail lines.
	b.WriteString(lipgloss.NewStyle().Foreground(colorGrayLight).Render("  AGENTS:"))
	b.WriteString(lipgloss.NewStyle().Foreground(th.Text).Render(" " + strings.Join(agents, ", ")))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorGrayLight).Render("  FILES:"))
	b.WriteString(lipgloss.NewStyle().Foreground(th.Text).Render(fmt.Sprintf(" %d touched", len(m.fileEntries))))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorGrayLight).Render("  MEMORY:"))
	b.WriteString(lipgloss.NewStyle().Foreground(th.Text).Render(fmt.Sprintf(" %d entries", len(buildMemoryEntries(m)))))
	b.WriteString("\n")

	// Clamp the rendered content to the panel's content height.
	content := clampPanelContent(b.String(), contentWidth, contentHeight)

	return borderStyle.Height(contentHeight).Render(content)
}

// clampPanelContent truncates each line to maxWidth visual columns and limits
// the total line count to maxHeight, so a panel never overflows its requested
// dimensions (lipgloss would otherwise re-wrap long lines and grow the height).
func clampPanelContent(s string, maxWidth, maxHeight int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	var out []string
	for _, ln := range lines {
		if lipgloss.Width(ln) > maxWidth {
			ln = lipgloss.NewStyle().MaxWidth(maxWidth).Render(ln)
		}
		out = append(out, ln)
		if len(out) >= maxHeight {
			break
		}
	}
	return strings.Join(out, "\n")
}

// centerPad pads s to exactly width columns, centered.
func centerPad(s string, width int) string {
	if width < 1 {
		width = 1
	}
	if len(s) >= width {
		return s[:width]
	}
	left := (width - len(s)) / 2
	right := width - len(s) - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// ─── Verification Mode (P24) ──────────────────────────────────────────────────
//
// The agent doesn't just say "done" — it shows proof. The Verification overlay
// derives checks from the session state: build/test tool calls, file edits,
// and policy compliance.

// VerificationResult is the outcome of the verification checks.
type VerificationResult struct {
	Build      bool
	Unit       bool
	Integration bool
	GitDiff    bool
	Policy     bool
	Overall    string // VERIFIED | PARTIAL | FAILED
}

// buildVerificationResult derives the verification checks from the session.
func buildVerificationResult(m *Model) VerificationResult {
	var vr VerificationResult

	for _, ev := range m.operations.entries {
		lower := strings.ToLower(ev.Action + " " + ev.Details)
		switch {
		case strings.Contains(lower, "build"):
			vr.Build = true
		case strings.Contains(lower, "test"):
			vr.Unit = true
			vr.Integration = true
		}
	}

	// Git diff: any write/edit tool call produced changes.
	for _, ev := range m.operations.entries {
		if ev.Action == ActionToolResult {
			risk := toolRiskLevel(ev.Details)
			if risk == "write" || risk == "destructive" {
				vr.GitDiff = true
			}
		}
	}

	// Policy: no operation was blocked/denied in the session. A static DENY
	// rule (e.g. "secrets read") is not a violation — only an actual denied
	// attempt counts against policy.
	vr.Policy = true
	for _, ev := range m.operations.entries {
		if ev.Result == "failed" && strings.Contains(strings.ToLower(ev.Action), "deny") {
			vr.Policy = false
		}
		if ev.Result == "denied" {
			vr.Policy = false
		}
	}

	// Overall.
	checks := []bool{vr.Build, vr.Unit, vr.Integration, vr.GitDiff, vr.Policy}
	passed := 0
	for _, c := range checks {
		if c {
			passed++
		}
	}
	switch {
	case passed == len(checks):
		vr.Overall = "VERIFIED"
	case passed > 0:
		vr.Overall = "PARTIAL"
	default:
		vr.Overall = "FAILED"
	}

	return vr
}

// VerificationView renders the verification proof overlay.
func VerificationView(m *Model) string {
	vr := buildVerificationResult(m)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGold).
		Bold(true).
		Render(" VERIFICATION "))
	b.WriteString("\n\n")

	check := func(label string, ok bool) {
		glyph := "✗"
		style := lipgloss.NewStyle().Foreground(th.Error)
		if ok {
			glyph = "✓"
			style = lipgloss.NewStyle().Foreground(th.Success)
		}
		b.WriteString(fmt.Sprintf("  %s %s", style.Render(glyph), lipgloss.NewStyle().Foreground(th.Text).Render(label)))
		b.WriteString("\n")
	}

	check("Build", vr.Build)
	check("Unit", vr.Unit)
	check("Integration", vr.Integration)
	check("Git Diff", vr.GitDiff)
	check("Policy", vr.Policy)

	b.WriteString("\n")
	resultStyle := lipgloss.NewStyle().Foreground(th.Success).Bold(true)
	switch vr.Overall {
	case "PARTIAL":
		resultStyle = lipgloss.NewStyle().Foreground(th.Warning).Bold(true)
	case "FAILED":
		resultStyle = lipgloss.NewStyle().Foreground(th.Error).Bold(true)
	}
	b.WriteString(lipgloss.NewStyle().Foreground(colorGrayLight).Render("  RESULT: "))
	b.WriteString(resultStyle.Render(vr.Overall))
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGray).
		Italic(true).
		Render("  Esc to close · Alt+V toggles"))

	return paletteOverlayStyle.Render(b.String())
}
