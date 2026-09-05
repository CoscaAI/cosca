package evolution

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStageOrderAndValid(t *testing.T) {
	if !StageTask.Valid() || !StagePromote.Valid() {
		t.Fatal("task/promote devem ser validos")
	}
	if Stage("wat").Valid() {
		t.Fatal("estagio desconhecido nao deve ser valido")
	}
	if len(StageOrder) != 7 {
		t.Fatalf("o percurso tem 7 estagios, got %d", len(StageOrder))
	}
}

func TestNext(t *testing.T) {
	if StageDesign.Next() != StageImplement {
		t.Fatalf("design -> implement, got %s", StageDesign.Next())
	}
	if StagePromote.Next() != "" {
		t.Fatalf("promote e o fim, Next deve ser vazio")
	}
}

func TestCanStageAdvance_Deterministic(t *testing.T) {
	cases := []struct {
		cur, next Stage
		want      bool
	}{
		{StageTask, StageRequirement, true},    // avanca exatamente 1
		{StageTask, StageImplement, false},     // pula estagio -> nao
		{StageRequirement, StageTask, false},   // retrocede -> nao
		{StageTask, StageTask, false},          // mesmo -> nao
		{Stage("x"), StageTask, false},         // invalido -> nao
		{StageTask, Stage("y"), false},         // invalido -> nao
		{StageEvaluate, StagePromote, true},    // ultimo passo valido
	}
	for _, c := range cases {
		if got := CanStageAdvance(c.cur, c.next); got != c.want {
			t.Errorf("CanStageAdvance(%s,%s)=%v, want %v", c.cur, c.next, got, c.want)
		}
	}
}

func TestStageRoll_AdvanceAppendOnly(t *testing.T) {
	roll := NewStageRoll("proj-x")
	if roll.Current != StageTask {
		t.Fatalf("roll comeca em task, got %s", roll.Current)
	}

	ph := StagePhase{Stage: StageRequirement, ArtifactRef: "abc123", GateUsed: "proposal", Verdict: "APPROVE", At: time.Now().UTC()}
	next, err := roll.Advance(ph)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	// Append-only: o original nao muda.
	if roll.Current != StageTask || len(roll.Completed) != 0 {
		t.Fatal("Advance nao deve mutar o roll original")
	}
	if next.Current != StageRequirement || len(next.Completed) != 1 || next.Next != StageDesign {
		t.Fatalf("next roll incorreto: %+v", next)
	}

	// Avanca invalida deve dar erro (sem mutar).
	if _, err := next.Advance(StagePhase{Stage: StagePromote}); err == nil {
		t.Fatal("pular de requirement para promote deve falhar (I1)")
	}
}

func TestStageRollStore_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roll.jsonl")
	store := NewStageRollStore(path)

	roll := NewStageRoll("proj-y")
	// task -> requirement (save 1)
	roll, err := roll.Advance(StagePhase{Stage: StageRequirement, ArtifactRef: "h1", GateUsed: "proposal", Verdict: "APPROVE"})
	if err != nil {
		t.Fatalf("advance1: %v", err)
	}
	// requirement -> design (save 2)
	roll, err = roll.Advance(StagePhase{Stage: StageDesign, ArtifactRef: "h2", GateUsed: "deliberate", Verdict: "EMIT_DESIGN", At: time.Now().UTC()})
	if err != nil {
		t.Fatalf("advance2: %v", err)
	}
	if err := store.Save(roll); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.Save(roll); err != nil {
		t.Fatalf("save2 (append): %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil {
		t.Fatal("load retornou nil")
	}
	if got.Current != StageDesign || len(got.Completed) != 2 {
		t.Fatalf("load deve devolver o ultimo snapshot: %+v", got)
	}
}

func TestStageRollStore_Volatile(t *testing.T) {
	store := NewStageRollStore("")
	if err := store.Save(NewStageRoll("p")); err != nil {
		t.Fatalf("save volatile: %v", err)
	}
	got, err := store.Load()
	if err != nil || got != nil {
		t.Fatalf("volatile deve retornar nil: %v, %+v", err, got)
	}
}
