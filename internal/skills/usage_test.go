// Tests for the skill usage telemetry + curator (internal/skills/usage.go).
//
// Covers:
//   - RecordUse increments + sets timestamp; atomic file (no .tmp left)
//   - Snapshot round-trip
//   - StaleSince: used today → not stale; used 31+ days ago → stale;
//     never used → stale after threshold
//   - Archive: moves local dir/file to .archive/ and marks archived;
//     embedded skill → marked only, no panic; Archive NEVER deletes
//   - Restore: moves the source back and re-activates
//   - MarkStale

package skills

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// newTestUsageStore cria um UsageStore sobre um .cosca temporário.
func newTestUsageStore(t *testing.T) (*UsageStore, string) {
	t.Helper()
	coscaDir := filepath.Join(t.TempDir(), ".cosca")
	store, err := NewUsageStore(coscaDir)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}
	return store, coscaDir
}

func TestNewUsageStore_CreatesParent(t *testing.T) {
	store, _ := newTestUsageStore(t)
	if store == nil {
		t.Fatal("store is nil")
	}
	if store.Path() != filepath.Join(store.coscaDir, "skills", usageFileName) {
		t.Errorf("unexpected usage path: %q", store.Path())
	}
	// o diretório pai (e o .archive) devem ser resolvíveis
	if _, err := os.Stat(filepath.Dir(store.Path())); err != nil {
		t.Errorf("parent dir should be created: %v", err)
	}
	if store.ArchiveDir() != filepath.Join(store.coscaDir, "skills", ".archive") {
		t.Errorf("unexpected archive dir: %q", store.ArchiveDir())
	}
}

func TestRecordUse_IncrementsAndWritesAtomically(t *testing.T) {
	store, _ := newTestUsageStore(t)

	if err := store.RecordUse("foo-skill"); err != nil {
		t.Fatalf("RecordUse: %v", err)
	}
	if err := store.RecordUse("foo-skill"); err != nil {
		t.Fatalf("RecordUse #2: %v", err)
	}

	// arquivo atômico existe e não sobrou .tmp
	if _, err := os.Stat(store.Path()); err != nil {
		t.Fatalf("usage file not written: %v", err)
	}
	if _, err := os.Stat(store.Path() + ".tmp"); !os.IsNotExist(err) {
		t.Error("tmp file should be cleaned up (atomic tmp + rename)")
	}

	u, err := store.Get("foo-skill")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if u.UseCount != 2 {
		t.Errorf("UseCount = %d, want 2", u.UseCount)
	}
	if u.LastActivityAt == "" {
		t.Error("LastActivityAt should be set")
	}
	if u.State != StateActive {
		t.Errorf("State = %q, want active", u.State)
	}

	// permissão do arquivo: 0600 (POSIX only — chmod is a no-op on Windows)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(store.Path())
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("file mode = %o, want 600", got)
		}
	}
}

func TestSnapshot_RoundTrip(t *testing.T) {
	store, _ := newTestUsageStore(t)
	want := map[string]SkillUsage{
		"a": {SkillName: "a", UseCount: 3, LastActivityAt: "2026-07-01T00:00:00Z", State: StateActive},
		"b": {SkillName: "b", UseCount: 1, State: StateActive},
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip mismatch:\ngot  %#v\nwant %#v", got, want)
	}
}

func TestSnapshot_NoFileReturnsEmpty(t *testing.T) {
	store, _ := newTestUsageStore(t)
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot on missing file: %v", err)
	}
	if len(snap) != 0 {
		t.Errorf("expected empty snapshot, got %d entries", len(snap))
	}
}

