package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

// ─── State Constants ─────────────────────────────────────────────────────

const (
	StateInitializing = "initializing"
	StatePlanning     = "planning"
	StateExecuting    = "executing"
	StateWaiting      = "waiting"
	StateCompleted    = "completed"
	StateFailed       = "failed"
	StatePaused       = "paused"
)

// ─── Supporting Types ────────────────────────────────────────────────────

type StateChange struct {
	Timestamp time.Time `json:"timestamp"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Reason    string    `json:"reason"`
}

type TaskProgress struct {
	TaskID   string    `json:"task_id"`
	Status   string    `json:"status"`
	Progress float64   `json:"progress"`
	Agent    string    `json:"agent"`
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended"`
}

type Blocker struct {
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	Discovered  time.Time `json:"discovered"`
	Resolved    bool      `json:"resolved"`
}

type KnowledgeItem struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
	Source  string  `json:"source"`
}

type AgentContext struct {
	AgentName    string             `json:"agent_name"`
	Task         *TaskNode          `json:"task"`
	Goal         string             `json:"goal"`
	Knowledge    []string           `json:"knowledge"`
	Memories     []string           `json:"memories"`
	Decisions    []Decision         `json:"decisions"`
	Handoffs     []*HandoffArtifact `json:"handoffs"`
	PreviousWork string             `json:"previous_work"`
}

type ContextSnapshot struct {
	SessionID        string                  `json:"session_id"`
	Goal             string                  `json:"goal"`
	CurrentState     string                  `json:"current_state"`
	StateHistory     []StateChange           `json:"state_history"`
	PlanID           string                  `json:"plan_id"`
	ActiveAgent      string                  `json:"active_agent"`
	PreviousAgents   []string                `json:"previous_agents"`
	TaskProgress     map[string]TaskProgress `json:"task_progress"`
	Blockers         []Blocker               `json:"blockers"`
	Decisions        []Decision              `json:"decisions"`
	KnowledgeUsed    []string                `json:"knowledge_used"`
	MemoriesUsed     []string                `json:"memories_used"`
	CompressionLevel int                     `json:"compression_level"`
	CreatedAt        time.Time               `json:"created_at"`
}

type ContextStats struct {
	TotalTasks       int     `json:"total_tasks"`
	CompletedTasks   int     `json:"completed_tasks"`
	ActiveAgent      string  `json:"active_agent"`
	KnowledgeItems   int     `json:"knowledge_items"`
	MemoriesUsed     int     `json:"memories_used"`
	StateChanges     int     `json:"state_changes"`
	Blockers         int     `json:"blockers"`
	ActiveBlockers   int     `json:"active_blockers"`
	CompressionLevel int     `json:"compression_level"`
	EstimatedTokens  int     `json:"estimated_tokens"`
	CompressionRatio float64 `json:"compression_ratio"`
}

// ─── GeneralContext ──────────────────────────────────────────────────────

type GeneralContext struct {
	mu sync.RWMutex

	session      *SessionContext
	goal         string
	currentState string
	stateHistory []StateChange

	plan    *Plan
	planner *Planner

	activeAgent    string
	previousAgents []string
	agentRouter    *AgentRouter

	knowledge     orchestration.KnowledgeSearcher
	memory        orchestration.MemoryRetriever
	knowledgeUsed []string
	memoriesUsed  []string

	taskProgress map[string]TaskProgress
	blockers     []Blocker
	decisions    []Decision

	condenser        *ContextCondenser
	compressionLevel int

	metrics *SessionMetrics
	traceID string
}

// NewGeneralContext creates a new GeneralContext coordinator.
func NewGeneralContext(
	goal string,
	knowledge orchestration.KnowledgeSearcher,
	memory orchestration.MemoryRetriever,
	planner *Planner,
	agentResolver orchestration.AgentResolver,
) *GeneralContext {
	config := DefaultCondenserConfig()
	return &GeneralContext{
		goal:             goal,
		currentState:     StateInitializing,
		stateHistory:     make([]StateChange, 0),
		planner:          planner,
		previousAgents:   make([]string, 0),
		agentRouter:      NewAgentRouter(agentResolver),
		knowledge:        knowledge,
		memory:           memory,
		knowledgeUsed:    make([]string, 0),
		memoriesUsed:     make([]string, 0),
		taskProgress:     make(map[string]TaskProgress),
		blockers:         make([]Blocker, 0),
		decisions:        make([]Decision, 0),
		condenser:        NewContextCondenser(config),
		compressionLevel: 0,
	}
}

