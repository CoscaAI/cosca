package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CommitInfo holds the metadata and statistics of a single commit.
type CommitInfo struct {
	Hash       string
	Message    string
	Author     string
	Date       string
	Files      int
	Insertions int
	Deletions  int
	FileList   []string
}

// timelineBase is the runtime home for timeline artifacts, relative to the
// repository root. This is the NEW home (.cosca), NOT the legacy framework
// home (now .cosca/framework).
const timelineBase = ".cosca/memory/timeline"

// impactReportsDir is where per-commit impact reports are stored.
const impactReportsDir = timelineBase + "/impact-reports"

// GenerateImpactReport inspects the most recent commit in the repository at
// repoDir and produces an Impact Report plus an Engineering Timeline entry
// under .cosca/memory/timeline/. It returns the path of the generated report.
//
// Behaviour guarantees (mirroring the legacy bash post-commit hook):
//   - Idempotent: re-running for the same commit returns the existing report
//     path without creating duplicates.
//   - Loop-safe: it never creates commits, and commits that only touch .cosca/
//     (the hook's own output being committed back) are skipped and return an
//     empty string.
//   - Read-only on git: only log/show queries are issued.
func GenerateImpactReport(repoDir string) (string, error) {
	if err := ensureGitRepo(repoDir); err != nil {
		return "", err
	}

	info, err := collectCommitInfo(repoDir)
	if err != nil {
		return "", err
	}

	memoryDir := filepath.Join(repoDir, timelineBase)
	impactDir := filepath.Join(repoDir, impactReportsDir)
	if err := os.MkdirAll(impactDir, 0o755); err != nil {
		return "", fmt.Errorf("create impact reports dir %s: %w", impactDir, err)
	}

	reportPath := filepath.Join(impactDir, info.Hash+".md")

	// Idempotency: an existing report means this commit was already
	// processed. Re-run must not create duplicates.
	if _, err := os.Stat(reportPath); err == nil {
		return reportPath, nil
	}

	// Loop guard: commits touching only .cosca/ are the hook's own output
	// (impact reports, timeline) being committed back. Registering those
	// would generate reports about reports; skip them.
	if memoryOnlyCommit(info.FileList) {
		return "", nil
	}

	content := renderReport(info, time.Now().UTC())
	if err := os.WriteFile(reportPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write impact report %s: %w", reportPath, err)
	}

	if err := appendTimelineEntry(memoryDir, info); err != nil {
		return "", err
	}

	return reportPath, nil
}

// collectCommitInfo gathers commit metadata and diff statistics for HEAD.
// Stats are read via `git show --stat` (with a --numstat fallback for
// locales/edge cases where the summary line cannot be parsed).
func collectCommitInfo(repoDir string) (CommitInfo, error) {
	var info CommitInfo

	out, err := runGit(repoDir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return info, fmt.Errorf("resolve HEAD: %w", err)
	}
	info.Hash = strings.TrimSpace(out)
	if info.Hash == "" {
		return info, fmt.Errorf("repository %s has no commits", repoDir)
	}

	info.Message = gitLogField(repoDir, "%s")
	info.Author = gitLogField(repoDir, "%an")
	info.Date = gitLogField(repoDir, "%aI") // strict ISO 8601

	// ── Diff statistics via git show --stat ────────────────────────────────
	statOut, err := runGit(repoDir, "show", "--stat", "--format=", "HEAD")
	if err != nil {
		return info, fmt.Errorf("git show --stat: %w", err)
	}
	info.Files, info.Insertions, info.Deletions = parseStatSummary(statOut)
	if info.Files == 0 {
		info.Files, info.Insertions, info.Deletions = numstatTotals(repoDir)
	}

	// ── Changed file list (used by the memory-only loop guard) ─────────────
	namesOut, err := runGit(repoDir, "show", "--name-only", "--format=", "HEAD")
	if err == nil {
		for _, line := range strings.Split(namesOut, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				info.FileList = append(info.FileList, line)
			}
		}
	}

	return info, nil
}

