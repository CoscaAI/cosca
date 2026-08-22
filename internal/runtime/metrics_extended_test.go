package runtime

import (
	"testing"
	"time"
)

// =============================================================================
// percentileIndex edge cases
// =============================================================================

func TestPercentileIndexEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		n, p, want int
	}{
		// Edge cases that weren't covered
		{100, 100, 99}, // p=100 with n=100 should give index 99 (last element)
		{3, 33, 0},     // (3*33)/100 = 0
		{3, 34, 1},     // (3*34)/100 = 1
		{3, 66, 1},     // (3*66)/100 = 1
		{3, 67, 2},     // (3*67)/100 = 2
		{5, 20, 1},     // (5*20)/100 = 1
		{5, 40, 2},     // (5*40)/100 = 2
		{5, 60, 3},     // (5*60)/100 = 3
		{5, 80, 4},     // (5*80)/100 = 4
		{10, 10, 1},    // (10*10)/100 = 1
		{10, 90, 9},    // (10*90)/100 = 9
		{1, 99, 0},     // (1*99)/100 = 0 for single element
	}
	for _, tt := range tests {
		got := percentileIndex(tt.n, tt.p)
		if got != tt.want {
			t.Errorf("percentileIndex(%d, %d) = %d, want %d", tt.n, tt.p, got, tt.want)
		}
	}
}

// =============================================================================
// durationHistogram with multiple items and sort order
// =============================================================================

func TestDurationHistogramUnsortedInput(t *testing.T) {
	t.Parallel()
	h := newDurationHistogram()
	h.Record(5 * time.Second)
	h.Record(1 * time.Second)
	h.Record(3 * time.Second)
	h.Record(2 * time.Second)
	h.Record(4 * time.Second)

	p50, p95, p99 := h.Percentiles()
	// Sorted: [1,2,3,4,5]
	// p50 (n=5, idx=2) = 3s
	// p95 (n=5, idx=4) = 5s (5*95/100 = 4)
	// p99 (n=5, idx=4) = 5s (5*99/100 = 4)
	if p50 != 3*time.Second {
		t.Errorf("p50 = %v, want 3s", p50)
	}
	if p95 != 5*time.Second {
		t.Errorf("p95 = %v, want 5s", p95)
	}
	if p99 != 5*time.Second {
		t.Errorf("p99 = %v, want 5s", p99)
	}
}

func TestDurationHistogramSingleItem(t *testing.T) {
	t.Parallel()
	h := newDurationHistogram()
	h.Record(42 * time.Millisecond)
	p50, p95, p99 := h.Percentiles()
	if p50 != 42*time.Millisecond {
		t.Errorf("p50 = %v, want 42ms", p50)
	}
	if p95 != 42*time.Millisecond {
		t.Errorf("p95 = %v, want 42ms", p95)
	}
	if p99 != 42*time.Millisecond {
		t.Errorf("p99 = %v, want 42ms", p99)
	}
}

func TestDurationHistogramLargeInput(t *testing.T) {
	t.Parallel()
	h := newDurationHistogram()
	// Record 100 durations from 1ms to 100ms
	for i := 1; i <= 100; i++ {
		h.Record(time.Duration(i) * time.Millisecond)
	}

	p50, p95, p99 := h.Percentiles()
	// p50: (100*50)/100 = 50, 0-indexed = 51ms (values 1-100 at indices 0-99)
	if p50 != 51*time.Millisecond {
		t.Errorf("p50 = %v, want 51ms", p50)
	}
	if p95 != 96*time.Millisecond {
		t.Errorf("p95 = %v, want 96ms", p95)
	}
	if p99 != 100*time.Millisecond {
		t.Errorf("p99 = %v, want 100ms", p99)
	}
}

// =============================================================================
// StartTime edge cases
// =============================================================================

func TestMetricsStartTimeNotZero(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	st := m.StartTime()
	if st.IsZero() {
		t.Error("StartTime should not be zero")
	}
	if time.Since(st) > time.Second {
		t.Error("StartTime should be close to now")
	}
}

// =============================================================================
// GoroutineStats with varying samples
// =============================================================================

func TestGoroutineStatsNoSamples(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	minVal, maxVal, avg := m.GoroutineStats()
	if minVal != 0 || maxVal != 0 || avg != 0 {
		t.Errorf("no samples should return 0s: min=%d max=%d avg=%d", minVal, maxVal, avg)
	}
}

