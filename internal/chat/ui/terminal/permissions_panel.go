package terminal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Permission Center (P19) ──────────────────────────────────────────────────
//
// The Permissions panel shows the effective allow/ask/deny/approval policy for
// the tools and resources the agent can touch — the "computer operator" model
// that turns the agent from an omnipotent bash into a capability-scoped
// operator.
//
// DATA SOURCE: the terminal does not (yet) have direct access to the runtime's
// permission.Ruleset / policy.Engine. The panel is derived from the session's
// actual tool calls (operations ledger) classified by toolRiskLevel:
//
//	read        → ALLOW
//	write       → ALLOW (project-scoped)
//	exec        → ASK
//	destructive → DENY / APPROVAL
//
// TODO(runtime): when the runtime exposes an inspection API for the effective
// permission.Ruleset (permission.Evaluate) and policy.Engine verdicts, replace
// buildPermissionRules with a real evaluation and hydrate the same
// PermissionRule struct. The struct and rendering are prepared for that swap.

// Verdict is the effective permission outcome for a rule.
type Verdict string

const (
	VerdictAllow    Verdict = "ALLOW"
	VerdictAsk      Verdict = "ASK"
	VerdictDeny     Verdict = "DENY"
	VerdictApproval Verdict = "APPROVAL"
)

// PermissionRule is a single (resource, verdict) permission assertion.
type PermissionRule struct {
	Resource string // e.g. "filesystem project/*", "shell go test"
	Verdict  Verdict
	Detail   string
}

// verdictStyle returns the colored style for a verdict.
func verdictStyle(v Verdict) lipgloss.Style {
	switch v {
	case VerdictAllow:
		return lipgloss.NewStyle().Foreground(th.Success).Bold(true)
	case VerdictAsk:
		return lipgloss.NewStyle().Foreground(th.Warning).Bold(true)
	case VerdictApproval:
		return lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	default: // DENY
		return lipgloss.NewStyle().Foreground(th.Error).Bold(true)
	}
}

// buildPermissionRules derives the effective permission rules from the
// session's tool calls. It is the terminal-side derivation; see the TODO above
// for the runtime swap point.
func buildPermissionRules(m *Model) []PermissionRule {
	var rules []PermissionRule
	seen := map[string]bool{}

	addRule := func(resource string, v Verdict, detail string) {
		key := resource + "|" + string(v)
		if seen[key] {
			return
		}
		seen[key] = true
		rules = append(rules, PermissionRule{Resource: resource, Verdict: v, Detail: detail})
	}

	// Derive from the operations ledger: each tool call maps to a resource +
	// verdict based on its risk class.
	for _, ev := range m.operations.entries {
		if ev.Action != ActionToolResult {
			continue
		}
		risk := toolRiskLevel(ev.Details)
		resource := toolResourceName(ev.Details)
		switch risk {
		case "read":
			addRule("filesystem "+resource, VerdictAllow, "read tool: "+truncateStr(ev.Details, 40))
		case "write":
			addRule("filesystem "+resource, VerdictAllow, "write tool: "+truncateStr(ev.Details, 40))
		case "exec":
			addRule("shell "+resource, VerdictAsk, "exec tool: "+truncateStr(ev.Details, 40))
		case "destructive":
			addRule("shell "+resource, VerdictApproval, "destructive tool: "+truncateStr(ev.Details, 40))
		}
	}

	// Seed the canonical rule set so the panel is informative even before any
	// tool has run.
	addRule("filesystem project/*", VerdictAllow, "project-scoped file access")
	addRule("shell go test", VerdictAllow, "safe build/test command")
	addRule("shell rm", VerdictAsk, "destructive shell command")
	addRule("deploy staging", VerdictAllow, "staging deployment")
	addRule("deploy production", VerdictApproval, "production requires approval")
	addRule("secrets read", VerdictDeny, "secrets are never readable")

	return rules
}

// toolResourceName extracts a short resource name from a tool detail string.
func toolResourceName(detail string) string {
	fields := strings.Fields(detail)
	if len(fields) == 0 {
		return "*"
	}
	// Use the first token (e.g. the file path or command) truncated.
	return truncateStr(fields[0], 24)
}

