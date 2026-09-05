// Package multiagent implements multi-agent coordination for the Living World.
//
// The multi-agent system enables multiple Cosca agents (or other AI agents)
// to coexist and collaborate in the same world:
//
//	Agent A observes → Agent B observes → Shared World Model
//	  → Coordination (who does what?)
//	  → Conflict Resolution (competing actions)
//	  → Synchronized State (all agents see the same world)
//
// This is the foundation for team-based operations:
// one agent handles reconnaissance, another handles execution,
// a third handles logistics — all coordinated through the shared world model.
package multiagent

import (
	"fmt"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Agent representation
// ──────────────────────────────────────────────────────────────

// AgentRole classifies agent roles in the operation.
type AgentRole string

const (
	RoleLeader    AgentRole = "leader"    // coordinates others
	RoleRecon     AgentRole = "recon"     // reconnaissance/observation
	RoleExecution AgentRole = "execution" // carries out actions
	RoleSupport   AgentRole = "support"   // logistics, backup
	RoleGuard     AgentRole = "guard"     // security, perimeter
)

// Agent represents a single agent in the multi-agent system.
type Agent struct {
	ID           string            `json:"id"`
	Role         AgentRole         `json:"role"`
	Capabilities []string          `json:"capabilities"` // what this agent can do
	Position     worldmodel.Vec3   `json:"position"`
	Status       string            `json:"status"`       // "active", "idle", "disabled"
	LastSeen     time.Time         `json:"last_seen"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Task assignment
// ──────────────────────────────────────────────────────────────

// TaskPriority defines task urgency.
type TaskPriority int

const (
	PriorityLow    TaskPriority = 0
	PriorityNormal TaskPriority = 1
	PriorityHigh   TaskPriority = 2
	PriorityUrgent TaskPriority = 3
)

// Task represents a unit of work to be assigned.
type Task struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`         // "observe", "move", "interact", "defend"
	Target       worldmodel.Vec3   `json:"target"`
	Priority     TaskPriority      `json:"priority"`
	RequiredCaps []string          `json:"required_caps"` // capabilities needed
	AssignedTo   string            `json:"assigned_to"`   // agent ID (empty = unassigned)
	Status       string            `json:"status"`        // "pending", "assigned", "completed", "failed"
	Result       map[string]string `json:"result,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Conflict resolution
// ──────────────────────────────────────────────────────────────

// Conflict represents a disagreement between agents.
type Conflict struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`      // "resource", "territory", "action"
	Agents    []string `json:"agents"`    // conflicting agent IDs
	Resource  string   `json:"resource"`  // what's contested
	Proposals map[string]string `json:"proposals"` // agent -> proposed action
	Resolved  bool     `json:"resolved"`
	Resolution string  `json:"resolution,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Shared World Model
// ──────────────────────────────────────────────────────────────

// SharedWorld is a thread-safe shared world model for multi-agent coordination.
type SharedWorld struct {
	mu       sync.RWMutex
	state    worldmodel.WorldState
	agents   map[string]*Agent
	tasks    map[string]*Task
	conflicts map[string]*Conflict
	history  []worldmodel.WorldEvent
}

// NewSharedWorld creates a new shared world.
func NewSharedWorld() *SharedWorld {
	return &SharedWorld{
		agents:   make(map[string]*Agent),
		tasks:    make(map[string]*Task),
		conflicts: make(map[string]*Conflict),
	}
}

// GetState returns a copy of the current world state.
func (sw *SharedWorld) GetState() worldmodel.WorldState {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	return sw.state
}

// UpdateState replaces the world state.
func (sw *SharedWorld) UpdateState(state worldmodel.WorldState) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.state = state
}

// AddAgent registers an agent.
func (sw *SharedWorld) AddAgent(agent *Agent) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	agent.LastSeen = time.Now()
	sw.agents[agent.ID] = agent
}

// GetAgent returns an agent by ID.
func (sw *SharedWorld) GetAgent(id string) (*Agent, bool) {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	a, ok := sw.agents[id]
	return a, ok
}

// GetAgentsByRole returns all agents with a given role.
func (sw *SharedWorld) GetAgentsByRole(role AgentRole) []*Agent {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	var result []*Agent
	for _, a := range sw.agents {
		if a.Role == role {
			result = append(result, a)
		}
	}
	return result
}

// GetActiveAgents returns all active agents.
func (sw *SharedWorld) GetActiveAgents() []*Agent {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	var result []*Agent
	for _, a := range sw.agents {
		if a.Status == "active" {
			result = append(result, a)
		}
	}
	return result
}

// ──────────────────────────────────────────────────────────────
// Task management
// ──────────────────────────────────────────────────────────────

// AddTask creates a new task.
func (sw *SharedWorld) AddTask(task *Task) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	task.CreatedAt = time.Now()
	if task.Status == "" {
		task.Status = "pending"
	}
	sw.tasks[task.ID] = task
}

// AssignTask assigns a task to the best-suited agent.
func (sw *SharedWorld) AssignTask(taskID string) (*Agent, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	task, ok := sw.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != "pending" {
		return nil, fmt.Errorf("task %s is not pending (status: %s)", taskID, task.Status)
	}

	// Find best agent
	bestAgent := sw.findBestAgent(task)
	if bestAgent == nil {
		return nil, fmt.Errorf("no suitable agent found for task %s", taskID)
	}

	task.AssignedTo = bestAgent.ID
	task.Status = "assigned"
	return bestAgent, nil
}

// findBestAgent finds the agent best suited for a task.
func (sw *SharedWorld) findBestAgent(task *Task) *Agent {
	var best *Agent
	bestScore := -1.0

	for _, agent := range sw.agents {
		if agent.Status != "active" {
			continue
		}

		score := sw.scoreAgent(agent, task)
		if score > bestScore {
			bestScore = score
			best = agent
		}
	}

	// Require minimum score — no match if capabilities don't align
	if bestScore < 30.0 {
		return nil
	}
	return best
}

// scoreAgent scores how well an agent fits a task.
func (sw *SharedWorld) scoreAgent(agent *Agent, task *Task) float64 {
	score := 0.0

	// Capability match
	if len(task.RequiredCaps) > 0 {
		matchCount := 0
		for _, req := range task.RequiredCaps {
			for _, cap := range agent.Capabilities {
				if req == cap {
					matchCount++
					break
				}
			}
		}
		score += float64(matchCount) / float64(len(task.RequiredCaps)) * 50.0
	} else {
		score += 25.0 // no specific requirements
	}

	// Proximity to target
	dist := agent.Position.DistanceTo(task.Target)
	if dist < 10.0 {
		score += 30.0
	} else if dist < 50.0 {
		score += 20.0
	} else if dist < 100.0 {
		score += 10.0
	}

	// Role bonus
	switch task.Type {
	case "observe":
		if agent.Role == RoleRecon {
			score += 20.0
		}
	case "defend":
		if agent.Role == RoleGuard {
			score += 20.0
		}
	case "move", "interact":
		if agent.Role == RoleExecution {
			score += 20.0
		}
	}

	return score
}

// CompleteTask marks a task as completed.
func (sw *SharedWorld) CompleteTask(taskID string, result map[string]string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	task, ok := sw.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	now := time.Now()
	task.Status = "completed"
	task.Result = result
	task.CompletedAt = &now
	return nil
}

// GetPendingTasks returns all unassigned tasks.
func (sw *SharedWorld) GetPendingTasks() []*Task {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	var result []*Task
	for _, t := range sw.tasks {
		if t.Status == "pending" {
			result = append(result, t)
		}
	}
	return result
}

// ──────────────────────────────────────────────────────────────
// Conflict resolution
// ──────────────────────────────────────────────────────────────

// DetectConflicts checks for conflicting task assignments.
func (sw *SharedWorld) DetectConflicts() []Conflict {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	var conflicts []Conflict

	// Check for resource conflicts (multiple agents targeting same location)
	agentTargets := make(map[string][]string) // target -> agent IDs
	for _, t := range sw.tasks {
		if t.Status == "assigned" {
			key := fmt.Sprintf("%.0f,%.0f,%.0f", t.Target.X, t.Target.Y, t.Target.Z)
			agentTargets[key] = append(agentTargets[key], t.AssignedTo)
		}
	}

	for target, agentIDs := range agentTargets {
		if len(agentIDs) > 1 {
			conflict := Conflict{
				ID:       fmt.Sprintf("conflict_%s", target),
				Type:     "resource",
				Agents:   agentIDs,
				Resource: target,
				Proposals: make(map[string]string),
			}
			for _, id := range agentIDs {
				conflict.Proposals[id] = "execute"
			}
			sw.conflicts[conflict.ID] = &conflict
			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts
}

// ResolveConflict resolves a conflict by priority (highest priority task wins).
func (sw *SharedWorld) ResolveConflict(conflictID string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	conflict, ok := sw.conflicts[conflictID]
	if !ok {
		return fmt.Errorf("conflict %s not found", conflictID)
	}

	// Find highest priority task among conflicting agents
	var bestAgent string
	bestPriority := PriorityLow

	for _, agentID := range conflict.Agents {
		for _, task := range sw.tasks {
			if task.AssignedTo == agentID && task.Status == "assigned" {
				if task.Priority > bestPriority {
					bestPriority = task.Priority
					bestAgent = agentID
				}
			}
		}
	}

	// Assign to winner, reassign losers
	for _, agentID := range conflict.Agents {
		if agentID != bestAgent {
			for _, task := range sw.tasks {
				if task.AssignedTo == agentID && task.Status == "assigned" {
					task.AssignedTo = ""
					task.Status = "pending"
				}
			}
		}
	}

	conflict.Resolved = true
	conflict.Resolution = fmt.Sprintf("assigned to %s (priority %d)", bestAgent, bestPriority)
	return nil
}

// ──────────────────────────────────────────────────────────────
// Event history
// ──────────────────────────────────────────────────────────────

// AddEvent records a world event.
func (sw *SharedWorld) AddEvent(event worldmodel.WorldEvent) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.history = append(sw.history, event)
}

// GetHistory returns recent events.
func (sw *SharedWorld) GetHistory(limit int) []worldmodel.WorldEvent {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	if limit <= 0 || limit > len(sw.history) {
		limit = len(sw.history)
	}
	return sw.history[len(sw.history)-limit:]
}
