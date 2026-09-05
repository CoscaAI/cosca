// Package supervisor implements the Cosca Kernel Supervisor — the real-time
// monitor, controller, and corrector of all sub-kernels. It receives heartbeats,
// detects anomalies, interrupts tasks, diagnoses problems, adjusts and resumes,
// and escalates critical situations to the Don.
//
// The Supervisor is the Consigliere's right hand: it watches, controls, and
// corrects — the Don only sees escalations at Level 2+.
package supervisor

import (
	"time"
)

// ── Health States ──────────────────────────────────────────────────────────

// Health represents the operational health of a sub-kernel.
type Health string

const (
	HealthHealthy      Health = "HEALTHY"
	HealthDegraded     Health = "DEGRADED"
	HealthSlow         Health = "SLOW"
	HealthUnresponsive Health = "UNRESPONSIVE"
	HealthDead         Health = "DEAD"
	HealthStopped      Health = "STOPPED"
	HealthInterrupted  Health = "INTERRUPTED"
)

// ── Kernel Identity ────────────────────────────────────────────────────────

// KernelID identifies a sub-kernel in the hierarchy.
type KernelID string

const (
	KernelBackend  KernelID = "kernel-backend"
	KernelFrontend KernelID = "kernel-frontend"
	KernelMobile   KernelID = "kernel-mobile"
	KernelInfra    KernelID = "kernel-infra"
	KernelAI       KernelID = "kernel-ai"
	KernelQuality  KernelID = "kernel-quality"
)

// ── Heartbeat ──────────────────────────────────────────────────────────────

