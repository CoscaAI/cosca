package authority

import (
	"path/filepath"
	"testing"
)

func TestReconcile_FrozenNewer_UpdatesLive(t *testing.T) {
	// T2: FROZEN_NEWER → reconcile FROZEN→LIVE atualiza o LIVE.
	frozen := t.TempDir()
	live := t.TempDir()
	base := t.TempDir()

	mustWrite(t, filepath.Join(frozen, "a.md"), "frozen-v2")
	mustWrite(t, filepath.Join(live, "a.md"), "live-v1")
	mustSetMtime(t, filepath.Join(frozen, "a.md"), tFuture)
	mustSetMtime(t, filepath.Join(live, "a.md"), tPast)

	plan, err := PlanReconcile(frozen, live)
	if err != nil {
		t.Fatalf("PlanReconcile: %v", err)
	}
	if plan.Outcome != "dry-run" {
		t.Errorf("Outcome = %q, esperado dry-run", plan.Outcome)
	}
	if plan.Applied {
		t.Errorf("dry-run não pode marcar Applied")
	}
	// Dry-run não escreve nada: live permanece "live-v1".
	if got := mustRead(t, filepath.Join(live, "a.md")); got != "live-v1" {
		t.Fatalf("dry-run alterou o LIVE: %q", got)
	}
	// A ação foi update.
	if plan.Actions[0].Op != "update" || plan.Actions[0].State != StateFrozenNewer {
		t.Errorf("ação 0 = %+v, esperado update/FROZEN_NEWER", plan.Actions[0])
	}

	res, err := ApplyReconcile(frozen, live, plan, ReconcileOptions{SnapshotBase: base})
	if err != nil {
		t.Fatalf("ApplyReconcile: %v", err)
	}
	if res.Outcome != "applied" {
		t.Errorf("Outcome = %q, esperado applied", res.Outcome)
	}
	if got := mustRead(t, filepath.Join(live, "a.md")); got != "frozen-v2" {
		t.Errorf("LIVE a.md = %q, esperado frozen-v2 (atualizado do FROZEN)", got)
	}
	if res.SnapshotBefore == "" || !exists(res.SnapshotBefore) {
		t.Errorf("snapshot_before não criado: %q", res.SnapshotBefore)
	}
	// O FROZEN nunca muda por reconciliar.
	if got := mustRead(t, filepath.Join(frozen, "a.md")); got != "frozen-v2" {
		t.Errorf("FROZEN a.md = %q, não devia mudar", got)
	}
}

func TestReconcile_LiveNewer_PreservesEvidence(t *testing.T) {
	// T3: LIVE_NEWER → reconciliar preserva o LIVE como evidência (não sobrescreve).
	frozen := t.TempDir()
	live := t.TempDir()
	base := t.TempDir()

	mustWrite(t, filepath.Join(frozen, "a.md"), "frozen-v1")
	mustWrite(t, filepath.Join(live, "a.md"), "live-v2")
	mustSetMtime(t, filepath.Join(frozen, "a.md"), tPast)
	mustSetMtime(t, filepath.Join(live, "a.md"), tFuture)

	plan, _ := PlanReconcile(frozen, live)
	res, err := ApplyReconcile(frozen, live, plan, ReconcileOptions{SnapshotBase: base})
	if err != nil {
		t.Fatalf("ApplyReconcile: %v", err)
	}
	// O LIVE NÃO é sobrescrito.
	if got := mustRead(t, filepath.Join(live, "a.md")); got != "live-v2" {
		t.Errorf("LIVE a.md = %q, NÃO devia ser sobrescrito (preservar evidência)", got)
	}
	// A evidência foi guardada.
	evid := filepath.Join(res.EvidenceDir, "a.md")
	if !exists(evid) {
		t.Fatalf("evidência não copiada: %q", evid)
	}
	if got := mustRead(t, evid); got != "live-v2" {
		t.Errorf("evidência = %q, esperado live-v2", got)
	}
}

