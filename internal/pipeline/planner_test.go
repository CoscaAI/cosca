package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

func TestDetectIntent(t *testing.T) {
	tests := []struct {
		prompt     string
		wantIntent string
	}{
		{"create a CRUD API for users", "create-crud-api"},
		{"build an api rest for products", "create-crud-api"},
		{"add authentication with login page", "add-auth"},
		{"fix the crash bug in parser", "fix-bug"},
		{"refactor the database layer", "refactor"},
		{"add more test coverage", "add-tests"},
		{"explain how goroutines work", ""},
		{"deploy to production", ""},
	}

	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			got := detectIntent(tt.prompt)
			if got != tt.wantIntent {
				t.Errorf("detectIntent(%q) = %q, want %q", tt.prompt, got, tt.wantIntent)
			}
		})
	}
}

func TestTaskTemplateMatching(t *testing.T) {
	tests := []struct {
		intent      string
		wantTaskIDs []string
	}{
		{"create-crud-api", []string{"design-schema", "create-models", "create-handlers", "create-tests", "build-verify"}},
		{"add-auth", []string{"design-auth", "implement-middleware", "create-handlers", "add-tests", "security-review"}},
		{"fix-bug", []string{"diagnose", "locate-code", "implement-fix", "add-tests", "verify"}},
		{"refactor", []string{"analyze-code", "identify-smells", "plan-refactor", "execute-refactor", "run-tests"}},
		{"add-tests", []string{"analyze-coverage", "identify-gaps", "write-tests", "run-verify"}},
	}

	for _, tt := range tests {
		t.Run(tt.intent, func(t *testing.T) {
			tmpl, ok := taskTemplates[tt.intent]
			if !ok {
				t.Fatalf("template %q not found", tt.intent)
			}
			ids := make([]string, len(tmpl))
			for i, t := range tmpl {
				ids[i] = t.ID
			}
			if len(ids) != len(tt.wantTaskIDs) {
				t.Fatalf("template %q has %d tasks, want %d", tt.intent, len(ids), len(tt.wantTaskIDs))
			}
			for i, want := range tt.wantTaskIDs {
				if ids[i] != want {
					t.Errorf("task[%d] = %q, want %q", i, ids[i], want)
				}
			}
		})
	}
}

func TestPlannerPlanWithTemplates(t *testing.T) {
	p := &Planner{}

	tests := []struct {
		prompt         string
		wantIntentType string
		wantTasks      int
	}{
		{"create a CRUD API for users", "create-crud-api", 5},
		{"add authentication with login", "add-auth", 5},
		{"fix the crash bug", "fix-bug", 5},
		{"refactor the parser", "refactor", 5},
		{"add test coverage for handlers", "add-tests", 4},
	}

	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			plan := p.Plan(tt.prompt)
			if plan.IntentType != tt.wantIntentType {
				t.Errorf("IntentType = %q, want %q", plan.IntentType, tt.wantIntentType)
			}
			if len(plan.Tasks) != tt.wantTasks {
				t.Errorf("Tasks = %d, want %d", len(plan.Tasks), tt.wantTasks)
			}
			if plan.ID == "" {
				t.Error("Plan ID is empty")
			}
			if !strings.HasPrefix(plan.ID, "PLAN-") {
				t.Errorf("Plan ID %q should start with PLAN-", plan.ID)
			}
		})
	}
}

func TestPlannerPlanUnknownIntent(t *testing.T) {
	p := &Planner{}

	plan := p.Plan("explain how goroutines work")
	if plan.IntentType != "" {
		t.Errorf("IntentType = %q, want empty (unknown intent)", plan.IntentType)
	}
	if len(plan.Tasks) == 0 {
		t.Fatal("Tasks should not be empty for unknown intent (should fall back)")
	}
	if plan.Tasks[0].ID != "task-1" {
		t.Errorf("fallback task ID = %q, want %q", plan.Tasks[0].ID, "task-1")
	}
}

