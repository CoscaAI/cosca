// Package contextrouter — CONTEXT ROUTER progressivo (ADR-035, F4).
//
// Decide QUANTO contexto carregar para a decisão atual, em camadas
// progressivas (L0 → L1 → L2). O custo é adaptativo: o LLM só recebe o que a
// confiança exige — nada de despejar o cérebro inteiro.
//
//	L0 → estado mínimo (TASK + STATE + OUTPUT_CONTRACT) → confiança ≥ 0.70?
//	     ├── SIM → executa SEM LLM (o modo determinístico já existe)
//	     └── NÃO → L1
//	L1 → + CONSTRAINTS + FACTS → confiança ≥ 0.50?
//	     ├── SIM → LLM com L1
//	     └── NÃO → L2
//	L2 → + EVIDENCE + MEMORY → LLM com o contexto completo
//
// Reusa os thresholds do deliberate (ADR-032): EmitOK ≥ 0.70,
// EmitWithReservations 0.50-0.69, Escalate < 0.50. NÃO reinventa scoring —
// o search/ranking já pontuam; este pacote decide a CAMADA.
package contextrouter

import (
	"github.com/CoscaAI/cosca/internal/deliberate"
)

// Level é a camada de contexto a carregar.
type Level int

const (
	// Level0 é o estado mínimo (TASK + STATE + OUTPUT_CONTRACT) — para
	// decisões triviais que o modo determinístico resolve SEM LLM.
	Level0 Level = iota
	// Level1 adiciona CONSTRAINTS + FACTS — contexto relevante de alta
	// confiança.
	Level1
	// Level2 adiciona EVIDENCE + MEMORY — contexto completo para decisões
	// incertas.
	Level2
)

// String nomeia a camada para logs.
func (l Level) String() string {
	switch l {
	case Level0:
		return "L0"
	case Level1:
		return "L1"
	default:
		return "L2"
	}
}

// Confidence é a confiança da deliberação (reusa deliberate.ConfidenceBreakdown
// — a fonte canônica dos thresholds).
type Confidence struct {
	// Final é a confiança composta (0.0-1.0), do deliberate.
	Final float64
}

// DecideLevel escolhe a camada de contexto a carregar com base na confiança
// e na disponibilidade de conhecimento. Regras (mesmos thresholds do ADR-032):
//
//   - confiança ≥ 0.70 (EmitOK)  → L0: o Kernel pode decidir SEM LLM
//   - confiança ≥ 0.50 (reserv.) → L1: contexto relevante, LLM decide
//   - confiança < 0.50 (escalate)→ L2: contexto completo, LLM decide
func DecideLevel(conf Confidence) Level {
	switch deliberate.EvaluateEmit(deliberate.ConfidenceBreakdown{Final: conf.Final}) {
	case deliberate.EmitOK:
		return Level0
	case deliberate.EmitWithReservations:
		return Level1
	default:
		return Level2
	}
}

// DecideLevelWithInfo combina a confiança com a quantidade de conhecimento
// recuperado: mesmo com confiança alta, sem conhecimento suficiente o Kernel
// não decide sozinho (não inventa) — sobe para L1/L2. Confiança alta MAS
// knowledge vazio → L1 (precisa de contexto); confiança baixa → L2.
func DecideLevelWithInfo(conf Confidence, hasKnowledge, hasMemory bool) Level {
	switch deliberate.EvaluateEmit(deliberate.ConfidenceBreakdown{Final: conf.Final}) {
	case deliberate.EmitOK:
		// Confiança alta: o Kernel pode decidir sem LLM SÓ se tiver evidência.
		if hasKnowledge || hasMemory {
			return Level0
		}
		return Level1 // sem evidência, precisa de contexto para não inventar
	case deliberate.EmitWithReservations:
		return Level1
	default:
		return Level2
	}
}

// Emit decide se o Kernel pode responder SEM LLM nesta camada. L0 com
// confiança alta e evidência → emite (modo determinístico). Qualquer outra
// combinação → precisa do LLM.
func Emit(level Level, conf Confidence, hasKnowledge bool) bool {
	if level != Level0 {
		return false
	}
	if conf.Final < 0.70 {
		return false
	}
	return hasKnowledge
}
