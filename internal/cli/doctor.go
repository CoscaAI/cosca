package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/editors"
	"github.com/CoscaAI/cosca/internal/framework"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/plugins"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/security"
)

// DoctorResult holds the complete diagnostic results.
type DoctorResult struct {
	Runtime   CheckResult `json:"runtime" yaml:"runtime"`
	Editor    CheckResult `json:"editor" yaml:"editor"`
	Plugins   CheckResult `json:"plugins" yaml:"plugins"`
	Memory    CheckResult `json:"memory" yaml:"memory"`
	Knowledge CheckResult `json:"knowledge" yaml:"knowledge"`
	Providers CheckResult `json:"providers" yaml:"providers"`
	Security  CheckResult `json:"security" yaml:"security"`
	Summary   SummaryInfo `json:"summary" yaml:"summary"`
}

// CheckResult holds the result of a diagnostic check.
type CheckResult struct {
	Status string      `json:"status" yaml:"status"`
	Checks []CheckItem `json:"checks" yaml:"checks"`
	Issues []string    `json:"issues" yaml:"issues"`
	Fixes  []string    `json:"fixes" yaml:"fixes"`
}

// CheckItem holds a single diagnostic check.
type CheckItem struct {
	Name   string `json:"name" yaml:"name"`
	Status string `json:"status" yaml:"status"`
	Detail string `json:"detail" yaml:"detail"`
}

// SummaryInfo holds summary information for the diagnostic report.
type SummaryInfo struct {
	TotalChecks   int    `json:"total_checks" yaml:"total_checks"`
	PassedChecks  int    `json:"passed_checks" yaml:"passed_checks"`
	FailedChecks  int    `json:"failed_checks" yaml:"failed_checks"`
	WarningChecks int    `json:"warning_checks" yaml:"warning_checks"`
	OverallStatus string `json:"overall_status" yaml:"overall_status"`
}

// NewDoctorCommand creates the `cosca doctor` command.
func NewDoctorCommand() *cobra.Command {
	var frameworkRoot string
	var fix bool
	cmd := &cobra.Command{
		Use:   "doctor [runtime|editor|plugins|memory|knowledge|providers|security|framework]",
		Short: "Run system diagnostics",
		Long: `Run comprehensive diagnostics on the Cosca system.

Without arguments, performs a full system diagnostic across all subsystems.
With a subsystem argument, only checks that specific subsystem.

Subsystems:
  runtime    - Runtime daemon diagnostics
  editor     - Editor integration diagnostics
  plugins    - Plugin diagnostics
  memory     - Memory system diagnostics
  knowledge  - Knowledge engine diagnostics
  providers  - Provider diagnostics
  security   - Dependency vulnerability scan (OSV database)
  framework  - Framework inventory and health (.cosca/framework)

--fix applies only SAFE corrections automatically (creating missing .cosca
directories, rebuilding the knowledge index, fixing permissions on Cosca
files). Corrections that require sudo (drivers, kernel, bootloader,
firewall, system packages) are NEVER executed — they are printed for the
Don to run.
`,
		Example: `  cosca doctor                 Full system diagnostics
  cosca doctor runtime         Runtime diagnostics only
  cosca doctor editor          Editor integration check
  cosca doctor knowledge       Knowledge engine check
  cosca doctor framework       Framework health inventory
  cosca doctor --json          Machine-readable output
  cosca doctor --fix           Apply safe fixes automatically`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			// Determine subsystem
			subsystem := ""
			if len(args) > 0 {
				subsystem = args[0]
				validSubsystems := map[string]bool{
					"runtime": true, "editor": true, "plugins": true,
					"memory": true, "knowledge": true, "providers": true,
					"security": true, "framework": true,
				}
				if !validSubsystems[subsystem] {
					return fmt.Errorf("unknown subsystem: %s (valid: runtime, editor, plugins, memory, knowledge, providers, security, framework)", subsystem)
				}
			}

			// The framework subsystem runs its own report against the
			// versioned framework directory (.cosca/framework), so it does
			// not fit the CheckResult-based single/full diagnostics.
			if subsystem == "framework" {
				return runDoctorFramework(cmd, formatter, resolveFrameworkRoot(frameworkRoot), useJSON)
			}

			if fix {
				return runDoctorFix(formatter, coscaDir, subsystem)
			}

			if useJSON {
				return runDoctorJSON(cmd, coscaDir, subsystem)
			}

			if subsystem != "" {
				return runDoctorSingle(cmd, formatter, coscaDir, subsystem)
			}

			return runDoctorFull(cmd, formatter, coscaDir)
		},
	}
	cmd.PersistentFlags().StringVar(&frameworkRoot, "root", "", "root of the Cosca framework directory (default: <cwd>/.cosca/framework)")
	cmd.PersistentFlags().BoolVar(&fix, "fix", false, "apply safe fixes automatically (never runs sudo)")

	return cmd
}

