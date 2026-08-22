package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// LegacyIssue represents a detected legacy artifact that needs cleanup.
type LegacyIssue struct {
	Path    string `json:"path" yaml:"path"`
	Issue   string `json:"issue" yaml:"issue"`
	Action  string `json:"action" yaml:"action"`
	SizeEst string `json:"size_est,omitempty" yaml:"size_est,omitempty"`
}

// UpgradeReport holds the full upgrade analysis/report.
type UpgradeReport struct {
	ProjectDir   string        `json:"project_dir" yaml:"project_dir"`
	DryRun       bool          `json:"dry_run" yaml:"dry_run"`
	IssuesFound  int           `json:"issues_found" yaml:"issues_found"`
	Issues       []LegacyIssue `json:"issues" yaml:"issues"`
	BackupPath   string        `json:"backup_path,omitempty" yaml:"backup_path,omitempty"`
	ActionsTaken []string      `json:"actions_taken,omitempty" yaml:"actions_taken,omitempty"`
	Duration     string        `json:"duration" yaml:"duration"`
}

// NewUpgradeCommand creates the `cosca upgrade` command.
func NewUpgradeCommand() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade Cosca from legacy paths to current architecture",
		Long: `Detect and clean up legacy Cosca artifacts from old architectures:

  • .opencode/cosca/       — old framework directory (now internal/embed/cosca/)
  • .cosca/fallback/       — old sync target (now .cosca/framework/)
  • ~/.config/cosca in PATH — old global install path (now ~/.cosca/bin/)

Backs up .cosca/ before making changes. Use --dry-run to preview.`,
		Example: `  cosca upgrade --dry-run    # Preview what needs cleanup
  cosca upgrade              # Execute cleanup + re-init`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUpgrade(cmd, dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview changes without executing")
	return cmd
}

func runUpgrade(cmd *cobra.Command, dryRun bool) error {
	formatter := GetFormatter(cmd)
	useJSON := IsJSONOutput(cmd)
	startTime := time.Now()

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	report := UpgradeReport{
		ProjectDir: dir,
		DryRun:     dryRun,
	}

	// ── Phase 1: Detect ──────────────────────────────────────────
	issues := detectLegacyIssues(dir)

	report.IssuesFound = len(issues)
	report.Issues = issues

	if len(issues) == 0 {
		report.Duration = time.Since(startTime).Round(time.Millisecond).String()
		if useJSON {
			return printJSON(cmd, report)
		}
		formatter.Success("Nenhum artefato legado detectado — projeto ja esta na arquitetura atual.")
		return nil
	}

	// ── Show what was found ─────────────────────────────────────
	if useJSON {
		report.Duration = time.Since(startTime).Round(time.Millisecond).String()
		return printJSON(cmd, report)
	}

	formatter.Header("Cosca Upgrade — Artefatos Legados Detectados")
	for _, iss := range issues {
		formatter.KeyValue(iss.Issue, iss.Path)
		if iss.Action != "" {
			formatter.Bullet(iss.Action)
		}
	}

	if dryRun {
		formatter.Println("\n--dry-run: nenhuma alteracao foi feita. Rode 'cosca upgrade' para executar.")
		report.Duration = time.Since(startTime).Round(time.Millisecond).String()
		if useJSON {
			return printJSON(cmd, report)
		}
		return nil
	}

	// ── Phase 2: Backup ─────────────────────────────────────────
	coscaDir := filepath.Join(dir, ".cosca")
	backupDir := filepath.Join(dir, ".cosca", "backups", "pre-upgrade-"+time.Now().Format("20060102-150405"))

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Backup .cosca/ if it exists
	if _, err := os.Stat(coscaDir); err == nil {
		formatter.Bullet("Backup .cosca/ → " + backupDir)
		if err := copyDir(coscaDir, backupDir); err != nil {
			formatter.Warning("Full backup failed, salvaging critical files...")
			salvageCritical(coscaDir, backupDir)
		}
		report.BackupPath = backupDir
		report.ActionsTaken = append(report.ActionsTaken, "backup → "+backupDir)
	}

	// ── Phase 3: Clean ──────────────────────────────────────────
	for _, iss := range issues {
		formatter.Bullet("Limpando: " + iss.Path)
		if err := os.RemoveAll(iss.Path); err != nil {
			formatter.Warning("Falha ao remover " + iss.Path)
			continue
		}
		report.ActionsTaken = append(report.ActionsTaken, "removed → "+iss.Path)
	}

	// ── Phase 4: Re-init ────────────────────────────────────────
	formatter.Bullet("Reinicializando .cosca/...")
	// Run init --force equivalent inline
	if err := os.MkdirAll(coscaDir, 0700); err != nil {
		formatter.Warning("Falha ao criar .cosca/")
	} else {
		// Create minimal runtime structure
		for _, sub := range []string{"config", "cache", "plugins", "sessions", "audit"} {
			os.MkdirAll(filepath.Join(coscaDir, sub), 0700)
		}
		os.MkdirAll(filepath.Join(coscaDir, "memory", "agent"), 0700)
		os.MkdirAll(filepath.Join(coscaDir, "memory", "trust"), 0700)
		report.ActionsTaken = append(report.ActionsTaken, "re-initialized .cosca/")
	}

	// ── Phase 5: Report ─────────────────────────────────────────
	elapsed := time.Since(startTime)
	report.Duration = elapsed.Round(time.Millisecond).String()

	formatter.Success("Upgrade concluido em " + elapsed.Round(time.Millisecond).String())
	formatter.KeyValue("Issues resolvidos", fmt.Sprintf("%d", len(issues)))
	if report.BackupPath != "" {
		formatter.KeyValue("Backup", report.BackupPath)
	}
	formatter.Println("\nExecute 'cosca init --force' para regenerar a config completa se necessario.")

	if useJSON {
		return printJSON(cmd, report)
	}
	return nil
}

