package cognitive

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// writeFile writes a file into a fake .cosca tree (creating parents).
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newFakeTree builds a minimal cognitive .cosca tree under a temp dir and
// returns its root. Matches DefaultComponents so hashing/verification works.
func newFakeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, ".cosca/framework/KERNEL.md", "# Kernel\n")
	writeFile(t, root, ".cosca/framework/CONSTITUTION.md", "# Constitution\n")
	writeFile(t, root, ".cosca/framework/Cognitive_State_Specification.md", "# Cognitive State\n")
	writeFile(t, root, ".cosca/framework/shared/PROJECT_CONTEXT.md", "# Context\n")
	writeFile(t, root, ".cosca/framework/shared/AUTO_EVOLUTION_PROTOCOL.md", "# Evolution\n")
	writeFile(t, root, ".cosca/framework/knowledge/architecture/arch.md", "# Arch\n")
	writeFile(t, root, ".cosca/framework/knowledge/INDEX.md", "# Index\n")
	writeFile(t, root, ".cosca/knowledge/laws.json", `{"version":1}`)
	writeFile(t, root, ".cosca/memory/project/note.md", "# Note\n")
	writeFile(t, root, ".cosca/memory/decision/d.md", "# Decision\n")
	writeFile(t, root, ".cosca/framework/agents/cosca-kernel/PROMPT.md", "# Prompt\n")
	writeFile(t, root, ".cosca/framework/skills/ai/SKILL.md", "# Skill\n")
	writeFile(t, root, ".cosca/config.yaml", "cache:\n  enabled: true\n")
	return root
}

func TestCreateSnapshot_CreatesManifest(t *testing.T) {
	root := newFakeTree(t)
	snap, err := CreateSnapshot(root)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	if snap.Version != "CV-0001" {
		t.Errorf("expected CV-0001, got %q", snap.Version)
	}
	if snap.ID != snap.Version {
		t.Errorf("ID %q != Version %q", snap.ID, snap.Version)
	}
	if snap.CreatedAt.IsZero() {
		t.Error("CreatedAt must be set")
	}
	if snap.SchemaVersion != schemaVersion {
		t.Errorf("SchemaVersion = %d", snap.SchemaVersion)
	}

	// kernel-docs: 3 top-level + 2 in shared; knowledge: 2; laws: 1;
	// memory: 2; agents: 1; skills: 1; config: 1  => 13 total.
	want := map[string]int{
		"kernel-docs": 5,
		"knowledge":   2,
		"laws":        1,
		"memory":      2,
		"agents":      1,
		"skills":      1,
		"config":      1,
	}
	if snap.FilesAffected != 13 {
		t.Errorf("FilesAffected = %d, want 13", snap.FilesAffected)
	}
	for name, n := range want {
		rec, ok := snap.Components[name]
		if !ok {
			t.Errorf("missing component %q", name)
			continue
		}
		if len(rec.Files) != n {
			t.Errorf("component %q has %d files, want %d", name, len(rec.Files), n)
		}
		if len(rec.Hash) != 64 {
			t.Errorf("component %q hash length = %d, want 64", name, len(rec.Hash))
		}
		if rec.Hash != snap.Hashes[name] {
			t.Errorf("component %q aggregate hash mismatch", name)
		}
	}
	// Hashes must be real sha256 hex.
	for name, h := range snap.Hashes {
		if len(h) != 64 {
			t.Errorf("component %q hash = %q", name, h)
		}
	}

	// Manifest file persisted with 0644 (POSIX only — chmod is a no-op on
	// Windows, so the permission assertion does not apply there).
	if runtime.GOOS != "windows" {
		info, err := os.Stat(snap.ManifestPath)
		if err != nil {
			t.Fatalf("manifest not written: %v", err)
		}
		if got := info.Mode().Perm(); got != 0o644 {
			t.Errorf("manifest mode = %o, want 644", got)
		}
	}
	if filepath.Base(snap.ManifestPath) != "CV-0001.json" {
		t.Errorf("unexpected manifest name: %s", snap.ManifestPath)
	}
}

func TestCreateSnapshot_VersionIncrements(t *testing.T) {
	root := newFakeTree(t)
	if _, err := CreateSnapshot(root); err != nil {
		t.Fatal(err)
	}
	snap2, err := CreateSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if snap2.Version != "CV-0002" {
		t.Errorf("second snapshot = %q, want CV-0002", snap2.Version)
	}

	// Pre-existing higher CV must be respected.
	writeFile(t, root, ".cosca/cv/CV-0007.json", `{"id":"CV-0007","version":"CV-0007"}`)
	snap8, err := CreateSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if snap8.Version != "CV-0008" {
		t.Errorf("after CV-0007, next = %q, want CV-0008", snap8.Version)
	}
}