// gitLogField returns a single log field for HEAD, defaulting to an empty
// string on failure (the hook must never break the commit).
func gitLogField(repoDir, format string) string {
	out, err := runGit(repoDir, "log", "-1", "--format="+format, "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// statSummaryRe matches the `git show --stat` summary line, e.g.:
//
//	2 files changed, 5 insertions(+), 1 deletion(-)
//	1 file changed, 4 deletions(-)
var statSummaryRe = regexp.MustCompile(`(\d+) files? changed(?:, (\d+) insertions?\(\+\))?(?:, (\d+) deletions?\(-\))?`)

// parseStatSummary extracts files/insertions/deletions from git show --stat
// output. Returns zeros when no summary line is found.
func parseStatSummary(out string) (files, insertions, deletions int) {
	for _, line := range strings.Split(out, "\n") {
		m := statSummaryRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		files, _ = strconv.Atoi(m[1])
		if m[2] != "" {
			insertions, _ = strconv.Atoi(m[2])
		}
		if m[3] != "" {
			deletions, _ = strconv.Atoi(m[3])
		}
		return files, insertions, deletions
	}
	return 0, 0, 0
}

// numstatTotals computes per-file totals via `git show --numstat`, used as a
// fallback when the --stat summary line is unavailable (e.g. empty commits).
func numstatTotals(repoDir string) (files, insertions, deletions int) {
	out, err := runGit(repoDir, "show", "--numstat", "--format=", "HEAD")
	if err != nil {
		return 0, 0, 0
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		files++
		if fields[0] == "-" || fields[1] == "-" {
			continue // binary file — no textual stats
		}
		add, _ := strconv.Atoi(fields[0])
		del, _ := strconv.Atoi(fields[1])
		insertions += add
		deletions += del
	}
	return files, insertions, deletions
}

// renderReport builds the Impact Report markdown document. The layout is
// defined by the internal/hooks spec and inspired by the legacy bash hook and
// the memorize-commit workflow.
func renderReport(info CommitInfo, ts time.Time) string {
	msg := strings.ReplaceAll(info.Message, "|", `\|`)
	stats := fmt.Sprintf("%d files changed, %d insertions(+), %d deletions(-)",
		info.Files, info.Insertions, info.Deletions)

	return fmt.Sprintf(`---
type: impact-report
commit: %s
timestamp: %s
agent: cosca-kernel
---
# Impact Report — %s
| Campo | Valor |
|---|---|
| Commit | %s |
| Mensagem | %s |
| Autor | %s |
| Data | %s |
| Arquivos | %d |
| +%d/-%d | %s |
`,
		info.Hash, ts.Format(time.RFC3339), info.Hash,
		info.Hash, msg, info.Author, info.Date,
		info.Files, info.Insertions, info.Deletions, stats)
}

// appendTimelineEntry appends one row to ENGINEERING_TIMELINE.md, creating
// the file with a header on first use. Row format follows the
// memorize-commit workflow:
//
//	| {time} | {type} | {message} | +{ins}/-{del} | {files} | {cost} | {learnings} | {hash} |
func appendTimelineEntry(memoryDir string, info CommitInfo) error {
	timelinePath := filepath.Join(memoryDir, "ENGINEERING_TIMELINE.md")

	data, err := os.ReadFile(timelinePath)
	if err != nil {
		data = []byte("# Engineering Timeline\n\n" +
			"> Registro cronológico de commits (gerado por `cosca hook post-commit`)\n\n" +
			"| Horário | Tipo | Descrição | Impacto | Arquivos | Custo | Aprendizados | Commit |\n" +
			"|---------|------|-----------|---------|:--------:|:-----:|:------------:|--------|\n")
	}

	entry := fmt.Sprintf("| %s | %s | %s | +%d/-%d | %d | $%.3f | 0 | `%s` |",
		time.Now().Format("15:04:05"),
		changeType(info.Message),
		strings.ReplaceAll(info.Message, "|", `\|`),
		info.Insertions, info.Deletions,
		info.Files,
		estCost(info),
		info.Hash,
	)

	content := strings.TrimRight(string(data), "\n")
	if content != "" {
		content += "\n"
	}
	content += entry + "\n"

	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		return fmt.Errorf("create timeline dir %s: %w", memoryDir, err)
	}
	if err := os.WriteFile(timelinePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("update engineering timeline %s: %w", timelinePath, err)
	}
	return nil
}

// changeType classifies a commit message prefix into the workflow's change
// type taxonomy (memorize-commit.md Step 2).
func changeType(msg string) string {
	prefix := strings.TrimSpace(msg)
	if i := strings.Index(prefix, ":"); i > 0 {
		prefix = prefix[:i]
	}
	switch strings.ToLower(prefix) {
	case "feat":
		return "feature"
	case "fix":
		return "bug-fix"
	case "test":
		return "testing"
	case "docs":
		return "documentation"
	case "refactor":
		return "refactoring"
	case "chore":
		return "chore"
	case "learn":
		return "learning"
	case "perf":
		return "performance"
	case "sec", "security":
		return "security"
	case "ci":
		return "ci"
	case "arch", "architecture":
		return "architecture"
	case "release":
		return "release"
	default:
		return "other"
	}
}

// estCost estimates the engineering cost of a commit using the
// memorize-commit workflow formula:
//
//	(insertions × 0.02 + files_changed × 0.5) / 1000 USD
func estCost(info CommitInfo) float64 {
	return (float64(info.Insertions)*0.02 + float64(info.Files)*0.5) / 1000
}

// memoryOnlyCommit reports whether every changed file lives under .cosca/,
// the runtime data directory that holds the hook's own output (impact
// reports, ENGINEERING_TIMELINE.md). Such commits are the hook's artifacts
// being committed back; skipping them prevents recursive report generation.
func memoryOnlyCommit(files []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		// Git reports changed paths with forward slashes on every platform,
		// so match both separators — on Windows filepath.Separator is '\'
		// and would miss ".cosca/memory/...". Linux behavior is unchanged.
		if f != ".cosca" && !strings.HasPrefix(f, ".cosca/") && !strings.HasPrefix(f, ".cosca\\") {
			return false
		}
	}
	return true
}
