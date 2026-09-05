package terminal

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestAnimatedProgressWidth(t *testing.T) {
	for _, pct := range []float64{0, 0.1, 0.33, 0.5, 0.99, 1.0} {
		for _, w := range []int{10, 20, 40} {
			got := AnimatedProgress(pct, w, 0)
			if lipgloss.Width(got) != w {
				t.Fatalf("AnimatedProgress(%.2f, %d) width = %d, want %d (%q)", pct, w, lipgloss.Width(got), w, got)
			}
		}
	}
}

func TestAnimatedProgressClamps(t *testing.T) {
	if got := lipgloss.Width(AnimatedProgress(2.0, 20, 0)); got != 20 {
		t.Fatalf("AnimatedProgress(2.0) width = %d, want 20", got)
	}
	if got := lipgloss.Width(AnimatedProgress(-1.0, 20, 0)); got != 20 {
		t.Fatalf("AnimatedProgress(-1.0) width = %d, want 20", got)
	}
}

func TestAnimatedProgressAnimates(t *testing.T) {
	a := AnimatedProgress(0.5, 20, 0)
	b := AnimatedProgress(0.5, 20, 1)
	if a == b {
		t.Fatalf("progress bar should differ across frames")
	}
}

func TestStatusDotToggle(t *testing.T) {
	r0 := StatusDot("running", 0)
	r1 := StatusDot("running", 1)
	if r0 == r1 {
		t.Fatalf("running dot should animate across frames: %q == %q", r0, r1)
	}

	if got := StatusDot("done", 5); got != "✓" {
		t.Fatalf("done dot = %q, want ✓", got)
	}
	if got := StatusDot("failed", 5); got != "✗" {
		t.Fatalf("failed dot = %q, want ✗", got)
	}
	if got := StatusDot("pending", 5); got != "○" {
		t.Fatalf("pending dot = %q, want ○", got)
	}
	if got := StatusDot("idle", 5); got != "●" {
		t.Fatalf("idle dot = %q, want ●", got)
	}
}

func TestSpinnerByOp(t *testing.T) {
	for _, op := range []string{"agent", "tool", "build", "test", "index"} {
		if _, ok := SpinnerByOp[op]; !ok {
			t.Fatalf("SpinnerByOp missing op %q", op)
		}
	}
}
