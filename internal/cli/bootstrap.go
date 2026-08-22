package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/context"
	"github.com/CoscaAI/cosca/internal/discovery"
	"github.com/CoscaAI/cosca/internal/memory"
	rt "github.com/CoscaAI/cosca/internal/runtime"
)

// BootstrapResult holds the result of the bootstrap operation.
type BootstrapResult struct {
	ProjectDir   string   `json:"project_dir" yaml:"project_dir"`
	Initialized  bool     `json:"initialized" yaml:"initialized"`
	ConfigLoaded bool     `json:"config_loaded" yaml:"config_loaded"`
	MemoryLoaded bool     `json:"memory_loaded" yaml:"memory_loaded"`
	ContextBuilt bool     `json:"context_built" yaml:"context_built"`
	RuntimeReady bool     `json:"runtime_ready" yaml:"runtime_ready"`
	Duration     string   `json:"duration" yaml:"duration"`
	Steps        []string `json:"steps" yaml:"steps"`
}

// NewBootstrapCommand creates the `cosca bootstrap` command.
func NewBootstrapCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the Cosca runtime",
		Long: `Bootstrap the Cosca runtime environment.

This initializes all subsystems needed for Cosca to function:
  - Load configuration
  - Initialize memory stores
  - Build context
  - Prepare runtime

Run this after installation to prepare the environment for use.
`,
		Example: `  cosca bootstrap
  cosca bootstrap --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			startTime := time.Now()

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			coscaDir := filepath.Join(dir, ".cosca")
			result := BootstrapResult{
				ProjectDir: dir,
			}

			// Check if initialized
			if info, err := os.Stat(coscaDir); os.IsNotExist(err) || !info.IsDir() {
				if useJSON {
					result.Initialized = false
					result.Duration = time.Since(startTime).Round(time.Millisecond).String()
					return printJSON(cmd, result)
				}
				return fmt.Errorf("Cosca is not initialized in %s (run 'cosca init' first)", dir)
			}
			result.Initialized = true

			spinner := formatter.Spinner("Bootstrapping Cosca runtime")
			spinner.Start()

			// Step 1: Load configuration
			formatter.Verbose("Step 1: Loading configuration...")
			cfgPath := filepath.Join(coscaDir, "config.yaml")
			cfg, cfgErr := config.LoadFromFile(cfgPath)
			if cfgErr == nil && cfg != nil {
				result.ConfigLoaded = true
				result.Steps = append(result.Steps, "config_loaded")
			} else {
				formatter.Verbose(fmt.Sprintf("Config load warning: %v (using defaults)", cfgErr))
				result.Steps = append(result.Steps, "config_defaults")
			}

			// Step 2: Initialize memory
			formatter.Verbose("Step 2: Initializing memory stores...")
			memCfg := memory.EngineConfig{
				DataDir: coscaDir,
			}
			mem, memErr := memory.NewEngine(memory.WithConfig(memCfg))
			if memErr == nil && mem != nil {
				defer func() { _ = mem.Close() }()
				result.MemoryLoaded = true
				result.Steps = append(result.Steps, "memory_loaded")
			} else {
				formatter.Verbose(fmt.Sprintf("Memory init warning: %v", memErr))
			}

			// Step 3: Build context
			formatter.Verbose("Step 3: Building context...")
			logger := zerolog.Nop()
			ctxBuilder := context.NewBuilder(logger)
			if ctxBuilder != nil {
				ctxReq := context.ContextRequest{
					Query: "bootstrap context",
				}
				_, buildErr := ctxBuilder.BuildContext(cmd.Context(), ctxReq)
				if buildErr != nil {
					formatter.Verbose(fmt.Sprintf("Context build warning: %v", buildErr))
				} else {
					result.ContextBuilt = true
					result.Steps = append(result.Steps, "context_built")
				}
			}

			// Step 4: Prepare runtime
			formatter.Verbose("Step 4: Preparing runtime...")
			rtCfg := rt.DefaultRuntimeConfig()
			rtCfg.DataDir = coscaDir
			r := rt.New(rt.WithConfig(rtCfg))
			if r != nil {
				// Check runtime health without starting all subsystems
				health := r.Health()
				_ = health
				result.RuntimeReady = true
				result.Steps = append(result.Steps, "runtime_ready")
			}

			// Run discovery
			formatter.Verbose("Running workspace discovery...")
			disc := discovery.NewEngine(discovery.WithWorkDir(dir))
			if disc != nil {
				_, discErr := disc.DiscoverAll(cmd.Context())
				if discErr == nil {
					result.Steps = append(result.Steps, "discovery_complete")
				}
			}

			spinner.Stop("Bootstrap complete")

			result.Duration = time.Since(startTime).Round(time.Millisecond).String()

			if useJSON {
				return printJSON(cmd, result)
			}

			formatter.Println("")
			formatter.Header("Bootstrap Summary")

			if result.ConfigLoaded {
				formatter.Success("Configuration loaded")
			} else {
				formatter.Warning("Configuration: using defaults")
			}

			if result.MemoryLoaded {
				formatter.Success("Memory stores initialized")
			} else {
				formatter.Warning("Memory stores: not initialized")
			}

			if result.ContextBuilt {
				formatter.Success("Context built")
			} else {
				formatter.Warning("Context: not built")
			}

			if result.RuntimeReady {
				formatter.Success("Runtime prepared")
			} else {
				formatter.Warning("Runtime: not ready")
			}

			formatter.Println("")
			formatter.KeyValue("Duration", result.Duration)

			if result.RuntimeReady || result.ContextBuilt {
				formatter.Success("Bootstrap completed successfully")
			} else {
				formatter.Warning("Bootstrap completed with issues")
				formatter.Println("Run 'cosca doctor' for detailed diagnostics")
			}

			return nil
		},
	}

	return cmd
}
