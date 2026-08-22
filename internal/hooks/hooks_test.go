package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// =============================================================================
// Helpers
// =============================================================================

// gitCmd runs a git command inside dir and fails the test on error.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// initRepo creates a fresh git repository in a temp dir with commit identity.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "kernel@cosca.local")
	gitCmd(t, dir, "config", "user.name", "Cosca Kernel")
	return dir
}

// commitFile writes a file and commits it with the given message.
func commitFile(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", msg)
}

// =============================================================================
// InstallHooks
// =============================================================================

func TestInstallHooks(t *testing.T) {
	dir := initRepo(t)

	installed, skipped, err := InstallHooks(dir)
	if err != nil {
		t.Fatalf("InstallHooks returned error: %v", err)
	}
	if installed != 1 {
		t.Errorf("expected 1 installed hook, got %d", installed)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped hooks, got %d", skipped)
	}

	hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("post-commit hook not installed: %v", err)
	}
	// POSIX-only: the executable bit is not enforced on Windows (chmod is a
	// no-op and a writable file reports 0666). Git for Windows runs hooks via
	// sh regardless of the exec bit.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Errorf("hook is not executable: %v", info.Mode())
	}

	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	if !strings.Contains(string(data), hookMarker) {
		t.Error("hook script missing cosca marker")
	}
	if !strings.Contains(string(data), "hook post-commit") {
		t.Error("hook script does not invoke `cosca hook post-commit`")
	}
}

func TestInstallHooks_SecondRunSkips(t *testing.T) {
	dir := initRepo(t)

	if _, _, err := InstallHooks(dir); err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	installed, skipped, err := InstallHooks(dir)
	if err != nil {
		t.Fatalf("second install returned error: %v", err)
	}
	if installed != 0 {
		t.Errorf("expected 0 installed on re-run, got %d", installed)
	}
	if skipped != 1 {
		t.Errorf("expected 1 skipped on re-run, got %d", skipped)
	}
}

func TestInstallHooks_BacksUpExistingHook(t *testing.T) {
	dir := initRepo(t)

	hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write foreign hook: %v", err)
	}

	installed, skipped, err := InstallHooks(dir)
	if err != nil {
		t.Fatalf("InstallHooks returned error: %v", err)
	}
	if installed != 1 {
		t.Errorf("expected 1 installed hook, got %d", installed)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped hooks, got %d", skipped)
	}

	// Foreign hook must have been backed up.
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(hookPath), "post-commit.bak.*"))
	if err != nil || len(matches) == 0 {
		t.Errorf("expected backup of foreign hook, matches=%v err=%v", matches, err)
	}
}

func TestInstallHooks_UpgradesOutdatedOwnHook(t *testing.T) {
	dir := initRepo(t)

	hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	// An older cosca-generated hook pointing at a different binary.
	stale := "#!/bin/sh\n# " + hookMarker + "\nexec '/old/path/cosca' hook post-commit\n"
	if err := os.WriteFile(hookPath, []byte(stale), 0o755); err != nil {
		t.Fatalf("write stale hook: %v", err)
	}

	installed, skipped, err := InstallHooks(dir)
	if err != nil {
		t.Fatalf("InstallHooks returned error: %v", err)
	}
	if installed != 1 {
		t.Errorf("expected 1 upgraded hook, got %d", installed)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped hooks, got %d", skipped)
	}

	// No backup should exist for our own (outdated) hook.
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(hookPath), "post-commit.bak.*"))
	if len(matches) != 0 {
		t.Errorf("expected no backup for cosca-owned hook, got %v", matches)
	}
}

func TestInstallHooks_NotARepository(t *testing.T) {
	dir := t.TempDir()

	installed, skipped, err := InstallHooks(dir)
	if err == nil {
		t.Error("expected error when installing hooks in a non-repo dir")
	}
	if installed != 0 || skipped != 0 {
		t.Errorf("expected no-op counts, got installed=%d skipped=%d", installed, skipped)
	}
}

func TestGitDirFor_WorktreeGitFile(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, "custom-git-dir")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir git dir: %v", err)
	}
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatalf("mkdir work dir: %v", err)
	}
	// Simulate a worktree .git file pointing at the real git dir.
	if err := os.WriteFile(filepath.Join(work, ".git"), []byte("gitdir: "+gitDir), 0o644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	got, err := gitDirFor(work)
	if err != nil {
		t.Fatalf("gitDirFor returned error: %v", err)
	}
	if got != gitDir {
		t.Errorf("gitDirFor = %q, want %q", got, gitDir)
	}
}

// =============================================================================
// GenerateImpactReport
// =============================================================================

