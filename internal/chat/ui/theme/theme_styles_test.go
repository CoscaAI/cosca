package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Force TrueColor so style assertions work outside a TTY (CI/headless).
func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

func TestThemeStylesRender(t *testing.T) {
	styles := []struct {
		name string
		got  string
	}{
		{"Panel", Petrol.Panel().Render("x")},
		{"Header", Petrol.Header().Render("x")},
		{"HeaderSub", Petrol.HeaderSub().Render("x")},
		{"TabBar", Petrol.TabBar().Render("x")},
		{"TabActive", Petrol.TabActive().Render("x")},
		{"TabInactive", Petrol.TabInactive().Render("x")},
		{"StatusBar", Petrol.StatusBar().Render("x")},
		{"Selected", Petrol.Selected().Render("x")},
		{"App", Petrol.App().Render("x")},
		{"InputBox", Petrol.InputBox().Render("x")},
		{"InputFocused", Petrol.InputFocused().Render("x")},
		{"Breadcrumb", Petrol.Breadcrumb().Render("x")},
		{"BreadcrumbActive", Petrol.BreadcrumbActive().Render("x")},
		{"DiffAddedLine", Petrol.DiffAddedLine().Render("x")},
		{"DiffRemovedLine", Petrol.DiffRemovedLine().Render("x")},
		{"DiffContextLine", Petrol.DiffContextLine().Render("x")},
		{"DiffHunkLine", Petrol.DiffHunkLine().Render("x")},
	}
	for _, s := range styles {
		if strings.TrimSpace(s.got) == "" {
			t.Fatalf("%s rendered empty", s.name)
		}
	}
}

func TestRiskLevel(t *testing.T) {
	write := Petrol.RiskLevel("write").Render("x")
	exec := Petrol.RiskLevel("exec").Render("x")
	destructive := Petrol.RiskLevel("destructive").Render("x")
	defaultS := Petrol.RiskLevel("unknown").Render("x")

	if write == "" || exec == "" || destructive == "" || defaultS == "" {
		t.Fatal("risk styles must render")
	}
	// Destructive must be bold (ANSI bold sequence \x1b[1).
	if !strings.Contains(destructive, "\x1b[1") {
		t.Fatalf("destructive not bold: %q", destructive)
	}
}

func TestAppBackgroundFallback(t *testing.T) {
	// Theme with empty Background falls back to BackgroundPanel — the render
	// carries a background colour (48;2 = RGB background) for the panel hex.
	t1 := Theme{BackgroundPanel: "#111111", Surface: "#000000"}
	if got := t1.App().Render("x"); !strings.Contains(got, "48;2") {
		t.Fatalf("fallback to panel must render a background: %q", got)
	}
	// Empty theme still renders (Surface fallback path, no panic).
	t2 := Theme{}
	if got := t2.App().Render("x"); strings.TrimSpace(got) == "" {
		t.Fatal("empty theme app rendered empty")
	}
}

func TestHeaderUsesPrimaryAndSurface(t *testing.T) {
	rendered := Petrol.Header().Render("x")
	// Header = bold text on a coloured background (48;2 = RGB background).
	if !strings.Contains(rendered, "\x1b[1") || !strings.Contains(rendered, "48;2") {
		t.Fatalf("header must be bold on coloured background: %q", rendered)
	}
}

func TestInputFocusedDiffersFromInput(t *testing.T) {
	// Focused and unfocused input render differently (different border colour).
	if Petrol.InputFocused().Render("x") == Petrol.InputBox().Render("x") {
		t.Fatal("focused and unfocused inputs must render differently")
	}
}

func TestSelectedUsesSelection(t *testing.T) {
	rendered := Petrol.Selected().Render("x")
	// Selected = text on a coloured background.
	if !strings.Contains(rendered, "48;2") {
		t.Fatalf("selected must render on a coloured background: %q", rendered)
	}
}

func TestMarkdownRenderer(t *testing.T) {
	r, err := Petrol.MarkdownRenderer(80)
	if err != nil {
		t.Fatalf("MarkdownRenderer: %v", err)
	}
	if r == nil {
		t.Fatal("nil renderer")
	}
	out, err := r.Render("# Hello\n\nSome **bold** text")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "Hello") {
		t.Fatalf("render output: %q", out)
	}
}

func TestMarkdownStyleHasDocument(t *testing.T) {
	cfg := Petrol.MarkdownStyle()
	_ = cfg.Document
}
