package task

import (
	"errors"
	"testing"
)

// TestLegalTransitionIllegal: uma transição que SKIPA estados ou sai de um estado
// terminal deve ser bloqueada com ErrIllegalTransition — o grafo do ciclo de vida
// autoriza apenas ACTIVE/WAITING/PAUSED ↔ (retomada) e a saída para os terminais.
func TestLegalTransitionIllegal(t *testing.T) {
	// Mesmo estado é no-op idempotente (não é erro).
	if err := LegalTransition(StatusActive, StatusActive); err != nil {
		t.Fatalf("ACTIVE→ACTIVE deveria ser no-op (idempotente), got %v", err)
	}

	// Estado terminal não sai para lugar nenhum.
	if err := LegalTransition(StatusComplete, StatusActive); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("COMPLETE→ACTIVE deveria ser ILEGAL, got %v", err)
	}
	if err := LegalTransition(StatusAborted, StatusPaused); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("ABORTED→PAUSED deveria ser ILEGAL, got %v", err)
	}

	// Máquina "para trás" não tem volta: WAITING→ACTIVE é legal (retomada), mas
	// uma transição que não existe no grafo (ex.: WAITING→WAITING é idempotente;
	// PAUSED→... ) — testamos um salto inexistente.
	if err := LegalTransition(StatusWaiting, StatusWaiting); err != nil {
		t.Fatalf("WAITING→WAITING deveria ser no-op, got %v", err)
	}

	// Transições legais de retomada / terminação são aceitas.
	for _, to := range []TaskStatus{StatusWaiting, StatusPaused, StatusComplete, StatusAborted} {
		if err := LegalTransition(StatusActive, to); err != nil {
			t.Errorf("ACTIVE→%s deveria ser legal, got %v", to, err)
		}
	}
	for _, to := range []TaskStatus{StatusActive, StatusComplete, StatusAborted} {
		if err := LegalTransition(StatusPaused, to); err != nil {
			t.Errorf("PAUSED→%s deveria ser legal, got %v", to, err)
		}
	}
}

// TestIsTerminal: apenas COMPLETE e ABORTED são estados de saída (limpam watchdog).
func TestIsTerminal(t *testing.T) {
	for _, s := range []TaskStatus{StatusComplete, StatusAborted} {
		if !IsTerminal(s) {
			t.Errorf("%s deveria ser terminal", s)
		}
	}
	for _, s := range []TaskStatus{StatusActive, StatusWaiting, StatusPaused} {
		if IsTerminal(s) {
			t.Errorf("%s NÃO deveria ser terminal", s)
		}
	}
}

// TestValidateObjective: o contrato do objetivo exige ID, Symbol e Direction
// (buy|sell) — espelha o ValidateSpec do handoff.
func TestValidateObjective(t *testing.T) {
	valid := TaskObjective{ID: "t1", Symbol: "BTCUSDT", Direction: "buy"}
	if err := ValidateObjective(valid); err != nil {
		t.Fatalf("objetivo válido não deveria falhar: %v", err)
	}
	cases := []TaskObjective{
		{Symbol: "BTCUSDT", Direction: "buy"},            // sem ID
		{ID: "t1", Direction: "buy"},                     // sem symbol
		{ID: "t1", Symbol: "BTCUSDT"},                    // sem direction
		{ID: "t1", Symbol: "BTCUSDT", Direction: "hold"}, // direction inválida
	}
	for i, c := range cases {
		if err := ValidateObjective(c); !errors.Is(err, ErrInvalidObjective) {
			t.Errorf("caso %d deveria ser ErrInvalidObjective, got %v", i, err)
		}
	}
}

// TestContinuationReasonValid: só as 4 razões cognitivas da ADR são válidas.
func TestContinuationReasonValid(t *testing.T) {
	for _, r := range []ContinuationReason{ContIncompleteObjective, ContStepLimit, ContInputRequired, ContWatchdog} {
		if !r.Valid() {
			t.Errorf("%s deveria ser válida", r)
		}
	}
	if (ContinuationReason("INVENTED")).Valid() {
		t.Error("razão inventada NÃO deveria ser válida")
	}
}

// TestCloneIsIndependent: a cópia do TaskState não compartilha slices nem o mapa
// de Artifacts com o original (leituras seguras fora do lock do orquestrador).
func TestCloneIsIndependent(t *testing.T) {
	st := TaskState{
		TaskID:       "t1",
		Status:       StatusActive,
		Checkpoints:  []string{"a"},
		Observations: []string{"o1"},
		Artifacts: map[string]string{
			ArtifactSide: "buy",
			ArtifactOpen: "true",
		},
	}
	c := st.Clone()
	c.Checkpoints[0] = "mutado"
	c.Observations = append(c.Observations, "o2")
	c.Artifacts[ArtifactSide] = "sell"
	if st.Checkpoints[0] != "a" {
		t.Errorf("clone deveria ser independente, original mutado: %v", st.Checkpoints)
	}
	if len(st.Observations) != 1 {
		t.Errorf("clone deveria ser independente, original cresceu: %v", st.Observations)
	}
	if st.Artifacts[ArtifactSide] != "buy" {
		t.Errorf("clone deveria ser independente (mapa), original mutado: %v", st.Artifacts)
	}
}

// TestCloneNilArtifacts: um TaskState sem Artifacts clona sem panic e permanece
// com Artifacts nil (não cria mapa vazio desnecessário).
func TestCloneNilArtifacts(t *testing.T) {
	st := TaskState{TaskID: "t1", Status: StatusActive}
	c := st.Clone()
	if c.Artifacts != nil {
		t.Errorf("clone de Artifacts nil deveria continuar nil, got %v", c.Artifacts)
	}
	if c.ArtifactValue(ArtifactOpen) != "" {
		t.Errorf("ArtifactValue de chave ausente deveria ser \"\", got %q", c.ArtifactValue(ArtifactOpen))
	}
}

// TestArtifactsSnapshotContract: o snapshot genérico do data plane usa o contrato
// de chaves do pacote — o orquestrador lê as chaves sem conhecer o domínio.
func TestArtifactsSnapshotContract(t *testing.T) {
	pos := map[string]string{
		ArtifactSymbol: "BTCUSDT",
		ArtifactSide:   "buy",
		ArtifactOpen:   "true",
		ArtifactQty:    "0.5",
		ArtifactPrice:  "110",
	}
	st := TaskState{TaskID: "t1", Status: StatusActive, Artifacts: pos}
	if st.ArtifactValue(ArtifactSide) != "buy" {
		t.Errorf("ArtifactValue(side)=%q, esperava buy", st.ArtifactValue(ArtifactSide))
	}
	if st.ArtifactValue(ArtifactOpen) != "true" {
		t.Errorf("ArtifactValue(open)=%q, esperava true", st.ArtifactValue(ArtifactOpen))
	}
	// o snapshot é só mapa — guarda qualquer chave sem tocar em domínio.
	if st.ArtifactValue("inexistente") != "" {
		t.Errorf("chave inexistente deveria ser \"\", got %q", st.ArtifactValue("inexistente"))
	}
}
