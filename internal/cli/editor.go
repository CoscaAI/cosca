package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/editors"
	"github.com/CoscaAI/cosca/internal/installers"
)

// NewEditorCommand creates the `cosca editor` command and its subcommands.
func NewEditorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "editor",
		Short: "Manage editor integration",
		Long: `Manage the Cosca editor integration.

Cosca integrates with popular editors and IDEs to provide AI-powered
assistance directly in your development environment.

Subcommands:
  list           List all supported editors and their integration state
  setup          Setup editor integration
  status         Check editor integration status
  adapt <editor> Create an adapter for a specific editor
`,
		Example: `  cosca editor list
  cosca editor setup
  cosca editor setup --editor claude
  cosca editor status
  cosca editor adapt vscode
  cosca editor adapt intellij`,
	}

	cmd.AddCommand(
		NewEditorListCommand(),
		NewEditorSetupCommand(),
		NewEditorStatusCommand(),
		NewEditorAdaptCommand(),
	)

	return cmd
}

// editorSetupState describes how an editor appears in `cosca editor list`.
const (
	editorStateConfigured    = "✓ configurado"
	editorStateDetected      = "? detectado"
	editorStateNotConfigured = "✗ não configurado"
)

// NewEditorListCommand creates the `cosca editor list` subcommand.
func NewEditorListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List supported editors and their integration state",
		Long: `List every editor supported by Cosca and whether the Cosca integration
is configured, detected, or missing.

States:
  ✓ configurado     Cosca integration is installed and valid
  ? detectado       Editor found on the system, integration pending
  ✗ não configurado Integration not installed

Use 'cosca editor setup --editor <name>' to configure a specific editor.
`,
		Example: `  cosca editor list
  cosca editor list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			cfg := editors.DefaultEditorConfig(dir)
			mgr := editors.NewManager(cfg)
			if mgr == nil {
				return fmt.Errorf("editor manager not available")
			}

			// Run detection first so List() can report detected/setup state.
			mgr.DetectAll()
			infos := mgr.List()

			type editorListEntry struct {
				Name       string `json:"name"`
				Configured bool   `json:"configured"`
				Detected   bool   `json:"detected"`
				State      string `json:"state"`
				Version    string `json:"version,omitempty"`
				Path       string `json:"path,omitempty"`
			}

			entries := make([]editorListEntry, 0, len(infos))
			rows := make([][]string, 0, len(infos))

			for _, info := range infos {
				state := editorStateNotConfigured
				configured := false
				if err := mgr.Validate(info.Name); err == nil {
					state = editorStateConfigured
					configured = true
				} else if info.Detected {
					state = editorStateDetected
				}

				entries = append(entries, editorListEntry{
					Name:       info.Name,
					Configured: configured,
					Detected:   info.Detected,
					State:      state,
					Version:    info.Version,
					Path:       info.Path,
				})
				rows = append(rows, []string{info.Name, state, info.Version, info.Path})
			}

			if useJSON {
				return printJSON(cmd, entries)
			}

			formatter.Header("Editores Suportados")
			formatter.Table([]string{"Editor", "Estado", "Versão", "Caminho"}, rows)
			formatter.Println("")
			formatter.Bullet("✓ configurado · ? detectado (setup pendente) · ✗ não configurado")
			formatter.Bullet("Rode 'cosca editor setup --editor <name>' para configurar um editor específico")

			return nil
		},
	}

	return cmd
}

// NewEditorSetupCommand creates the `cosca editor setup` subcommand.
func NewEditorSetupCommand() *cobra.Command {
	var editorFlag string

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup editor integration",
		Long: `Auto-detect the editor and install the Cosca integration.

Use --editor <name> to install the integration into ONE specific editor
without auto-detection (the name is validated against the registered
editors). Without --editor the active editor is auto-detected.

