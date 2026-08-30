package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

// openTestFS abre uma KnowledgeFS num project root temporário.
func openTestFS(t *testing.T) (*KnowledgeFS, string) {
	t.Helper()
	root := t.TempDir()
	fs, err := NewKnowledgeFS(root)
	if err != nil {
		t.Fatalf("NewKnowledgeFS: %v", err)
	}
	return fs, root
}

// addTestObject adiciona um objeto de conteúdo fixo e devolve o hash.
func addTestObject(t *testing.T, fs *KnowledgeFS, content string, class KnowledgeEpistemic, path string) string {
	t.Helper()
	o, err := fs.Add([]byte(content), class, path)
	if err != nil {
		t.Fatalf("Add(%q): %v", content, err)
	}
	return o.Hash
}

// countBlobs conta os arquivos de blob (não o index.yaml) num diretório de objetos.
func countBlobs(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir %s: %v", dir, err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || e.Name() == knowledgeFSObjectList {
			continue
		}
		n++
	}
	return n
}

// =============================================================================
// (a) Snapshot copy-on-write: aponta para objetos SEM duplicar
// =============================================================================

func TestKnowledgeFS_SnapshotCoW_NoDuplication(t *testing.T) {
	fs, _ := openTestFS(t)

	hAlpha := addTestObject(t, fs, "alpha-content", EpistemicFACT, "knowledge/laws.json")
	hBeta := addTestObject(t, fs, "beta-content", EpistemicFACT, "knowledge/manifest.yaml")

	// Snapshot A referencia apenas o alpha; B referencia alpha+beta.
	snapA, err := fs.Snapshot("main", []string{hAlpha})
	if err != nil {
		t.Fatalf("Snapshot A: %v", err)
	}
	snapB, err := fs.Snapshot("experimental", []string{hAlpha, hBeta})
	if err != nil {
		t.Fatalf("Snapshot B: %v", err)
	}

	// B não duplica A: ambos referenciam o MESMO hash hAlpha.
	if len(snapA.Objects) != 1 || snapA.Objects[0] != hAlpha {
		t.Errorf("snapshot A deveria referenciar [%s], got %v", hAlpha, snapA.Objects)
	}
	if len(snapB.Objects) != 2 {
		t.Fatalf("snapshot B deveria referenciar 2 objetos, got %v", snapB.Objects)
	}
	if snapB.Objects[0] != hAlpha || snapB.Objects[1] != hBeta {
		t.Errorf("snapshot B deveria referenciar [%s %s], got %v", hAlpha, hBeta, snapB.Objects)
	}

	// O índice de objetos tem EXATAMENTE os 2 blobs (mais o index.yaml).
	if got := fs.ObjectsCount(); got != 2 {
		t.Errorf("índice de objetos deveria ter 2, got %d", got)
	}
	blobs := countBlobs(t, filepath.Join(fs.Dir(), knowledgeFSObjects))
	if blobs != 2 {
		t.Errorf("deveriam existir 2 blobs content-addressable (nenhum duplicado), got %d", blobs)
	}

	// O objeto alpha é o MESMO (dedup por conteúdo): adicionar de novo não duplica.
	dup, err := fs.Add([]byte("alpha-content"), EpistemicFACT, "knowledge/laws.json")
	if err != nil {
		t.Fatalf("Add dup: %v", err)
	}
	if dup.Hash != hAlpha {
		t.Errorf("conteúdo igual deveria deduplicar para %s, got %s", hAlpha, dup.Hash)
	}
	if got := fs.ObjectsCount(); got != 2 {
		t.Errorf("adicionar conteúdo igual não deveria criar novo objeto; count=%d", got)
	}
}

// =============================================================================
// (b) refs/HEAD troca e persiste
// =============================================================================

