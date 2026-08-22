//
// Tests for knowledge aging — internal/knowledge/aging.go.
//
// Covers:
//   - HashFile: SHA-256 correto de um conteúdo conhecido; arquivo inexistente
//     → erro (hash vazio)
//   - RevalidateEvidence: hash confere → Changed=false; arquivo modificado →
//     Changed=true; sem SHA256/path → skipped (Skipped=true), sem erro
//   - RevalidateItem: itera todas as evidências do item na ordem
//   - ApplyAging: evidência mudou → item STALE + LastVerified atualizado;
//     inalterado → status não muda (apenas refresh); tudo pulado → nada muda
//
// A revalidação nunca apaga evidências e nunca promove/demote nível.

package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestHashFile_ComputesSHA256 verifica o hash de um conteúdo conhecido.
func TestHashFile_ComputesSHA256(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "foo.go")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	// sha256("hello")
	const want = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Errorf("HashFile = %q, want %q", got, want)
	}
}

// TestHashFile_MissingFileErrors verifica que um arquivo inexistente devolve
// hash vazio + erro (nunca um hash fabricado).
func TestHashFile_MissingFileErrors(t *testing.T) {
	t.Parallel()

	got, err := HashFile(filepath.Join(t.TempDir(), "nao-existe.go"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if got != "" {
		t.Errorf("HashFile on missing file = %q, want empty", got)
	}
}

// writeSourceFile escreve um arquivo-fonte de teste e devolve o caminho.
func writeSourceFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	return rel
}

// revalidateAgainstFile monta um item + evidência com o hash real do arquivo
// e roda RevalidateEvidence.
func revalidateAgainstFile(t *testing.T, dir, fileRel, content string) AgingResult {
	t.Helper()
	rel := writeSourceFile(t, dir, fileRel, content)
	hash, err := HashFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	item := &KnowledgeItem{ID: "K-442", Title: "API X funciona dessa maneira"}
	ev := &Evidence{ID: "E-1", Path: rel, SHA256: hash}
	res, err := RevalidateEvidence(item, ev, dir)
	if err != nil {
		t.Fatalf("RevalidateEvidence: %v", err)
	}
	return res
}

// TestRevalidateEvidence_Unchanged verifica: hash confere → Changed=false.
func TestRevalidateEvidence_Unchanged(t *testing.T) {
	t.Parallel()

	res := revalidateAgainstFile(t, t.TempDir(), "internal/runtime/foo.go", "v1")
	if res.Skipped {
		t.Error("expected evidence with hash+path to be revalidated, got skipped")
	}
	if res.Changed {
		t.Error("unchanged file should not be marked changed")
	}
	if res.CurrentHash == "" {
		t.Error("expected a current hash")
	}
	if res.RecordedHash != res.CurrentHash {
		t.Errorf("recorded %q != current %q", res.RecordedHash, res.CurrentHash)
	}
}

// TestRevalidateEvidence_Modified verifica: arquivo modificado → Changed=true
// com hash diferente do registrado.
func TestRevalidateEvidence_Modified(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rel := writeSourceFile(t, dir, "internal/runtime/foo.go", "v1")
	hash, err := HashFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	item := &KnowledgeItem{ID: "K-442"}
	ev := &Evidence{ID: "E-1", Path: rel, SHA256: hash}

	res, err := RevalidateEvidence(item, ev, dir)
	if err != nil {
		t.Fatalf("RevalidateEvidence: %v", err)
	}
	if res.Changed {
		t.Fatal("file not modified yet — should be unchanged")
	}

	// Nova release → arquivo alterado → hash diferente.
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.WriteFile(abs, []byte("v2 — API X mudou"), 0o644); err != nil {
		t.Fatalf("modify file: %v", err)
	}
	res2, err := RevalidateEvidence(item, ev, dir)
	if err != nil {
		t.Fatalf("RevalidateEvidence after modify: %v", err)
	}
	if !res2.Changed {
		t.Error("modified file should be marked changed")
	}
	if res2.CurrentHash == res2.RecordedHash {
		t.Error("current hash should differ from recorded hash")
	}
}

// TestRevalidateEvidence_Skipped verifica: sem SHA256 ou sem Path → skipped,
// Changed=false, SEM erro.
func TestRevalidateEvidence_Skipped(t *testing.T) {
	t.Parallel()

	item := &KnowledgeItem{ID: "K-1"}
	cases := []struct {
		name string
		ev   *Evidence
	}{
		{"no sha256 and no path", &Evidence{ID: "E-1", Kind: "test", Source: "s", Description: "d"}},
		{"path but no sha256", &Evidence{ID: "E-1", Path: "internal/runtime/foo.go"}},
		{"sha256 but no path", &Evidence{ID: "E-1", SHA256: "abc123"}},
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res, err := RevalidateEvidence(item, tt.ev, t.TempDir())
			if err != nil {
				t.Fatalf("expected no error for skipped evidence, got %v", err)
			}
			if !res.Skipped {
				t.Error("expected Skipped=true")
			}
			if res.Changed {
				t.Error("skipped evidence must not be changed")
			}
		})
	}
}

// TestRevalidateEvidence_MissingFile verifica: fonte local sumiu → erro
// propagado (o Don deve saber que a origem desapareceu).
func TestRevalidateEvidence_MissingFile(t *testing.T) {
	t.Parallel()

	item := &KnowledgeItem{ID: "K-1"}
	ev := &Evidence{ID: "E-1", Path: "internal/runtime/foo.go", SHA256: "abc123"}
	if _, err := RevalidateEvidence(item, ev, t.TempDir()); err == nil {
		t.Fatal("expected error when the local source file is missing")
	}
}