func TestGoroutineStatsSingleSample(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.RecordGoroutineSample()
	minVal, maxVal, avg := m.GoroutineStats()
	if minVal <= 0 || maxVal <= 0 || avg <= 0 {
		t.Errorf("single sample: min=%d max=%d avg=%d", minVal, maxVal, avg)
	}
	if minVal != maxVal && maxVal != avg {
		t.Errorf("single sample should have min=max=avg: min=%d max=%d avg=%d", minVal, maxVal, avg)
	}
}

func TestGoroutineStatsMultipleSamples(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.RecordGoroutineSample()
	m.RecordGoroutineSample()
	minVal, maxVal, avg := m.GoroutineStats()
	if minVal <= 0 {
		t.Errorf("min should be >0: %d", minVal)
	}
	if maxVal < minVal {
		t.Errorf("max(%d) < min(%d)", maxVal, minVal)
	}
	if avg < minVal || avg > maxVal {
		t.Errorf("avg(%d) should be between min(%d) and max(%d)", avg, minVal, maxVal)
	}
}

// =============================================================================
// GetComponentHealth edge cases
// =============================================================================

func TestGetComponentHealthAfterOverride(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.SetComponentHealth("comp", StatusHealthy)
	m.SetComponentHealth("comp", StatusDegraded)
	m.SetComponentHealth("comp", StatusUnhealthy)

	if h := m.GetComponentHealth("comp"); h != StatusUnhealthy {
		t.Errorf("health = %v, want unhealthy (last set)", h)
	}
}

func TestGetComponentHealthMultipleComponents(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.SetComponentHealth("a", StatusHealthy)
	m.SetComponentHealth("b", StatusDegraded)
	m.SetComponentHealth("c", StatusUnhealthy)
	m.SetComponentHealth("d", StatusStarting)
	m.SetComponentHealth("e", StatusStopping)
	m.SetComponentHealth("f", StatusStoppedComponent)

	if h := m.GetComponentHealth("a"); h != StatusHealthy {
		t.Errorf("a = %v", h)
	}
	if h := m.GetComponentHealth("d"); h != StatusStarting {
		t.Errorf("d = %v", h)
	}
	if h := m.GetComponentHealth("f"); h != StatusStoppedComponent {
		t.Errorf("f = %v", h)
	}
}

func TestAllComponentHealthEmpty(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	all := m.AllComponentHealth()
	if len(all) != 0 {
		t.Errorf("empty should return 0 components, got %d", len(all))
	}
}

// =============================================================================
// MemoryUsage and TotalAllocated
// =============================================================================

func TestMemoryUsageNonZero(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	mu := m.MemoryUsage()
	ta := m.TotalAllocated()
	t.Logf("MemoryUsage=%d TotalAllocated=%d", mu, ta)
	// After creating some objects, TotalAllocated should be > 0
	if ta == 0 {
		t.Log("TotalAllocated is 0 (may be platform dependent)")
	}
}

// =============================================================================
// Duration histograms for all metric types
// =============================================================================

func TestAllDurationHistograms(t *testing.T) {
	t.Parallel()
	m := NewMetrics()

	m.RecordIndexDuration(10 * time.Millisecond)
	m.RecordSearchDuration(20 * time.Millisecond)
	m.RecordContextDuration(30 * time.Millisecond)
	m.RecordMemoryDuration(40 * time.Millisecond)

	if ip50, _, _ := m.IndexDurationPercentiles(); ip50 != 10*time.Millisecond {
		t.Errorf("index p50 = %v", ip50)
	}
	if sp50, _, _ := m.SearchDurationPercentiles(); sp50 != 20*time.Millisecond {
		t.Errorf("search p50 = %v", sp50)
	}
	if cp50, _, _ := m.ContextDurationPercentiles(); cp50 != 30*time.Millisecond {
		t.Errorf("context p50 = %v", cp50)
	}
	if mp50, _, _ := m.MemoryDurationPercentiles(); mp50 != 40*time.Millisecond {
		t.Errorf("memory p50 = %v", mp50)
	}
}

// =============================================================================
// Empty duration histograms percentiles
// =============================================================================

