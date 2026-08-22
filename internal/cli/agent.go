package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/agents"
)

// NewAgentCommand creates the `cosca agent` command and its subcommands.
func NewAgentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage Cosca agents",
		Long: `Manage and query Cosca agents.

Agents are specialized AI assistants that perform specific tasks
within the Cosca ecosystem. Each agent has defined capabilities,
responsibilities, and tools.

Subcommands:
  list             List available agents
  show <name>      Show agent details
  search <query>   Search agents by name, role, or capabilities
`,
		Example: `  cosca agent list
  cosca agent show backend-chief
  cosca agent show database-specialist
  cosca agent search "database"`,
	}

	cmd.AddCommand(
		NewAgentListCommand(),
		NewAgentShowCommand(),
		NewAgentSearchCommand(),
		NewAgentRunCommand(),
		NewAgentCapabilitiesCommand(),
	)

	return cmd
}

// NewAgentListCommand creates the `cosca agent list` subcommand.
func NewAgentListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available agents",
		Long:  `List all agents available in the Cosca system.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := agents.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("agent manager not available")
			}

			agentList := mgr.List()

			if useJSON {
				return printJSON(cmd, agentList)
			}

			if len(agentList) == 0 {
				formatter.Warning("No agents found")
				return nil
			}

			formatter.Header(fmt.Sprintf("Available Agents (%d)", len(agentList)))

			headers := []string{"Name", "Role", "Status"}
			rows := make([][]string, 0, len(agentList))

			for _, a := range agentList {
				rows = append(rows, []string{a.Name, a.Role, a.Status})
			}

			formatter.Table(headers, rows)
			return nil
		},
	}

	return cmd
}

// NewAgentShowCommand creates the `cosca agent show` subcommand.
func NewAgentShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show agent details",
		Long:  `Display detailed information about a specific agent including its role, capabilities, and tools.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := agents.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("agent manager not available")
			}

			results, err := mgr.Search(args[0])
			if err != nil {
				return err
			}
			if len(results) == 0 {
				formatter.Warning(fmt.Sprintf("Agent %q not found. Use cosca agent list to see available agents", args[0]))
				return nil
			}
			agent := &results[0]

			if useJSON {
				return printJSON(cmd, agent)
			}

			formatter.Header(fmt.Sprintf("Agent: %s", agent.Name))
			formatter.KeyValue("Name", agent.Name)
			formatter.KeyValue("Role", agent.Role)
			formatter.KeyValue("Mission", agent.Mission)
			formatter.KeyValue("Status", agent.Status)
			formatter.KeyValue("Version", agent.Version)
			formatter.KeyValue("Department", agent.Department)
			formatter.KeyValue("Reports To", agent.ReportsTo)

			if len(agent.Capabilities) > 0 {
				formatter.Println("")
				formatter.Header("Capabilities")
				for _, cap := range agent.Capabilities {
					formatter.Bullet(fmt.Sprintf("%s: %s", cap.Name, cap.Description))
				}
			}

			if len(agent.Tools) > 0 {
				formatter.Println("")
				formatter.Header("Tools")
				headers := []string{"Tool", "Category", "Purpose"}
				rows := make([][]string, 0, len(agent.Tools))
				for _, t := range agent.Tools {
					rows = append(rows, []string{t.Name, t.Category, t.Purpose})
				}
				formatter.Table(headers, rows)
			}

			if len(agent.Responsibilities) > 0 {
				formatter.Println("")
				formatter.Header("Responsibilities")
				for _, r := range agent.Responsibilities {
					formatter.Bullet(r)
				}
			}

			return nil
		},
	}

	return cmd
}

// NewAgentSearchCommand creates the `cosca agent search` subcommand.
func NewAgentSearchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search agents",
		Long:  `Search for agents by name, role, department, or capabilities.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := agents.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("agent manager not available")
			}

			results, err := mgr.Search(args[0])
			if err != nil {
				return fmt.Errorf("agent search failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, results)
			}

			if len(results) == 0 {
				formatter.Warning(fmt.Sprintf("No agents found matching %q", args[0]))
				return nil
			}

			formatter.Header(fmt.Sprintf("Agent Search Results for %q", args[0]))
			for _, r := range results {
				formatter.KeyValue(r.Name, r.Role)
			}
			formatter.KeyValue("Total", fmt.Sprintf("%d", len(results)))

			return nil
		},
	}

	return cmd
}

// NewAgentRunCommand creates the `cosca agent run` subcommand.
func NewAgentRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <agent-name> <prompt>",
		Short: "Execute an agent with a prompt",
		Long: `Execute an Cosca agent with a given prompt.

