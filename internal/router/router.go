// Package router implements weighted scoring and ranking of LLM provider
// candidates, with "mode packs" that bias routing toward a single objective
// (health, cost, latency, or task fit). It is a pure package: no I/O, no
// external dependencies — callers feed it pre-computed candidate metrics and
// consume the ranked result.
//
// The package is inspired by the OmniRoute pattern: a single pre-dispatch
// decision (Decide) turns a set of candidates into either an ordered list or
// a rejection, instead of scattering ad-hoc selection logic across call sites.
package router

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Mode identifies a routing "mode pack": a predefined set of weights that
// biases candidate selection toward a particular objective.
type Mode string

// Predefined routing modes.
const (
	// ModeBalanced weights all four metrics roughly equally.
	ModeBalanced Mode = "balanced"
	// ModeFast favors low latency.
	ModeFast Mode = "fast"
	// ModeCheap favors low cost.
	ModeCheap Mode = "cheap"
	// ModeQuality favors task fit (and health).
	ModeQuality Mode = "quality"
	// ModeOffline restricts routing to offline (local) providers only.
	ModeOffline Mode = "offline"
)

// String returns the canonical string form of the mode.
func (m Mode) String() string { return string(m) }

// ModeFromString parses a mode name into a Mode. It is case-insensitive and
// ignores surrounding whitespace. Unknown names return ErrUnknownMode.
func ModeFromString(s string) (Mode, error) {
	m := Mode(strings.ToLower(strings.TrimSpace(s)))
	if !m.valid() {
		return "", fmt.Errorf("%w: %q", ErrUnknownMode, s)
	}
	return m, nil
}

// valid reports whether the mode is one of the predefined routing modes.
func (m Mode) valid() bool {
	switch m {
	case ModeBalanced, ModeFast, ModeCheap, ModeQuality, ModeOffline:
		return true
	default:
		return false
	}
}

// Weights are normalized routing weights (each 0..1, summing to 1.0).
type Weights struct {
	Health  float64
	Cost    float64
	Latency float64
	TaskFit float64
}

// WeightsForMode returns the predefined weight profile for the given mode.
// Unknown modes fall back to the balanced profile.
func WeightsForMode(m Mode) Weights {
	switch m {
	case ModeFast:
		return Weights{Health: 0.20, Cost: 0.10, Latency: 0.55, TaskFit: 0.15}
	case ModeCheap:
		return Weights{Health: 0.20, Cost: 0.60, Latency: 0.10, TaskFit: 0.10}
	case ModeQuality:
		return Weights{Health: 0.35, Cost: 0.05, Latency: 0.10, TaskFit: 0.50}
	case ModeOffline:
		// Offline providers are free, so cost is irrelevant; local availability
		// (health) and latency dominate, with task fit still informing ranking.
		return Weights{Health: 0.40, Cost: 0.00, Latency: 0.30, TaskFit: 0.30}
	default:
		return Weights{Health: 0.35, Cost: 0.25, Latency: 0.15, TaskFit: 0.25}
	}
}

// Validate reports whether the weights are well-formed: every weight
// non-negative and the sum equal to 1.0 (within a small floating-point
// tolerance).
func (w Weights) Validate() error {
	if w.Health < 0 || w.Cost < 0 || w.Latency < 0 || w.TaskFit < 0 {
		return fmt.Errorf("weights must be non-negative: %+v", w)
	}
	sum := w.Health + w.Cost + w.Latency + w.TaskFit
	if math.Abs(sum-1.0) > 1e-6 {
		return fmt.Errorf("weights must sum to 1.0, got %v", sum)
	}
	return nil
}

// Candidate is a single provider under consideration for routing. All metric
// fields are normalized to 0..1 with higher meaning better:
//
//	Health  1.0 = healthy (circuit closed), 0.0 = open (see circuitbreaker).
//	Cost    1.0 = cheapest (inverse of price, normalized by the caller).
//	Latency 1.0 = fastest (inverse of average latency, normalized).
//	TaskFit 1.0 = best fit for the requested task.
//	Offline marks a local provider (e.g. ollama/local).
//
// Available=false excludes the candidate from ranking; Offline only matters
// for ModeOffline.
type Candidate struct {
	Name      string
	Health    float64
	Cost      float64
	Latency   float64
	TaskFit   float64
	Available bool
	Offline   bool
}