// TestRevalidateItem_IteratesAllEvidence verifica que RevalidateItem devolve
// um resultado por evidência, na ordem em que aparecem.
func TestRevalidateItem_IteratesAllEvidence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rel := writeSourceFile(t, dir, "internal/runtime/foo.go", "v1")
	hash, err := HashFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}

	item := &KnowledgeItem{
		ID: "K-442",
		Evidence: []Evidence{
			{ID: "E-1", Path: rel, SHA256: hash},
			{ID: "E-2", Kind: "incident", Source: "F001", Description: "sem fonte"},
			{ID: "E-3", Path: rel, SHA256: hash},
		},
	}

	results, err := RevalidateItem(item, dir)
	if err != nil {
		t.Fatalf("RevalidateItem: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	if results[0].EvidenceID != "E-1" || results[1].EvidenceID != "E-2" || results[2].EvidenceID != "E-3" {
		t.Errorf("evidence order not preserved: %+v", results)
	}
	if !results[1].Skipped {
		t.Error("evidence without hash should be skipped")
	}
	if results[1].Changed {
		t.Error("skipped evidence must not be changed")
	}
	if results[0].Changed || results[2].Changed {
		t.Error("unchanged files should not be changed")
	}
}

// TestApplyAging_MarksStale verifica: evidência mudou → item STALE e
// LastVerified atualizado; nada é apagado.
func TestApplyAging_MarksStale(t *testing.T) {
	t.Parallel()

	item := &KnowledgeItem{ID: "K-442", Status: StatusKnown}
	results := []AgingResult{
		{ItemID: "K-442", EvidenceID: "E-1", Changed: true},
		{ItemID: "K-442", EvidenceID: "E-2", Changed: false, Skipped: false},
	}

	changed := ApplyAging(item, results)
	if !changed {
		t.Error("expected ApplyAging to report a change")
	}
	if item.Status != StatusStale {
		t.Errorf("status = %q, want STALE", item.Status)
	}
	if item.LastVerified.IsZero() {
		t.Error("LastVerified should be set after a change")
	}
	if len(item.Evidence) != 0 {
		t.Error("ApplyAging must never add/remove evidence")
	}
}

// TestApplyAging_UnchangedRefreshOnly verifica: evidências inalteradas → o
// status NÃO muda (apenas LastVerified é atualizado — refresh).
func TestApplyAging_UnchangedRefreshOnly(t *testing.T) {
	t.Parallel()

	before := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	item := &KnowledgeItem{ID: "K-442", Status: StatusKnown, LastVerified: before}
	results := []AgingResult{
		{ItemID: "K-442", EvidenceID: "E-1", Changed: false},
		{ItemID: "K-442", EvidenceID: "E-2", Changed: false},
	}

	changed := ApplyAging(item, results)
	if changed {
		t.Error("no evidence changed — ApplyAging should not report a change")
	}
	if item.Status != StatusKnown {
		t.Errorf("status = %q, want KNOWN (no status change)", item.Status)
	}
	if item.LastVerified.Equal(before) {
		t.Error("LastVerified should be refreshed when evidence was verified")
	}
}

// TestApplyAging_AllSkippedChangesNothing verifica: nada foi verificado (tudo
// pulado) → nenhuma alteração de status nem refresh.
func TestApplyAging_AllSkippedChangesNothing(t *testing.T) {
	t.Parallel()

	before := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	item := &KnowledgeItem{ID: "K-1", Status: StatusKnown, LastVerified: before}
	results := []AgingResult{
		{ItemID: "K-1", EvidenceID: "E-1", Skipped: true},
	}

	changed := ApplyAging(item, results)
	if changed {
		t.Error("all-skipped must not report a change")
	}
	if item.Status != StatusKnown {
		t.Errorf("status = %q, want KNOWN", item.Status)
	}
	if !item.LastVerified.Equal(before) {
		t.Error("all-skipped must not refresh LastVerified")
	}
}

// TestApplyAging_NilItemIsSafe verifica que um item nulo não panics.
func TestApplyAging_NilItemIsSafe(t *testing.T) {
	t.Parallel()

	if ApplyAging(nil, []AgingResult{{Changed: true}}) {
		t.Error("ApplyAging(nil, ...) must not report a change")
	}
}

// TestApplyAging_NeverDeletes verifica o contrato "no regression": mesmo com
// mudança, evidências permanecem intactas.
func TestApplyAging_NeverDeletes(t *testing.T) {
	t.Parallel()

	item := &KnowledgeItem{
		ID:       "K-442",
		Status:   StatusKnown,
		Evidence: []Evidence{{ID: "E-1", Path: "internal/runtime/foo.go", SHA256: "abc"}},
	}
	ApplyAging(item, []AgingResult{{ItemID: "K-442", EvidenceID: "E-1", Changed: true}})
	if len(item.Evidence) != 1 {
		t.Fatalf("evidence deleted: len = %d, want 1", len(item.Evidence))
	}
	if item.Evidence[0].ID != "E-1" || item.Evidence[0].SHA256 != "abc" {
		t.Errorf("evidence mutated: %+v", item.Evidence[0])
	}
}

// TestRevalidateItem_NilItem verifica que um item nulo devolve erro.
func TestRevalidateItem_NilItem(t *testing.T) {
	t.Parallel()

	if _, err := RevalidateItem(nil, t.TempDir()); err == nil {
		t.Fatal("expected error for nil item")
	}
}

// TestHashFile_EmptyContentHash é uma âncora: o hash de conteúdo vazio deve
// ser o SHA-256 padrão conhecido (determinismo).
func TestHashFile_EmptyContentHash(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "vazio")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	// sha256("")
	const want = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("HashFile(empty) = %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, "e3b0c442") {
		t.Errorf("unexpected hash prefix: %q", got)
	}
}