func TestExecutionOrderLinear(t *testing.T) {
	plan := &Plan{
		ID:         "PLAN-TEST",
		IntentType: "fix-bug",
		Tasks:      cloneTasks(taskTemplates["fix-bug"]),
	}

	order := plan.ExecutionOrder()
	expected := []string{"diagnose", "locate-code", "implement-fix", "add-tests", "verify"}

	for i, task := range order {
		if task.ID != expected[i] {
			t.Errorf("order[%d] = %q, want %q", i, task.ID, expected[i])
		}
	}
}

func TestExecutionOrderParallel(t *testing.T) {
	plan := &Plan{
		ID: "PLAN-PARALLEL",
		Tasks: []*TaskNode{
			{ID: "a"},
			{ID: "b"},
			{ID: "c", DependsOn: []string{"a"}},
			{ID: "d", DependsOn: []string{"b"}},
			{ID: "e", DependsOn: []string{"c", "d"}},
		},
	}

	order := plan.ExecutionOrder()
	idPos := make(map[string]int)
	for i, task := range order {
		idPos[task.ID] = i
	}

	// a and b must come before c and d
	if idPos["a"] >= idPos["c"] {
		t.Errorf("a must precede c")
	}
	if idPos["b"] >= idPos["d"] {
		t.Errorf("b must precede d")
	}
	// c and d must come before e
	if idPos["c"] >= idPos["e"] {
		t.Errorf("c must precede e")
	}
	if idPos["d"] >= idPos["e"] {
		t.Errorf("d must precede e")
	}
}

func TestExecutionOrderCycle(t *testing.T) {
	plan := &Plan{
		ID: "PLAN-CYCLE",
		Tasks: []*TaskNode{
			{ID: "a", DependsOn: []string{"b"}},
			{ID: "b", DependsOn: []string{"a"}},
		},
	}

	if !plan.HasCycle() {
		t.Errorf("cycle should be detected")
	}
}

func TestHasCycleNegative(t *testing.T) {
	plan := &Plan{
		ID: "PLAN-NO-CYCLE",
		Tasks: []*TaskNode{
			{ID: "a"},
			{ID: "b", DependsOn: []string{"a"}},
		},
	}
	if plan.HasCycle() {
		t.Errorf("no cycle expected")
	}
}

func TestHasCycleEmpty(t *testing.T) {
	plan := &Plan{ID: "PLAN-EMPTY"}
	if plan.HasCycle() {
		t.Errorf("empty plan should not have a cycle")
	}
}

func TestChain(t *testing.T) {
	plan := &Plan{
		Tasks: []*TaskNode{
			{ID: "a"},
			{ID: "b", DependsOn: []string{"a"}},
			{ID: "c", DependsOn: []string{"b"}},
		},
	}

	chain := plan.Chain()
	if chain[0] != "a" || chain[1] != "b" || chain[2] != "c" {
		t.Errorf("chain = %v, want [a b c]", chain)
	}
}

func TestTaskLookup(t *testing.T) {
	plan := &Plan{
		Tasks: []*TaskNode{
			{ID: "design-schema"},
			{ID: "create-models", DependsOn: []string{"design-schema"}},
		},
	}

	task := plan.Task("create-models")
	if task == nil {
		t.Fatal("task not found")
	}
	if task.ID != "create-models" {
		t.Errorf("task ID = %q, want %q", task.ID, "create-models")
	}

	if plan.Task("nonexistent") != nil {
		t.Errorf("expected nil for missing task")
	}
}

func TestReadyTasks(t *testing.T) {
	plan := &Plan{
		Tasks: []*TaskNode{
			{ID: "a", Status: TaskCompleted},
			{ID: "b", DependsOn: []string{"a"}, Status: TaskPending},
			{ID: "c", DependsOn: []string{"a"}, Status: TaskPending},
			{ID: "d", DependsOn: []string{"b"}, Status: TaskPending},
		},
	}

	ready := plan.ReadyTasks()
	ids := make([]string, len(ready))
	for i, t := range ready {
		ids[i] = t.ID
	}

	// b and c should be ready; d still blocked on b
	for _, id := range ids {
		if id == "d" {
			t.Errorf("d should not be ready (depends on b which is pending)")
		}
	}

	found := make(map[string]bool)
	for _, id := range ids {
		found[id] = true
	}
	if !found["b"] {
		t.Errorf("b should be ready")
	}
	if !found["c"] {
		t.Errorf("c should be ready")
	}
}

