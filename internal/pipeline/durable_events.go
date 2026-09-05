package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Durable event types. These are the immutable event kinds appended to a run's
// event log; replaying them reconstructs the full execution state.
const (
	DurablePlanCreated   = "plan_created"
	DurableStepStarted   = "step_started"
	DurableStepCompleted = "step_completed"
	DurableStepFailed    = "step_failed"
	DurableRunCompleted  = "run_completed"
	DurableRunFailed     = "run_failed"
)

// genesisHash is the hash prefix of the first event in a run — nothing precedes
// it, so the chain starts from an all-zero digest.
const genesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// DurableEvent is an immutable pipeline event in a run's append-only log.
type DurableEvent struct {
	RunID     string      `json:"run_id"`
	Seq       int         `json:"seq"` // monotonic per run
	Type      string      `json:"type"`
	TaskID    string      `json:"task_id,omitempty"`
	Result    *TaskResult `json:"result,omitempty"`
	TraceID   string      `json:"trace_id,omitempty"` // optional universal Trace ID (flight recorder linkage)
	Hash      string      `json:"hash"`
	Timestamp int64       `json:"ts"`
}

// DurableEventLog is an append-only JSONL event log with hash chaining.
// File per run: <dir>/<runID>.jsonl (0600).
type DurableEventLog struct {
	dir string
	mu  sync.Mutex
}

// NewDurableEventLog creates (and prepares) the event log directory.
func NewDurableEventLog(dir string) (*DurableEventLog, error) {
	if dir == "" {
		return nil, fmt.Errorf("durable: empty event log directory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("durable: create directory: %w", err)
	}
	return &DurableEventLog{dir: dir}, nil
}

func (l *DurableEventLog) filePath(runID string) string {
	return filepath.Join(l.dir, runID+".jsonl")
}

// Append records an event for runID and returns the persisted event.
func (l *DurableEventLog) Append(runID, eventType, taskID string, result *TaskResult) (*DurableEvent, error) {
	return l.AppendWithTrace(runID, eventType, taskID, "", result)
}

// AppendWithTrace records an event carrying an optional universal TraceID so
// the flight recorder can link the durable event to the desktop execution tree.
func (l *DurableEventLog) AppendWithTrace(runID, eventType, taskID, traceID string, result *TaskResult) (*DurableEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if runID == "" || eventType == "" {
		return nil, fmt.Errorf("durable: run id and event type are required")
	}

	prevHash := genesisHash
	prevSeq := 0
	events, err := l.loadLocked(runID)
	if err != nil {
		return nil, err
	}
	if len(events) > 0 {
		prevHash = events[len(events)-1].Hash
		prevSeq = events[len(events)-1].Seq
	}

	ev := &DurableEvent{
		RunID:     runID,
		Seq:       prevSeq + 1,
		Type:      eventType,
		TaskID:    taskID,
		Result:    result,
		TraceID:   traceID,
		Timestamp: time.Now().UnixMilli(),
	}

	// Hash chain: sha256(prevHash + canonical JSON of this event without Hash).
	payload, err := json.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("durable: marshal event: %w", err)
	}
	sum := sha256.Sum256([]byte(prevHash + string(payload)))
	ev.Hash = hex.EncodeToString(sum[:])

	line, err := json.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("durable: marshal event: %w", err)
	}
	line = append(line, '\n')

	f, err := os.OpenFile(l.filePath(runID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("durable: open log: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return nil, fmt.Errorf("durable: write event: %w", err)
	}
	return ev, nil
}

func (l *DurableEventLog) loadLocked(runID string) ([]DurableEvent, error) {
	data, err := os.ReadFile(l.filePath(runID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("durable: read log: %w", err)
	}
	var events []DurableEvent
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var ev DurableEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return nil, fmt.Errorf("durable: corrupt event line: %w", err)
		}
		events = append(events, ev)
	}
	return events, nil
}

// LoadRun returns a run's events in append order.
func (l *DurableEventLog) LoadRun(runID string) ([]DurableEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loadLocked(runID)
}

