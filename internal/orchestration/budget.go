package orchestration

import (
	"fmt"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Execution Cost Budget (teto de custo por execução)
// ═══════════════════════════════════════════════════════════════════════════════
//
// Regra do Don: o caminho `cosca run` NÃO pode "gastar horrores" ao habilitar
// um provider de código (ex.: deepseek). Este pacote fecha a rede de custo:
// quando o consumo acumulado de uma execução estoura o teto (tokens, tempo ou
// custo), o laço de tool-calls PARÁ de chamar o provider — como o
// engine.BudgetTracker do motor antigo faz via CanCall().
//
// DECISÃO DE DESIGN (honesta): NÃO importamos internal/engine aqui. Apesar de
// não haver ciclo de importação, importar o motor legado arrastaria deps
// pesadas (chat/executor, modlink, plugins, providers, safeerror, contenttrust)
// apenas para reusar ~80 linhas de lógica de contagem de consumo — e faria o
// motor moderno (orchestration) acoplar-se ao motor antigo (engine), que é uma
// direção arquitetural errada (irmãos, não pai/filho). Preferimos a duplicação
// CONTROLADA de um conceito pequeno e estável. A semântica (CanCall pessimista,
// Exceeded, Record, média-por-chamada) é reproduzida fielmente para que o
// default seja idêntico ao aprovado pelo Don.
//
// ORIGEM DOS VALORES: o mesmo resultado de engine.DefaultCognitiveBudget
// (Tokens: 8k | Tempo: 20s | Custo: $0.05), referenciado no ADR-031 (Frente 1)
// e documentado em internal/embed/cosca/BUDGET_PROTOCOL.md.
//
// O budget é puramente opt-in: quando ExecutorConfig.Budget é nil, o Executor
// roda EXATAMENTE como antes (zero mudança de comportamento). Quando não-nil,
// cada chamada de IA é registrada e a PRÓXIMA só avança se ainda couber no teto.

// CognitiveBudget define o teto de consumo de UMA execução antes que o Executor
// esteja autorizado a chamar o modelo. Espelha a semântica de
// engine.CognitiveBudget para que o default seja idêntico ao do Don.
type CognitiveBudget struct {
	// MaxTokens é o teto de tokens acumulados. Default 8000.
	MaxTokens int `json:"max_tokens"`

	// MaxDuration é o teto de tempo acumulado do turno de LLM. Default 20s.
	MaxDuration time.Duration `json:"max_duration"`

	// MaxCost é o teto de custo em USD. Default 0.05.
	MaxCost float64 `json:"max_cost"`
}

// DefaultCognitiveBudget devolve o teto aprovado pelo Don para o caminho
// `cosca run`: Tokens: 8k | Tempo: 20s | Custo: $0.05 — idêntico ao
// engine.DefaultCognitiveBudget.
func DefaultCognitiveBudget() CognitiveBudget {
	return CognitiveBudget{
		MaxTokens:   8000,
		MaxDuration: 20 * time.Second,
		MaxCost:     0.05,
	}
}

// BudgetSpent é a fotografia do consumo acumulado de uma execução.
type BudgetSpent struct {
	// Tokens é o total de tokens acumulados (entrada + saída).
	Tokens int `json:"tokens"`

	// Duration é o tempo acumulado das chamadas de LLM.
	Duration time.Duration `json:"duration"`

	// Cost é o custo acumulado estimado em USD.
	Cost float64 `json:"cost"`

	// AICalls é o número de chamadas de IA que consumiram tokens (>= 1).
	AICalls int `json:"ai_calls"`
}

// BudgetTracker acumula tokens/tempo/custo e guarda as chamadas de IA: antes de
// cada chamada ao modelo, CanCall decide se uma próxima chamada estimada ainda
// caberia no budget. O custo é computado pelo chamador (Executor) usando um
// CostEstimator — nunca é inventado aqui.
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
// chamada, acrescenta o marcador "sem IA".
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

// exceededLocked NÃO fornece lock — é chamado por métodos que já o fazem.
func (b *BudgetTracker) exceededLocked() bool {
	return b.spent.Tokens > b.budget.MaxTokens ||
		b.spent.Duration > b.budget.MaxDuration ||
		b.spent.Cost > b.budget.MaxCost
}

// budgetString renderiza o teto configurado para logs/diagnóstico.
func budgetString(b CognitiveBudget) string {
	return fmt.Sprintf("tokens=%d tempo=%ds custo=$%.4f",
		b.MaxTokens, int(b.MaxDuration.Seconds()), b.MaxCost)
}