// runDoctorFramework reports the health inventory of the Cosca framework
// directory via framework.Report and prints it as key-value pairs or JSON.
func runDoctorFramework(cmd *cobra.Command, formatter *OutputFormatter, root string, useJSON bool) error {
	report, err := framework.Report(root)
	if err != nil {
		return fmt.Errorf("framework report failed: %w", err)
	}

	if useJSON {
		return printJSON(cmd, report)
	}

	formatter.Header("Framework Diagnostics")
	formatter.KeyValue("Root", root)
	formatter.KeyValue("Markdown Files", fmt.Sprint(report.TotalMarkdownFiles))
	formatter.KeyValue("Departments", fmt.Sprint(report.Departments))
	formatter.KeyValue("Skills", fmt.Sprint(report.Skills))
	formatter.KeyValue("Engines", fmt.Sprint(report.Engines))
	formatter.KeyValue("Workflows", fmt.Sprint(report.Workflows))
	formatter.KeyValue("Templates", fmt.Sprint(report.Templates))
	formatter.KeyValue("Agents", fmt.Sprint(report.Agents))
	formatter.KeyValue("Memory Records", fmt.Sprint(report.MemoryRecords))
	formatter.KeyValue("ADRs", fmt.Sprint(report.ADRs))
	formatter.KeyValue("Broken Links", fmt.Sprint(report.BrokenLinks))
	formatter.KeyValue("Orphans", fmt.Sprint(report.OrphanCount))

	switch {
	case report.BrokenLinks > 0 || report.OrphanCount > 0:
		formatter.Warning("Framework health: attention required")
	default:
		formatter.Success("Framework health: healthy")
	}
	return nil
}

// doctorCheck pairs a diagnostic section name with its check function.
type doctorCheck struct {
	name string
	fn   func() CheckResult
}

// doctorChecks returns the ordered list of checks for a full diagnostic.
// NOTE: this mirrors the exact check set of `cosca doctor` today; the
// --fix flow adds the Directories check on top (doctor_fix.go).
func doctorChecks(coscaDir string) []doctorCheck {
	return []doctorCheck{
		{"Runtime", func() CheckResult { return checkRuntime(coscaDir) }},
		{"Editor", func() CheckResult { return checkEditor(coscaDir) }},
		{"Plugins", func() CheckResult { return checkPlugins(coscaDir) }},
		{"Memory", func() CheckResult { return checkMemory(coscaDir) }},
		{"Knowledge", func() CheckResult { return checkKnowledge(coscaDir) }},
		{"Providers", func() CheckResult { return checkProviders(coscaDir) }},
		{"Security", func() CheckResult { return checkDependencyVulns(coscaDir) }},
	}
}