// ─── State Management ────────────────────────────────────────────────────

func (gc *GeneralContext) TransitionState(newState, reason string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	change := StateChange{
		Timestamp: time.Now().UTC(),
		From:      gc.currentState,
		To:        newState,
		Reason:    reason,
	}
	gc.stateHistory = append(gc.stateHistory, change)
	gc.currentState = newState
}

func (gc *GeneralContext) State() string {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.currentState
}

func (gc *GeneralContext) Snapshot() *ContextSnapshot {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	snap := &ContextSnapshot{
		SessionID:        gc.goal,
		Goal:             gc.goal,
		CurrentState:     gc.currentState,
		StateHistory:     cloneStateHistory(gc.stateHistory),
		ActiveAgent:      gc.activeAgent,
		PreviousAgents:   cloneStrings(gc.previousAgents),
		KnowledgeUsed:    cloneStrings(gc.knowledgeUsed),
		MemoriesUsed:     cloneStrings(gc.memoriesUsed),
		CompressionLevel: gc.compressionLevel,
		CreatedAt:        time.Now().UTC(),
	}

	if gc.plan != nil {
		snap.PlanID = gc.plan.ID
	}

	snap.TaskProgress = make(map[string]TaskProgress, len(gc.taskProgress))
	for k, v := range gc.taskProgress {
		snap.TaskProgress[k] = v
	}

	snap.Blockers = make([]Blocker, len(gc.blockers))
	copy(snap.Blockers, gc.blockers)

	snap.Decisions = make([]Decision, len(gc.decisions))
	copy(snap.Decisions, gc.decisions)

	if gc.session != nil {
		snap.SessionID = gc.session.SessionID
	}

	return snap
}

func (gc *GeneralContext) RestoreFromSnapshot(snap *ContextSnapshot) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	gc.goal = snap.Goal
	gc.currentState = snap.CurrentState
	gc.stateHistory = cloneStateHistory(snap.StateHistory)
	gc.activeAgent = snap.ActiveAgent
	gc.previousAgents = cloneStrings(snap.PreviousAgents)
	gc.knowledgeUsed = cloneStrings(snap.KnowledgeUsed)
	gc.memoriesUsed = cloneStrings(snap.MemoriesUsed)
	gc.compressionLevel = snap.CompressionLevel

	gc.taskProgress = make(map[string]TaskProgress, len(snap.TaskProgress))
	for k, v := range snap.TaskProgress {
		gc.taskProgress[k] = v
	}

	gc.blockers = make([]Blocker, len(snap.Blockers))
	copy(gc.blockers, snap.Blockers)

	gc.decisions = make([]Decision, len(snap.Decisions))
	copy(gc.decisions, snap.Decisions)
}

// ─── Planning ────────────────────────────────────────────────────────────

func (gc *GeneralContext) PlanExecution(prompt string) (*Plan, error) {
	if gc.planner == nil {
		return nil, fmt.Errorf("general context: no planner configured")
	}
	plan := gc.planner.Plan(prompt)
	gc.mu.Lock()
	gc.plan = plan
	gc.session = NewSessionContext(plan)
	gc.taskProgress = make(map[string]TaskProgress)
	for _, t := range plan.Tasks {
		gc.taskProgress[t.ID] = TaskProgress{
			TaskID:  t.ID,
			Status:  string(TaskPending),
			Progress: 0,
			Started: time.Time{},
			Ended:   time.Time{},
		}
	}
	gc.mu.Unlock()
	return plan, nil
}

func (gc *GeneralContext) CurrentPlan() *Plan {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.plan
}

func (gc *GeneralContext) TaskProgress(taskID string) TaskProgress {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	if tp, ok := gc.taskProgress[taskID]; ok {
		return tp
	}
	return TaskProgress{TaskID: taskID, Status: string(TaskPending), Progress: 0}
}

