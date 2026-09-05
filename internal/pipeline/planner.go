package pipeline

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

// TaskNode represents a single task in the execution DAG.
type TaskNode struct {
	ID                string      `json:"id"`
	Description       string      `json:"description"`
	Agent             string      `json:"agent"`
	DependsOn         []string    `json:"depends_on,omitempty"`
	InputFiles        []string    `json:"input_files,omitempty"`
	OutputFiles       []string    `json:"output_files,omitempty"`
	Priority          int         `json:"priority"`
	Status            TaskStatus  `json:"status"`
	Result            *TaskResult `json:"result,omitempty"`
	HandoffArtifactID string      `json:"handoff_artifact_id,omitempty"`
	CreatedAt         time.Time   `json:"created_at"`

	// IntentType é a intenção da task (detectTaskType). É o dado EXPLÍCITO que
	// decide, no executor, se a task é de AÇÃO/IMPLEMENTAÇÃO (deve ir ao LLM
	// com tools) ou de CONSULTA/AVALIAÇÃO (pode ser resolvida pelo knowledge
	// determinístico). Decidido no PLANEJAMENTO (workflowToPlan/Planner), nunca
	// inferido dentro do Executor — evita um "segundo COSCA" heurístico.
	IntentType string `json:"intent_type,omitempty"`
}

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskSkipped   TaskStatus = "skipped"
)

type TaskResult struct {
	Success    bool   `json:"success"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Agent      string `json:"agent,omitempty"`
	TraceID    string `json:"trace_id,omitempty"`
	// BuildResult and TestResult carry real build/test evidence from the
	// runner. They back the Definition of Done "builds"/"tests_pass" checks
	// so a plan without build/test evidence can never claim success.
	BuildResult *BuildResult `json:"build_result,omitempty"`
	TestResult  *TestResult  `json:"test_result,omitempty"`
}

// IsSuccess reports whether the result represents a completed step. A nil
// result is never a success, which keeps durable replay honest: an event whose
// result was lost must not be replayed as completed.
func (r *TaskResult) IsSuccess() bool {
	return r != nil && r.Success
}

// Clone returns a deep copy of the result, including build/test evidence.
func (r *TaskResult) Clone() *TaskResult {
	if r == nil {
		return nil
	}
	cp := *r
	if r.BuildResult != nil {
		b := *r.BuildResult
		cp.BuildResult = &b
	}
	if r.TestResult != nil {
		t := *r.TestResult
		cp.TestResult = &t
	}
	return &cp
}

// Plan is a directed acyclic graph of tasks.
type Plan struct {
	ID               string      `json:"id"`
	Intent           string      `json:"intent"`
	IntentType       string      `json:"intent_type"`
	Tasks            []*TaskNode `json:"tasks"`
	EstimatedMinutes int         `json:"estimated_minutes"`
	RiskLevel        string      `json:"risk_level"`
	Objective        string      `json:"objective,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	Status           string      `json:"status,omitempty"`
}

var taskTemplates = map[string][]TaskNode{
	"create-crud-api": {
		{ID: "design-schema", Description: "Design database schema", Agent: "cosca-database"},
		{ID: "create-models", Description: "Create data models", Agent: "cosca-backend", DependsOn: []string{"design-schema"}},
		{ID: "create-handlers", Description: "Create API handlers", Agent: "cosca-backend", DependsOn: []string{"create-models"}},
		{ID: "create-tests", Description: "Create tests", Agent: "cosca-testing", DependsOn: []string{"create-handlers"}},
		{ID: "build-verify", Description: "Build and verify", Agent: "cosca-backend", DependsOn: []string{"create-tests"}},
	},
	"add-auth": {
		{ID: "design-auth", Description: "Design authentication flow", Agent: "cosca-security"},
		{ID: "implement-middleware", Description: "Implement auth middleware", Agent: "cosca-backend", DependsOn: []string{"design-auth"}},
		{ID: "create-handlers", Description: "Create login/register handlers", Agent: "cosca-backend", DependsOn: []string{"implement-middleware"}},
		{ID: "add-tests", Description: "Add auth tests", Agent: "cosca-testing", DependsOn: []string{"create-handlers"}},
		{ID: "security-review", Description: "Security review", Agent: "cosca-security", DependsOn: []string{"add-tests"}},
	},
	"fix-bug": {
		{ID: "diagnose", Description: "Diagnose the bug from error logs", Agent: "cosca-backend"},
		{ID: "locate-code", Description: "Locate affected code", Agent: "cosca-backend", DependsOn: []string{"diagnose"}},
		{ID: "implement-fix", Description: "Implement fix", Agent: "cosca-backend", DependsOn: []string{"locate-code"}},
		{ID: "add-tests", Description: "Add regression test", Agent: "cosca-testing", DependsOn: []string{"implement-fix"}},
		{ID: "verify", Description: "Build and run tests", Agent: "cosca-backend", DependsOn: []string{"add-tests"}},
	},
	"refactor": {
		{ID: "analyze-code", Description: "Analyze current code structure", Agent: "cosca-backend"},
		{ID: "identify-smells", Description: "Identify code smells", Agent: "cosca-critic", DependsOn: []string{"analyze-code"}},
		{ID: "plan-refactor", Description: "Plan refactoring steps", Agent: "cosca-backend", DependsOn: []string{"identify-smells"}},
		{ID: "execute-refactor", Description: "Execute refactoring", Agent: "cosca-backend", DependsOn: []string{"plan-refactor"}},
		{ID: "run-tests", Description: "Run tests to verify", Agent: "cosca-testing", DependsOn: []string{"execute-refactor"}},
	},
	"add-tests": {
		{ID: "analyze-coverage", Description: "Analyze current test coverage", Agent: "cosca-testing"},
		{ID: "identify-gaps", Description: "Identify coverage gaps", Agent: "cosca-testing", DependsOn: []string{"analyze-coverage"}},
		{ID: "write-tests", Description: "Write new tests", Agent: "cosca-testing", DependsOn: []string{"identify-gaps"}},
		{ID: "run-verify", Description: "Run tests and verify", Agent: "cosca-testing", DependsOn: []string{"write-tests"}},
	},
}

