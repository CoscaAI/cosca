// Package contextmetrics — MEDIÇÃO do Context Compiler (ADR-035, F5).
//
// Conecta o Context Compiler ao ADR-031 (Token Efficiency): mede a ECONOMIA
// real de tokens que a compilação de contexto entrega, e expõe como dimensão
// de valor no cost.Record. A métrica central:
//
//	savings = raw_context_tokens - compiled_context_tokens
//
// Sem o compilador, o contexto bruto (retrieval inteiro) entraria no LLM; com
// ele, entra o estado operacional compilado (TASK/STATE/FACTS...). A economia
// é a diferença — e a camada usada (L0/L1/L2) revela o custo adaptativo.
package contextmetrics

import (
	"time"

	"github.com/CoscaAI/cosca/internal/cost"
)

// CompileStats é a fotografia de UMA compilação de contexto.
type CompileStats struct {
	// Level é a camada usada (L0/L1/L2 do ContextRouter). L0 = sem LLM.
	Level string `json:"level"`
	// RawContextTokens é o custo estimado do contexto BRUTO (retrieval
	// inteiro, sem compilação) — o contrafactual honesto.
	RawContextTokens int `json:"raw_context_tokens"`
	// CompiledTokens é o custo estimado do contexto COMPILADO (o que o LLM
	// realmente recebeu).
	CompiledTokens int `json:"compiled_tokens"`
	// Sections é o número de seções preenchidas no contexto compilado.
	Sections int `json:"sections"`
	// Deterministic indica se a decisão foi resolvida SEM LLM (L0 emit).
	Deterministic bool `json:"deterministic"`
	// At é quando a compilação aconteceu.
	At time.Time `json:"at"`
}

// Savings devolve os tokens economizados pela compilação (nunca negativo:
// compilar nunca AUMENTA o custo — se o compilado for maior, economiza 0).
func (s CompileStats) Savings() int {
	if s.RawContextTokens <= s.CompiledTokens {
		return 0
	}
	return s.RawContextTokens - s.CompiledTokens
}

// EfficiencyRatio devolve a fração de redução (1.0 = corte total, 0 = nenhum).
func (s CompileStats) EfficiencyRatio() float64 {
	if s.RawContextTokens <= 0 {
		return 0
	}
	return float64(s.Savings()) / float64(s.RawContextTokens)
}

// ApplyToRecord registra a economia como evidência no cost.Record:
//   - ContextTokens = tokens compilados (o que de fato entrou no LLM)
//   - CachedTokens  = tokens economizados (o contrafactual honesto)
//   - KnowledgeGain += 1 quando a decisão foi determinística (L0 sem LLM)
//
// É ADITIVO: apenas preenche campos que o Record já tem (ADR-031); quem não
// usa o Context Compiler continua com 0 (honesto — não inventa economia).
func (s CompileStats) ApplyToRecord(rec *cost.Record) {
	if rec == nil {
		return
	}
	rec.ContextTokens = s.CompiledTokens
	if s.Savings() > 0 {
		rec.CachedTokens = s.Savings()
	}
	if s.Deterministic {
		rec.KnowledgeGain = 1
	}
}

// Track é o acumulador de sessão: soma as estatísticas de várias compilações
// para o relatório final (F5 — dashboard de custo por decisão).
type Track struct {
	// Compiles é o número de compilações registradas.
	Compiles int `json:"compiles"`
	// Deterministic é o número de decisões resolvidas SEM LLM.
	Deterministic int `json:"deterministic"`
	// RawTokens é o total de tokens brutos (contrafactual).
	RawTokens int `json:"raw_tokens"`
	// CompiledTokens é o total de tokens compilados (real).
	CompiledTokens int `json:"compiled_tokens"`
	// Levels contabiliza compilações por camada (L0/L1/L2).
	Levels map[string]int `json:"levels"`
}

// NewTrack cria um acumulador vazio.
func NewTrack() *Track {
	return &Track{Levels: make(map[string]int)}
}

// Add incorpora uma compilação ao acumulador.
func (t *Track) Add(s CompileStats) {
	if t.Levels == nil {
		t.Levels = make(map[string]int)
	}
	t.Compiles++
	t.RawTokens += s.RawContextTokens
	t.CompiledTokens += s.CompiledTokens
	t.Levels[s.Level]++
	if s.Deterministic {
		t.Deterministic++
	}
}

// SavingsTotal devolve a economia acumulada da sessão.
func (t *Track) SavingsTotal() int {
	if t.RawTokens <= t.CompiledTokens {
		return 0
	}
	return t.RawTokens - t.CompiledTokens
}

// SavingsPercent devolve a economia percentual da sessão (0-100).
func (t *Track) SavingsPercent() float64 {
	if t.RawTokens <= 0 {
		return 0
	}
	return float64(t.SavingsTotal()) / float64(t.RawTokens) * 100
}
