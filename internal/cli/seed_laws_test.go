package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

func TestSeedDefinitions(t *testing.T) {
	defs := SeedDefinitions()
	if len(defs) != 5 {
		t.Fatalf("SeedDefinitions len = %d, want 5", len(defs))
	}

	ids := map[string]bool{}
	for _, d := range defs {
		if !d.Reproducible {
			t.Fatalf("law %s must be reproducible", d.ID)
		}
		// Laws born at learning level require >= 3 evidences.
		if len(d.Evidences) < 3 {
			t.Fatalf("law %s has %d evidences, want >= 3", d.ID, len(d.Evidences))
		}
		ids[d.ID] = true
	}
	for _, want := range []string{"K-01", "K-02", "K-03", "K-04", "K-05"} {
		if !ids[want] {
			t.Fatalf("missing law %s in definitions", want)
		}
	}
}

func TestSeedTimeDeterministic(t *testing.T) {
	a := seedTime(30, 9, 15)
	b := seedTime(30, 9, 15)
	if !a.Equal(b) {
		t.Fatal("seedTime must be deterministic")
	}
	if a.Year() != 2026 || a.Month() != 7 || a.Day() != 30 {
		t.Fatalf("seedTime = %v", a)
	}
}

func TestFileSHA256(t *testing.T) {
	// Missing file → error.
	if _, err := fileSHA256("/nonexistent/definitely-missing.txt"); err == nil {
		t.Fatal("fileSHA256(missing) expected error")
	}

	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	sha, err := fileSHA256(p)
	if err != nil {
		t.Fatalf("fileSHA256: %v", err)
	}
	if len(sha) != 64 {
		t.Fatalf("sha len = %d, want 64", len(sha))
	}
}

func TestSeedEvidenceProvenance(t *testing.T) {
	// Without path: no SHA256, official provenance.
	ev := seedEvidenceProvenance("E1", "audit", knowledge.ProvenanceOfficial)
	if ev.Path != "" || ev.SHA256 != "" {
		t.Fatalf("no-path evidence: %+v", ev)
	}
	if ev.Provenance != knowledge.ProvenanceOfficial || ev.Repository == "" || ev.Commit == "" {
		t.Fatalf("provenance fields: %+v", ev)
	}

	// With existing path: SHA256 computed.
	dir := t.TempDir()
	p := filepath.Join(dir, "test.go")
	_ = os.WriteFile(p, []byte("package x\n"), 0o644)
	ev2 := seedEvidenceProvenance("E2", "test", knowledge.ProvenanceIndependent, p)
	if len(ev2.SHA256) != 64 {
		t.Fatalf("computed sha = %q", ev2.SHA256)
	}
	if ev2.Path != p {
		t.Fatalf("path = %q", ev2.Path)
	}

	// With missing path: SHA empty, no error.
	ev3 := seedEvidenceProvenance("E3", "test", knowledge.ProvenanceIndependent, "definitely/missing.go")
	if ev3.SHA256 != "" {
		t.Fatalf("missing-file sha should be empty, got %q", ev3.SHA256)
	}
}

func TestEnsureSeededLawsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "laws.json")

	// First run: seeds the file.
	seeded, err := ensureSeededLaws(path)
	if err != nil {
		t.Fatalf("ensureSeededLaws: %v", err)
	}
	if !seeded {
		t.Fatal("first run must seed")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("laws file not created: %v", err)
	}

	// Second run: existing file is never overwritten.
	seeded2, err := ensureSeededLaws(path)
	if err != nil {
		t.Fatalf("second ensureSeededLaws: %v", err)
	}
	if seeded2 {
		t.Fatal("existing file must not be re-seeded")
	}
}

func TestSeedLawsRegisterAtLearningLevel(t *testing.T) {
	engine := knowledge.NewPromotionEngine()
	if err := seedLaws(engine); err != nil {
		t.Fatalf("seedLaws: %v", err)
	}

	for _, id := range []string{"K-01", "K-02", "K-03", "K-04", "K-05"} {
		item, ok := engine.Get(id)
		if !ok {
			t.Fatalf("law %s not registered", id)
		}
		if len(item.Evidence) < 3 {
			t.Fatalf("law %s has %d evidences", id, len(item.Evidence))
		}
		if item.Confidence < 0.70 {
			t.Fatalf("law %s confidence = %v, want >= 0.70", id, item.Confidence)
		}
		// Register derives the level from thresholds: >=3 evidence + conf>=0.7
		// → learning (never unknown).
		if item.Level == "" {
			t.Fatalf("law %s has no derived level", id)
		}
	}
}

func TestSeedLawsRoundtripThroughEngine(t *testing.T) {
	engine := knowledge.NewPromotionEngine()
	if err := seedLaws(engine); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "laws.json")
	if err := engine.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded := knowledge.NewPromotionEngine()
	if err := loaded.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	all := loaded.All()
	if len(all) != 5 {
		t.Fatalf("loaded laws = %d, want 5", len(all))
	}
}