// Planner decomposes user intent into executable task graphs.
type Planner struct {
	knowledgeSearcher orchestration.KnowledgeSearcher
	agentResolver     orchestration.AgentResolver
}

// NewPlanner creates a Planner backed by the given port interfaces.
func NewPlanner(
	knowledgeSearcher orchestration.KnowledgeSearcher,
	agentResolver orchestration.AgentResolver,
) *Planner {
	return &Planner{
		knowledgeSearcher: knowledgeSearcher,
		agentResolver:     agentResolver,
	}
}

// Plan decomposes a user prompt into a Plan (DAG of tasks).
// Deterministic first via template matching; falls back to agent
// resolution when confidence is low.
func (p *Planner) Plan(prompt string) *Plan {
	intent := detectIntent(prompt)
	template, ok := taskTemplates[intent]

	id := generatePlanID()
	plan := &Plan{
		ID:         id,
		Intent:     prompt,
		IntentType: intent,
		Objective:  prompt,
		CreatedAt:  time.Now().UTC(),
		RiskLevel:  "low",
	}

	if ok {
		tasks := cloneTasks(template)
		plan.Tasks = tasks
		plan.EstimatedMinutes = len(tasks) * 3
		plan.RiskLevel = estimateRisk(tasks)
		return plan
	}

	if p.knowledgeSearcher != nil {
		tasks := p.contextualTasks(prompt)
		if len(tasks) > 0 {
			plan.Tasks = tasks
			plan.EstimatedMinutes = len(tasks) * 3
			plan.RiskLevel = "medium"
			return plan
		}
	}

	plan.Tasks = []*TaskNode{
		{ID: "task-1", Description: prompt, Agent: ""},
	}
	plan.EstimatedMinutes = 5
	plan.RiskLevel = "medium"
	return plan
}

func detectIntent(prompt string) string {
	lower := strings.ToLower(prompt)
	if strings.Contains(lower, "crud") || strings.Contains(lower, "api rest") {
		return "create-crud-api"
	}
	if strings.Contains(lower, "auth") || strings.Contains(lower, "login") {
		return "add-auth"
	}
	if strings.Contains(lower, "fix") || strings.Contains(lower, "bug") || strings.Contains(lower, "erro") {
		return "fix-bug"
	}
	if strings.Contains(lower, "refactor") {
		return "refactor"
	}
	if strings.Contains(lower, "test") || strings.Contains(lower, "coverage") {
		return "add-tests"
	}
	return ""
}

// ExecutionOrder returns tasks in dependency order (Kahn's algorithm).
// When a cycle is present, returns all tasks as-is.
func (p *Plan) ExecutionOrder() []*TaskNode {
	order := p.topologicalSort()
	if order == nil {
		return nil
	}
	return order
}

