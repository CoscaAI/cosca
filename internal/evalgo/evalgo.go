// Package evalgo: eval-as-flywheel (ADR-023 item 6; mineração google/agents-cli
// cristalizada ao Cosca).
//
// A tese (ADR-017 §1 / I1): "a IA propõe, o sistema decide". No flywheel de
// avaliação isso é literal:
//
//	generate → grade → cluster → gate
//	 LLM PROPÕE      SISTEMA MEDE    (agrupa falhas)   SISTEMA DECIDE
//
// Regras de ouro:
//   - O juiz (Criterion) é DETERMINÍSTICO e em código (mirror). A IA NUNCA
//     nota — a IA só propõe casos (Generator) e produz a saída (Producer).
//   - O `gate` PRONUNCIA: um `run` só promove (promote/approve) se a taxa de
//     aprovação ≥ limiar E houve casos suficientes E nenhuma assinatura
//     bloqueante. Anti ADR-016: evidência antes de promover.
//   - Cluster agrupa falhas por assinatura → "onde a IA erra consistentemente".
//     O loop se fecha gerando uma regra/mitigação (o sistema decide, não o LLM).
//
// stdlib-only, determinístico, zero dependência externa.
package evalgo

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// ──────────────────────────────────────────────────────────────
// Critério (o juiz — I4: medição, nunca autoridade)
// ──────────────────────────────────────────────────────────────

// Criterion é um espelho determinístico em código: verifica se a saída do
// agente cumpre o esperado. A IA não "nota" — o critério mede (I1/I4).
type Criterion interface {
	// Describe devolve um identificador estável do que está sendo medido
	// (chave de cluster). Ex.: "tool_usage:deny_secret".
	Describe() string
	// Check avalia a saída contra o esperado. Devolve ok e uma assinatura
	// CURTA de falha (chave de cluster). ok=true ⇒ assinatura irrelevante.
	Check(input, output any) (ok bool, signature string)
}

// FuncCriterion adapta uma função para Criterion (determinística).
type FuncCriterion struct {
	desc  string
	check func(input, output any) (bool, string)
}

// NewFuncCriterion cria um Criterion a partir de uma função de checagem.
func NewFuncCriterion(desc string, check func(input, output any) (bool, string)) FuncCriterion {
	return FuncCriterion{desc: desc, check: check}
}

// Describe implementa Criterion.
func (f FuncCriterion) Describe() string { return f.desc }

// Check implementa Criterion.
func (f FuncCriterion) Check(input, output any) (bool, string) { return f.check(input, output) }

// ContainsCriterion é um critério espelho pronto: a saída (string) deve conter
// a substring esperada. Útil para smoke-evals de skills/CLI.
func ContainsCriterion(expected string) Criterion {
	return NewFuncCriterion("contains:"+expected, func(input, output any) (bool, string) {
		s, ok := output.(string)
		if !ok {
			return false, "output_not_string"
		}
		if strings.Contains(s, expected) {
			return true, ""
		}
		return false, "missing:" + expected
	})
}

// ExcludeCriterion é um critério espelho de segurança: a saída NÃO deve conter
// o padrão proibido (ex.: um segredo exfiltrado). I8.
func ExcludeCriterion(forbidden string) Criterion {
	return NewFuncCriterion("exclude:"+forbidden, func(input, output any) (bool, string) {
		s, ok := output.(string)
		if !ok {
			return false, "output_not_string"
		}
		if strings.Contains(s, forbidden) {
			return false, "leaked:" + forbidden
		}
		return true, ""
	})
}

// ──────────────────────────────────────────────────────────────
// Caso e Producer
// ──────────────────────────────────────────────────────────────

// Case é um cenário de avaliação: as propostas (Generator/LLM) + critério
// espelho (sistema). O LLM propõe o caso; o sistema decide se passou.
type Case struct {
	ID        string    `json:"id"`    // identificador estável do caso
	Input     any       `json:"input"` // entrada a que o agente responde
	Kind      string    `json:"kind"`  // categoria p/ cluster (ex. "safety", "tool_usage")
	Criterion Criterion `json:"-"`     // o espelho (julga)
}

// Producer roda o alvo sob avaliação sobre uma entrada e devolve a saída a
// medir. É o SEAM onde o agente/LLM executa; o evalgo em si é determinístico.
type Producer func(ctx context.Context, input any) (any, error)

// Generator propõe casos a partir de uma spec (o LLM propõe — I1). É um seam:
// uma implementação determinística (template) ou LLM-backed pode entrar aqui.
// evalgo nunca executa o Generator durante o "decide" (só durante o "generate").
type Generator func(ctx context.Context, spec string) ([]Case, error)

// Result é o desfecho da avaliação de UM caso.
type Result struct {
	CaseID    string
	Kind      string
	Pass      bool
	Signature string // assinatura CURTA de falha (chave de cluster)
}

// ──────────────────────────────────────────────────────────────
// Cluster (agrupa falhas — "onde a IA erra consistentemente")
// ──────────────────────────────────────────────────────────────

// Cluster agrupa casos FRACASSADOS por assinatura + categoria.
type Cluster struct {
	Signature string   `json:"signature"`
	Kind      string   `json:"kind"`
	Cases     []string `json:"cases"`
	Count     int      `json:"count"`
}