func TestReconcile_OnlyFrozen_Preserved(t *testing.T) {
	// T5: ONLY_FROZEN → nunca deletado; não copiado ao LIVE por padrão; nunca deletar.
	frozen := t.TempDir()
	live := t.TempDir()
	base := t.TempDir()

	mustWrite(t, filepath.Join(frozen, "blocks", "agent1.md"), "bloco")
	mustWrite(t, filepath.Join(frozen, "chain.dat"), "cadena")
	mustWrite(t, filepath.Join(frozen, "protocol", "x.md"), "proto")

	plan, _ := PlanReconcile(frozen, live)
	for _, a := range plan.Actions {
		if a.Op != "preserve" {
			t.Errorf("ação para %q = %q, esperado preserve", a.Path, a.Op)
		}
	}
	res, err := ApplyReconcile(frozen, live, plan, ReconcileOptions{SnapshotBase: base})
	if err != nil {
		t.Fatalf("ApplyReconcile: %v", err)
	}
	if res.Outcome != "no-change" {
		t.Errorf("Outcome = %q, esperado no-change (só preserve/skip)", res.Outcome)
	}
	// Exclusivos do FROZEN intactos.
	for _, p := range []string{"blocks/agent1.md", "chain.dat", "protocol/x.md"} {
		if !exists(filepath.Join(frozen, p)) {
			t.Errorf("exclusivo do FROZEN %q sumiu", p)
		}
	}
	// Não copiado ao LIVE por padrão.
	if exists(filepath.Join(live, "blocks", "agent1.md")) || exists(filepath.Join(live, "chain.dat")) {
		t.Errorf("ONLY_FROZEN foi copiado ao LIVE — proibido por padrão (G5)")
	}
}

func TestReconcile_OnlyLive_NotDeleted(t *testing.T) {
	// T6: ONLY_LIVE → reconciliar não bloqueia/não apaga; registra como evidência.
	frozen := t.TempDir()
	live := t.TempDir()
	base := t.TempDir()

	mustWrite(t, filepath.Join(live, "skills", "evoluir.md"), "aprendizado")

	plan, _ := PlanReconcile(frozen, live)
	res, err := ApplyReconcile(frozen, live, plan, ReconcileOptions{SnapshotBase: base})
	if err != nil {
		t.Fatalf("ApplyReconcile: %v", err)
	}
	// O arquivo vivo do LIVE NÃO é removido.
	if !exists(filepath.Join(live, "skills", "evoluir.md")) {
		t.Fatalf("ONLY_LIVE foi removido — viola G2/G4")
	}
	if got := mustRead(t, filepath.Join(live, "skills", "evoluir.md")); got != "aprendizado" {
		t.Errorf("ONLY_LIVE conteúdo alterado: %q", got)
	}
	// Registro como evidência.
	evid := filepath.Join(res.EvidenceDir, "skills", "evoluir.md")
	if !exists(evid) {
		t.Fatalf("evidência do ONLY_LIVE não registrada: %q", evid)
	}
}

func TestReconcile_IdempotentTwice(t *testing.T) {
	// T12: reconciliar 2× = 1× (mesmo resultado), sem corrupção.
	frozen := t.TempDir()
	live := t.TempDir()
	base := t.TempDir()

	mustWrite(t, filepath.Join(frozen, "a.md"), "frozen-v2")
	mustWrite(t, filepath.Join(live, "a.md"), "live-v1")
	mustSetMtime(t, filepath.Join(frozen, "a.md"), tFuture)
	mustSetMtime(t, filepath.Join(live, "a.md"), tPast)

	plan1, _ := PlanReconcile(frozen, live)
	res1, err := ApplyReconcile(frozen, live, plan1, ReconcileOptions{SnapshotBase: base})
	if err != nil {
		t.Fatalf("1ª apply: %v", err)
	}
	after1 := mustRead(t, filepath.Join(live, "a.md"))

	// 2ª rodada: agora a.md é MATCH → plan2 não tem update → no-change.
	plan2, _ := PlanReconcile(frozen, live)
	res2, err := ApplyReconcile(frozen, live, plan2, ReconcileOptions{SnapshotBase: base})
	if err != nil {
		t.Fatalf("2ª apply: %v", err)
	}
	if res2.Outcome != "no-change" {
		t.Errorf("2ª rodada Outcome = %q, esperado no-change (idempotente)", res2.Outcome)
	}
	if after1 != mustRead(t, filepath.Join(live, "a.md")) {
		t.Errorf("2ª rodada alterou o LIVE: %q → %q", after1, mustRead(t, filepath.Join(live, "a.md")))
	}
	_ = res1
}