// Heartbeat is the periodic signal a sub-kernel sends to the Supervisor.
type Heartbeat struct {
	KernelID   KernelID  `json:"kernel_id"`
	TaskID     string    `json:"task_id,omitempty"`
	Status     string    `json:"status"`   // "running", "idle", "waiting"
	Progress   int       `json:"progress"` // 0-100 percentage
	Errors     int       `json:"errors"`   // error count since last beat
	MemoryMB   int64     `json:"memory_mb"`
	Goroutines int       `json:"goroutines"`
	LastAction string    `json:"last_action,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// ── Kernel Status ──────────────────────────────────────────────────────────

// KernelStatus is the Supervisor's tracked state for one sub-kernel.
type KernelStatus struct {
	KernelID      KernelID     `json:"kernel_id"`
	Health        Health       `json:"health"`
	CurrentTask   string       `json:"current_task,omitempty"`
	Progress      int          `json:"progress"`
	LastHeartbeat time.Time    `json:"last_heartbeat"`
	Heartbeats    int64        `json:"heartbeats"`
	MissedBeats   int          `json:"missed_beats"`
	Errors        int          `json:"errors"`
	MemoryMB      int64        `json:"memory_mb"`
	Goroutines    int          `json:"goroutines"`
	Checkpoints   []Checkpoint `json:"checkpoints,omitempty"`
}

// ── Checkpoint ─────────────────────────────────────────────────────────────

// Checkpoint is a snapshot of kernel state taken before interruption.
type Checkpoint struct {
	ID            string    `json:"id"`
	KernelID      KernelID  `json:"kernel_id"`
	TaskID        string    `json:"task_id"`
	CreatedAt     time.Time `json:"created_at"`
	FilesModified []string  `json:"files_modified"`
	TestsRun      int       `json:"tests_run"`
	TestsPassed   int       `json:"tests_passed"`
	DiffSnapshot  string    `json:"diff_snapshot,omitempty"`
}

// ── Interrupt Action ───────────────────────────────────────────────────────

// InterruptAction is the command sent by the Supervisor to stop/resume a kernel.
type InterruptAction string

const (
	ActionStop   InterruptAction = "STOP"
	ActionResume InterruptAction = "RESUME"
	ActionPing   InterruptAction = "PING"
	ActionKill   InterruptAction = "KILL"
)

// InterruptRequest is sent from Supervisor to a sub-kernel.
type InterruptRequest struct {
	Action     InterruptAction `json:"action"`
	TaskID     string          `json:"task_id"`
	Reason     string          `json:"reason"`
	SaveState  bool            `json:"save_state"`
	Adjustment *Adjustment     `json:"adjustment,omitempty"`
}

// InterruptResponse is sent from sub-kernel back to Supervisor.
type InterruptResponse struct {
	Status        string   `json:"status"` // "STOPPED", "RESUMED", "ERROR"
	CheckpointID  string   `json:"checkpoint_id,omitempty"`
	FilesModified []string `json:"files_modified,omitempty"`
	TestsRun      int      `json:"tests_run"`
	TestsPassed   int      `json:"tests_passed"`
	Error         string   `json:"error,omitempty"`
}

// ── Adjustment ─────────────────────────────────────────────────────────────

// Adjustment is a correction instruction sent to a stopped kernel.
type Adjustment struct {
	Description string   `json:"description"`
	Commands    []string `json:"commands,omitempty"`
	RetryTests  []int    `json:"retry_tests,omitempty"`
}

// ── Escalation ──────────────────────────────────────────────────────────────

// EscalationLevel defines how critical a situation is.
type EscalationLevel int

const (
	LevelAutoCorrect EscalationLevel = iota // 0: Supervisor fixes automatically
	LevelRestart                            // 1: Restart kernel
	LevelRollback                           // 2: Rollback + notify Don
	LevelConflict                           // 3: Kernel conflict → Don decides
	LevelEmergency                          // 4: Halt all, critical
)

// Escalation is a situation that requires attention.
type Escalation struct {
	Level     EscalationLevel `json:"level"`
	KernelID  KernelID        `json:"kernel_id"`
	TaskID    string          `json:"task_id,omitempty"`
	Summary   string          `json:"summary"`
	Detail    string          `json:"detail"`
	Decision  string          `json:"decision,omitempty"` // what the Supervisor decided
	NeedsDon  bool            `json:"needs_don"`          // true if Don must approve
	Timestamp time.Time       `json:"timestamp"`
}

// ── Dependency Request ─────────────────────────────────────────────────────

// DependencyRequest is a cross-kernel dependency ask.
type DependencyRequest struct {
	ID         string              `json:"id"`
	FromKernel KernelID            `json:"from_kernel"`
	ToKernel   KernelID            `json:"to_kernel"`
	Request    string              `json:"request"` // "contract", "endpoint", "schema"
	Endpoint   string              `json:"endpoint,omitempty"`
	Timeout    time.Duration       `json:"timeout"`
	Status     string              `json:"status"` // "pending", "resolved", "timeout"
	Response   *DependencyResponse `json:"response,omitempty"`
	Retries    int                 `json:"retries"`
	CreatedAt  time.Time           `json:"created_at"`
}

// DependencyResponse is the answer to a DependencyRequest.
type DependencyResponse struct {
	Status   string      `json:"status"` // "ready", "not_found", "building"
	Contract interface{} `json:"contract,omitempty"`
	Message  string      `json:"message,omitempty"`
}

// ── Supervisor Config ──────────────────────────────────────────────────────

// Config defines the Supervisor's operational parameters.
type Config struct {
	HeartbeatInterval    time.Duration `json:"heartbeat_interval"`     // expected (default: 5s)
	HeartbeatTimeout     time.Duration `json:"heartbeat_timeout"`      // slow threshold (default: 10s)
	UnresponsiveTimeout  time.Duration `json:"unresponsive_timeout"`   // interrupt threshold (default: 20s)
	DeadTimeout          time.Duration `json:"dead_timeout"`           // kill threshold (default: 60s)
	DependencyTimeout    time.Duration `json:"dependency_timeout"`     // ask/answer timeout (default: 30s)
	DependencyMaxRetries int           `json:"dependency_max_retries"` // (default: 3)
	CheckpointDir        string        `json:"checkpoint_dir"`         // where checkpoints live
	MaxCheckpoints       int           `json:"max_checkpoints"`        // per kernel (default: 10)
	AutoCorrect          bool          `json:"auto_correct"`           // enable Level 0 auto-fix
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		HeartbeatInterval:    5 * time.Second,
		HeartbeatTimeout:     10 * time.Second,
		UnresponsiveTimeout:  20 * time.Second,
		DeadTimeout:          60 * time.Second,
		DependencyTimeout:    30 * time.Second,
		DependencyMaxRetries: 3,
		CheckpointDir:        ".cosca/checkpoints",
		MaxCheckpoints:       10,
		AutoCorrect:          true,
	}
}

// String returns a human description of the escalation level.
func (l EscalationLevel) String() string {
	switch l {
	case LevelAutoCorrect:
		return "AUTO-CORRECT"
	case LevelRestart:
		return "RESTART"
	case LevelRollback:
		return "ROLLBACK"
	case LevelConflict:
		return "CONFLICT"
	case LevelEmergency:
		return "EMERGENCY"
	default:
		return "UNKNOWN"
	}
}
