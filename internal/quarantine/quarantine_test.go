//
// Tests for the quarantine store (internal/quarantine/quarantine.go).
//
// Covers:
//   - Add atribui IDs incrementais (Q-0001, Q-0002) e grava atomicamente
//   - Arquivo gravado é legível de volta (Get) com conteúdo íntegro
//   - SetStatus valida as transições (transição inválida é rejeitada)
//   - Promote exige validating + PromotedTo
//   - Discard arquiva: arquivo vai para archive/ e ainda existe
//   - NormalizeID aceita formas alternativas
//
// NOTE: todos os testes usam t.TempDir() — nunca o .cosca real.

package quarantine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// addProps cria duas proposals de teste (Q-0001, Q-0002) num Store de temp.
func addProps(t *testing.T, store *Store) (string, string) {
	t.Helper()
	id1, err := store.Add(Proposal{
		Title:   "Cache de embeddings",
		Content: "Propor um LRU no indexer de vetores.",
		Source:  "agent-x",
	})
	if err != nil {
		t.Fatalf("Add #1: %v", err)
	}
	id2, err := store.Add(Proposal{
		Title:   "Lei do rollback",
		Content: "Nunca sobrescrever arquivos sem comparar antes.",
		Source:  "cosca-kernel",
	})
	if err != nil {
		t.Fatalf("Add #2: %v", err)
	}
	return id1, id2
}

// =============================================================================
// Add — IDs incrementais + gravação atômica + leitura de volta
// =============================================================================

func TestAdd_AssignsIncrementingIDs(t *testing.T) {
	store := NewStore(t.TempDir())
	id1, id2 := addProps(t, store)

	if id1 != "Q-0001" {
		t.Errorf("expected first ID Q-0001, got %q", id1)
	}
	if id2 != "Q-0002" {
		t.Errorf("expected second ID Q-0002, got %q", id2)
	}
}

