package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/security"
)

// GateResult holds the complete pre-commit quality assessment.
type GateResult struct {
	// Build & Test
	BuildPass  bool `json:"build_pass"`
	TestPass   bool `json:"test_pass"`
	TestTotal  int  `json:"test_total"`
	TestFailed int  `json:"test_failed"`
	LintErrors int  `json:"lint_errors"`

	// Security
	SecretsFound    int `json:"secrets_found"`
	CriticalSecrets int `json:"critical_secrets"`

	// Dependency vulnerabilities (OSV)
	SecurityVulns       int  `json:"security_vulns"`
	CriticalHighVulns   int  `json:"critical_high_vulns"`
	SecurityScanSkipped bool `json:"security_scan_skipped"`
	SecurityScanError   bool `json:"security_scan_error"`

	// Diff
	DiffAdded   int `json:"diff_added"`
	DiffRemoved int `json:"diff_removed"`
	DiffFiles   int `json:"diff_files"`
	TODOsAdded  int `json:"todos_added"`
	FIXMEsAdded int `json:"fixmes_added"`

	// Evidência da IA
	AgentsInvolved     int     `json:"agents_involved"`
	KnowledgeSources   int     `json:"knowledge_sources"`
	LLMCalls           int     `json:"llm_calls"`
	AutoRecoveries     int     `json:"auto_recoveries"`
	HumanInterventions int     `json:"human_interventions"`
	AutonomyScore      float64 `json:"autonomy_score"`

	// Scores
	Correctness     int `json:"correctness"`     // 0-100
	Security        int `json:"security"`        // 0-100
	Maintainability int `json:"maintainability"` // 0-100
	Regression      int `json:"regression"`      // 0-100
	QualityScore    int `json:"quality_score"`   // 0-100 weighted average

	Ready   bool   `json:"ready"`
	Verdict string `json:"verdict"`
}

// Gate runs the pre-commit quality gate assessment.
type Gate struct {
	workDir           string
	history           *WorkflowHistory
	skipSecurityVulns bool
	skipTests         bool
}

// NewGate creates a new quality gate for the given workspace.
func NewGate(workDir string) *Gate {
	return &Gate{workDir: workDir}
}

// SetHistory configures the analytics store for autonomy metrics.
func (g *Gate) SetHistory(history *WorkflowHistory) {
	g.history = history
}

// SetSkipSecurityVulns disables the dependency vulnerability (OSV) scan.
// Use for offline environments or when the caller explicitly opts out.
func (g *Gate) SetSkipSecurityVulns(skip bool) {
	g.skipSecurityVulns = skip
}

// SetSkipTests disables test execution. Use when the caller explicitly
// opts out (--no-tests); the gate then treats tests as passing.
func (g *Gate) SetSkipTests(skip bool) {
	g.skipTests = skip
}

