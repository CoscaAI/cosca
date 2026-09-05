package skilleval

import (
	"fmt"
	"strings"
	"unicode"
)

// Guardrail is the safety net that sits between a GEPA-produced variant and a
// promotion. It does not judge fitness; it judges whether a variant is safe to
// surface at all. A variant that breaks a guardrail is never offered as a PR
// candidate, regardless of how well it scored.
//
// FATIA 7G (bounded) introduces the four guardrails that gate `skill evolve`:
//
//   - CheckSizeLimit       the skill must not bloat past a line cap and must
//     not grow beyond a percentage of the baseline.
//   - CheckSemanticPreservation the mutation must not "drift from purpose": it
//     must keep enough of the baseline content that it is recognizably the
//     same skill, not a rewrite.
//   - CheckStructural      the result must still satisfy the catalog identity
//     invariant (name/description/level) — a mutation must never break the
//     identity contract that Apply() re-marshals.
//   - CheckCaching         workflow note: a promoted/cached skill is only valid
//     within the session that produced it ("só nova sessão").
//
// All guardrails are deterministic and pure: no randomness, no time, no LLM.

// GuardrailReport is the outcome of a single guardrail check. OK is false when
// the guardrail found a violation worth blocking; Violations carries a
// human-readable (and test-assertable) list of why it failed.
type GuardrailReport struct {
	OK         bool
	Violations []string
}

// Guardrail defaults. These mirror the phase-2 bounds: a SKILL.md may be at
// most 500 lines, may grow at most 20% over the baseline, and a mutation must
// keep at least 0.8 semantic similarity to the baseline body.
const (
	// DefaultMaxSkillLines is the default maximum number of lines a SKILL.md
	// (via Apply) may occupy.
	DefaultMaxSkillLines = 500
	// DefaultMaxGrowthPct is the default maximum growth, in percent, of the
	// applied skill vs the baseline.
	DefaultMaxGrowthPct = 20.0
	// DefaultMinSimilarity is the default semantic-similarity threshold below
	// which a mutation is considered to have "drifted from purpose".
	DefaultMinSimilarity = 0.8
)

// CheckSizeLimit enforces the bloat guardrail on `genome` (the candidate):
//
//   - the applied SKILL.md (genome.Apply()) must have at most maxLines lines;
//     maxLines <= 0 falls back to DefaultMaxSkillLines.
//   - when `baseline` is non-nil, the candidate's applied line-count may not
//     grow by more than maxGrowthPct percent over the baseline's applied
//     line-count; maxGrowthPct <= 0 falls back to DefaultMaxGrowthPct.
//
// Both measurements are over the whole applied SKILL.md (frontmatter + body),
// which is the unit that actually ships, so identity lines are counted too.
func CheckSizeLimit(genome *Genome, maxLines int, maxGrowthPct float64, baseline *Genome) GuardrailReport {
	report := GuardrailReport{OK: true}
	if maxLines <= 0 {
		maxLines = DefaultMaxSkillLines
	}
	if maxGrowthPct <= 0 {
		maxGrowthPct = DefaultMaxGrowthPct
	}

	if genome == nil {
		report.OK = false
		report.Violations = append(report.Violations, "candidate genome is nil")
		return report
	}

	appliedLines := countLines(genome.Apply())
	if appliedLines > maxLines {
		report.OK = false
		report.Violations = append(report.Violations,
			fmt.Sprintf("skill has %d lines, exceeds the %d-line cap", appliedLines, maxLines))
	}

	if baseline != nil {
		baseLines := countLines(baseline.Apply())
		growthPct := 0.0
		if baseLines > 0 {
			growthPct = float64(appliedLines-baseLines) / float64(baseLines) * 100
		}
		if growthPct > maxGrowthPct {
			report.OK = false
			report.Violations = append(report.Violations,
				fmt.Sprintf("skill grew %.1f%%, exceeds the %.1f%% growth cap", growthPct, maxGrowthPct))
		}
	}

	return report
}

