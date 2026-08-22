package runtime

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
	if m.StartTime().IsZero() {
		t.Error("StartTime should not be zero")
	}
}

func TestMetricsUptime(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	u := m.Uptime()
	if u < 0 {
		t.Errorf("Uptime should be >= 0, got %v", u)
	}
}

func TestMetricsCountersStartAtZero(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	if m.IndexCount() != 0 {
		t.Errorf("IndexCount = %d", m.IndexCount())
	}
	if m.SearchCount() != 0 {
		t.Errorf("SearchCount = %d", m.SearchCount())
	}
	if m.ContextBuildCount() != 0 {
		t.Errorf("ContextBuildCount = %d", m.ContextBuildCount())
	}
	if m.MemoryStoreCount() != 0 {
		t.Errorf("MemoryStoreCount = %d", m.MemoryStoreCount())
	}
	if m.PluginCallCount() != 0 {
		t.Errorf("PluginCallCount = %d", m.PluginCallCount())
	}
	if m.ErrorCount() != 0 {
		t.Errorf("ErrorCount = %d", m.ErrorCount())
	}
	if m.SyncCount() != 0 {
		t.Errorf("SyncCount = %d", m.SyncCount())
	}
	if m.EventCount() != 0 {
		t.Errorf("EventCount = %d", m.EventCount())
	}
}

func TestMetricsIncrement(t *testing.T) {
	t.Parallel()
	m := NewMetrics()

	m.IncrementIndex()
	if m.IndexCount() != 1 {
		t.Errorf("IndexCount = %d, want 1", m.IndexCount())
	}

	m.IncrementSearch()
	if m.SearchCount() != 1 {
		t.Errorf("SearchCount = %d", m.SearchCount())
	}

	m.IncrementContextBuild()
	m.IncrementContextBuild()
	if m.ContextBuildCount() != 2 {
		t.Errorf("ContextBuildCount = %d, want 2", m.ContextBuildCount())
	}

	m.IncrementMemoryStore()
	m.IncrementPluginCall()
	m.IncrementError()
	m.IncrementSync()
	m.IncrementEvent()

	if m.MemoryStoreCount() != 1 {
		t.Errorf("MemoryStoreCount = %d", m.MemoryStoreCount())
	}
	if m.PluginCallCount() != 1 {
		t.Errorf("PluginCallCount = %d", m.PluginCallCount())
	}
	if m.ErrorCount() != 1 {
		t.Errorf("ErrorCount = %d", m.ErrorCount())
	}
	if m.SyncCount() != 1 {
		t.Errorf("SyncCount = %d", m.SyncCount())
	}
	if m.EventCount() != 1 {
		t.Errorf("EventCount = %d", m.EventCount())
	}
}

func TestMetricsComponentHealth(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.SetComponentHealth("knowledge", StatusHealthy)
	m.SetComponentHealth("cache", StatusDegraded)

	h := m.GetComponentHealth("knowledge")
	if h != StatusHealthy {
		t.Errorf("knowledge health = %v, want %v", h, StatusHealthy)
	}

	h = m.GetComponentHealth("cache")
	if h != StatusDegraded {
		t.Errorf("cache health = %v, want %v", h, StatusDegraded)
	}

	h = m.GetComponentHealth("nonexistent")
	if h != StatusUnknown {
		t.Errorf("nonexistent health = %v, want %v", h, StatusUnknown)
	}
}

func TestAllComponentHealth(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.SetComponentHealth("a", StatusHealthy)
	m.SetComponentHealth("b", StatusUnhealthy)

	all := m.AllComponentHealth()
	if len(all) != 2 {
		t.Errorf("len = %d, want 2", len(all))
	}
	if all["a"] != StatusHealthy {
		t.Errorf("a = %v", all["a"])
	}
	if all["b"] != StatusUnhealthy {
		t.Errorf("b = %v", all["b"])
	}
}

func TestMetricsDurationHistogram(t *testing.T) {
	t.Parallel()
	m := NewMetrics()

	m.RecordIndexDuration(100 * time.Millisecond)
	m.RecordIndexDuration(200 * time.Millisecond)
	m.RecordIndexDuration(300 * time.Millisecond)

	p50, p95, p99 := m.IndexDurationPercentiles()
	if p50 <= 0 {
		t.Errorf("p50 should be > 0, got %v", p50)
	}
	if p95 <= 0 {
		t.Errorf("p95 should be > 0, got %v", p95)
	}
	if p99 <= 0 {
		t.Errorf("p99 should be > 0, got %v", p99)
	}
}

func TestMetricsSearchDuration(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.RecordSearchDuration(50 * time.Millisecond)
	p50, _, _ := m.SearchDurationPercentiles()
	if p50 != 50*time.Millisecond {
		t.Errorf("p50 = %v, want 50ms", p50)
	}
}

