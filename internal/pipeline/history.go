package pipeline

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// StepEventType classifies events in workflow history.
type StepEventType string

const (
	StepEventStarted   StepEventType = "step_started"
	StepEventCompleted StepEventType = "step_completed"
	StepEventFailed    StepEventType = "step_failed"
	StepEventRetrying  StepEventType = "step_retrying"
	StepEventSkipped   StepEventType = "step_skipped"
	PlanCreated        StepEventType = "plan_created"
	PlanCompleted      StepEventType = "plan_completed"
	PlanFailed         StepEventType = "plan_failed"
)

// StepEvent is an immutable record of a workflow event.
type StepEvent struct {
	ID        string        `json:"id"`
	Type      StepEventType `json:"type"`
	PlanID    string        `json:"plan_id"`
	TaskID    string        `json:"task_id"`
	Agent     string        `json:"agent,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Sequence  int64         `json:"sequence"`
	Input     string        `json:"input,omitempty"`
	Output    string        `json:"output,omitempty"`
	Duration  int64         `json:"duration_ms"`
	Hash      string        `json:"hash"`
}

// WorkflowHistory is an append-only event store backed by a JSONL file.
type WorkflowHistory struct {
	dir string
	mu  sync.Mutex
}

func NewWorkflowHistory(dir string) (*WorkflowHistory, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("history: create directory: %w", err)
	}
	return &WorkflowHistory{dir: dir}, nil
}

// NewEventID generates a new event ID: EVT-YYYYMMDD-XXXXXXXX.
func NewEventID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		n := uint32(time.Now().UnixNano() & 0xFFFFFFFF)
		b[0] = byte(n >> 24)
		b[1] = byte(n >> 16)
		b[2] = byte(n >> 8)
		b[3] = byte(n)
	}
	return fmt.Sprintf("EVT-%s-%02X%02X%02X%02X",
		time.Now().UTC().Format("20060102"), b[0], b[1], b[2], b[3])
}

func (h *WorkflowHistory) filePath(planID string) string {
	return filepath.Join(h.dir, planID+".jsonl")
}

func (h *WorkflowHistory) LastSequence(planID string) (int64, error) {
	events, err := h.Load(planID)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}
	return events[len(events)-1].Sequence, nil
}

// Append records a StepEvent to the plan's JSONL file.
// It sets the event's Hash from the chain and auto-assigns Sequence, Timestamp, ID if needed.
func (h *WorkflowHistory) Append(planID string, event StepEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if event.ID == "" {
		event.ID = NewEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	prevHash := ""
	prevSeq, err := h.lastSequenceLocked(planID)
	if err != nil {
		prevSeq = 0
	}
	if prevSeq > 0 {
		events, loadErr := h.loadLocked(planID)
		if loadErr == nil && len(events) > 0 {
			prevHash = events[len(events)-1].Hash
		}
	}

	event.Sequence = prevSeq + 1

	if event.Hash == "" {
		h := sha256.Sum256([]byte(prevHash + event.ID))
		event.Hash = fmt.Sprintf("%x", h)
	}

	f, err := os.OpenFile(h.filePath(planID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("history: open file: %w", err)
	}
	defer f.Close()

	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("history: marshal event: %w", err)
	}
	line = append(line, '\n')

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("history: write event: %w", err)
	}
	return nil
}

// Load reads all events for a plan sorted by sequence.
func (h *WorkflowHistory) Load(planID string) ([]StepEvent, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.loadLocked(planID)
}

func (h *WorkflowHistory) loadLocked(planID string) ([]StepEvent, error) {
	path := h.filePath(planID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("history: read file: %w", err)
	}

	var events []StepEvent
	lines := splitLines(string(data))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var ev StepEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Sequence < events[j].Sequence
	})
	return events, nil
}

func (h *WorkflowHistory) lastSequenceLocked(planID string) (int64, error) {
	events, err := h.loadLocked(planID)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}
	return events[len(events)-1].Sequence, nil
}

// ListPlans returns all plan IDs that have event history files.
func (h *WorkflowHistory) ListPlans() ([]string, error) {
	entries, err := os.ReadDir(h.dir)
	if err != nil {
		return nil, fmt.Errorf("history: read dir: %w", err)
	}

	var plans []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".jsonl") {
			plans = append(plans, strings.TrimSuffix(name, ".jsonl"))
		}
	}
	return plans, nil
}

// VerifyIntegrity validates the hash chain of a plan's event history.
// Returns the number of verified events and an error if integrity is breached.
func (h *WorkflowHistory) VerifyIntegrity(planID string) (int, error) {
	events, err := h.Load(planID)
	if err != nil {
		return 0, fmt.Errorf("verify: load events: %w", err)
	}
	if len(events) <= 1 {
		return len(events), nil
	}

	for i := 1; i < len(events); i++ {
		prev := events[i-1]
		curr := events[i]

		expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(prev.Hash+curr.ID)))
		if curr.Hash != expectedHash {
			return i, fmt.Errorf("hash chain broken at event %d (%s): expected %s, got %s",
				i, curr.ID, expectedHash[:16]+"...", curr.Hash[:16]+"...")
		}

		if curr.Sequence != prev.Sequence+1 {
			return i, fmt.Errorf("sequence gap at event %d (%s): prev=%d, curr=%d",
				i, curr.ID, prev.Sequence, curr.Sequence)
		}
	}

	return len(events), nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// ReplayPlan reconstructs a Plan's task state from its event history.
func ReplayPlan(plan *Plan, events []StepEvent) *Plan {
	replayed := &Plan{
		ID:               plan.ID,
		Intent:           plan.Intent,
		IntentType:       plan.IntentType,
		Tasks:            make([]*TaskNode, len(plan.Tasks)),
		EstimatedMinutes: plan.EstimatedMinutes,
		RiskLevel:        plan.RiskLevel,
		Objective:        plan.Objective,
		CreatedAt:        plan.CreatedAt,
		Status:           plan.Status,
	}

	taskIdx := make(map[string]int)
	for i, t := range plan.Tasks {
		cp := *t
		cp.Status = TaskPending
		cp.Result = nil
		if cp.DependsOn == nil {
			cp.DependsOn = []string{}
		}
		replayed.Tasks[i] = &cp
		taskIdx[t.ID] = i
	}

	for _, ev := range events {
		idx, ok := taskIdx[ev.TaskID]
		if !ok {
			continue
		}
		task := replayed.Tasks[idx]
		switch ev.Type {
		case StepEventStarted:
			task.Status = TaskRunning
		case StepEventCompleted:
			task.Status = TaskCompleted
			task.Result = &TaskResult{
				Success:    true,
				Output:     ev.Output,
				Agent:      ev.Agent,
				DurationMs: ev.Duration,
			}
		case StepEventFailed:
			task.Status = TaskFailed
			task.Result = &TaskResult{
				Success:    false,
				Error:      ev.Output,
				Agent:      ev.Agent,
				DurationMs: ev.Duration,
			}
		case StepEventSkipped:
			task.Status = TaskSkipped
		case StepEventRetrying:
			task.Status = TaskPending
		}
	}

	for _, ev := range events {
		switch ev.Type {
		case PlanCompleted:
			replayed.Status = "completed"
		case PlanFailed:
			replayed.Status = "failed"
		}
	}

	return replayed
}
