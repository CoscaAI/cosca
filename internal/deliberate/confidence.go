package deliberate

// Confidence arithmetic (ADR-011 A4): the confidence is arithmetically built
// as Base + typed Adjustments (clamped to [0,1]), never "intuitive". The
// system — not the model — decides whether to EmitOK / EmitWithReservations /
// Escalate via the Emit gates.

import (
	"fmt"
	"strings"
)

// Adjustment reasons — typed, deterministic tags used to build Adjustments
// (A4). They make the breakdown auditable and the arithmetic model-independent.
const (
	adjustCriticBelowThreshold = "critic_below_threshold"
	adjustRiskCatastrophic     = "risk_catastrophic"
	adjustRiskHigh             = "risk_high"
	adjustRiskMedium           = "risk_medium"
	adjustUnresolvedContradict = "unresolved_contradiction"
	adjustCorroboratedMultisrc = "corroborated_multi_source"
)

// CriticAdjustment: when the meta-critic score is below 0.70 the council is
// docked -0.20 (A4). Otherwise it returns a no-op (Delta 0) so callers can
// always include it.
func CriticAdjustment(criticScore float64) Adjustment {
	if criticScore < DefaultEmitThreshold {
		return Adjustment{
			Reason: fmt.Sprintf("%s (critic score %.2f < %.2f)", adjustCriticBelowThreshold, criticScore, DefaultEmitThreshold),
			Delta:  -0.20,
		}
	}
	return Adjustment{
		Reason: "critic_score_ok",
		Delta:  0,
	}
}

// RiskAdjustment maps a (case/accents-insensitive) risk level to its typed
// penalty (A4): catastrophic -0.15, high/alta -0.10, medium/media -0.05.
// Unknown levels yield a no-op (-0.00) so a caller is not silently penalised
// for an unrecognised label. Higher risk always docks more than lower risk.
func RiskAdjustment(level string) Adjustment {
	switch normalizeLevel(level) {
	case "catastrophic", "critico", "critic", "critical":
		return Adjustment{Reason: adjustRiskCatastrophic, Delta: -0.15}
	case "high", "alta", "alto", "grave":
		return Adjustment{Reason: adjustRiskHigh, Delta: -0.10}
	case "medium", "media", "medio", "moderada", "moderado":
		return Adjustment{Reason: adjustRiskMedium, Delta: -0.05}
	default:
		return Adjustment{Reason: "risk_unknown", Delta: 0}
	}
}

// UnresolvedContradictionAdjustment docks -0.10 when a contradiction between
// positions remains unresolved (A4).
func UnresolvedContradictionAdjustment() Adjustment {
	return Adjustment{Reason: adjustUnresolvedContradict, Delta: -0.10}
}

// CorroborationAdjustment credits +0.10 when an evidence claim is corroborated
// by three or more independent sources (A4); otherwise it is 0.
func CorroborationAdjustment(sourceCount int) Adjustment {
	if sourceCount >= 3 {
		return Adjustment{Reason: adjustCorroboratedMultisrc, Delta: 0.10}
	}
	return Adjustment{Reason: "corroboration_insufficient", Delta: 0}
}

// ComputeConfidence applies a base score plus a list of typed adjustments,
// clamps the sum to [0,1] and produces an audit-ready breakdown.
//
//	Final = clamp01(Base + Σ(Adjustments.Delta))
//
// The Breakdown string is deterministic and includes the base, each applied
// adjustment (reason + delta) and the final result.
func ComputeConfidence(base float64, adjustments []Adjustment) ConfidenceBreakdown {
	base = clamp01(base)
	final := base
	var b strings.Builder
	fmt.Fprintf(&b, "base=%.4f", base)

	for _, a := range adjustments {
		if a.Delta == 0 {
			continue
		}
		final += a.Delta
		fmt.Fprintf(&b, " %s=%.4f", a.Reason, a.Delta)
	}

	final = clamp01(final)
	fmt.Fprintf(&b, " final=%.4f", final)

	return ConfidenceBreakdown{
		Base:        base,
		Adjustments: adjustments,
		Final:       final,
		Breakdown:   b.String(),
	}
}

// EvaluateEmit applies the emission thresholds (A4) to a confidence
// breakdown:
//
//	>= 0.70               → EmitOK
//	0.50 <= c < 0.70      → EmitWithReservations
//	< 0.50                → Escalate (does NOT re-run, anti-loop)
//
// The type is named Emit (see types.go); the function is EvaluateEmit because
// Go forbids a type and a function sharing one identifier.
func EvaluateEmit(b ConfidenceBreakdown) Emit {
	switch {
	case b.Final >= DefaultEmitThreshold-convergenceEpsilon:
		return EmitOK
	case b.Final >= 0.50-convergenceEpsilon:
		return EmitWithReservations
	default:
		return Escalate
	}
}

// normalizeLevel normalises a risk label for deterministic matching.
func normalizeLevel(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
