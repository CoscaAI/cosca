package engine

import (
	"fmt"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Cognitive Budget
// ═══════════════════════════════════════════════════════════════════════════════
//
// Regra do Don: antes de chamar IA o sistema tenta primeiro — local knowledge →
// external deterministic search → cached evidence → existing solution — e só
// então chama o modelo, e apenas enquanto o budget cognitivo permitir. Se
// resolver sem IA: AI calls: 0, Cost: $0.
//
// O BudgetTracker é puramente opt-in: quando EngineConfig.Budget é nil o loop
// do agente roda exatamente como antes (zero mudança de comportamento).

// CognitiveBudget define os limites de consumo para uma execução antes que o
// engine esteja autorizado a chamar o modelo.
type CognitiveBudget struct {
	// MaxTokens é o teto de tokens acumulados. Default 8000.
	MaxTokens int `json:"max_tokens"`

	// MaxDuration é o teto de tempo acumulado do turno de LLM. Default 20s.
	MaxDuration time.Duration `json:"max_duration"`

	// MaxCost é o teto de custo em USD. Default 0.05.
	MaxCost float64 `json:"max_cost"`
}

// DefaultCognitiveBudget devolve o budget cognitivo padrão aprovado pelo Don:
// Tokens: 8k | Tempo: 20s | Custo: $0.05.
func DefaultCognitiveBudget() CognitiveBudget {
	return CognitiveBudget{
		MaxTokens:   8000,
		MaxDuration: 20 * time.Second,
		MaxCost:     0.05,
	}
}

// UsefulWork é o vetor de valor decomposto de uma execução — "useful work"
// NÃO é uma métrica única (ADR-031): é decomposto em dimensões para não
// ensinar o agente a lição errada de "só vale aprender" (revisão do professor).
// Campos opcionais (zero-default) para manter retrocompatibilidade: uma
// execução que só resolve (task_progress=1, artifact=1) sem aprender nada
// também é trabalho útil.
type UsefulWork struct {
	// KnowledgeGain — aprendizado (delta de conhecimento antes/depois), 0..1.
	KnowledgeGain float64 `json:"knowledge_gain,omitempty"`

	// TaskProgress — sucesso na resolução, 0..1.
	TaskProgress float64 `json:"task_progress,omitempty"`

	// ArtifactValue — artefato produzido (código/docs/evidência), contagem.
	ArtifactValue int `json:"artifact_value,omitempty"`

	// EvidenceGain — evidência validada (P0-P5), contagem.
	EvidenceGain int `json:"evidence_gain,omitempty"`

	// DecisionGain — decisão tomada (D-XXXX), contagem.
	DecisionGain int `json:"decision_gain,omitempty"`
}

// Total devolve a soma das dimensões do vetor de valor (o numerador da métrica
// Useful Work / Tokens). Para a Fase 0 a soma direta é o valor honesto; a
// normalização/pesagem por dimensão é refinamento da Fase 0.1.
func (w UsefulWork) Total() float64 {
	return w.KnowledgeGain + w.TaskProgress +
		float64(w.ArtifactValue) + float64(w.EvidenceGain) + float64(w.DecisionGain)
}

// BudgetSpent é a fotografia do consumo acumulado de uma execução.
type BudgetSpent struct {
	Tokens   int           `json:"tokens"`
	Duration time.Duration `json:"duration"`
	Cost     float64       `json:"cost"`
	AICalls  int           `json:"ai_calls"`

	// Useful Work — DECOMPOSTO em dimensões (ADR-031, Frente 1). Aditivo e
	// opcional (omitempty/zero-default): não quebra quem já marshala BudgetSpent
	// nem altera o comportamento do gate (CanCall/Exceeded) — é só o vetor de
	// valoração da execução.
	KnowledgeGain float64 `json:"knowledge_gain,omitempty"`
	TaskProgress  float64 `json:"task_progress,omitempty"`
	ArtifactValue int     `json:"artifact_value,omitempty"`
	EvidenceGain  int     `json:"evidence_gain,omitempty"`
	DecisionGain  int     `json:"decision_gain,omitempty"`
}

// Work devolve o vetor de valor (UsefulWork) desta execução.
func (b BudgetSpent) Work() UsefulWork {
	return UsefulWork{
		KnowledgeGain: b.KnowledgeGain,
		TaskProgress:  b.TaskProgress,
		ArtifactValue: b.ArtifactValue,
		EvidenceGain:  b.EvidenceGain,
		DecisionGain:  b.DecisionGain,
	}
}

// UsefulWork devolve a soma das dimensões do vetor de valor.
func (b BudgetSpent) UsefulWork() float64 {
	return b.Work().Total()
}

// Efficiency devolve a métrica Useful Work / Tokens (tokens_total consumidos).
// Retorna 0 quando nenhum token foi consumido (evita divisão por zero).
func (b BudgetSpent) Efficiency() float64 {
	if b.Tokens <= 0 {
		return 0
	}
	return b.UsefulWork() / float64(b.Tokens)
}

// BudgetTracker acumula tokens/tempo/custo e guarda as chamadas de IA: antes de
// cada chamada ao modelo, CanCall decide se uma próxima chamada estimada ainda
// caberia no budget. Custo é 0 por padrão — um pricing hook (não implementado)
// o preencheria a partir da tabela de preços do provedor.
type BudgetTracker struct {
	mu     sync.Mutex
	budget CognitiveBudget
	spent  BudgetSpent
}

// NewBudgetTracker cria um BudgetTracker vinculado ao budget dado.
func NewBudgetTracker(budget CognitiveBudget) *BudgetTracker {
	return &BudgetTracker{budget: budget}
}

// Record acumula o consumo de uma chamada (tokens, duração e custo). AICalls é
// incrementado apenas quando tokens > 0 — i.e. uma chamada de IA que de fato
// consumiu tokens. Valores negativos são ignorados.
func (b *BudgetTracker) Record(tokens int, duration time.Duration, cost float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if tokens < 0 {
		tokens = 0
	}
	if duration < 0 {
		duration = 0
	}
	if cost < 0 {
		cost = 0
	}

	b.spent.Tokens += tokens
	b.spent.Duration += duration
	b.spent.Cost += cost
	if tokens > 0 {
		b.spent.AICalls++
	}
}

// Spent devolve uma fotografia do consumo acumulado.
func (b *BudgetTracker) Spent() BudgetSpent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.spent
}

