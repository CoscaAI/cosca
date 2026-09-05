package supervisor

import (
	"fmt"
	"sync"
	"time"
)

// InterruptManager handles the STOP/RESUME lifecycle of sub-kernel tasks.
// It creates checkpoints before interruption and manages adjustments.
type InterruptManager struct {
	checkpoints map[string][]Checkpoint // kernelID → checkpoints
	cfg         Config
	mu          sync.Mutex
}

// NewInterruptManager creates a new interrupt manager.
func NewInterruptManager(cfg Config) *InterruptManager {
	return &InterruptManager{
		checkpoints: make(map[string][]Checkpoint),
		cfg:         cfg,
	}
}

// Checkpoint creates a snapshot of the kernel's current state before
// interrupting it. Returns the checkpoint ID.
func (m *InterruptManager) Checkpoint(kernelID KernelID, taskID string, filesModified []string, testsRun, testsPassed int) Checkpoint {
	m.mu.Lock()
	defer m.mu.Unlock()

	ckpt := Checkpoint{
		ID:            fmt.Sprintf("%s-%s-ckpt-%d", kernelID, taskID, time.Now().Unix()),
		KernelID:      kernelID,
		TaskID:        taskID,
		CreatedAt:     time.Now(),
		FilesModified: filesModified,
		TestsRun:      testsRun,
		TestsPassed:   testsPassed,
	}

	key := string(kernelID)
	m.checkpoints[key] = append(m.checkpoints[key], ckpt)

	// Prune old checkpoints if over the limit.
	if len(m.checkpoints[key]) > m.cfg.MaxCheckpoints {
		m.checkpoints[key] = m.checkpoints[key][len(m.checkpoints[key])-m.cfg.MaxCheckpoints:]
	}

	return ckpt
}

// LatestCheckpoint returns the most recent checkpoint for a kernel.
func (m *InterruptManager) LatestCheckpoint(kernelID KernelID) (Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := string(kernelID)
	ckpts, ok := m.checkpoints[key]
	if !ok || len(ckpts) == 0 {
		return Checkpoint{}, fmt.Errorf("supervisor: no checkpoints for kernel %q", kernelID)
	}
	return ckpts[len(ckpts)-1], nil
}

// AllCheckpoints returns all checkpoints for a kernel.
func (m *InterruptManager) AllCheckpoints(kernelID KernelID) []Checkpoint {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := string(kernelID)
	ckpts := m.checkpoints[key]
	result := make([]Checkpoint, len(ckpts))
	copy(result, ckpts)
	return result
}

// BuildStopRequest creates an InterruptRequest to stop a kernel with state saving.
func (m *InterruptManager) BuildStopRequest(kernelID KernelID, taskID, reason string, saveState bool) InterruptRequest {
	return InterruptRequest{
		Action:    ActionStop,
		TaskID:    taskID,
		Reason:    reason,
		SaveState: saveState,
	}
}

// BuildResumeRequest creates an InterruptRequest to resume a kernel after adjustment.
func (m *InterruptManager) BuildResumeRequest(kernelID KernelID, taskID string, adj *Adjustment) InterruptRequest {
	return InterruptRequest{
		Action:     ActionResume,
		TaskID:     taskID,
		Adjustment: adj,
	}
}

// BuildKillRequest creates an InterruptRequest to kill a dead kernel.
func (m *InterruptManager) BuildKillRequest(kernelID KernelID, reason string) InterruptRequest {
	return InterruptRequest{
		Action: ActionKill,
		Reason: reason,
	}
}

// BuildPingRequest creates an InterruptRequest to ping a slow kernel.
func (m *InterruptManager) BuildPingRequest() InterruptRequest {
	return InterruptRequest{
		Action: ActionPing,
	}
}

// IsRecoverable checks if a kernel error can be fixed automatically (Level 0-1).
func (m *InterruptManager) IsRecoverable(resp InterruptResponse) bool {
	// If the kernel stopped cleanly and saved state, it's recoverable.
	if resp.Status == "STOPPED" && resp.CheckpointID != "" {
		return true
	}
	// If tests failed but a checkpoint exists, we can rollback.
	if resp.TestsRun > resp.TestsPassed && resp.CheckpointID != "" {
		return true
	}
	return false
}

// NeedsRollback checks if the situation requires git rollback (Level 2).
func (m *InterruptManager) NeedsRollback(resp InterruptResponse) bool {
	// If multiple tests failed with modified files, rollback is safer.
	if resp.TestsPassed < resp.TestsRun/2 && len(resp.FilesModified) > 0 {
		return true
	}
	return false
}
