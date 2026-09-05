package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/discovery"
	"github.com/CoscaAI/cosca/internal/editors"
	"github.com/CoscaAI/cosca/internal/embed"
	embedcosca "github.com/CoscaAI/cosca/internal/embed/cosca"
)

// InitResult holds the result of the init operation.
type InitResult struct {
	ProjectDir  string              `json:"project_dir" yaml:"project_dir"`
	CoscaDir    string              `json:"cosca_dir" yaml:"cosca_dir"`
	ConfigFile  string              `json:"config_file" yaml:"config_file"`
	Constraints *config.Constraints `json:"constraints" yaml:"constraints"`
	FoundEnv    EnvInfo             `json:"found_env" yaml:"found_env"`
	NextSteps   []string            `json:"next_steps" yaml:"next_steps"`
	Directories []string            `json:"directories" yaml:"directories"`
}

// EnvInfo holds environment discovery information.
type EnvInfo struct {
	Framework string `json:"framework" yaml:"framework"`
	Language  string `json:"language" yaml:"language"`
	Runtime   string `json:"runtime" yaml:"runtime"`
}

// NewInitCommand creates the `cosca init` command.
func NewInitCommand() *cobra.Command {
	var force bool
	var (
		noDocs     bool
		noNetwork  bool
		noTest     bool
		noBuild    bool
		readOnly   bool
		maxFiles   int
		maxTimeStr string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Cosca in the current project",
		Long: `Initialize the Cosca framework in the current project directory.

This command:
  - Creates the .cosca/ directory structure
  - Creates initial configuration
  - Discovers the project environment
  - Sets up the runtime context
  - Applies safety constraints (--no-network, --no-docs, etc.)

Constraints are saved to .cosca/constraints.yaml and enforced by the jail
and runtime on every subsequent execution.`,
		Example: `  cosca init                           # Initialize with all defaults
  cosca init --no-network               # Disable network access
  cosca init --no-docs --no-test        # Code-only, no docs or tests
  cosca init --read-only --max-files 200 # Strict audit mode
  cosca init --force                    # Force re-initialization`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			formatter.Verbose("Starting Cosca initialization...")

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			coscaDir := filepath.Join(dir, ".cosca")

			// Check if already initialized
			if _, err := os.Stat(coscaDir); err == nil {
				if !force {
					return fmt.Errorf("Cosca is already initialized in %s (use --force to re-initialize)", dir)
				}
				// Backup before overwriting: .cosca → .cosca.bak.2026-08-05T15:00:00Z
				backupDir := coscaDir + ".bak." + time.Now().UTC().Format("2006-01-02T150405Z")
				formatter.Warning("Re-initializing Cosca in " + dir)
				formatter.Verbose(fmt.Sprintf("Backup: %s → %s", coscaDir, backupDir))
				if err := os.Rename(coscaDir, backupDir); err != nil {
					return fmt.Errorf("failed to backup existing .cosca: %w", err)
				}
			}

			// Create .cosca/ directory structure
			dirs := []string{
				coscaDir,
				filepath.Join(coscaDir, "config"),
				filepath.Join(coscaDir, "memory"),
				filepath.Join(coscaDir, "memory", "short"),
				filepath.Join(coscaDir, "memory", "long"),
				filepath.Join(coscaDir, "memory", "project"),
				filepath.Join(coscaDir, "memory", "architecture"),
				filepath.Join(coscaDir, "memory", "decision"),
				filepath.Join(coscaDir, "cache"),
				filepath.Join(coscaDir, "plugins"),
				filepath.Join(coscaDir, "runtime"),
				filepath.Join(coscaDir, "index"),
				filepath.Join(coscaDir, "vectors"),
				filepath.Join(coscaDir, "graph"),
				filepath.Join(coscaDir, "sessions"),
				filepath.Join(coscaDir, "audit"),
			}

			spinner := formatter.Spinner("Creating .cosca/ directory structure")
			spinner.Start()

			// M6b: create the .cosca tree with owner-only permissions (0700).
			// The directory holds the knowledge base, memories, cache and
			// audit data — it must not be readable by other local users.
			if err := createPrivateDirs(dirs); err != nil {
				spinner.Fail(fmt.Sprintf("Failed to create directory: %s", err))
				return fmt.Errorf("failed to create .cosca directory structure: %w", err)
			}

			spinner.Stop("Directory structure created")

			// Materialize .cosca/fallback/ from the embedded framework so the
			// knowledge compiler, ORC and cosca-indexer read real content off
			// disk even on fresh installs.
			spinnerFB := formatter.Spinner("Materializing fallback framework")
			spinnerFB.Start()
			if err := materializeFallback(dir); err != nil {
				spinnerFB.Fail(fmt.Sprintf("Fallback materialization: %s", err))
				return fmt.Errorf("fallback materialization: %w", err)
			}
			spinnerFB.Stop("Fallback framework materialized")

			// Sync framework files so the Kernel can boot in any project.
			// Extracts core identity files from the embedded binary into
			// .cosca/framework/ — KERNEL.md, cognitive-state, constitution.
			// ONLY for the Cosca self-project. Third-party projects use
			// self-contained editor prompts (no framework extraction).
			if embedcosca.IsSelfProject(dir) {
				spinnerFW := formatter.Spinner("Syncing framework files")
				spinnerFW.Start()
				if err := syncEmbedFramework(coscaDir); err != nil {
					spinnerFW.Fail(fmt.Sprintf("Framework sync: %s", err))
					return fmt.Errorf("framework sync: %w", err)
				}
				spinnerFW.Stop("Framework synced")
			}

			// Create default configuration
			spinner2 := formatter.Spinner("Creating configuration")
			spinner2.Start()

			cfg := config.DefaultConfig()
			cfgPath := filepath.Join(coscaDir, "config.yaml")
			if err := cfg.Save(cfgPath); err != nil {
				spinner2.Fail("Failed to create configuration")
				return fmt.Errorf("failed to write config: %w", err)
			}

			spinner2.Stop("Configuration created")

			// Create constraints (safety rails)
			constraints := config.DefaultConstraints()
			if noDocs {
				constraints.Docs = false
			}
			if noNetwork {
				constraints.Network = false
			}
			if noTest {
				constraints.Test = false
			}
			if noBuild {
				constraints.Build = false
			}
			if readOnly {
				constraints.ReadOnly = true
			}
			if maxFiles > 0 {
				constraints.MaxFiles = maxFiles
			}
			if maxTimeStr != "" {
				constraints.MaxTimeStr = maxTimeStr
			}

			if err := config.SaveConstraints(dir, constraints); err != nil {
				spinner2.Fail("Failed to save constraints")
				return fmt.Errorf("failed to write constraints: %w", err)
			}

			// Devolve posse dos arquivos ao usuario original (sudo cria como root)
			if sudoUID := os.Getenv("SUDO_UID"); sudoUID != "" {
				if sudoGID := os.Getenv("SUDO_GID"); sudoGID != "" {
					uid, _ := strconv.Atoi(sudoUID)
					gid, _ := strconv.Atoi(sudoGID)
					chownRecursive(coscaDir, uid, gid)
				}
			}

			// Auto-detect and configure the editor so the Kernel is
			// recognized immediately — no need to run `cosca install`
			// as a separate step.
			// Priority: force OpenCode first (o editor do Don), fall
			// back to auto-detect for other editors.
			editorName := "none"
			editorCfg := editors.DefaultEditorConfig(dir)
			mgr := editors.NewManager(editorCfg)
			if mgr != nil {
				// Try OpenCode first — always configure the Don's editor.
				if err := mgr.SetupForce("opencode"); err == nil {
					editorName = "opencode"
					formatter.Verbose("Editor configured: opencode")
				} else if info, err := mgr.AutoDetectAndSetup(); err == nil && info != nil {
					editorName = info.Name
					formatter.Verbose(fmt.Sprintf("Editor configured: %s", editorName))
				} else {
					formatter.Verbose(fmt.Sprintf("Editor auto-setup skipped: %v", err))
				}
			}

			// Discover environment
			ctx := context.Background()
			formatter.Verbose("Discovering project environment...")
			envInfo := EnvInfo{}

			disc := discovery.NewEngine(discovery.WithWorkDir(dir))
			if disc != nil {
				projInfo, err := disc.DiscoverProject(ctx)
				if err == nil && projInfo != nil {
					envInfo.Framework = projInfo.Framework
					envInfo.Language = projInfo.Language
				}
				rtInfo, err := disc.DiscoverRuntime(ctx)
				if err == nil && rtInfo != nil {
					envInfo.Runtime = rtInfo.Mode
				}
			}

			result := InitResult{
				ProjectDir:  dir,
				CoscaDir:    coscaDir,
				ConfigFile:  cfgPath,
				Constraints: constraints,
				FoundEnv:    envInfo,
				NextSteps:   []string{"Run 'cosca sync' to index your project", "Run 'cosca status' to see system status"},
				Directories: dirs,
			}

			if useJSON {
				return printJSON(cmd, result)
			}

			formatter.Println("")
			formatter.Header("Initialization Complete")
			formatter.Success(fmt.Sprintf("Cosca initialized in %s", dir))
			formatter.Println("")
			formatter.KeyValue("Project", dir)
			formatter.KeyValue("Config", cfgPath)
			formatter.KeyValue("Editor", editorName)
			formatter.Println("")
			formatter.Header("Next Steps")
			for _, step := range result.NextSteps {
				formatter.Bullet(step)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force re-initialization")

	// Constraint flags
	cmd.Flags().BoolVar(&noDocs, "no-docs", false, "Disable documentation generation agents")
	cmd.Flags().BoolVar(&noNetwork, "no-network", false, "Disable network access (blocks API calls)")
	cmd.Flags().BoolVar(&noTest, "no-test", false, "Disable test agents")
	cmd.Flags().BoolVar(&noBuild, "no-build", false, "Disable build/exec commands")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Workspace read-only (no file modifications)")
	cmd.Flags().IntVar(&maxFiles, "max-files", 0, "Maximum files to create (0 = unlimited)")
	cmd.Flags().StringVar(&maxTimeStr, "max-time", "", "Maximum time per operation (e.g. 30m, 1h)")

	return cmd
}

// createPrivateDirs creates each directory with owner-only permissions
// (0700) and enforces them with an explicit chmod. MkdirAll alone leaves
// pre-existing directories untouched, so the chmod also hardens directories
// created by older versions of Cosca. Used for the .cosca tree, which holds
// the knowledge base, memories, cache and audit data (M6b).
func createPrivateDirs(dirs []string) error {
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
		if err := os.Chmod(d, 0700); err != nil {
			return fmt.Errorf("restrict directory permissions %s: %w", d, err)
		}
	}
	return nil
}

// chownRecursive walks a directory tree and changes ownership of all files.
func chownRecursive(root string, uid, gid int) {
	_ = filepath.Walk(root, func(path string, _ os.FileInfo, _ error) error {
		_ = os.Chown(path, uid, gid)
		return nil
	})
}

// syncEmbedFramework extrai os arquivos essenciais do framework do binário
// para .cosca/framework/. Isso permite que o Kernel carregue sua identidade
// em qualquer projeto inicializado com cosca init.
func syncEmbedFramework(coscaDir string) error {
	frameworkDir := filepath.Join(coscaDir, "framework")
	if err := os.MkdirAll(frameworkDir, 0o700); err != nil {
		return fmt.Errorf("mkdir framework: %w", err)
	}

	// Core files the Kernel needs to boot.
	coreFiles := []string{
		"KERNEL.md",
		"CONSTITUTION.md",
		"AGENT_DNA.md",
		"CONVENTIONS.md",
		"QUALITY_GATES.md",
		"MEMORY_MODEL.md",
		"shared/AUTO_EVOLUTION_PROTOCOL.md",
		"shared/KNOWLEDGE_PROTOCOL.md",
		"shared/PROJECT_CONTEXT.md",
		"knowledge/cognitive/cognitive-state.md",
		"memory/LEARNING_PROTOCOL.md",
	}

	for _, relPath := range coreFiles {
		data, err := embed.ReadFile(relPath)
		if err != nil {
			return fmt.Errorf("read embed %q: %w", relPath, err)
		}
		target := filepath.Join(frameworkDir, relPath)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return fmt.Errorf("mkdir %q: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			return fmt.Errorf("write %q: %w", target, err)
		}
	}

	return nil
}

// dirExists returns true if the path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