func TestProgress(t *testing.T) {
	plan := &Plan{
		Tasks: []*TaskNode{
			{ID: "a", Status: TaskCompleted},
			{ID: "b", Status: TaskCompleted},
			{ID: "c", Status: TaskRunning},
			{ID: "d", Status: TaskPending},
		},
	}

	completed, total := plan.Progress()
	if completed != 2 {
		t.Errorf("completed = %d, want 2", completed)
	}
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
}

func TestSetStatus(t *testing.T) {
	plan := &Plan{
		Tasks: []*TaskNode{
			{ID: "a", Status: TaskPending},
		},
	}

	if !plan.SetStatus("a", TaskRunning) {
		t.Errorf("SetStatus should return true for existing task")
	}
	if plan.Tasks[0].Status != TaskRunning {
		t.Errorf("status = %s, want %s", plan.Tasks[0].Status, TaskRunning)
	}

	if plan.SetStatus("nonexistent", TaskCompleted) {
		t.Errorf("SetStatus should return false for missing task")
	}
}

func TestIsDone(t *testing.T) {
	tests := []struct {
		name     string
		statuses []TaskStatus
		want     bool
	}{
		{"all-completed", []TaskStatus{TaskCompleted, TaskCompleted}, true},
		{"mixed-terminal", []TaskStatus{TaskCompleted, TaskFailed, TaskSkipped}, true},
		{"with-running", []TaskStatus{TaskCompleted, TaskRunning}, false},
		{"with-pending", []TaskStatus{TaskPending}, false},
		{"empty", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tasks []*TaskNode
			for i, s := range tt.statuses {
				tasks = append(tasks, &TaskNode{ID: string(rune('a' + i)), Status: s})
			}
			plan := &Plan{Tasks: tasks}
			if got := plan.IsDone(); got != tt.want {
				t.Errorf("IsDone() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEstimateRisk(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "low"},
		{3, "low"},
		{4, "medium"},
		{6, "medium"},
		{7, "high"},
		{10, "high"},
	}

	for _, tt := range tests {
		tasks := make([]*TaskNode, tt.n)
		for i := range tasks {
			tasks[i] = &TaskNode{ID: string(rune('a' + i))}
		}
		if got := estimateRisk(tasks); got != tt.want {
			t.Errorf("estimateRisk(%d tasks) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestCloneTasks(t *testing.T) {
	src := taskTemplates["fix-bug"]
	cloned := cloneTasks(src)

	if len(cloned) != len(src) {
		t.Fatalf("len = %d, want %d", len(cloned), len(src))
	}

	for i := range cloned {
		if cloned[i].Status != TaskPending {
			t.Errorf("clone[%d].Status = %s, want %s", i, cloned[i].Status, TaskPending)
		}
		if cloned[i] == &src[i] {
			t.Errorf("clone[%d] shares address with source", i)
		}
	}

	// Mutate a clone — source must stay unchanged
	originalStatus := cloned[0].Status
	cloned[0].Status = TaskRunning

	srcCopy := cloneTasks(taskTemplates["fix-bug"])
	if srcCopy[0].Status != TaskPending {
		t.Errorf("source[0].Status = %s after clone mutation, want %s", srcCopy[0].Status, originalStatus)
	}
}

func TestGeneratePlanID(t *testing.T) {
	id1 := generatePlanID()
	id2 := generatePlanID()

	if !strings.HasPrefix(id1, "PLAN-20") {
		t.Errorf("plan ID %q should start with PLAN-20", id1)
	}
	if id1 == id2 {
		t.Error("consecutive plan IDs should differ (probability)")
	}
}

func TestTaskNode_ZeroPriorityDefaults(t *testing.T) {
	node := TaskNode{ID: "test"}
	if node.Priority != 0 {
		t.Errorf("Priority default = %d, want 0", node.Priority)
	}
	if node.Status != "" {
		t.Errorf("Status default = %s, want empty", node.Status)
	}
}

func TestPlannerSearchDelegation(t *testing.T) {
	p := &Planner{}
	params := orchestration.KnowledgeSearchParams{Query: "test"}
	results, err := p.Search(nil, params)
	if err != nil {
		t.Errorf("Search with nil searcher should not error: %v", err)
	}
	if results == nil {
		t.Error("Search should return non-nil results even with nil searcher")
	}
}

func TestExecutionOrderSingleNode(t *testing.T) {
	plan := &Plan{
		Tasks: []*TaskNode{{ID: "only"}},
	}

	order := plan.ExecutionOrder()
	if len(order) != 1 || order[0].ID != "only" {
		t.Errorf("single node order: %v", order)
	}
}

func TestExecutionOrderEmpty(t *testing.T) {
	plan := &Plan{}
	order := plan.ExecutionOrder()
	if order != nil {
		t.Errorf("empty plan order should be nil, got %v", order)
	}
}

func TestReadyTasksSortedByPriority(t *testing.T) {
	plan := &Plan{
		Tasks: []*TaskNode{
			{ID: "low", Priority: 10, Status: TaskPending},
			{ID: "high", Priority: 1, Status: TaskPending},
			{ID: "mid", Priority: 5, Status: TaskPending},
		},
	}

	ready := plan.ReadyTasks()
	if len(ready) != 3 {
		t.Fatalf("expected 3 ready tasks, got %d", len(ready))
	}
	if ready[0].ID != "high" {
		t.Errorf("first = %q, want high", ready[0].ID)
	}
	if ready[1].ID != "mid" {
		t.Errorf("second = %q, want mid", ready[1].ID)
	}
	if ready[2].ID != "low" {
		t.Errorf("third = %q, want low", ready[2].ID)
	}
}

func TestDepsCompletedNilDep(t *testing.T) {
	plan := &Plan{
		Tasks: []*TaskNode{
			{ID: "a", DependsOn: []string{"nonexistent"}, Status: TaskPending},
		},
	}

	if plan.depsCompleted(plan.Tasks[0]) {
		t.Errorf("task with nonexistent dep should not be ready")
	}
}

func TestValidateDependencies_Ok(t *testing.T) {
	plan := &Plan{
		ID: "PLAN-OK",
		Tasks: []*TaskNode{
			{ID: "a"},
			{ID: "b", DependsOn: []string{"a"}},
			{ID: "c", DependsOn: []string{"a", "b"}},
		},
	}

	if err := plan.ValidateDependencies(); err != nil {
		t.Errorf("valid plan should pass validation: %v", err)
	}
}

func TestValidateDependencies_Dangling(t *testing.T) {
	plan := &Plan{
		ID: "PLAN-BAD",
		Tasks: []*TaskNode{
			{ID: "a"},
			{ID: "b", DependsOn: []string{"does-not-exist"}},
			{ID: "c", DependsOn: []string{"a", "also-missing"}},
		},
	}

	err := plan.ValidateDependencies()
	if err == nil {
		t.Fatal("plan with dangling DependsOn must return an error")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error should mention the dangling dep, got: %v", err)
	}
	if !strings.Contains(err.Error(), "also-missing") {
		t.Errorf("error should mention every dangling dep, got: %v", err)
	}
}

func TestStepRunnerRejectsDanglingDependencies(t *testing.T) {
	plan := &Plan{
		ID: "PLAN-RUN-BAD",
		Tasks: []*TaskNode{
			{ID: "a"},
			{ID: "b", DependsOn: []string{"ghost-task"}},
		},
	}

	runner := &mockRunner{shouldSucceed: true, response: "ok"}
	sr := NewStepRunner(runner)
	if _, err := sr.RunPlan(context.Background(), plan); err == nil {
		t.Fatal("RunPlan must reject a plan with dangling DependsOn")
	}
	if _, err := sr.RunPlanParallel(context.Background(), plan, nil); err == nil {
		t.Fatal("RunPlanParallel must reject a plan with dangling DependsOn")
	}
}
