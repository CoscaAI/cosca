package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/templates"
)

// NewTemplateCommand creates the `cosca template` command.
func NewTemplateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage Cosca templates",
		Long: `Manage Cosca templates.

Templates are re-usable blueprints for prompts, skills, workflows,
and other Cosca artifacts. They provide a starting point for creating
new resources with consistent structure.
`,
		Example: `  cosca template list
  cosca template show my-template
  cosca template search "workflow"
  cosca template create my-template --type prompt`,
	}

	cmd.AddCommand(
		NewTemplateListCommand(),
		NewTemplateShowCommand(),
		NewTemplateSearchCommand(),
		NewTemplateCreateCommand(),
	)

	return cmd
}

// NewTemplateListCommand creates the `cosca template list` subcommand.
func NewTemplateListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available templates",
		Long:  `List all templates available in the Cosca system.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := templates.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("template manager not available")
			}

			templateList := mgr.List()

			if useJSON {
				return printJSON(cmd, templateList)
			}

			if len(templateList) == 0 {
				formatter.Warning("Nenhum template encontrado. Templates built-in estao disponiveis via o Cosca Framework.")
				return nil
			}

			formatter.Header(fmt.Sprintf("Available Templates (%d)", len(templateList)))

			headers := []string{"Name", "Description", "Type", "Version"}
			rows := make([][]string, 0, len(templateList))

			for _, t := range templateList {
				rows = append(rows, []string{t.Name, t.Description, t.Type, t.Version})
			}

			formatter.Table(headers, rows)
			return nil
		},
	}

	return cmd
}

// NewTemplateShowCommand creates the `cosca template show` subcommand.
func NewTemplateShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show template details",
		Long:  `Display detailed information about a specific template.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := templates.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("template manager not available")
			}

			tmpl, err := mgr.Get(args[0])
			if err != nil {
				return fmt.Errorf("template %q not found: %w", args[0], err)
			}

			if useJSON {
				return printJSON(cmd, tmpl)
			}

			formatter.Header(fmt.Sprintf("Template: %s", tmpl.Name))
			formatter.KeyValue("Name", tmpl.Name)
			formatter.KeyValue("Description", tmpl.Description)
			formatter.KeyValue("Type", tmpl.Type)
			formatter.KeyValue("Version", tmpl.Version)

			if tmpl.Content != "" {
				formatter.Println("")
				formatter.Header("Content")
				formatter.Println(tmpl.Content)
			}

			return nil
		},
	}

	return cmd
}

// NewTemplateSearchCommand creates the `cosca template search` subcommand.
func NewTemplateSearchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search templates",
		Long:  `Search for templates by name, description, or type.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := templates.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("template manager not available")
			}

			results, err := mgr.Search(args[0])
			if err != nil {
				return fmt.Errorf("template search failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, results)
			}

			if len(results) == 0 {
				formatter.Warning(fmt.Sprintf("No templates found matching %q", args[0]))
				return nil
			}

			formatter.Header(fmt.Sprintf("Template Search Results for %q", args[0]))
			for _, r := range results {
				formatter.KeyValue(r.Name, r.Description)
			}
			formatter.KeyValue("Total", fmt.Sprintf("%d", len(results)))

			return nil
		},
	}

	return cmd
}

// NewTemplateCreateCommand creates the `cosca template create` subcommand.
func NewTemplateCreateCommand() *cobra.Command {
	var templateType string
	var description string
	var content string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a template",
		Long:  `Create a new template for prompts, skills, workflows, or other Cosca artifacts.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			spinner := formatter.Spinner(fmt.Sprintf("Creating template %s", args[0]))
			spinner.Start()

			mgr := templates.NewManager(coscaDir)
			if mgr == nil {
				spinner.Fail("Template manager not available")
				return fmt.Errorf("template manager not available")
			}

			opts := templates.CreateOptions{
				Name:        args[0],
				Type:        templateType,
				Description: description,
				Content:     content,
			}

			tmpl, err := mgr.Create(opts)
			if err != nil {
				spinner.Fail(fmt.Sprintf("Create failed: %v", err))
				return fmt.Errorf("template creation failed: %w", err)
			}

			spinner.Stop("Template created")

			if useJSON {
				return printJSON(cmd, tmpl)
			}

			formatter.Success(fmt.Sprintf("Template %s created", tmpl.Name))
			return nil
		},
	}

	cmd.Flags().StringVarP(&templateType, "type", "t", "prompt", "template type (prompt, skill, workflow)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "template description")
	cmd.Flags().StringVarP(&content, "content", "c", "", "template content")
	return cmd
}
