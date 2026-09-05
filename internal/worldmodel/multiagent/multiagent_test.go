package multiagent

import (
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// SharedWorld tests
// ──────────────────────────────────────────────────────────────

func TestNewSharedWorld(t *testing.T) {
	sw := NewSharedWorld()

	if sw == nil {
		t.Fatal("NewSharedWorld returned nil")
	}
	if len(sw.agents) != 0 {
		t.Errorf("initial agents: got %d, want 0", len(sw.agents))
	}
	if len(sw.tasks) != 0 {
		t.Errorf("initial tasks: got %d, want 0", len(sw.tasks))
	}
}

func TestSharedWorldState(t *testing.T) {
	sw := NewSharedWorld()

	state := worldmodel.WorldState{
		Step: 42,
		Climate: worldmodel.ClimateState{
			Temperature: 25.0,
		},
	}

	sw.UpdateState(state)
	got := sw.GetState()

	if got.Step != 42 {
		t.Errorf("step: got %v, want 42", got.Step)
	}
	if got.Climate.Temperature != 25.0 {
		t.Errorf("temperature: got %v, want 25.0", got.Climate.Temperature)
	}
}

// ──────────────────────────────────────────────────────────────
// Agent management tests
// ──────────────────────────────────────────────────────────────

func TestAddAgent(t *testing.T) {
	sw := NewSharedWorld()

	agent := &Agent{
		ID:           "agent_01",
		Role:         RoleRecon,
		Capabilities: []string{"observe", "move"},
		Position:     worldmodel.Vec3{X: 0, Y: 0, Z: 0},
		Status:       "active",
	}

	sw.AddAgent(agent)

	got, ok := sw.GetAgent("agent_01")
	if !ok {
		t.Fatal("agent not found")
	}
	if got.ID != "agent_01" {
		t.Errorf("id: got %v, want agent_01", got.ID)
	}
	if got.Role != RoleRecon {
		t.Errorf("role: got %v, want recon", got.Role)
	}
	if got.Status != "active" {
		t.Errorf("status: got %v, want active", got.Status)
	}
}

func TestGetAgentsByRole(t *testing.T) {
	sw := NewSharedWorld()

	sw.AddAgent(&Agent{ID: "a1", Role: RoleRecon, Status: "active"})
	sw.AddAgent(&Agent{ID: "a2", Role: RoleRecon, Status: "active"})
	sw.AddAgent(&Agent{ID: "a3", Role: RoleExecution, Status: "active"})

	recons := sw.GetAgentsByRole(RoleRecon)
	if len(recons) != 2 {
		t.Errorf("recon agents: got %d, want 2", len(recons))
	}

	executors := sw.GetAgentsByRole(RoleExecution)
	if len(executors) != 1 {
		t.Errorf("execution agents: got %d, want 1", len(executors))
	}
}

func TestGetActiveAgents(t *testing.T) {
	sw := NewSharedWorld()

	sw.AddAgent(&Agent{ID: "a1", Status: "active"})
	sw.AddAgent(&Agent{ID: "a2", Status: "idle"})
	sw.AddAgent(&Agent{ID: "a3", Status: "active"})

	active := sw.GetActiveAgents()
	if len(active) != 2 {
		t.Errorf("active agents: got %d, want 2", len(active))
	}
}

// ──────────────────────────────────────────────────────────────
// Task management tests
// ──────────────────────────────────────────────────────────────

func TestAddTask(t *testing.T) {
	sw := NewSharedWorld()

	task := &Task{
		ID:       "task_01",
		Type:     "observe",
		Target:   worldmodel.Vec3{X: 10, Y: 0, Z: 0},
		Priority: PriorityNormal,
	}

	sw.AddTask(task)

	if len(sw.tasks) != 1 {
		t.Errorf("tasks: got %d, want 1", len(sw.tasks))
	}
	if task.Status != "pending" {
		t.Errorf("status: got %v, want pending", task.Status)
	}
}

func TestAssignTask(t *testing.T) {
	sw := NewSharedWorld()

	sw.AddAgent(&Agent{
		ID:           "agent_01",
		Role:         RoleRecon,
		Capabilities: []string{"observe", "move"},
		Position:     worldmodel.Vec3{X: 0, Y: 0, Z: 0},
		Status:       "active",
	})

	sw.AddTask(&Task{
		ID:           "task_01",
		Type:         "observe",
		Target:       worldmodel.Vec3{X: 5, Y: 0, Z: 0},
		Priority:     PriorityNormal,
		RequiredCaps: []string{"observe"},
	})

	agent, err := sw.AssignTask("task_01")
	if err != nil {
		t.Fatalf("AssignTask: %v", err)
	}

	if agent.ID != "agent_01" {
		t.Errorf("assigned agent: got %v, want agent_01", agent.ID)
	}

	task := sw.tasks["task_01"]
	if task.Status != "assigned" {
		t.Errorf("task status: got %v, want assigned", task.Status)
	}
}

func TestAssignTaskNotFound(t *testing.T) {
	sw := NewSharedWorld()

	_, err := sw.AssignTask("nonexistent")
	if err == nil {
		t.Error("AssignTask with nonexistent task should fail")
	}
}

func TestAssignTaskNoSuitableAgent(t *testing.T) {
	sw := NewSharedWorld()

	sw.AddAgent(&Agent{
		ID:           "agent_01",
		Role:         RoleRecon,
		Capabilities: []string{"observe"},
		Status:       "active",
	})

	sw.AddTask(&Task{
		ID:           "task_01",
		Type:         "defend",
		Target:       worldmodel.Vec3{X: 100, Y: 0, Z: 0},
		Priority:     PriorityHigh,
		RequiredCaps: []string{"combat", "defense"},
	})

	_, err := sw.AssignTask("task_01")
	if err == nil {
		t.Error("AssignTask with no suitable agent should fail")
	}
}

func TestCompleteTask(t *testing.T) {
	sw := NewSharedWorld()

	sw.AddTask(&Task{
		ID:     "task_01",
		Type:   "observe",
		Status: "assigned",
	})

	result := map[string]string{"observation": "clear area"}
	err := sw.CompleteTask("task_01", result)
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	task := sw.tasks["task_01"]
	if task.Status != "completed" {
		t.Errorf("status: got %v, want completed", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("completed_at should not be nil")
	}
	if task.Result["observation"] != "clear area" {
		t.Errorf("result: got %v, want 'clear area'", task.Result["observation"])
	}
}

func TestGetPendingTasks(t *testing.T) {
	sw := NewSharedWorld()

	sw.AddTask(&Task{ID: "t1", Status: "pending"})
	sw.AddTask(&Task{ID: "t2", Status: "assigned"})
	sw.AddTask(&Task{ID: "t3", Status: "pending"})

	pending := sw.GetPendingTasks()
	if len(pending) != 2 {
		t.Errorf("pending tasks: got %d, want 2", len(pending))
	}
}

// ──────────────────────────────────────────────────────────────
// Task scoring tests
// ──────────────────────────────────────────────────────────────

func TestScoreAgentCapabilities(t *testing.T) {
	sw := NewSharedWorld()

	agent := &Agent{
		ID:           "a1",
		Capabilities: []string{"observe", "move", "shoot"},
		Status:       "active",
	}

	task := &Task{
		Type:         "observe",
		RequiredCaps: []string{"observe"},
	}

	score := sw.scoreAgent(agent, task)
	if score <= 0 {
		t.Errorf("score should be > 0, got %v", score)
	}
}

func TestScoreAgentProximity(t *testing.T) {
	sw := NewSharedWorld()

	near := &Agent{ID: "near", Position: worldmodel.Vec3{X: 1, Y: 0, Z: 0}, Status: "active"}
	far := &Agent{ID: "far", Position: worldmodel.Vec3{X: 100, Y: 0, Z: 0}, Status: "active"}

	task := &Task{Target: worldmodel.Vec3{X: 0, Y: 0, Z: 0}}

	nearScore := sw.scoreAgent(near, task)
	farScore := sw.scoreAgent(far, task)

	if nearScore <= farScore {
		t.Errorf("near agent should score higher: near=%v, far=%v", nearScore, farScore)
	}
}

func TestScoreAgentRoleBonus(t *testing.T) {
	sw := NewSharedWorld()

	recon := &Agent{ID: "recon", Role: RoleRecon, Status: "active"}
	exec := &Agent{ID: "exec", Role: RoleExecution, Status: "active"}

	task := &Task{Type: "observe", Target: worldmodel.Vec3{}}

	reconScore := sw.scoreAgent(recon, task)
	execScore := sw.scoreAgent(exec, task)

	if reconScore <= execScore {
		t.Errorf("recon agent should score higher for observe task: recon=%v, exec=%v", reconScore, execScore)
	}
}

// ──────────────────────────────────────────────────────────────
// Conflict resolution tests
// ──────────────────────────────────────────────────────────────

func TestDetectConflicts(t *testing.T) {
	sw := NewSharedWorld()

	sw.AddAgent(&Agent{ID: "a1", Status: "active"})
	sw.AddAgent(&Agent{ID: "a2", Status: "active"})

	sw.AddTask(&Task{ID: "t1", Type: "observe", Target: worldmodel.Vec3{X: 5, Y: 0, Z: 0}, Status: "assigned", AssignedTo: "a1"})
	sw.AddTask(&Task{ID: "t2", Type: "observe", Target: worldmodel.Vec3{X: 5, Y: 0, Z: 0}, Status: "assigned", AssignedTo: "a2"})

	conflicts := sw.DetectConflicts()
	if len(conflicts) != 1 {
		t.Errorf("conflicts: got %d, want 1", len(conflicts))
	}
}

func TestResolveConflict(t *testing.T) {
	sw := NewSharedWorld()

	sw.AddAgent(&Agent{ID: "a1", Status: "active"})
	sw.AddAgent(&Agent{ID: "a2", Status: "active"})

	sw.AddTask(&Task{ID: "t1", Type: "observe", Target: worldmodel.Vec3{X: 5, Y: 0, Z: 0}, Priority: PriorityHigh, Status: "assigned", AssignedTo: "a1"})
	sw.AddTask(&Task{ID: "t2", Type: "observe", Target: worldmodel.Vec3{X: 5, Y: 0, Z: 0}, Priority: PriorityLow, Status: "assigned", AssignedTo: "a2"})

	sw.DetectConflicts()
	err := sw.ResolveConflict("conflict_5,0,0")
	if err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}

	// a1 (high priority) should keep task, a2 (low) should be reassigned
	t1 := sw.tasks["t1"]
	t2 := sw.tasks["t2"]

	if t1.AssignedTo != "a1" || t1.Status != "assigned" {
		t.Errorf("t1: got agent=%v status=%v, want a1/assigned", t1.AssignedTo, t1.Status)
	}
	if t2.Status != "pending" {
		t.Errorf("t2: got status=%v, want pending", t2.Status)
	}
}

// ──────────────────────────────────────────────────────────────
// Event history tests
// ──────────────────────────────────────────────────────────────

func TestAddEvent(t *testing.T) {
	sw := NewSharedWorld()

	sw.AddEvent(worldmodel.WorldEvent{
		Type:      "spawn",
		EntityID:  "npc_01",
		Position:  worldmodel.Vec3{X: 5, Y: 0, Z: 0},
		Timestamp: time.Now(),
	})

	history := sw.GetHistory(10)
	if len(history) != 1 {
		t.Errorf("history: got %d, want 1", len(history))
	}
	if history[0].Type != "spawn" {
		t.Errorf("event type: got %v, want spawn", history[0].Type)
	}
}

func TestGetHistoryLimit(t *testing.T) {
	sw := NewSharedWorld()

	for i := 0; i < 10; i++ {
		sw.AddEvent(worldmodel.WorldEvent{Type: "test", Timestamp: time.Now()})
	}

	history := sw.GetHistory(3)
	if len(history) != 3 {
		t.Errorf("history limit: got %d, want 3", len(history))
	}
}

// ──────────────────────────────────────────────────────────────
// Agent role tests
// ──────────────────────────────────────────────────────────────

func TestAgentRoles(t *testing.T) {
	roles := []AgentRole{RoleLeader, RoleRecon, RoleExecution, RoleSupport, RoleGuard}

	for _, role := range roles {
		if role == "" {
			t.Error("role should not be empty")
		}
	}
}

func TestTaskPriority(t *testing.T) {
	if PriorityUrgent <= PriorityHigh {
		t.Error("urgent should be higher than high")
	}
	if PriorityHigh <= PriorityNormal {
		t.Error("high should be higher than normal")
	}
	if PriorityNormal <= PriorityLow {
		t.Error("normal should be higher than low")
	}
}