// topologicalSort performs Kahn's algorithm. Returns nil when there are no
// tasks, and may return fewer tasks than exist when a cycle is present.
func (p *Plan) topologicalSort() []*TaskNode {
	if len(p.Tasks) == 0 {
		return nil
	}

	inDegree := make(map[string]int)
	adj := make(map[string][]string)
	idMap := make(map[string]*TaskNode)

	for _, task := range p.Tasks {
		idMap[task.ID] = task
		if _, ok := inDegree[task.ID]; !ok {
			inDegree[task.ID] = 0
		}
		for _, dep := range task.DependsOn {
			adj[dep] = append(adj[dep], task.ID)
			inDegree[task.ID]++
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []*TaskNode
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, idMap[current])

		for _, next := range adj[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	return order
}

// HasCycle returns true when the plan contains a dependency cycle.
func (p *Plan) HasCycle() bool {
	if len(p.Tasks) == 0 {
		return false
	}
	order := p.ExecutionOrder()
	return len(order) != len(p.Tasks)
}

// ValidateDependencies returns an error when any task references a DependsOn
// ID that does not exist in the plan. Dangling dependencies silently drop the
// dependent task (its dep never completes), so plans must be validated before
// execution.
func (p *Plan) ValidateDependencies() error {
	if len(p.Tasks) == 0 {
		return nil
	}
	ids := make(map[string]struct{}, len(p.Tasks))
	for _, t := range p.Tasks {
		ids[t.ID] = struct{}{}
	}
	var dangling []string
	for _, t := range p.Tasks {
		for _, dep := range t.DependsOn {
			if _, ok := ids[dep]; !ok {
				dangling = append(dangling, fmt.Sprintf("%q -> %q", t.ID, dep))
			}
		}
	}
	if len(dangling) > 0 {
		return fmt.Errorf("plan %q has dangling DependsOn references: %s", p.ID, strings.Join(dangling, ", "))
	}
	return nil
}

// Chain returns the task IDs in execution order.
func (p *Plan) Chain() []string {
	order := p.ExecutionOrder()
	out := make([]string, len(order))
	for i, t := range order {
		out[i] = t.ID
	}
	return out
}

// Task returns the task with the given ID, or nil.
func (p *Plan) Task(id string) *TaskNode {
	for _, t := range p.Tasks {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// ReadyTasks returns tasks whose dependencies are all completed
// and whose status is still pending.
func (p *Plan) ReadyTasks() []*TaskNode {
	var ready []*TaskNode
	for _, t := range p.Tasks {
		if t.Status != TaskPending {
			continue
		}
		if p.depsCompleted(t) {
			ready = append(ready, t)
		}
	}
	sort.SliceStable(ready, func(i, j int) bool {
		return ready[i].Priority < ready[j].Priority
	})
	return ready
}

func (p *Plan) depsCompleted(task *TaskNode) bool {
	for _, depID := range task.DependsOn {
		dep := p.Task(depID)
		if dep == nil || dep.Status != TaskCompleted {
			return false
		}
	}
	return true
}

// Progress returns the number of completed tasks and total.
func (p *Plan) Progress() (completed, total int) {
	total = len(p.Tasks)
	for _, t := range p.Tasks {
		if t.Status == TaskCompleted {
			completed++
		}
	}
	return
}

// SetStatus updates the status of a task by ID.
func (p *Plan) SetStatus(id string, status TaskStatus) bool {
	task := p.Task(id)
	if task == nil {
		return false
	}
	task.Status = status
	return true
}

// IsDone returns true when all tasks are completed, failed, or skipped.
func (p *Plan) IsDone() bool {
	for _, t := range p.Tasks {
		switch t.Status {
		case TaskCompleted, TaskFailed, TaskSkipped:
			continue
		default:
			return false
		}
	}
	return true
}

// Search delegates to the underlying KnowledgeSearcher.
func (p *Planner) Search(ctx context.Context, params orchestration.KnowledgeSearchParams) (*orchestration.KnowledgeSearchResults, error) {
	if p.knowledgeSearcher == nil {
		return &orchestration.KnowledgeSearchResults{}, nil
	}
	return p.knowledgeSearcher.Search(ctx, params)
}

// ── internal helpers ─────────────────────────────────────────────────────────

func cloneTasks(src []TaskNode) []*TaskNode {
	out := make([]*TaskNode, len(src))
	for i := range src {
		cp := src[i]
		cp.Status = TaskPending
		cp.CreatedAt = time.Now().UTC()
		if cp.DependsOn == nil {
			cp.DependsOn = []string{}
		} else {
			deps := make([]string, len(cp.DependsOn))
			copy(deps, cp.DependsOn)
			cp.DependsOn = deps
		}
		out[i] = &cp
	}
	return out
}

func estimateRisk(tasks []*TaskNode) string {
	n := len(tasks)
	switch {
	case n >= 7:
		return "high"
	case n >= 4:
		return "medium"
	default:
		return "low"
	}
}

func generatePlanID() string {
	t := time.Now().UTC()
	n, _ := rand.Int(rand.Reader, big.NewInt(1<<24))
	//nolint:gosec // not security-sensitive
	return fmt.Sprintf("PLAN-%s-%06X", t.Format("20060102"), n.Uint64()%0xFFFFFF)
}

func (p *Planner) contextualTasks(prompt string) []*TaskNode {
	if p.agentResolver == nil {
		return nil
	}

	agents, err := p.agentResolver.Search(prompt)
	if err != nil || len(agents) == 0 {
		return nil
	}

	var tasks []*TaskNode
	for i, agent := range agents {
		if i >= 5 {
			break
		}
		tasks = append(tasks, &TaskNode{
			ID:          fmt.Sprintf("auto-task-%d", i+1),
			Description: fmt.Sprintf("Execute prompt with agent %s: %s", agent.Name, prompt),
			Agent:       agent.Name,
			DependsOn:   nil,
			Status:      TaskPending,
			CreatedAt:   time.Now().UTC(),
		})
	}

	if len(tasks) <= 1 {
		return tasks
	}

	return tasks
}
