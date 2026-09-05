package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/chat"
	chatprovider "github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/providers/bootstrap"
)

// NewProviderCommand creates the `cosca provider` command and its subcommands.
func NewProviderCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage AI providers",
		Long: `Manage AI provider configurations.

Providers are AI model backends used for embeddings, completions, and other AI tasks.
`,
		Example: `  cosca provider list              List all providers
  cosca provider set openai        Set OpenAI as active provider
  cosca provider test openai       Test OpenAI connection
  cosca provider info openai       Show provider info
  cosca provider bootstrap         Detect and bootstrap the Ollama provider`,
	}

	cmd.AddCommand(
		NewProviderListCommand(),
		NewProviderSetCommand(),
		NewProviderTestCommand(),
		NewProviderInfoCommand(),
		NewProviderWatchCommand(),
		NewProviderBootstrapCommand(),
	)

	return cmd
}

// NewProviderListCommand creates the `cosca provider list` subcommand.
func NewProviderListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available providers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			mgr := NewProviderManager()
			providers := mgr.List()

			if useJSON {
				return printJSON(cmd, providers)
			}

			formatter.Header("Available Providers")
			var rows [][]string
			for _, p := range providers {
				rows = append(rows, []string{p.Name, p.Status, p.Model})
			}
			formatter.Table([]string{"Name", "Status", "Model"}, rows)

			return nil
		},
	}

	return cmd
}

// NewProviderSetCommand creates the `cosca provider set` subcommand.
func NewProviderSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <provider> [model]",
		Short: "Set the active provider",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			model := ""
			if len(args) > 1 {
				model = args[1]
			}

			mgr := NewProviderManager()
			if err := mgr.SetActive(args[0], model); err != nil {
				return fmt.Errorf("failed to set provider: %w", err)
			}

			// Persist to project config so the setting survives process restarts.
			if err := persistProviderToConfig(args[0]); err != nil {
				formatter.Verbose(fmt.Sprintf("Warning: could not persist provider to config: %v", err))
			}

			if useJSON {
				return printJSON(cmd, map[string]string{"status": "ok", "provider": args[0]})
			}

			formatter.Success(fmt.Sprintf("Active provider set to %s", args[0]))
			return nil
		},
	}

	return cmd
}

// persistProviderToConfig writes provider.name to the project config file
// so the active provider survives process restarts.
func persistProviderToConfig(provider string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(dir, ".cosca", "config.yaml")

	cfg, err := config.LoadFromFile(cfgPath)
	if err != nil {
		// Config doesn't exist yet — create a minimal one.
		cfg = &config.Config{
			Version: "1",
			Provider: config.ProviderConfig{
				Name: provider,
			},
		}
	} else {
		cfg.Provider.Name = provider
	}

	return cfg.Save(cfgPath)
}

// NewProviderTestCommand creates the `cosca provider test` subcommand.
func NewProviderTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <provider>",
		Short: "Test a provider connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			mgr := NewProviderManager()
			result, err := mgr.Test(args[0])
			if err != nil {
				return fmt.Errorf("provider test failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, result)
			}

			if result.Status == "ok" {
				formatter.Success(fmt.Sprintf("Provider %s: connection OK (latency: %s)", args[0], result.ResponseTime))
			} else {
				formatter.Errorf("Provider %s: connection FAILED: %s", args[0], result.Status)
			}

			return nil
		},
	}

	return cmd
}

// NewProviderInfoCommand creates the `cosca provider info` subcommand.
func NewProviderInfoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <provider>",
		Short: "Show provider information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			mgr := NewProviderManager()
			info, err := mgr.Info(args[0])
			if err != nil {
				return fmt.Errorf("failed to get provider info: %w", err)
			}

			if useJSON {
				return printJSON(cmd, info)
			}

			formatter.Header(fmt.Sprintf("Provider: %s", info.Name))
			formatter.KeyValue("Status", info.Status)
			formatter.KeyValue("Model", info.Model)
			formatter.KeyValue("Version", info.APIVersion)

			return nil
		},
	}

	return cmd
}

// NewProviderWatchCommand creates the `cosca provider watch` subcommand.
func NewProviderWatchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch for provider config changes and auto-reload",
		Long: `Start a background watcher that monitors configuration files
and environment variables for provider changes.

When a change is detected (e.g., new API key set), the chat provider
is automatically re-selected without restarting the daemon.

Example:
  cosca provider watch`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			registry := chat.GetRegistry()
			cfg := chat.DefaultChatRegistryConfig()

			// Register the built-in chat provider factories so the watcher
			// actually has providers to watch. Non-fatal: a registration
			// failure still surfaces via Select below.
			if err := chatprovider.RegisterChatProviders(registry, nil); err != nil {
				formatter.Verbose(fmt.Sprintf("Warning: failed to register chat providers: %v", err))
			}

			if err := registry.Select(cmd.Context(), cfg); err != nil {
				return fmt.Errorf("failed to select initial provider: %w", err)
			}

			hotReload, err := chat.NewHotReload(registry, chat.DefaultHotReloadConfig())
			if err != nil {
				return fmt.Errorf("failed to create watcher: %w", err)
			}

			hotReload.Start()
			defer hotReload.Stop()

			formatter.Success(fmt.Sprintf("Watching for provider config changes... (provider: %s, model: %s)", registry.Name(), registry.Model()))
			formatter.Println("Press Ctrl+C to stop.")

			// Block until context is cancelled.
			<-cmd.Context().Done()
			return nil
		},
	}

	return cmd
}