// LastCompleted returns the most recently completed step for a run.
// A falha de leitura do log NÃO é "nenhum step completado": retornar
// ok=false nesse caso levaria um chamador a re-executar steps já feitos.
// Por isso o erro é propagado (falso "ok=false" seria erro de contrato).
func (l *DurableEventLog) LastCompleted(runID string) (taskID string, result *TaskResult, ok bool, err error) {
	events, err := l.LoadRun(runID)
	if err != nil {
		return "", nil, false, err
	}
	for _, ev := range events {
		if ev.Type == DurableStepCompleted && ev.Result != nil && ev.Result.Success {
			taskID = ev.TaskID
			result = ev.Result
			ok = true
		}
	}
	return taskID, result, ok, nil
}

// ListRuns returns the run IDs that have an event log file.
func (l *DurableEventLog) ListRuns() ([]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, fmt.Errorf("durable: read dir: %w", err)
	}
	var runs []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		runs = append(runs, strings.TrimSuffix(e.Name(), ".jsonl"))
	}
	return runs, nil
}

// VerifyChain replays a run's events and validates the hash chain and the
// monotonic sequence. Returns false with a descriptive error on any breach.
func (l *DurableEventLog) VerifyChain(runID string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	events, err := l.loadLocked(runID)
	if err != nil {
		return false, err
	}

	prevHash := genesisHash
	for i, ev := range events {
		if ev.Seq != i+1 {
			return false, fmt.Errorf("durable: sequence gap at event %d (got seq %d)", i, ev.Seq)
		}
		noHash := ev
		noHash.Hash = ""
		payload, err := json.Marshal(noHash)
		if err != nil {
			return false, fmt.Errorf("durable: marshal for verify: %w", err)
		}
		sum := sha256.Sum256([]byte(prevHash + string(payload)))
		if ev.Hash != hex.EncodeToString(sum[:]) {
			return false, fmt.Errorf("durable: hash chain broken at event %d (%s)", i, ev.Type)
		}
		prevHash = ev.Hash
	}
	return true, nil
}

// IsStale reports whether a run has events but never reached a terminal event
// (run_completed / run_failed) — i.e. it was interrupted mid-workflow.
func (l *DurableEventLog) IsStale(runID string) (bool, error) {
	events, err := l.LoadRun(runID)
	if err != nil {
		return false, err
	}
	if len(events) == 0 {
		return false, nil
	}
	last := events[len(events)-1]
	return !IsTerminalEvent(last.Type), nil
}

// IsTerminalEvent reports whether an event type terminates a run.
func IsTerminalEvent(eventType string) bool {
	return eventType == DurableRunCompleted || eventType == DurableRunFailed
}

// hasEvent reports whether the run already contains an event of the given type.
func (l *DurableEventLog) hasEvent(runID, eventType string) bool {
	events, err := l.LoadRun(runID)
	if err != nil {
		return false
	}
	for _, ev := range events {
		if ev.Type == eventType {
			return true
		}
	}
	return false
}

// ReplayDurable reconstructs a plan's task state from its durable event log.
// Only success results are replayed (failed steps are never cached); a step
// that only has a step_started event is left pending so execution continues
// from the first incomplete task.
func ReplayDurable(plan *Plan, events []DurableEvent) *Plan {
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
	for i, t := range plan.Tasks {
		cp := *t
		cp.Status = TaskPending
		cp.Result = nil
		if cp.DependsOn == nil {
			cp.DependsOn = []string{}
		}
		replayed.Tasks[i] = &cp
	}

	taskIdx := make(map[string]int, len(plan.Tasks))
	for i, t := range plan.Tasks {
		taskIdx[t.ID] = i
	}

	for _, ev := range events {
		idx, ok := taskIdx[ev.TaskID]
		if !ok {
			continue
		}
		switch ev.Type {
		case DurableStepStarted:
			if replayed.Tasks[idx].Status == TaskPending {
				replayed.Tasks[idx].Status = TaskRunning
			}
		case DurableStepCompleted:
			if ev.Result != nil && ev.Result.Success {
				replayed.Tasks[idx].Status = TaskCompleted
				replayed.Tasks[idx].Result = ev.Result
			}
		case DurableStepFailed:
			replayed.Tasks[idx].Status = TaskFailed
			replayed.Tasks[idx].Result = ev.Result
		}
	}

	for _, ev := range events {
		switch ev.Type {
		case DurableRunCompleted:
			replayed.Status = "completed"
		case DurableRunFailed:
			replayed.Status = "failed"
		}
	}
	return replayed
}