func TestCreateSnapshot_ConcurrentClaims(t *testing.T) {
	root := newFakeTree(t)
	const n = 8
	var wg sync.WaitGroup
	versions := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			snap, err := CreateSnapshot(root)
			if err != nil {
				errs[i] = err
				return
			}
			versions[i] = snap.Version
		}(i)
	}
	wg.Wait()

	seen := map[string]bool{}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if versions[i] == "" {
			t.Fatal("empty version returned")
		}
		if seen[versions[i]] {
			t.Errorf("duplicate version claimed: %s", versions[i])
		}
		seen[versions[i]] = true
	}
	list, err := ListSnapshots(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != n {
		t.Errorf("expected %d snapshots on disk, got %d", n, len(list))
	}
}

func TestListSnapshots(t *testing.T) {
	root := newFakeTree(t)

	// Empty cv dir → empty list, no error.
	got, err := ListSnapshots(root)
	if err != nil {
		t.Fatalf("ListSnapshots (empty): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no snapshots, got %d", len(got))
	}

	for i := 0; i < 3; i++ {
		if _, err := CreateSnapshot(root); err != nil {
			t.Fatal(err)
		}
	}
	got, err = ListSnapshots(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(got))
	}
	if got[0].Version != "CV-0001" || got[2].Version != "CV-0003" {
		t.Errorf("unsorted list: %s, %s, %s", got[0].Version, got[1].Version, got[2].Version)
	}
}

func TestVerifySnapshot_Matches(t *testing.T) {
	root := newFakeTree(t)
	if _, err := CreateSnapshot(root); err != nil {
		t.Fatal(err)
	}
	ok, report, err := VerifySnapshot(root, "CV-0001")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected match, report: %+v", report)
	}
	if !report.Matches {
		t.Error("report.Matches must be true")
	}
	for name, st := range report.Components {
		if !st.Match {
			t.Errorf("component %q should match", name)
		}
	}
}

func TestVerifySnapshot_DetectsChange(t *testing.T) {
	root := newFakeTree(t)
	if _, err := CreateSnapshot(root); err != nil {
		t.Fatal(err)
	}

	// Modify one file inside the memory component.
	writeFile(t, root, ".cosca/memory/project/note.md", "# Note — ALTERADO\n")

	ok, report, err := VerifySnapshot(root, "CV-0001")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected verification to fail after modification")
	}
	if report.Matches {
		t.Error("report.Matches must be false after modification")
	}
	mem := report.Components["memory"]
	if mem.Match {
		t.Error("memory component must be reported as changed")
	}
	found := false
	for _, rel := range mem.Changed {
		// Normalize to forward slashes so the comparison works on Windows
		// (where rel uses backslashes).
		if strings.Contains(filepath.ToSlash(rel), ".cosca/memory/project/note.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("changed list missing the modified file: %v", mem.Changed)
	}

	// A fresh snapshot after the change verifies clean.
	if _, err := CreateSnapshot(root); err != nil {
		t.Fatal(err)
	}
	ok2, _, err := VerifySnapshot(root, "CV-0002")
	if err != nil {
		t.Fatal(err)
	}
	if !ok2 {
		t.Error("fresh snapshot should verify clean")
	}
}

func TestVerifySnapshot_DetectsAddedFile(t *testing.T) {
	root := newFakeTree(t)
	if _, err := CreateSnapshot(root); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, ".cosca/framework/knowledge/patterns/new.md", "# New\n")

	ok, report, err := VerifySnapshot(root, "CV-0001")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected verification to fail after adding a file")
	}
	know := report.Components["knowledge"]
	if know.Match {
		t.Error("knowledge component must be reported as changed")
	}
	if len(know.Added) != 1 {
		t.Errorf("expected 1 added file, got %v", know.Added)
	}
}

