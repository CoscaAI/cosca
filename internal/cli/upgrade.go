package cli

import (
	"crypto/sha256"
	"encoding/hex"
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
	// Protect marca o alvo como zona protegida de autoridade (FROZEN/LIVE/RUNTIME).
	// O upgrade reporta, tira snapshot e PRESERVA — nunca RemoveAll.
	Protect bool `json:"protect,omitempty" yaml:"protect,omitempty"`
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
		Long: `Detect and clean up legacy Cosca artifacts from old architectures.

Zonas de autoridade (Contrato de Autoridade — FROZEN > LIVE > RUNTIME):
  • .opencode/cosca/       — zona LIVE (superfície do OpenCode): é preservada,
                             snapshot backup + aviso de drift; NUNCA é removida.
  • .cosca/fallback/       — alvo ativo do MaterializeFallback (embed.go):
                             é preservado, NUNCA é removido.
  • ~/.config/cosca in PATH — old global install path (now ~/.cosca/bin/)

Backs up .cosca/ and snapshots the LIVE zone before making changes.
Use --dry-run to preview (mandatory for review; nothing is ever removed from a
protected zone).`,
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

	// ── Zona LIVE: verificação de drift (determinística, não-destrutiva) ──
	// Lê apenas (.opencode/cosca vs internal/embed/cosca por caminho+hash).
	// O FROZEN permanece autoridade durante qualquer drift; o LIVE é preservado
	// como evidência e candidato a promoção explícita (Contrato §2/§3/§4).
	if drift, err := measureLiveDrift(dir); err == nil && drift.Present {
		formatter.Bullet("Zona LIVE (.opencode/cosca) detectada — " + drift.Summary())
		if drift.DriftCount() > 0 {
			formatter.Warning(fmt.Sprintf(
				"DRIFT: %d arquivo(s) da zona LIVE divergem do FROZEN (internal/embed/cosca). "+
					"O FROZEN permanece autoridade; o LIVE é preservado como evidência — nenhuma remoção é feita.",
				drift.DriftCount()))
		}
		report.ActionsTaken = append(report.ActionsTaken, "live drift → "+drift.Summary())
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

	// Snapshot da zona LIVE (.opencode/cosca) — preserva evidência e permite
	// recuperação. O upgrade nunca remove o LIVE; apenas o arquiva.
	liveDir := filepath.Join(dir, ".opencode", "cosca")
	if _, err := os.Stat(liveDir); err == nil {
		liveSnap := filepath.Join(backupDir, "live-snapshot")
		formatter.Bullet("Snapshot zona LIVE (.opencode/cosca) → " + liveSnap)
		if err := copyDir(liveDir, liveSnap); err != nil {
			formatter.Warning("Falha no snapshot LIVE (best-effort): " + err.Error())
		}
		report.ActionsTaken = append(report.ActionsTaken, "live snapshot → "+liveSnap)
	}

	// ── Phase 3: Clean ──────────────────────────────────────────
	// Zonas protegidas (FROZEN/LIVE/RUNTIME) são PRESERVADAS, nunca removidas.
	// O restante passa pelo guard FAIL-CLOSED antes de qualquer RemoveAll.
	removed := 0
	for _, iss := range issues {
		if iss.Protect {
			formatter.Bullet("Preservando (zona protegida): " + iss.Path)
			report.ActionsTaken = append(report.ActionsTaken, "preserved → "+iss.Path)
			continue
		}
		formatter.Bullet("Limpando: " + iss.Path)
		if err := GuardedRemoveAll(iss.Path); err != nil {
			formatter.Warning("Falha ao remover " + iss.Path + ": " + err.Error())
			continue
		}
		report.ActionsTaken = append(report.ActionsTaken, "removed → "+iss.Path)
		removed++
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

	// 1. .opencode/cosca/ — zona LIVE do Contrato de Autoridade.
	//	G2: "Nenhuma operação pode deletar/conteúdo-destruir o LIVE".
	//	Aqui o upgrade NÃO remove: reporta, leva snapshot e alerta DRIFT.
	opencodeCosca := filepath.Join(dir, ".opencode", "cosca")
	if info, err := os.Stat(opencodeCosca); err == nil && info.IsDir() {
		issues = append(issues, LegacyIssue{
			Path:    opencodeCosca,
			Issue:   "Zona LIVE detectada (.opencode/cosca/)",
			Action:  "PROTEGIDO — contrato de autoridade (FROZEN>LIVE>RUNTIME): NÃO remover. Snapshot + aviso de drift, evidência preservada.",
			Protect: true,
		})
	}

	// 2. .cosca/fallback/ — alvo ativo do MaterializeFallback (embed.go).
	//	Removê-lo quebraria os consumidores do fallback tree. NÃO remover.
	fallbackDir := filepath.Join(dir, ".cosca", "fallback")
	if info, err := os.Stat(fallbackDir); err == nil && info.IsDir() {
		size := dirSize(fallbackDir)
		issues = append(issues, LegacyIssue{
			Path:    fallbackDir,
			Issue:   "MaterializeFallback target (.cosca/fallback/)",
			Action:  "PRESERVADO — alvo ativo do MaterializeFallback (internal/embed/embed.go): NÃO remover.",
			SizeEst: formatSize(size),
			Protect: true,
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

// ── Detector de drift FROZEN ↔ LIVE (não-destrutivo) ────────────────────────
//
// O detector compara a zona LIVE (.opencode/cosca) com o FROZEN
// (internal/embed/cosca) por caminho relativo canônico + SHA-256 (Contrato §2),
// de forma determinística e apenas-leitura. Nunca escolhe vencedor: apenas
// classifica e orienta. O FROZEN permanece autoridade durante qualquer drift; o
// LIVE é preservado como evidência (candidato a promoção explícita, §4).

// LiveDriftReport descreve a divergência entre LIVE e FROZEN.
type LiveDriftReport struct {
	Present    bool `json:"present"`     // LIVE (.opencode/cosca) existe?
	Comparable bool `json:"comparable"`  // FROZEN em disco existe para comparar?
	Matches    int  `json:"matches"`     // arquivos idênticos (MATCH)
	Divergent  int  `json:"divergent"`   // mesmo caminho, hash != (drift)
	OnlyFrozen int  `json:"only_frozen"` // só no FROZEN (exclusivos — preservar, G5)
	OnlyLive   int  `json:"only_live"`   // só no LIVE (evolução a promover, §4)
}

// DriftCount é o total de divergências relevantes (a sinalizar DRIFT).
func (r LiveDriftReport) DriftCount() int { return r.Divergent + r.OnlyLive }

// Summary resume o estado de forma legível para o relatório do upgrade.
func (r LiveDriftReport) Summary() string {
	if !r.Present {
		return "LIVE não presente — sem drift"
	}
	if !r.Comparable {
		return fmt.Sprintf("%d arquivo(s) no LIVE; FROZEN embutido no binário — sem árvore em disco para comparar (fluxo third-party), tudo registrado como evidência", r.OnlyLive+r.Matches)
	}
	return fmt.Sprintf("%d MATCH, %d divergente(s), %d só-LIVE, %d exclusivos-FROZEN",
		r.Matches, r.Divergent, r.OnlyLive, r.OnlyFrozen)
}

// measureLiveDrift calcula a divergência LIVE↔FROZEN. Determinístico e
// não-destrutivo: apenas lê arquivos. Se o FROZEN não existir em disco
// (projeto third-party, framework apenas no binário), toda a árvore LIVE
// é registrada como evidência (Comparable=false).
func measureLiveDrift(dir string) (LiveDriftReport, error) {
	var rep LiveDriftReport

	liveRoot := filepath.Join(dir, ".opencode", "cosca")
	if _, err := os.Stat(liveRoot); err != nil {
		rep.Present = false
		return rep, nil
	}
	rep.Present = true

	frozenRoot := filepath.Join(dir, "internal", "embed", "cosca")
	if _, err := os.Stat(frozenRoot); err != nil {
		// FROZEN embutido no binário (não há árvore em disco).
		rep.Comparable = false
		rep.OnlyLive = countFiles(liveRoot)
		return rep, nil
	}
	rep.Comparable = true

	live := fileHashes(liveRoot)
	frozen := fileHashes(frozenRoot)
	for rel, lh := range live {
		fh, ok := frozen[rel]
		switch {
		case !ok:
			rep.OnlyLive++
		case fh == lh:
			rep.Matches++
		default:
			rep.Divergent++
		}
	}
	for rel := range frozen {
		if _, ok := live[rel]; !ok {
			rep.OnlyFrozen++ // exclusivos do FROZEN — preservar (G5)
		}
	}
	return rep, nil
}

// fileHashes devolve relPathCanonical → sha256 para todos os arquivos sob root.
// Comparamos por caminho relativo canônico com "/" (evita o bug de filepath.Rel).
func fileHashes(root string) map[string]string {
	out := map[string]string{}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		data, derr := os.ReadFile(path)
		if derr != nil {
			return nil // skip unreadable
		}
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	return out
}

// countFiles conta os arquivos (não diretórios) sob root.
func countFiles(root string) int {
	n := 0
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			n++
		}
		return nil
	})
	return n
}
