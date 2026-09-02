package terminal

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
)

// SpinnerByOp maps an operation class to a matching spinner animation.
var SpinnerByOp = map[string]spinner.Spinner{
	"agent": spinner.Line,
	"tool":  spinner.Dot,
	"build": spinner.Meter,
	"test":  spinner.Moon,
	"index": spinner.Points,
}

// runningFrames is a smooth braille cycle for running status. It starts at
// the full glyph (⠿) and sweeps through a spinner arc.
var runningFrames = []rune("⠿⠙⠹⠸⠼⠴⠦⠧⠇⠏⠋")

// StatusDot returns the status glyph for a tree status. The running glyph
// animates across frames so a moving dot signals live work.
func StatusDot(status string, frame int) string {
	switch status {
	case "running":
		return string(frameRune(runningFrames, frame))
	case "done":
		return "✓"
	case "failed":
		return "✗"
	case "pending":
		return "○"
	default:
		return "●"
	}
}

// AnimatedProgress renders a progress bar with an animated leading edge.
// The output always has exactly width columns (when width >= 5).
func AnimatedProgress(pct float64, width int, frame int) string {
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

	var b strings.Builder
	b.WriteString(strings.Repeat("█", filled))
	if filled < width {
		heads := []rune("▉▊▋▌▍▎▏")
		idx := frame % len(heads)
		if idx < 0 {
			idx = -idx
		}
		b.WriteRune(heads[idx])
		b.WriteString(strings.Repeat("░", width-filled-1))
	}
	return b.String()
}