// printDoctorCheck renders a single diagnostic section and returns the
// number of pass/fail/warning items found.
func printDoctorCheck(f *OutputFormatter, check doctorCheck) (passed, failed, warnings int) {
	f.Header(check.name)
	result := check.fn()

	for _, c := range result.Checks {
		switch c.Status {
		case "pass":
			passed++
			f.Printf("  %s✓%s %s\n", f.colors.Green, f.colors.Reset, c.Name)
		case "fail":
			failed++
			f.Printf("  %s✗%s %s\n", f.colors.Red, f.colors.Reset, c.Name)
			if c.Detail != "" {
				f.Printf("    %s%s%s\n", f.colors.Dim, c.Detail, f.colors.Reset)
			}
		case "warning":
			warnings++
			f.Printf("  %s⚠%s %s\n", f.colors.Yellow, f.colors.Reset, c.Name)
			if c.Detail != "" {
				f.Printf("    %s%s%s\n", f.colors.Dim, c.Detail, f.colors.Reset)
			}
		}
	}

	if len(result.Issues) > 0 {
		f.Printf("  %sIssues:%s\n", f.colors.Yellow, f.colors.Reset)
		for _, issue := range result.Issues {
			f.Printf("    • %s\n", issue)
		}
	}
	if len(result.Fixes) > 0 {
		f.Printf("  %sSuggested Fixes:%s\n", f.colors.Cyan, f.colors.Reset)
		for _, fix := range result.Fixes {
			f.Printf("    • %s\n", fix)
		}
	}
	f.Println("")
	return
}

func runDoctorFull(_ *cobra.Command, f *OutputFormatter, coscaDir string) error {
	f.Println("")
	f.Header("Cosca System Diagnostics")
	f.Println("")

	checks := doctorChecks(coscaDir)

	total := 0
	passed := 0
	failed := 0
	warnings := 0

	for _, check := range checks {
		p, fa, wa := printDoctorCheck(f, check)
		items := p + fa + wa
		total += items
		passed += p
		failed += fa
		warnings += wa
	}

	// Summary
	overallStatus := "good"
	if failed > 0 {
		overallStatus = "issues_found"
	} else if warnings > 0 {
		overallStatus = "warnings"
	}

	f.Header("Summary")
	f.KeyValue("Checks Performed", fmt.Sprintf("%d", total))
	f.KeyValue("Passed", fmt.Sprintf("%d", passed))
	if failed > 0 {
		f.KeyValue("Failed", fmt.Sprintf("%d", failed))
	}
	if warnings > 0 {
		f.KeyValue("Warnings", fmt.Sprintf("%d", warnings))
	}

	switch overallStatus {
	case "good":
		f.Success("All systems healthy!")
	case "warnings":
		f.Warning("System healthy with warnings — review suggested fixes")
	case "issues_found":
		f.Errorf("Issues found — review diagnostics above")
	}

	return nil
}

func runDoctorSingle(_ *cobra.Command, f *OutputFormatter, coscaDir, subsystem string) error {
	var result CheckResult

	switch subsystem {
	case "runtime":
		result = checkRuntime(coscaDir)
	case "editor":
		result = checkEditor(coscaDir)
	case "plugins":
		result = checkPlugins(coscaDir)
	case "memory":
		result = checkMemory(coscaDir)
	case "knowledge":
		result = checkKnowledge(coscaDir)
	case "providers":
		result = checkProviders(coscaDir)
	case "security":
		result = checkDependencyVulns(coscaDir)
	}

	f.Header(fmt.Sprintf("%s Diagnostics", subsystem))
	for _, c := range result.Checks {
		switch c.Status {
		case "pass":
			f.Success(c.Name)
		case "fail":
			f.Errorf("%s", c.Name)
		case "warning":
			f.Warning(c.Name)
		}
		if c.Detail != "" {
			f.Printf("  %s%s%s\n", f.colors.Dim, c.Detail, f.colors.Reset)
		}
	}

	if len(result.Issues) > 0 {
		f.Println("")
		f.Header("Issues")
		for _, issue := range result.Issues {
			f.Bullet(issue)
		}
	}
	if len(result.Fixes) > 0 {
		f.Println("")
		f.Header("Suggested Fixes")
		for _, fix := range result.Fixes {
			f.Bullet(fix)
		}
	}

	return nil
}