func TestMetricsContextDuration(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.RecordContextDuration(150 * time.Millisecond)
	p50, _, _ := m.ContextDurationPercentiles()
	if p50 != 150*time.Millisecond {
		t.Errorf("p50 = %v", p50)
	}
}

func TestMetricsMemoryDuration(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.RecordMemoryDuration(75 * time.Millisecond)
	p50, _, _ := m.MemoryDurationPercentiles()
	if p50 != 75*time.Millisecond {
		t.Errorf("p50 = %v", p50)
	}
}

func TestMetricsNumGoroutine(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	n := m.NumGoroutine()
	if n <= 0 {
		t.Errorf("NumGoroutine should be > 0, got %d", n)
	}
}

func TestMetricsRecordGoroutineSample(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.RecordGoroutineSample()
	m.RecordGoroutineSample()
	minVal, maxVal, avg := m.GoroutineStats()
	if minVal <= 0 {
		t.Errorf("minVal should be > 0, got %d", minVal)
	}
	if maxVal < minVal {
		t.Errorf("maxVal (%d) < minVal (%d)", maxVal, minVal)
	}
	if avg <= 0 {
		t.Errorf("avg should be > 0, got %d", avg)
	}
}

func TestMetricsMemoryUsage(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
	mu := m.MemoryUsage()
	if mu == 0 && m.TotalAllocated() == 0 {
		// On some systems, memory stats may be 0
		t.Log("MemoryUsage is 0 (may be platform dependent)")
	} else {
		t.Logf("MemoryUsage=%d TotalAllocated=%d", mu, m.TotalAllocated())
	}
}

func TestMetricsSnapshot(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	// Let a measurable uptime elapse: Windows time.Now() can jump in ~0.5ms
	// steps, so an immediate snapshot may legitimately read 0s.
	time.Sleep(2 * time.Millisecond)
	m.IncrementIndex()
	m.IncrementSearch()
	m.IncrementError()
	m.RecordIndexDuration(time.Second)
	m.SetComponentHealth("test", StatusHealthy)

	s := m.Snapshot()
	if s.IndexCount != 1 {
		t.Errorf("IndexCount = %d", s.IndexCount)
	}
	if s.SearchCount != 1 {
		t.Errorf("SearchCount = %d", s.SearchCount)
	}
	if s.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d", s.ErrorCount)
	}
	if s.Uptime <= 0 {
		t.Errorf("Uptime should be > 0, got %v", s.Uptime)
	}
	if s.Goroutines <= 0 {
		t.Errorf("Goroutines should be > 0, got %d", s.Goroutines)
	}
}

func TestDurationHistogramPercentiles(t *testing.T) {
	t.Parallel()
	h := newDurationHistogram()
	h.Record(time.Second)
	h.Record(2 * time.Second)
	h.Record(3 * time.Second)
	h.Record(4 * time.Second)
	h.Record(5 * time.Second)

	p50, p95, p99 := h.Percentiles()
	if p50 <= 0 {
		t.Errorf("p50 should be > 0, got %v", p50)
	}
	if p95 <= 0 {
		t.Errorf("p95 should be > 0, got %v", p95)
	}
	if p99 <= 0 {
		t.Errorf("p99 should be > 0, got %v", p99)
	}
}

func TestDurationHistogramEmpty(t *testing.T) {
	t.Parallel()
	h := newDurationHistogram()
	p50, p95, p99 := h.Percentiles()
	if p50 != 0 || p95 != 0 || p99 != 0 {
		t.Errorf("empty histogram should return 0s, got %v, %v, %v", p50, p95, p99)
	}
}

func TestDurationHistogramCount(t *testing.T) {
	t.Parallel()
	h := newDurationHistogram()
	if h.Count() != 0 {
		t.Errorf("Count = %d, want 0", h.Count())
	}
	h.Record(time.Second)
	if h.Count() != 1 {
		t.Errorf("Count = %d, want 1", h.Count())
	}
}

func TestDurationHistogramReset(t *testing.T) {
	t.Parallel()
	h := newDurationHistogram()
	h.Record(time.Second)
	h.Reset()
	if h.Count() != 0 {
		t.Errorf("Count after reset = %d, want 0", h.Count())
	}
}

func TestPercentileIndex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		n, p, want int
	}{
		{100, 50, 50},
		{100, 95, 95},
		{100, 99, 99},
		{10, 50, 5},
		{1, 50, 0},
		{0, 50, 0},
		{100, 0, 0},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := percentileIndex(tt.n, tt.p)
			if got != tt.want {
				t.Errorf("percentileIndex(%d, %d) = %d, want %d", tt.n, tt.p, got, tt.want)
			}
		})
	}
}

func TestMetricsAtomicCounters(t *testing.T) {
	t.Parallel()
	var val atomic.Int64
	val.Add(1)
	if val.Load() != 1 {
		t.Error("atomic int64 add failed")
	}
}
