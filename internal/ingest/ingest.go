// ingest.go — Pipeline de INGEST em steps nomeados com early-halt (Plausible).
//
// Padrão minerado do Plausible (lib/plausible/ingestion/event.ex:137): todo
// evento (aqui: execução de agente) atravessa uma lista DECLARATIVA de steps,
// cada um pode `halt` e marcar `dropped` com `drop_reason`, medindo duração por
// step. É a espinha dorsal de OBSERVAR uso de agentes (análogo ao ADR-031 do
// COSCA): saber quanto cada step custou e por que algo foi descartado.
//
// READ-ONLY, sem estado global: o Pipeline é reutilizado; o recorrer de cada
// execução é um IngestContext local (camada fina de observabilidade).

package ingest

import (
	"context"
	"time"
)

// DropReason é o motivo do descarte early-halt de um step (análogo ao
// plausible `:bot`/`:dc_ip`/`:site_country_blocklist` — aqui: custo, falha,
// sem-permissão, rate-limit, budget-excedido).
type DropReason string

const (
	DropNone         DropReason = ""                 // não descartado
	DropCostoExcedido DropReason = "cost_exceeded"    // ADR-031: orçamento estourado
	DropFalha        DropReason = "failure"          // erro de execução
	DropSemPermissao DropReason = "permission_denied" // capability/política negada
	DropRateLimit    DropReason = "rate_limited"
	DropTimeout      DropReason = "timeout"
	DropCancelado    DropReason = "cancelled"
)

// StepOutcome é o resultado de UM step do pipeline.
type StepOutcome struct {
	Name       string      `json:"name"`
	DurationMs int64       `json:"duration_ms"`
	Dropped    bool        `json:"dropped"`
	DropReason DropReason  `json:"drop_reason,omitempty"`
}

// Tracked é a interface que um step pode implementar para reportar drop.
type Tracked interface {
	StepName() string
}

// StepFn é a assinatura de um step do pipeline. Retorna (drop, dropReason):
// drop=true → early-halt (pipeline para, marcar o step como dropped).
type StepFn func(ctx context.Context) (drop bool, reason DropReason)

// Step é um step nomeado do pipeline.
type Step struct {
	Name string
	Fn   StepFn
}

// Pipeline é a sequência declarativa de steps (reutilizável, sem estado).
type Pipeline struct {
	Steps []Step
}

// IngestResult acumula o resultado de uma passada do pipeline.
type IngestResult struct {
	Steps []StepOutcome `json:"steps"`
	// Dropped é true se algum step haltou o pipeline (early-halt).
	Dropped    bool       `json:"dropped"`
	DropReason DropReason `json:"drop_reason,omitempty"`
	// TotalMs é a soma das durações dos steps executados.
	TotalMs int64 `json:"total_ms"`
}

// Run executa os steps em ordem; o primeiro que retorna drop=true encerra o
// pipeline (early-halt), marcando o step como dropped com o motivo. Steps
// posteriores não são executados.
func (p *Pipeline) Run(ctx context.Context) *IngestResult {
	res := &IngestResult{}
	for _, s := range p.Steps {
		start := time.Now()
		drop, reason := s.Fn(ctx)
		dur := time.Since(start).Milliseconds()

		step := StepOutcome{Name: s.Name, DurationMs: dur}
		if drop {
			step.Dropped = true
			step.DropReason = reason
			res.Steps = append(res.Steps, step)
			res.Dropped = true
			res.DropReason = reason
			res.TotalMs += dur
			return res
		}
		res.Steps = append(res.Steps, step)
		res.TotalMs += dur
	}
	return res
}
