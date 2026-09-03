package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/executor"
	chatprovider "github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/chat/tool"
	fs "github.com/CoscaAI/cosca/internal/chat/tools/filesystem"
	"github.com/CoscaAI/cosca/internal/config"
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
//
// It actually executes an agent: it loads the agent's persona, selects the
// configured LLM provider (Ollama local by default), passes the cross-platform
// filesystem tools via function calling, runs the agent-to-tool loop (execute
// tool calls, feed results back to the model, repeat up to a safety cap), and
// returns the real result to the user.
func NewAgentRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <agent-name> <prompt>",
		Short: "Execute an agent with a prompt",
		Long: `Execute an Cosca agent with a given prompt.

The agent will:
1. Load its capabilities and context
2. Build a system prompt from its persona and responsibilities
3. Query the configured LLM provider with filesystem tools (function calling)
4. Execute any tool calls the agent requests and feed results back
5. Return the real result

Example:
  cosca agent run backend-chief "Design an authentication API"
  cosca agent run "Backend Specialist" "create src/hello.go that prints ola"
  cosca agent run "Runtime Chief" "read go.mod and list src/*.go"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			ctx := cmd.Context()
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

			// 1. Build the tool executor with the cross-platform filesystem
			// tools. The sandbox gate is nil so the tools are never blocked by a
			// missing Linux-only bubblewrap: path validation (rails) — which the
			// executor applies on every call via ValidateToolCall, and which each
			// tool also applies — is the protection. This works on Windows and
			// Linux alike.
			reg := tool.NewRegistry()
			fsTools := fs.Register(reg, dir)
			ex := executor.New(reg, nil, dir)

			// 2. Select the configured LLM provider (same wiring as `cosca
			// chat`). Load the project env first so the declared provider
			// (Ollama default) is honoured out of the box.
			projectCfg, _ := config.Load()
			loadChatEnv(projectCfg)

			registry := chat.GetRegistry()
			cfg := chat.DefaultChatRegistryConfig()
			if err := chatprovider.RegisterChatProviders(registry, nil); err != nil {
				formatter.Verbose(fmt.Sprintf("Warning: failed to register chat providers: %v", err))
			}
			if err := registry.Select(ctx, cfg); err != nil {
				return fmt.Errorf("no LLM provider available: %w", err)
			}

			// 3. Discover the available tools (OpenAI-compatible definitions)
			// for the LLM.
			toolDefs, err := ex.ListTools(ctx)
			if err != nil {
				return fmt.Errorf("list tools: %w", err)
			}

			// 4. Build the conversation: system (agent persona) + user prompt.
			systemPrompt := buildAgentSystemPrompt(agent)
			messages := []chat.Message{
				{Role: chat.RoleSystem, Content: systemPrompt},
				{Role: chat.RoleUser, Content: prompt},
			}

			// 5. Run the agent-to-tool loop (LLM → tool calls → execute → feed
			// back → LLM ... until the agent finishes). Before the loop, the
			// conversation is enriched with automatic vision recognition when
			// an image is present and vision is enabled (opt-in).
			finalResponse, err := runAgentToolLoop(ctx, registry, ex, messages, toolDefs, projectCfg.Vision.Enabled)
			if err != nil {
				return fmt.Errorf("agent execution failed: %w", err)
			}

			// 6. Report the real numbers and the real result.
			formatter.KeyValue("Tools", fmt.Sprintf("%d", len(fsTools)))
			formatter.KeyValue("Capabilities", fmt.Sprintf("%d", len(agent.Capabilities)))
			formatter.Println("")
			formatter.Success("Agent execution completed")
			if finalResponse != "" {
				formatter.Println(finalResponse)
			}
			return nil
		},
	}
	return cmd
}

