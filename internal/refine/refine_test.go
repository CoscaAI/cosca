package refine

import (
	"errors"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func seedSkill(s *State, id, content string) {
	s.Set(KindSkill, id, Entry{ID: id, Kind: KindSkill, Title: id, Content: content, Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})
}

func proposal(edits ...Edit) Proposal {
	return Proposal{Summary: "p", Rationale: "root cause identified", Edits: edits, ExpectedOutcome: "better"}
}

// ---------------------------------------------------------------------------
// (a) P1 — todo registro promovido tem evidence não-vazio + trace_id (fail-closed)
// ---------------------------------------------------------------------------

func TestApply_RequiresEvidenceAndTraceID_FailClosed(t *testing.T) {
	s := NewState()
	seedSkill(s, "net-catcher", "v1")

	// Sem evidence → fail-closed (I2): nunca promove sem justificativa.
	plan := PlanRefinement(s, "a1", Proposal{Summary: "p", Edits: []Edit{{Action: ActionUpdate, Kind: KindSkill, ID: "net-catcher", Content: "v2"}}}, "trace-1", "")
	if _, err := Apply(s, plan); !errors.Is(err, ErrMissingEvidence) {
		t.Fatalf("esperava ErrMissingEvidence, got %v", err)
	}

	// Sem trace_id → fail-closed (I4).
	plan = PlanRefinement(s, "a2", proposal(Edit{Action: ActionUpdate, Kind: KindSkill, ID: "net-catcher", Content: "v2"}), "", "")
	if _, err := Apply(s, plan); !errors.Is(err, ErrMissingTraceID) {
		t.Fatalf("esperava ErrMissingTraceID, got %v", err)
	}

	// Nenhum registro deve ter sido gravado (log limpo).
	if recs := s.Records(); len(recs) != 0 {
		t.Fatalf("autoedits rejeitados não deveriam gravar registro: %d", len(recs))
	}
}

func TestApply_PromotedRecord_CarriesEvidenceAndTrace(t *testing.T) {
	s := NewState()
	seedSkill(s, "net-catcher", "v1")

	res, err := Apply(s, PlanRefinement(s, "p1", proposal(Edit{Action: ActionUpdate, Kind: KindSkill, ID: "net-catcher", Content: "v2"}), "trace-x", ""))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	rec := res.Record
	if rec.Evidence == "" {
		t.Fatalf("registro promovido deve ter evidence não-vazio")
	}
	if rec.TraceID == "" {
		t.Fatalf("registro promovido deve ter trace_id (I4)")
	}
	if rec.Changes[0] != "update skill:net-catcher" {
		t.Fatalf("changes inesperadas: %v", rec.Changes)
	}
	if rec.Version == 0 {
		t.Fatalf("versão do registro deve ser monotônica > 0")
	}
}

// ---------------------------------------------------------------------------
// (b) P1 — rollback reconstrói o estado a partir de before/after
// ---------------------------------------------------------------------------

func TestRollback_RestoresFromBeforeAfter(t *testing.T) {
	s := NewState()
	seedSkill(s, "net-catcher", "v1")

	before, _ := s.Get(KindSkill, "net-catcher")
	res, err := Apply(s, PlanRefinement(s, "r1", proposal(Edit{Action: ActionUpdate, Kind: KindSkill, ID: "net-catcher", Content: "v2"}), "trace-1", ""))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if after, _ := s.Get(KindSkill, "net-catcher"); after.Content != "v2" {
		t.Fatalf("apply deveria ter gravado v2, got %s", after.Content)
	}

	rb, err := Rollback(s, res, "trace-1")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rb.RollbackOf != res.ID {
		t.Fatalf("rollback deve apontar para %s, got %s", res.ID, rb.RollbackOf)
	}
	got, _ := s.Get(KindSkill, "net-catcher")
	if got.Content != before.Content || got.Version != before.Version {
		t.Fatalf("rollback deve reconstruir %q v%d, got %q v%d", before.Content, before.Version, got.Content, got.Version)
	}
}

func TestRollback_DeletesCreatedEntry(t *testing.T) {
	s := NewState()
	res, err := Apply(s, PlanRefinement(s, "c1",
		proposal(Edit{Action: ActionCreate, Kind: KindMemory, ID: "note", Title: "note", Content: "hello"}),
		"trace-1", ""))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, ok := s.Get(KindMemory, "note"); !ok {
		t.Fatalf("create deveria ter gravado a entrada")
	}
	if _, err := Rollback(s, res, "trace-1"); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if _, ok := s.Get(KindMemory, "note"); ok {
		t.Fatalf("rollback deveria ter removido a entrada criada")
	}
}

func TestRollback_RecreatesDeletedEntry(t *testing.T) {
	s := NewState()
	seedSkill(s, "net-catcher", "v1")

	res, err := Apply(s, PlanRefinement(s, "d1", proposal(Edit{Action: ActionDelete, Kind: KindSkill, ID: "net-catcher"}), "trace-1", ""))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, ok := s.Get(KindSkill, "net-catcher"); ok {
		t.Fatalf("delete deveria ter removido a entrada")
	}
	if _, err := Rollback(s, res, "trace-1"); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	got, ok := s.Get(KindSkill, "net-catcher")
	if !ok || got.Content != "v1" {
		t.Fatalf("rollback deveria recriar %q, got %+v ok=%v", "v1", got, ok)
	}
}

// ---------------------------------------------------------------------------
// (c) P2 — baseline-conflict: aplicador com baseline obsoleto é rejeitado
// ---------------------------------------------------------------------------

func TestApply_TwoConcurrentApplicators_SecondStaleBaselineRejected(t *testing.T) {
	s := NewState()
	seedSkill(s, "net-catcher", "v1")

	// Dois aplicadores planejam a MESMA entrada ao mesmo tempo (baseline v1).
	appA := PlanRefinement(s, "A", proposal(Edit{Action: ActionUpdate, Kind: KindSkill, ID: "net-catcher", Content: "v2"}), "trace-a", "")
	appB := PlanRefinement(s, "B", proposal(Edit{Action: ActionUpdate, Kind: KindSkill, ID: "net-catcher", Content: "v3"}), "trace-b", "")

	// A aplica primeiro: estado v1 → v2.
	resA, err := Apply(s, appA)
	if err != nil {
		t.Fatalf("aplicador A: %v", err)
	}
	if got, _ := s.Get(KindSkill, "net-catcher"); got.Content != "v2" {
		t.Fatalf("A deveria ter gravado v2, got %s", got.Content)
	}
	if !resA.AppliedEdits[0].Applied {
		t.Fatalf("A deveria ter aplicado o update")
	}

	// B aplica com baseline obsoleto (v1) — antes = v2 ≠ baseline → REJEITADO.
	resB, err := Apply(s, appB)
	if err != nil {
		t.Fatalf("aplicador B não deve dar erro fatal (o edit é rejeitado): %v", err)
	}
	be := resB.AppliedEdits[0]
	if be.Applied {
		t.Fatalf("B com baseline obsoleto deve ser rejeitado (default-não-clobber)")
	}
	// O motivo deve ser exatamente o da mensagem do erro tipado.
	canonical := (&EntryChangedDuringPlanError{Kind: KindSkill, ID: "net-catcher"}).Error()
	if be.Error != canonical {
		t.Fatalf("motivo deve vir do erro tipado %q, got %q", canonical, be.Error)
	}
	if !strings.Contains(be.Error, "entry changed during refinement planning") {
		t.Fatalf("motivo deve ser o canônico, got %q", be.Error)
	}
	// Estado final deve permanecer v2 (sem clobber).
	if got, _ := s.Get(KindSkill, "net-catcher"); got.Content != "v2" {
		t.Fatalf("estado não deve ser clobberado: got %s", got.Content)
	}
}

func TestApply_IntraProposalChainOnSameKeyAllowed(t *testing.T) {
	s := NewState()
	// create + update na MESMA chave dentro de UM plano é permitido
	// (proposalModifiedKeys exime o segundo edit do baseline-conflict).
	plan := PlanRefinement(s, "chain", proposal(
		Edit{Action: ActionCreate, Kind: KindMemory, ID: "note", Title: "note", Content: "a"},
		Edit{Action: ActionUpdate, Kind: KindMemory, ID: "note", Title: "note", Content: "b"},
	), "trace-1", "")
	res, err := Apply(s, plan)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.AppliedEdits[0].Applied || !res.AppliedEdits[1].Applied {
		t.Fatalf("edits encadeados numa mesma chave deveriam aplicar: %+v", res.AppliedEdits)
	}
	if got, _ := s.Get(KindMemory, "note"); got.Content != "b" || got.Version != 2 {
		t.Fatalf("estado final da cadeia: content=%q version=%d", got.Content, got.Version)
	}
}

// ---------------------------------------------------------------------------
// (d) P3 — validateEdit rejeita TODA mutação na identidade/base (fuzz)
// ---------------------------------------------------------------------------

func TestValidateEdit_FuzzIdentityTouchesAllRejected(t *testing.T) {
	s := NewState()
	// Identidade/base imutável: prompt base (constitution) e identidade de skill.
	s.Set(KindPrompt, BasePromptID, Entry{ID: BasePromptID, Kind: KindPrompt, Title: "Base System Prompt", Content: "constitution"})
	s.MarkImmutable(KindPrompt, BasePromptID)
	s.Set(KindSkill, "net-catcher/_identity", Entry{ID: "net-catcher/_identity", Kind: KindSkill, Title: "Net Catcher", Content: "name/desc/level"})
	s.MarkImmutable(KindSkill, "net-catcher/_identity")

	actions := []Action{ActionCreate, ActionUpdate, ActionDelete}
	ids := []string{BasePromptID, "net-catcher/_identity"}
	rng := rand.New(rand.NewSource(42)) // determinístico (I1)
	mutations := 0

	for _, id := range ids {
		for _, act := range actions {
			for i := 0; i < 20; i++ {
				kind := KindPrompt
				if id == "net-catcher/_identity" {
					kind = KindSkill
				}
				// Conteúdo/título aleatório: NENHUMA variante pode passar.
				edit := Edit{Action: act, Kind: kind, ID: id, Content: randContent(rng, 24)}
				err := s.ValidateEdit(edit)
				if err == nil {
					t.Fatalf("mutação de identidade %s:%s (%s) deveria ser rejeitada", kind, id, act)
				}
				var imm *ImmutableBaseEditError
				if !errors.As(err, &imm) {
					t.Fatalf("erro deve ser tipado ImmutableBaseEditError, got %T: %v", err, err)
				}
				if imm.ID != id {
					t.Fatalf("erro deve carregar o id imutável %s, got %s", id, imm.ID)
				}
				mutations++
			}
		}
	}
	if mutations < 100 {
		t.Fatalf("fuzz deveria ter exercitado muitas mutações, got %d", mutations)
	}
}

func TestValidateEdit_AllowsMutableEntry(t *testing.T) {
	s := NewState()
	seedSkill(s, "net-catcher", "v1")
	// Entrada NÃO imutável deve ser editável (contraste).
	if err := s.ValidateEdit(Edit{Action: ActionUpdate, Kind: KindSkill, ID: "net-catcher", Content: "v2"}); err != nil {
		t.Fatalf("entrada mutável deveria passar, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// (e) applied:false + motivo retornados
// ---------------------------------------------------------------------------

func TestApply_ReturnsAppliedFalseWithReason(t *testing.T) {
	s := NewState()
	seedSkill(s, "net-catcher", "v1")
	s.Set(KindPrompt, BasePromptID, Entry{ID: BasePromptID, Kind: KindPrompt, Content: "constitution"})
	s.MarkImmutable(KindPrompt, BasePromptID)

	cases := []struct {
		name    string
		edits   []Edit
		idx     int
		applied bool
		contains string
	}{
		{"imutável (P3)", []Edit{{Action: ActionUpdate, Kind: KindPrompt, ID: BasePromptID, Content: "hack"}}, 0, false, "immutable base edit rejected"},
		{"update em chave inexistente", []Edit{{Action: ActionUpdate, Kind: KindSkill, ID: "nope", Content: "x"}}, 0, false, "entry not found"},
		{"create em chave já existente", []Edit{{Action: ActionCreate, Kind: KindSkill, ID: "net-catcher", Content: "x"}}, 0, false, "entry already exists"},
		{"válido aplica", []Edit{{Action: ActionUpdate, Kind: KindSkill, ID: "net-catcher", Content: "v2"}}, 0, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := NewState()
			seedSkill(st, "net-catcher", "v1")
			st.Set(KindPrompt, BasePromptID, Entry{ID: BasePromptID, Kind: KindPrompt, Content: "constitution"})
			st.MarkImmutable(KindPrompt, BasePromptID)

			res, err := Apply(st, PlanRefinement(st, "e1", proposal(tc.edits...), "trace-1", ""))
			if err != nil {
				t.Fatalf("apply não deve dar erro fatal (edit é rejeitado): %v", err)
			}
			ae := res.AppliedEdits[tc.idx]
			if ae.Applied != tc.applied {
				t.Fatalf("applied=%v, esperava %v (%+v)", ae.Applied, tc.applied, ae)
			}
			if !tc.applied {
				if ae.Error == "" {
					t.Fatalf("edit rejeitado deve carregar um motivo")
				}
				if !strings.Contains(ae.Error, tc.contains) {
					t.Fatalf("motivo deve conter %q, got %q", tc.contains, ae.Error)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// P1 storage — Journal append-only
// ---------------------------------------------------------------------------

func TestJournal_AppendOnly(t *testing.T) {
	dir := t.TempDir()
	j := NewJournal(filepath.Join(dir, "refine.jsonl"))

	recs := []OnlineRefinementRecord{
		{ID: "r1", Trigger: "t1", Evidence: "e1", TraceID: "tr1", Version: 1},
		{ID: "r2", Trigger: "t2", Evidence: "e2", TraceID: "tr2", Version: 2},
	}
	for _, r := range recs {
		if err := j.Append(r); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	got, err := j.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 2 || got[1].ID != "r2" {
		t.Fatalf("journal deve ser append-only em ordem: %+v", got)
	}

	// Novos appends acrescentam, nunca sobrescrevem.
	if err := j.Append(OnlineRefinementRecord{ID: "r3", Evidence: "e3", TraceID: "tr3", Version: 3}); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, _ = j.Load()
	if len(got) != 3 {
		t.Fatalf("após novo append deveria ter 3 registros, got %d", len(got))
	}
}

func TestJournal_VolatileWhenEmptyPath(t *testing.T) {
	j := NewJournal("")
	if err := j.Append(OnlineRefinementRecord{ID: "r1"}); err != nil {
		t.Fatalf("journal volátil não deve dar erro: %v", err)
	}
	if got, _ := j.Load(); got != nil {
		t.Fatalf("journal volátil deve carregar nil, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func randContent(rng *rand.Rand, n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789 \n\t"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}