func TestVerifySnapshot_DetectsRemovedFile(t *testing.T) {
	root := newFakeTree(t)
	if _, err := CreateSnapshot(root); err != nil {
		t.Fatal(err)
	}
	removed := filepath.Join(root, ".cosca", "framework", "agents", "cosca-kernel", "PROMPT.md")
	if err := os.Remove(removed); err != nil {
		t.Fatal(err)
	}

	ok, report, err := VerifySnapshot(root, "CV-0001")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected verification to fail after removing a file")
	}
	agents := report.Components["agents"]
	if agents.Match {
		t.Error("agents component must be reported as changed")
	}
	if len(agents.Removed) != 1 {
		t.Errorf("expected 1 removed file, got %v", agents.Removed)
	}
}

func TestVerifySnapshot_UnknownVersion(t *testing.T) {
	root := newFakeTree(t)
	if _, err := CreateSnapshot(root); err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifySnapshot(root, "CV-0099"); err == nil {
		t.Error("expected error for unknown version")
	}
}

func TestRollbackSnapshot_DoesNotOverwrite(t *testing.T) {
	root := newFakeTree(t)
	if _, err := CreateSnapshot(root); err != nil {
		t.Fatal(err)
	}
	// Regress the state, then request rollback to the intact CV-0001.
	writeFile(t, root, ".cosca/framework/shared/PROJECT_CONTEXT.md", "# Context — QUEBRADO\n")
	before, _ := os.ReadFile(filepath.Join(root, ".cosca", "framework", "shared", "PROJECT_CONTEXT.md"))

	err := RollbackSnapshot(root, "CV-0001")
	if err == nil {
		t.Fatal("expected a *RollbackError with the guide")
	}
	rerr, ok := err.(*RollbackError)
	if !ok {
		t.Fatalf("expected *RollbackError, got %T", err)
	}
	if rerr.Guide == nil {
		t.Fatal("guide must not be nil")
	}
	if rerr.Guide.Report == nil || rerr.Guide.Report.Matches {
		t.Error("report must describe the divergence")
	}

	// The broken file must NOT have been rewritten.
	after, _ := os.ReadFile(filepath.Join(root, ".cosca", "framework", "shared", "PROJECT_CONTEXT.md"))
	if string(before) != string(after) {
		t.Fatal("rollback must NEVER overwrite files automatically")
	}

	// A marker snapshot of the current state must have been recorded.
	if rerr.Guide.Snapshot == nil {
		t.Fatal("expected a marker snapshot before rollback guidance")
	}
	if rerr.Guide.Snapshot.Version == "" || rerr.Guide.Snapshot.Version == "CV-0001" {
		t.Errorf("marker snapshot version = %q", rerr.Guide.Snapshot.Version)
	}
	if len(rerr.Guide.GitCommands) == 0 {
		t.Error("guide must contain git instructions")
	}
	joined := strings.Join(rerr.Guide.GitCommands, "\n")
	if !strings.Contains(joined, "git revert") && !strings.Contains(joined, "git checkout") {
		t.Errorf("guide should mention git restore: %s", joined)
	}

	// Rollback to a version that already matches must not rewrite either.
	ok2, _, err := VerifySnapshot(root, rerr.Guide.Snapshot.Version)
	if err != nil || !ok2 {
		t.Fatalf("marker snapshot should verify clean: ok=%v err=%v", ok2, err)
	}
	err2 := RollbackSnapshot(root, rerr.Guide.Snapshot.Version)
	if err2 == nil {
		t.Fatal("expected a guide for the already-matching version too")
	}
	if _, ok := err2.(*RollbackError); !ok {
		t.Fatalf("expected *RollbackError, got %T", err2)
	}
}

func TestNormalizeVersion(t *testing.T) {
	cases := map[string]string{
		"CV-0003":  "CV-0003",
		"CV-3":     "CV-0003",
		"0003":     "CV-0003",
		"cv-3":     "CV-0003",
		" CV-123 ": "CV-0123",
	}
	for in, want := range cases {
		got, err := NormalizeVersion(in)
		if err != nil {
			t.Errorf("NormalizeVersion(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeVersion(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "abc", "CV-X", "-3", "CV-" + time.Now().String()} {
		if _, err := NormalizeVersion(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestSnapshotManifest_RoundTrip(t *testing.T) {
	root := newFakeTree(t)
	snap, err := CreateSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSnapshot(snap.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != snap.Version || loaded.FilesAffected != snap.FilesAffected {
		t.Errorf("round-trip mismatch: %+v vs %+v", loaded, snap)
	}
	for name, rec := range snap.Components {
		lrec, ok := loaded.Components[name]
		if !ok || lrec.Hash != rec.Hash || len(lrec.Files) != len(rec.Files) {
			t.Errorf("round-trip mismatch in component %q", name)
		}
	}
}
