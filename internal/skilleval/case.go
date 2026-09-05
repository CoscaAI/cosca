package skilleval

import (
	"math"
	"sort"
	"time"

	"github.com/CoscaAI/cosca/internal/evals"
)

// SolveThreshold is the per-case score at or above which a case counts as
// "solved" when aggregating a Condition. It mirrors the neutral default used
// by the scaler and the binary cut-off already established in internal/evals.
const SolveThreshold = 0.5

// GradingResult is the outcome of grading one skill output against a rubric.
type GradingResult struct {
	Passed   int           // number of rubric conditions passed
	Total    int           // number of rubric conditions evaluated
	Score    float64       // pass-rate over the rubric, clamped to [0,1]
	Feedback []string      // per-condition feedback, one entry per rubric item
	Tokens   int           // tokens consumed (LLM scoring; 0 for static)
	Duration time.Duration // wall time spent grading the output
}

// Condition is the robust summary statistics for a benchmark arm. Dispersion
// is reported as median + IQR (never the mean), and the uncertainty of the
// mean is captured by a bootstrap std via internal/evals.BootstrapStd.
type Condition struct {
	SolveRate     float64    // fraction of cases solved (Score >= SolveThreshold)
	MedianScore   float64    // median Score across cases
	IQRS          [2]float64 // interquartile range bounds [Q1, Q3]
	MeanBootstrap float64    // bootstrap std of the mean (via evals.BootstrapStd)
	MedianTokens  int        // median Tokens across cases
	MedianTimeMS  int        // median Duration in milliseconds
}

// Delta captures the movement between two arms of a benchmark.
type Delta struct {
	Score     float64 // delta of the mean score between the two arms
	ScoreStd  float64 // bootstrap std of that delta (dispersion of the movement)
	Candidate bool    // whether this delta marks the arm as a candidate
}

// SkillBenchmark is the A/B comparison record for a single skill.
type SkillBenchmark struct {
	Skill, Date string
	With        Condition // arm with the candidate skill body
	Without     Condition // arm with the baseline skill body
	Delta       Delta
	IsCandidate bool
	Gate        *GateResult
}

// GateResult is the regression gate that gates promotion of a skill body to
// candidate status. Each sub-gate is a pass/fail boolean; Details carries the
// human-readable notes for each verdict.
type GateResult struct {
	CatalogAudit bool     // catalog consistency audit passed
	RegTests     bool     // regression test suite passed
	RegDelta     bool     // regression delta within tolerance
	Passed       bool     // overall gate verdict
	Details      []string // per-gate notes
}

// BuildCondition aggregates a batch of grading results into a robust
// Condition. It reuses the deterministic bootstrap primitive from
// internal/evals (BootstrapStd) rather than duplicating it. For an empty batch
// it returns the zero Condition.
func BuildCondition(results []GradingResult) Condition {
	if len(results) == 0 {
		return Condition{}
	}

	scores := make([]float64, 0, len(results))
	tokens := make([]int, 0, len(results))
	times := make([]int, 0, len(results))
	var solved int

	for _, r := range results {
		scores = append(scores, r.Score)
		tokens = append(tokens, r.Tokens)
		times = append(times, int(r.Duration.Milliseconds()))
		if r.Score >= SolveThreshold {
			solved++
		}
	}

	return Condition{
		SolveRate:     float64(solved) / float64(len(results)),
		MedianScore:   medianFloat(scores),
		IQRS:          [2]float64{quantileFloat(scores, 0.25), quantileFloat(scores, 0.75)},
		MeanBootstrap: evals.BootstrapStd(scores, evals.DefaultBootstrapSamples),
		MedianTokens:  medianInt(tokens),
		MedianTimeMS:  medianInt(times),
	}
}

// medianFloat returns the median of xs (operating on a sorted copy). An empty
// slice yields 0.
func medianFloat(xs []float64) float64 {
	return quantileFloat(xs, 0.5)
}

// quantileFloat returns the q-quantile (0..1) of xs using linear interpolation
// between the two nearest order statistics (the "R-7" / NumPy default). An
// empty slice yields 0.
func quantileFloat(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	if q <= 0 {
		return s[0]
	}
	if q >= 1 {
		return s[len(s)-1]
	}
	pos := q * float64(len(s)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return s[lo]
	}
	frac := pos - float64(lo)
	return s[lo]*(1-frac) + s[hi]*frac
}

// medianInt returns the median of xs (operating on a sorted copy), rounding
// the middle average down for even-length inputs. An empty slice yields 0.
func medianInt(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int(nil), xs...)
	sort.Ints(s)
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}
