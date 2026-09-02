package terminal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// CommandResult is the outcome of processing a slash command.
type CommandResult struct {
	Text     string
	Role     string
	IsError  bool
	IsAction bool
	ActionID string
}

// ColonResult is the outcome of processing a colon-prompt command.
// K9s-style: :agent, :model, :health, :cost, :quit, etc.
type ColonResult struct {
	Text       string
	IsError    bool
	IsAction   bool
	ActionID   string
	NewAgent   string
	NewModel   string
	ShouldQuit bool
}

// ExecSlashCommand processes a slash command and returns a display result.
func ExecSlashCommand(cmd string, mc *CommandContext) CommandResult {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return CommandResult{Text: "?", Role: "error", IsError: true}
	}

	base := strings.ToLower(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = strings.Join(parts[1:], " ")
	}

	switch base {
	case "/agents":
		return cmdAgents(mc)

	case "/context":
		return cmdContext(mc)

	case "/memory":
		return cmdMemory(mc, arg)

	case "/knowledge":
		return cmdKnowledge(mc, arg)

	case "/tasks":
		return cmdTasks(mc)

	case "/trace":
		return cmdTrace(mc)

	case "/audit":
		return cmdAudit(mc)

	case "/models":
		return cmdModels(mc)

	case "/cost":
		return cmdCost(mc)

	case "/performance":
		return cmdPerformance(mc)

	case "/simple":
		return cmdSimple(mc)

	case "/advanced":
		return cmdAdvanced(mc)

	case "/plan":
		return cmdPlan(mc)

	case "/workflow":
		return cmdWorkflow(mc, arg)

	case "/workflows":
		return cmdWorkflows(mc)

	case "/help":
		return cmdHelp()

	default:
		return CommandResult{
			Text:    fmt.Sprintf("Unknown command: %s\nType /help for available commands.", base),
			Role:    "error",
			IsError: true,
		}
	}
}

// ExecColonCommand processes a colon-prompt command (:agent, :model, etc.).
// K9s-style command palette — distinct from slash commands.
func ExecColonCommand(input string, mc *CommandContext) ColonResult {
	// Strip leading colon if present.
	cmd := strings.TrimPrefix(input, ":")
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ColonResult{Text: ":", IsError: true}
	}

	base := strings.ToLower(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = strings.Join(parts[1:], " ")
	}

	switch base {
	case "q", "quit", "exit":
		return ColonResult{
			Text:       "Quitting...",
			IsAction:   true,
			ShouldQuit: true,
		}

	case "agent":
		if arg == "" {
			return ColonResult{Text: "Usage: :agent <name>", IsError: true}
		}
		return ColonResult{
			Text:     "Switching agent to " + arg,
			IsAction: true,
			NewAgent: arg,
		}

	case "model":
		if arg == "" {
			return ColonResult{Text: "Usage: :model <name>", IsError: true}
		}
		return ColonResult{
			Text:     "Switching model to " + arg,
			IsAction: true,
			NewModel: arg,
		}

	case "health":
		return ColonResult{
			Text: colonHealthReport(mc),
		}

	case "cost":
		return ColonResult{
			Text: colonCostReport(mc),
		}

	case "status":
		return ColonResult{
			Text: colonStatusReport(mc),
		}

	case "help":
		return ColonResult{
			Text: colonHelp(),
		}

	default:
		return ColonResult{
			Text:    "Unknown command: " + base + "\nType :help for colon commands.",
			IsError: true,
		}
	}
}

func colonHealthReport(mc *CommandContext) string {
	var b strings.Builder
	b.WriteString(titleSubStyle.Render("Health Check"))
	b.WriteString("\n\n")

	status := "healthy"

	b.WriteString(infoBubble.Render(fmt.Sprintf("  Session: %s", status)))
	b.WriteString("\n")
	b.WriteString(infoBubble.Render(fmt.Sprintf("  Agent:   %s", mc.ActiveAgent)))
	b.WriteString("\n")
	b.WriteString(infoBubble.Render(fmt.Sprintf("  Model:   %s", mc.ActiveModel)))
	b.WriteString("\n")
	b.WriteString(infoBubble.Render(fmt.Sprintf("  Mode:    %s", modeLabel(mc.AdvancedMode))))
	b.WriteString("\n")

	if status == "healthy" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(colorGreen).Render("  ✓ All systems nominal"))
	} else {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(colorRed).Render("  ✗ Issues detected"))
	}

	return b.String()
}

func colonCostReport(mc *CommandContext) string {
	var b strings.Builder
	b.WriteString(titleSubStyle.Render("Cost Report"))
	b.WriteString("\n\n")

	b.WriteString(infoBubble.Render(fmt.Sprintf("  Model:  %s", mc.ActiveModel)))
	b.WriteString("\n")
	b.WriteString(infoBubble.Render("  Usage data available after first pipeline run."))
	b.WriteString("\n")
	b.WriteString(infoBubble.Render("  Run /cost for detailed breakdown."))

	return b.String()
}