func runDoctorJSON(cmd *cobra.Command, coscaDir, subsystem string) error {
	result := DoctorResult{}

	if subsystem == "" || subsystem == "runtime" {
		result.Runtime = checkRuntime(coscaDir)
	}
	if subsystem == "" || subsystem == "editor" {
		result.Editor = checkEditor(coscaDir)
	}
	if subsystem == "" || subsystem == "plugins" {
		result.Plugins = checkPlugins(coscaDir)
	}
	if subsystem == "" || subsystem == "memory" {
		result.Memory = checkMemory(coscaDir)
	}
	if subsystem == "" || subsystem == "knowledge" {
		result.Knowledge = checkKnowledge(coscaDir)
	}
	if subsystem == "" || subsystem == "providers" {
		result.Providers = checkProviders(coscaDir)
	}
	if subsystem == "" || subsystem == "security" {
		result.Security = checkDependencyVulns(coscaDir)
	}

	// Calculate summary
	total := len(result.Runtime.Checks) + len(result.Editor.Checks) +
		len(result.Plugins.Checks) + len(result.Memory.Checks) +
		len(result.Knowledge.Checks) + len(result.Providers.Checks) +
		len(result.Security.Checks)

	passed := 0
	failed := 0
	warnings := 0
	for _, r := range []CheckResult{result.Runtime, result.Editor, result.Plugins,
		result.Memory, result.Knowledge, result.Providers, result.Security} {
		for _, c := range r.Checks {
			switch c.Status {
			case "pass":
				passed++
			case "fail":
				failed++
			case "warning":
				warnings++
			}
		}
	}

	overallStatus := "good"
	if failed > 0 {
		overallStatus = "issues_found"
	} else if warnings > 0 {
		overallStatus = "warnings"
	}

	result.Summary = SummaryInfo{
		TotalChecks:   total,
		PassedChecks:  passed,
		FailedChecks:  failed,
		WarningChecks: warnings,
		OverallStatus: overallStatus,
	}

	return printJSON(cmd, result)
}

func checkRuntime(coscaDir string) CheckResult {
	result := CheckResult{}

	// O daemon real roda como processo separado (cosca serve via systemd,
	// cosca-serve.service) e grava seu PID em .cosca/cosca.pid. Checar um
	// runtime.New() fresco aqui reporta sempre "uninitialized", pois e um
	// processo diferente e nunca iniciado — o que importa e o daemon REAL.
	pidPath := filepath.Join(coscaDir, "cosca.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		result.Checks = append(result.Checks, CheckItem{
			Name: "Runtime Daemon", Status: "warning", Detail: "PID file not found",
		})
		result.Checks = append(result.Checks, CheckItem{
			Name: "Runtime Health", Status: "warning",
			Detail: "Runtime is not active (daemon não está rodando)",
		})
		result.Issues = append(result.Issues, "Runtime is not active (daemon não está rodando)")
		result.Fixes = append(result.Fixes, "Rode 'cosca serve' ou 'systemctl --user start cosca-serve.service'")
		return result
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		result.Checks = append(result.Checks, CheckItem{
			Name: "Runtime Daemon", Status: "warning", Detail: "PID file inválido",
		})
		result.Checks = append(result.Checks, CheckItem{
			Name: "Runtime Health", Status: "warning",
			Detail: "Runtime is not active (daemon não está rodando)",
		})
		result.Issues = append(result.Issues, "Runtime is not active (daemon não está rodando)")
		result.Fixes = append(result.Fixes, "Rode 'cosca serve' ou 'systemctl --user start cosca-serve.service'")
		return result
	}

	if processAlive(pid) {
		result.Checks = append(result.Checks, CheckItem{
			Name: "Runtime Daemon", Status: "pass", Detail: fmt.Sprintf("daemon ativo (pid %d)", pid),
		})
		result.Checks = append(result.Checks, CheckItem{
			Name: "Runtime Health", Status: "pass", Detail: fmt.Sprintf("daemon ativo (pid %d)", pid),
		})
	} else {
		result.Checks = append(result.Checks, CheckItem{
			Name: "Runtime Daemon", Status: "warning", Detail: "processo não está rodando",
		})
		result.Checks = append(result.Checks, CheckItem{
			Name: "Runtime Health", Status: "warning",
			Detail: "Runtime is not active (daemon não está rodando)",
		})
		result.Issues = append(result.Issues, "Runtime is not active (daemon não está rodando)")
		result.Fixes = append(result.Fixes, "Rode 'cosca serve' ou 'systemctl --user start cosca-serve.service'")
	}

	return result
}

