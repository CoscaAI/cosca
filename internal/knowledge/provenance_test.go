//
// Tests for the evidence provenance (P0-P5) — internal/knowledge/provenance.go.
//
// Covers:
//   - Valid() for the 6 provenance levels (and invalid values)
//   - Description() pt-BR for the 6 levels
//   - ParseProvenance: case-insensitive "P0".."P5" (with spaces) + invalid
//   - Evidence with provenance + source ref marshals/unmarshals
//   - Old evidence (no provenance fields) parses → P0 default on
//     FillProvenanceDefaults, on Register and on Load (migration bridge)
//   - AddEvidence with provenance + SourceRef preserves the fields
//   - IsReproducible: P4+repository+commit+sha256 → true; P2 → false
//
// The CLI output test lives in internal/cli/cli_knowledge_evidence_test.go
// (it needs cli's formatter-injection helpers, which are package-private).

package knowledge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProvenance_Valid(t *testing.T) {
	t.Parallel()

	valid := []ProvenanceLevel{
		ProvenanceUnknown,
		ProvenanceExternalUnverified,
		ProvenanceIdentifiableRepo,
		ProvenanceOfficial,
		ProvenanceReproducible,
		ProvenanceIndependent,
	}
	for _, p := range valid {
		if !p.Valid() {
			t.Errorf("expected %q to be valid", p)
		}
	}
	for _, p := range []ProvenanceLevel{"", "P6", "p4", "P4 ", "bogus", "0"} {
		if p.Valid() {
			t.Errorf("expected %q to be invalid", p)
		}
	}
}

func TestProvenance_Description(t *testing.T) {
	t.Parallel()

	want := map[ProvenanceLevel]string{
		ProvenanceUnknown:            "desconhecida",
		ProvenanceExternalUnverified: "fonte externa não verificada",
		ProvenanceIdentifiableRepo:   "repositório identificável",
		ProvenanceOfficial:           "fonte oficial",
		ProvenanceReproducible:       "código/teste reproduzível",
		ProvenanceIndependent:        "múltiplas fontes independentes",
	}
	for p, wantDesc := range want {
		if got := p.Description(); got != wantDesc {
			t.Errorf("Description(%q) = %q, want %q", p, got, wantDesc)
		}
	}
	if got := ProvenanceLevel("bogus").Description(); got == "" {
		t.Error("unknown provenance should still return a non-empty description")
	}
}

func TestParseProvenance(t *testing.T) {
	t.Parallel()

	// Case-insensitive, com ou sem espaços ao redor.
	for _, s := range []string{"P0", "p0", "P4", "p4", "P5", "P2", " p2 "} {
		if _, ok := ParseProvenance(s); !ok {
			t.Errorf("ParseProvenance(%q) should be ok", s)
		}
	}
	if p, ok := ParseProvenance("p4"); !ok || p != ProvenanceReproducible {
		t.Errorf("ParseProvenance(p4) = %q, %v; want P4, true", p, ok)
	}
	if p, ok := ParseProvenance("P5"); !ok || p != ProvenanceIndependent {
		t.Errorf("ParseProvenance(P5) = %q, %v; want P5, true", p, ok)
	}
	for _, s := range []string{"", "P6", "P10", "4", "PX", "P", "0", "unknown"} {
		if _, ok := ParseProvenance(s); ok {
			t.Errorf("ParseProvenance(%q) should fail", s)
		}
	}
}

