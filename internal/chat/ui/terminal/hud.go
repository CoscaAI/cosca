package terminal

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/CoscaAI/cosca/internal/pipeline"
)

// HUDPhase tracks the current execution phase for the progress bar.
type HUDPhase int

const (
	PhaseIdle HUDPhase = iota
	PhasePlanning
	PhaseExecuting
	PhaseBuilding
	PhaseTesting
	PhaseDone
)

var phaseNames = map[HUDPhase]string{
	PhaseIdle:      "Idle",
	PhasePlanning:  "Plan",
	PhaseExecuting: "Execute",
	PhaseBuilding:  "Build",
	PhaseTesting:   "Test",
	PhaseDone:      "Done",
}

// HUDInfo contains the data shown in the status bar.
type HUDInfo struct {
	Model      string
	Agent      string
	Status     string
	Progress   float64
	Tokens     pipeline.TokenUsage
	Cost       float64
	Uptime     string
	Phase      HUDPhase
	PhasePct   float64
	ActiveTask string
	Elapsed    time.Duration
}

// HudView renders the bottom status bar with phase progress,
// active task name, and per-minute cost burn rate.
func HudView(info HUDInfo, width int) string {
	if width < 20 {
		return ""
	}

	statusColor := colorGreen
	if info.Status == "busy" || info.Status == "planning" {
		statusColor = colorGold
	} else if info.Status == "streaming" {
		statusColor = colorCyan
	} else if info.Status == "error" {
		statusColor = colorRed
	}

	statusTag := lipgloss.NewStyle().
		Foreground(colorBg).
		Background(statusColor).
		Bold(true).
		Padding(0, 1).
		Render(" " + info.Status + " ")

	modelTag := hudModelStyle.Render(info.Model)
	agentTag := ""
	if info.Agent != "" {
		agentTag = hudAgentStyle.Render(" " + info.Agent + " ")
	}

	tokensTag := ""
	if info.Tokens.Input > 0 || info.Tokens.Output > 0 {
		tokensTag = hudLabelStyle.Render(fmt.Sprintf("tokens:%d/%d",
			info.Tokens.Input, info.Tokens.Output))
	}

	costTag := ""
	if info.Cost > 0 {
		costTag = hudLabelStyle.Render(fmt.Sprintf("$%.4f", info.Cost))
	}

	burnTag := ""
	if info.Cost > 0 && info.Elapsed > 0 {
		mins := info.Elapsed.Minutes()
		if mins > 0 {
			rate := info.Cost / mins
			burnTag = hudLabelStyle.Render(fmt.Sprintf("%.2f/min", rate))
		}
	}

	uptimeTag := hudLabelStyle.Render(info.Uptime)

	phaseBarStr := renderPhaseBar(info.Phase, info.PhasePct, 24)

	progressBarStr := renderProgressBar(info.Progress, 20)

	activeTaskTag := ""
	if info.ActiveTask != "" {
		activeTaskTag = hudAgentStyle.Render(" " + truncateStr(info.ActiveTask, 30) + " ")
	}
	hudBackground := lipgloss.NewStyle().Background(th.BgAlt)

	left := lipgloss.JoinHorizontal(lipgloss.Center,
		statusTag,
		hudBackground.Render(" ")+modelTag,
		agentTag,
		activeTaskTag,
	)

	rightParts := []string{}
	if phaseBarStr != "" {
		rightParts = append(rightParts, phaseBarStr)
	}
	rightParts = append(rightParts, progressBarStr)
	if tokensTag != "" {
		rightParts = append(rightParts, tokensTag)
	}
	if burnTag != "" {
		rightParts = append(rightParts, burnTag)
	}
	if costTag != "" {
		rightParts = append(rightParts, costTag)
	}
	rightParts = append(rightParts, uptimeTag)
	right := lipgloss.JoinHorizontal(lipgloss.Center, rightParts...)

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)

	midSpace := width - leftW - rightW - 4
	if midSpace < 0 {
		midSpace = 0
	}

	mid := hudBackground.Render(strings.Repeat(" ", midSpace))

	full := lipgloss.JoinHorizontal(lipgloss.Center, left, mid, right)

	return hudStyle.Width(width).Render(full)
}

func renderProgressBar(percent float64, barWidth int) string {
	if barWidth < 5 {
		barWidth = 10
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	filled := int(math.Round(percent * float64(barWidth)))
	return progressFillStyle.Render(strings.Repeat("█", filled)) +
		progressTrackStyle.Render(strings.Repeat("░", barWidth-filled))
}

func renderPhaseBar(phase HUDPhase, pct float64, barWidth int) string {
	if phase == PhaseIdle || phase == PhaseDone {
		return ""
	}
	if barWidth < 10 {
		barWidth = 16
	}

	phaseName := phaseNames[phase]
	if phaseName == "" {
		phaseName = "…"
	}

	label := hudPhaseLabel.Render(phaseName + " ")

	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	filled := int(math.Round(pct * float64(barWidth)))
	return label + hudPhaseFill.Render(strings.Repeat("█", filled)) +
		hudPhaseTrack.Render(strings.Repeat("░", barWidth-filled))
}
