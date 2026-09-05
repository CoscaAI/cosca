// Package compute provides the Cosca Compute Fabric — a multi-core adaptive
// execution engine that detects hardware capacity and scales worker pools
// automatically without configuration.
package compute

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// Hardware Probe
// =============================================================================

// HardwareProbe detects and monitors system hardware capacity in real time.
// It runs background probes at a configurable interval and exposes the latest
// snapshot for the AdaptiveScheduler to make scaling decisions.
type HardwareProbe struct {
	mu       sync.RWMutex
	snapshot HardwareSnapshot
	interval time.Duration
	stopCh   chan struct{}
	running  bool
}

// HardwareSnapshot is a point-in-time reading of system resources.
type HardwareSnapshot struct {
	// CPU
	LogicalCores int     // runtime.NumCPU()
	Load1        float64 // /proc/loadavg 1-min
	Load5        float64 // /proc/loadavg 5-min
	Load15       float64 // /proc/loadavg 15-min
	CPUUsage     float64 // Calculated from /proc/stat (0.0-100.0)

	// Memory (bytes)
	TotalRAM     uint64
	AvailableRAM uint64
	UsedRAM      uint64
	MemoryUsage  float64 // 0.0-100.0

	// GPU (read-only capacity probe, best-effort)
	GPU GPUInfo `json:"gpu,omitempty"`

	// CPU (read-only discovery probe, best-effort — vendor, cores, features)
	CPU CPUInfo `json:"cpu,omitempty"`

	// Memory (read-only discovery probe, best-effort — physical/visible/
	// available/effective, page size, swap, limitations)
	Memory MemoryInfo `json:"memory,omitempty"`

	// Topology (read-only discovery probe, best-effort — NUMA nodes, CPU→node,
	// packages, cores per package, per-CPU topology; Fase 3 do Hardware Brain)
	Topology TopologyInfo `json:"topology,omitempty"`

	// Environment (read-only discovery probe, best-effort — bare-metal/VM/
	// container/jail, sysfs, cpuset efetivo; Fase 4 do Hardware Brain)
	Environment EnvironmentInfo `json:"environment,omitempty"`

	// Storage (read-only discovery probe, best-effort — mounts, capacidade,
	// meio físico NVMe/SSD/HDD, device nodes; Fase 6 do Hardware Brain)
	Storage StorageInfo `json:"storage,omitempty"`

	// Timestamp
	ProbedAt time.Time
}

// NewHardwareProbe creates a probe that samples every `interval`.
func NewHardwareProbe(interval time.Duration) *HardwareProbe {
	return &HardwareProbe{
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins background probing. Safe to call multiple times.
func (hp *HardwareProbe) Start() {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	if hp.running {
		return
	}
	hp.running = true
	hp.stopCh = make(chan struct{})
	// Take an immediate sample.
	hp.snapshot = hp.probe()
	ch := hp.stopCh // Capture under lock to avoid race with Stop.
	go hp.loop(ch)
}

// Stop halts background probing. Safe to call multiple times.
func (hp *HardwareProbe) Stop() {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	if !hp.running {
		return
	}
	hp.running = false
	close(hp.stopCh)
}

// Snapshot returns the latest hardware reading. Never nil after Start().
func (hp *HardwareProbe) Snapshot() HardwareSnapshot {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	return hp.snapshot
}

// RecommendWorkers returns the ideal number of workers for a pool
// based on a percentage of logical cores, clamped to [min, max].
func (hp *HardwareProbe) RecommendWorkers(pct float64, min, max int) int {
	hp.mu.RLock()
	cores := hp.snapshot.LogicalCores
	hp.mu.RUnlock()

	n := int(float64(cores) * pct)
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

// IsOverloaded returns true when system load exceeds 80% of available cores.
func (hp *HardwareProbe) IsOverloaded() bool {
	hp.mu.RLock()
	load := hp.snapshot.Load1
	cores := hp.snapshot.LogicalCores
	hp.mu.RUnlock()

	if cores == 0 {
		return false
	}
	return load > float64(cores)*0.8
}

// IsMemoryPressured returns true when less than 20% of RAM is available.
func (hp *HardwareProbe) IsMemoryPressured() bool {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	if hp.snapshot.TotalRAM == 0 {
		return false
	}
	return hp.snapshot.MemoryUsage > 80.0
}

// loop runs the background probe goroutine.
func (hp *HardwareProbe) loop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(hp.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			snap := hp.probe()
			hp.mu.Lock()
			hp.snapshot = snap
			hp.mu.Unlock()
		case <-stopCh:
			return
		}
	}
}

// probe performs a single hardware reading.
// All parsing functions are best-effort — if /proc is unavailable (macOS, Windows),
// we fall back to Go runtime values.
func (hp *HardwareProbe) probe() HardwareSnapshot {
	snap := HardwareSnapshot{
		LogicalCores: runtime.NumCPU(),
		ProbedAt:     time.Now(),
	}

	// CPU load from /proc/loadavg
	snap.Load1, snap.Load5, snap.Load15 = readLoadAvg()

	// Memory from /proc/meminfo
	snap.TotalRAM, snap.AvailableRAM = readMemInfo()
	if snap.TotalRAM > 0 {
		snap.UsedRAM = snap.TotalRAM - snap.AvailableRAM
		snap.MemoryUsage = float64(snap.UsedRAM) / float64(snap.TotalRAM) * 100.0
	}

	// GPU capacity (read-only, best-effort — never fatal).
	snap.GPU = ProbeGPU()

	// CPU discovery (read-only, best-effort — never fatal).
	snap.CPU = ProbeCPU()

	// Memory discovery (read-only, best-effort — never fatal).
	snap.Memory = ProbeMemory()

	// Topology discovery (read-only, best-effort — never fatal).
	snap.Topology = ProbeTopology()

	// Environment discovery (read-only, best-effort — never fatal).
	snap.Environment = ProbeEnvironment()

	// Storage discovery (read-only, best-effort — never fatal).
	snap.Storage = ProbeStorage()

	return snap
}

// ProbeHardware returns a one-shot hardware snapshot combining CPU, RAM and
// GPU capacity in a single call. Used by the capability profile.
func ProbeHardware() HardwareSnapshot {
	hp := &HardwareProbe{}
	return hp.probe()
}

// =============================================================================
// Linux /proc readers (no cgo, no external dependencies)
// =============================================================================

// readFileLines reads a file and returns non-empty trimmed lines.
func readFileLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil // coverage: accept — Error path unreachable on Linux (/proc always readable).
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// readLoadAvg parses /proc/loadavg returning 1, 5, 15-minute load averages.
// coverage: accept — /proc/loadavg always has content with 3+ fields on Linux.
func readLoadAvg() (float64, float64, float64) {
	lines := readFileLines("/proc/loadavg")
	if len(lines) == 0 {
		return 0, 0, 0
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 3 {
		return 0, 0, 0
	}
	l1, _ := strconv.ParseFloat(fields[0], 64)
	l5, _ := strconv.ParseFloat(fields[1], 64)
	l15, _ := strconv.ParseFloat(fields[2], 64)
	return l1, l5, l15
}

// readMemInfo parses /proc/meminfo returning total and available RAM in bytes.
// coverage: accept — /proc/meminfo always has well-formed lines on Linux.
func readMemInfo() (total, available uint64) {
	lines := readFileLines("/proc/meminfo")
	var memTotal, memAvailable uint64
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			memTotal = val * 1024 // kB → bytes
		case "MemAvailable:":
			memAvailable = val * 1024
		}
	}
	return memTotal, memAvailable
}
