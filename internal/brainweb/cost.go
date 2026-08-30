package brainweb

import (
	"strings"

	"github.com/CoscaAI/cosca/internal/cost"
)

// Projeção de Token Efficiency / Energy (ADR-031).
//
// Esta dimensão transforma a telemetria de custo (internal/cost, lida via
// cost.Store) em "energia" no cérebro visual. Regra do professor, NÃO-VIOLÁVEL:
// energia é INTENSIDADE OPERACIONAL, nunca nota moral. Não existe "agente bom"
// aqui — existe um agente que CONVERTE tokens em trabalho útil (🔥 produtivo)
// versus outro que GASTA muito e rende pouco (🌀 congestionado/circulando). Um
// agente pode gastar 150k tokens e ser produtivo; outro pode gastar 40k e estar
// estagnado. A energia reflete essa intensidade, nunca um julgamento.
//
// A distinção produtivo-vs-congestionado é PRESERVADA expondo AMBOS
// efficiency (caractere: conversão útil/token) e useful_work (magnitude: quanto
// trabalho útil real) — nunca um único escalar que embutiria um juízo de "bom".

// Estados de energia (intensidade operacional — não qualidade/nota).
const (
	// EnergyNeutral — sem telemetria de custo para o agente (honesto, não inventa).
	EnergyNeutral = "neutral"
	// EnergyUnmeasured — há consumo (tokens) mas o vetor de valor útil ainda não
	// foi decomposto (Fase 0 do ADR-031). Não é julgar se é bom ou ruim: é
	// dizer com honestidade que o trabalho útil ainda não foi medido.
	EnergyUnmeasured = "unmeasured"
	// EnergyProductive — 🔥 conversão alta de tokens em trabalho útil (produtivo/intenso).
	EnergyProductive = "productive"
	// EnergyCongested — 🌀 muitos tokens, pouco trabalho útil (circulando/estagnado).
	EnergyCongested = "congested"
	// EnergyIdle — atividade presente porém modesta (burn baixo).
	EnergyIdle = "idle"
)

// Limiares PROVISÓRIOS (Fase 0). Enquanto o motor não decompõe o vetor de valor
// (knowledge_gain/task_progress/...), estes valores apenas habilitam a mecânica;
// os números crus (efficiency, useful_work) são a fonte de verdade do visualizador.
const (
	// productivityFloor — conversão útil/token considerada produtiva (≥ 1 útil por token).
	productivityFloor = 1.0
	// congestedTokensFloor — burn mínimo para qualificar "congestionado" (evita
	// rotular uma execução minúscula de estagnada).
	congestedTokensFloor = 10_000
)

// NodeCost é a projeção de Token Efficiency / Energy de um agente (nó do grafo).
// Sempre presente no JSON (com has_data=false quando não há telemetria) para o
// visualizador ser honesto — a ausência de custo não é mascarada como zero útil.
type NodeCost struct {
	HasData    bool    `json:"has_data"`
	Runs       int     `json:"runs"`
	TokenUsage int     `json:"token_usage"`
	UsefulWork float64 `json:"useful_work"`
	Efficiency float64 `json:"efficiency"`
	Energy     string  `json:"energy"`
}

// costAccum é o agregador por agente (chave: nome lowercased/trimmed).
type costAccum struct {
	runs       int
	tokens     int
	usefulWork float64
}

// costAggregator agrega os Records do cost.Store por agente. Nil-safe: store
// nil ou erro de leitura → projeção vazia (nunca pânico).
type costAggregator struct {
	byAgent map[string]*costAccum
}

// newCostAggregator lê o cost.Store e agrega por agente. Store nil (ou ileso)
// devolve um agregador vazio — o grafo renderiza neutro em vez de pânico.
func newCostAggregator(store *cost.Store) *costAggregator {
	agg := &costAggregator{byAgent: map[string]*costAccum{}}
	if store == nil {
		return agg
	}
	records, err := store.Load()
	if err != nil {
		return agg // telemetria ilegível não derruba o cérebro
	}
	for _, r := range records {
		key := strings.ToLower(strings.TrimSpace(r.AgentID))
		if key == "" {
			continue
		}
		acc := agg.byAgent[key]
		if acc == nil {
			acc = &costAccum{}
			agg.byAgent[key] = acc
		}
		acc.runs++
		acc.tokens += r.TokensTotal
		acc.usefulWork += r.UsefulWork()
	}
	return agg
}

// projection devolve a projeção de custo de um agente (pelo nome). Sem dados →
// NodeCost neutro/zero (honesto, não inventa).
func (a *costAggregator) projection(name string) NodeCost {
	if a == nil {
		return NodeCost{Energy: EnergyNeutral}
	}
	acc, ok := a.byAgent[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return NodeCost{Energy: EnergyNeutral}
	}
	c := NodeCost{
		HasData:    true,
		Runs:       acc.runs,
		TokenUsage: acc.tokens,
		UsefulWork: acc.usefulWork,
	}
	c.Efficiency, c.Energy = classifyEnergy(c.TokenUsage, c.UsefulWork)
	return c
}

// classifyEnergy aplica a regra do professor. Devolve (efficiency, energy).
// efficiency = UsefulWork / TokensTotal (0 quando sem tokens). energy é a
// intensidade operacional — NUNCA uma nota do agente.
func classifyEnergy(tokens int, usefulWork float64) (eff float64, energy string) {
	if tokens <= 0 {
		return 0, EnergyNeutral
	}
	eff = usefulWork / float64(tokens)
	switch {
	case usefulWork <= 0:
		// Há consumo, mas o vetor de valor útil ainda não foi decomposto
		// (Fase 0). Honesto: não dá para julgar produtivo vs congestionado.
		return eff, EnergyUnmeasured
	case eff >= productivityFloor:
		// 🔥 Todo token bem convertido em trabalho útil (produtivo/intenso).
		return eff, EnergyProductive
	case tokens >= congestedTokensFloor:
		// 🌀 Muitos tokens, pouco trabalho útil — circulando/estagnado.
		return eff, EnergyCongested
	default:
		// Atividade presente porém modesta.
		return eff, EnergyIdle
	}
}