// Run executes all quality checks and returns the gate result.
func (g *Gate) Run(ctx context.Context) (*GateResult, error) {
	result := &GateResult{}

	// ── Build ──────────────────────────────────────────────
	result.BuildPass = g.runBuild(ctx)

	// ── Tests ──────────────────────────────────────────────
	if !g.skipTests {
		testTotal, testFailed, testPass := g.runTests(ctx)
		result.TestTotal = testTotal
		result.TestFailed = testFailed
		result.TestPass = testPass
	} else {
		result.TestPass = true
	}

	// ── Lint / Vet ──────────────────────────────────────────
	result.LintErrors = g.runVet(ctx)

	// ── Git Diff ───────────────────────────────────────────
	diffStats := g.analyzeDiff()
	result.DiffAdded = diffStats.added
	result.DiffRemoved = diffStats.removed
	result.DiffFiles = diffStats.files

	// ── Security Scan ──────────────────────────────────────
	result.SecretsFound = g.scanSecrets(diffStats.diff)
	if result.SecretsFound > 0 {
		result.CriticalSecrets = result.SecretsFound
	}

	// ── Dependency Vulnerabilities (OSV) ──────────────────
	g.scanDependencyVulns(ctx, result)

	// ── TODO/FIXME ─────────────────────────────────────────
	result.TODOsAdded = g.countPattern(diffStats.diff, `TODO`)
	result.FIXMEsAdded = g.countPattern(diffStats.diff, `FIXME`)

	// ── Autonomy from history ──────────────────────────────
	if g.history != nil {
		store := NewAnalyticsStore(g.history)
		agg := &AggregateAnalytics{}
		plans, _ := g.history.ListPlans()
		for _, pid := range plans {
			a, err := store.Load(pid)
			if err == nil {
				agg.Add(a)
			}
		}
		result.AgentsInvolved = agg.TotalPlans
		result.LLMCalls = agg.LLMCalls
		result.AutoRecoveries = agg.RecoveriesSucceeded
		result.AutonomyScore = agg.AvgAutonomyScore
	}

	// ── Compute Scores ─────────────────────────────────────
	g.computeScores(result)

	// Hard gates
	blockers := []string{}
	if !result.BuildPass {
		blockers = append(blockers, "BUILD FAILED")
	}
	if result.TestFailed > 0 {
		blockers = append(blockers, fmt.Sprintf("%d TEST FAILURES", result.TestFailed))
	}
	if result.CriticalSecrets > 0 {
		blockers = append(blockers, fmt.Sprintf("%d SECRETS EXPOSED", result.CriticalSecrets))
	}
	if result.CriticalHighVulns > 0 {
		blockers = append(blockers, fmt.Sprintf("%d CRITICAL/HIGH DEPENDENCY VULNERABILITIES", result.CriticalHighVulns))
	}
	if result.LintErrors > 0 {
		blockers = append(blockers, fmt.Sprintf("%d LINT ERRORS", result.LintErrors))
	}

	if len(blockers) > 0 {
		result.Ready = false
		result.Verdict = fmt.Sprintf("BLOCKED: %s", strings.Join(blockers, ", "))
	} else {
		result.Ready = true
		result.Verdict = "READY TO COMMIT"
	}

	return result, nil
}

func (g *Gate) runBuild(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = g.workDir
	out, err := cmd.CombinedOutput()
	_ = out
	return err == nil
}

func (g *Gate) runTests(ctx context.Context) (total, failed int, passed bool) {
	cmd := exec.CommandContext(ctx, "go", "test", "./...", "-v", "-count=1", "-timeout=240s")
	cmd.Dir = g.workDir
	out, err := cmd.CombinedOutput()
	output := string(out)

	// Parse test output for counts
	// "ok   pkg  0.123s" = passed
	// "FAIL pkg  0.123s" = failed
	// "--- PASS: TestName"
	// "--- FAIL: TestName"
	passRe := regexp.MustCompile(`--- PASS:`)
	failRe := regexp.MustCompile(`--- FAIL:`)
	okRe := regexp.MustCompile(`^ok\s+`)
	failPkgRe := regexp.MustCompile(`^FAIL\s+`)

	total = len(passRe.FindAllString(output, -1)) + len(failRe.FindAllString(output, -1))
	failed = len(failRe.FindAllString(output, -1))

	// If no test functions found but packages compiled ok, tests passed
	if total == 0 {
		// Check if all packages reported ok
		oks := len(okRe.FindAllString(output, -1))
		fails := len(failPkgRe.FindAllString(output, -1))
		if oks > 0 && fails == 0 {
			passed = true
			return
		}
	}

	passed = err == nil && failed == 0
	return
}

func (g *Gate) runVet(ctx context.Context) int {
	cmd := exec.CommandContext(ctx, "go", "vet", "./...")
	cmd.Dir = g.workDir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0
	}
	// Count lines of output as errors
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	count := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			count++
		}
	}
	return count
}

type diffStats struct {
	added   int
	removed int
	files   int
	diff    string
}