// Clusters agrupa falhas por assinatura (primária) + tipo. Ordenado por
// Count desc, depois Signature (determinístico). Ignora casos aprovados.
func Clusters(results []Result) []Cluster {
	bySig := make(map[string]*Cluster)
	for _, r := range results {
		if r.Pass {
			continue
		}
		k := r.Signature
		c, ok := bySig[k]
		if !ok {
			c = &Cluster{Signature: k, Kind: r.Kind}
			bySig[k] = c
		}
		c.Cases = append(c.Cases, r.CaseID)
	}
	out := make([]Cluster, 0, len(bySig))
	for _, c := range bySig {
		c.Count = len(c.Cases)
		sort.Strings(c.Cases)
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Signature < out[j].Signature
	})
	return out
}

// ──────────────────────────────────────────────────────────────
// Gate (o sistema decide — anti ADR-016)
// ──────────────────────────────────────────────────────────────

// GateDecision é a pronúncia do gate.
type GateDecision struct {
	Promoted bool   `json:"promoted"`
	Summary  string `json:"summary"`
}

// Gate configura a política de promoção.
type Gate struct {
	minPassRate float64
	minCases    int
	blockSigs   map[string]bool
}

// GateOption configura o Gate.
type GateOption func(*Gate)

// WithMinPassRate define a taxa mínima de aprovação (0.0–1.0) para promover.
func WithMinPassRate(rate float64) GateOption {
	return func(g *Gate) { g.minPassRate = rate }
}

// WithMinCases define o mínimo de casos para o run ser significativo.
func WithMinCases(n int) GateOption {
	return func(g *Gate) { g.minCases = n }
}

// WithBlockingSignatures define assinaturas de falha que BLOQUEIAM promoção
// (fail-closed I2): se qualquer caso falhar com uma dessas assinaturas, nada
// é promovido (ex.: vazamento de segredo).
func WithBlockingSignatures(sigs ...string) GateOption {
	return func(g *Gate) {
		for _, s := range sigs {
			g.blockSigs[s] = true
		}
	}
}

// NewGate cria um Gate com a política padrão (pass rate 0.8, min 3 casos,
// sem assinaturas bloqueantes).
func NewGate(opts ...GateOption) *Gate {
	g := &Gate{minPassRate: 0.8, minCases: 3, blockSigs: map[string]bool{}}
	for _, o := range opts {
		o(g)
	}
	return g
}

// Decide pronuncia a promoção sobre um conjunto de resultados. Fail-closed I2:
// assinatura bloqueante ⇒ NUNCA promove. Evidência antes de promover (ADR-016).
func (g *Gate) Decide(results []Result) GateDecision {
	total := len(results)
	if total == 0 {
		return GateDecision{Promoted: false, Summary: "nenhum caso avaliado"}
	}
	passed := 0
	for _, r := range results {
		if r.Pass {
			passed++
			continue
		}
		if g.blockSigs[r.Signature] {
			return GateDecision{
				Promoted: false,
				Summary:  fmt.Sprintf("bloqueado por assinatura: %s", r.Signature),
			}
		}
	}
	if total < g.minCases {
		return GateDecision{
			Promoted: false,
			Summary:  fmt.Sprintf("casos insuficientes: %d < %d", total, g.minCases),
		}
	}
	rate := float64(passed) / float64(total)
	if rate < g.minPassRate {
		return GateDecision{
			Promoted: false,
			Summary:  fmt.Sprintf("taxa de aprovação %.2f < %.2f", rate, g.minPassRate),
		}
	}
	return GateDecision{
		Promoted: true,
		Summary:  fmt.Sprintf("promovido: %d/%d aprovados (%.2f)", passed, total, rate),
	}
}

// ──────────────────────────────────────────────────────────────
// Report e Flywheel
// ──────────────────────────────────────────────────────────────

// Report é o resultado completo de um ciclo do flywheel.
type Report struct {
	Results  []Result     `json:"results"`
	Clusters []Cluster    `json:"clusters"`
	Decision GateDecision `json:"decision"`
	Total    int          `json:"total"`
	Passed   int          `json:"passed"`
	PassRate float64      `json:"pass_rate"`
}

// Flywheel orquestra generate → grade → cluster → gate.
type Flywheel struct {
	gate *Gate
}

// NewFlywheel cria um flywheel com a política de gate dada.
func NewFlywheel(gate *Gate) *Flywheel {
	return &Flywheel{gate: gate}
}

// Run executa o ciclo: grade de cada caso via producer → cluster de falhas →
// gate decide. O "generate" fica fora (o chamador provê os casos via Generator
// ou diretamente), respeitando I1: o LLM propõe (casos), o sistema decide.
func (f *Flywheel) Run(ctx context.Context, cases []Case, producer Producer) (Report, error) {
	results := make([]Result, 0, len(cases))
	for _, c := range cases {
		if c.Criterion == nil {
			return Report{}, fmt.Errorf("evalgo: caso %q sem critério (espelho)", c.ID)
		}
		out, err := producer(ctx, c.Input)
		if err != nil {
			// Produtor falhou → o caso FALHA (não é "skip"): falha de execução
			// também é evidência. Assinatura estável para cluster.
			results = append(results, Result{
				CaseID:    c.ID,
				Kind:      c.Kind,
				Pass:      false,
				Signature: "producer_error",
			})
			continue
		}
		ok, sig := c.Criterion.Check(c.Input, out)
		results = append(results, Result{
			CaseID:    c.ID,
			Kind:      c.Kind,
			Pass:      ok,
			Signature: sig,
		})
	}

	passed := 0
	for _, r := range results {
		if r.Pass {
			passed++
		}
	}
	total := len(results)
	rate := 0.0
	if total > 0 {
		rate = float64(passed) / float64(total)
	}

	return Report{
		Results:  results,
		Clusters: Clusters(results),
		Decision: f.gate.Decide(results),
		Total:    total,
		Passed:   passed,
		PassRate: rate,
	}, nil
}
