package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/telemetry"
)

// rootCmdMu protects the global variables (cfgFile, globalFlags) from
// concurrent access when NewRootCommand is called from parallel tests.
var rootCmdMu sync.Mutex

// contextKey is used for storing values in the command context.
type contextKey string

const (
	formatterKey contextKey = "formatter"
	verboseKey   contextKey = "verbose"
	quietKey     contextKey = "quiet"
	noColorKey   contextKey = "no-color"
)

// GlobalFlags holds the values of global command-line flags.
type GlobalFlags struct {
	Config  string
	Verbose bool
	Quiet   bool
	JSON    bool
	Format  string
	NoColor bool
}

var (
	globalFlags GlobalFlags
	cfgFile     string
)

// NewRootCommand creates the root `cosca` command.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "cosca",
		Short: "Cosca — AI Orchestration System Enterprise Platform",
		Long: `Cosca is the command-line interface for the AI Orchestration System (Cosca) 
Enterprise Platform. It provides tools for managing agents, skills, prompts, 
workflows, knowledge, memory, and all other Cosca subsystems.

Cosca is an enterprise-grade AI orchestration framework that enables organizations 
to build, deploy, and manage AI-powered workflows across multiple runtimes.

Documentation: https://cosca.enterprise/docs
`,
		Example: `  cosca init                Initialize Cosca in the current project
  cosca install             Full auto-install flow
  cosca status              Show system status
  cosca doctor              Run system diagnostics
  cosca plan                Estimate an execution plan before approval
  cosca approve             Approve an execution plan (runs tests + audit)
  cosca delegate            Delegate a task with a prior execution plan
  cosca knowledge search    Search the knowledge base
  cosca runtime start       Start the runtime daemon
  cosca version             Show version information`,
		SilenceUsage:      true,
		SilenceErrors:     true,
		Version:           Version,
		PersistentPreRunE: persistentPreRun,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// If no subcommand is given, show help
			return cmd.Help()
		},
	}

	// Global flags — protected by mutex because pflag.StringVar/BoolVarP
	// write to shared package-level variables (cfgFile, globalFlags).
	rootCmdMu.Lock()
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "path to config file (default: .cosca/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&globalFlags.Verbose, "verbose", "V", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&globalFlags.Quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVarP(&globalFlags.JSON, "json", "j", false, "output in JSON format")
	rootCmd.PersistentFlags().StringVar(&globalFlags.Format, "format", "text", "output format (text, json, yaml, table)")
	rootCmd.PersistentFlags().BoolVar(&globalFlags.NoColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolP("version", "v", false, "version for cosca")
	rootCmdMu.Unlock()

	// Disable the default completion command (we provide our own)
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Add subcommands
	rootCmd.AddCommand(
		NewStartCommand(),
		NewInitCommand(),
		NewInstallCommand(),
		NewUninstallCommand(),
		NewUpdateCommand(),
		NewUpgradeCommand(),
		NewSyncCommand(),
		NewStatusCommand(),
		NewVersionCommand(),
		NewDespertarCommand(),
		NewKnowledgeCommand(),
		NewSearchCommand(),
		NewDoctorCommand(),
		NewRuntimeCommand(),
		NewConfigCommand(),
		NewCacheCommand(),
		NewContextCommand(),
		NewMemoryCommand(),
		NewKernelCommand(),
		NewDonCommand(),
		NewCircadianCommand(),
		NewPluginCommand(),
		NewEditorCommand(),
		NewProviderCommand(),
		NewModelsCommand(),
		NewIndexCommand(),
		NewGraphCommand(),
		NewQGateCommand(),
		NewPluginsCommand(),
		NewPipelineTestCommand(),
		NewWorkflowCommand(),
		NewAgentCommand(),
		NewSkillCommand(),
		NewSkillsCommand(),
		NewPromptCommand(),
		NewTemplateCommand(),
		NewDocsCommand(),
		NewHealthCommand(),
		NewValidateCommand(),
		NewBenchmarkCommand(),
		NewSymbolsCommand(),
		NewEmbedCommand(),
		NewEvalCommand(),
		NewBootstrapCommand(),
		NewCompletionCommand(),
		NewRunCommand(),
		NewPipelineCommand(),
		NewChatCommand(),
		NewExecCommand(),
		NewMCPCommand(),
		NewMetricsCommand(),
		NewServeCommand(),
		NewHookCommand(),
		NewPlanCommand(),
		NewApproveCommand(),
		NewProposeCommand(),
		NewDecisionCommand(),
		NewDelegateCommand(),
		NewLicenseCommand(),
		NewCVCommand(),
		NewCapabilityCommand(),
		NewQuarantineCommand(),
		NewGateCommand(),
		NewBugCommand(),
		NewHardwareCommand(),
		NewFabricCommand(),
		NewMachineCommand(),
		NewCronCommand(),
		NewSessionCommand(),
		NewTraceCommand(),
		NewConflictCommand(),
		NewEvidenceCommand(),
		NewBudgetCommand(),
		NewRankingCommand(),
		NewAcquisitionCommand(),
		NewDepartmentCommand(),
		NewTerminalCommand(),
		NewProjectCommand(),
		NewAssetCommand(),
		NewTaskCommand(),
		NewModelCommand(),
		NewGPUCommand(),
		NewNodeGraphCommand(),
		NewRenderCommand(),
		NewMediaCommand(),
		NewFlowCommand(),
		NewProvenanceCommand(),
		NewSecurityCommand(),
		NewDesktopCommand(),
		NewVoiceCommand(),
		NewSlopCommand(),
	)

	return rootCmd
}

