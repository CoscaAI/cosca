package chat

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/circuitbreaker"
)

// latencyEMASmoothing is the exponential moving average factor applied to the
// observed per-call latency score (higher = more responsive to recent samples).
const latencyEMASmoothing = 0.3

// latencyBaseline normalizes a raw duration into a 0..1 "speed" score via
// 1 - d/(d+baseline). With a 500ms baseline this yields ~1.0 for sub-50ms
// calls, ~0.91 for 50ms, and ~0.2 for 2s — a monotonic decreasing function
// that never leaves the 0..1 range.
const latencyBaseline = 500 * time.Millisecond

// defaultLatencyEMA is the neutral starting latency score for a provider with
// no recorded samples: neither faster nor slower than the median.
const defaultLatencyEMA = 0.5

// defaultCostScore is applied to providers with no explicit cost tier.
const defaultCostScore = 0.5

// defaultTaskFit is the neutral task-fit score. v1 routes with a fixed neutral
// value; this is an extension point for callers to supply real task-based
// signals (e.g. "needs vision" → prefer gemini).
const defaultTaskFit = 0.5

// providerHealth tracks per-provider circuit breaker state and a smoothed
// latency score. It is the bridge between the live ChatRegistry call path and
// the pure router.Candidate metrics: every success/failure on the hot path is
// recorded here, and Select() reads the resulting health/latency when building
// router candidates. The values are passive — they inform routing preference
// but never gate or short-circuit a request.
type providerHealth struct {
	mu sync.Mutex

	breaker *circuitbreaker.CircuitBreaker

	// latencyEMA is a 0..1 score (1.0 = fastest) smoothed with an EMA over
	// the normalized latency of each successful call. Defaults to 0.5.
	latencyEMA float64

	// samples counts recorded successful calls, for observability.
	samples int
}

// newProviderHealth returns a fresh tracker: a closed circuit with default
// thresholds and a neutral latency estimate.
func newProviderHealth() *providerHealth {
	return &providerHealth{
		breaker:    circuitbreaker.New(circuitbreaker.DefaultConfig()),
		latencyEMA: defaultLatencyEMA,
	}
}

// recordSuccess records a successful call that took dur. The circuit breaker
// is advanced through its Execute path so the success participates in closed /
// half-open recovery exactly like a real gated call, and the latency EMA is
// updated with a monotonic speed score derived from dur.
func (h *providerHealth) recordSuccess(dur time.Duration) {
	if dur < 0 {
		dur = 0
	}
	_, _ = h.breaker.Execute(func() (interface{}, error) { return nil, nil })

	instant := latencyScore(dur)
	h.mu.Lock()
	h.latencyEMA = latencyEMASmoothing*instant + (1-latencyEMASmoothing)*h.latencyEMA
	h.samples++
	h.mu.Unlock()
}

// recordFailure records a failed call. It advances the circuit breaker so
// repeated failures eventually trip the circuit to open, which degrades the
// provider's Health score used by router.Rank.
func (h *providerHealth) recordFailure() {
	_, _ = h.breaker.Execute(func() (interface{}, error) { return nil, errHealthFailure })
}

// health reports the 0..1 health score derived from the breaker state:
// closed=1.0, half-open=0.5, open=0.0.
func (h *providerHealth) health() float64 {
	switch h.breaker.State() {
	case circuitbreaker.StateClosed:
		return 1.0
	case circuitbreaker.StateHalfOpen:
		return 0.5
	default:
		return 0.0
	}
}

// latency returns the current 0..1 latency score (1.0 = fastest).
func (h *providerHealth) latency() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.latencyEMA
}

// reset restores the tracker to its default state: closed circuit, neutral
// latency, no samples.
func (h *providerHealth) reset() {
	h.breaker.Reset()
	h.mu.Lock()
	h.latencyEMA = defaultLatencyEMA
	h.samples = 0
	h.mu.Unlock()
}

// latencyScore maps a raw call duration to a 0..1 "speed" score, monotonically
// decreasing with duration: ~1.0 for sub-50ms calls and ~0.2 for 2s calls.
func latencyScore(dur time.Duration) float64 {
	f := float64(dur)
	base := float64(latencyBaseline)
	return 1 - f/(f+base)
}

// costScore returns a static 0..1 cost estimate (1.0 = free) for a provider
// identified by its name and model. It is an extension point: replace this
// table with live pricing data without touching the routing logic. Tiers:
//
//	ollama / local / localhost  → 1.0 (free, local inference)
//	deepseek / groq             → 0.8
//	azure / bedrock / mistral   → 0.5
//	openai / gemini / anthropic → 0.4
//	default                     → 0.5
func costScore(name, model string) float64 {
	n := strings.ToLower(name + " " + model)
	switch {
	case containsAny(n, "ollama", "local", "localhost"):
		return 1.0
	case containsAny(n, "deepseek", "groq"):
		return 0.8
	case containsAny(n, "azure", "bedrock", "mistral"):
		return 0.5
	case containsAny(n, "openai", "gemini", "anthropic"):
		return 0.4
	default:
		return defaultCostScore
	}
}

// containsAny reports whether haystack contains any of the given substrings.
func containsAny(haystack string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(haystack, sub) {
			return true
		}
	}
	return false
}

// isOfflineProvider reports whether the provider runs locally (e.g. ollama).
// It feeds router.Candidate.Offline, which ModeOffline routes exclusively to.
func isOfflineProvider(name string) bool {
	return strings.EqualFold(name, "ollama") || strings.EqualFold(name, "local")
}

// errHealthFailure is an internal sentinel used to record a failure through
// the circuit breaker Execute path. It is never surfaced to callers.
var errHealthFailure = errors.New("chat: provider health failure")