Supports VS Code, JetBrains IDEs, Vim, Neovim, Emacs, and more.
The integration provides AI-powered features directly in the editor.
`,
		Example: `  cosca editor setup
  cosca editor setup --editor claude
  cosca editor setup --editor codex`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			// Use editors.NewManager instead of non-existent NewDetector
			cfg := editors.DefaultEditorConfig(dir)
			mgr := editors.NewManager(cfg)
			if mgr == nil {
				return fmt.Errorf("editor manager not available")
			}

			// Explicit editor: install into exactly one editor, no detection needed.
			if editorFlag != "" {
				if _, err := mgr.GetAdapter(editorFlag); err != nil {
					return fmt.Errorf("unsupported editor: %q (supported: %s)",
						editorFlag, strings.Join(mgr.Names(), ", "))
				}

				formatter.Verbose(fmt.Sprintf("Setting up %s integration", editorFlag))

				if err := mgr.SetupForce(editorFlag); err != nil {
					return fmt.Errorf("failed to install editor integration for %s: %w", editorFlag, err)
				}

				if useJSON {
					return printJSON(cmd, map[string]string{
						"editor":   editorFlag,
						"status":   "installed",
						"location": coscaDir,
					})
				}

				formatter.Success(fmt.Sprintf("%s integration installed", editorFlag))
				formatter.Bullet(fmt.Sprintf("Editor config written to %s", coscaDir))
				formatter.Bullet("Restart your editor to activate the integration")

				return nil
			}

			// Detect returns (types.EditorInfo, error) instead of just a string
			info, err := mgr.Detect()
			if err != nil {
				formatter.Warning("No supported editor detected")
				formatter.Println("You can manually specify an editor:")
				formatter.Println("  cosca editor adapt <editor>")
				formatter.Println("")
				formatter.Println("Supported editors: vscode, intellij, vim, neovim, emacs, sublime, atom")
				return err
			}

			editorName := info.Name
			formatter.Verbose(fmt.Sprintf("Detected editor: %s", editorName))

			spinner := formatter.Spinner(fmt.Sprintf("Setting up %s integration", editorName))
			spinner.Start()

			// Use installer if available, otherwise fall back to editor manager's Setup
			installer := installers.NewInstaller(dir)
			if installer != nil {
				if err := installer.InstallEditorIntegration(editorName); err != nil {
					// Fall back to editor manager Setup
					if setupErr := mgr.Setup(editorName); setupErr != nil {
						spinner.Fail(fmt.Sprintf("Integration failed: %v", setupErr))
						return fmt.Errorf("failed to install editor integration: %w", setupErr)
					}
				}
			} else {
				if err := mgr.Setup(editorName); err != nil {
					spinner.Fail(fmt.Sprintf("Integration failed: %v", err))
					return fmt.Errorf("failed to install editor integration: %w", err)
				}
			}

			spinner.Stop("Integration installed")

			if useJSON {
				return printJSON(cmd, map[string]string{
					"editor":   editorName,
					"status":   "installed",
					"location": coscaDir,
				})
			}

			formatter.Success(fmt.Sprintf("%s integration installed", editorName))
			formatter.Bullet(fmt.Sprintf("Editor config written to %s", coscaDir))
			formatter.Bullet("Restart your editor to activate the integration")

			return nil
		},
	}

	cmd.Flags().StringVar(&editorFlag, "editor", "", "install into a specific editor by name (e.g. --editor claude)")

	return cmd
}

// NewEditorStatusCommand creates the `cosca editor status` subcommand.
func NewEditorStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check editor integration status",
		Long:  `Check whether the Cosca editor integration is installed and active.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			// Use editors.NewManager instead of non-existent NewDetector
			cfg := editors.DefaultEditorConfig(dir)
			mgr := editors.NewManager(cfg)

			detectedEditor := "unknown"
			isIntegrated := false
			integrationVersion := "unknown"

			if mgr != nil {
				// Detect returns (types.EditorInfo, error)
				info, err := mgr.Detect()
				if err == nil {
					detectedEditor = info.Name
					integrationVersion = info.Version

					// Validate checks if integration is properly configured
					validateErr := mgr.Validate(info.Name)
					isIntegrated = validateErr == nil
				}
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"detected_editor":     detectedEditor,
					"integrated":          isIntegrated,
					"integration_version": integrationVersion,
				})
			}

			formatter.Header("Editor Integration Status")
			formatter.KeyValue("Detected Editor", detectedEditor)
			formatter.KeyValue("Integration", fmt.Sprintf("%v", isIntegrated))
			formatter.KeyValue("Version", integrationVersion)

			if !isIntegrated {
				formatter.Println("")
				formatter.Warning("Editor integration is not installed")
				formatter.Println("Run 'cosca editor setup' to install the integration")
			}

			return nil
		},
	}

	return cmd
}

// NewEditorAdaptCommand creates the `cosca editor adapt` subcommand.
func NewEditorAdaptCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adapt <editor>",
		Short: "Create editor adapter",
		Long: `Create an Cosca integration adapter for a specific editor or IDE.

Supported editors: vscode, intellij, vim, neovim, emacs, sublime, atom, cursor
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			editor := args[0]
			dir, _ := os.Getwd()

			validEditors := map[string]bool{
				"vscode": true, "intellij": true, "vim": true,
				"neovim": true, "emacs": true, "sublime": true,
				"atom": true, "cursor": true,
			}

			if !validEditors[editor] {
				return fmt.Errorf("unsupported editor: %s (supported: vscode, intellij, vim, neovim, emacs, sublime, atom, cursor)", editor)
			}

			formatter.Verbose(fmt.Sprintf("Creating adapter for %s...", editor))

			spinner := formatter.Spinner(fmt.Sprintf("Creating %s adapter", editor))
			spinner.Start()

			// Use editors.NewManager and SetUp instead of non-existent NewAdapter/Install
			cfg := editors.DefaultEditorConfig(dir)
			mgr := editors.NewManager(cfg)
			if mgr == nil {
				spinner.Fail("Editor manager not available")
				return fmt.Errorf("failed to create editor adapter")
			}

			if err := mgr.Setup(editor); err != nil {
				spinner.Fail(fmt.Sprintf("Adapter installation failed: %v", err))
				return fmt.Errorf("failed to install editor adapter: %w", err)
			}

			spinner.Stop("Adapter created")

			if useJSON {
				return printJSON(cmd, map[string]string{
					"editor": editor,
					"status": "adapter_created",
				})
			}

			formatter.Success(fmt.Sprintf("%s adapter created", editor))
			formatter.Bullet("Restart your editor to activate")
			return nil
		},
	}

	return cmd
}
