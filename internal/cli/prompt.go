package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/prompts"
)

// NewPromptCommand creates the `cosca prompt` command.
func NewPromptCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Manage Cosca prompts",
		Long: `Manage Cosca prompts.

Prompts are reusable instruction templates that guide AI agents
in performing specific tasks. They can be parameterized and composed.
`,
		Example: `  cosca prompt list
  cosca prompt show code-review
  cosca prompt search "architecture"
  cosca prompt create my-prompt --template default`,
	}

	cmd.AddCommand(
		NewPromptListCommand(),
		NewPromptShowCommand(),
		NewPromptSearchCommand(),
		NewPromptCreateCommand(),
	)

	return cmd
}

// NewPromptListCommand creates the `cosca prompt list` subcommand.
func NewPromptListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available prompts",
		Long:  `List all prompts available in the Cosca system.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := prompts.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("prompt manager not available")
			}

			promptList := mgr.List()

			if useJSON {
				return printJSON(cmd, promptList)
			}

			if len(promptList) == 0 {
				formatter.Warning("No prompts found")
				return nil
			}

			formatter.Header(fmt.Sprintf("Available Prompts (%d)", len(promptList)))

			headers := []string{"Name", "Description", "Version"}
			rows := make([][]string, 0, len(promptList))

			for _, p := range promptList {
				rows = append(rows, []string{p.Name, p.Description, p.Version})
			}

			formatter.Table(headers, rows)
			return nil
		},
	}

	return cmd
}

// NewPromptShowCommand creates the `cosca prompt show` subcommand.
func NewPromptShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show prompt details",
		Long:  `Display the full content and metadata of a specific prompt.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := prompts.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("prompt manager not available")
			}

			prompt, err := mgr.Get(args[0])
			if err != nil {
				return fmt.Errorf("prompt %q not found: %w", args[0], err)
			}

			if useJSON {
				return printJSON(cmd, prompt)
			}

			formatter.Header(fmt.Sprintf("Prompt: %s", prompt.Name))
			formatter.KeyValue("Name", prompt.Name)
			formatter.KeyValue("Description", prompt.Description)
			formatter.KeyValue("Version", prompt.Version)
			formatter.KeyValue("Category", prompt.Category)

			if prompt.Template != "" {
				formatter.Println("")
				formatter.Header("Template")
				formatter.Println(prompt.Template)
			}

			if len(prompt.Parameters) > 0 {
				formatter.Println("")
				formatter.Header("Parameters")
				for _, param := range prompt.Parameters {
					formatter.Bullet(fmt.Sprintf("%s (%s, default: %v)", param.Name, param.Type, param.Default))
				}
			}

			return nil
		},
	}

	return cmd
}

// NewPromptSearchCommand creates the `cosca prompt search` subcommand.
func NewPromptSearchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search prompts",
		Long:  `Search for prompts by name, description, or category.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := prompts.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("prompt manager not available")
			}

			results, err := mgr.Search(args[0])
			if err != nil {
				return fmt.Errorf("prompt search failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, results)
			}

			if len(results) == 0 {
				formatter.Warning(fmt.Sprintf("No prompts found matching %q", args[0]))
				return nil
			}

			formatter.Header(fmt.Sprintf("Prompt Search Results for %q", args[0]))
			for _, r := range results {
				formatter.KeyValue(r.Name, r.Description)
			}
			formatter.KeyValue("Total", fmt.Sprintf("%d", len(results)))

			return nil
		},
	}

	return cmd
}

// NewPromptCreateCommand creates the `cosca prompt create` subcommand.
func NewPromptCreateCommand() *cobra.Command {
	var template string
	var description string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a prompt",
		Long:  `Create a new prompt from a template or from scratch.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			spinner := formatter.Spinner(fmt.Sprintf("Creating prompt %s", args[0]))
			spinner.Start()

			mgr := prompts.NewManager(coscaDir)
			if mgr == nil {
				spinner.Fail("Prompt manager not available")
				return fmt.Errorf("prompt manager not available")
			}

			opts := prompts.CreateOptions{
				Name:        args[0],
				Template:    template,
				Description: description,
			}

			prompt, err := mgr.Create(opts)
			if err != nil {
				spinner.Fail(fmt.Sprintf("Create failed: %v", err))
				return fmt.Errorf("prompt creation failed: %w", err)
			}

			spinner.Stop("Prompt created")

			if useJSON {
				return printJSON(cmd, prompt)
			}

			formatter.Success(fmt.Sprintf("Prompt %s created", prompt.Name))
			return nil
		},
	}

	cmd.Flags().StringVarP(&template, "template", "t", "default", "template to use")
	cmd.Flags().StringVarP(&description, "description", "d", "", "prompt description")
	return cmd
}
