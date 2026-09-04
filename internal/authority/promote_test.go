package authority

import (
	"path/filepath"
	"testing"
)

func TestPromote_Approved_WithBaselineAndReread(t *testing.T) {
	// T8: promoção aprovada — base hash ok + reread ok + sem conflito; gera
	// provenance + snapshot before/after.
	frozen := t.TempDir()
	live := t.TempDir()
	base := t.TempDir()

	// FROZEN tem "v1", LIVE tem "v2" (v2 é a evolução a promover).
	mustWrite(t, filepath.Join(frozen, "a.md"), "v1")
	mustWrite(t, filepath.Join(live, "a.md"), "v2")
	mustSetMtime(t, filepath.Join(frozen, "a.md"), tPast)
	mustSetMtime(t, filepath.Join(live, "a.md"), tFuture)

	// PROPOSE: registra live_sha + base_frozen_sha (revisão da FROZEN).
	prop, err := NewPromotionProposal(frozen, live, "a.md", "evoluir a.md para v2", "cosca-architecture")
	if err != nil {
		t.Fatalf("NewPromotionProposal: %v", err)
	}
	if prop.LiveSHA == "" || prop.BaseFrozenSHA == "" || prop.Material != "v2" {
		t.Fatalf("proposta inconsistente: %+v", prop)
	}

	// APPLY: reread imediatamente antes, sem conflito → APPLIED.
	res, err := Promote(frozen, live, prop, PromoteOptions{SnapshotBase: base})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if res.Outcome != OutcomeApplied {
		t.Fatalf("Outcome = %s, esperado APPLIED (reasons: %v)", res.Outcome, res.Reasons)
	}
	if res.PromotionID == "" || res.Actor != "cosca-architecture" {
		t.Errorf("proveniência incompleta: %+v", res)
	}
	if res.SnapshotBefore == "" || res.SnapshotAfter == "" {
		t.Errorf("snapshots before/after ausentes: %+v", res)
	}
	if !exists(res.SnapshotBefore) || !exists(res.SnapshotAfter) {
		t.Errorf("snapshots não existem em disco")
	}
	// O FROZEN agora contém o conteúdo promovido.
	if got := mustRead(t, filepath.Join(frozen, "a.md")); got != "v2" {
		t.Errorf("FROZEN a.md = %q, esperado v2 (promovido)", got)
	}
	// frozen_sha_after == sha do conteúdo promovido.
	if res.FrozenSHAAfter != prop.LiveSHA {
		t.Errorf("frozen_sha_after=%s, esperado live_sha=%s", res.FrozenSHAAfter, prop.LiveSHA)
	}
	// O LIVE não é alterado pela promoção.
	if got := mustRead(t, filepath.Join(live, "a.md")); got != "v2" {
		t.Errorf("LIVE a.md = %q, não devia mudar", got)
	}
}

func TestPromote_RejectedConflict_FrozenChanged(t *testing.T) {
	// T9: promoção rejeitada — a FROZEN mudou desde a proposta (base sha divergente).
	frozen := t.TempDir()
	live := t.TempDir()
	base := t.TempDir()

	mustWrite(t, filepath.Join(frozen, "a.md"), "v1")
	mustWrite(t, filepath.Join(live, "a.md"), "v2")
	mustSetMtime(t, filepath.Join(frozen, "a.md"), tPast)
	mustSetMtime(t, filepath.Join(live, "a.md"), tFuture)

	prop, err := NewPromotionProposal(frozen, live, "a.md", "evoluir", "cosca-architecture")
	if err != nil {
		t.Fatalf("NewPromotionProposal: %v", err)
	}
	// A FROZEN muda depois da proposta (outro agente alterou a autoridade).
	mustWrite(t, filepath.Join(frozen, "a.md"), "v1-shifted")

	res, err := Promote(frozen, live, prop, PromoteOptions{SnapshotBase: base})
	if err != nil {
		t.Fatalf("Promote deve devolver resultado, não erro: %v", err)
	}
	if res.Outcome != OutcomeRejectedConflict {
		t.Fatalf("Outcome = %s, esperado REJECTED_CONFLICT (reasons: %v)", res.Outcome, res.Reasons)
	}
	// Nada escrito: a FROZEN mantém o conteúdo alterado.
	if got := mustRead(t, filepath.Join(frozen, "a.md")); got != "v1-shifted" {
		t.Errorf("FROZEN a.md = %q, nada devia ser escrito", got)
	}
	// Nenhum snapshot after deveria existir.
	entries, _ := filepath.Glob(filepath.Join(base, "promote-*", "after"))
	if len(entries) != 0 {
		t.Errorf("conflito ainda escreveu snapshot after: %v", entries)
	}
}