func (gc *GeneralContext) UpdateTaskProgress(taskID, status string, progress float64, agent string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	tp, ok := gc.taskProgress[taskID]
	if !ok {
		tp = TaskProgress{
			TaskID:  taskID,
			Started: time.Now().UTC(),
		}
	}
	tp.Status = status
	tp.Progress = progress
	tp.Agent = agent
	if status == string(TaskCompleted) || status == string(TaskFailed) {
		tp.Ended = time.Now().UTC()
	}
	gc.taskProgress[taskID] = tp
}

// ─── Agent Routing ───────────────────────────────────────────────────────

func (gc *GeneralContext) SelectAgent(task *TaskNode) (string, error) {
	gc.mu.RLock()
	router := gc.agentRouter
	gc.mu.RUnlock()

	agent, _, reason := router.Route(task)
	if agent == "" {
		return "", fmt.Errorf("agent routing failed: %s", reason)
	}
	return agent, nil
}

func (gc *GeneralContext) RegisterAgentSwitch(from, to, reason string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	if gc.activeAgent != "" && gc.activeAgent != from {
		gc.previousAgents = append(gc.previousAgents, gc.activeAgent)
	}
	gc.activeAgent = to

	gc.TransitionStateL(StateExecuting, reason)
}

func (gc *GeneralContext) ActiveAgent() string {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.activeAgent
}

// TransitionStateL is the lock-free variant for internal use when the caller
// already holds the write lock.
func (gc *GeneralContext) TransitionStateL(newState, reason string) {
	change := StateChange{
		Timestamp: time.Now().UTC(),
		From:      gc.currentState,
		To:        newState,
		Reason:    reason,
	}
	gc.stateHistory = append(gc.stateHistory, change)
	gc.currentState = newState
}

// ─── Knowledge Orchestration ─────────────────────────────────────────────

func (gc *GeneralContext) AcquireKnowledge(query string) ([]KnowledgeItem, error) {
	gc.mu.RLock()
	knowledge := gc.knowledge
	gc.mu.RUnlock()

	if knowledge == nil {
		return []KnowledgeItem{}, nil
	}

	results, err := knowledge.Search(context.Background(), orchestration.KnowledgeSearchParams{
		Query: query,
		Limit: 10,
	})
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var items []KnowledgeItem
	for _, r := range results.Results {
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		items = append(items, KnowledgeItem{
			ID:      r.ID,
			Title:   r.Title,
			Content: r.Snippet,
			Score:   r.Score,
			Source:  r.DocumentPath,
		})
	}
	return items, nil
}

func (gc *GeneralContext) RecordKnowledgeUsed(ids []string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	existing := make(map[string]bool, len(gc.knowledgeUsed))
	for _, id := range gc.knowledgeUsed {
		existing[id] = true
	}
	for _, id := range ids {
		if !existing[id] {
			gc.knowledgeUsed = append(gc.knowledgeUsed, id)
		}
	}
}

