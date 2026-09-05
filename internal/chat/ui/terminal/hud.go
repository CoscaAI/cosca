package terminal

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
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
	Branch     string
}

// busyFrames and streamingFrames are smooth braille cycles used by the status
// bar while work is running (kept in sync with the frame tick).
var (
	busyFrames      = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
	streamingFrames = []rune("⠁⠁⠉⠙⠚⠒⠂⠂⠒⠲⠴⠤⠄⠄⠤⠠⠠⠤⠦⠖⠒⠐⠐⠒⠓⠋⠉⠈⠈")
)

// gitBranch returns the current git branch name by reading .git/HEAD. It never
// shells out to git: in detached HEAD it falls back to the short commit sha.
func gitBranch(projectPath string) string {
	if projectPath == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(projectPath, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	ref := strings.TrimSpace(string(data))
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "ref: refs/heads/") {
		return strings.TrimPrefix(ref, "ref: refs/heads/")
	}
	if len(ref) > 7 {
		return ref[:7]
	}
	return ref
}

func frameRune(frames []rune, frame int) rune {
	if len(frames) == 0 {
		return '⠿'
	}
	idx := frame % len(frames)
	if idx < 0 {
		idx = -idx
	}
	return frames[idx]
}

// HudView renders the bottom status bar with phase progress,
// active task name, git branch and per-minute cost burn rate.
// frame drives the busy/streaming spinner glyphs.
func HudView(info HUDInfo, width int, frame int) string {
	if width < 20 {
		return ""
	}

	statusColor := colorGreen
	statusGlyph := "●"
	if info.Status == "busy" || info.Status == "planning" {
		statusColor = colorGold
		statusGlyph = string(frameRune(busyFrames, frame))
	} else if info.Status == "streaming" {
		statusColor = colorCyan
		statusGlyph = string(frameRune(streamingFrames, frame))
	} else if info.Status == "error" {
		statusColor = colorRed
		statusGlyph = "✗"
	}

	statusTag := lipgloss.NewStyle().
		Foreground(colorBg).
		Background(statusColor).
		Bold(true).
		Padding(0, 1).
		Render(" " + statusGlyph + " " + info.Status + " ")

	modelTag := hudModelStyle.Render(info.Model)
	agentTag := ""
	if info.Agent != "" {
		agentTag = hudAgentStyle.Render(" " + info.Agent + " ")
	}

	branchTag := ""
	if info.Branch != "" {
		branchTag = hudLabelStyle.Render(" ⎇") + hudValueStyle.Render(" "+info.Branch+" ")
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
	if branchTag != "" {
		rightParts = append(rightParts, branchTag)
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
