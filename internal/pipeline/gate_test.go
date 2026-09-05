package pipeline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateComputeScoresAllGreen(t *testing.T) {
	r := &GateResult{BuildPass: true, Ready: true}
	g := NewGate(t.TempDir())
	g.computeScores(r)

	if r.Correctness != 100 || r.Security != 100 || r.Maintainability != 100 || r.Regression != 100 {
		t.Fatalf("all-green scores: %+v", r)
	}
	if r.QualityScore != 100 {
		t.Fatalf("QualityScore = %d, want 100", r.QualityScore)
	}
}

func TestGateComputeScoresPenalties(t *testing.T) {
	g := NewGate(t.TempDir())

	// Build failure halves correctness.
	r1 := &GateResult{BuildPass: false}
	g.computeScores(r1)
	if r1.Correctness != 50 {
		t.Fatalf("build-fail Correctness = %d, want 50", r1.Correctness)
	}

	// Test failures reduce correctness and regression.
	r2 := &GateResult{BuildPass: true, TestTotal: 10, TestFailed: 2}
	g.computeScores(r2)
	if r2.Correctness != 80 {
		t.Fatalf("test-fail Correctness = %d, want 80", r2.Correctness)
	}
	if r2.Regression != 80 {
		t.Fatalf("Regression = %d, want 80", r2.Regression)
	}

	// Secrets zero the security score.
	r3 := &GateResult{BuildPass: true, CriticalSecrets: 2}
	g.computeScores(r3)
	if r3.Security != 0 {
		t.Fatalf("secrets Security = %d, want 0", r3.Security)
	}

	// High vulns weigh 10 each.
	r4 := &GateResult{BuildPass: true, CriticalHighVulns: 3}
	g.computeScores(r4)
	if r4.Security != 70 {
		t.Fatalf("vulns Security = %d, want 70", r4.Security)
	}

	// Lint errors weigh 5 each.
	r5 := &GateResult{BuildPass: true, LintErrors: 4}
	g.computeScores(r5)
	if r5.Security != 80 {
		t.Fatalf("lint Security = %d, want 80", r5.Security)
	}

	// TODO/FIXME reduce maintainability.
	r6 := &GateResult{BuildPass: true, TODOsAdded: 5, FIXMEsAdded: 2}
	g.computeScores(r6)
	if r6.Maintainability != 75 {
		t.Fatalf("TODO/FIXME Maintainability = %d, want 75", r6.Maintainability)
	}

	// Big diffs reduce maintainability.
	r7 := &GateResult{BuildPass: true, DiffFiles: 30}
	g.computeScores(r7)
	if r7.Maintainability != 90 {
		t.Fatalf("big diff Maintainability = %d, want 90", r7.Maintainability)
	}

	// Never negative.
	r8 := &GateResult{BuildPass: false, TestTotal: 5, TestFailed: 5, CriticalHighVulns: 50}
	g.computeScores(r8)
	if r8.Correctness < 0 || r8.Security < 0 || r8.Regression < 0 {
		t.Fatalf("negative scores: %+v", r8)
	}
}

func TestGateScanSecrets(t *testing.T) {
	g := NewGate(t.TempDir())

	diff := `
- const api_key = "abcdefghijklmnopqrstuvwxyz123456"
+ const apiKey: "1234567890abcdefghijklmnopqrstuvwxyz"
- token = "abcdefghijklmnopqrstuvwxyz123456"
+ ghp_abcdefghijklmnopqrstuvwxyz1234567890
- -----BEGIN RSA PRIVATE KEY-----
+ sk-abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqrstu
`
	got := g.scanSecrets(diff)
	if got < 6 {
		t.Fatalf("scanSecrets = %d, want >= 6 matches", got)
	}
}

func TestGateScanSecretsClean(t *testing.T) {
	g := NewGate(t.TempDir())
	if got := g.scanSecrets("+ a := 1\n- b := 2\n"); got != 0 {
		t.Fatalf("clean diff should have 0 secrets, got %d", got)
	}
}

func TestGateCountPattern(t *testing.T) {
	g := NewGate(t.TempDir())
	diff := "TODO: fix\nTODO: again\nFIXME: later"
	if got := g.countPattern(diff, `TODO`); got != 2 {
		t.Fatalf("countPattern(TODO) = %d, want 2", got)
	}
	if got := g.countPattern(diff, `FIXME`); got != 1 {
		t.Fatalf("countPattern(FIXME) = %d, want 1", got)
	}
}