// PermissionsPanelView renders the permission center in the right side panel.
func PermissionsPanelView(rules []PermissionRule, selected int, width, height int) string {
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
	b.WriteString(sidePanelBadge("PERMISSIONS", "[session]", false))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGray).
		Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n")

	if len(rules) == 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(colorGrayLight).
			Italic(true).
			Render("No rules yet — run a task."))
		return borderStyle.Height(contentHeight).Render(b.String())
	}

	displayCount := contentHeight - 3
	if displayCount < 1 {
		displayCount = 1
	}
	for i, r := range rules {
		if i >= displayCount {
			b.WriteString(lipgloss.NewStyle().
				Foreground(colorGrayLight).
				Italic(true).
				Render(fmt.Sprintf("  ... %d more", len(rules)-displayCount)))
			break
		}
		row := renderPermissionRow(r, contentWidth)
		if i == selected {
			row = lipgloss.NewStyle().
				Background(th.Selection).
				Foreground(colorFg).
				Render(row)
		}
		b.WriteString(row)
		b.WriteString("\n")
	}

	return borderStyle.Height(contentHeight).Render(b.String())
}

// renderPermissionRow renders a single permission rule with its verdict badge.
func renderPermissionRow(r PermissionRule, width int) string {
	badge := verdictStyle(r.Verdict).Render(" " + string(r.Verdict) + " ")
	resource := lipgloss.NewStyle().Foreground(th.Text).Render(truncateStr(r.Resource, width-16))
	return fmt.Sprintf(" %s %s", badge, resource)
}

// ─── Computer Mode (P20) ──────────────────────────────────────────────────────
//
// Computer capabilities with permission state, derived from the session's tool
// usage. Terminal/filesystem are ✓ (used, allowed); browser/network are
// ⚠ approval (not used or restricted); secrets are ✕ (denied).

// ComputerCapability is a single computer capability with its state.
type ComputerCapability struct {
	Name   string
	State  string // "ok" | "approval" | "denied" | "unused"
	Detail string
}

// buildComputerCapabilities derives the computer capability states from the
// session's tool usage.
func buildComputerCapabilities(m *Model) []ComputerCapability {
	used := map[string]bool{}
	for _, ev := range m.operations.entries {
		lower := strings.ToLower(ev.Action + " " + ev.Details)
		switch {
		case strings.Contains(lower, "bash"), strings.Contains(lower, "shell"), strings.Contains(lower, "exec"):
			used["terminal"] = true
		case strings.Contains(lower, "write"), strings.Contains(lower, "edit"), strings.Contains(lower, "read"):
			used["filesystem"] = true
		case strings.Contains(lower, "web"), strings.Contains(lower, "fetch"), strings.Contains(lower, "http"):
			used["network"] = true
		case strings.Contains(lower, "browser"), strings.Contains(lower, "playwright"):
			used["browser"] = true
		case strings.Contains(lower, "clipboard"):
			used["clipboard"] = true
		case strings.Contains(lower, "process"), strings.Contains(lower, "ps "):
			used["processes"] = true
		case strings.Contains(lower, "secret"), strings.Contains(lower, "credential"):
			used["secrets"] = true
		}
	}

	caps := []ComputerCapability{
		{Name: "terminal", State: "ok", Detail: "shell commands"},
		{Name: "filesystem", State: "ok", Detail: "project-scoped read/write"},
		{Name: "browser", State: "approval", Detail: "requires approval"},
		{Name: "clipboard", State: "ok", Detail: "clipboard access"},
		{Name: "processes", State: "ok", Detail: "process inspection"},
		{Name: "network", State: "approval", Detail: "requires approval"},
		{Name: "secrets", State: "denied", Detail: "never accessible"},
	}

	// Reflect actual usage: a capability that was used and allowed is "ok".
	for i := range caps {
		if used[caps[i].Name] {
			caps[i].State = "ok"
		}
	}
	return caps
}

// ComputerModeView renders the computer capabilities section (used within the
// Permissions panel or standalone).
func ComputerModeView(m *Model, width int) string {
	caps := buildComputerCapabilities(m)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorGold).
		Bold(true).
		Render(" COMPUTER "))
	b.WriteString("\n")

	for _, c := range caps {
		var glyph string
		var style lipgloss.Style
		switch c.State {
		case "ok":
			glyph = "✓"
			style = lipgloss.NewStyle().Foreground(th.Success)
		case "approval":
			glyph = "⚠"
			style = lipgloss.NewStyle().Foreground(th.Warning)
		case "denied":
			glyph = "✕"
			style = lipgloss.NewStyle().Foreground(th.Error)
		default:
			glyph = "○"
			style = lipgloss.NewStyle().Foreground(th.Muted)
		}
		name := lipgloss.NewStyle().Foreground(th.Text).Render(c.Name)
		detail := lipgloss.NewStyle().Foreground(colorTextMuted).Render(c.Detail)
		b.WriteString(fmt.Sprintf("  %s %-12s %s", style.Render(glyph), name, detail))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}