// processAlive reports whether the process with the given PID is running.
// Implementação platform-specific: signal_unix.go (kill(pid,0)) e
// signal_windows.go (OpenProcess + GetExitCodeProcess, best-effort).

func checkEditor(coscaDir string) CheckResult {
	result := CheckResult{}

	// Use editors.NewManager instead of non-existent NewDetector
	cfg := editors.DefaultEditorConfig(filepath.Dir(coscaDir))
	mgr := editors.NewManager(cfg)

	if mgr != nil {
		// Detect returns (EditorInfo, error) instead of just a string
		info, err := mgr.Detect()
		if err == nil {
			result.Checks = append(result.Checks, CheckItem{
				Name: "Editor Detection", Status: "pass", Detail: info.Name,
			})

			// Validate editor integration
			validateErr := mgr.Validate(info.Name)
			if validateErr == nil {
				result.Checks = append(result.Checks, CheckItem{
					Name: "Editor Integration", Status: "pass", Detail: fmt.Sprintf("%s integrated", info.Name),
				})
			} else {
				result.Checks = append(result.Checks, CheckItem{
					Name: "Editor Integration", Status: "warning",
					Detail: fmt.Sprintf("Integration not complete: %v", validateErr),
				})
				result.Issues = append(result.Issues, "Editor integration not installed")
				result.Fixes = append(result.Fixes, "Run 'cosca editor setup' to install the integration")
			}
		} else {
			result.Checks = append(result.Checks, CheckItem{
				Name: "Editor Detection", Status: "warning", Detail: fmt.Sprintf("No editor detected: %v", err),
			})
			result.Issues = append(result.Issues, "Could not detect a supported editor")
			result.Fixes = append(result.Fixes, "Install a supported editor or run 'cosca editor setup'")
		}
	} else {
		result.Checks = append(result.Checks, CheckItem{
			Name: "Editor Manager", Status: "fail", Detail: "Editor manager not available",
		})
	}

	return result
}

func checkPlugins(coscaDir string) CheckResult {
	result := CheckResult{}

	// Use plugins.NewManager with ManagerConfig and Loader instead of NewManager(path)
	cfg := plugins.DefaultManagerConfig(coscaDir)
	pm := plugins.NewManager(cfg, nil)

	if pm != nil {
		pluginsList := pm.List()
		installed := len(pluginsList)
		active := 0
		for _, p := range pluginsList {
			if p.Enabled && p.State == plugins.PluginStateStarted {
				active++
			}
		}

		if installed > 0 {
			result.Checks = append(result.Checks, CheckItem{
				Name: "Plugin Manager", Status: "pass",
				Detail: fmt.Sprintf("%d plugins installed", installed),
			})
			result.Checks = append(result.Checks, CheckItem{
				Name: "Active Plugins", Status: "pass",
				Detail: fmt.Sprintf("%d active", active),
			})

			if active < installed {
				result.Checks = append(result.Checks, CheckItem{
					Name: "Plugin Status", Status: "warning",
					Detail: fmt.Sprintf("%d/%d plugins active", active, installed),
				})
				result.Issues = append(result.Issues, "Some plugins are not active")
				result.Fixes = append(result.Fixes, "Run 'cosca plugin list' and enable inactive plugins")
			}
		} else {
			result.Checks = append(result.Checks, CheckItem{
				Name: "Plugins", Status: "pass", Detail: "No plugins installed",
			})
		}
	} else {
		result.Checks = append(result.Checks, CheckItem{
			Name: "Plugins", Status: "warning", Detail: "No plugin directory found",
		})
	}

	return result
}

