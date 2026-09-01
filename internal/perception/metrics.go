package perception

import (
	"runtime"
	rsmetrics "runtime/metrics"
	"sort"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Performance telemetry (the professor's benchmark)
// ──────────────────────────────────────────────────────────────

const (
	// metricsWindow is the rolling measurement window over which FPS / latency /
	// CPU are averaged. It resets periodically so a long-running loop reports
	// *recent* numbers rather than a lifetime average (a benchmark sees fresh
	// throughput every window).
	metricsWindow = 5 * time.Second
	// metricsRingCap bounds the number of recent tick latencies kept for the
	// average / p95 computation. Bigger = smoother but more memory; 256 is
	// plenty for a benchmark window.
	metricsRingCap = 256
)

// Metrics is the performance telemetry for the Perception Loop. It is embedded
// on every State (and therefore exposed on /v1/perception/state and the SSE
// stream) so the professor can measure the real throughput of the native Go
// ONNX pipeline (CLIP + GroundingDINO + Depth + SAM) without Python.
//
// All values are best-effort over the current measurement window (see
// metricsWindow): FPS/CPU reflect the recent past, not the process lifetime.
type Metrics struct {
	// Ticks is the number of capture→vision ticks completed in the window.
	Ticks uint64 `json:"ticks"`
	// DroppedFrames is the number of frames dropped instead of queued
	// (backpressure slots skipped while a previous tick is still running, plus
	// safety-timeout cancels). The loop never accumulates a queue.
	DroppedFrames uint64 `json:"dropped_frames"`
	// CaptureFailures is the number of capture failures in the window.
	CaptureFailures uint64 `json:"capture_failures"`
	// VisionFailures is the number of vision-pipeline failures in the window
	// (excluding safety-timeout drops).
	VisionFailures uint64 `json:"vision_failures"`
	// SkippedFrames is the number of frames the change-detection gate skipped
	// (no meaningful screen change → vision not run). The efficiency signal:
	// a long static screen yields many skipped frames and ~0 inferences while
	// COSCA keeps "perceiving".
	SkippedFrames uint64 `json:"skipped_frames"`
	// ChangedFrames is the number of frames the change-detection gate passed
	// (screen changed → vision ran). When the gate is disabled this stays 0 and
	// every tick is processed (previous behaviour).
	ChangedFrames uint64 `json:"changed_frames"`
	// LastChangeAt is when the screen last actually changed (a frame ran vision).
	// Persistent across measurement windows (the "time since last change" is a
	// feature, not a rate).
	LastChangeAt time.Time `json:"last_change_at,omitempty"`
	// CaptureFPS is the achieved capture cadence (ticks/sec) over the window.
	CaptureFPS float64 `json:"capture_fps"`
	// VisionFPS is the inference throughput the vision pipeline sustains
	// (1 / mean tick latency in seconds).
	VisionFPS float64 `json:"vision_fps"`
	// LatencyAvgMS is the mean tick latency (capture → vision → state) in ms.
	LatencyAvgMS float64 `json:"latency_avg_ms"`
	// LatencyP95MS is the 95th-percentile tick latency in ms.
	LatencyP95MS float64 `json:"latency_p95_ms"`
	// CPUPercent is a best-effort process CPU% over the window (from Go's own
	// CPU accounting — it does not add the cost of external cgo/OS threads).
	CPUPercent float64 `json:"cpu_pct"`
	// MemBytes is the current Go heap memory in use (best-effort).
	MemBytes uint64 `json:"mem_bytes"`
	// MemMB is MemBytes expressed in MiB.
	MemMB float64 `json:"mem_mb"`
	// NumGoroutine is the current number of goroutines — a leak indicator: a
	// steady climb over time points to goroutines never exiting (e.g. a bus
	// subscriber / capture pump / TTS worker left behind).
	NumGoroutine int `json:"num_goroutine"`
	// HeapObjects is the count of live heap objects (another leak/retention
	// indicator that rises steadily when buffers/observations accumulate).
	HeapObjects uint64 `json:"heap_objects"`
	// ClipMS / GroundingMS / DepthMS / SAMMS are the per-model latencies (ms)
	// observed on the most recent frame. 0 is reported when the model is not in
	// the pipeline or degraded (absent/onnxruntime-unavailable).
	ClipMS      float64 `json:"clip_ms,omitempty"`
	GroundingMS float64 `json:"grounding_ms,omitempty"`
	DepthMS     float64 `json:"depth_ms,omitempty"`
	SAMMS       float64 `json:"sam_ms,omitempty"`
	// WindowSeconds is the length of the measurement window used for this snapshot.
	WindowSeconds float64 `json:"window_seconds"`
}

// metricTracker accumulates performance telemetry. It is guarded by the owning
// Service's mu: the loop writes it from a single goroutine (plus the drop
// counter from the ticker path) and readers snapshot it.
type metricTracker struct {
	windowStart time.Time
	windowCPU   float64

	ring    [metricsRingCap]time.Duration
	ringPos int
	ringN   int

	ticks       uint64
	dropped     uint64
	capFails    uint64
	visFails    uint64
	skipped     uint64
	changed     uint64
	// lastChange is persistent: it is NOT reset when the measurement window
	// rolls, so "time since last real change" stays meaningful across windows.
	lastChange time.Time

	clipMS      float64
	groundingMS float64
	depthMS     float64
	samMS       float64
}

func newMetricTracker() *metricTracker {
	return &metricTracker{windowStart: time.Now()}
}

// record pushes a completed tick's latency and per-model timings into the
// window. The caller must hold the owning Service's mu.
func (m *metricTracker) record(elapsed time.Duration, clip, grounding, depth, sam float64) {
	m.ticks++
	m.ring[m.ringPos] = elapsed
	m.ringPos = (m.ringPos + 1) % metricsRingCap
	if m.ringN < metricsRingCap {
		m.ringN++
	}
	m.clipMS = clip
	m.groundingMS = grounding
	m.depthMS = depth
	m.samMS = sam
}

func (m *metricTracker) recordDrop()       { m.dropped++ }
func (m *metricTracker) recordCapFail()    { m.capFails++ }
func (m *metricTracker) recordVisFail()    { m.visFails++ }
func (m *metricTracker) recordSkipped()    { m.skipped++ }
func (m *metricTracker) recordChanged()    { m.changed++ }

// markChanged records the timestamp of a real change and is persistent across
// window resets.
func (m *metricTracker) markChanged(t time.Time) {
	if !t.IsZero() {
		m.lastChange = t
	}
}

// snapshot computes a Metrics view and lazily resets the window once it has
// elapsed, so FPS/CPU reflect the recent past. The caller must hold the owning
// Service's mu (it mutates the tracker on reset).
func (m *metricTracker) snapshot() *Metrics {
	now := time.Now()
	if now.Sub(m.windowStart) >= metricsWindow {
		m.windowStart = now
		m.windowCPU = processCPUSeconds()
		m.ticks = 0
		m.dropped = 0
		m.capFails = 0
		m.visFails = 0
		m.skipped = 0
		m.changed = 0
		m.ringN = 0
		m.ringPos = 0
		// m.lastChange is deliberately kept: it is a persistent timestamp, not
		// a per-window rate.
	}

	elapsed := now.Sub(m.windowStart)
	secs := elapsed.Seconds()
	if secs <= 0 {
		secs = 1e-9
	}

	latencies := make([]time.Duration, m.ringN)
	copy(latencies, m.ring[:m.ringN])
	avg := meanDuration(latencies)
	p95 := percentileDuration(latencies, 0.95)

	// CPU: delta of Go's own CPU accounting over the window, as a %.
	cpuSec := processCPUSeconds() - m.windowCPU
	if cpuSec < 0 {
		cpuSec = 0
	}
	cpuPct := (cpuSec / secs) * 100
	if cpuPct < 0 {
		cpuPct = 0
	}

	heapAlloc, numGoroutine, heapObjCount := memInfo()

	visionFPS := 0.0
	if avg > 0 {
		visionFPS = 1.0 / avg.Seconds()
	}

	return &Metrics{
		Ticks:           m.ticks,
		DroppedFrames:   m.dropped,
		CaptureFailures: m.capFails,
		VisionFailures:  m.visFails,
		SkippedFrames:   m.skipped,
		ChangedFrames:   m.changed,
		LastChangeAt:    m.lastChange,
		CaptureFPS:      float64(m.ticks) / secs,
		VisionFPS:       visionFPS,
		LatencyAvgMS:    toMS(avg),
		LatencyP95MS:    toMS(p95),
		CPUPercent:      cpuPct,
		MemBytes:        heapAlloc,
		MemMB:           float64(heapAlloc) / (1024 * 1024),
		NumGoroutine:    numGoroutine,
		HeapObjects:     heapObjCount,
		ClipMS:          m.clipMS,
		GroundingMS:     m.groundingMS,
		DepthMS:         m.depthMS,
		SAMMS:           m.samMS,
		WindowSeconds:   secs,
	}
}

// ──────────────────────────────────────────────────────────────
// Stat helpers
// ──────────────────────────────────────────────────────────────

func meanDuration(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var total int64
	for _, d := range ds {
		total += int64(d)
	}
	return time.Duration(total / int64(len(ds)))
}

func percentileDuration(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	if len(ds) == 1 {
		return ds[0]
	}
	sorted := make([]time.Duration, len(ds))
	copy(sorted, ds)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(p * float64(len(sorted)-1))
	return sorted[idx]
}

func toMS(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// ──────────────────────────────────────────────────────────────
// Best-effort process telemetry (portable; no external deps)
// ──────────────────────────────────────────────────────────────

// processCPUSeconds returns the CPU seconds already consumed by this process,
// per Go's runtime `/cpu/classes/total:cpu-seconds` metric. It is portable and
// free of external dependencies (best-effort; does not include cgo/OS threads).
func processCPUSeconds() float64 {
	const name = "/cpu/classes/total:cpu-seconds"
	samples := []rsmetrics.Sample{{Name: name}}
	rsmetrics.Read(samples)
	if len(samples) > 0 && samples[0].Value.Kind() == rsmetrics.KindFloat64 {
		return samples[0].Value.Float64()
	}
	return 0
}

// memInfo returns the current Go heap bytes, the live goroutine count and the
// live heap object count — the three leak/retention indicators the stress test
// (and the perception metrics) use to tell a stable pipeline from one that is
// accumulating.
func memInfo() (heapAlloc uint64, numGoroutine int, heapObjects uint64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc, runtime.NumGoroutine(), m.HeapObjects
}
