package authority

import (
	"path/filepath"
	"testing"
)

func TestManifest_DeterministicAndSorted(t *testing.T) {
	// §6.1: manifesto da zona — caminhos + SHA-256, ordenado, revisão estável.
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "b.md"), "b")
	mustWrite(t, filepath.Join(root, "a.md"), "a")
	mustWrite(t, filepath.Join(root, "sub", "c.md"), "c")

	m1, err := BuildManifest(root, "FROZEN")
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	if m1.Zone != "FROZEN" || m1.Revision == "" {
		t.Fatalf("manifest inválido: zone=%q rev=%q", m1.Zone, m1.Revision)
	}
	// Ordenado pela canonical path.
	for i := 1; i < len(m1.Entries); i++ {
		if m1.Entries[i-1].Path > m1.Entries[i].Path {
			t.Errorf("manifest não ordenado: %q > %q", m1.Entries[i-1].Path, m1.Entries[i].Path)
		}
	}
	// Revisão determinística.
	rev2, _ := TreeRevision(root)
	if rev2 != m1.Revision {
		t.Errorf("TreeRevision = %q, manifest.Revision = %q — divergem", rev2, m1.Revision)
	}
	// Manifesto muda quando muda conteúdo.
	mustWrite(t, filepath.Join(root, "a.md"), "a-changed")
	rev3, _ := TreeRevision(root)
	if rev3 == m1.Revision {
		t.Errorf("revisão não detectou mudança de conteúdo")
	}
}

func TestManifest_WriteReadRoundTrip(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "x.md"), "x")
	m, _ := BuildManifest(root, "LIVE")
	dest := filepath.Join(t.TempDir(), "manifest.json")
	if err := WriteManifest(m, dest); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	m2, err := ReadManifest(dest)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if m2.Revision != m.Revision || len(m2.Entries) != len(m.Entries) {
		t.Errorf("round-trip quebrou: %+v vs %+v", m2.Revision, m.Revision)
	}
}

func TestRollback_RestoresExactState(t *testing.T) {
	// T10: rollback restaura exatamente o snapshot anterior; estado volta ao
	// hash de snapshot_before (DestructionExact).
	live := t.TempDir()
	snapBase := t.TempDir()

	mustWrite(t, filepath.Join(live, "a.md"), "v1")
	mustWrite(t, filepath.Join(live, "sub", "b.md"), "b1")

	before := filepath.Join(snapBase, "before")
	if _, err := SnapshotTree(live, before); err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}
	snapRev, _ := TreeRevision(before)

	// Simula escrita feita por uma reconciliação: a.md muda e um extra aparece.
	mustWrite(t, filepath.Join(live, "a.md"), "v2-pós-reconcile")
	mustWrite(t, filepath.Join(live, "extra.md"), "extra")

	res, err := Rollback(live, before, RollbackOptions{DestroyExtras: true})
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if !res.Exact {
		t.Errorf("rollback não exato: final=%q snapshot=%q", res.FinalRevision, res.SnapshotRevision)
	}
	if res.FinalRevision != snapRev {
		t.Errorf("final_revision=%q, esperado snapshot_revision=%q", res.FinalRevision, snapRev)
	}
	if got := mustRead(t, filepath.Join(live, "a.md")); got != "v1" {
		t.Errorf("a.md = %q, esperado v1 (restaurado)", got)
	}
	if exists(filepath.Join(live, "extra.md")) {
		t.Errorf("extra.md não foi removido pelo rollback exato")
	}
	if res.Removed == 0 {
		t.Errorf("esperava remover o extra no rollback exato")
	}
}

func TestRollback_DefaultIsNonDestructive(t *testing.T) {
	// §6.3: sem DestroyExtras o rollback é não-destrutivo (upsert-only); extras
	// são retidos.
	live := t.TempDir()
	snapBase := t.TempDir()
	mustWrite(t, filepath.Join(live, "a.md"), "v1")
	before := filepath.Join(snapBase, "before")
	SnapshotTree(live, before)

	mustWrite(t, filepath.Join(live, "a.md"), "v2")
	mustWrite(t, filepath.Join(live, "extra.md"), "extra")

	res, _ := Rollback(live, before, RollbackOptions{})
	if got := mustRead(t, filepath.Join(live, "a.md")); got != "v1" {
		t.Errorf("a.md = %q, esperado v1 (restaurado)", got)
	}
	if !exists(filepath.Join(live, "extra.md")) {
		t.Errorf("extra.md deve ser retido quando DestroyExtras=false")
	}
	if res.Removed != 0 {
		t.Errorf("rollback não-destrutivo removeu %d arquivo(s)", res.Removed)
	}
}

func TestReconcile_NeverWritesFrozen(t *testing.T) {
	// G1 guard-rail: reconciliar nunca escreve no FROZEN (só a promoção escrita
	// em que o FROZEN é autoridade ao promover). Revisão do FROZEN intacta.
	frozen := t.TempDir()
	live := t.TempDir()
	base := t.TempDir()

	mustWrite(t, filepath.Join(frozen, "a.md"), "frozen-v2")
	mustWrite(t, filepath.Join(live, "a.md"), "live-v1")
	mustSetMtime(t, filepath.Join(frozen, "a.md"), tFuture)
	mustSetMtime(t, filepath.Join(live, "a.md"), tPast)

	frozenRevBefore, _ := TreeRevision(frozen)

	plan, _ := PlanReconcile(frozen, live)
	if _, err := ApplyReconcile(frozen, live, plan, ReconcileOptions{SnapshotBase: base}); err != nil {
		t.Fatalf("ApplyReconcile: %v", err)
	}

	frozenRevAfter, _ := TreeRevision(frozen)
	if frozenRevAfter != frozenRevBefore {
		t.Errorf("reconciliar alterou o FROZEN (G1/§5): revisão %q → %q", frozenRevBefore, frozenRevAfter)
	}
}