func TestEvidence_MarshalUnmarshalProvenance(t *testing.T) {
	t.Parallel()

	ev := Evidence{
		ID:          "E-401",
		Kind:        "test",
		Source:      "github.com/CoscaAI/cosca",
		Description: "teste reproduzível",
		Timestamp:   time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC),
		Provenance:  ProvenanceReproducible,
		Repository:  "CoscaAI/cosca",
		Commit:      "7a91c2",
		Path:        "internal/runtime/foo.go",
		SHA256:      "abc123...",
		Retrieved:   time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC),
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`"provenance":"P4"`,
		`"repository":"CoscaAI/cosca"`,
		`"commit":"7a91c2"`,
		`"path":"internal/runtime/foo.go"`,
		`"sha256":"abc123..."`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("marshaled JSON missing %q; got: %s", want, data)
		}
	}

	var back Evidence
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Provenance != ev.Provenance || back.Repository != ev.Repository ||
		back.Commit != ev.Commit || back.SHA256 != ev.SHA256 || back.Path != ev.Path {
		t.Errorf("roundtrip provenance mismatch: %+v", back)
	}
	if !back.Retrieved.Equal(ev.Retrieved) {
		t.Errorf("retrieved mismatch: %v vs %v", back.Retrieved, ev.Retrieved)
	}
}

// TestEvidence_OldEvidenceDefaultsToP0 covers the migration bridge: an old
// evidence written without provenance fields parses unchanged and, on
// FillProvenanceDefaults / Register / Load, gains P0 — exactly like the
// epistemic status migration (DefaultStatus).
func TestEvidence_OldEvidenceDefaultsToP0(t *testing.T) {
	t.Parallel()

	const oldJSON = `{"id":"E-1","kind":"incident","source":"F001","description":"fuga","timestamp":"2026-07-30T09:15:00Z"}`
	var ev Evidence
	if err := json.Unmarshal([]byte(oldJSON), &ev); err != nil {
		t.Fatalf("unmarshal old evidence: %v", err)
	}
	if ev.Provenance != "" {
		t.Fatalf("old evidence should load with empty provenance, got %q", ev.Provenance)
	}
	ev.FillProvenanceDefaults()
	if ev.Provenance != ProvenanceUnknown {
		t.Errorf("FillProvenanceDefaults: provenance = %q, want P0", ev.Provenance)
	}

	// Load path: laws.json with old evidence → P0 aplicado no load.
	path := filepath.Join(t.TempDir(), "laws.json")
	const oldLawsJSON = `{"version":1,"items":[{"id":"K-1","title":"T","level":"law","confidence":0.99,
		"evidence":[{"id":"E-1","kind":"incident","source":"F001","description":"fuga","timestamp":"2026-07-30T09:15:00Z"}]}]}`
	if err := os.WriteFile(path, []byte(oldLawsJSON), 0o600); err != nil {
		t.Fatalf("write laws.json: %v", err)
	}
	engine := NewPromotionEngine()
	if err := engine.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	it, ok := engine.Get("K-1")
	if !ok {
		t.Fatal("K-1 not loaded")
	}
	if len(it.Evidence) != 1 {
		t.Fatalf("len(Evidence) = %d, want 1", len(it.Evidence))
	}
	if it.Evidence[0].Provenance != ProvenanceUnknown {
		t.Errorf("loaded old evidence provenance = %q, want P0", it.Evidence[0].Provenance)
	}
	if it.Evidence[0].ID != "E-1" || it.Evidence[0].Description != "fuga" {
		t.Errorf("old evidence fields mutated: %+v", it.Evidence[0])
	}

	// Round-trip: Save/Load mantém o default P0 persistido.
	if err := engine.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	engine2 := NewPromotionEngine()
	if err := engine2.Load(path); err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	it2, _ := engine2.Get("K-1")
	if it2.Evidence[0].Provenance != ProvenanceUnknown {
		t.Errorf("reloaded evidence provenance = %q, want P0", it2.Evidence[0].Provenance)
	}
}