// NewProviderBootstrapCommand creates the `cosca provider bootstrap`
// subcommand: it detects, installs, starts and validates the Ollama provider
// end to end, producing a per-step evidence report.
//
// By default the command only DETECTS and reports — it never installs or
// pulls anything. Auto-install and auto-pull are opt-in flags. Every step is
// idempotent: an already-reached state is reported as "skipped".
//
// When the active provider is not ollama the command still reports the local
// Ollama state but never installs/starts/pulls.
func NewProviderBootstrapCommand() *cobra.Command {
	var (
		allowInstall bool
		allowPull    bool
		model        string
		baseURL      string
		jsonOut      bool
	)

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Detect and bootstrap the Ollama provider (install daemon + pull model)",
		Long: `Detect, install, start and validate the Ollama provider end to end.

By default this command only DETECTS the current state and reports it — it
never installs or pulls anything. Installation and model pulls are opt-in:

  cosca provider bootstrap                   detect only (no side effects)
  cosca provider bootstrap --install         install Ollama if missing
  cosca provider bootstrap --pull            pull the configured model if missing
  cosca provider bootstrap --install --pull  full bootstrap

Every step is idempotent: an already-reached state is reported as "skipped"
with its evidence instead of being re-done.

Install mechanisms (detected, never assumed):
  Windows: winget -> chocolatey -> scoop -> official OllamaSetup.exe (silent)
  Linux:   official install script (curl + sh)

When the active provider is NOT ollama the command still reports the local
Ollama state but never installs, starts or pulls anything.
`,
		Example: `  cosca provider bootstrap
  cosca provider bootstrap --install --pull
  cosca provider bootstrap --json
  cosca provider bootstrap --model qwen2.5-coder:14b-128k --url http://localhost:11434`,
		Args:            cobra.NoArgs,
		SilenceUsage:    true,
		SilenceErrors:   true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd) || jsonOut
			ctx := cmd.Context()

			projectCfg, cfgErr := config.Load()
			if cfgErr != nil {
				formatter.Verbose(fmt.Sprintf("Warning: could not load config (%v); using defaults", cfgErr))
			}

			bCfg := bootstrap.Config{
				Model:            model,
				BaseURL:          baseURL,
				AllowAutoInstall: allowInstall,
				AllowAutoPull:    allowPull,
			}
			if projectCfg != nil {
				if bCfg.Model == "" {
					bCfg.Model = projectCfg.Provider.Model
				}
				if bCfg.BaseURL == "" {
					bCfg.BaseURL = projectCfg.Provider.BaseURL
				}
			}
			if bCfg.Model == "" {
				bCfg.Model = config.DefaultProviderModel
			}
			if bCfg.BaseURL == "" {
				bCfg.BaseURL = bootstrap.DefaultBaseURL
			}

			// The bootstrap chain is specific to the Ollama provider. When the
			// active provider is not ollama we still report the local state but
			// never install/start/pull — the warning goes to stderr so stdout
			// stays pure JSON under --json.
			isOllama := projectCfg == nil || strings.EqualFold(projectCfg.Provider.Name, "ollama")
			if !isOllama {
				msg := fmt.Sprintf("active provider is %q — the bootstrap is specific to Ollama; reporting local Ollama state only (no install/start/pull)",
					activeProviderName(projectCfg))
				if useJSON {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Warning: "+msg)
				} else {
					formatter.Warning(msg)
				}
				bCfg.AllowAutoInstall = false
				bCfg.AllowAutoPull = false
				bCfg.DetectOnly = true
			}

			report, err := bootstrap.EnsureOllama(ctx, bCfg)
			if err != nil {
				return fmt.Errorf("ollama bootstrap failed: %w", err)
			}

			if useJSON {
				if err := printJSON(cmd, report); err != nil {
					return err
				}
			} else {
				printBootstrapReport(formatter, report)
			}

			if !report.OK {
				return fmt.Errorf("ollama bootstrap incomplete: %s", report.Error)
			}
			if !useJSON {
				formatter.Success("Ollama provider is fully functional")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&allowInstall, "install", false, "install Ollama automatically when missing (Windows: winget/choco/scoop/direct; Linux: official script)")
	cmd.Flags().BoolVar(&allowPull, "pull", false, "pull the configured model automatically when missing")
	cmd.Flags().StringVar(&model, "model", "", "model to verify/pull (default: provider.model from config)")
	cmd.Flags().StringVar(&baseURL, "url", "", "Ollama base URL (default: provider.base_url from config, or http://localhost:11434)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output the report as JSON")

	return cmd
}

// activeProviderName returns the configured provider name, or "(unset)".
func activeProviderName(cfg *config.Config) string {
	if cfg == nil || cfg.Provider.Name == "" {
		return "(unset)"
	}
	return cfg.Provider.Name
}

// printBootstrapReport renders the bootstrap Report as a human table.
func printBootstrapReport(f *OutputFormatter, r *bootstrap.Report) {
	f.Header("Ollama Bootstrap Report")
	f.KeyValue("Provider", r.Provider)
	f.KeyValue("OK", fmt.Sprintf("%t", r.OK))
	if r.Model != "" {
		f.KeyValue("Model", r.Model)
	}

	var rows [][]string
	for _, s := range r.Steps {
		rows = append(rows, []string{s.Name, s.Status, s.Duration, s.Detail})
	}
	f.Table([]string{"Step", "Status", "Duration", "Evidence"}, rows)

	if r.Error != "" {
		f.Errorf("Error: %s", r.Error)
	}
}
