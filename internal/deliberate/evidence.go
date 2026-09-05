package deliberate

// Evidence confidence mirrors the canonical CONFIDENCE_MODEL.md evidence
// ladder (L0–L5 base weights + M1–M7 modifiers, clamped to [0,1]). It is
// conceptually the *evidence × claim* trust score, distinct from the
// Agent Confidence Tracker (internal/confidence) which tracks per-agent
// reliability.

// Evidence is a single traceable piece of evidence for a Position's claim
// (A2). Its Level drives the base trust weight; the boolean modifier flags
// drive the M1–M7 adjustments, exactly as defined in CONFIDENCE_MODEL.md.
//
// The ZeroValue Level is 0 (LLM response) — the least trusted tier — so a
// caller that forgets to set a level is conservatively treated as "LLM
// opinion", never as a fact.
type Evidence struct {
	ID             string
	Level          int     // 0–5 (CONFIDENCE_MODEL ladder)
	BaseWeight     float64 // optional override; when 0, derived from Level
	Source         string
	Concrete       bool // M1: exact location (file/line/commit hash)
	CrossValidated bool // M2: confirmed by independent agents
	Recent         bool // M3: verified within 7 days
	Stale          bool // M4: not verified in > 90 days
	Contradicted   bool // M5: contradicted by a higher-level source
	SingleSource   bool // M6: single source, not corroborated
	Aspirational   bool // M7: explicitly marked as a future goal
}

// baseWeightFor returns the L0–L5 base trust weight (CONFIDENCE_MODEL.md).
func baseWeightFor(level int) float64 {
	switch level {
	case 5:
		return 1.00 // código executado
	case 4:
		return 0.90 // testes aprovados
	case 3:
		return 0.60 // documentação oficial
	case 2:
		return 0.50 // memória do agente
	case 1:
		return 0.30 // opinião de agente
	default:
		return 0.20 // resposta de LLM (L0) — least trusted
	}
}

// EvidenceConfidence maps a piece of evidence to a [0,1] confidence score
// following CONFIDENCE_MODEL.md: clamp01(base_weight + Σ(M1–M7 modifiers)).
//
// It accepts an `any` so callers can hand over either a concrete Evidence or
// (for convenience) a *Evidence pointer. Anything that is not evidence-shaped
// is treated conservatively as a level-0 LLM opinion (0.20): an unknown
// artifact never inflates trust.
func EvidenceConfidence(ev any) float64 {
	switch e := ev.(type) {
	case Evidence:
		return evidenceConfidence(&e)
	case *Evidence:
		if e == nil {
			return baseWeightFor(0)
		}
		return evidenceConfidence(e)
	default:
		return baseWeightFor(0)
	}
}

// evidenceConfidence computes the clamped score for a non-nil Evidence.
func evidenceConfidence(e *Evidence) float64 {
	base := e.BaseWeight
	if base == 0 {
		base = baseWeightFor(e.Level)
	}

	score := base
	if e.Concrete {
		score += 0.15 // M1
	}
	if e.CrossValidated {
		score += 0.10 // M2
	}
	if e.Recent {
		score += 0.05 // M3
	}
	if e.Stale {
		score -= 0.15 // M4
	}
	if e.Contradicted {
		score -= 0.40 // M5
	}
	if e.SingleSource {
		score -= 0.10 // M6
	}
	if e.Aspirational {
		score -= 0.30 // M7
	}

	return clamp01(score)
}