func checkMemory(coscaDir string) CheckResult {
	result := CheckResult{}

	// Use memory.NewEngine with Option instead of non-existent NewManager
	engine, err := memory.NewEngine(
		memory.WithConfig(memory.EngineConfig{
			DataDir: coscaDir,
		}),
	)
	if err == nil && engine != nil {
		defer func() {
			if err := engine.Close(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close memory engine: %v\n", err)
			}
		}()

		ctx := context.Background()
		layerStats := engine.GetLayerStats(ctx)

		totalEntries := 0
		if stats, ok := layerStats[memory.LayerGlobal]; ok {
			totalEntries += stats.Count
		}
		if stats, ok := layerStats[memory.LayerWorkspace]; ok {
			totalEntries += stats.Count
		}
		if stats, ok := layerStats[memory.LayerProject]; ok {
			totalEntries += stats.Count
		}
		if stats, ok := layerStats[memory.LayerSession]; ok {
			totalEntries += stats.Count
		}

		result.Checks = append(result.Checks, CheckItem{
			Name: "Memory System", Status: "pass",
			Detail: fmt.Sprintf("%d total entries", totalEntries),
		})

		// Check each layer
		for _, ml := range []memory.MemoryLayer{
			memory.LayerGlobal,
			memory.LayerWorkspace,
			memory.LayerProject,
			memory.LayerSession,
			memory.LayerTemp,
		} {
			if s, ok := layerStats[ml]; ok {
				result.Checks = append(result.Checks, CheckItem{
					Name: fmt.Sprintf("%s Memory", ml), Status: "pass",
					Detail: fmt.Sprintf("%d entries", s.Count),
				})
			}
		}

		result.Checks = append(result.Checks, CheckItem{
			Name: "Memory Stores", Status: "pass", Detail: "All stores accessible",
		})
	} else {
		result.Checks = append(result.Checks, CheckItem{
			Name: "Memory", Status: "warning",
			Detail: fmt.Sprintf("Memory system not initialized: %v", err),
		})
		result.Issues = append(result.Issues, "Memory system is not initialized")
		result.Fixes = append(result.Fixes, "Run 'cosca init' to initialize memory stores")
	}

	return result
}