func TestPromote_RejectedInvalid_LiveChanged(t *testing.T) {
	// T9-invalid: live_sha atual diverge do proposto → REJECTED_INVALID, repropor.
	frozen := t.TempDir()
	live := t.TempDir()
	base := t.TempDir()

	mustWrite(t, filepath.Join(frozen, "a.md"), "v1")
	mustWrite(t, filepath.Join(live, "a.md"), "v2")
	mustSetMtime(t, filepath.Join(frozen, "a.md"), tPast)
	mustSetMtime(t, filepath.Join(live, "a.md"), tFuture)

	prop, _ := NewPromotionProposal(frozen, live, "a.md", "evoluir", "cosca-architecture")
	// O LIVE muda desde a proposta.
	mustWrite(t, filepath.Join(live, "a.md"), "v3")

	res, _ := Promote(frozen, live, prop, PromoteOptions{SnapshotBase: base})
	if res.Outcome != OutcomeRejectedInvalid {
		t.Fatalf("Outcome = %s, esperado REJECTED_INVALID (reasons: %v)", res.Outcome, res.Reasons)
	}
	if got := mustRead(t, filepath.Join(frozen, "a.md")); got != "v1" {
		t.Errorf("FROZEN a.md = %q, nada devia ser escrito", got)
	}
}

func TestPromote_RejectedNoIntent(t *testing.T) {
	// Sem intenção explícita (proposta nula) → REJECTED_NO_INTENT, nada escrito.
	frozen := t.TempDir()
	live := t.TempDir()
	mustWrite(t, filepath.Join(live, "a.md"), "v2")

	res, _ := Promote(frozen, live, nil, PromoteOptions{})
	if res.Outcome != OutcomeRejectedNoIntent {
		t.Fatalf("Outcome = %s, esperado REJECTED_NO_INTENT", res.Outcome)
	}
}

func TestPromote_Idempotent_AlreadySynced(t *testing.T) {
	// T12: rodar promote com a mesma proposta já aplicada → no-op (already-synced).
	frozen := t.TempDir()
	live := t.TempDir()
	base := t.TempDir()

	mustWrite(t, filepath.Join(frozen, "a.md"), "v1")
	mustWrite(t, filepath.Join(live, "a.md"), "v2")
	mustSetMtime(t, filepath.Join(frozen, "a.md"), tPast)
	mustSetMtime(t, filepath.Join(live, "a.md"), tFuture)

	prop, _ := NewPromotionProposal(frozen, live, "a.md", "evoluir", "cosca-architecture")

	res1, _ := Promote(frozen, live, prop, PromoteOptions{SnapshotBase: base})
	if res1.Outcome != OutcomeApplied {
		t.Fatalf("1ª promoção Outcome = %s", res1.Outcome)
	}

	// Segunda vez com a MESMA proposta: FROZEN já contém v2 → no-op.
	res2, _ := Promote(frozen, live, prop, PromoteOptions{SnapshotBase: base})
	if res2.Outcome != OutcomeApplied {
		t.Fatalf("2ª promoção (idempotente) Outcome = %s, esperado APPLIED no-op (reasons: %v)", res2.Outcome, res2.Reasons)
	}
	if got := mustRead(t, filepath.Join(frozen, "a.md")); got != "v2" {
		t.Errorf("FROZEN a.md = %q, idempotência quebrada", got)
	}
}

func TestPromote_NeverWritesFrozenWithoutIntent(t *testing.T) {
	// A promoção é o ÚNICO caminho; sem proposta válida o FROZEN não muda.
	frozen := t.TempDir()
	live := t.TempDir()

	mustWrite(t, filepath.Join(frozen, "a.md"), "v1")
	mustWrite(t, filepath.Join(live, "a.md"), "v2")
	mustSetMtime(t, filepath.Join(frozen, "a.md"), tPast)
	mustSetMtime(t, filepath.Join(live, "a.md"), tFuture)

	// Proposta para um path que não existe no LIVE (só no FROZEN) → FROZEN_NEWER.
	_, err := NewPromotionProposal(frozen, live, "only-frozen.md", "evoluir", "cosca-architecture")
	// only-frozen.md não existe no LIVE → NewPromotionProposal deve falhar.
	if err == nil {
		t.Fatalf("NewPromotionProposal deveria falhar para path ausente no LIVE")
	}
}
