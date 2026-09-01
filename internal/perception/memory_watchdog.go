package perception

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// memoryWatchdog is the process-memory safety net for the Perception Loop. It
// samples the live Go heap on a cadence, compares it to a settling baseline, and
// flags (or, when configured, stops) growth that looks like a leak. This is a
// cheap protection — a few runtime.ReadMemStats calls per interval — against a
// long-running loop accumulating memory (e.g. a STT segment that never resets).
//
// It is deliberately conservative: a single transient spike is not a leak, only
// MaxConsecutive consecutive samples above Ratio*baseline are.
type memoryWatchdog struct {
	cfg          MemoryWatchdogConfig
	log          zerolog.Logger
	mu           sync.Mutex
	baselineSet  bool
	baselineHeap uint64
	consecutive  int
	forceStop    bool
}

// newMemoryWatchdog builds the watchdog from config.
func newMemoryWatchdog(cfg MemoryWatchdogConfig) *memoryWatchdog {
	if cfg.Interval <= 0 {
		cfg.Interval = time.Duration(0) // sampled on the loop cadence
	}
	return &memoryWatchdog{cfg: cfg, log: zerolog.Nop()}
}

// withLogger attaches a logger.
func (w *memoryWatchdog) withLogger(l zerolog.Logger) *memoryWatchdog {
	if w != nil {
		w.log = l
	}
	return w
}

// sample reads the current heap and, once a baseline is settled, detects
// sustained growth. It returns true when the loop should stop (either
// StopOnLeak or the force-stop threshold was hit).
//
// The first sample establishes the baseline (the process is still warming up);
// after that, any heap above Ratio*baseline for MaxConsecutive consecutive
// samples is flagged.
func (w *memoryWatchdog) sample() {
	if w == nil || !w.cfg.Enabled {
		return
	}
	heapAlloc, _, _ := memInfo()

	w.mu.Lock()
	defer w.mu.Unlock()

	// Establish the baseline on the first sample. It settles after a couple of
	// intervals to avoid catching the allocation spike of a cold start.
	if !w.baselineSet {
		w.baselineHeap = heapAlloc
		w.baselineSet = true
		return
	}

	ratio := w.cfg.Ratio
	if ratio <= 0 {
		ratio = 2.0
	}
	threshold := uint64(float64(w.baselineHeap) * ratio)
	if heapAlloc <= threshold {
		w.consecutive = 0
		return
	}

	w.consecutive++
	maxConsec := w.cfg.MaxConsecutive
	if maxConsec <= 0 {
		maxConsec = 3
	}
	if w.consecutive >= maxConsec {
		msg := "perception memory watchdog: sustained heap growth detected (possible leak)"
		w.log.Warn().
			Uint64("heap_alloc", heapAlloc).
			Uint64("baseline", w.baselineHeap).
			Float64("ratio", ratio).
			Int("consecutive", w.consecutive).
			Msg(msg)
		_ = msg
	}

	// Force-stop if configured, after the (possibly longer) MaxStop threshold.
	maxStop := w.cfg.MaxStop
	if maxStop > 0 && w.consecutive >= maxStop {
		w.forceStop = true
	}
}

// stopRequested reports whether the loop should stop (StopOnLeak or force-stop).
func (w *memoryWatchdog) stopRequested() bool {
	if w == nil || !w.cfg.Enabled {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cfg.StopOnLeak && w.forceStop {
		return true
	}
	return w.forceStop
}
