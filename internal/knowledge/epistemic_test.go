//
// Tests for the typed epistemic states (CKL) — internal/knowledge/epistemic.go.
//
// Covers:
//   - Valid() for the 6 epistemic states (and invalid values)
//   - Description() pt-BR for the 6 states
//   - DefaultStatus mapping: learning → SUPPORTED, law → KNOWN, "" → UNKNOWN
//   - Parsing an existing laws.json with and without status fields: both
//     parse; missing status → DefaultStatus applied on Load
//   - Load preserves an explicit status and never touches level/validation
//
// The CLI output test lives in internal/cli/cli_knowledge_status_test.go
// (it needs cli's formatter-injection helpers, which are package-private).

package knowledge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStatus_Valid(t *testing.T) {
	valid := []EpistemicStatus{
		StatusKnown,
		StatusSupported,
		StatusUncertain,
		StatusConflicting,
		StatusUnknown,
		StatusStale,
	}
	for _, s := range valid {
		if !s.Valid() {
			t.Errorf("expected %q to be valid", s)
		}
	}
	for _, s := range []EpistemicStatus{"", "KNOWN ", "unknown", "NOPE"} {
		if s.Valid() {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestStatus_Description(t *testing.T) {
	want := map[EpistemicStatus]string{
		StatusKnown:       "conhecimento estabelecido com evidência",
		StatusSupported:   "apoiado por evidência parcial",
		StatusUncertain:   "evidência insuficiente",
		StatusConflicting: "evidências contraditórias",
		StatusUnknown:     "sem evidência",
		StatusStale:       "expirado — requer revalidação",
	}
	for s, wantDesc := range want {
		if got := s.Description(); got != wantDesc {
			t.Errorf("Description(%q) = %q, want %q", s, got, wantDesc)
		}
	}
	if got := EpistemicStatus("bogus").Description(); got == "" {
		t.Error("unknown status should still return a non-empty description")
	}
}

func TestDefaultStatus_Mapping(t *testing.T) {
	want := map[string]EpistemicStatus{
		"learning": StatusSupported,
		"law":      StatusKnown,
		"":         StatusUnknown,
	}
	for level, wantStatus := range want {
		if got := DefaultStatus(level); got != wantStatus {
			t.Errorf("DefaultStatus(%q) = %q, want %q", level, got, wantStatus)
		}
	}
}

// sampleLawsJSON is a representative existing laws.json — no status fields,
// like every file the current CKL already wrote.
const sampleLawsJSON = `{
  "version": 1,
  "items": [
    {
      "id": "K-01",
      "title": "Nunca executar como root automaticamente",
      "level": "learning",
      "evidence": [{"id": "F001", "kind": "incident", "source": "F001", "description": "fuga", "timestamp": "2026-07-30T09:15:00Z"}],
      "projects": 1,
      "confidence": 0.95,
      "rollbacks": 0,
      "reproducible": true,
      "created_at": "2026-07-30T09:15:00Z"
    },
    {
      "id": "K-02",
      "title": "Registro público desabilitado por padrão",
      "level": "law",
      "evidence": [{"id": "ev-1", "kind": "audit", "source": "red-team-A2", "description": "público", "timestamp": "2026-07-31T10:10:00Z"}],
      "projects": 1,
      "confidence": 0.99,
      "rollbacks": 0,
      "reproducible": false,
      "created_at": "2026-07-31T10:10:00Z"
    },
    {
      "id": "K-03",
      "title": "Observação solta",
      "evidence": [{"id": "ev-2", "kind": "test", "source": "test/", "description": "primeira", "timestamp": "2026-07-31T14:30:00Z"}],
      "confidence": 0.10,
      "created_at": "2026-07-31T14:30:00Z"
    }
  ]
}`

// writeLawsSample writes the given JSON as the laws.json under a temp dir.
func writeLawsSample(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "laws.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write sample laws.json: %v", err)
	}
	return path
}

func TestLoad_AppliesDefaultStatusWhenMissing(t *testing.T) {
	path := writeLawsSample(t, sampleLawsJSON)

	engine := NewPromotionEngine()
	if err := engine.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := map[string]EpistemicStatus{}
	for _, it := range engine.All() {
		got[it.ID] = it.Status
	}

	want := map[string]EpistemicStatus{
		"K-01": StatusSupported, // learning → SUPPORTED
		"K-02": StatusKnown,     // law → KNOWN
		"K-03": StatusUnknown,   // observation → UNKNOWN
	}
	for id, wantStatus := range want {
		if got[id] != wantStatus {
			t.Errorf("loaded %s status = %q, want %q (DefaultStatus applied)", id, got[id], wantStatus)
		}
	}

	// A carga jamais altera o nível/validação existentes.
	if it, ok := engine.Get("K-01"); ok {
		if it.Level != LevelLearning {
			t.Errorf("K-01 level changed to %q, want learning (no regression)", it.Level)
		}
		if len(it.Evidence) != 1 {
			t.Errorf("K-01 evidence count changed to %d, want 1", len(it.Evidence))
		}
	}
}

func TestLoad_PreservesExplicitStatus(t *testing.T) {
	const withStatus = `{
  "version": 1,
  "items": [
    {
      "id": "K-01",
      "title": "Lei com estado explícito",
      "level": "learning",
      "evidence": [{"id": "ev-1", "kind": "test", "source": "test/", "description": "d", "timestamp": "2026-07-31T14:30:00Z"}],
      "projects": 1,
      "confidence": 0.95,
      "rollbacks": 0,
      "created_at": "2026-07-31T10:00:00Z",
      "status": "STALE",
      "last_verified": "2026-01-15T12:00:00Z",
      "verification_count": 4,
      "contradiction_count": 1
    }
  ]
}`
	path := writeLawsSample(t, withStatus)

	engine := NewPromotionEngine()
	if err := engine.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	it, ok := engine.Get("K-01")
	if !ok {
		t.Fatal("K-01 not loaded")
	}
	if it.Status != StatusStale {
		t.Errorf("status = %q, want STALE (explicit must be preserved)", it.Status)
	}
	if !it.LastVerified.Equal(time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("last_verified = %v, want 2026-01-15", it.LastVerified)
	}
	if it.VerificationCount != 4 {
		t.Errorf("verification_count = %d, want 4", it.VerificationCount)
	}
	if it.ContradictionCount != 1 {
		t.Errorf("contradiction_count = %d, want 1", it.ContradictionCount)
	}
}

func TestRegister_AppliesDefaultStatus(t *testing.T) {
	engine := NewPromotionEngine()
	if err := engine.Register(&KnowledgeItem{
		ID:         "K-1",
		Title:      "Lei nova",
		Level:      LevelLaw,
		Confidence: 0.99,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	it, ok := engine.Get("K-1")
	if !ok {
		t.Fatal("K-1 not registered")
	}
	if it.Status != StatusKnown {
		t.Errorf("registered law status = %q, want KNOWN", it.Status)
	}

	if err := engine.Register(&KnowledgeItem{ID: "K-2", Title: "Sem nível"}); err != nil {
		t.Fatalf("Register K-2: %v", err)
	}
	it2, ok := engine.Get("K-2")
	if !ok {
		t.Fatal("K-2 not registered")
	}
	if it2.Status != StatusUnknown {
		t.Errorf("registered observation status = %q, want UNKNOWN", it2.Status)
	}
}