// detectLegacyIssues scans for known legacy artifacts.
func detectLegacyIssues(dir string) []LegacyIssue {
	var issues []LegacyIssue

	// 1. .opencode/cosca/ — old framework directory
	opencodeCosca := filepath.Join(dir, ".opencode", "cosca")
	if info, err := os.Stat(opencodeCosca); err == nil && info.IsDir() {
		issues = append(issues, LegacyIssue{
			Path:   opencodeCosca,
			Issue:  "Framework legado (.opencode/cosca/)",
			Action: "remover (framework agora vive em internal/embed/cosca/)",
		})
	}

	// 2. .cosca/fallback/ — old sync target
	fallbackDir := filepath.Join(dir, ".cosca", "fallback")
	if info, err := os.Stat(fallbackDir); err == nil && info.IsDir() {
		size := dirSize(fallbackDir)
		issues = append(issues, LegacyIssue{
			Path:    fallbackDir,
			Issue:   "Sync target obsoleto (.cosca/fallback/)",
			Action:  "remover (framework sync agora usa .cosca/framework/)",
			SizeEst: formatSize(size),
		})
	}

	// 3. .cosca/fallback memory copies (if fallback dir is gone but subpaths remain)
	// Already covered by #2 since we check the directory.

	// 4. Old global PATH entries in shell rc files
	home, _ := os.UserHomeDir()
	for _, rc := range []string{".bashrc", ".zshrc", ".bash_profile", ".profile"} {
		rcPath := filepath.Join(home, rc)
		data, err := os.ReadFile(rcPath)
		if err != nil {
			continue
		}
		content := string(data)
		oldPath := `$HOME/.config/cosca`
		if strings.Contains(content, oldPath) {
			issues = append(issues, LegacyIssue{
				Path:   rcPath,
				Issue:  fmt.Sprintf("PATH antigo no %s (%s)", rc, oldPath),
				Action: "substituir por $HOME/.cosca/bin (execute 'source " + rcPath + "' depois)",
			})
		}
	}

	// 5. Legacy-opencode dir
	legacyOpenCode := filepath.Join(dir, ".cosca", "legacy-opencode")
	if info, err := os.Stat(legacyOpenCode); err == nil && info.IsDir() {
		issues = append(issues, LegacyIssue{
			Path:   legacyOpenCode,
			Issue:  "Config legada do OpenCode (.cosca/legacy-opencode/)",
			Action: "remover (config agora em .opencode/opencode.json)",
		})
	}

	return issues
}

// ── Helpers ──────────────────────────────────────────────────────────

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip the backup dir itself to avoid recursion
		if strings.Contains(path, "backups/pre-upgrade-") {
			return nil
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

func salvageCritical(srcDir, dstDir string) {
	for _, f := range []string{"config.yaml", "knowledge.db"} {
		src := filepath.Join(srcDir, f)
		dst := filepath.Join(dstDir, f)
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		os.MkdirAll(filepath.Dir(dst), 0700)
		os.WriteFile(dst, data, 0600)
	}
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func formatSize(bytes int64) string {
	switch {
	case bytes > 10*1024*1024:
		return fmt.Sprintf("%d MB", bytes/(1024*1024))
	case bytes > 1024:
		return fmt.Sprintf("%d KB", bytes/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