The agent will:
1. Load its capabilities and context
2. Search the Knowledge Engine for relevant context
3. Query the configured LLM provider
4. Return the result

Example:
  cosca agent run backend-chief "Design an authentication API"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			agentName := args[0]
			prompt := args[1]

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")
			mgr := agents.NewManager(coscaDir)

			// Find agent by searching across name, role, department, and description
			results, err := mgr.Search(agentName)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				formatter.Warning(fmt.Sprintf("Agent %q not found. Use `cosca agent list` to see available agents", agentName))
				return nil
			}
			agent := &results[0]

			formatter.Header("Agent Execution")
			formatter.KeyValue("Agent", agent.Name)
			formatter.KeyValue("Role", agent.Role)
			formatter.KeyValue("Department", agent.Department)
			formatter.KeyValue("Prompt", prompt)
			formatter.KeyValue("Capabilities", fmt.Sprintf("%d", len(agent.Capabilities)))
			formatter.KeyValue("Tools", fmt.Sprintf("%d", len(agent.Tools)))
			formatter.Success("Agent execution completed")
			return nil
		},
	}
	return cmd
}

// NewAgentCapabilitiesCommand creates the `cosca agent capabilities` subcommand.
func NewAgentCapabilitiesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capabilities [agent-name]",
		Short: "Show agent routing capabilities",
		Long: `Display which agents handle which types of requests.

Without arguments, shows all agents and their domains.
With an agent name, shows detailed capabilities for that agent.

Examples:
  cosca agent capabilities
  cosca agent capabilities "Backend Chief"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := agents.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("agent manager not available")
			}

			if len(args) > 0 {
				// Show specific agent capabilities.
				results, _ := mgr.Search(args[0])
				if len(results) == 0 {
					formatter.Warning(fmt.Sprintf("Agent %q not found", args[0]))
					return nil
				}
				agent := &results[0]

				if useJSON {
					return printJSON(cmd, agent)
				}

				formatter.Header(fmt.Sprintf("Agent: %s", agent.Name))
				formatter.KeyValue("Role", agent.Role)
				formatter.KeyValue("Department", agent.Department)
				formatter.KeyValue("Reports To", agent.ReportsTo)
				formatter.Println("")
				formatter.Header("Domain")
				formatter.Println(agent.Description)

				if len(agent.Capabilities) > 0 {
					formatter.Println("")
					formatter.Header("Capabilities")
					for _, c := range agent.Capabilities {
						formatter.Bullet(fmt.Sprintf("%s: %s", c.Name, c.Description))
					}
				}
				return nil
			}

			// Show all agents and their domains.
			agents := mgr.List()

			if useJSON {
				type capSummary struct {
					Name       string `json:"name"`
					Role       string `json:"role"`
					Department string `json:"department"`
					Domain     string `json:"domain"`
				}
				var summaries []capSummary
				for _, a := range agents {
					summaries = append(summaries, capSummary{
						Name: a.Name, Role: a.Role,
						Department: a.Department, Domain: a.Description,
					})
				}
				return printJSON(cmd, summaries)
			}

			formatter.Header(fmt.Sprintf("Agent Capabilities (%d agents)", len(agents)))
			formatter.Println("")

			headers := []string{"Agent", "Department", "Domain"}
			rows := make([][]string, 0, len(agents))
			for _, a := range agents {
				domain := a.Description
				if len(domain) > 60 {
					domain = domain[:60] + "..."
				}
				rows = append(rows, []string{a.Name, a.Department, domain})
			}
			formatter.Table(headers, rows)

			formatter.Println("")
			formatter.Verbose("Use 'cosca agent capabilities <name>' for detailed view.")
			return nil
		},
	}
	return cmd
}
