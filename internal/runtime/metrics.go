//
// Package runtime provides runtime metrics collection including uptime tracking,
// operation counters, request duration histograms, and component health gauges.

package runtime

import (
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// Metrics
// =============================================================================

// Metrics collects runtime metrics including operation counters,
// timing histograms, memory tracking, and component health gauges.
type Metrics struct {
	mu sync.RWMutex

	// Uptime tracking
	startedAt atomic.Value // time.Time

	// Operation counters
	indexCount    atomic.Int64
	searchCount   atomic.Int64
	contextBuilds atomic.Int64
	memoryStores  atomic.Int64
	pluginCalls   atomic.Int64
	errorCount    atomic.Int64
	syncCount     atomic.Int64
	eventCount    atomic.Int64

	// Duration histograms (nanoseconds)
	indexDurations   *durationHistogram
	searchDurations  *durationHistogram
	contextDurations *durationHistogram
	memoryDurations  *durationHistogram

	// Component health
	componentHealth sync.Map // map[string]ComponentStatus

	// Goroutine count samples
	goroutineSamples []int
}

// NewMetrics creates a new metrics collector.
func NewMetrics() *Metrics {
	m := &Metrics{
		indexDurations:   newDurationHistogram(),
		searchDurations:  newDurationHistogram(),
		contextDurations: newDurationHistogram(),
		memoryDurations:  newDurationHistogram(),
	}
	m.startedAt.Store(time.Now())
	return m
}

// =============================================================================
// Uptime
// =============================================================================

// StartTime returns the time the runtime was started.
func (m *Metrics) StartTime() time.Time {
	v := m.startedAt.Load()
	if t, ok := v.(time.Time); ok {
		return t
	}
	return time.Now()
}

// Uptime returns the duration since the runtime started.
func (m *Metrics) Uptime() time.Duration {
	return time.Since(m.StartTime())
}

// =============================================================================
// Operation Counters
// =============================================================================

// IncrementIndex increments the index operation counter.
func (m *Metrics) IncrementIndex() {
	m.indexCount.Add(1)
}

// IndexCount returns the number of index operations.
func (m *Metrics) IndexCount() int64 {
	return m.indexCount.Load()
}

// IncrementSearch increments the search operation counter.
func (m *Metrics) IncrementSearch() {
	m.searchCount.Add(1)
}

// SearchCount returns the number of search operations.
func (m *Metrics) SearchCount() int64 {
	return m.searchCount.Load()
}

// IncrementContextBuild increments the context build counter.
func (m *Metrics) IncrementContextBuild() {
	m.contextBuilds.Add(1)
}

// ContextBuildCount returns the number of context builds.
func (m *Metrics) ContextBuildCount() int64 {
	return m.contextBuilds.Load()
}

// IncrementMemoryStore increments the memory store counter.
func (m *Metrics) IncrementMemoryStore() {
	m.memoryStores.Add(1)
}

// MemoryStoreCount returns the number of memory store operations.
func (m *Metrics) MemoryStoreCount() int64 {
	return m.memoryStores.Load()
}

// IncrementPluginCall increments the plugin call counter.
func (m *Metrics) IncrementPluginCall() {
	m.pluginCalls.Add(1)
}

// PluginCallCount returns the number of plugin calls.
func (m *Metrics) PluginCallCount() int64 {
	return m.pluginCalls.Load()
}

// IncrementError increments the error counter.
func (m *Metrics) IncrementError() {
	m.errorCount.Add(1)
}

// ErrorCount returns the number of errors.
func (m *Metrics) ErrorCount() int64 {
	return m.errorCount.Load()
}

// IncrementSync increments the sync operation counter.
func (m *Metrics) IncrementSync() {
	m.syncCount.Add(1)
}

// SyncCount returns the number of sync operations.
func (m *Metrics) SyncCount() int64 {
	return m.syncCount.Load()
}

// IncrementEvent increments the event counter.
func (m *Metrics) IncrementEvent() {
	m.eventCount.Add(1)
}

// EventCount returns the number of events processed.
func (m *Metrics) EventCount() int64 {
	return m.eventCount.Load()
}

// =============================================================================
// Duration Histograms
// =============================================================================

// RecordIndexDuration records the duration of an index operation.
func (m *Metrics) RecordIndexDuration(d time.Duration) {
	m.indexDurations.Record(d)
}

// RecordSearchDuration records the duration of a search operation.
func (m *Metrics) RecordSearchDuration(d time.Duration) {
	m.searchDurations.Record(d)
}

// RecordContextDuration records the duration of a context build.
func (m *Metrics) RecordContextDuration(d time.Duration) {
	m.contextDurations.Record(d)
}

// RecordMemoryDuration records the duration of a memory operation.
func (m *Metrics) RecordMemoryDuration(d time.Duration) {
	m.memoryDurations.Record(d)
}

// =============================================================================
// Duration Percentiles
// =============================================================================

// IndexDurationPercentiles returns p50, p95, p99 for index operations.
func (m *Metrics) IndexDurationPercentiles() (p50, p95, p99 time.Duration) {
	return m.indexDurations.Percentiles()
}

// SearchDurationPercentiles returns p50, p95, p99 for search operations.
func (m *Metrics) SearchDurationPercentiles() (p50, p95, p99 time.Duration) {
	return m.searchDurations.Percentiles()
}

// ContextDurationPercentiles returns p50, p95, p99 for context builds.
func (m *Metrics) ContextDurationPercentiles() (p50, p95, p99 time.Duration) {
	return m.contextDurations.Percentiles()
}

// MemoryDurationPercentiles returns p50, p95, p99 for memory operations.
func (m *Metrics) MemoryDurationPercentiles() (p50, p95, p99 time.Duration) {
	return m.memoryDurations.Percentiles()
}

// =============================================================================
// Component Health
// =============================================================================

// SetComponentHealth sets the health status of a component.
func (m *Metrics) SetComponentHealth(name string, status ComponentStatus) {
	m.componentHealth.Store(name, status)
}

// GetComponentHealth returns the health status of a component.
func (m *Metrics) GetComponentHealth(name string) ComponentStatus {
	val, ok := m.componentHealth.Load(name)
	if !ok {
		return StatusUnknown
	}
	if s, ok := val.(ComponentStatus); ok {
		return s
	}
	return StatusUnknown
}

// AllComponentHealth returns the health of all tracked components.
func (m *Metrics) AllComponentHealth() map[string]ComponentStatus {
	result := make(map[string]ComponentStatus)
	m.componentHealth.Range(func(key, value interface{}) bool {
		if name, ok := key.(string); ok {
			if status, ok := value.(ComponentStatus); ok {
				result[name] = status
			}
		}
		return true
	})
	return result
}

// =============================================================================
// System Metrics
// =============================================================================

// NumGoroutine returns the current number of goroutines.
func (m *Metrics) NumGoroutine() int {
	return runtime.NumGoroutine()
}

// RecordGoroutineSample records a goroutine count sample.
func (m *Metrics) RecordGoroutineSample() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.goroutineSamples = append(m.goroutineSamples, runtime.NumGoroutine())
}