func TestStaleSince(t *testing.T) {
	t.Run("used today not stale", func(t *testing.T) {
		store, _ := newTestUsageStore(t)
		if err := store.RecordUse("fresh"); err != nil {
			t.Fatal(err)
		}
		stale, err := store.StaleSince(30)
		if err != nil {
			t.Fatal(err)
		}
		if len(stale) != 0 {
			t.Errorf("expected 0 stale, got %d: %+v", len(stale), stale)
		}
	})

	t.Run("used 31 days ago stale", func(t *testing.T) {
		store, _ := newTestUsageStore(t)
		old := time.Now().UTC().AddDate(0, 0, -31).Format(time.RFC3339)
		if err := store.Save(map[string]SkillUsage{
			"rusty": {SkillName: "rusty", UseCount: 5, LastActivityAt: old, State: StateActive},
			"kept":  {SkillName: "kept", UseCount: 1, LastActivityAt: time.Now().UTC().Format(time.RFC3339), State: StateActive},
		}); err != nil {
			t.Fatal(err)
		}
		stale, err := store.StaleSince(30)
		if err != nil {
			t.Fatal(err)
		}
		if len(stale) != 1 || stale[0].SkillName != "rusty" {
			t.Errorf("expected only rusty stale, got %+v", stale)
		}
	})

	t.Run("never used stale after threshold", func(t *testing.T) {
		store, _ := newTestUsageStore(t)
		if err := store.Save(map[string]SkillUsage{
			"ghost": {SkillName: "ghost", State: StateActive}, // sem timestamp
		}); err != nil {
			t.Fatal(err)
		}
		// simulando telemetria criada há 31 dias (referência da "nunca usada")
		past := time.Now().AddDate(0, 0, -31)
		if err := os.Chtimes(store.Path(), past, past); err != nil {
			t.Fatal(err)
		}
		stale, err := store.StaleSince(30)
		if err != nil {
			t.Fatal(err)
		}
		if len(stale) != 1 || stale[0].SkillName != "ghost" {
			t.Errorf("expected ghost stale, got %+v", stale)
		}
	})

	t.Run("never used not stale when tracking is fresh", func(t *testing.T) {
		store, _ := newTestUsageStore(t)
		if err := store.Save(map[string]SkillUsage{
			"ghost": {SkillName: "ghost", State: StateActive},
		}); err != nil {
			t.Fatal(err)
		}
		stale, err := store.StaleSince(30)
		if err != nil {
			t.Fatal(err)
		}
		if len(stale) != 0 {
			t.Errorf("expected 0 stale for fresh tracking, got %+v", stale)
		}
	})

	t.Run("archived and stale states are excluded", func(t *testing.T) {
		store, _ := newTestUsageStore(t)
		old := time.Now().UTC().AddDate(0, 0, -60).Format(time.RFC3339)
		if err := store.Save(map[string]SkillUsage{
			"arch":  {SkillName: "arch", LastActivityAt: old, State: StateArchived},
			"staly": {SkillName: "staly", LastActivityAt: old, State: StateStale},
		}); err != nil {
			t.Fatal(err)
		}
		stale, err := store.StaleSince(30)
		if err != nil {
			t.Fatal(err)
		}
		if len(stale) != 0 {
			t.Errorf("archived/stale must not be candidates: %+v", stale)
		}
	})
}