// RecordWork valora a execução com o vetor de "useful work" decomposto
// (ADR-031). É o valor FINAL da execução — substitui o vetor (não acumula),
// porque as dimensões (task_progress, knowledge_gain) são escalares 0..1 e as
// contagens (artifact, evidence, decision) já vêm agregadas pelo chamador.
// Zero-default: nunca altera o gate (CanCall/Exceeded). Campos não gravados
// permanecem 0.
func (b *BudgetTracker) RecordWork(work UsefulWork) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.spent.KnowledgeGain = work.KnowledgeGain
	b.spent.TaskProgress = work.TaskProgress
	b.spent.ArtifactValue = work.ArtifactValue
	b.spent.EvidenceGain = work.EvidenceGain
	b.spent.DecisionGain = work.DecisionGain
}

// Exceeded informa se alguma dimensão do budget foi estourada (tokens > Max,
// tempo > Max ou custo > Max).
func (b *BudgetTracker) Exceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.exceededLocked()
}

// CanCall decide se uma próxima chamada de IA estimada ainda caberia no budget.
// Com nada gasto, a estimativa pessimista é uma chamada consumindo o budget
// inteiro (garante que a primeira chamada sempre entra). Com chamadas
// anteriores, a estimativa é a média de consumo por chamada até agora.
func (b *BudgetTracker) CanCall() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	var (
		estTokens int
		estDur    time.Duration
		estCost   float64
	)

	if b.spent.AICalls > 0 {
		estTokens = b.spent.Tokens / b.spent.AICalls
		estDur = b.spent.Duration / time.Duration(b.spent.AICalls)
		estCost = b.spent.Cost / float64(b.spent.AICalls)
	} else {
		estTokens = b.budget.MaxTokens
		estDur = b.budget.MaxDuration
		estCost = b.budget.MaxCost
	}

	return b.spent.Tokens+estTokens <= b.budget.MaxTokens &&
		b.spent.Duration+estDur <= b.budget.MaxDuration &&
		b.spent.Cost+estCost <= b.budget.MaxCost
}

// Summary renderiza o consumo da execução em pt-BR no formato:
// "AI calls: N | Tokens: X | Tempo: Ys | Custo: $Z". Quando nenhuma IA foi
// chamada, acrescenta o marcador "✓ resolvido sem IA (AI calls: 0, Cost: $0)".
func (b *BudgetTracker) Summary() string {
	s := b.Spent()

	cost := fmt.Sprintf("$%.4f", s.Cost)
	if s.Cost == 0 {
		cost = "$0"
	}

	summary := fmt.Sprintf("AI calls: %d | Tokens: %d | Tempo: %ds | Custo: %s",
		s.AICalls, s.Tokens, int(s.Duration.Seconds()), cost)
	if s.AICalls == 0 {
		summary += " ✓ resolvido sem IA (AI calls: 0, Cost: $0)"
	}
	return summary
}

func (b *BudgetTracker) exceededLocked() bool {
	return b.spent.Tokens > b.budget.MaxTokens ||
		b.spent.Duration > b.budget.MaxDuration ||
		b.spent.Cost > b.budget.MaxCost
}