// GoroutineStats returns minVal, maxVal, avg goroutine counts from samples.
func (m *Metrics) GoroutineStats() (minVal, maxVal, avg int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.goroutineSamples) == 0 {
		return 0, 0, 0
	}
	minVal = m.goroutineSamples[0]
	maxVal = m.goroutineSamples[0]
	sum := 0
	for _, v := range m.goroutineSamples {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
		sum += v
	}
	return minVal, maxVal, sum / len(m.goroutineSamples)
}

// MemoryUsage returns approximate memory usage in bytes.
func (m *Metrics) MemoryUsage() uint64 {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return mem.Alloc
}

// TotalAllocated returns total bytes allocated over lifetime.
func (m *Metrics) TotalAllocated() uint64 {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return mem.TotalAlloc
}

// =============================================================================
// Snapshot
// =============================================================================

// MetricsSnapshot is a point-in-time snapshot of all metrics.
type MetricsSnapshot struct {
	Uptime    time.Duration `json:"uptime" yaml:"uptime"`
	StartTime time.Time     `json:"start_time" yaml:"start_time"`

	// Counters
	IndexCount    int64 `json:"index_count" yaml:"index_count"`
	SearchCount   int64 `json:"search_count" yaml:"search_count"`
	ContextBuilds int64 `json:"context_builds" yaml:"context_builds"`
	MemoryStores  int64 `json:"memory_stores" yaml:"memory_stores"`
	PluginCalls   int64 `json:"plugin_calls" yaml:"plugin_calls"`
	ErrorCount    int64 `json:"error_count" yaml:"error_count"`
	SyncCount     int64 `json:"sync_count" yaml:"sync_count"`
	EventCount    int64 `json:"event_count" yaml:"event_count"`

	// Duration Percentiles
	IndexDurationP50   time.Duration `json:"index_duration_p50" yaml:"index_duration_p50"`
	IndexDurationP95   time.Duration `json:"index_duration_p95" yaml:"index_duration_p95"`
	IndexDurationP99   time.Duration `json:"index_duration_p99" yaml:"index_duration_p99"`
	SearchDurationP50  time.Duration `json:"search_duration_p50" yaml:"search_duration_p50"`
	SearchDurationP95  time.Duration `json:"search_duration_p95" yaml:"search_duration_p95"`
	SearchDurationP99  time.Duration `json:"search_duration_p99" yaml:"search_duration_p99"`
	ContextDurationP50 time.Duration `json:"context_duration_p50" yaml:"context_duration_p50"`
	ContextDurationP95 time.Duration `json:"context_duration_p95" yaml:"context_duration_p95"`
	ContextDurationP99 time.Duration `json:"context_duration_p99" yaml:"context_duration_p99"`
	MemoryDurationP50  time.Duration `json:"memory_duration_p50" yaml:"memory_duration_p50"`
	MemoryDurationP95  time.Duration `json:"memory_duration_p95" yaml:"memory_duration_p95"`
	MemoryDurationP99  time.Duration `json:"memory_duration_p99" yaml:"memory_duration_p99"`

	// System
	Goroutines      int                        `json:"goroutines" yaml:"goroutines"`
	MemoryAlloc     uint64                     `json:"memory_alloc_bytes" yaml:"memory_alloc_bytes"`
	TotalAlloc      uint64                     `json:"total_alloc_bytes" yaml:"total_alloc_bytes"`
	ComponentHealth map[string]ComponentStatus `json:"component_health" yaml:"component_health"`

	// Goroutine stats
	GoroutineMin int `json:"goroutine_min" yaml:"goroutine_min"`
	GoroutineMax int `json:"goroutine_max" yaml:"goroutine_max"`
	GoroutineAvg int `json:"goroutine_avg" yaml:"goroutine_avg"`
}