// buildAgentSystemPrompt builds the system prompt for an agent execution from
// its persona: name, role, mission, department, description, and
// responsibilities. It also tells the agent which filesystem tools it can call
// and that paths are relative to the workspace root.
func buildAgentSystemPrompt(a *agents.Agent) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("You are %s — %s.\n", a.Name, a.Role))
	if a.Mission != "" {
		b.WriteString(fmt.Sprintf("Mission: %s\n", a.Mission))
	}
	if a.Department != "" {
		b.WriteString(fmt.Sprintf("Department: %s\n", a.Department))
	}
	if a.Description != "" {
		b.WriteString(fmt.Sprintf("About: %s\n", a.Description))
	}
	if len(a.Responsibilities) > 0 {
		b.WriteString("\nResponsibilities:\n")
		for _, r := range a.Responsibilities {
			b.WriteString("- " + r + "\n")
		}
	}
	b.WriteString("\nYou have access to these filesystem tools via function calling: write_file, read_file, edit_file, list_dir, glob.\n")
	b.WriteString("Paths are relative to the workspace root.\n")
	b.WriteString("You MUST follow these filesystem safety rules (they are enforced by the tools, not optional):\n")
	b.WriteString("- SEMPRE use read_file to read an existing file BEFORE making any change to it. Never modify code you have not read.\n")
	b.WriteString("- edit_file pode ser chamado SOMENTE DEPOIS que o arquivo alvo foi lido (read_file) NESTA execucao. old_string deve ser uma substring LITERAL observada no conteudo retornado pelo read_file.\n")
	b.WriteString("- Use write_file ONLY to create a NEW file. write_file refuses to overwrite a file unless you already read it in this session.\n")
	b.WriteString("- Never delete or overwrite code without first reading and understanding the file.\n")
	b.WriteString("REGRA DE RECUPERACAO (obrigatoria): se uma ferramenta falhar (ex: edit_file -> old_string not found), NAO desista nem use search. Faca: (1) read_file no arquivo alvo, (2) inspecione o conteudo real, (3) chame edit_file com o old_string EXATO observado, (4) repita ate funcionar.\n")
	b.WriteString("After completing the task, describe exactly what you did (e.g. the file created and its content).\n")
	return b.String()
}

// runAgentToolLoop drives the agent→tool loop: it calls the LLM with the
// current messages and tool definitions; if the model requests tool calls, it
// executes them via the Executor, appends the tool results back into the
// conversation, and repeats. It stops when the model produces a final text
// answer (no further tool calls) or after maxToolIterations turns (loop guard).
//
// When visionEnabled is true, the initial conversation is enriched ONCE, before
// the loop, with the semantic understanding of any embedded images (automatic
// vision recognition). The enrichment is additive and never breaks the loop: a
// missing image or a failing pipeline simply leaves the conversation unchanged.
func runAgentToolLoop(
	ctx context.Context,
	provider chat.ChatProvider,
	ex *executor.Executor,
	messages []chat.Message,
	toolDefs []chat.ToolDefinition,
	visionEnabled bool,
) (string, error) {
	const maxToolIterations = 10

	// Automatic vision recognition: inject the semantic understanding of any
	// embedded images into the base conversation, once, before the loop. This
	// is additive and best-effort — no image / disabled / vision failure all
	// keep the messages unchanged, so the tool-calling flow never breaks.
	messages = enrichMessagesWithVision(ctx, messages, visionEnabled, nil)

	for iteration := 0; iteration < maxToolIterations; iteration++ {
		resp, err := provider.Chat(ctx, messages, chat.ChatOptions{
			Temperature: 0.7,
			Tools:       toolDefs,
		})
		if err != nil {
			return "", fmt.Errorf("LLM call failed: %w", err)
		}
		if resp == nil || len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" && len(resp.Choices[0].Message.ToolCalls) == 0 {
			return "", fmt.Errorf("LLM returned an empty response")
		}

		assistantMsg := resp.Choices[0].Message
		if assistantMsg.Role == "" {
			assistantMsg.Role = chat.RoleAssistant
		}

		// The model's own message (content and/or requested tool calls) becomes
		// part of the conversation so it can reason about past steps.
		messages = append(messages, assistantMsg)

		// No tool calls → this is the final answer.
		if len(assistantMsg.ToolCalls) == 0 {
			return assistantMsg.Content, nil
		}

		// Execute each requested tool call and feed the result back to the model.
		for _, tc := range assistantMsg.ToolCalls {
			execCall := executor.ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
			}
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &execCall.Input); err != nil {
					execCall.Input = map[string]interface{}{}
				}
			}
			if execCall.Input == nil {
				execCall.Input = map[string]interface{}{}
			}

			result, _ := ex.Execute(ctx, execCall)
			messages = append(messages, chat.Message{
				Role:       chat.RoleTool,
				Content:    toolCallResultString(result),
				ToolCallID: tc.ID,
			})
		}
	}

	return "", fmt.Errorf("maximum tool iterations (%d) exceeded", maxToolIterations)
}

// toolCallResultString renders an executor.ToolResult into the text that is
// fed back to the LLM as a tool message.
func toolCallResultString(res *executor.ToolResult) string {
	if res == nil {
		return "tool execution returned no result"
	}
	if res.Status == executor.StatusError || res.Error != "" {
		msg := res.Error
		if msg == "" {
			msg = res.Status
		}
		return "Tool error: " + msg
	}
	if s, ok := res.Output.(string); ok {
		return s
	}
	if res.Output != nil {
		return fmt.Sprintf("%v", res.Output)
	}
	return "tool succeeded"
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
