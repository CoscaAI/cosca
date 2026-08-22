//
// Tests for the Bug Fingerprint engine (internal/kernel/bugfingerprint.go).
//
// Covers:
//   - Key: normalização canônica (case + whitespace) de component:error:path:phase
//   - FamilyKey: núcleo component:error que agrupa a família
//   - Matches: component+error exigidos; path/phase só comparados se ambos
//   - Register: primeiro → BUG-0001; MESMO fingerprint de novo → TimesSeen++
//     (sem duplicar); erro diferente → nova família/novo ID
//   - Match/Family retornam a mesma família; família diferente para erro diferente
//   - List/Get/NormalizeBugID/persistência ao reabrir
//
// A base SQLite vive em t.TempDir() — nunca no .cosca real do projeto.

package kernel

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestBugStore abre um BugStore em um diretório temporário.
func newTestBugStore(t *testing.T) *BugStore {
	t.Helper()
	store, err := NewBugStore(filepath.Join(t.TempDir(), "bug.db"))
	if err != nil {
		t.Fatalf("NewBugStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// backupFP monta o fingerprint do demo do Don: runtime.backup/permission_denied.
func backupFP(path, phase string) BugFingerprint {
	return BugFingerprint{
		Component: "runtime.backup",
		Error:     "permission_denied",
		Path:      path,
		Phase:     phase,
	}
}

// =============================================================================
// Key — normalização canônica
// =============================================================================

func TestFingerprintKey_NormalizesCaseAndWhitespace(t *testing.T) {
	fp := BugFingerprint{
		Component: "  Runtime.Backup  ",
		Error:     "Permission  Denied",
		Path:      "/Backups",
		Phase:     " Snapshot\t",
	}
	want := "runtime.backup:permission denied:/backups:snapshot"
	if got := fp.Key(); got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
}

func TestFingerprintKey_ComponentAndErrorAlwaysPresent(t *testing.T) {
	fp := BugFingerprint{Component: "runtime.backup", Error: "permission_denied"}
	want := "runtime.backup:permission_denied::"
	if got := fp.Key(); got != want {
		t.Errorf("Key() = %q, want %q (path/phase vazios entram como campos vazios)", got, want)
	}
}

func TestFingerprintFamilyKey_IsComponentErrorCore(t *testing.T) {
	withContext := backupFP("/backups", "snapshot")
	if got := withContext.FamilyKey(); got != "runtime.backup:permission_denied" {
		t.Errorf("FamilyKey() = %q, want runtime.backup:permission_denied", got)
	}
}

// =============================================================================
// Matches — component+error exigidos; path/phase condicionais
// =============================================================================

func TestFingerprintMatches_ComponentErrorRequired(t *testing.T) {
	base := backupFP("/backups", "snapshot")

	matching := []BugFingerprint{
		backupFP("/backups", "snapshot"), // idêntico
		backupFP("/backups", ""),         // phase ausente em um lado → curinga
		backupFP("", "snapshot"),         // path ausente em um lado → curinga
	}
	for _, other := range matching {
		if !base.Matches(other) {
			t.Errorf("Matches(%+v) = false, want true (mesma família)", other)
		}
	}

	nonMatching := []BugFingerprint{
		{Component: "runtime.backup", Error: "disk_full", Path: "/backups", Phase: "snapshot"},     // erro diferente
		{Component: "runtime.db", Error: "permission_denied", Path: "/backups", Phase: "snapshot"}, // componente diferente
		backupFP("/tmp", "restore"), // path E phase diferentes em AMBOS os lados
	}
	for _, other := range nonMatching {
		if base.Matches(other) {
			t.Errorf("Matches(%+v) = true, want false (família diferente)", other)
		}
	}

	// Curinga em ambas as direções: base só com component+error casa com um
	// fingerprint completo (path/phase presentes em apenas um lado).
	core := backupFP("", "")
	for _, other := range []BugFingerprint{backupFP("/backups", "snapshot"), backupFP("/backups", "")} {
		if !core.Matches(other) {
			t.Errorf("Matches(%+v) = false, want true (curinga)", other)
		}
	}
}

// =============================================================================
// Register — BUG-0001, dedup por fingerprint, famílias distintas
// =============================================================================

func TestRegister_FirstBugIsBUG0001(t *testing.T) {
	store := newTestBugStore(t)

	id, err := store.Register(BugRecord{
		Title:       "backup falhou",
		Fingerprint: backupFP("/backups", "snapshot"),
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if id != "BUG-0001" {
		t.Errorf("first id = %q, want BUG-0001", id)
	}

	rec, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("record not found after Register")
	}
	if rec.Fingerprint.Key() != "runtime.backup:permission_denied:/backups:snapshot" {
		t.Errorf("Fingerprint.Key() = %q", rec.Fingerprint.Key())
	}
	if rec.Family != "runtime.backup:permission_denied" {
		t.Errorf("Family = %q, want runtime.backup:permission_denied", rec.Family)
	}
	if rec.TimesSeen != 1 {
		t.Errorf("TimesSeen = %d, want 1", rec.TimesSeen)
	}
	if rec.Status != BugStatusOpen {
		t.Errorf("Status = %q, want open", rec.Status)
	}
	if rec.FirstSeen.IsZero() || rec.LastSeen.IsZero() {
		t.Error("FirstSeen/LastSeen should be set")
	}
}

func TestRegister_SameFingerprintIncrementsTimesSeen(t *testing.T) {
	store := newTestBugStore(t)

	id1, err := store.Register(BugRecord{
		Title:       "backup falhou",
		Fingerprint: backupFP("/backups", "snapshot"),
	})
	if err != nil {
		t.Fatalf("Register #1: %v", err)
	}

	// O MESMO fingerprint aparece de novo — deve incrementar, não duplicar.
	id2, err := store.Register(BugRecord{
		Title:       "backup falhou de novo",
		Fingerprint: backupFP("/backups", "snapshot"),
	})
	if err != nil {
		t.Fatalf("Register #2: %v", err)
	}
	if id2 != id1 {
		t.Errorf("second register id = %q, want %q (mesmo fingerprint → mesmo bug)", id2, id1)
	}

	rec, err := store.Get(id1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.TimesSeen != 2 {
		t.Errorf("TimesSeen = %d, want 2", rec.TimesSeen)
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("List len = %d, want 1 (sem duplicata)", len(all))
	}
}

func TestRegister_DifferentErrorIsDifferentFamily(t *testing.T) {
	store := newTestBugStore(t)

	id1, err := store.Register(BugRecord{Fingerprint: backupFP("/backups", "snapshot")})
	if err != nil {
		t.Fatalf("Register #1: %v", err)
	}
	id2, err := store.Register(BugRecord{Fingerprint: BugFingerprint{
		Component: "runtime.backup", Error: "disk_full", Path: "/backups", Phase: "snapshot",
	}})
	if err != nil {
		t.Fatalf("Register #2: %v", err)
	}

	if id1 == id2 {
		t.Errorf("different errors must produce different bugs, got same id %q", id1)
	}
	if id2 != "BUG-0002" {
		t.Errorf("second bug id = %q, want BUG-0002", id2)
	}
}

func TestRegister_RequiresComponentAndError(t *testing.T) {
	store := newTestBugStore(t)
	if _, err := store.Register(BugRecord{Fingerprint: BugFingerprint{Error: "x"}}); err == nil {
		t.Error("Register without component should fail")
	}
	if _, err := store.Register(BugRecord{Fingerprint: BugFingerprint{Component: "x"}}); err == nil {
		t.Error("Register without error should fail")
	}
}

func TestRegister_InvalidStatus(t *testing.T) {
	store := newTestBugStore(t)
	_, err := store.Register(BugRecord{
		Status:      "banana",
		Fingerprint: backupFP("/backups", "snapshot"),
	})
	if err == nil || !strings.Contains(err.Error(), "status inválido") {
		t.Errorf("invalid status should fail, got %v", err)
	}
}

// =============================================================================
// Match / Family — agrupamento por família
// =============================================================================

func TestMatch_SameFamilyWithPathAndPhaseWildcards(t *testing.T) {
	store := newTestBugStore(t)

	// BUG-0001: mesma família, path/phase preenchidos.
	if _, err := store.Register(BugRecord{Fingerprint: backupFP("/backups", "snapshot")}); err != nil {
		t.Fatalf("Register #1: %v", err)
	}
	// BUG-0002: mesma família, só component+error.
	if _, err := store.Register(BugRecord{Fingerprint: backupFP("", "")}); err != nil {
		t.Fatalf("Register #2: %v", err)
	}
	// BUG-0003: família diferente.
	if _, err := store.Register(BugRecord{Fingerprint: BugFingerprint{
		Component: "runtime.backup", Error: "disk_full", Path: "/backups", Phase: "snapshot",
	}}); err != nil {
		t.Fatalf("Register #3: %v", err)
	}

	// match por component+error → 2 bugs da mesma família.
	records, err := store.Match(backupFP("", ""))
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("Match len = %d, want 2", len(records))
	}
	for _, r := range records {
		if r.Family != "runtime.backup:permission_denied" {
			t.Errorf("matched record family = %q, want runtime.backup:permission_denied", r.Family)
		}
	}

	// match por fingerprint completo (path+phase) também agrupa a família.
	full, err := store.Match(backupFP("/backups", "snapshot"))
	if err != nil {
		t.Fatalf("Match full: %v", err)
	}
	if len(full) != 2 {
		t.Errorf("Match full len = %d, want 2", len(full))
	}

	// Família do erro diferente → só BUG-0003.
	disk, err := store.Match(BugFingerprint{Component: "runtime.backup", Error: "disk_full"})
	if err != nil {
		t.Fatalf("Match disk_full: %v", err)
	}
	if len(disk) != 1 || disk[0].ID != "BUG-0003" {
		t.Errorf("Match disk_full = %+v, want [BUG-0003]", disk)
	}
}

func TestFamily_ReturnsAllBugsInFamily(t *testing.T) {
	store := newTestBugStore(t)
	if _, err := store.Register(BugRecord{Fingerprint: backupFP("/backups", "snapshot")}); err != nil {
		t.Fatalf("Register #1: %v", err)
	}
	if _, err := store.Register(BugRecord{Fingerprint: backupFP("", "")}); err != nil {
		t.Fatalf("Register #2: %v", err)
	}

	records, err := store.Family("runtime.backup:permission_denied")
	if err != nil {
		t.Fatalf("Family: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("Family len = %d, want 2", len(records))
	}

	// Case/whitespace não importam na chave de família.
	records, err = store.Family("  Runtime.Backup : Permission_Denied  ")
	if err != nil {
		t.Fatalf("Family (normalizada): %v", err)
	}
	if len(records) != 2 {
		t.Errorf("Family normalized len = %d, want 2", len(records))
	}

	// Família sem bugs → lista vazia (não erro).
	empty, err := store.Family("runtime.db:connection_refused")
	if err != nil {
		t.Fatalf("Family empty: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("Family empty len = %d, want 0", len(empty))
	}
}

// =============================================================================
// Persistência + NormalizeBugID
// =============================================================================

func TestBugStore_PersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "bug.db")

	store, err := NewBugStore(dbPath)
	if err != nil {
		t.Fatalf("NewBugStore: %v", err)
	}
	id, err := store.Register(BugRecord{
		Title:       "backup falhou",
		Fingerprint: backupFP("/backups", "snapshot"),
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	// Registro que incrementa TimesSeen.
	if _, err := store.Register(BugRecord{Fingerprint: backupFP("/backups", "snapshot")}); err != nil {
		t.Fatalf("Register #2: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := NewBugStore(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	rec, err := reopened.Get(id)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if rec == nil {
		t.Fatal("record lost after reopen")
	}
	if rec.TimesSeen != 2 {
		t.Errorf("TimesSeen after reopen = %d, want 2", rec.TimesSeen)
	}
	if rec.Fingerprint.Key() != "runtime.backup:permission_denied:/backups:snapshot" {
		t.Errorf("fingerprint after reopen = %q", rec.Fingerprint.Key())
	}

	// IDs continuam sequenciais após reabrir.
	id2, err := reopened.Register(BugRecord{Fingerprint: BugFingerprint{
		Component: "runtime.db", Error: "connection_refused",
	}})
	if err != nil {
		t.Fatalf("Register after reopen: %v", err)
	}
	if id2 != "BUG-0002" {
		t.Errorf("id after reopen = %q, want BUG-0002", id2)
	}
}

func TestNormalizeBugID(t *testing.T) {
	cases := map[string]string{
		"BUG-0001": "BUG-0001",
		"bug-0001": "BUG-0001",
		"BUG-1":    "BUG-0001",
		"0001":     "BUG-0001",
		"0184":     "BUG-0184",
		"BUG-201":  "BUG-0201",
	}
	for in, want := range cases {
		got, err := NormalizeBugID(in)
		if err != nil {
			t.Errorf("NormalizeBugID(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeBugID(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := NormalizeBugID("X-0001"); err == nil {
		t.Error("NormalizeBugID of a non-bug id should fail")
	}
}

func TestRegister_RevisitKeepsFirstSeenAndUpdatesLastSeen(t *testing.T) {
	store := newTestBugStore(t)
	id, err := store.Register(BugRecord{Fingerprint: backupFP("/backups", "snapshot")})
	if err != nil {
		t.Fatalf("Register #1: %v", err)
	}
	before, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	firstSeen := before.FirstSeen
	time.Sleep(2 * time.Millisecond)

	if _, err := store.Register(BugRecord{Fingerprint: backupFP("/backups", "snapshot")}); err != nil {
		t.Fatalf("Register #2: %v", err)
	}
	after, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !after.FirstSeen.Equal(firstSeen) {
		t.Errorf("FirstSeen changed on revisit: %v → %v", firstSeen, after.FirstSeen)
	}
	if !after.LastSeen.After(firstSeen) {
		t.Errorf("LastSeen should advance on revisit: first=%v last=%v", firstSeen, after.LastSeen)
	}
}