func TestArchive_MovesLocalDir(t *testing.T) {
	store, coscaDir := newTestUsageStore(t)

	// skill local como DIRETÓRIO .cosca/skills/my-skill/
	dir := filepath.Join(coscaDir, "skills", "my-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# my-skill\n\n## Description\nLocal skill\n"
	if err := os.WriteFile(filepath.Join(dir, "my-skill.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordUse("my-skill"); err != nil {
		t.Fatal(err)
	}

	if err := store.Archive("my-skill"); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	// fonte movida para .archive/my-skill/ e NUNCA deletada
	archivedDir := filepath.Join(store.ArchiveDir(), "my-skill")
	if _, err := os.Stat(archivedDir); err != nil {
		t.Fatalf("archived dir must exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archivedDir, "my-skill.md")); err != nil {
		t.Errorf("archived content missing (never deleted): %v", err)
	}
	// sumiu do local
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("local dir should have moved away (err=%v)", err)
	}

	u, err := store.Get("my-skill")
	if err != nil {
		t.Fatal(err)
	}
	if u.State != StateArchived {
		t.Errorf("State = %q, want archived", u.State)
	}
}

func TestArchive_MovesLocalFile(t *testing.T) {
	store, coscaDir := newTestUsageStore(t)

	// skill local como ARQUIVO .cosca/skills/other-skill.md
	md := filepath.Join(coscaDir, "skills", "other-skill.md")
	if err := os.WriteFile(md, []byte("# other-skill\n\n## Description\nX\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordUse("other-skill"); err != nil {
		t.Fatal(err)
	}
	if err := store.Archive("other-skill"); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	archived := filepath.Join(store.ArchiveDir(), "other-skill", "other-skill.md")
	if _, err := os.Stat(archived); err != nil {
		t.Fatalf("archived file must exist: %v", err)
	}
	if _, err := os.Stat(md); !os.IsNotExist(err) {
		t.Errorf("local file should have moved away (err=%v)", err)
	}

	u, _ := store.Get("other-skill")
	if u.State != StateArchived {
		t.Errorf("State = %q, want archived", u.State)
	}
}

func TestArchive_EmbeddedMarkedOnlyNoPanic(t *testing.T) {
	store, _ := newTestUsageStore(t)

	// skill sem fonte local → tratada como embutida
	if err := store.RecordUse("embedded-fake"); err != nil {
		t.Fatal(err)
	}
	if err := store.Archive("embedded-fake"); err != nil {
		t.Fatalf("Archive embedded: %v", err)
	}

	u, err := store.Get("embedded-fake")
	if err != nil {
		t.Fatal(err)
	}
	if u.State != StateArchived {
		t.Errorf("State = %q, want archived", u.State)
	}
	if !strings.Contains(u.Notes, "embedded") {
		t.Errorf("Notes should mention embedded, got %q", u.Notes)
	}
	// nada foi movido para o .archive
	if _, err := os.Stat(filepath.Join(store.ArchiveDir(), "embedded-fake")); !os.IsNotExist(err) {
		t.Errorf("embedded skill should NOT have been moved (err=%v)", err)
	}
}

func TestArchive_NeverDeletes(t *testing.T) {
	store, coscaDir := newTestUsageStore(t)

	dir := filepath.Join(coscaDir, "skills", "precious")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"precious.md", "notes.md"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("# content\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RecordUse("precious"); err != nil {
		t.Fatal(err)
	}
	if err := store.Archive("precious"); err != nil {
		t.Fatal(err)
	}

	// o conteúdo existe no archive — nada foi deletado
	archivedDir := filepath.Join(store.ArchiveDir(), "precious")
	for _, f := range []string{"precious.md", "notes.md"} {
		if _, err := os.Stat(filepath.Join(archivedDir, f)); err != nil {
			t.Errorf("archived file %s missing (archive must never delete): %v", f, err)
		}
	}
}

func TestRestore_MovesBackAndActivates(t *testing.T) {
	store, coscaDir := newTestUsageStore(t)

	dir := filepath.Join(coscaDir, "skills", "revive")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "revive.md"), []byte("# revive\n\n## Description\nR\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordUse("revive"); err != nil {
		t.Fatal(err)
	}
	if err := store.Archive("revive"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("precondition: skill should be archived")
	}

	if err := store.Restore("revive"); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("restored dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "revive.md")); err != nil {
		t.Errorf("restored content missing: %v", err)
	}
	u, err := store.Get("revive")
	if err != nil {
		t.Fatal(err)
	}
	if u.State != StateActive {
		t.Errorf("State = %q, want active", u.State)
	}
	if u.Notes != "" {
		t.Errorf("Notes should be cleared, got %q", u.Notes)
	}
}

func TestRestore_EmbeddedOnlyReactivates(t *testing.T) {
	store, _ := newTestUsageStore(t)
	if err := store.RecordUse("emb-restore"); err != nil {
		t.Fatal(err)
	}
	if err := store.Archive("emb-restore"); err != nil {
		t.Fatal(err)
	}
	if err := store.Restore("emb-restore"); err != nil {
		t.Fatalf("Restore embedded: %v", err)
	}
	u, _ := store.Get("emb-restore")
	if u.State != StateActive {
		t.Errorf("State = %q, want active", u.State)
	}
	if u.Notes != "" {
		t.Errorf("Notes should be cleared, got %q", u.Notes)
	}
}

func TestMarkStale(t *testing.T) {
	store, _ := newTestUsageStore(t)
	if err := store.RecordUse("will-stale"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkStale("will-stale"); err != nil {
		t.Fatalf("MarkStale: %v", err)
	}
	u, err := store.Get("will-stale")
	if err != nil {
		t.Fatal(err)
	}
	if u.State != StateStale {
		t.Errorf("State = %q, want stale", u.State)
	}
	// skills marcadas stale não entram na lista de candidatos do curador
	stale, err := store.StaleSince(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 0 {
		t.Errorf("marked-stale skill should not be a candidate: %+v", stale)
	}
}