func TestReconcile_FailClosed_PlanApplyConflict(t *testing.T) {
	// T7 (reconcile): conflito plan→apply — o FROZEN/LIVE mudou entre dry-run e
	// apply; a aplicação é abortada fail-closed e nada é escrito.
	frozen := t.TempDir()
	live := t.TempDir()
	base := t.TempDir()

	mustWrite(t, filepath.Join(frozen, "a.md"), "v2")
	mustWrite(t, filepath.Join(live, "a.md"), "v1")
	mustSetMtime(t, filepath.Join(frozen, "a.md"), tFuture)
	mustSetMtime(t, filepath.Join(live, "a.md"), tPast)

	plan, _ := PlanReconcile(frozen, live)
	if plan.Actions[0].Op != "update" {
		t.Fatalf("pré-requisito: plano deve ter update, obteve %+v", plan.Actions[0])
	}

	// Entre o plan e o apply, o LIVE é alinhado ao FROZEN → estado muda para MATCH.
	mustWrite(t, filepath.Join(live, "a.md"), "v2")

	_, err := ApplyReconcile(frozen, live, plan, ReconcileOptions{SnapshotBase: base})
	if err == nil {
		t.Fatalf("esperava erro fail-closed (conflito plan→apply), obteve nil")
	}
	// Nada foi escrito: nenhum snapshot de reconcile foi criado.
	entries, _ := filepath.Glob(filepath.Join(base, "reconcile-*"))
	if len(entries) != 0 {
		t.Errorf("fail-closed escreveu snapshot: %v", entries)
	}
}

func TestReconcile_NeverRemovesLiveSurface(t *testing.T) {
	// T15/T16 guard-rail (G2/G5): reconciliar NUNCA remove superfície do OpenCode
	// nem exclusivos do FROZEN — compara o conjunto de arquivos do LIVE antes/depois.
	frozen := t.TempDir()
	live := t.TempDir()
	base := t.TempDir()

	// Superfície LIVE (templates do OpenCode — G2).
	mustWrite(t, filepath.Join(live, "KERNEL.md"), "kernel")
	mustWrite(t, filepath.Join(live, "engines", "E.md"), "engine")
	mustWrite(t, filepath.Join(live, "workflows", "W.md"), "workflow")
	// Exclusivos do FROZEN (G5).
	mustWrite(t, filepath.Join(frozen, "memory", "agent", "blocks", "b.md"), "bloco")
	mustWrite(t, filepath.Join(frozen, "merkle", "m.dat"), "merkle")

	before := liveFileSet(t, live)

	plan, _ := PlanReconcile(frozen, live)
	if _, err := ApplyReconcile(frozen, live, plan, ReconcileOptions{SnapshotBase: base}); err != nil {
		t.Fatalf("ApplyReconcile: %v", err)
	}

	after := liveFileSet(t, live)
	for p := range before {
		if !after[p] {
			t.Errorf("reconciliar removeu a superfície do LIVE: %q", p)
		}
	}
	// As superfícies continuam com o mesmo conteúdo (não sobrescritas sem intenção).
	if mustRead(t, filepath.Join(live, "KERNEL.md")) != "kernel" {
		t.Errorf("KERNEL.md do LIVE alterado")
	}
	// Exclusivos do FROZEN intactos.
	for _, p := range []string{"memory/agent/blocks/b.md", "merkle/m.dat"} {
		if !exists(filepath.Join(frozen, p)) {
			t.Errorf("exclusivo do FROZEN %q removido", p)
		}
	}
}

// liveFileSet returns the set of canonical relative paths under root.
func liveFileSet(t *testing.T, root string) map[string]bool {
	t.Helper()
	metas, err := fileMetas(root)
	if err != nil {
		t.Fatalf("fileMetas: %v", err)
	}
	out := map[string]bool{}
	for _, fm := range metas {
		out[fm.Path] = true
	}
	return out
}
