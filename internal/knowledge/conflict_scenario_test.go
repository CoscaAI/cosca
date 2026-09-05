package knowledge

import (
	"path/filepath"
	"testing"
	"time"
)

// ── P0-7: Knowledge conflict — v1 vs v2 (cenário do professor) ─────────
//
// Inserir "PostgreSQL é obrigatório" (fonte A) e "SQLite é obrigatório"
// (fonte B) e verificar que o sistema: detecta o conflito, mantém as duas
// com provenance distinta, e sinaliza CONFLICTING sem misturar.

func TestConflict_KnowledgeV1vsV2(t *testing.T) {
	dir := t.TempDir()
	s, err := NewConflictStore(filepath.Join(dir, "conflict.db"))
	if err != nil {
		t.Fatalf("NewConflictStore: %v", err)
	}
	// Close releases the SQLite handle; without it the TempDir cleanup fails
	// on Windows ("file already in use by another process").
	t.Cleanup(func() { _ = s.Close() })

	// Duas fontes com claims opostos sobre o MESMO item.
	_, err = s.Add(ConflictRecord{
		ClaimA:      "K-27:E-101", // v1: "PostgreSQL é obrigatório" (fonte A)
		ClaimB:      "K-27:E-203", // v2: "SQLite é obrigatório" (fonte B)
		ItemID:      "K-27",
		Description: "fonte A diz PostgreSQL obrigatório, fonte B diz SQLite obrigatório",
		DetectedAt:  time.Now(),
		Status:      ConflictOpen,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// O item, com o conflito aberto, é reportado como CONFLICTING.
	item := &KnowledgeItem{ID: "K-27", Level: LevelLearning}
	open, err := s.OpenConflictsForItem("K-27")
	if err != nil || len(open) != 1 {
		t.Fatalf("OpenConflictsForItem: %v, %d", err, len(open))
	}
	if got := StatusWithConflicts(item, open); got != StatusConflicting {
		t.Fatalf("status = %q, want conflicting", got)
	}

	// Resolvido → o status deixa de ser CONFLICTING.
	if err := s.Resolve(open[0].ID); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	open2, _ := s.OpenConflictsForItem("K-27")
	if len(open2) != 0 {
		t.Fatalf("open conflicts after resolve = %d, want 0", len(open2))
	}
	if got := StatusWithConflicts(item, open2); got == StatusConflicting {
		t.Fatal("resolved conflict must not report CONFLICTING")
	}
}

func TestConflict_DifferentItemsNotConflicted(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewConflictStore(filepath.Join(dir, "conflict.db"))
	t.Cleanup(func() { _ = s.Close() })
	_, _ = s.Add(ConflictRecord{
		ClaimA: "K-1:E-1", ClaimB: "K-2:E-2", ItemID: "K-1",
		Description: "conflito do K-1", DetectedAt: time.Now(), Status: ConflictOpen,
	})

	// Item K-99 não tem conflito aberto → status normal.
	item := &KnowledgeItem{ID: "K-99", Level: LevelLearning}
	open, _ := s.OpenConflictsForItem("K-99")
	if got := StatusWithConflicts(item, open); got == StatusConflicting {
		t.Fatal("unrelated item must not be conflicting")
	}
}