func (g *Gate) analyzeDiff() diffStats {
	cmd := exec.Command("git", "diff", "--stat")
	cmd.Dir = g.workDir
	out, _ := cmd.CombinedOutput()
	statOutput := string(out)

	// Parse --stat output for counts
	var stats diffStats
	re := regexp.MustCompile(`(\d+) files? changed(?:, (\d+) insertions?\(\+\))?(?:, (\d+) deletions?\(\-\))?`)
	matches := re.FindStringSubmatch(statOutput)
	if len(matches) >= 2 {
		stats.files, _ = strconv.Atoi(matches[1])
	}
	if len(matches) >= 3 && matches[2] != "" {
		stats.added, _ = strconv.Atoi(matches[2])
	}
	if len(matches) >= 4 && matches[3] != "" {
		stats.removed, _ = strconv.Atoi(matches[3])
	}

	// Also get full diff for scanning
	cmd2 := exec.Command("git", "diff")
	cmd2.Dir = g.workDir
	diffOut, _ := cmd2.CombinedOutput()
	stats.diff = string(diffOut)

	return stats
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret|token|password|passwd)\s*[:=]\s*['"]?[a-zA-Z0-9_\-\.]{20,}['"]?`),
	regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`-----BEGIN (RSA|EC|OPENSSH|DSA) PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{32,})`),
}

func (g *Gate) scanSecrets(diff string) int {
	count := 0
	for _, pattern := range secretPatterns {
		matches := pattern.FindAllString(diff, -1)
		count += len(matches)
	}
	return count
}

// scanDependencyVulns scans the workspace for known dependency vulnerabilities
// using the OSV database. Network errors do not block the gate (offline-safe):
// they are recorded so the dashboard can surface them, but only actual
// CRITICAL/HIGH findings block the commit.
func (g *Gate) scanDependencyVulns(ctx context.Context, result *GateResult) {
	if g.skipSecurityVulns {
		result.SecurityScanSkipped = true
		return
	}

	scanCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	scanResult, err := security.Scan(scanCtx, security.Options{
		Dir:         g.workDir,
		Recursive:   false,
		MinSeverity: security.SeverityLow,
	})
	if err != nil {
		result.SecurityScanError = true
		return
	}
	if scanResult.NoDependencyFiles {
		return
	}

	result.SecurityVulns = scanResult.TotalVulns
	result.CriticalHighVulns = scanResult.CriticalHigh()
}

func (g *Gate) countPattern(diff, pattern string) int {
	re := regexp.MustCompile(pattern)
	return len(re.FindAllString(diff, -1))
}

func (g *Gate) computeScores(r *GateResult) {
	// Correctness: based on build + tests
	r.Correctness = 100
	if !r.BuildPass {
		r.Correctness -= 50
	}
	if r.TestTotal > 0 {
		r.Correctness -= (r.TestFailed * 100) / r.TestTotal
	}
	if r.Correctness < 0 {
		r.Correctness = 0
	}

	// Security: based on secrets + lint + dependency vulnerabilities
	r.Security = 100
	if r.CriticalSecrets > 0 {
		r.Security = 0 // Hard zero — secrets exposed
	} else if r.CriticalHighVulns > 0 {
		r.Security -= r.CriticalHighVulns * 10 // Dependency vulns weigh heavily
	} else if r.LintErrors > 0 {
		r.Security -= r.LintErrors * 5
	}
	if r.Security < 0 {
		r.Security = 0
	}

	// Maintainability: based on TODO/FIXME + diff size
	r.Maintainability = 100
	r.Maintainability -= r.TODOsAdded * 3
	r.Maintainability -= r.FIXMEsAdded * 5
	if r.DiffFiles > 20 {
		r.Maintainability -= 10
	}
	if r.Maintainability < 0 {
		r.Maintainability = 0
	}

	// Regression: tests pass and no breaking changes
	r.Regression = 100
	if r.TestFailed > 0 {
		r.Regression -= r.TestFailed * 10
	}
	if r.Regression < 0 {
		r.Regression = 0
	}

	// Weighted Quality Score
	r.QualityScore = (r.Correctness*30 + r.Security*25 + r.Maintainability*20 + r.Regression*25) / 100
	if r.QualityScore < 0 {
		r.QualityScore = 0
	}
}