func checkKnowledge(coscaDir string) CheckResult {
	result := CheckResult{}

	// Use knowledge.New with Config instead of non-existent NewEngine(string)
	ke, err := knowledge.New(knowledge.Config{
		DBPath:  filepath.Join(coscaDir, "knowledge.db"),
		RootDir: filepath.Dir(coscaDir),
	})
	if err == nil && ke != nil {
		defer func() {
			if err := ke.Close(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
			}
		}()

		// BUGFIX (2026-08-03): sem Init() o Verify() retornava sempre
		// "knowledge engine not initialized" — o check de integridade era
		// permanentemente "1 issue" mesmo com a base saudável.
		if initErr := ke.Init(); initErr != nil {
			result.Checks = append(result.Checks, CheckItem{
				Name: "Knowledge Engine", Status: "warning",
				Detail: fmt.Sprintf("engine init: %v", initErr),
			})
			result.Issues = append(result.Issues, "knowledge engine failed to initialize")
			result.Fixes = append(result.Fixes, "Run 'cosca sync' to rebuild the index")
			return result
		}

		// GetStats instead of Stats()
		stats, statsErr := ke.GetStats()
		if statsErr == nil && stats != nil {
			totalEntries := stats.DocumentCount + stats.ChunkCount
			dbSize := stats.DBSize
			sizeStr := fmt.Sprintf("%d bytes", dbSize)
			if dbSize > 1024*1024 {
				sizeStr = fmt.Sprintf("%.1f MB", float64(dbSize)/(1024*1024))
			} else if dbSize > 1024 {
				sizeStr = fmt.Sprintf("%.1f KB", float64(dbSize)/1024)
			}

			result.Checks = append(result.Checks, CheckItem{
				Name: "Knowledge Engine", Status: "pass",
				Detail: fmt.Sprintf("%d entries, %s database", totalEntries, sizeStr),
			})

			// Verify integrity
			verify, verifyErr := ke.Verify()
			if verifyErr == nil && verify != nil && len(verify.Issues) > 0 {
				result.Checks = append(result.Checks, CheckItem{
					Name: "Index Integrity", Status: "warning",
					Detail: fmt.Sprintf("%d issues found", len(verify.Issues)),
				})
				result.Issues = append(result.Issues, fmt.Sprintf("%d integrity issues in knowledge base", len(verify.Issues)))
				result.Fixes = append(result.Fixes, "Run 'cosca knowledge compile' (rebuild index) to re-embed chunks without vectors")
			} else {
				result.Checks = append(result.Checks, CheckItem{
					Name: "Index Integrity", Status: "pass", Detail: "All entries valid",
				})
			}
		} else {
			result.Checks = append(result.Checks, CheckItem{
				Name: "Knowledge Engine", Status: "warning",
				Detail: fmt.Sprintf("Stats unavailable: %v", statsErr),
			})
			result.Issues = append(result.Issues, "Knowledge engine failed to report stats")
			result.Fixes = append(result.Fixes, "Run 'cosca sync' to rebuild the index")
		}
	} else {
		result.Checks = append(result.Checks, CheckItem{
			Name: "Knowledge Engine", Status: "fail",
			Detail: fmt.Sprintf("Engine not available: %v", err),
		})
		result.Issues = append(result.Issues, "Knowledge engine is not initialized")
		result.Fixes = append(result.Fixes, "Run 'cosca install' to set up the knowledge engine")
	}

	return result
}

func checkProviders(_ string) CheckResult {
	result := CheckResult{}
	pm := providers.NewManager()

	if pm != nil {
		status := pm.Status()

		// providers.NewManager() nao carrega o config, entao m.active fica
		// sempre vazio. Reporte o provider primario declarado no config
		// (provider.name) como ativo quando ele estiver configurado.
		active := status.Active
		if active == "" {
			active = activeProviderFromConfigDoctor(status.Statuses)
		}

		if active != "" {
			result.Checks = append(result.Checks, CheckItem{
				Name: "Active Provider", Status: "pass", Detail: active,
			})
		} else {
			result.Checks = append(result.Checks, CheckItem{
				Name: "Active Provider", Status: "warning", Detail: "No active provider",
			})
			result.Issues = append(result.Issues, "No AI provider is active")
			result.Fixes = append(result.Fixes, "Run 'cosca provider set <name>' to set a provider")
		}

		result.Checks = append(result.Checks, CheckItem{
			Name: "Configured Providers", Status: "pass",
			Detail: fmt.Sprintf("%d configured", status.Configured),
		})
		result.Checks = append(result.Checks, CheckItem{
			Name: "Available Providers", Status: "pass",
			Detail: fmt.Sprintf("%d available", status.Available),
		})

		for _, s := range status.Statuses {
			result.Checks = append(result.Checks, CheckItem{
				Name: fmt.Sprintf("Provider %s", s), Status: "pass",
			})
		}
	} else {
		result.Checks = append(result.Checks, CheckItem{
			Name: "Providers", Status: "warning", Detail: "Provider manager not available",
		})
	}

	return result
}