func TestGenerateImpactReport(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "main.go", "package main\n\nfunc main() {}\n", "feat: initial commit")

	report, err := GenerateImpactReport(dir)
	if err != nil {
		t.Fatalf("GenerateImpactReport returned error: %v", err)
	}
	if report == "" {
		t.Fatal("expected non-empty report path")
	}

	// Report file must exist at .cosca/memory/timeline/impact-reports/.
	if _, err := os.Stat(report); err != nil {
		t.Fatalf("report file not found: %v", err)
	}
	if !strings.Contains(report, filepath.Join(".cosca", "memory", "timeline", "impact-reports")) {
		t.Errorf("report path %q not under .cosca/memory/timeline/impact-reports", report)
	}

	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"type: impact-report",
		"agent: cosca-kernel",
		"# Impact Report —",
		"feat: initial commit",
		"| Commit |",
		"| Mensagem |",
		"| Autor |",
		"| Data |",
		"| Arquivos | 1 |",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("report missing %q\n---\n%s", want, content)
		}
	}

	// Engineering timeline must have been created.
	timeline := filepath.Join(dir, ".cosca", "memory", "timeline", "ENGINEERING_TIMELINE.md")
	td, err := os.ReadFile(timeline)
	if err != nil {
		t.Fatalf("engineering timeline not created: %v", err)
	}
	if !strings.Contains(string(td), "feat: initial commit") {
		t.Errorf("timeline missing commit message:\n%s", td)
	}
}

func TestGenerateImpactReport_DeletionOnlyCommit(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "line1\nline2\nline3\n", "chore: seed")
	// Remove a file — deletion-only commit.
	gitCmd(t, dir, "rm", "a.txt")
	gitCmd(t, dir, "commit", "-m", "chore: delete file")

	report, err := GenerateImpactReport(dir)
	if err != nil {
		t.Fatalf("GenerateImpactReport returned error: %v", err)
	}
	if report == "" {
		t.Fatal("expected non-empty report path")
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "| Arquivos | 1 |") {
		t.Errorf("expected 1 changed file, got:\n%s", content)
	}
	if !strings.Contains(content, "| +0/-3 |") {
		t.Errorf("expected +0/-3 stats row, got:\n%s", content)
	}
}

func TestGenerateImpactReport_Idempotent(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "app.go", "package app\n", "fix: app bootstrap")

	first, err := GenerateImpactReport(dir)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	second, err := GenerateImpactReport(dir)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if first == "" || second == "" {
		t.Fatalf("expected non-empty paths, got %q and %q", first, second)
	}
	if first != second {
		t.Errorf("expected same report path on re-run, got %q vs %q", first, second)
	}

	// No duplicate timeline entries for the same commit.
	timeline := filepath.Join(dir, ".cosca", "memory", "timeline", "ENGINEERING_TIMELINE.md")
	td, err := os.ReadFile(timeline)
	if err != nil {
		t.Fatalf("read timeline: %v", err)
	}
	hash := strings.TrimSuffix(filepath.Base(first), ".md")
	if count := strings.Count(string(td), "`"+hash+"`"); count != 1 {
		t.Errorf("expected exactly 1 timeline entry for %s, found %d:\n%s", hash, count, td)
	}
}

func TestGenerateImpactReport_SkipsMemoryOnlyCommits(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "code.go", "package code\n", "feat: seed")

	// Commit that only touches .cosca/ (the hook's own output home).
	commitFile(t, dir, ".cosca/memory/timeline/impact-reports/note.md", "# note\n", "chore: report commit")

	report, err := GenerateImpactReport(dir)
	if err != nil {
		t.Fatalf("GenerateImpactReport returned error: %v", err)
	}
	if report != "" {
		t.Errorf("expected memory-only commit to be skipped, got report %q", report)
	}
}

func TestGenerateImpactReport_NotARepository(t *testing.T) {
	dir := t.TempDir()

	report, err := GenerateImpactReport(dir)
	if err == nil {
		t.Error("expected error for non-repo dir")
	}
	if report != "" {
		t.Errorf("expected empty report path on error, got %q", report)
	}
}

func TestMemoryOnlyCommit(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  bool
	}{
		{"empty", nil, false},
		{"code only", []string{"main.go"}, false},
		{"mixed", []string{".cosca/x.md", "main.go"}, false},
		{"dotcosca dir itself", []string{".cosca"}, true},
		{"report file", []string{".cosca/memory/timeline/impact-reports/abc.md"}, true},
		{"timeline", []string{".cosca/memory/timeline/ENGINEERING_TIMELINE.md"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := memoryOnlyCommit(tt.files); got != tt.want {
				t.Errorf("memoryOnlyCommit(%v) = %v, want %v", tt.files, got, tt.want)
			}
		})
	}
}

func TestChangeType(t *testing.T) {
	tests := []struct {
		msg  string
		want string
	}{
		{"feat: add hooks", "feature"},
		{"FIX: uppercase", "bug-fix"},
		{"test: unit", "testing"},
		{"docs: readme", "documentation"},
		{"refactor: internals", "refactoring"},
		{"chore: deps", "chore"},
		{"learn: pattern", "learning"},
		{"perf: cache", "performance"},
		{"sec: auth", "security"},
		{"ci: pipeline", "ci"},
		{"arch: boundaries", "architecture"},
		{"release: v1", "release"},
		{"no prefix at all", "other"},
	}
	for _, tt := range tests {
		if got := changeType(tt.msg); got != tt.want {
			t.Errorf("changeType(%q) = %q, want %q", tt.msg, got, tt.want)
		}
	}
}