// persistentPreRun runs before every command for initialization.
func persistentPreRun(cmd *cobra.Command, args []string) error {
	// Initialize configuration
	if err := initConfig(cmd); err != nil {
		return fmt.Errorf("config initialization failed: %w", err)
	}

	// Set up output formatting
	format := OutputFormat(globalFlags.Format)
	if globalFlags.JSON {
		format = OutputFormatJSON
	}

	formatter := NewOutputFormatter(
		cmd.OutOrStdout(),
		format,
		globalFlags.Verbose,
		globalFlags.Quiet,
		globalFlags.NoColor,
	)

	// Store formatter in command context for child commands to use
	ctx := context.WithValue(cmd.Context(), formatterKey, formatter)
	cmd.SetContext(ctx)

	// Emit telemetry event
	telemetry.Emit("command_executed", map[string]interface{}{
		"command": cmd.Name(),
		"args":    args,
	})

	return nil
}

// initConfig initializes viper configuration.
func initConfig(_ *cobra.Command) error {
	v := viper.New()

	if cfgFile != "" {
		// Use config file from the flag
		v.SetConfigFile(cfgFile)
	} else {
		// Search for .cosca/config.yaml in current directory and parents
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		coscaDir := filepath.Join(dir, ".cosca")
		if info, err := os.Stat(coscaDir); err == nil && info.IsDir() {
			v.AddConfigPath(coscaDir)
			v.SetConfigName("config")
		}

		// Also check home directory for global config
		home, err := homedir.Dir()
		if err == nil {
			v.AddConfigPath(filepath.Join(home, ".cosca"))
		}

		v.SetConfigName("config")
	}

	// Environment variable bindings
	v.SetEnvPrefix("Cosca")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Default values
	viper.SetDefault("cache.enabled", true)
	viper.SetDefault("cache.ttl", 3600)
	viper.SetDefault("bootstrap.auto_init", true)
	viper.SetDefault("observability.log_resolution_events", true)

	// Try to read config
	if err := v.ReadInConfig(); err != nil {
		// Config file is optional
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config: %w", err)
		}
	}

	config.SetViper(v)
	return nil
}

// GetFormatter extracts the OutputFormatter from the command context.
func GetFormatter(cmd *cobra.Command) *OutputFormatter {
	if v, ok := cmd.Context().Value(formatterKey).(*OutputFormatter); ok {
		return v
	}
	return NewOutputFormatter(os.Stdout, OutputFormatText, false, false, false)
}

// IsJSONOutput checks if JSON output is requested.
func IsJSONOutput(cmd *cobra.Command) bool {
	jsonFlag := cmd.Root().PersistentFlags().Lookup("json")
	if jsonFlag != nil && jsonFlag.Value.String() == "true" {
		return true
	}
	return globalFlags.JSON
}

// printJSON is a helper to print JSON output from commands.
func printJSON(cmd *cobra.Command, v interface{}) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// newContextWithFormatter stores the formatter in the context.
func newContextWithFormatter(ctx context.Context, formatter *OutputFormatter) context.Context {
	return context.WithValue(ctx, formatterKey, formatter)
}
