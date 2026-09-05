package orchestration

import (
	"testing"
	"time"
)

// TestBudgetTracker_FirstCallAlwaysAllowed: com nada gasto, a primeira chamada
// sempre entra (a estimativa pessimista é a própria execução). É o que garante
// que uma tarefa começa mesmo com teto apertado.
func TestBudgetTracker_FirstCallAlwaysAllowed(t *testing.T) {
	b := NewBudgetTracker(DefaultCognitiveBudget())
	if !b.CanCall() {
		t.Fatal("esperava a primeira chamada ser permitida (nada gasto)")
	}
}

// TestBudgetTracker_DefaultBudget: o default aprovado pelo Don ($0.05 / 8000 /
// 20s) — deve bater com a rede de custo do caminho cosca run.
func TestBudgetTracker_DefaultBudget(t *testing.T) {
	b := DefaultCognitiveBudget()
	if b.MaxCost != 0.05 {
		t.Errorf("MaxCost = %v, esperava 0.05", b.MaxCost)
	}
	if b.MaxTokens != 8000 {
		t.Errorf("MaxTokens = %d, esperava 8000", b.MaxTokens)
	}
	if b.MaxDuration != 20*time.Second {
		t.Errorf("MaxDuration = %v, esperava 20s", b.MaxDuration)
	}
}

// TestBudgetTracker_Exceeded_BlocksNextCall: quando o custo acumulado estoura o
// teto, uma próxima chamada é bloqueada — a rede de custo PARAR de gastar.
func TestBudgetTracker_Exceeded_BlocksNextCall(t *testing.T) {
	b := NewBudgetTracker(DefaultCognitiveBudget())

	// Simula uma chamada que já estourou o teto de custo (ex.: muito acima de 0.05).
	// RFC: custo alto o bastante para passar de MaxCost sozinho.
	b.Record(60000, 30*time.Second, 0.50)

	if !b.Exceeded() {
		t.Fatal("esperava Exceeded() == true após estourar o custo")
	}
	if b.CanCall() {
		t.Fatal("esperava CanCall() == false (teto estourado — deve BLOQUEAR nova chamada)")
	}
}

// TestBudgetTracker_TokenCeiling_BlocksCall: o teto de tokens também bloqueia.
func TestBudgetTracker_TokenCeiling_BlocksCall(t *testing.T) {
	b := NewBudgetTracker(DefaultCognitiveBudget())

	// Estoura só o teto de tokens (8000), custo ainda baixo.
	b.Record(9000, 100*time.Millisecond, 0.01)

	if !b.Exceeded() {
		t.Fatal("esperava Exceeded() == true após estourar tokens")
	}
	if b.CanCall() {
		t.Fatal("esperava CanCall() == false (teto de tokens estourado)")
	}
}

// TestBudgetTracker_DurationCeiling_BlocksCall: o teto de tempo também bloqueia.
func TestBudgetTracker_DurationCeiling_BlocksCall(t *testing.T) {
	b := NewBudgetTracker(DefaultCognitiveBudget())

	// Estoura só o teto de duração (20s), custo baixo.
	b.Record(100, 30*time.Second, 0.001)

	if !b.Exceeded() {
		t.Fatal("esperava Exceeded() == true após estourar duração")
	}
	if b.CanCall() {
		t.Fatal("esperava CanCall() == false (teto de duração estourado)")
	}
}

// TestBudgetTracker_WithinBudget_AllowsCall: sem estourar, a próxima chamada é
// permitida — a rede de custo NÃO trava uma execução saudável.
func TestBudgetTracker_WithinBudget_AllowsCall(t *testing.T) {
	b := NewBudgetTracker(DefaultCognitiveBudget())

	b.Record(500, 2*time.Second, 0.002)

	if b.Exceeded() {
		t.Fatal("não deveria exceder com consumo baixo")
	}
	if !b.CanCall() {
		t.Fatal("esperava CanCall() == true (dentro do teto)")
	}
}

// TestBudgetTracker_NegativeValuesIgnored: valores negativos não devem inflar.
func TestBudgetTracker_NegativeValuesIgnored(t *testing.T) {
	b := NewBudgetTracker(DefaultCognitiveBudget())
	b.Record(-5, -time.Second, -0.5)

	if b.Spent().Tokens != 0 || b.Spent().Cost != 0 {
		t.Fatalf("valores negativos devem ser ignorados: got tokens=%d cost=%v", b.Spent().Tokens, b.Spent().Cost)
	}
}

// TestBudgetTracker_Summary: o resumo é legível e não quebra.
func TestBudgetTracker_Summary(t *testing.T) {
	b := NewBudgetTracker(DefaultCognitiveBudget())
	b.Record(300, 1*time.Second, 0.001)
	s := b.Summary()
	if s == "" {
		t.Fatal("esperava um resumo não-vazio")
	}
}
