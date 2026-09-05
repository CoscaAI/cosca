package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── P0-4: Provenance/integridade — adulteração de 1 byte ───────────────
//
// Cenário do professor: alterar 1 byte no conhecimento → hash esperado !=
// hash atual → item marcado STALE.

func TestProvenance_OneByteChangeMarksStale(t *testing.T) {
	dir := t.TempDir()
	rel := writeSourceFile(t, dir, "knowledge/rules/database.md", "PostgreSQL é obrigatório")
	abs := filepath.Join(dir, filepath.FromSlash(rel))

	hash, err := HashFile(abs)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}

	item := &KnowledgeItem{ID: "K-500", Title: "Regra de banco"}
	ev := &Evidence{ID: "E-500", Path: rel, SHA256: hash}

	// Antes da adulteração: Changed=false.
	res, err := RevalidateEvidence(item, ev, dir)
	if err != nil || res.Changed {
		t.Fatalf("before tamper: Changed=%v err=%v", res.Changed, err)
	}

	// ADULTERAÇÃO: 1 byte (PostgreSQL → PostgreSQl).
	data, _ := os.ReadFile(abs)
	data[0] = 'p' // "postgreSQL..." — minúscula em vez de maiúscula
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err = RevalidateEvidence(item, ev, dir)
	if err != nil {
		t.Fatalf("RevalidateEvidence after tamper: %v", err)
	}
	if !res.Changed {
		t.Fatal("1-byte tamper must be detected (hash mismatch)")
	}
	if res.RecordedHash == res.CurrentHash {
		t.Fatal("recorded hash must differ from current hash")
	}

	// ApplyAging marca o item STALE.
	changed := ApplyAging(item, []AgingResult{res})
	if !changed {
		t.Fatal("ApplyAging must mark the item changed")
	}
	if item.Status != StatusStale {
		t.Fatalf("item status = %q, want stale", item.Status)
	}
}

// TestProvenance_MissingSourceFileIsStale: fonte sumiu → erro propagado.
func TestProvenance_MissingSourceFileIsStale(t *testing.T) {
	dir := t.TempDir()
	rel := writeSourceFile(t, dir, "docs/x.md", "conteúdo")
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	hash, _ := HashFile(abs)
	os.Remove(abs) // fonte desapareceu

	item := &KnowledgeItem{ID: "K-501"}
	ev := &Evidence{ID: "E-501", Path: rel, SHA256: hash}
	_, err := RevalidateEvidence(item, ev, dir)
	if err == nil {
		t.Fatal("missing source file must propagate an error")
	}
}

// TestProvenance_EvidenceWithoutPathIsSkipped: sem hash/path → skipped,
// nunca tratado como adulterado (falso positivo zero).
func TestProvenance_EvidenceWithoutPathIsSkipped(t *testing.T) {
	item := &KnowledgeItem{ID: "K-502"}
	ev := &Evidence{ID: "E-502"} // sem Path nem SHA256
	res, err := RevalidateEvidence(item, ev, t.TempDir())
	if err != nil {
		t.Fatalf("RevalidateEvidence: %v", err)
	}
	if !res.Skipped {
		t.Fatalf("evidence without path/hash must be skipped, got %+v", res)
	}
}

// TestProvenance_HashFileDeterministic: o mesmo conteúdo → o mesmo hash.
func TestProvenance_HashFileDeterministic(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "f.txt")
	_ = os.WriteFile(f, []byte("stable content"), 0o644)
	h1, _ := HashFile(f)
	h2, _ := HashFile(f)
	if h1 != h2 || len(h1) != 64 {
		t.Fatalf("hash not deterministic: %q vs %q", h1, h2)
	}
	if strings.Contains(h1, "stable") {
		t.Fatal("hash must not embed content")
	}
}