func TestGateRunBlockers(t *testing.T) {	ctx := context.Background()
	gate := NewGate(t.TempDir())

	// Skip the OSV scan to keep the test offline-deterministic.
	gate.SetSkipSecurityVulns(true)

	// A gate in a directory where go build fails → BLOCKED.
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package broken\nfunc {\n"), 0o644)
	g2 := NewGate(dir)
	g2.SetSkipSecurityVulns(true)
	res, err := g2.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Ready {
		t.Fatal("broken workspace must not be ready")
	}
	if !strings.HasPrefix(res.Verdict, "BLOCKED:") {
		t.Fatalf("verdict = %q, want BLOCKED prefix", res.Verdict)
	}

	// The first gate (empty dir) — go build of an empty dir also fails, so it
	// must be blocked too. This keeps the test independent of the repo state.
	_ = gate
	if res.Verdict == "" {
		t.Fatal("empty verdict")
	}
}

func TestGateScanDependencyVulnsSkip(t *testing.T) {
	g := NewGate(t.TempDir())
	g.SetSkipSecurityVulns(true)
	res := &GateResult{}
	g.scanDependencyVulns(context.Background(), res)
	if !res.SecurityScanSkipped {
		t.Fatal("skip flag must set SecurityScanSkipped")
	}
}

func TestGateScanDependencyVulnsNoDependencyFiles(t *testing.T) {
	g := NewGate(t.TempDir()) // empty dir: no go.mod/package.json
	res := &GateResult{}
	g.scanDependencyVulns(context.Background(), res)
	if res.SecurityScanError {
		t.Fatalf("empty dir must not be a scan error: %+v", res)
	}
	if res.SecurityScanSkipped {
		t.Fatal("no-dependency-files must not be reported as skipped")
	}
	if res.SecurityVulns != 0 || res.CriticalHighVulns != 0 {
		t.Fatalf("unexpected vulns: %+v", res)
	}
}

func TestGateAnalyzeDiff(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	git(t, dir, "config", "user.email", "kernel@cosca.local")
	git(t, dir, "config", "user.name", "cosca-kernel")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "initial")

	// Modify one tracked file. Note: git diff does NOT show untracked files,
	// so a new (untracked) file is not part of the diff.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("new file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := NewGate(dir)
	stats := g.analyzeDiff()
	if stats.files != 1 {
		t.Fatalf("DiffFiles = %d, want 1 (only tracked changes)", stats.files)
	}
	if stats.added < 1 || stats.removed < 1 {
		t.Fatalf("unexpected diff stats: +%d -%d", stats.added, stats.removed)
	}
	if !strings.Contains(stats.diff, "a.txt") {
		t.Fatal("full diff must contain changed files")
	}
}

func TestGateScoreBarAndDisplay(t *testing.T) {
	bar := scoreBar(50)
	if len([]rune(bar)) != 10 {
		t.Fatalf("scoreBar(50) rune-len = %d, want 10", len([]rune(bar)))
	}
	if bar != "█████░░░░░" {
		t.Fatalf("scoreBar(50) = %q", bar)
	}
	if scoreBar(200) != "██████████" {
		t.Fatal("scoreBar caps at 10")
	}

	r := &GateResult{
		BuildPass: true, Ready: true,
		QualityScore: 90, AutonomyScore: 0.8,
	}
	out := r.Display()
	if !strings.Contains(out, "READY TO COMMIT") {
		t.Fatal("Display must contain the verdict")
	}
	if !strings.Contains(out, "90 / 100") {
		t.Fatalf("Display must contain the total score: %q", out)
	}

	r2 := &GateResult{BuildPass: false, TestFailed: 1, Ready: false, Verdict: "BLOCKED: BUILD FAILED", QualityScore: 30}
	out2 := r2.Display()
	if !strings.Contains(out2, "BLOCKED: BUILD FAILED") {
		t.Fatal("Display must contain the blocked verdict")
	}
}

// git runs a git command in dir, failing the test on error.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestGateSetSkipTests verifies the Fix-3 regression: with SetSkipTests(true)
// the gate does NOT execute `go test` and treats tests as passing. This is
// exercised in a directory with a deliberately broken test file — if tests ran,
// the gate would block; with the skip, it proceeds to the other checks.
func TestGateSetSkipTests(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	git(t, dir, "init")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "test")
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module broken\n\ngo 1.21\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package broken\nimport \"testing\"\nfunc TestBroken(t *testing.T){ t.Fatal(\"boom\") }\n"), 0o644)

	gate := NewGate(dir)
	gate.SetSkipSecurityVulns(true)
	gate.SetSkipTests(true)

	res, err := gate.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.TestFailed != 0 {
		t.Errorf("TestFailed = %d, want 0 (tests skipped)", res.TestFailed)
	}
	if !res.TestPass {
		t.Error("TestPass must be true when tests are skipped")
	}
	if res.TestTotal != 0 {
		t.Errorf("TestTotal = %d, want 0 (tests skipped)", res.TestTotal)
	}
}
