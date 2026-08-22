package orchestration

import (
	"sync"
	"sync/atomic"
	"time"
)

// ─── Stage Metrics ────────────────────────────────────────────────────────────

// StageMetrics tracks per-stage execution metrics.
type StageMetrics struct {
	StageName     string
	TotalCalls    int64
	SuccessCalls  int64
	ErrorCalls    int64
	TotalDuration time.Duration // cumulative
	MinDuration   time.Duration
	MaxDuration   time.Duration
	LastDuration  time.Duration
	mu            sync.Mutex
}

// record tracks one stage execution.
func (s *StageMetrics) record(success bool, d time.Duration) {
	atomic.AddInt64(&s.TotalCalls, 1)
	if success {
		atomic.AddInt64(&s.SuccessCalls, 1)
	} else {
		atomic.AddInt64(&s.ErrorCalls, 1)
	}

	s.mu.Lock()
	s.TotalDuration += d
	s.LastDuration = d
	if s.MinDuration == 0 || d < s.MinDuration {
		s.MinDuration = d
	}
	if d > s.MaxDuration {
		s.MaxDuration = d
	}
	s.mu.Unlock()
}

// getOrCreateStage returns the StageMetrics for the given stage name,
// creating it if necessary. Must be called while holding m.mu (write lock).
func (m *OrchestrationMetrics) getOrCreateStage(name string) *StageMetrics {
	sm, ok := m.Stages[name]
	if !ok {
		sm = &StageMetrics{StageName: name}
		m.Stages[name] = sm
	}
	return sm
}

// ─── Orchestration Metrics ────────────────────────────────────────────────────

// OrchestrationMetrics tracks overall pipeline metrics.
//
//nolint:revive // Stutter name preserved for API compatibility — used as orchestration.OrchestrationMetrics externally.
type OrchestrationMetrics struct {
	// Per-stage metrics
	Stages map[string]*StageMetrics

	// Overall metrics
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64

	// LLM metrics
	TotalLLMCalls  int64
	TotalLLMTokens int64 // prompt + completion
	LLMErrors      int64
	LLMFallbacks   int64 // primary failed, fallback used

	// Router metrics
	RouterExplicitHits int64 // explicit agent hint used
	RouterKeywordHits  int64 // keyword matching succeeded
	RouterSearchHits   int64 // full-text search succeeded
	RouterFallbacks    int64 // CEO fallback used

	// Embed Cache metrics
	EmbedCacheHits   int64
	EmbedCacheMisses int64

	// MAG metrics
	MAGMemoriesRetrieved int64
	MAGMemoriesStored    int64
	MAGStoreErrors       int64

	// Timing
	TotalDuration time.Duration
	AvgDuration   time.Duration
	mu            sync.RWMutex
}

// NewOrchestrationMetrics creates a new metrics collector.
func NewOrchestrationMetrics() *OrchestrationMetrics {
	return &OrchestrationMetrics{
		Stages: make(map[string]*StageMetrics),
	}
}

// RecordStageStart returns a function that should be called when the stage
// completes. The returned function records the stage duration and success
// status in both the per-stage metrics and (for the "total" metric) the
// aggregated counters.
//
// Usage:
//
//	done := metrics.RecordStageStart("router")
//	// ... do work ...
//	done(true) // or done(false) on error
func (m *OrchestrationMetrics) RecordStageStart(stage string) func(success bool) {
	start := time.Now()
	return func(success bool) {
		d := time.Since(start)

		m.mu.Lock()
		sm := m.getOrCreateStage(stage)
		m.mu.Unlock()

		sm.record(success, d)
	}
}

// RecordLLMCall records an LLM call with token usage.
//   - success: whether the call completed without error
//   - tokens: total tokens (prompt + completion)
//   - fallback: true if a fallback provider or retry layer was used
func (m *OrchestrationMetrics) RecordLLMCall(success bool, tokens int, fallback bool) {
	atomic.AddInt64(&m.TotalLLMCalls, 1)
	atomic.AddInt64(&m.TotalLLMTokens, int64(tokens))
	if !success {
		atomic.AddInt64(&m.LLMErrors, 1)
	}
	if fallback {
		atomic.AddInt64(&m.LLMFallbacks, 1)
	}
}

