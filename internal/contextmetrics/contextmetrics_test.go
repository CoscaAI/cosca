package contextmetrics

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/cost"
)

// TestSavings_Computed: economia = raw - compiled, nunca negativa.
func TestSavings_Computed(t *testing.T) {
	s := CompileStats{RawContextTokens: 2000, CompiledTokens: 800, Level: "L1"}
	if got := s.Savings(); got != 1200 {
		t.Fatalf("savings = %d, esperava 1200", got)
	}
	if got := s.EfficiencyRatio(); got != 0.6 {
		t.Fatalf("efficiency = %.2f, esperava 0.60", got)
	}
}

// TestSavings_NeverNegative: se o compilado for maior que o raw (improvável,
// mas defensivo), economia = 0 — nunca custo negativo.
func TestSavings_NeverNegative(t *testing.T) {
	s := CompileStats{RawContextTokens: 500, CompiledTokens: 900, Level: "L2"}
	if got := s.Savings(); got != 0 {
		t.Fatalf("savings = %d, esperava 0 (nunca negativo)", got)
	}
}

// TestApplyToRecord_RegistersSavings: o Record recebe ContextTokens (real) e
// CachedTokens (economia) — os campos que o ADR-031 já define.
func TestApplyToRecord_RegistersSavings(t *testing.T) {
	rec := &cost.Record{TokensTotal: 800}
	s := CompileStats{RawContextTokens: 2000, CompiledTokens: 800, Level: "L1"}
	s.ApplyToRecord(rec)

	if rec.ContextTokens != 800 {
		t.Fatalf("ContextTokens = %d, esperava 800 (o que de fato entrou no LLM)", rec.ContextTokens)
	}
	if rec.CachedTokens != 1200 {
		t.Fatalf("CachedTokens = %d, esperava 1200 (economia honesta)", rec.CachedTokens)
	}
}

// TestApplyToRecord_Deterministic: decisão sem LLM (L0 emit) vira
// KnowledgeGain = 1 (valor real — o Kernel respondeu sem gastar tokens).
func TestApplyToRecord_Deterministic(t *testing.T) {
	rec := &cost.Record{}
	s := CompileStats{Level: "L0", Deterministic: true, RawContextTokens: 1000, CompiledTokens: 100}
	s.ApplyToRecord(rec)
	if rec.KnowledgeGain != 1 {
		t.Fatalf("KnowledgeGain = %v, esperava 1 (decisão determinística)", rec.KnowledgeGain)
	}
	if rec.CachedTokens != 900 {
		t.Fatalf("CachedTokens = %d, esperava 900", rec.CachedTokens)
	}
}

// TestApplyToRecord_NoSavingsHonest: sem economia (raw <= compiled), o Record
// não registra CachedTokens (honesto — não inventa).
func TestApplyToRecord_NoSavingsHonest(t *testing.T) {
	rec := &cost.Record{}
	s := CompileStats{RawContextTokens: 100, CompiledTokens: 200, Level: "L2"}
	s.ApplyToRecord(rec)
	if rec.CachedTokens != 0 {
		t.Fatalf("CachedTokens = %d, esperava 0 (sem economia não inventa)", rec.CachedTokens)
	}
}

// TestTrack_SessionAggregate: o acumulador soma compilações e calcula a
// economia percentual da sessão.
func TestTrack_SessionAggregate(t *testing.T) {
	tr := NewTrack()
	tr.Add(CompileStats{Level: "L0", Deterministic: true, RawContextTokens: 1000, CompiledTokens: 100})
	tr.Add(CompileStats{Level: "L1", RawContextTokens: 2000, CompiledTokens: 800})
	tr.Add(CompileStats{Level: "L2", RawContextTokens: 500, CompiledTokens: 450})

	if tr.Compiles != 3 {
		t.Fatalf("compiles = %d, esperava 3", tr.Compiles)
	}
	if tr.Deterministic != 1 {
		t.Fatalf("deterministic = %d, esperava 1", tr.Deterministic)
	}
	if tr.Levels["L0"] != 1 || tr.Levels["L1"] != 1 || tr.Levels["L2"] != 1 {
		t.Fatalf("levels = %+v, esperava 1 de cada", tr.Levels)
	}
	if got := tr.SavingsTotal(); got != 2150 {
		t.Fatalf("savings total = %d, esperava 2150 (3500 raw - 1350 compiled)", got)
	}
	if got := tr.SavingsPercent(); got < 60 || got > 62 {
		t.Fatalf("savings percent = %.2f%%, esperava ~61.4%%", got)
	}
}

// TestTrack_NewTrack_LevelsInit: o acumulador novo tem o mapa de níveis
// inicializado (sem panic em Add).
func TestTrack_NewTrack_LevelsInit(t *testing.T) {
	tr := NewTrack()
	tr.Add(CompileStats{Level: "L1", RawContextTokens: 100, CompiledTokens: 50})
	if tr.Levels["L1"] != 1 {
		t.Fatal("nível L1 não contabilizado")
	}
}