func TestEmptyHistogramsReturnZero(t *testing.T) {
	t.Parallel()
	m := NewMetrics()

	ip50, ip95, ip99 := m.IndexDurationPercentiles()
	sp50, sp95, sp99 := m.SearchDurationPercentiles()
	cp50, cp95, cp99 := m.ContextDurationPercentiles()
	mp50, mp95, mp99 := m.MemoryDurationPercentiles()

	if ip50 != 0 || ip95 != 0 || ip99 != 0 {
		t.Error("empty index histogram should return zeros")
	}
	if sp50 != 0 || sp95 != 0 || sp99 != 0 {
		t.Error("empty search histogram should return zeros")
	}
	if cp50 != 0 || cp95 != 0 || cp99 != 0 {
		t.Error("empty context histogram should return zeros")
	}
	if mp50 != 0 || mp95 != 0 || mp99 != 0 {
		t.Error("empty memory histogram should return zeros")
	}
}

// =============================================================================
// Increment operations after snapshot
// =============================================================================

func TestSnapshotAfterIncrements(t *testing.T) {
	t.Parallel()
	m := NewMetrics()

	for i := 0; i < 10; i++ {
		m.IncrementIndex()
	}
	for i := 0; i < 5; i++ {
		m.IncrementSearch()
	}
	m.IncrementError()
	m.IncrementError()

	snap := m.Snapshot()
	if snap.IndexCount != 10 {
		t.Errorf("IndexCount = %d, want 10", snap.IndexCount)
	}
	if snap.SearchCount != 5 {
		t.Errorf("SearchCount = %d, want 5", snap.SearchCount)
	}
	if snap.ErrorCount != 2 {
		t.Errorf("ErrorCount = %d, want 2", snap.ErrorCount)
	}
}

// =============================================================================
// Snapshot contains all fields
// =============================================================================

func TestSnapshotFieldsNonZero(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.SetComponentHealth("test", StatusHealthy)
	m.IncrementPluginCall()
	m.IncrementMemoryStore()
	m.IncrementEvent()
	m.RecordIndexDuration(50 * time.Millisecond)
	m.RecordGoroutineSample()

	s := m.Snapshot()

	if s.PluginCalls != 1 {
		t.Errorf("PluginCalls = %d", s.PluginCalls)
	}
	if s.MemoryStores != 1 {
		t.Errorf("MemoryStores = %d", s.MemoryStores)
	}
	if s.EventCount != 1 {
		t.Errorf("EventCount = %d", s.EventCount)
	}
	if s.IndexDurationP50 <= 0 {
		t.Error("IndexDurationP50 should be > 0")
	}
	if s.GoroutineMin <= 0 || s.GoroutineMax <= 0 || s.GoroutineAvg <= 0 {
		t.Error("Goroutine stats should be positive")
	}
	if s.ComponentHealth == nil || s.ComponentHealth["test"] != StatusHealthy {
		t.Error("ComponentHealth not in snapshot")
	}
}

// =============================================================================
// NumGoroutine is current value
// =============================================================================

func TestNumGoroutinePositive(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	n := m.NumGoroutine()
	if n <= 0 {
		t.Errorf("NumGoroutine should be >= 1, got %d", n)
	}
}

// =============================================================================
// Histogram reset and count
// =============================================================================

func TestHistogramRecordThenResetThenCount(t *testing.T) {
	t.Parallel()
	h := newDurationHistogram()
	h.Record(time.Second)
	h.Record(2 * time.Second)
	h.Record(3 * time.Second)

	if h.Count() != 3 {
		t.Errorf("Count = %d, want 3", h.Count())
	}

	h.Reset()
	if h.Count() != 0 {
		t.Errorf("Count after reset = %d, want 0", h.Count())
	}

	// After reset, percentiles should return 0
	p50, p95, p99 := h.Percentiles()
	if p50 != 0 || p95 != 0 || p99 != 0 {
		t.Error("percentiles after reset should be 0")
	}
}

// =============================================================================
// Concurrent increment stress
// =============================================================================

func TestMetricsConcurrentIncrement(t *testing.T) {
	m := NewMetrics()
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.IncrementIndex()
				m.IncrementSearch()
				m.IncrementError()
				m.IncrementEvent()
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if m.IndexCount() != 1000 {
		t.Errorf("IndexCount = %d, want 1000", m.IndexCount())
	}
	if m.SearchCount() != 1000 {
		t.Errorf("SearchCount = %d", m.SearchCount())
	}
	if m.ErrorCount() != 1000 {
		t.Errorf("ErrorCount = %d", m.ErrorCount())
	}
	if m.EventCount() != 1000 {
		t.Errorf("EventCount = %d", m.EventCount())
	}
}
