
//go:build integration

package integration

import (
	"sync"
	"testing"
	"time"
)

// ── Types ──────────────────────────────────────────────────────────────────

// RuntimeStatus represents the state of a runtime instance.
type RuntimeStatus int

const (
	StatusStopped  RuntimeStatus = iota
	StatusStarting RuntimeStatus = iota
	StatusRunning  RuntimeStatus = iota
	StatusStopping RuntimeStatus = iota
	StatusError    RuntimeStatus = iota
)

func (s RuntimeStatus) String() string {
	switch s {
	case StatusStopped:
		return "stopped"
	case StatusStarting:
		return "starting"
	case StatusRunning:
		return "running"
	case StatusStopping:
		return "stopping"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// SubsystemHealth represents the health of a single subsystem.
type SubsystemHealth struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// HealthReport aggregates health information for all subsystems.
type HealthReport struct {
	Status     string            `json:"status"`
	Uptime     time.Duration     `json:"uptime"`
	Subsystems []SubsystemHealth `json:"subsystems"`
}

// Runtime defines the interface for a runtime instance.
type Runtime interface {
	Start() error
	Stop() error
	Status() RuntimeStatus
	Health() HealthReport
}

// ── SimpleRuntime Implementation ───────────────────────────────────────────

// SimpleRuntime is a minimal runtime implementation for integration testing.
type SimpleRuntime struct {
	mu         sync.RWMutex
	status     RuntimeStatus
	startedAt  time.Time
	subsystems []Subsystem
}

// Subsystem represents a runtime subsystem that can be started and stopped.
type Subsystem struct {
	Name   string
	Health func() SubsystemHealth
}

// NewSimpleRuntime creates a new SimpleRuntime with the given subsystems.
func NewSimpleRuntime(subsystems []Subsystem) *SimpleRuntime {
	return &SimpleRuntime{
		status:     StatusStopped,
		subsystems: subsystems,
	}
}

// Start transitions the runtime from Stopped to Running.
func (r *SimpleRuntime) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status != StatusStopped {
		return nil // already running or transitioning
	}

	r.status = StatusStarting
	r.startedAt = time.Now()
	r.status = StatusRunning
	return nil
}

// Stop transitions the runtime from Running to Stopped.
func (r *SimpleRuntime) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status != StatusRunning {
		return nil
	}

	r.status = StatusStopping
	r.status = StatusStopped
	return nil
}

// Status returns the current runtime status.
func (r *SimpleRuntime) Status() RuntimeStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// Health returns a health report for the runtime and its subsystems.
func (r *SimpleRuntime) Health() HealthReport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	report := HealthReport{
		Status: "healthy",
		Uptime: time.Since(r.startedAt),
	}

	if r.status != StatusRunning {
		report.Status = "unhealthy"
	}

	for _, sub := range r.subsystems {
		h := sub.Health()
		report.Subsystems = append(report.Subsystems, h)
		if h.Status != "healthy" && report.Status == "healthy" {
			report.Status = "degraded"
		}
	}

	return report
}

// ── Tests ──────────────────────────────────────────────────────────────────

// TestRuntime_StartStop exercises the full runtime lifecycle:
// create → start → verify Running → stop → verify Stopped.
func TestRuntime_StartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	subsystems := []Subsystem{
		{
			Name: "knowledge",
			Health: func() SubsystemHealth {
				return SubsystemHealth{Name: "knowledge", Status: "healthy"}
			},
		},
		{
			Name: "memory",
			Health: func() SubsystemHealth {
				return SubsystemHealth{Name: "memory", Status: "healthy"}
			},
		},
	}

	rt := NewSimpleRuntime(subsystems)

	// ── Initial state ─────────────────────────────────────────────────────
	if got := rt.Status(); got != StatusStopped {
		t.Errorf("expected StatusStopped, got %s", got)
	}

	// ── Start ─────────────────────────────────────────────────────────────
	if err := rt.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if got := rt.Status(); got != StatusRunning {
		t.Errorf("expected StatusRunning after Start, got %s", got)
	}

	// ── Health report while running ───────────────────────────────────────
	health := rt.Health()
	if health.Status != "healthy" {
		t.Errorf("expected healthy status, got %q", health.Status)
	}
	if health.Uptime <= 0 {
		t.Error("expected positive uptime")
	}
	if len(health.Subsystems) != 2 {
		t.Errorf("expected 2 subsystems, got %d", len(health.Subsystems))
	}
	for _, sub := range health.Subsystems {
		if sub.Status != "healthy" {
			t.Errorf("subsystem %q expected healthy, got %q", sub.Name, sub.Status)
		}
	}

	// ── Stop ──────────────────────────────────────────────────────────────
	if err := rt.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if got := rt.Status(); got != StatusStopped {
		t.Errorf("expected StatusStopped after Stop, got %s", got)
	}

	// ── Health report after stop ──────────────────────────────────────────
	healthAfter := rt.Health()
	if healthAfter.Status != "unhealthy" {
		t.Errorf("expected unhealthy status after stop, got %q", healthAfter.Status)
	}
}

// TestRuntime_DegradedHealth verifies that subsystem health issues
// are correctly reflected in the health report.
func TestRuntime_DegradedHealth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	subsystems := []Subsystem{
		{
			Name: "healthy-sub",
			Health: func() SubsystemHealth {
				return SubsystemHealth{Name: "healthy-sub", Status: "healthy"}
			},
		},
		{
			Name: "degraded-sub",
			Health: func() SubsystemHealth {
				return SubsystemHealth{Name: "degraded-sub", Status: "degraded", Message: "slow response"}
			},
		},
	}

	rt := NewSimpleRuntime(subsystems)
	rt.Start()
	defer rt.Stop()

	health := rt.Health()
	if health.Status != "degraded" {
		t.Errorf("expected degraded status, got %q", health.Status)
	}

	// Verify subsystem details
	var foundDegraded bool
	for _, sub := range health.Subsystems {
		if sub.Name == "degraded-sub" {
			foundDegraded = true
			if sub.Message != "slow response" {
				t.Errorf("expected message 'slow response', got %q", sub.Message)
			}
		}
	}
	if !foundDegraded {
		t.Error("degraded-sub not found in health report")
	}
}

// TestRuntime_IdempotentStartStop verifies that Start/Stop are idempotent.
func TestRuntime_IdempotentStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	rt := NewSimpleRuntime(nil)

	// Multiple starts should not error
	if err := rt.Start(); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}
	if err := rt.Start(); err != nil {
		t.Fatalf("second Start should be no-op: %v", err)
	}
	if got := rt.Status(); got != StatusRunning {
		t.Errorf("expected StatusRunning, got %s", got)
	}

	// Multiple stops should not error
	if err := rt.Stop(); err != nil {
		t.Fatalf("first Stop failed: %v", err)
	}
	if err := rt.Stop(); err != nil {
		t.Fatalf("second Stop should be no-op: %v", err)
	}
	if got := rt.Status(); got != StatusStopped {
		t.Errorf("expected StatusStopped, got %s", got)
	}
}

// TestRuntime_ConcurrentStartStop verifies concurrent access safety.
func TestRuntime_ConcurrentStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	rt := NewSimpleRuntime(nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.Start()
			rt.Health()
			rt.Stop()
			rt.Status()
		}()
	}
	wg.Wait()

	// Final state should be Stopped
	if got := rt.Status(); got != StatusStopped {
		t.Errorf("expected StatusStopped after concurrent operations, got %s", got)
	}
}
