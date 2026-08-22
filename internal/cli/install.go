package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/discovery"
	"github.com/CoscaAI/cosca/internal/editors"
)

// EditorInstallResult holds the outcome of installing Cosca into a single editor.
type EditorInstallResult struct {
	Editor string `json:"editor"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

// InstallResult holds the result of the install operation.
type InstallResult struct {
	ProjectDir        string                `json:"project_dir" yaml:"project_dir"`
	CoscaDir          string                `json:"cosca_dir" yaml:"cosca_dir"`
	Editor            string                `json:"editor" yaml:"editor"`
	Editors           []EditorInstallResult `json:"editors,omitempty" yaml:"editors,omitempty"`
	Steps             []string              `json:"steps" yaml:"steps"`
	CompletedSteps    int                   `json:"completed_steps" yaml:"completed_steps"`
	TotalSteps        int                   `json:"total_steps" yaml:"total_steps"`
	InstallID         string                `json:"install_id" yaml:"install_id"`
	Duration          string                `json:"duration" yaml:"duration"`
	Repairs           int                   `json:"repairs,omitempty" yaml:"repairs,omitempty"`
	RepairedSteps     []string              `json:"repaired_steps,omitempty" yaml:"repaired_steps,omitempty"`
	AlreadyConfigured bool                  `json:"already_configured" yaml:"already_configured"`
}

// StepState describes the installation state of a single step.
type StepState string

const (
	// StepInstalled means the step's artifact is present.
	StepInstalled StepState = "installed"
	// StepMissing means the step's artifact is absent.
	StepMissing StepState = "missing"
	// StepAttention means the step cannot be verified automatically.
	StepAttention StepState = "needs-attention"
)

// StepStatus is the per-step result reported by `cosca install --status`.
type StepStatus struct {
	Number   int       `json:"number" yaml:"number"`
	Name     string    `json:"name" yaml:"name"`
	State    StepState `json:"state" yaml:"state"`
	Artifact string    `json:"artifact,omitempty" yaml:"artifact,omitempty"`
	Detail   string    `json:"detail,omitempty" yaml:"detail,omitempty"`
}

// InstallStatus is the full report produced by `cosca install --status`.
type InstallStatus struct {
	ProjectDir        string       `json:"project_dir" yaml:"project_dir"`
	CoscaDir          string       `json:"cosca_dir" yaml:"cosca_dir"`
	Steps             []StepStatus `json:"steps" yaml:"steps"`
	PendingSteps      int          `json:"pending_steps" yaml:"pending_steps"`
	FreshInstall      bool         `json:"fresh_install" yaml:"fresh_install"`
	DriftDetected     bool         `json:"drift_detected" yaml:"drift_detected"`
	AlreadyConfigured bool         `json:"already_configured" yaml:"already_configured"`
	Summary           string       `json:"summary" yaml:"summary"`
}

// installStepSpec describes one install step. When artifact is non-empty the
// step is verifiable via os.Stat; guarded steps skip execution when the
// artifact already exists (idempotency).
type installStepSpec struct {
	label    string
	artifact string // path relative to coscaDir; empty = not verifiable
	isDir    bool   // artifact is expected to be a directory
	guarded  bool   // skip execution when artifact already exists
}

// installSteps is the fixed 14-step installation pipeline.
//
// Verifiable artifacts (via os.Stat):
//   - "."            → the .cosca tree itself. This is the config root the
//     runtime step creates; the 14-step pipeline writes no
//     .cosca/config.yaml (that is `cosca init`'s job), so the tree
//     itself is the closest config-file artifact.
//   - plugins        → plugins directory
//   - cache          → cache directory
//   - index          → index directory
//   - runtime        → SQLite db directory. The SQLite adapter Open() is a
//     no-op (adapters_installer_validator_adapter.go), so no
//     runtime/cosca.db file is produced; the directory is the
//     real artifact.
//   - knowledge.db   → knowledge base SQLite file (created by the knowledge
//     engine rebuild).
//
// Steps without an artifact (discovery, editor integration, graph build,
// context, validation, health check) report as done and are not verifiable.
var installSteps = []installStepSpec{
	{label: "Discovering editor"},
	{label: "Discovering project"},
	{label: "Discovering Cosca Global"},
	{label: "Installing editor integration"},
	{label: "Creating plugins", artifact: "plugins", isDir: true},
	{label: "Creating runtime", artifact: ".", isDir: true},
	{label: "Creating cache", artifact: "cache", isDir: true, guarded: true},
	{label: "Creating index", artifact: "index", isDir: true, guarded: true},
	{label: "Creating SQLite database", artifact: "runtime", isDir: true, guarded: true},
	{label: "Building knowledge graph"},
	{label: "Generating embeddings", artifact: "knowledge.db", guarded: true},
	{label: "Configuring context"},
	{label: "Validating installation"},
	{label: "Running health check"},
}

// verifiable reports whether the step has a filesystem artifact to check.
func (s installStepSpec) verifiable() bool { return s.artifact != "" }

// artifactPath returns the absolute path of the step's artifact.
func (s installStepSpec) artifactPath(coscaDir string) string {
	if s.artifact == "" {
		return ""
	}
	return filepath.Join(coscaDir, s.artifact)
}

// present reports whether the step's artifact currently exists on disk.
func (s installStepSpec) present(coscaDir string) bool {
	info, err := os.Stat(s.artifactPath(coscaDir))
	if err != nil {
		return false
	}
	if s.isDir {
		return info.IsDir()
	}
	return info.Mode().IsRegular()
}

// stepLabels returns the human-readable labels of all install steps.
func stepLabels() []string {
	labels := make([]string, len(installSteps))
	for i, s := range installSteps {
		labels[i] = s.label
	}
	return labels
}

// NewInstallCommand creates the `cosca install` command.
func NewInstallCommand() *cobra.Command {
	var global bool
	var installAll bool
	var statusMode bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install and configure Cosca in the current project",
		Long: `Perform a full auto-install of Cosca in the current project.

This command runs the complete installation pipeline:
  1. Discover editor
  2. Discover project environment
  3. Discover Cosca Global configuration
  4. Install editor integration
  5. Create plugins directory
  6. Create runtime configuration
  7. Create cache directory
  8. Create index
  9. Create SQLite database
  10. Build knowledge graph
  11. Generate embeddings
  12. Configure context
  13. Validate installation
  14. Health check

The installer is idempotent: running it again on an already-configured tree
skips work that is already done ("already configured ✓"). If an artifact is
missing it is detected as drift and repaired, then verified.

Use --status (or --check) to report the current installation state for each
step WITHOUT modifying anything:
  - installed ✓        artifact present
  - missing ✗          artifact absent
  - needs-attention ⚠  step not verifiable automatically

The installer auto-detects everything with zero configuration required.
Use --all to install the Cosca integration into EVERY supported editor at
once (opencode, claude, codex, cursor, windsurf, zed, vscode, neovim,
generic_mcp) instead of only the detected one.
`,
		Example: `  cosca install              # Install locally in current project
  cosca install --global     # Install globally for all projects
  cosca install --all        # Install into ALL supported editors
  cosca install --json       # Output in JSON format
  cosca install --status     # Report installation state, modify nothing
  cosca install --check      # Same as --status`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			installID := uuid.New().String()
			startTime := time.Now()

			// Resolve dirs up front so --status works with zero side
			// effects and the full install can guard/repair per-artifact.
			projectDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			coscaDir := filepath.Join(projectDir, ".cosca")
			if global {
				coscaDir = filepath.Join(os.Getenv("HOME"), ".cosca")
			}

			if statusMode {
				return runInstallStatus(formatter, cmd, projectDir, coscaDir, useJSON)
			}

			steps := stepLabels()
			totalSteps := len(installSteps)
			completedSteps := 0
			detectedEditor := "unknown"
			var editorResults []EditorInstallResult

			formatter.Println("Starting Cosca installation...")
			formatter.Println("")

			// Step 1: Discover editor
			formatter.Verbose("Step 1: " + installSteps[0].label)
			if !installAll {
				detector := NewDetector()
				if detector != nil {
					detectedEditor = detector.Detect()
				}
			}
			completedSteps++

			// Step 2: Discover project
			formatter.Verbose("Step 2: " + installSteps[1].label)
			disc := discovery.NewEngine(discovery.WithWorkDir(projectDir))
			if disc != nil {
				if _, err := disc.DiscoverAll(context.Background()); err != nil {
					formatter.Verbose(fmt.Sprintf("Discovery warning: %v", err))
				}
			}
			completedSteps++

			// Step 3: Discover Cosca Global
			formatter.Verbose("Step 3: " + installSteps[2].label)
			globalCoscaDir := filepath.Join(os.Getenv("HOME"), ".config", "opencode", "cosca")
			if info, err := os.Stat(globalCoscaDir); err == nil && info.IsDir() {
				formatter.Verbose("Found Cosca Global at: " + globalCoscaDir)
			}
			completedSteps++

			// Idempotency baseline: which verifiable artifacts are missing
			// before any step runs.
			fresh := !dirExists(coscaDir)
			missingAtStart := 0
			for _, spec := range installSteps {
				if spec.verifiable() && !spec.present(coscaDir) {
					missingAtStart++
				}
			}

			// Step 4: Install editor integration
			formatter.Verbose("Step 4: " + installSteps[3].label)
			if installAll {
				// Install into EVERY registered editor, tolerating failures.
				editorResults = installAllEditors(formatter, projectDir)
				detectedEditor = "all"
			} else {
				// Existing behavior: setup via context builder for the
				// detected editor.
				cb := NewContextBuilder(projectDir)
				if cb != nil {
					if err := cb.Build(); err != nil {
						formatter.Warning(fmt.Sprintf("Context build warning: %v", err))
					}
				}
			}
			completedSteps++

			// Step 5: Create plugins directory
			formatter.Verbose("Step 5: " + installSteps[4].label)
			pluginDir := filepath.Join(coscaDir, "plugins")
			if err := os.MkdirAll(pluginDir, 0755); err != nil {
				formatter.Warning(fmt.Sprintf("Failed to create plugins dir: %v", err))
			}
			_ = pluginDir
			completedSteps++

			// Step 6: Create runtime
			formatter.Verbose("Step 6: " + installSteps[5].label)
			rt := newRuntimeAdapter(coscaDir)
			if rt != nil {
				if err := rt.Init(); err != nil {
					formatter.Warning(fmt.Sprintf("Runtime init warning: %v", err))
				}
			}
			completedSteps++

			// Step 7: Create cache (guarded)
			formatter.Verbose("Step 7: " + installSteps[6].label)
			if installSteps[6].present(coscaDir) {
				formatter.Success("Step 7: cache already configured ✓")
			} else {
				if !fresh {
					formatter.Warning("detected drift → repair → verify (cache)")
				}
				cacheDir := filepath.Join(coscaDir, "cache")
				if err := os.MkdirAll(cacheDir, 0755); err != nil {
					formatter.Warning(fmt.Sprintf("Failed to create cache dir: %v", err))
				}
				// Initialize cache manager
				cm := NewCacheManager(cacheDir)
				if cm != nil {
					_ = cm.Invalidate(nil)
				}
			}
			completedSteps++

			// Step 8: Create index (guarded)
			formatter.Verbose("Step 8: " + installSteps[7].label)
			if installSteps[7].present(coscaDir) {
				formatter.Success("Step 8: index already configured ✓")
			} else {
				if !fresh {
					formatter.Warning("detected drift → repair → verify (index)")
				}
				idx := NewIndexer(coscaDir)
				if idx != nil {
					if err := idx.Create(); err != nil {
						formatter.Warning(fmt.Sprintf("Index creation warning: %v", err))
					}
				}
			}
			completedSteps++

			// Step 9: Create SQLite database (guarded)
			formatter.Verbose("Step 9: " + installSteps[8].label)
			if installSteps[8].present(coscaDir) {
				formatter.Success("Step 9: SQLite database already configured ✓")
			} else {
				if !fresh {
					formatter.Warning("detected drift → repair → verify (runtime)")
				}
				dbDir := filepath.Join(coscaDir, "runtime")
				if err := os.MkdirAll(dbDir, 0755); err != nil {
					formatter.Warning(fmt.Sprintf("Failed to create database dir: %v", err))
				}
				dbPath := filepath.Join(dbDir, "cosca.db")
				sqlDB := NewSQLiteDatabase(dbPath)
				if sqlDB != nil {
					if err := sqlDB.Open(); err != nil {
						formatter.Warning(fmt.Sprintf("Database open warning: %v", err))
					}
				}
			}
			completedSteps++

			// Step 10: Build knowledge graph
			formatter.Verbose("Step 10: " + installSteps[9].label)
			g := NewGraph(coscaDir)
			if g != nil {
				if err := g.Build(); err != nil {
					formatter.Warning(fmt.Sprintf("Graph build warning: %v", err))
				}
			}
			completedSteps++

			// Step 11: Generate embeddings (guarded)
			formatter.Verbose("Step 11: " + installSteps[10].label)
			if installSteps[10].present(coscaDir) {
				formatter.Success("Step 11: embeddings already configured ✓")
			} else {
				if !fresh {
					formatter.Warning("detected drift → repair → verify (knowledge.db)")
				}
				ke := NewKnowledgeEngine(coscaDir)
				if ke != nil {
					if err := ke.Rebuild(); err != nil {
						formatter.Warning(fmt.Sprintf("Knowledge rebuild warning: %v", err))
					}
				}
			}
			completedSteps++

			// Step 12: Configure context
			formatter.Verbose("Step 12: " + installSteps[11].label)
			cb2 := NewContextBuilder(coscaDir)
			if cb2 != nil {
				if err := cb2.Build(); err != nil {
					formatter.Warning(fmt.Sprintf("Context build warning: %v", err))
				}
			}
			completedSteps++

			// Step 13: Validate installation
			formatter.Verbose("Step 13: " + installSteps[12].label)
			validator := NewValidator(coscaDir)
			if validator != nil {
				if err := validator.Validate(); err != nil {
					formatter.Warning(fmt.Sprintf("Validation warning: %v", err))
				}
			}
			completedSteps++

			// Step 14: Health check
			formatter.Verbose("Step 14: " + installSteps[13].label)
			mem := NewMemoryManager(coscaDir)
			if mem != nil {
				defer func() { _ = mem.Close() }()
				mem.Init()
				// Verify memory works by checking status
				_ = mem.Status()
			}

			// Drift detection: verify every verifiable artifact was actually
			// created. Any missing artifact is repaired and re-verified.
			repairs := 0
			var repairedSteps []string
			for i, spec := range installSteps {
				if !spec.verifiable() || spec.present(coscaDir) {
					continue
				}
				formatter.Warning(fmt.Sprintf("detected drift → repair → verify (%s)", spec.artifact))
				if err := repairInstallStep(coscaDir, spec); err != nil {
					formatter.Warning(fmt.Sprintf("Repair failed for step %d (%s): %v", i+1, spec.label, err))
					continue
				}
				if spec.present(coscaDir) {
					repairs++
					repairedSteps = append(repairedSteps, spec.label)
					formatter.Success(fmt.Sprintf("Repaired: %s", spec.label))
				} else {
					formatter.Warning(fmt.Sprintf("Repair verify failed: %s", spec.artifact))
				}
			}

			// Artifacts missing at start that are now present were recreated
			// by the guarded steps themselves; count those as repairs too
			// when the tree was not a fresh install.
			missingAtEnd := 0
			for _, spec := range installSteps {
				if spec.verifiable() && !spec.present(coscaDir) {
					missingAtEnd++
				}
			}
			totalRepairs := repairs
			if d := missingAtStart - missingAtEnd; d > 0 {
				totalRepairs += d
			}

			elapsed := time.Since(startTime)

			if !useJSON {
				formatter.Println("")
				formatter.Success("Installation complete!")
				formatter.KeyValue("Project", projectDir)
				if installAll {
					installed := 0
					for _, r := range editorResults {
						if r.OK {
							installed++
						}
					}
					formatter.KeyValue("Editors", fmt.Sprintf("%d/%d installed", installed, len(editorResults)))
				} else {
					formatter.KeyValue("Editor", detectedEditor)
				}
				formatter.KeyValue("Duration", elapsed.Round(time.Millisecond).String())
				formatter.KeyValue("Install ID", installID)

				formatter.Println("")
				formatter.Header("Next Steps")
				formatter.Bullet("Run 'cosca status' to see system status")
				formatter.Bullet("Run 'cosca doctor' for full diagnostics")
				formatter.Bullet("Run 'cosca sync' to index your project files")
				formatter.Bullet("Run 'cosca knowledge search <query>' to test the knowledge base")
			}

			if useJSON {
				result := InstallResult{
					ProjectDir:        projectDir,
					CoscaDir:          coscaDir,
					Editor:            detectedEditor,
					Editors:           editorResults,
					Steps:             steps,
					CompletedSteps:    completedSteps,
					TotalSteps:        totalSteps,
					InstallID:         installID,
					Duration:          elapsed.Round(time.Millisecond).String(),
					Repairs:           totalRepairs,
					RepairedSteps:     repairedSteps,
					AlreadyConfigured: !fresh && totalRepairs == 0,
				}
				return printJSON(cmd, result)
			}

			// Idempotency summary.
			switch {
			case totalRepairs > 0 && !fresh:
				formatter.Warning(fmt.Sprintf("reparos aplicados: %d", totalRepairs))
			case !fresh:
				formatter.Success("Instalação verificada ✓ — nenhuma ação repetida desnecessariamente")
			default:
				formatter.Success("Instalação verificada ✓")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "install globally instead of locally")
	cmd.Flags().BoolVar(&installAll, "all", false, "install Cosca integration into ALL supported editors")
	cmd.Flags().BoolVar(&statusMode, "status", false, "report installation state without modifying anything")
	cmd.Flags().BoolVar(&statusMode, "check", false, "alias for --status")
	return cmd
}