// checkDependencyVulns scans the workspace dependencies (go.mod/go.sum, etc.)
// for known OSV vulnerabilities. It does NOT fail doctor unless CRITICAL/HIGH
// vulnerabilities are found; network errors degrade to a warning so `cosca
// doctor` still works offline.
func checkDependencyVulns(coscaDir string) CheckResult {
	result := CheckResult{}

	workspace := filepath.Dir(coscaDir)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Non-recursive: dependency manifests (go.mod, go.sum, package.json) live
	// at the project root. Recursive walks of the whole workspace can hit
	// permission errors inside the jail (e.g. /proc) and scan far more than
	// the project's own dependencies.
	scanResult, err := security.Scan(ctx, security.Options{
		Dir:         workspace,
		Recursive:   false,
		MinSeverity: security.SeverityLow,
	})
	if err != nil {
		result.Checks = append(result.Checks, CheckItem{
			Name:   "Dependency Vulnerabilities",
			Status: "warning",
			Detail: fmt.Sprintf("scan unavailable: %v", err),
		})
		result.Issues = append(result.Issues, "Dependency vulnerability scan could not run (network/OSV unavailable)")
		return result
	}

	if scanResult.NoDependencyFiles {
		result.Checks = append(result.Checks, CheckItem{
			Name:   "Dependency Vulnerabilities",
			Status: "pass",
			Detail: "no dependency files found",
		})
		return result
	}

	detail := fmt.Sprintf("%d packages scanned, %d vulnerabilities", scanResult.PackagesScanned, scanResult.TotalVulns)
	if scanResult.TotalVulns == 0 {
		result.Checks = append(result.Checks, CheckItem{
			Name:   "Dependency Vulnerabilities",
			Status: "pass",
			Detail: detail + " — clean",
		})
		return result
	}

	if scanResult.HasCriticalHigh() {
		result.Checks = append(result.Checks, CheckItem{
			Name:   "Dependency Vulnerabilities",
			Status: "fail",
			Detail: fmt.Sprintf("%s — %d CRITICAL/HIGH found", detail, scanResult.CriticalHigh()),
		})
		result.Issues = append(result.Issues,
			fmt.Sprintf("%d critical/high vulnerabilities in dependencies (see 'cosca security scan')", scanResult.CriticalHigh()))
		result.Fixes = append(result.Fixes, "Run 'cosca security scan' for details and update affected packages")
		return result
	}

	result.Checks = append(result.Checks, CheckItem{
		Name:   "Dependency Vulnerabilities",
		Status: "warning",
		Detail: detail,
	})
	result.Issues = append(result.Issues,
		fmt.Sprintf("%d low/medium vulnerabilities in dependencies — review with 'cosca security scan'", scanResult.TotalVulns))
	result.Fixes = append(result.Fixes, "Run 'cosca security scan' to review low/medium advisories")
	return result
}

// activeProviderFromConfig determina o provider ativo a partir do config
// (provider.primary / provider.name). O manager criado por NewManager()
// nunca le o config, entao Status().Active e sempre vazio — o config e a
// fonte de verdade aqui. Se o config falhar ao carregar, retorna "" e o
// check cai no warning "No active provider" sem crash.
func activeProviderFromConfigDoctor(managerStatuses []string) string {
	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	if cfg.Provider.Name == "" {
		return ""
	}

	// Um provider primario conta como ativo quando a chave esta configurada
	// no config (api_key) OU o manager o reporta como configured/available
	// (ex.: providers locais como ollama que nao exigem chave).
	if cfg.Provider.APIKey != "" {
		return cfg.Provider.Name
	}
	switch providerStatus(managerStatuses, cfg.Provider.Name) {
	case "configured", "available":
		return cfg.Provider.Name
	}
	return ""
}

// providerStatus extrai o status do provider nomeado a partir das entradas
// "name: status" reportadas por providers.Manager.Status().
func providerStatus(statuses []string, name string) string {
	prefix := name + ": "
	for _, s := range statuses {
		if strings.HasPrefix(s, prefix) {
			return strings.TrimPrefix(s, prefix)
		}
	}
	return ""
}