func (gc *GeneralContext) AcquireMemories(query string, agentName string) ([]orchestration.MemoryRecord, error) {
	gc.mu.RLock()
	memory := gc.memory
	gc.mu.RUnlock()

	if memory == nil {
		return []orchestration.MemoryRecord{}, nil
	}

	opts := orchestration.MemorySearchOptions{
		Layers:      []string{"session", "long_term"},
		Limit:       5,
		ReaderAgent: agentName,
	}
	results, err := memory.Search(context.Background(), query, opts)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (gc *GeneralContext) RecordMemoriesUsed(ids []string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	existing := make(map[string]bool, len(gc.memoriesUsed))
	for _, id := range gc.memoriesUsed {
		existing[id] = true
	}
	for _, id := range ids {
		if !existing[id] {
			gc.memoriesUsed = append(gc.memoriesUsed, id)
		}
	}
}

// ─── Context Management ──────────────────────────────────────────────────

func (gc *GeneralContext) BuildContextForAgent(agentName string, task *TaskNode) *AgentContext {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	actx := &AgentContext{
		AgentName: agentName,
		Task:      task,
		Goal:      gc.goal,
		Knowledge: cloneStrings(gc.knowledgeUsed),
		Memories:  cloneStrings(gc.memoriesUsed),
		Decisions: cloneDecisions(gc.decisions),
	}

	if gc.session != nil {
		actx.Handoffs = make([]*HandoffArtifact, len(gc.session.Handoffs))
		copy(actx.Handoffs, gc.session.Handoffs)
	}

	if len(gc.previousAgents) > 0 {
		actx.PreviousWork = fmt.Sprintf("Previously handled by: %s",
			strings.Join(gc.previousAgents, ", "))
	}

	return actx
}

func (gc *GeneralContext) CompactContext() {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	gc.compressionLevel++
	if gc.compressionLevel > 10 {
		gc.compressionLevel = 10
	}
}

func (gc *GeneralContext) ContextStats() ContextStats {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	stats := ContextStats{
		StateChanges:     len(gc.stateHistory),
		Blockers:         len(gc.blockers),
		CompressionLevel: gc.compressionLevel,
		KnowledgeItems:   len(gc.knowledgeUsed),
		MemoriesUsed:     len(gc.memoriesUsed),
		ActiveAgent:      gc.activeAgent,
	}

	if gc.plan != nil {
		stats.TotalTasks = len(gc.plan.Tasks)
		completed, _ := gc.plan.Progress()
		stats.CompletedTasks = completed
	}

	for _, b := range gc.blockers {
		if !b.Resolved {
			stats.ActiveBlockers++
		}
	}

	stats.EstimatedTokens = estimateTokens(stats)
	stats.CompressionRatio = float64(gc.compressionLevel) / 10.0

	return stats
}

// ─── Decision Tracking ───────────────────────────────────────────────────

func (gc *GeneralContext) RecordDecision(decision Decision) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.decisions = append(gc.decisions, decision)
}

func (gc *GeneralContext) RecordBlocker(blocker Blocker) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.blockers = append(gc.blockers, blocker)
}

// ─── Trace ───────────────────────────────────────────────────────────────

func (gc *GeneralContext) SetTraceID(traceID string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.traceID = traceID
}

func (gc *GeneralContext) TraceID() string {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.traceID
}

// ─── Observability ───────────────────────────────────────────────────────

func (gc *GeneralContext) Metrics() *SessionMetrics {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	if gc.metrics != nil {
		return gc.metrics
	}
	if gc.session != nil {
		return &gc.session.Metrics
	}
	return nil
}

func (gc *GeneralContext) Summary() string {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "GeneralContext State: %s\n", gc.currentState)
	fmt.Fprintf(&b, "Goal: %s\n", gc.goal)
	fmt.Fprintf(&b, "Active Agent: %s\n", gc.activeAgent)
	fmt.Fprintf(&b, "Knowledge Items: %d\n", len(gc.knowledgeUsed))
	fmt.Fprintf(&b, "Memories Used: %d\n", len(gc.memoriesUsed))
	fmt.Fprintf(&b, "Decisions: %d\n", len(gc.decisions))
	fmt.Fprintf(&b, "Blockers: %d\n", len(gc.blockers))
	fmt.Fprintf(&b, "State Changes: %d\n", len(gc.stateHistory))
	fmt.Fprintf(&b, "Compression Level: %d\n", gc.compressionLevel)
	if gc.plan != nil {
		completed, total := gc.plan.Progress()
		fmt.Fprintf(&b, "Plan: %d/%d tasks completed\n", completed, total)
	}
	if gc.session != nil {
		fmt.Fprintf(&b, "Session: %s\n", gc.session.SessionID)
	}
	return b.String()
}

// ─── Internal Helpers ────────────────────────────────────────────────────

func cloneStateHistory(src []StateChange) []StateChange {
	if src == nil {
		return nil
	}
	dst := make([]StateChange, len(src))
	copy(dst, src)
	return dst
}

func cloneStrings(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func cloneDecisions(src []Decision) []Decision {
	if src == nil {
		return nil
	}
	dst := make([]Decision, len(src))
	copy(dst, src)
	return dst
}

func estimateTokens(stats ContextStats) int {
	tokens := 0
	tokens += stats.StateChanges * 10
	tokens += stats.Blockers * 20
	tokens += stats.KnowledgeItems * 100
	tokens += stats.MemoriesUsed * 80
	tokens += stats.TotalTasks * 50
	tokens += stats.StateChanges * 5
	tokens += 200
	if stats.CompressionLevel > 0 {
		tokens = tokens / (stats.CompressionLevel + 1)
	}
	return tokens
}