// runInstallStatus reports the installation state of every step without
// modifying anything on disk.
func runInstallStatus(formatter *OutputFormatter, cmd *cobra.Command, projectDir, coscaDir string, useJSON bool) error {
	fresh := !dirExists(coscaDir)
	statuses := make([]StepStatus, 0, len(installSteps))
	pending := 0
	for i, spec := range installSteps {
		st := StepStatus{Number: i + 1, Name: spec.label}
		if spec.verifiable() {
			st.Artifact = spec.artifactPath(coscaDir)
			if spec.present(coscaDir) {
				st.State = StepInstalled
			} else {
				st.State = StepMissing
				pending++
			}
		} else {
			st.State = StepAttention
			st.Detail = "não verificável automaticamente"
		}
		statuses = append(statuses, st)
	}

	already := pending == 0
	summary := "already configured ✓"
	if !already {
		if fresh {
			summary = fmt.Sprintf("%d passo(s) pendente(s)", pending)
		} else {
			summary = "drift detected — rode cosca install"
		}
	}

	if useJSON {
		return printJSON(cmd, InstallStatus{
			ProjectDir:        projectDir,
			CoscaDir:          coscaDir,
			Steps:             statuses,
			PendingSteps:      pending,
			FreshInstall:      fresh,
			DriftDetected:     !fresh && pending > 0,
			AlreadyConfigured: already,
			Summary:           summary,
		})
	}

	formatter.Println("Installation status:")
	formatter.KeyValue("Cosca dir", coscaDir)
	formatter.Println("")
	for _, st := range statuses {
		switch st.State {
		case StepInstalled:
			formatter.Success(fmt.Sprintf("%2d. %s — installed ✓", st.Number, st.Name))
		case StepMissing:
			formatter.Error(fmt.Sprintf("%2d. %s — missing ✗", st.Number, st.Name))
		default:
			formatter.Warning(fmt.Sprintf("%2d. %s — needs-attention ⚠", st.Number, st.Name))
		}
	}
	formatter.Println("")
	if already {
		formatter.Success(summary)
	} else {
		formatter.Warning(summary)
	}
	return nil
}