// Snapshot returns a point-in-time snapshot of all metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	idxP50, idxP95, idxP99 := m.IndexDurationPercentiles()
	srP50, srP95, srP99 := m.SearchDurationPercentiles()
	ctxP50, ctxP95, ctxP99 := m.ContextDurationPercentiles()
	memP50, memP95, memP99 := m.MemoryDurationPercentiles()

	gMin, gMax, gAvg := m.GoroutineStats()

	return MetricsSnapshot{
		Uptime:             m.Uptime(),
		StartTime:          m.StartTime(),
		IndexCount:         m.IndexCount(),
		SearchCount:        m.SearchCount(),
		ContextBuilds:      m.ContextBuildCount(),
		MemoryStores:       m.MemoryStoreCount(),
		PluginCalls:        m.PluginCallCount(),
		ErrorCount:         m.ErrorCount(),
		SyncCount:          m.SyncCount(),
		EventCount:         m.EventCount(),
		IndexDurationP50:   idxP50,
		IndexDurationP95:   idxP95,
		IndexDurationP99:   idxP99,
		SearchDurationP50:  srP50,
		SearchDurationP95:  srP95,
		SearchDurationP99:  srP99,
		ContextDurationP50: ctxP50,
		ContextDurationP95: ctxP95,
		ContextDurationP99: ctxP99,
		MemoryDurationP50:  memP50,
		MemoryDurationP95:  memP95,
		MemoryDurationP99:  memP99,
		Goroutines:         runtime.NumGoroutine(),
		MemoryAlloc:        m.MemoryUsage(),
		TotalAlloc:         m.TotalAllocated(),
		ComponentHealth:    m.AllComponentHealth(),
		GoroutineMin:       gMin,
		GoroutineMax:       gMax,
		GoroutineAvg:       gAvg,
	}
}

// =============================================================================
// Duration Histogram Implementation
// =============================================================================

// durationHistogram records durations and computes percentiles.
// Uses a sorted slice approach for simplicity and correctness.
type durationHistogram struct {
	mu    sync.RWMutex
	items []time.Duration
}

func newDurationHistogram() *durationHistogram {
	return &durationHistogram{
		items: make([]time.Duration, 0, 1000),
	}
}

// Record adds a duration to the histogram.
func (h *durationHistogram) Record(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = append(h.items, d)
}

// Percentiles returns p50, p95, and p99 percentiles.
func (h *durationHistogram) Percentiles() (p50, p95, p99 time.Duration) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	n := len(h.items)
	if n == 0 {
		return 0, 0, 0
	}

	// Work on a copy and sort it
	sorted := make([]time.Duration, n)
	copy(sorted, h.items)
	h.mu.RUnlock()
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	h.mu.RLock()

	return sorted[percentileIndex(n, 50)],
		sorted[percentileIndex(n, 95)],
		sorted[percentileIndex(n, 99)]
}

// percentileIndex returns the index for the given percentile rank.
func percentileIndex(n int, p int) int {
	if n == 0 {
		return 0
	}
	idx := (n * p) / 100
	if idx >= n {
		return n - 1
	}
	if idx < 0 {
		return 0
	}
	return idx
}

// Reset clears all recorded durations.
func (h *durationHistogram) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = make([]time.Duration, 0, 1000)
}

// Count returns the number of recorded durations.
func (h *durationHistogram) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.items)
}