func TestKnowledgeFS_RefsHEAD_SwitchAndPersist(t *testing.T) {
	fs, root := openTestFS(t)
	h1 := addTestObject(t, fs, "one", EpistemicFACT, "knowledge/laws.json")
	h2 := addTestObject(t, fs, "two", EpistemicFACT, "knowledge/manifest.yaml")

	if _, err := fs.Snapshot("main", []string{h1}); err != nil {
		t.Fatalf("snapshot main: %v", err)
	}
	if _, err := fs.Snapshot("experimental", []string{h1, h2}); err != nil {
		t.Fatalf("snapshot experimental: %v", err)
	}

	// HEAD deve apontar para o último ref (experimental).
	head, err := fs.ReadHEAD()
	if err != nil {
		t.Fatalf("ReadHEAD: %v", err)
	}
	if head != "experimental" {
		t.Errorf("HEAD deveria ser 'experimental', got %q", head)
	}

	// Checkout de volta para main → HEAD vira 'main'.
	if _, err := fs.Checkout("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	head, _ = fs.ReadHEAD()
	if head != "main" {
		t.Errorf("após checkout, HEAD deveria ser 'main', got %q", head)
	}
	cur, err := fs.ResolveHEAD()
	if err != nil {
		t.Fatalf("ResolveHEAD: %v", err)
	}
	if cur.ID == "" || len(cur.Objects) != 1 || cur.Objects[0] != h1 {
		t.Errorf("main deveria resolver para [%s], got %v", h1, cur.Objects)
	}

	// PERSISTE: reabrir a store deve manter refs/HEAD + refs/main/experimental.
	reopened, err := NewKnowledgeFS(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	head, err = reopened.ReadHEAD()
	if err != nil {
		t.Fatalf("reopen ReadHEAD: %v", err)
	}
	if head != "main" {
		t.Errorf("HEAD não persistiu; após reopen = %q", head)
	}
	if v, err := reopened.RefValue("main"); err != nil || v == "" {
		t.Errorf("refs/main não persistiu: %q err=%v", v, err)
	}
	if v, err := reopened.RefValue("experimental"); err != nil || v == "" {
		t.Errorf("refs/experimental não persistiu: %q err=%v", v, err)
	}

	// Refs listadas excluem HEAD.
	refs, err := reopened.Refs()
	if err != nil {
		t.Fatalf("Refs: %v", err)
	}
	want := map[string]bool{"main": true, "experimental": true}
	if len(refs) != 2 {
		t.Fatalf("Refs deveria ter 2, got %v", refs)
	}
	for _, r := range refs {
		if !want[r] {
			t.Errorf("ref inesperado %q", r)
		}
	}
}

func TestKnowledgeFS_Checkout_DetachedSnapshotID(t *testing.T) {
	fs, root := openTestFS(t)
	h := addTestObject(t, fs, "content", EpistemicFACT, "knowledge/laws.json")
	snap, err := fs.Snapshot("main", []string{h})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if _, err := fs.Checkout(snap.ID); err != nil {
		t.Fatalf("checkout por id: %v", err)
	}
	head, _ := fs.ReadHEAD()
	if head != snap.ID {
		t.Errorf("HEAD detached deveria ser %q, got %q", snap.ID, head)
	}

	// Snapshot id inexistente → erro.
	if _, err := fs.Checkout("ks_19990101_999"); err == nil {
		t.Error("checkout de snapshot inexistente deveria falhar")
	}
	// Ref inexistente → erro.
	if _, err := fs.Checkout("nao-existe"); err == nil {
		t.Error("checkout de ref inexistente deveria falhar")
	}

	// Persistência do detached HEAD.
	reopened, _ := NewKnowledgeFS(root)
	h2, _ := reopened.ReadHEAD()
	if h2 != snap.ID {
		t.Errorf("HEAD detached não persistiu: %q", h2)
	}
}

// =============================================================================
// (d) diff cognitivo entre snapshots
// =============================================================================

func TestKnowledgeFS_Diff_Cognitive(t *testing.T) {
	fs, _ := openTestFS(t)

	// Estado A: laws.json (FACT), next docs (FACT), hall-of-fame (FACT).
	hLawsV1 := addTestObject(t, fs, "laws-content-v1", EpistemicFACT, "knowledge/laws.json")
	hNext := addTestObject(t, fs, "next-docs", EpistemicFACT, "knowledge/acquired/next/docs.md")
	hHall := addTestObject(t, fs, "hall-content", EpistemicFACT, "knowledge/hall-of-fame.json")
	if _, err := fs.Snapshot("main", []string{hLawsV1, hNext, hHall}); err != nil {
		t.Fatalf("snapshot A: %v", err)
	}

	// Estado B: laws.json MUDOU (v2, mesmo path), next docs EMIGUAL (same hash),
	// hall-of-fame REMOVIDO, e um packages/ui.json NOVO.
	hLawsV2 := addTestObject(t, fs, "laws-content-v2", EpistemicFACT, "knowledge/laws.json")
	hUI := addTestObject(t, fs, "ui-pkg", EpistemicDECISION, "knowledge/packages/ui.json")
	if _, err := fs.Snapshot("experimental", []string{hLawsV2, hNext, hUI}); err != nil {
		t.Fatalf("snapshot B: %v", err)
	}

	diffs, err := fs.Diff("main", "experimental")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	byPath := map[string]ObjectDiff{}
	for _, d := range diffs {
		byPath[d.Path] = d
	}

	// changed: laws.json (conteúdo mudou, mesmo path).
	if d, ok := byPath["knowledge/laws.json"]; !ok || d.Change != DiffChanged || d.Class != EpistemicFACT {
		t.Errorf("laws.json deveria estar CHANGED (FACT), got %+v", byPath["knowledge/laws.json"])
	}
	// added: packages/ui.json (novo, DECISION).
	if d, ok := byPath["knowledge/packages/ui.json"]; !ok || d.Change != DiffAdded || d.Class != EpistemicDECISION {
		t.Errorf("ui.json deveria estar ADDED (DECISION), got %+v", byPath["knowledge/packages/ui.json"])
	}
	// removed: hall-of-fame.json (só em A).
	if d, ok := byPath["knowledge/hall-of-fame.json"]; !ok || d.Change != DiffRemoved || d.Class != EpistemicFACT {
		t.Errorf("hall-of-fame.json deveria estar REMOVED, got %+v", byPath["knowledge/hall-of-fame.json"])
	}
	// next docs inalterado → NÃO aparece no diff.
	if _, ok := byPath["knowledge/acquired/next/docs.md"]; ok {
		t.Errorf("next/docs.md não mudou — não deveria aparecer no diff")
	}
	// Exatamente 3 entradas.
	if len(diffs) != 3 {
		t.Errorf("diff deveria ter 3 entradas, got %d: %+v", len(diffs), diffs)
	}
}

// =============================================================================
// IDs de snapshot
// =============================================================================

func TestKnowledgeFS_SnapshotIDsUniqueAndSequential(t *testing.T) {
	fs, _ := openTestFS(t)
	h := addTestObject(t, fs, "content", EpistemicFACT, "knowledge/laws.json")

	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		snap, err := fs.Snapshot("main", []string{h})
		if err != nil {
			t.Fatalf("snapshot %d: %v", i, err)
		}
		if !isSnapshotID(snap.ID) {
			t.Errorf("id %q deveria ser ks_<data>_<seq>", snap.ID)
		}
		if seen[snap.ID] {
			t.Errorf("id de snapshot duplicado %q", snap.ID)
		}
		seen[snap.ID] = true
	}

	snaps, err := fs.ListSnapshots()
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(snaps) != 3 {
		t.Errorf("deveria haver 3 snapshots, got %d", len(snaps))
	}
	if snaps[0].ID >= snaps[1].ID || snaps[1].ID >= snaps[2].ID {
		t.Errorf("snapshots deveriam estar ordenados por id: %v", []string{snaps[0].ID, snaps[1].ID, snaps[2].ID})
	}
}