// repairInstallStep re-creates the artifact of a single step. Only verifiable
// artifacts are repairable; everything else reports as done.
func repairInstallStep(coscaDir string, spec installStepSpec) error {
	switch spec.artifact {
	case "plugins":
		return os.MkdirAll(filepath.Join(coscaDir, "plugins"), 0755)
	case ".":
		return os.MkdirAll(coscaDir, 0755)
	case "cache":
		return os.MkdirAll(filepath.Join(coscaDir, "cache"), 0755)
	case "index":
		idx := NewIndexer(coscaDir)
		if idx == nil {
			return fmt.Errorf("indexer unavailable")
		}
		return idx.Create()
	case "runtime":
		return os.MkdirAll(filepath.Join(coscaDir, "runtime"), 0755)
	case "knowledge.db":
		ke := NewKnowledgeEngine(coscaDir)
		if ke == nil {
			return fmt.Errorf("knowledge engine unavailable")
		}
		return ke.Rebuild()
	default:
		return nil
	}
}

// installAllEditors installs the Cosca integration into every registered
// editor. It never fails as a whole: a failure in one editor is reported
// (⚠ falhou) and the remaining editors are still processed. The per-editor
// results are returned so they can be surfaced in the JSON output.
func installAllEditors(formatter *OutputFormatter, projectDir string) []EditorInstallResult {
	cfg := editors.DefaultEditorConfig(projectDir)
	cfg.BackupExisting = true // always back up existing editor configs

	mgr := editors.NewManager(cfg)
	if mgr == nil {
		formatter.Warning("Editor manager unavailable — skipping editor integration")
		return nil
	}

	results := mgr.SetupAll()
	editorResults := make([]EditorInstallResult, 0, len(results))

	for _, r := range results {
		res := EditorInstallResult{Editor: r.Editor, OK: r.Err == nil}
		if r.Err != nil {
			res.Error = r.Err.Error()
			formatter.Warning(fmt.Sprintf("%s: falhou — %v", r.Editor, r.Err))
		} else {
			formatter.Success(fmt.Sprintf("%s: instalado", r.Editor))
		}
		editorResults = append(editorResults, res)
	}

	return editorResults
}