func TestAdd_WrittenAtomicallyAndReadableBack(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	id, err := store.Add(Proposal{
		Title:   "Cache de embeddings",
		Content: "Propor um LRU no indexer de vetores.",
		Source:  "agent-x",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Nenhum tmp deixado para trás.
	if _, err := os.Stat(filepath.Join(store.Path(), id+".json.tmp")); !os.IsNotExist(err) {
		t.Errorf("tmp file should not remain after atomic write (err=%v)", err)
	}

	p, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.ID != id {
		t.Errorf("ID mismatch: %q", p.ID)
	}
	if p.Title != "Cache de embeddings" || p.Content != "Propor um LRU no indexer de vetores." {
		t.Errorf("content roundtrip mismatch: %+v", p)
	}
	if p.Source != "agent-x" {
		t.Errorf("source mismatch: %q", p.Source)
	}
	if p.Status != StatusPending {
		t.Errorf("expected default status pending, got %q", p.Status)
	}
	if p.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestAdd_RequiresTitleAndContent(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Add(Proposal{Title: "", Content: "x"}); err == nil {
		t.Error("expected error when title is empty")
	}
	if _, err := store.Add(Proposal{Title: "x", Content: ""}); err == nil {
		t.Error("expected error when content is empty")
	}
}

func TestAdd_IDSkippedWhenHigherExists(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	addProps(t, store)

	// Discard Q-0001 → Q-0001 só existe no archive. O próximo ID deve ser
	// Q-0003 (IDs arquivados nunca são reutilizados).
	if err := store.SetStatus("Q-0001", StatusValidating); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := store.Discard("Q-0001"); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	id3, err := store.Add(Proposal{Title: "Terceira", Content: "texto"})
	if err != nil {
		t.Fatalf("Add #3: %v", err)
	}
	if id3 != "Q-0003" {
		t.Errorf("expected Q-0003 after archival, got %q", id3)
	}
}

// =============================================================================
// SetStatus — validação de transições
// =============================================================================

func TestSetStatus_ValidTransitions(t *testing.T) {
	store := NewStore(t.TempDir())
	id, _ := addProps(t, store)
	id1 := id

	if err := store.SetStatus(id1, StatusValidating); err != nil {
		t.Fatalf("pending → validating: %v", err)
	}
	if err := store.SetStatus(id1, StatusDiscarded); err != nil {
		t.Fatalf("validating → discarded: %v", err)
	}
}

func TestSetStatus_InvalidTransitionRejected(t *testing.T) {
	store := NewStore(t.TempDir())
	id, _ := addProps(t, store)
	id1 := id

	// pending → promoted (pula a validação).
	if err := store.SetStatus(id1, StatusPromoted); err == nil {
		t.Error("expected rejection for pending → promoted")
	}
	// pending → discarded (precisa passar por validating).
	if err := store.SetStatus(id1, StatusDiscarded); err == nil {
		t.Error("expected rejection for pending → discarded")
	}

	// Depois de validar, retroceder é inválido.
	if err := store.SetStatus(id1, StatusValidating); err != nil {
		t.Fatalf("pending → validating: %v", err)
	}
	if err := store.SetStatus(id1, StatusPending); err == nil {
		t.Error("expected rejection for validating → pending")
	}
	// Status inexistente.
	if err := store.SetStatus(id1, "aprovada"); err == nil {
		t.Error("expected rejection for unknown status")
	}
	// Não é possível promover sem PromotedTo.
	if err := store.SetStatus(id1, StatusPromoted); err == nil {
		t.Error("expected rejection for promoting without PromotedTo")
	}
}

func TestSetStatus_PromotedRequiresPromotedTo(t *testing.T) {
	store := NewStore(t.TempDir())
	id, _ := addProps(t, store)
	id1 := id

	if err := store.SetStatus(id1, StatusValidating); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := store.SetStatus(id1, StatusPromoted); err == nil {
		t.Error("promotion via SetStatus without PromotedTo must fail")
	}
}

// =============================================================================
// Promote — exige validating + PromotedTo
// =============================================================================

func TestPromote_SetsPromotedAndRecordsTarget(t *testing.T) {
	store := NewStore(t.TempDir())
	id, _ := addProps(t, store)
	id1 := id

	if err := store.SetStatus(id1, StatusValidating); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := store.Promote(id1, "K-06"); err != nil {
		t.Fatalf("Promote: %v", err)
	}
	p, err := store.Get(id1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.Status != StatusPromoted {
		t.Errorf("expected status promoted, got %q", p.Status)
	}
	if p.PromotedTo != "K-06" {
		t.Errorf("expected PromotedTo K-06, got %q", p.PromotedTo)
	}
}

func TestPromote_RequiresValidating(t *testing.T) {
	store := NewStore(t.TempDir())
	id, _ := addProps(t, store)
	id1 := id

	if err := store.Promote(id1, "K-06"); err == nil {
		t.Error("expected rejection promoting a pending proposal (must validate first)")
	}
}

func TestPromote_RequiresTarget(t *testing.T) {
	store := NewStore(t.TempDir())
	id, _ := addProps(t, store)
	id1 := id

	if err := store.SetStatus(id1, StatusValidating); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := store.Promote(id1, "  "); err == nil {
		t.Error("expected rejection promoting without a target")
	}
}

// =============================================================================
// Discard — arquiva (move para archive/), nunca deleta
// =============================================================================

func TestDiscard_ArchivesFile(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	id, _ := addProps(t, store)
	id1 := id

	if err := store.SetStatus(id1, StatusValidating); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := store.Discard(id1); err != nil {
		t.Fatalf("Discard: %v", err)
	}

	activePath := filepath.Join(store.Path(), id1+".json")
	archivedPath := filepath.Join(store.ArchivePath(), id1+".json")

	// Saiu do ativo, existe no archive.
	if _, err := os.Stat(activePath); !os.IsNotExist(err) {
		t.Errorf("active file should be gone after discard (err=%v)", err)
	}
	if _, err := os.Stat(archivedPath); err != nil {
		t.Fatalf("archived file must still exist: %v", err)
	}

	// Ainda é legível via Get (fallback para o archive).
	p, err := store.Get(id1)
	if err != nil {
		t.Fatalf("Get after discard: %v", err)
	}
	if p.Status != StatusDiscarded {
		t.Errorf("expected status discarded, got %q", p.Status)
	}

	// Não aparece no List (apenas ativas).
	props, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, prop := range props {
		if prop.ID == id1 {
			t.Error("archived proposal must not appear in List()")
		}
	}
	if len(props) != 1 {
		t.Errorf("expected 1 active proposal after discard, got %d", len(props))
	}
}

func TestDiscard_RejectsWhenNotValidating(t *testing.T) {
	store := NewStore(t.TempDir())
	id, _ := addProps(t, store)
	id1 := id

	if err := store.Discard(id1); err == nil {
		t.Error("expected rejection discarding a pending proposal (must validate first)")
	}
}

// =============================================================================
// NormalizeID
// =============================================================================

func TestNormalizeID(t *testing.T) {
	cases := map[string]string{
		"Q-0001": "Q-0001",
		"q-0001": "Q-0001",
		"Q-1":    "Q-0001",
		"0001":   "Q-0001",
	}
	for in, want := range cases {
		got, err := NormalizeID(in)
		if err != nil {
			t.Errorf("NormalizeID(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeID(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "ABC", "Q-x", "Q--1"} {
		if _, err := NormalizeID(bad); err == nil {
			t.Errorf("expected error for NormalizeID(%q)", bad)
		}
	}
}

// =============================================================================
// List — ordenação e vazio
// =============================================================================

func TestList_EmptyStore(t *testing.T) {
	store := NewStore(t.TempDir())
	props, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(props) != 0 {
		t.Errorf("expected empty list, got %d", len(props))
	}
}

func TestList_SortedByID(t *testing.T) {
	store := NewStore(t.TempDir())
	addProps(t, store)
	props, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(props) != 2 {
		t.Fatalf("expected 2 proposals, got %d", len(props))
	}
	if props[0].ID != "Q-0001" || props[1].ID != "Q-0002" {
		t.Errorf("expected ascending order, got %q, %q", props[0].ID, props[1].ID)
	}
}

// =============================================================================
// Transição table documentada
// =============================================================================

func TestTransitionTable(t *testing.T) {
	table := TransitionTable()
	if !strings.Contains(table, "pending → validating") {
		t.Errorf("transition table missing the validation step: %q", table)
	}
	if !strings.Contains(table, "promoted") || !strings.Contains(table, "discarded") {
		t.Errorf("transition table missing terminal states: %q", table)
	}
}
