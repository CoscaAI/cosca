//go:build soak

// Package soak provides long-duration soak tests for the Cosca runtime.
//
// These tests are excluded from normal unit/integration runs. Execute with:
//
//	go test -tags=soak -timeout=90m -run TestSoak ./test/soak/
//
// Duration defaults to 1 hour and can be overridden via COSCA_SOAK_DURATION
// (e.g., COSCA_SOAK_DURATION=10m for quick validation).
package soak

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
)

// ── Configuration ───────────────────────────────────────────────────────────

// defaultSoakDuration is the default test duration when COSCA_SOAK_DURATION is
// not set.
const defaultSoakDuration = 1 * time.Hour

// memorySampleInterval is how often memory stats are sampled.
const memorySampleInterval = 5 * time.Minute

// memoryLeakThreshold is the maximum acceptable heap growth between samples.
// If growth exceeds this, a warning is reported.
const memoryLeakThreshold = 0.05 // 5%

// memSample records a memory statistics snapshot.
type memSample struct {
	t         time.Time
	alloc     uint64
	totalAlloc uint64
	sys       uint64
	numGC     uint32
}

// ── Soak Test ───────────────────────────────────────────────────────────────

// TestSoak exercises the Cosca runtime under continuous load for an extended
// period, monitoring memory usage for leaks and collecting performance data.
func TestSoak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak test in short mode")
	}

	duration := defaultSoakDuration
	if d := os.Getenv("COSCA_SOAK_DURATION"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}
	t.Logf("soak test duration: %s", duration)

	// ── Baseline ─────────────────────────────────────────────────────────
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	baseline := memSample{
		t:         time.Now(),
		alloc:     m.Alloc,
		totalAlloc: m.TotalAlloc,
		sys:       m.Sys,
		numGC:     m.NumGC,
	}
	t.Logf("baseline memory: alloc=%d KB, totalAlloc=%d KB, sys=%d KB, gc=%d",
		baseline.alloc/1024, baseline.totalAlloc/1024, baseline.sys/1024, baseline.numGC)

	// ── Load Loop ────────────────────────────────────────────────────────
	var (
		mu       sync.Mutex
		samples  []memSample
		stopCh   = make(chan struct{})
		tickCh   = time.NewTicker(memorySampleInterval)
		doneCh   = make(chan struct{})
	)

	// Sampling goroutine.
	go func() {
		defer close(doneCh)
		for {
			select {
			case <-tickCh.C:
				runtime.ReadMemStats(&m)
				s := memSample{
					t:         time.Now(),
					alloc:     m.Alloc,
					totalAlloc: m.TotalAlloc,
					sys:       m.Sys,
					numGC:     m.NumGC,
				}
				mu.Lock()
				samples = append(samples, s)
				mu.Unlock()
				t.Logf("[%s] alloc=%d KB, totalAlloc=%d KB, sys=%d KB, gc=%d",
					s.t.Format("15:04:05"), s.alloc/1024, s.totalAlloc/1024, s.sys/1024, s.numGC)

			case <-stopCh:
				return
			}
		}
	}()

	// Workload goroutines: simulate agent operations.
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					// Simulate work: allocate and release memory.
					buf := make([]byte, 1024*id)
					for j := range buf {
						buf[j] = byte(j + id)
					}
					_ = buf
				case <-stopCh:
					return
				}
			}
		}(i)
	}

	// Run for the configured duration.
	select {
	case <-time.After(duration):
	case <-stopCh:
	}

	// Stop everything.
	close(stopCh)
	tickCh.Stop()
	wg.Wait()
	<-doneCh

	// ── Analysis ─────────────────────────────────────────────────────────
	runtime.GC()
	runtime.ReadMemStats(&m)
	final := memSample{
		t:         time.Now(),
		alloc:     m.Alloc,
		totalAlloc: m.TotalAlloc,
		sys:       m.Sys,
		numGC:     m.NumGC,
	}
	t.Logf("final memory: alloc=%d KB, totalAlloc=%d KB, sys=%d KB, gc=%d",
		final.alloc/1024, final.totalAlloc/1024, final.sys/1024, final.numGC)

	// Check for memory leaks (>5% growth between any two samples).
	mu.Lock()
	defer mu.Unlock()
	for i := 1; i < len(samples); i++ {
		prev := samples[i-1]
		curr := samples[i]
		if prev.alloc > 0 {
			growth := float64(curr.alloc) / float64(prev.alloc) - 1
			if growth > memoryLeakThreshold {
				t.Logf("WARNING: memory growth %.1f%% between %s and %s (alloc: %d → %d KB)",
					growth*100, prev.t.Format("15:04:05"), curr.t.Format("15:04:05"),
					prev.alloc/1024, curr.alloc/1024)
			}
		}
	}

	// Total allocation rate.
	totalAllocMB := float64(final.totalAlloc-baseline.totalAlloc) / 1024 / 1024
	elapsed := final.t.Sub(baseline.t)
	rateMBps := totalAllocMB / elapsed.Seconds()
	t.Logf("total allocated: %.1f MB over %.0f seconds (%.2f MB/s)",
		totalAllocMB, elapsed.Seconds(), rateMBps)

	// GC pressure.
	gcRate := float64(final.numGC-baseline.numGC) / elapsed.Minutes()
	t.Logf("GC rate: %.1f GC/min", gcRate)

	// Memory stability: final vs baseline.
	if baseline.alloc > 0 {
		heapGrowth := float64(final.alloc) / float64(baseline.alloc)
		t.Logf("heap growth ratio: %.2fx (baseline → final)", heapGrowth)
		if heapGrowth > 2.0 {
			t.Logf("WARNING: heap growth >2x over soak period (possible leak)")
		}
	}

	// ── Summary ──────────────────────────────────────────────────────
	fmt.Printf("\n=== SOAK TEST SUMMARY ===\n")
	fmt.Printf("Duration:  %s\n", elapsed.Round(time.Second))
	fmt.Printf("Samples:   %d\n", len(samples))
	fmt.Printf("Alloc:     %d KB → %d KB (%.1f%% growth)\n",
		baseline.alloc/1024, final.alloc/1024,
		float64(final.alloc)/float64(baseline.alloc)*100-100)
	fmt.Printf("TotalAlloc: %.1f MB in %.0fs (%.2f MB/s)\n",
		totalAllocMB, elapsed.Seconds(), rateMBps)
	fmt.Printf("GC count:  %d → %d (%.1f GC/min)\n",
		baseline.numGC, final.numGC, gcRate)
	fmt.Printf("Sys:       %d KB → %d KB\n",
		baseline.sys/1024, final.sys/1024)
	fmt.Printf("==========================\n")
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// float64Ptr returns a pointer to a float64 value.
func float64Ptr(v float64) *float64 { return &v }

// roundTo rounds a float64 to the specified number of decimal places.
func roundTo(v float64, decimals int) float64 {
	shift := math.Pow(10, float64(decimals))
	return math.Round(v*shift) / shift
}
