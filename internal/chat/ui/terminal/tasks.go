package terminal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TaskInfo is a display-friendly wrapper around pipeline task data.
type TaskInfo struct {
	ID         string
	Name       string
	Agent      string
	Status     string
	Progress   float64
	Error      string
	HandoffTo  string
	DurationMs int64
}

// TaskStage describes a named phase of a task with a progress fraction.
type TaskStage struct {
	Name string
	Pct  float64 // 0..1
}

// TaskPanelView renders the task panel for the right side panel. When a task
// is running it shows per-stage progress bars and the per-agent delegation
// line (P12); otherwise it lists tasks compactly as before.
func TaskPanelView(tasks []TaskInfo, width, height int) string {
	styleWidth := width - taskPanelStyle.GetHorizontalBorderSize()
	if styleWidth < 1 {
		styleWidth = 1
	}
	contentWidth := styleWidth - taskPanelStyle.GetHorizontalPadding()
	if contentWidth < 1 {
		contentWidth = 1
	}
	contentHeight := height - taskPanelStyle.GetVerticalFrameSize()
	if contentHeight < 1 {
		contentHeight = 1
	}
	panel := taskPanelStyle.Width(styleWidth).Height(contentHeight)

	if len(tasks) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(colorGrayLight).
			Italic(true).
			Render("No active tasks")
		return panel.Render(empty)
	}

	var b strings.Builder
	b.WriteString(titleSubStyle.Render(" TASKS "))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", contentWidth))
	b.WriteString("\n")

	displayCount := contentHeight - 4
	if displayCount < 1 {
		displayCount = 1
	}

	// The first task gets the rich per-stage view; the rest are compact rows.
	for i, t := range tasks {
		if i >= displayCount {
			b.WriteString(infoBubble.Render(fmt.Sprintf("  ... %d more", len(tasks)-displayCount)))
			break
		}
		if i == 0 {
			b.WriteString(renderTaskDetail(t, contentWidth))
		} else {
			b.WriteString(renderTaskRow(t, contentWidth))
		}
		b.WriteString("\n")
	}

	return panel.Render(b.String())
}

// renderTaskDetail renders the rich per-task view with stage progress bars and
// the per-agent delegation line (P12). It is used for the active/first task.
func renderTaskDetail(t TaskInfo, width int) string {
	var b strings.Builder

	// Header: TASK #ID — STATUS
	statusLabel := strings.ToUpper(t.Status)
	if t.Status == "" {
		statusLabel = "PENDING"
	}
	statusStyle := taskPendingStyle
	switch t.Status {
	case "running":
		statusStyle = taskRunningStyle
	case "completed":
		statusStyle = taskDoneStyle
	case "failed":
		statusStyle = taskFailedStyle
	}
	header := lipgloss.NewStyle().Foreground(colorGold).Bold(true).Render("TASK")
	if t.ID != "" {
		header += " " + lipgloss.NewStyle().Foreground(colorGrayLight).Render("#"+t.ID)
	}
	header += " — " + statusStyle.Render(statusLabel)
	b.WriteString(header)
	b.WriteString("\n")

	// Name line (if present).
	if t.Name != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(th.Text).Render("  " + truncateStr(t.Name, width-4)))
		b.WriteString("\n")
	}

	// Stage progress bars.
	stages := taskStagesFor(t)
	barWidth := width - 14
	if barWidth < 8 {
		barWidth = 8
	}
	for _, s := range stages {
		label := lipgloss.NewStyle().Foreground(colorGrayLight).Render(fmt.Sprintf("  %-14s", truncateStr(s.Name, 14)))
		bar := renderTaskBar(s.Pct, barWidth)
		b.WriteString(label + " " + bar)
		b.WriteString("\n")
	}

	// Per-agent delegation line.
	if t.Agent != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(colorGray).Render("  Agents: "))
		b.WriteString(renderAgentChip(t.Agent, t.Status))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

// taskStagesFor derives the stage progress for a task from its status. When
// the runtime exposes real stage data this should be replaced; for now it
// derives a visual breakdown from the task status.
func taskStagesFor(t TaskInfo) []TaskStage {
	stages := []TaskStage{
		{Name: "Planning", Pct: 0},
		{Name: "Research", Pct: 0},
		{Name: "Implementation", Pct: 0},
		{Name: "Tests", Pct: 0},
	}
	switch t.Status {
	case "completed":
		for i := range stages {
			stages[i].Pct = 1
		}
	case "failed":
		for i := range stages {
			stages[i].Pct = 1
		}
	case "running":
		// A running task has finished planning/research and is mid-implementation.
		stages[0].Pct = 1
		stages[1].Pct = 1
		stages[2].Pct = 0.6
		stages[3].Pct = 0
	default: // pending / waiting
		stages[0].Pct = 0
	}
	return stages
}

// renderTaskBar renders a block progress bar of the given width.
func renderTaskBar(pct float64, width int) string {
	if width < 5 {
		width = 10
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return progressFillStyle.Render(strings.Repeat("█", filled)) +
		progressTrackStyle.Render(strings.Repeat("░", width-filled))
}

// renderAgentChip renders a single agent with its status dot.
func renderAgentChip(agent, taskStatus string) string {
	dot := "○"
	dotStyle := lipgloss.NewStyle().Foreground(th.Muted)
	if taskStatus == "running" {
		dot = "●"
		dotStyle = lipgloss.NewStyle().Foreground(th.Accent)
	} else if taskStatus == "completed" {
		dot = "✓"
		dotStyle = lipgloss.NewStyle().Foreground(th.Success)
	}
	return dotStyle.Render(dot) + " " + lipgloss.NewStyle().Foreground(th.Text).Render(agent)
}

func renderTaskRow(t TaskInfo, width int) string {
	marker := "◌"
	style := taskPendingStyle

	switch t.Status {
	case "running":
		marker = "●"
		style = taskRunningStyle
	case "completed":
		marker = "✓"
		style = taskDoneStyle
	case "failed":
		marker = "✗"
		style = taskFailedStyle
	case "skipped":
		marker = "○"
		style = taskPendingStyle
	}

	statusMark := style.Render(marker)

	agent := ""
	if t.Agent != "" {
		agent = taskAgentStyle.Render("[" + t.Agent + "]")
	}

	label := style.Render(truncateStr(t.Name, width-8-lipgloss.Width(agent)))
	return fmt.Sprintf(" %s %s %s", statusMark, label, agent)
}

func truncateStr(s string, maxLen int) string {
	if maxLen < 3 {
		maxLen = 3
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