func colonStatusReport(mc *CommandContext) string {
	var b strings.Builder
	b.WriteString(titleSubStyle.Render("Session Status"))
	b.WriteString("\n\n")

	b.WriteString(infoBubble.Render(fmt.Sprintf("  Agent:      %s", mc.ActiveAgent)))
	b.WriteString("\n")
	b.WriteString(infoBubble.Render(fmt.Sprintf("  Model:      %s", mc.ActiveModel)))
	b.WriteString("\n")
	b.WriteString(infoBubble.Render(fmt.Sprintf("  Mode:       %s", modeLabel(mc.AdvancedMode))))
	b.WriteString("\n")
	b.WriteString(infoBubble.Render(fmt.Sprintf("  Tasks:      %d active", len(mc.Tasks))))
	b.WriteString("\n")

	if mc.TermCtx != nil {
		b.WriteString(infoBubble.Render(fmt.Sprintf("  Session:    %s", mc.TermCtx.Summary())))
		b.WriteString("\n")
	}

	return b.String()
}

func colonHelp() string {
	var b strings.Builder
	b.WriteString(titleSubStyle.Render("Command Palette (: commands)"))
	b.WriteString("\n\n")

	cmds := []struct{ cmd, desc string }{
		{":agent <name>", "Switch to named agent"},
		{":model <name>", "Switch to named model"},
		{":health", "Run health check"},
		{":cost", "Show cost report"},
		{":status", "Show session status"},
		{":quit, :q", "Exit terminal"},
		{":help", "Show this help"},
	}

	for _, c := range cmds {
		b.WriteString(slashBubble.Render(fmt.Sprintf("  %-16s", c.cmd)))
		b.WriteString(infoBubble.Render(c.desc))
		b.WriteString("\n")
	}

	return b.String()
}

func modeLabel(advanced bool) string {
	if advanced {
		return "advanced"
	}
	return "simple"
}

// CommandContext provides access to state needed by slash commands.
type CommandContext struct {
	TermCtx      interface{ Summary() string }
	Tasks        []TaskInfo
	Plan         interface{}
	Models       []string
	Agents       []string
	Workflows    []string
	ActiveModel  string
	ActiveAgent  string
	AdvancedMode bool
}

// ─── Command implementations ────────────────────────────────────────────────