// Score computes the weighted score (0..1) for a candidate under the given
// weights. It is the weighted sum of the four metrics.
func Score(c Candidate, w Weights) float64 {
	return w.Health*c.Health + w.Cost*c.Cost + w.Latency*c.Latency + w.TaskFit*c.TaskFit
}

// Rank filters and sorts candidates for the given mode.
//
// Filtering: unavailable candidates and candidates with a non-positive Health
// (open circuit) are dropped. ModeOffline additionally restricts the set to
// Offline providers, returning ErrNoOfflineProvider when none remain.
//
// Ordering: descending by Score, with a lexicographic Name tie-break for
// deterministic output.
//
// Fail-open: when filtering empties the list (and mode is not ModeOffline),
// the full original candidate set is returned ranked by Score instead of an
// error, degrading gracefully rather than refusing to route. ErrNoCandidates
// is returned only when the input slice itself is empty.
func Rank(candidates []Candidate, mode Mode) ([]Candidate, error) {
	if !mode.valid() {
		return nil, fmt.Errorf("%w: %q", ErrUnknownMode, mode)
	}
	if len(candidates) == 0 {
		return nil, ErrNoCandidates
	}

	w := WeightsForMode(mode)
	filtered := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if mode == ModeOffline && !c.Offline {
			continue
		}
		if !c.Available {
			continue
		}
		if c.Health <= 0 {
			continue
		}
		filtered = append(filtered, c)
	}

	if len(filtered) == 0 {
		if mode == ModeOffline {
			return nil, ErrNoOfflineProvider
		}
		filtered = append([]Candidate(nil), candidates...)
	}

	sortByScore(filtered, w)
	return filtered, nil
}

// sortByScore orders candidates by descending Score, tie-broken by Name in
// ascending lexicographic order for deterministic output.
func sortByScore(cs []Candidate, w Weights) {
	sort.Slice(cs, func(i, j int) bool {
		si := Score(cs[i], w)
		sj := Score(cs[j], w)
		if si != sj {
			return si > sj
		}
		return cs[i].Name < cs[j].Name
	})
}

// Verdict is a single pre-dispatch routing decision.
type Verdict struct {
	// Allowed reports whether the best candidate meets the minimum score.
	Allowed bool
	// Reason explains a rejection (empty when allowed).
	Reason string
	// Ordered is the ranked candidate list (empty when Rank returned an error).
	Ordered []Candidate
}

// Decide ranks candidates and gates dispatch on a minimum score threshold.
// When the best available score falls below minScore it returns Allowed=false
// with an explanatory Reason; otherwise Allowed=true with the ranked list.
// Ranking errors (empty input, unknown mode, no offline provider) surface as
// Allowed=false with the error text as Reason.
func Decide(candidates []Candidate, mode Mode, minScore float64) Verdict {
	ordered, err := Rank(candidates, mode)
	if err != nil {
		return Verdict{Allowed: false, Reason: err.Error()}
	}

	w := WeightsForMode(mode)
	best := Score(ordered[0], w)
	if best < minScore {
		return Verdict{
			Allowed: false,
			Reason:  fmt.Sprintf("best score %.2f below threshold %.2f", best, minScore),
			Ordered: ordered,
		}
	}

	return Verdict{Allowed: true, Ordered: ordered}
}

// Sentinel errors.
var (
	// ErrNoCandidates is returned when Rank is given an empty input slice.
	ErrNoCandidates = errors.New("router: no candidates")
	// ErrNoOfflineProvider is returned when ModeOffline finds no offline
	// provider after filtering.
	ErrNoOfflineProvider = errors.New("router: no offline provider available")
	// ErrUnknownMode is returned when parsing or ranking an unrecognized mode.
	ErrUnknownMode = errors.New("router: unknown mode")
)
