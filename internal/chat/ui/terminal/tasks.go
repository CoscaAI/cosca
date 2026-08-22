package terminal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/CoscaAI/cosca/internal/pipeline"
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

// TasksFromPlan converts pipeline task nodes to display tasks.
func TasksFromPlan(plan *pipeline.Plan) []TaskInfo {
	if plan == nil {
		return nil
	}
	tasks := make([]TaskInfo, 0, len(plan.Tasks))
	for _, t := range plan.Tasks {
		ti := TaskInfo{
			ID:     t.ID,
			Name:   t.Description,
			Agent:  t.Agent,
			Status: string(t.Status),
		}
		if t.Result != nil {
			if !t.Result.Success {
				ti.Error = t.Result.Error
				ti.Status = "failed"
			} else {
				ti.Status = "completed"
			}
			ti.DurationMs = t.Result.DurationMs
		}
		tasks = append(tasks, ti)
	}
	return tasks
}

// TaskPanelView renders the task panel for the right side panel.
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

	for i, t := range tasks {
		if i >= displayCount {
			b.WriteString(infoBubble.Render(fmt.Sprintf("  ... %d more", len(tasks)-displayCount)))
			break
		}
		b.WriteString(renderTaskRow(t, contentWidth))
		b.WriteString("\n")
	}

	return panel.Render(b.String())
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