// Display renders the gate result as a dashboard string.
// Uses ASCII-only characters for consistent alignment across all terminals.
func (r *GateResult) Display() string {
	var sb strings.Builder

	// Status markers
	passIcon := "✅"
	failIcon := "❌"

	buildIcon := passIcon
	if !r.BuildPass {
		buildIcon = failIcon
	}
	testIcon := passIcon
	if r.TestFailed > 0 {
		testIcon = failIcon
	}
	lintIcon := passIcon
	if r.LintErrors > 0 {
		lintIcon = failIcon
	}
	secIcon := passIcon
	if r.SecretsFound > 0 {
		secIcon = failIcon
	}
	depsIcon := passIcon
	if r.CriticalHighVulns > 0 {
		depsIcon = failIcon
	}

	// Confidence
	confidence := "🟢 HIGH"
	if r.QualityScore < 80 {
		confidence = "🟡 MED "
	}
	if r.QualityScore < 60 {
		confidence = "🔴 LOW "
	}

	// Verdict
	vIcon := "✅"
	vColor := "READY TO COMMIT"
	if !r.Ready {
		vIcon = "❌"
		vColor = r.Verdict
	}

	// Bars
	barLen := 10
	filled := int(r.AutonomyScore * float64(barLen))
	if filled > barLen {
		filled = barLen
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)

	qBarLen := 20
	qFilled := r.QualityScore * qBarLen / 100
	if qFilled > qBarLen {
		qFilled = qBarLen
	}
	qBar := strings.Repeat("█", qFilled) + strings.Repeat("░", qBarLen-qFilled)

	buildStatus := "PASS"
	if !r.BuildPass {
		buildStatus = "FAIL"
	}
	testStatus := fmt.Sprintf("%d/%d", r.TestTotal-r.TestFailed, r.TestTotal)
	lintStatus := fmt.Sprintf("%d errors", r.LintErrors)
	secStatus := fmt.Sprintf("%d secrets", r.SecretsFound)
	depsStatus := fmt.Sprintf("%d vulns", r.SecurityVulns)
	if r.SecurityScanSkipped {
		depsStatus = "skipped"
	} else if r.SecurityScanError {
		depsStatus = "scan error"
	}

	sb.WriteString(fmt.Sprintf(`
  ═══════════════ COSCA COMMIT GATE ═══════════════

  🏗️ Build        %s  %s
  🧪 Tests        %s  %s
  🔍 Lint         %s  %s
  🔒 Security     %s  %s
  🛡️ Deps         %s  %s

  📝 Diff         +%d / -%d  |  %d files
  💡 TODO         %d new
  ⚠️ FIXME        %d new

  🤖 LLM          %d calls
  🔄 Recovery     %d
  👤 Human        %d interventions
  📊 Autonomy     %s  %3.0f%%

  ─────────── QUALITY SCORE ───────────
  ✅ Correctness  %3d  %s
  🔒 Security     %3d  %s
  🧹 Maint.       %3d  %s
  🔁 Regression   %3d  %s
  ─────────────────────────────────────
  🎯 TOTAL        %3d / 100  %s  %s

  %s  %s
`,
		buildStatus, buildIcon,
		testStatus, testIcon,
		lintStatus, lintIcon,
		secStatus, secIcon,
		depsStatus, depsIcon,
		r.DiffAdded, r.DiffRemoved, r.DiffFiles,
		r.TODOsAdded,
		r.FIXMEsAdded,
		r.LLMCalls,
		r.AutoRecoveries,
		r.HumanInterventions,
		bar, r.AutonomyScore*100,
		r.Correctness, scoreBar(r.Correctness),
		r.Security, scoreBar(r.Security),
		r.Maintainability, scoreBar(r.Maintainability),
		r.Regression, scoreBar(r.Regression),
		r.QualityScore, confidence, qBar,
		vIcon, vColor,
	))

	return sb.String()
}

// scoreBar returns a 10-char ASCII bar for a 0-100 score.
func scoreBar(score int) string {
	n := score / 10
	if n > 10 {
		n = 10
	}
	return strings.Repeat("█", n) + strings.Repeat("░", 10-n)
}

// RunGate is a convenience function that runs the gate and prints the dashboard.
// Returns true if the gate is ready (commit can proceed).
func RunGate(ctx context.Context, workDir string, history *WorkflowHistory, skipSecurityVulns ...bool) bool {
	gate := NewGate(workDir)
	gate.SetHistory(history)
	if len(skipSecurityVulns) > 0 && skipSecurityVulns[0] {
		gate.SetSkipSecurityVulns(true)
	}
	start := time.Now()

	result, err := gate.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Gate failed: %v\n", err)
		return false
	}

	fmt.Print(result.Display())
	fmt.Printf("\n  ⏱  Gate completed in %v\n\n", time.Since(start).Round(time.Millisecond))

	return result.Ready
}