// CheckSemanticPreservation measures how much of the baseline content survived
// the mutation. It returns the similarity in [0,1] (1 = identical token
// content) and ok = similarity >= DefaultMinSimilarity. A mutation that drops
// below the threshold is treated as "drifted from purpose" and is rejected.
//
// The chosen metric is token-set Jaccard over the two bodies: lowercase words
// and numbers are tokenized (punctuation and markdown syntax are separators),
// then similarity = |A ∩ B| / |A ∪ B|. It is deterministic, O(n), and rewards
// a mutation that keeps the baseline's vocabulary while penalizing a rewrite
// that introduces mostly new terms. It does not model ordering (a reordered
// body that keeps every word scores 1.0), which is the intended "content
// preservation" semantic.
func CheckSemanticPreservation(original, mutated string) (similarity float64, ok bool) {
	similarity = SemanticSimilarity(original, mutated)
	return similarity, similarity >= DefaultMinSimilarity
}

// SemanticSimilarity returns the token-set Jaccard similarity between two
// bodies, in [0,1]. Two empty bodies are identical (1.0); a body compared
// against nothing is maximally different (0.0).
func SemanticSimilarity(a, b string) float64 {
	ta := tokenSet(a)
	tb := tokenSet(b)
	if len(ta) == 0 && len(tb) == 0 {
		return 1.0
	}
	intersection := 0
	for tok := range ta {
		if _, ok := tb[tok]; ok {
			intersection++
		}
	}
	union := len(ta) + len(tb) - intersection
	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

// CheckStructural enforces the catalog identity invariant ("invariante B"):
// name and description must be non-empty and level must be a positive integer.
// Because Apply() derives the frontmatter exclusively from Identity, a
// structural violation can only arise if the identity itself is malformed —
// which is exactly what this guardrail catches before a promotion is offered.
func CheckStructural(genome *Genome) GuardrailReport {
	report := GuardrailReport{OK: true}
	if genome == nil {
		report.OK = false
		report.Violations = append(report.Violations, "candidate genome is nil")
		return report
	}
	id := genome.Identity
	if strings.TrimSpace(id.Name) == "" {
		report.OK = false
		report.Violations = append(report.Violations, "frontmatter name is missing (identity invariant)")
	}
	if strings.TrimSpace(id.Description) == "" {
		report.OK = false
		report.Violations = append(report.Violations, "frontmatter description is missing (identity invariant)")
	}
	if id.Level <= 0 {
		report.OK = false
		report.Violations = append(report.Violations, fmt.Sprintf("frontmatter level is invalid: %d (must be > 0)", id.Level))
	}
	return report
}

// CacheRuleNewSessionOnly is the workflow rule attached by CheckCaching: an
// evolved (and therefore promoted/cached) skill is only valid within the
// session that produced it. Reusing a previous session's cache is forbidden —
// a fresh session must re-run the evolution to get a fresh candidate.
const CacheRuleNewSessionOnly = "evolve: cache only within the session that produced the skill (só nova sessão)"

// CheckCaching attaches the "só nova sessão" caching rule. It is intentionally
// a lightweight note rather than complex logic: the only requirement is that
// the evolution ran in a session with a non-empty sessionID (which the evolve
// command always mints fresh per invocation). A missing sessionID means the
// candidate cannot be safely cached and the promotion is refused.
func CheckCaching(sessionID string) GuardrailReport {
	report := GuardrailReport{OK: true}
	if strings.TrimSpace(sessionID) == "" {
		report.OK = false
		report.Violations = append(report.Violations, CacheRuleNewSessionOnly+": sessionID is required")
	}
	return report
}

// countLines returns the number of lines in s (a trailing empty line does not
// add a line). An empty string has 0 lines.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// tokenSet lowercases s, splits it on any non-letter/non-digit rune, and
// returns the set of distinct tokens. Markdown syntax, punctuation, and
// whitespace all act as separators, so the vocabulary of a SKILL body is what
// is compared.
func tokenSet(s string) map[string]struct{} {
	tokens := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	set := make(map[string]struct{}, len(tokens))
	for _, tok := range tokens {
		set[tok] = struct{}{}
	}
	return set
}