// RecordRouterDecision records how the agent was selected.
// method must be one of: "explicit", "keyword", "search", "fallback".
func (m *OrchestrationMetrics) RecordRouterDecision(method string) {
	switch method {
	case "explicit":
		atomic.AddInt64(&m.RouterExplicitHits, 1)
	case "keyword":
		atomic.AddInt64(&m.RouterKeywordHits, 1)
	case "search":
		atomic.AddInt64(&m.RouterSearchHits, 1)
	case "fallback":
		atomic.AddInt64(&m.RouterFallbacks, 1)
	}
}

// RecordEmbedCacheHit records a knowledge cache hit.
func (m *OrchestrationMetrics) RecordEmbedCacheHit() {
	atomic.AddInt64(&m.EmbedCacheHits, 1)
}

// RecordEmbedCacheMiss records a knowledge cache miss.
func (m *OrchestrationMetrics) RecordEmbedCacheMiss() {
	atomic.AddInt64(&m.EmbedCacheMisses, 1)
}

// RecordMAGRetrieve records memory retrieval.
func (m *OrchestrationMetrics) RecordMAGRetrieve(count int) {
	atomic.AddInt64(&m.MAGMemoriesRetrieved, int64(count))
}

// RecordMAGStore records memory storage.
func (m *OrchestrationMetrics) RecordMAGStore(success bool) {
	if success {
		atomic.AddInt64(&m.MAGMemoriesStored, 1)
	} else {
		atomic.AddInt64(&m.MAGStoreErrors, 1)
	}
}

// RecordRequest records overall request outcome.
func (m *OrchestrationMetrics) RecordRequest(success bool, duration time.Duration) {
	atomic.AddInt64(&m.TotalRequests, 1)
	if success {
		atomic.AddInt64(&m.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&m.FailedRequests, 1)
	}

	m.mu.Lock()
	m.TotalDuration += duration
	// Recompute average: total duration / total requests.
	total := atomic.LoadInt64(&m.TotalRequests)
	if total > 0 {
		// Use nanoseconds for precision then convert.
		m.AvgDuration = time.Duration(int64(m.TotalDuration) / total)
	}
	m.mu.Unlock()
}

// ─── Snapshot ─────────────────────────────────────────────────────────────────

// MetricsSnapshot is a JSON-serializable snapshot of metrics.
type MetricsSnapshot struct {
	Timestamp            time.Time             `json:"timestamp"`
	TotalRequests        int64                 `json:"total_requests"`
	SuccessfulRequests   int64                 `json:"successful_requests"`
	FailedRequests       int64                 `json:"failed_requests"`
	SuccessRate          float64               `json:"success_rate"`
	TotalLLMCalls        int64                 `json:"total_llm_calls"`
	TotalLLMTokens       int64                 `json:"total_llm_tokens"`
	LLMErrors            int64                 `json:"llm_errors"`
	LLMFallbacks         int64                 `json:"llm_fallbacks"`
	AvgDurationMs        float64               `json:"avg_duration_ms"`
	RouterExplicitHits   int64                 `json:"router_explicit_hits"`
	RouterKeywordHits    int64                 `json:"router_keyword_hits"`
	RouterSearchHits     int64                 `json:"router_search_hits"`
	RouterFallbacks      int64                 `json:"router_fallbacks"`
	EmbedCacheHits       int64                 `json:"embed_cache_hits"`
	EmbedCacheMisses     int64                 `json:"embed_cache_misses"`
	MAGMemoriesRetrieved int64                 `json:"mag_memories_retrieved"`
	MAGMemoriesStored    int64                 `json:"mag_memories_stored"`
	StageBreakdown       map[string]StageStats `json:"stage_breakdown"`
}

// StageStats is per-stage statistics.
type StageStats struct {
	Calls       int64   `json:"calls"`
	Successes   int64   `json:"successes"`
	Errors      int64   `json:"errors"`
	ErrorRate   float64 `json:"error_rate"`
	AvgDuration float64 `json:"avg_duration_ms"`
	MinDuration float64 `json:"min_duration_ms"`
	MaxDuration float64 `json:"max_duration_ms"`
}

