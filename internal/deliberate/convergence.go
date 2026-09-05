package deliberate

// Convergence is computed deterministically from the council positions
// (ADR-011 A3): convergência = Σ(peso × concordância). The circuit breaker is
// explicit: MaxRounds, MaxIterations and a hash-based loop detector.

import (
	"fmt"
	"math"
	"strings"
)

// Deliberation circuit breaker constants (A3). They bound the deliberation to
// a finite number of rounds/iterations and a wall-clock timeout so a council
// cannot spin forever on a loop.
const (
	// MaxRounds bounds the number of deliberation rounds.
	MaxRounds = 3
	// MaxIterations bounds the number of per-round iterations.
	MaxIterations = 5
	// Timeout bounds the deliberation in seconds.
	Timeout = 300
)

const (
	// DefaultEmitThreshold is the convergence/emission gate: a decision
	// converges at >= 0.70 (A3/A4).
	DefaultEmitThreshold = 0.70
	// convergenceEpsilon guards the threshold comparison against float drift.
	convergenceEpsilon = 1e-9
)

// weightFor returns the share of the total weight belonging to a dimension.
// Unknown dimensions carry zero weight (they never add convergence).
func weightFor(d Dimension, w ConvergenceWeights) float64 {
	switch d {
	case DimensionRecommendation:
		return w.Recommendation
	case DimensionPremises:
		return w.Premises
	case DimensionRisks:
		return w.Risks
	case DimensionTiming:
		return w.Timing
	default:
		return 0
	}
}

// stance normalizes a position's stance (claim) for comparison. Case and
// surrounding whitespace are normalized so two positions that assert the same
// thing in different casing still agree.
func stance(p Position) string {
	return strings.ToLower(strings.TrimSpace(p.Claim))
}

// ComputeConvergence computes Σ(peso × concordância) across the four council
// dimensions and reports whether the result is >= DefaultEmitThreshold (0.70).
//
// Semantics (deterministic, zero-LLM):
//   - Positions are grouped by Dimension.
//   - Only Effective positions count (substantiated AND carrying evidence) —
//     "zero achismo": unsubstantiated positions are deweighted (they contribute
//     nothing to agreement).
//   - Concordância per dimension = (effective agreeing with the majority
//     stance) / (effective positions in that dimension), using a deterministic
//     tie-break (most substantiated agreement, then lexicographic stance).
//   - A dimension with no effective positions yields concordância 0 and adds
//     0 to the total (it does NOT renormalize — an uncovered dimension is a
//     gap, not a pass).
//   - The bool result is `score >= 0.70` within a tiny epsilon.
func ComputeConvergence(positions []Position, w ConvergenceWeights) (float64, bool) {
	groups := map[Dimension][]Position{}
	for _, p := range positions {
		if !p.Effective() {
			// "zero achismo": ignore unsubstantiated positions (deweighted).
			continue
		}
		groups[p.Dimension] = append(groups[p.Dimension], p)
	}

	var score float64
	for _, d := range []Dimension{
		DimensionRecommendation,
		DimensionPremises,
		DimensionRisks,
		DimensionTiming,
	} {
		weight := weightFor(d, w)
		if weight == 0 {
			continue
		}
		agree, total := dimensionAgreement(groups[d])
		if total == 0 {
			// Uncovered dimension contributes nothing.
			continue
		}
		score += weight * (float64(agree) / float64(total))
	}

	return score, score >= DefaultEmitThreshold-convergenceEpsilon
}

// dimensionAgreement returns (agree, total) for the majority stance within a
// single dimension. agree is the count of effective positions here that agree
// with the majority stance.
func dimensionAgreement(positions []Position) (int, int) {
	if len(positions) == 0 {
		return 0, 0
	}

	// Count substantiated support per stance, then resolve a tie
	// deterministically (highest substantiated agreement, then lexicographic).
	type acc struct {
		count        int
		substantiated int
	}
	counts := map[string]*acc{}
	order := []string{}
	for _, p := range positions {
		s := stance(p)
		if counts[s] == nil {
			counts[s] = &acc{}
			order = append(order, s)
		}
		counts[s].count++
		if p.Substantiated {
			counts[s].substantiated++
		}
	}

	majority := ""
	var best acc
	for _, s := range order {
		a := counts[s]
		if majority == "" ||
			a.substantiated > best.substantiated ||
			(a.substantiated == best.substantiated && a.count > best.count) ||
			(a.substantiated == best.substantiated && a.count == best.count && s < majority) {
			majority = s
			best = *a
		}
	}

	agree := counts[majority].count
	return agree, len(positions)
}

// DetectLoop reports whether any round hash repeats at least twice (A3 circuit
// breaker). A repeated hash means the council is cycling and must stop.
func DetectLoop(hashes []string) bool {
	seen := make(map[string]int, len(hashes))
	for _, h := range hashes {
		seen[h]++
		if seen[h] >= 2 {
			return true
		}
	}
	return false
}

// RoundHash builds a deterministic hash string from a set of positions so the
// caller can feed it to DetectLoop across rounds. It is used to detect
// convergent-loops (A3): if two rounds produce the same position set, the
// council is not progressing.
func RoundHash(positions []Position) string {
	if len(positions) == 0 {
		return "deliberate:empty-round"
	}
	var b strings.Builder
	for _, p := range positions {
		fmt.Fprintf(&b, "%s|%s|%t|%d:", p.Dimension, stance(p), p.Effective(), len(p.EvidenceIDs))
	}
	return b.String()
}

// clamp01 pins a value to the closed [0,1] interval.
func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}