// TestRegister_AppliesP0Default verifies the register bridge: an item
// registered with evidence without provenance gains P0.
func TestRegister_AppliesP0Default(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()
	if err := engine.Register(&KnowledgeItem{
		ID:    "K-1",
		Title: "Item novo",
		Evidence: []Evidence{
			{ID: "E-1", Kind: "test", Source: "s", Description: "d"},
		},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	it, ok := engine.Get("K-1")
	if !ok {
		t.Fatal("K-1 not registered")
	}
	if it.Evidence[0].Provenance != ProvenanceUnknown {
		t.Errorf("registered evidence provenance = %q, want P0", it.Evidence[0].Provenance)
	}
}

// TestAddEvidence_WithProvenanceAndSourceRef verifies AddEvidence preserves
// the provenance + auditable source reference (repository, commit, sha256...).
func TestAddEvidence_WithProvenanceAndSourceRef(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()
	if err := engine.Register(&KnowledgeItem{ID: "K-1", Title: "Item"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ref := SourceRef{
		Repository: "CoscaAI/cosca",
		Commit:     "7a91c2",
		Path:       "internal/runtime/foo.go",
		SHA256:     "abc123...",
		Retrieved:  time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC),
	}
	ev := Evidence{
		ID:          "E-401",
		Kind:        "test",
		Source:      "github.com/CoscaAI/cosca",
		Description: "teste reproduzível",
		Timestamp:   time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC),
		Provenance:  ProvenanceReproducible,
	}
	ev.FillSourceRef(ref)
	if _, err := engine.AddEvidence("K-1", ev); err != nil {
		t.Fatalf("AddEvidence: %v", err)
	}

	got, ok := engine.Get("K-1")
	if !ok {
		t.Fatal("K-1 not found")
	}
	if len(got.Evidence) != 1 {
		t.Fatalf("len(Evidence) = %d, want 1", len(got.Evidence))
	}
	e := got.Evidence[0]
	if e.Provenance != ProvenanceReproducible || e.Repository != "CoscaAI/cosca" ||
		e.Commit != "7a91c2" || e.SHA256 != "abc123..." || e.Path != "internal/runtime/foo.go" {
		t.Errorf("provenance/source ref not preserved: %+v", e)
	}
	if back := e.SourceRef(); back != ref {
		t.Errorf("SourceRef() = %+v, want %+v", back, ref)
	}
}

func TestIsReproducible(t *testing.T) {
	t.Parallel()

	// P4 + repository + commit + sha256 → reproduzível.
	ev := Evidence{
		Provenance: ProvenanceReproducible,
		Repository: "CoscaAI/cosca",
		Commit:     "7a91c2",
		SHA256:     "abc123...",
	}
	if !ev.IsReproducible() {
		t.Error("P4 + repository+commit+sha256 should be reproducible")
	}
	if repro := (&KnowledgeItem{Evidence: []Evidence{ev}}).ReproducibleEvidence(); len(repro) != 1 {
		t.Errorf("ReproducibleEvidence = %d, want 1", len(repro))
	}

	// P2 → não reproduzível (nível abaixo de P4).
	if (Evidence{Provenance: ProvenanceIdentifiableRepo}).IsReproducible() {
		t.Error("P2 should not be reproducible")
	}

	// P4 mas sem sha256 → não reproduzível.
	if (Evidence{Provenance: ProvenanceReproducible, Repository: "r", Commit: "c"}).IsReproducible() {
		t.Error("P4 without sha256 should not be reproducible")
	}

	// P4 mas sem commit → não reproduzível.
	if (Evidence{Provenance: ProvenanceReproducible, Repository: "r", SHA256: "s"}).IsReproducible() {
		t.Error("P4 without commit should not be reproducible")
	}

	// P5 sem repository → não reproduzível.
	if (Evidence{Provenance: ProvenanceIndependent, Commit: "c", SHA256: "s"}).IsReproducible() {
		t.Error("P5 without repository should not be reproducible")
	}

	// P0 → não reproduzível.
	if (Evidence{Provenance: ProvenanceUnknown, Repository: "r", Commit: "c", SHA256: "s"}).IsReproducible() {
		t.Error("P0 should not be reproducible")
	}
}
