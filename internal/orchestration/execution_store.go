// Package orchestration provides the execution store for persisting
// orchestration run history in-memory with a bounded ring buffer.
package orchestration

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ExecutionStatus represents the completion status of an execution.
type ExecutionStatus string

const (
	ExecutionStatusSuccess ExecutionStatus = "success"
	ExecutionStatusError   ExecutionStatus = "error"
)

// Execution represents a single AI orchestration run stored for history retrieval.
type Execution struct {
	ID            string      `json:"id"`
	UserID        string      `json:"user_id,omitempty"` // claims.Sub of the requesting user (A6 owner scoping)
	Prompt        string      `json:"prompt"`
	Response      string      `json:"response"`
	Agent         string      `json:"agent"`
	Provider      string      `json:"provider"`
	Model         string      `json:"model,omitempty"`
	Status        string      `json:"status"`
	DurationMs    int64       `json:"duration_ms"`
	CreatedAt     time.Time   `json:"created_at"`
	SkillsUsed    []string    `json:"skills_used,omitempty"`
	MemoryID      string      `json:"memory_id,omitempty"`
	PipelineTrace interface{} `json:"pipeline_trace,omitempty"`
	TraceID       string      `json:"trace_id,omitempty"` // trace universal (TRACE-YYYYMMDD-XXXX); populado apenas quando passado
}

// ExecutionStore is a thread-safe, bounded in-memory store for execution history.
// It retains the last N executions using a ring-buffer approach.
type ExecutionStore struct {
	mu      sync.RWMutex
	records []*Execution   // ordered from oldest to newest
	index   map[string]int // execution ID -> position in records slice
	maxSize int
}

// NewExecutionStore creates a new ExecutionStore with the given maximum capacity.
func NewExecutionStore(maxSize int) *ExecutionStore {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &ExecutionStore{
		records: make([]*Execution, 0, maxSize+1),
		index:   make(map[string]int, maxSize+1),
		maxSize: maxSize,
	}
}

// Store persists an execution. When capacity is exceeded, the oldest record
// is evicted. If the execution has no ID, one is generated.
func (s *ExecutionStore) Store(exec *Execution) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if exec.ID == "" {
		exec.ID = uuid.New().String()
	}
	if exec.CreatedAt.IsZero() {
		exec.CreatedAt = time.Now().UTC()
	}

	// Evict oldest when at capacity.
	if len(s.records) >= s.maxSize {
		oldest := s.records[0]
		s.records = s.records[1:]
		delete(s.index, oldest.ID)

		// Reindex remaining entries.
		for i, r := range s.records {
			s.index[r.ID] = i
		}
	}

	s.index[exec.ID] = len(s.records)
	s.records = append(s.records, exec)
}

// List returns a paginated slice of executions ordered by creation time
// (newest first). If agent, status or userID are non-empty, results are
// filtered accordingly. An empty userID returns executions from every user
// (system/admin view).
func (s *ExecutionStore) List(limit, offset int, agent, status, userID string) ([]*Execution, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect in reverse order (newest first).
	filtered := make([]*Execution, 0)
	for i := len(s.records) - 1; i >= 0; i-- {
		r := s.records[i]
		// Apply filters.
		if agent != "" && r.Agent != agent {
			continue
		}
		if status != "" && r.Status != status {
			continue
		}
		if userID != "" && r.UserID != userID {
			continue
		}
		filtered = append(filtered, r)
	}

	total := len(filtered)

	// Apply offset.
	if offset >= len(filtered) {
		return []*Execution{}, total
	}
	filtered = filtered[offset:]

	// Apply limit.
	if limit <= 0 {
		limit = 20
	}
	if limit > len(filtered) {
		limit = len(filtered)
	}

	return filtered[:limit], total
}

// Get retrieves a single execution by ID. Returns nil if not found.
func (s *ExecutionStore) Get(id string) *Execution {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, ok := s.index[id]
	if !ok {
		return nil
	}
	if idx >= len(s.records) {
		return nil
	}
	return s.records[idx]
}

// StoreExecution is a convenience function for storing an execution with
// computed fields. userID is the claims.Sub of the requesting user (empty for
// anonymous/system runs). It returns the execution ID assigned.
func (s *ExecutionStore) StoreExecution(userID, prompt, response, agent, provider, model, status string, durationMs int64, skillsUsed []string, memoryID string) string {
	exec := &Execution{
		UserID:     userID,
		Prompt:     prompt,
		Response:   response,
		Agent:      agent,
		Provider:   provider,
		Model:      model,
		Status:     status,
		DurationMs: durationMs,
		SkillsUsed: skillsUsed,
		MemoryID:   memoryID,
	}
	s.Store(exec)
	return exec.ID
}

// ─── Global execution store singleton ────────────────────────────────────────

var globalExecStore *ExecutionStore
var execStoreOnce sync.Once

// GetExecutionStore returns the global singleton ExecutionStore.
// The store is lazily initialized with a default capacity of 1000 records.
func GetExecutionStore() *ExecutionStore {
	execStoreOnce.Do(func() {
		globalExecStore = NewExecutionStore(1000)
	})
	return globalExecStore
}

// ResetExecutionStore is exported for testing purposes.
// It replaces the global store with a fresh instance.
func ResetExecutionStore() {
	execStoreOnce = sync.Once{}
	globalExecStore = NewExecutionStore(1000)
}

// String returns a human-readable representation for debugging.
func (e *Execution) String() string {
	return fmt.Sprintf("Execution{id=%s, agent=%s, status=%s, duration_ms=%d}",
		e.ID, e.Agent, e.Status, e.DurationMs)
}
