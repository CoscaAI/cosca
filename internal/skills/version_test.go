package skills

import (
	"path/filepath"
	"testing"
)

func TestVersionKey(t *testing.T) {
	if got := VersionKey("adr-generation", "1.1.0"); got != "skill.adr-generation.version.1.1.0" {
		t.Errorf("unexpected version key: %q", got)
	}
}

func TestMemVersionStoreRoundTrip(t *testing.T) {
	st := NewMemVersionStore()
	g := SkillGovernance{
		Origin:    OriginEvolution,
		Status:    LifecycleValidated,
		GatePassed: true,
		SuccessRate: 0.82,
	}
	rec, err := RecordVersion(st, "adr-generation", "2.0.0", g, "1.1.0")
	if err != nil {
		t.Fatalf("RecordVersion: %v", err)
	}
	if rec.Origin != OriginEvolution || rec.Lifecycle != LifecycleValidated || !rec.GatePassed {
		t.Errorf("record mismatch: %+v", rec)
	}
	got, err := st.Get(VersionKey("adr-generation", "2.0.0"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.PrevVersion != "1.1.0" || got.SuccessRate != 0.82 {
		t.Errorf("got wrong record: %+v", got)
	}
}

func TestRecordVersion_GrandfatherDefault(t *testing.T) {
	st := NewMemVersionStore()
	// Sem status explícito e sem benchmark → cláusula de avô (active + grandfathered).
	rec, err := RecordVersion(st, "legacy-skill", "1.0", SkillGovernance{Grandfathered: true}, "")
	if err != nil {
		t.Fatalf("RecordVersion: %v", err)
	}
	if rec.Lifecycle != LifecycleActive {
		t.Errorf("grandfathered skill must be active, got %q", rec.Lifecycle)
	}
	if !rec.Grandfathered {
		t.Error("expected grandfathered=true")
	}
}

func TestRecordVersion_RequiresNameAndVersion(t *testing.T) {
	st := NewMemVersionStore()
	if _, err := RecordVersion(st, "", "1.0", SkillGovernance{}, ""); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestLedgerVersionStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLedgerVersionStore(filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatalf("NewLedgerVersionStore: %v", err)
	}
	defer store.Close()
	g := SkillGovernance{Origin: OriginHuman, Status: LifecycleActive}
	if _, err := RecordVersion(store, "demo", "3.0", g, "2.0"); err != nil {
		t.Fatalf("RecordVersion: %v", err)
	}
	rec, err := store.Get(VersionKey("demo", "3.0"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.Skill != "demo" || rec.Version != "3.0" || rec.Origin != OriginHuman {
		t.Errorf("ledger round-trip mismatch: %+v", rec)
	}
	// Tamper-evidence (I5): chave inexistente deve falhar.
	if _, err := store.Get(VersionKey("demo", "99")); err == nil {
		t.Error("expected not-found error")
	}
}