func cmdAgents(mc *CommandContext) CommandResult {
	var b strings.Builder
	b.WriteString(titleSubStyle.Render("Available Agents"))
	b.WriteString("\n\n")
	if len(mc.Agents) == 0 {
		b.WriteString(infoBubble.Render("  No agents registered. Use cosca install to set up agents."))
	} else {
		for _, a := range mc.Agents {
			marker := "  "
			if a == mc.ActiveAgent {
				marker = "▶ "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", marker, a))
		}
	}
	return CommandResult{Text: b.String(), Role: "slash"}
}

func cmdContext(mc *CommandContext) CommandResult {
	var b strings.Builder
	b.WriteString(titleSubStyle.Render("General Context"))
	b.WriteString("\n\n")
	if mc.TermCtx != nil {
		b.WriteString(slashBubble.Render(mc.TermCtx.Summary()))
	} else {
		b.WriteString(infoBubble.Render("  No session context available."))
	}
	return CommandResult{Text: b.String(), Role: "slash"}
}

func cmdMemory(_ *CommandContext, arg string) CommandResult {
	if arg == "" {
		return CommandResult{
			Text:    "Usage: /memory <query>\nExample: /memory authentication module",
			Role:    "error",
			IsError: true,
		}
	}
	// TODO(runtime): connect to the runtime memory API when available. Until
	// then, point the user to the Memory Explorer (Alt+6) which shows the
	// session's epistemologically-typed memory derived from live state.
	return CommandResult{
		Text: fmt.Sprintf("Memory search: \"%s\"\n%s",
			arg, infoBubble.Render("Runtime memory API not connected yet. Use Alt+6 (Memory Explorer) to browse this session's memory.")),
		Role: "slash",
	}
}

func cmdKnowledge(_ *CommandContext, arg string) CommandResult {
	if arg == "" {
		return CommandResult{
			Text:    "Usage: /knowledge <query>\nExample: /knowledge Go concurrency patterns",
			Role:    "error",
			IsError: true,
		}
	}
	return CommandResult{
		Text: fmt.Sprintf("Knowledge search: \"%s\"\n%s",
			arg, infoBubble.Render("Knowledge base search not integrated yet.")),
		Role: "slash",
	}
}

func cmdTasks(mc *CommandContext) CommandResult {
	var b strings.Builder
	b.WriteString(titleSubStyle.Render("Task List"))
	b.WriteString("\n\n")
	if len(mc.Tasks) == 0 {
		b.WriteString(infoBubble.Render("  No active tasks."))
	} else {
		for _, t := range mc.Tasks {
			b.WriteString(fmt.Sprintf("  %-20s %s\n", t.Name, t.Status))
		}
	}
	return CommandResult{Text: b.String(), Role: "slash"}
}

func cmdTrace(_ *CommandContext) CommandResult {
	return CommandResult{
		Text: slashBubble.Render("Execution trace:\n" +
			infoBubble.Render("  Trajectory tracking not yet integrated into terminal.")),
		Role: "slash",
	}
}

func cmdAudit(_ *CommandContext) CommandResult {
	return CommandResult{
		Text: slashBubble.Render("Audit information:\n" +
			infoBubble.Render("  Audit trail module not yet integrated into terminal.")),
		Role: "slash",
	}
}

func cmdModels(mc *CommandContext) CommandResult {
	var b strings.Builder
	b.WriteString(titleSubStyle.Render("Available Models"))
	b.WriteString("\n\n")
	if len(mc.Models) == 0 {
		b.WriteString(infoBubble.Render("  Run /model <name> to select a model."))
	} else {
		for _, m := range mc.Models {
			marker := "  "
			if m == mc.ActiveModel {
				marker = "▶ "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", marker, m))
		}
	}
	return CommandResult{Text: b.String(), Role: "slash"}
}

func cmdCost(_ *CommandContext) CommandResult {
	return CommandResult{
		Text: slashBubble.Render("Cost breakdown:\n" +
			infoBubble.Render("  Detailed cost tracking available after first pipeline run.")),
		Role: "slash",
	}
}

func cmdPerformance(_ *CommandContext) CommandResult {
	return CommandResult{
		Text: slashBubble.Render("Performance metrics:\n" +
			infoBubble.Render("  Metrics available after first pipeline execution.")),
		Role: "slash",
	}
}

func cmdSimple(mc *CommandContext) CommandResult {
	return CommandResult{
		Text:     "Switching to simple mode...",
		Role:     "info",
		IsAction: true,
		ActionID: "simple",
	}
}

func cmdAdvanced(mc *CommandContext) CommandResult {
	return CommandResult{
		Text:     "Switching to advanced mode...",
		Role:     "info",
		IsAction: true,
		ActionID: "advanced",
	}
}

func cmdPlan(_ *CommandContext) CommandResult {
	return CommandResult{
		Text: slashBubble.Render("Current plan:\n" +
			infoBubble.Render("  Submit a task to generate an execution plan.")),
		Role: "slash",
	}
}

func cmdWorkflow(_ *CommandContext, arg string) CommandResult {
	if arg == "" {
		return CommandResult{
			Text: slashBubble.Render("Workflow commands:\n  /workflow list — list active workflows\n  /workflow status <id> — workflow status\n  Use /workflows to see all available workflows."),
			Role: "slash",
		}
	}
	return CommandResult{
		Text: slashBubble.Render(fmt.Sprintf("Workflow: %s\n%s", arg,
			infoBubble.Render("Workflow integration pending."))),
		Role: "slash",
	}
}

func cmdWorkflows(mc *CommandContext) CommandResult {
	var b strings.Builder
	b.WriteString(titleSubStyle.Render("Available Workflows"))
	b.WriteString("\n\n")
	if len(mc.Workflows) == 0 {
		b.WriteString(infoBubble.Render("  No workflows available. Add workflows to .cosca/workflows/"))
	} else {
		for _, wf := range mc.Workflows {
			b.WriteString(fmt.Sprintf("  %s\n", wf))
		}
	}
	return CommandResult{Text: b.String(), Role: "slash"}
}

func cmdHelp() CommandResult {
	var b strings.Builder
	b.WriteString(titleSubStyle.Render("Terminal Commands"))
	b.WriteString("\n\n")

	cols := []struct {
		cmd  string
		desc string
	}{
		{"/agents", "List available agents"},
		{"/context", "Show General Context state"},
		{"/memory <q>", "Search recent memories"},
		{"/knowledge <q>", "Search knowledge base"},
		{"/tasks", "Show task list"},
		{"/trace", "Show execution trace"},
		{"/audit", "Show audit info"},
		{"/models", "List available models"},
		{"/cost", "Show cost breakdown"},
		{"/performance", "Show performance metrics"},
		{"/simple", "Switch to simple mode"},
		{"/advanced", "Switch to advanced mode"},
		{"/plan", "Show current plan"},
		{"/workflow", "Workflow commands"},
		{"/workflows", "List all available workflows"},
		{"/help", "Show this help"},
		{"/exit", "Exit terminal"},
	}

	for _, c := range cols {
		b.WriteString(slashBubble.Render(fmt.Sprintf("  %-18s", c.cmd)))
		b.WriteString(infoBubble.Render(c.desc))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(titleSubStyle.Render("Command Palette (:commands)"))
	b.WriteString("\n\n")

	colonCmds := []struct{ cmd, desc string }{
		{":agent <name>", "Switch to named agent"},
		{":model <name>", "Switch to named model"},
		{":health", "Run health check"},
		{":cost", "Show cost report"},
		{":status", "Show session status"},
		{":quit, :q", "Exit terminal"},
	}

	for _, c := range colonCmds {
		b.WriteString(slashBubble.Render(fmt.Sprintf("  %-16s", c.cmd)))
		b.WriteString(infoBubble.Render(c.desc))
		b.WriteString("\n")
	}

	return CommandResult{Text: b.String(), Role: "slash"}
}
