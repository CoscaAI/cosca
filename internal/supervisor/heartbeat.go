package supervisor

import (
	"fmt"
	"sync"
	"time"
)

// HeartbeatTracker monitors all sub-kernel heartbeats and determines
// health state transitions.
type HeartbeatTracker struct {
	kernels map[KernelID]*KernelStatus
	cfg     Config
	mu      sync.RWMutex

	// Callbacks
	onHealthChange func(KernelID, Health, Health) // old, new
	onTimeout      func(KernelID)
	onDead         func(KernelID)
}

// NewHeartbeatTracker creates a heartbeat tracker.
func NewHeartbeatTracker(cfg Config) *HeartbeatTracker {
	return &HeartbeatTracker{
		kernels: make(map[KernelID]*KernelStatus),
		cfg:     cfg,
	}
}

// Register adds a kernel to be tracked.
func (t *HeartbeatTracker) Register(id KernelID) *KernelStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	ks := &KernelStatus{
		KernelID: id,
		Health:   HealthHealthy,
	}
	t.kernels[id] = ks
	return ks
}

// ReceiveHeartbeat processes an incoming heartbeat from a sub-kernel.
// Returns the new health state and whether it changed.
func (t *HeartbeatTracker) ReceiveHeartbeat(hb Heartbeat) (Health, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ks, ok := t.kernels[hb.KernelID]
	if !ok {
		ks = &KernelStatus{KernelID: hb.KernelID}
		t.kernels[hb.KernelID] = ks
	}

	previousHealth := ks.Health

	// Update status from heartbeat.
	ks.LastHeartbeat = hb.Timestamp
	ks.Heartbeats++
	ks.MissedBeats = 0
	ks.CurrentTask = hb.TaskID
	ks.Progress = hb.Progress
	ks.Errors = hb.Errors
	ks.MemoryMB = hb.MemoryMB
	ks.Goroutines = hb.Goroutines

	// Determine new health.
	newHealth := t.computeHealth(ks, hb)
	if newHealth != previousHealth {
		ks.Health = newHealth
		if t.onHealthChange != nil {
			t.onHealthChange(hb.KernelID, previousHealth, newHealth)
		}
		return newHealth, true
	}

	ks.Health = newHealth
	return newHealth, false
}

// computeHealth determines the kernel's health from its state.
func (t *HeartbeatTracker) computeHealth(ks *KernelStatus, hb Heartbeat) Health {
	// Errors reported → degraded.
	if hb.Errors > 0 {
		return HealthDegraded
	}

	// High goroutine count relative to normal → degraded.
	if hb.Goroutines > 200 {
		return HealthDegraded
	}

	// High memory usage (relative to 512 MB default) → degraded.
	if hb.MemoryMB > 400 {
		return HealthDegraded
	}

	// Idle is healthy.
	if hb.Status == "idle" || hb.Status == "waiting" {
		return HealthHealthy
	}

	return HealthHealthy
}

// CheckTimeouts scans all kernels for missed heartbeats.
// Should be called periodically (every heartbeat interval).
func (t *HeartbeatTracker) CheckTimeouts(now time.Time) []KernelID {
	t.mu.Lock()
	defer t.mu.Unlock()

	var timedOut []KernelID
	for id, ks := range t.kernels {
		if ks.Health == HealthStopped || ks.Health == HealthInterrupted {
			continue
		}

		elapsed := now.Sub(ks.LastHeartbeat)
		newHealth := ks.Health

		switch {
		case elapsed > t.cfg.DeadTimeout:
			newHealth = HealthDead
			if t.onDead != nil {
				t.onDead(id)
			}
		case elapsed > t.cfg.UnresponsiveTimeout:
			newHealth = HealthUnresponsive
			if t.onTimeout != nil && ks.Health != HealthUnresponsive {
				t.onTimeout(id)
			}
		case elapsed > t.cfg.HeartbeatTimeout:
			newHealth = HealthSlow
			ks.MissedBeats++
		}

		if newHealth != ks.Health {
			old := ks.Health
			ks.Health = newHealth
			if t.onHealthChange != nil {
				t.onHealthChange(id, old, newHealth)
			}
			if newHealth == HealthUnresponsive || newHealth == HealthDead {
				timedOut = append(timedOut, id)
			}
		}
	}
	return timedOut
}

// Status returns the status of a specific kernel.
func (t *HeartbeatTracker) Status(id KernelID) (*KernelStatus, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ks, ok := t.kernels[id]
	if !ok {
		return nil, fmt.Errorf("supervisor: kernel %q not registered", id)
	}
	// Return a copy to avoid races.
	cp := *ks
	return &cp, nil
}

// AllStatuses returns a snapshot of all kernel statuses.
func (t *HeartbeatTracker) AllStatuses() map[KernelID]*KernelStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make(map[KernelID]*KernelStatus, len(t.kernels))
	for id, ks := range t.kernels {
		cp := *ks
		result[id] = &cp
	}
	return result
}

// SetOnHealthChange sets a callback for health transitions.
func (t *HeartbeatTracker) SetOnHealthChange(fn func(KernelID, Health, Health)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onHealthChange = fn
}

// SetOnTimeout sets a callback for unresponsive kernels.
func (t *HeartbeatTracker) SetOnTimeout(fn func(KernelID)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onTimeout = fn
}

// SetOnDead sets a callback for dead kernels.
func (t *HeartbeatTracker) SetOnDead(fn func(KernelID)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onDead = fn
}

// ActiveKernels returns the count of kernels that are not dead/stopped.
func (t *HeartbeatTracker) ActiveKernels() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	count := 0
	for _, ks := range t.kernels {
		switch ks.Health {
		case HealthDead, HealthStopped:
			continue
		default:
			count++
		}
	}
	return count
}

// Reset resets a kernel's missed beats and health after recovery.
func (t *HeartbeatTracker) Reset(id KernelID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ks, ok := t.kernels[id]; ok {
		ks.MissedBeats = 0
		ks.Health = HealthHealthy
	}
}