// Snapshot returns a point-in-time copy of all metrics.
// The returned snapshot is safe for JSON serialization.
func (m *OrchestrationMetrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap := MetricsSnapshot{
		Timestamp:            time.Now(),
		TotalRequests:        atomic.LoadInt64(&m.TotalRequests),
		SuccessfulRequests:   atomic.LoadInt64(&m.SuccessfulRequests),
		FailedRequests:       atomic.LoadInt64(&m.FailedRequests),
		TotalLLMCalls:        atomic.LoadInt64(&m.TotalLLMCalls),
		TotalLLMTokens:       atomic.LoadInt64(&m.TotalLLMTokens),
		LLMErrors:            atomic.LoadInt64(&m.LLMErrors),
		LLMFallbacks:         atomic.LoadInt64(&m.LLMFallbacks),
		RouterExplicitHits:   atomic.LoadInt64(&m.RouterExplicitHits),
		RouterKeywordHits:    atomic.LoadInt64(&m.RouterKeywordHits),
		RouterSearchHits:     atomic.LoadInt64(&m.RouterSearchHits),
		RouterFallbacks:      atomic.LoadInt64(&m.RouterFallbacks),
		EmbedCacheHits:       atomic.LoadInt64(&m.EmbedCacheHits),
		EmbedCacheMisses:     atomic.LoadInt64(&m.EmbedCacheMisses),
		MAGMemoriesRetrieved: atomic.LoadInt64(&m.MAGMemoriesRetrieved),
		MAGMemoriesStored:    atomic.LoadInt64(&m.MAGMemoriesStored),
		StageBreakdown:       make(map[string]StageStats, len(m.Stages)),
	}

	// Compute success rate.
	if snap.TotalRequests > 0 {
		snap.SuccessRate = float64(snap.SuccessfulRequests) / float64(snap.TotalRequests)
	}

	// Compute average duration in milliseconds.
	if snap.TotalRequests > 0 {
		snap.AvgDurationMs = float64(m.TotalDuration.Nanoseconds()) / float64(snap.TotalRequests) / float64(time.Millisecond)
	}

	// Per-stage statistics.
	for name, sm := range m.Stages {
		stats := StageStats{
			Calls:     atomic.LoadInt64(&sm.TotalCalls),
			Successes: atomic.LoadInt64(&sm.SuccessCalls),
			Errors:    atomic.LoadInt64(&sm.ErrorCalls),
		}

		// Compute error rate safely (avoid NaN when 0 calls)
		if stats.Calls > 0 {
			stats.ErrorRate = float64(stats.Errors) / float64(stats.Calls)
		}

		// Duration stats in milliseconds.
		sm.mu.Lock()
		if stats.Calls > 0 {
			stats.AvgDuration = float64(sm.TotalDuration.Nanoseconds()) / float64(stats.Calls) / float64(time.Millisecond)
		}
		stats.MinDuration = float64(sm.MinDuration.Nanoseconds()) / float64(time.Millisecond)
		stats.MaxDuration = float64(sm.MaxDuration.Nanoseconds()) / float64(time.Millisecond)
		sm.mu.Unlock()

		snap.StageBreakdown[name] = stats
	}

	return snap
}

// Reset zeroes all metrics.
// Atomic counters are set to 0; maps are cleared.
func (m *OrchestrationMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Atomic counters.
	atomic.StoreInt64(&m.TotalRequests, 0)
	atomic.StoreInt64(&m.SuccessfulRequests, 0)
	atomic.StoreInt64(&m.FailedRequests, 0)
	atomic.StoreInt64(&m.TotalLLMCalls, 0)
	atomic.StoreInt64(&m.TotalLLMTokens, 0)
	atomic.StoreInt64(&m.LLMErrors, 0)
	atomic.StoreInt64(&m.LLMFallbacks, 0)
	atomic.StoreInt64(&m.RouterExplicitHits, 0)
	atomic.StoreInt64(&m.RouterKeywordHits, 0)
	atomic.StoreInt64(&m.RouterSearchHits, 0)
	atomic.StoreInt64(&m.RouterFallbacks, 0)
	atomic.StoreInt64(&m.EmbedCacheHits, 0)
	atomic.StoreInt64(&m.EmbedCacheMisses, 0)
	atomic.StoreInt64(&m.MAGMemoriesRetrieved, 0)
	atomic.StoreInt64(&m.MAGMemoriesStored, 0)
	atomic.StoreInt64(&m.MAGStoreErrors, 0)

	// Duration fields.
	m.TotalDuration = 0
	m.AvgDuration = 0

	// Per-stage metrics: clear the map.
	m.Stages = make(map[string]*StageMetrics)
}
