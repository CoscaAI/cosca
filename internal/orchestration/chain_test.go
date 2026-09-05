package orchestration

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ─── Mock Orchestrator ───────────────────────────────────────────────────────

// mockOrchestrator implements Orchestrator for testing chain execution.
type mockOrchestrator struct {
	mu               sync.Mutex
	executeFn        func(ctx context.Context, req *Request) (*Result, error)
	recordedRequests []*Request
	results          map[string]*Result
	callCount        int32
}

func newMockOrchestrator() *mockOrchestrator {
	return &mockOrchestrator{
		results: make(map[string]*Result),
	}
}

func (m *mockOrchestrator) Execute(ctx context.Context, req *Request) (*Result, error) {
	m.mu.Lock()
	m.recordedRequests = append(m.recordedRequests, req)
	m.mu.Unlock()
	atomic.AddInt32(&m.callCount, 1)

	if m.executeFn != nil {
		return m.executeFn(ctx, req)
	}

	// Look up by agent name in context.
	agentName := "CEO Agent"
	if req.Context != nil {
		if a, ok := req.Context["agent"].(string); ok && a != "" {
			agentName = a
		}
	}

	if res, ok := m.results[agentName]; ok {
		result := *res
		result.ID = req.ID
		if req.ID == "" {
			result.ID = GenerateRequestID()
		}
		return &result, nil
	}

	// Default response echoes the agent and prompt.
	return &Result{
		ID:       req.ID,
		Response: fmt.Sprintf("[%s] processed: %s", agentName, req.Prompt),
		Agent:    agentName,
		Duration: 10 * time.Millisecond,
	}, nil
}

func (m *mockOrchestrator) ExecuteStream(_ context.Context, _ *Request) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 1)
	go func() {
		defer close(ch)
		ch <- StreamEvent{Type: StreamEventChunk, Content: "mock stream"}
	}()
	return ch, nil
}

func (m *mockOrchestrator) recordedAgentNames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var names []string
	for _, req := range m.recordedRequests {
		if req.Context != nil {
			if a, ok := req.Context["agent"].(string); ok {
				names = append(names, a)
			}
		}
	}
	return names
}

// ─── Test Helpers ────────────────────────────────────────────────────────────

// newTestChainExecutor creates a ChainExecutor with a mock orchestrator,
// reusing existing mockAgentResolver and mockSkillResolver from the package.
func newTestChainExecutor(mock *mockOrchestrator) *ChainExecutor {
	agents := newMockAgentResolver()
	agents.add("CEO Agent", "Strategic Decision Maker", "ceo", "Makes strategic decisions")
	agents.add("Backend Chief", "Backend Development Lead", "backend", "Leads backend development")
	agents.add("Database Chief", "Database Lead", "database", "Leads database design")
	agents.add("Frontend Chief", "Frontend Development Lead", "frontend", "Leads frontend development")
	agents.add("Testing Chief", "Testing Lead", "qa", "Leads testing")

	skills := newMockSkillResolver()

	return NewChainExecutor(mock, agents, skills, ChainConfig{
		Enabled:        true,
		MaxSteps:       5,
		MaxParallel:    3,
		SynthesisAgent: "CEO Agent",
	})
}

// ─── Tests: Sequential Execution ─────────────────────────────────────────────

func TestChainExecutor_Sequential(t *testing.T) {
	mock := newMockOrchestrator()
	exec := newTestChainExecutor(mock)

	def := ChainDefinition{
		Name: "test-sequential",
		Steps: []ChainStep{
			{Name: "step1", Agent: "Backend Chief", Prompt: "design the API"},
			{Name: "step2", Agent: "Database Chief", Prompt: "design the schema"},
		},
		Parallel: false,
	}

	result, err := exec.Execute(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}

	if !result.Steps[0].Success {
		t.Errorf("step1 should succeed, got error: %s", result.Steps[0].Error)
	}
	if !result.Steps[1].Success {
		t.Errorf("step2 should succeed, got error: %s", result.Steps[1].Error)
	}

	if !strings.Contains(result.Response, "Database Chief") {
		t.Errorf("expected response from Database Chief, got: %s", result.Response)
	}

	agentNames := mock.recordedAgentNames()
	if len(agentNames) != 2 {
		t.Fatalf("expected 2 recorded agents, got %d", len(agentNames))
	}
	if agentNames[0] != "Backend Chief" {
		t.Errorf("expected first agent Backend Chief, got %s", agentNames[0])
	}
	if agentNames[1] != "Database Chief" {
		t.Errorf("expected second agent Database Chief, got %s", agentNames[1])
	}
}

func TestChainExecutor_Sequential_LargeChain(t *testing.T) {
	mock := newMockOrchestrator()
	exec := newTestChainExecutor(mock)

	steps := make([]ChainStep, 10)
	for i := range steps {
		steps[i] = ChainStep{
			Name:   fmt.Sprintf("step-%d", i+1),
			Agent:  "Backend Chief",
			Prompt: fmt.Sprintf("task %d", i+1),
		}
	}

	def := ChainDefinition{Name: "large-chain", Steps: steps, Parallel: false}
	result, err := exec.Execute(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Steps) != 10 {
		t.Errorf("expected 10 steps, got %d", len(result.Steps))
	}
}

// ─── Tests: Parallel Execution ───────────────────────────────────────────────

func TestChainExecutor_Parallel(t *testing.T) {
	mock := newMockOrchestrator()

	execOrder := make(chan string, 10)
	mock.executeFn = func(_ context.Context, req *Request) (*Result, error) {
		agent := "CEO Agent"
		if req.Context != nil {
			if a, ok := req.Context["agent"].(string); ok {
				agent = a
			}
		}
		execOrder <- agent
		return &Result{
			ID:       req.ID,
			Response: fmt.Sprintf("[%s] done", agent),
			Agent:    agent,
			Duration: 20 * time.Millisecond,
		}, nil
	}

	exec := newTestChainExecutor(mock)

	def := ChainDefinition{
		Name: "test-parallel",
		Steps: []ChainStep{
			{Name: "a", Agent: "Backend Chief", Prompt: "task a"},
			{Name: "b", Agent: "Database Chief", Prompt: "task b"},
			{Name: "c", Agent: "Frontend Chief", Prompt: "task c"},
		},
		Parallel: true,
	}

	result, err := exec.Execute(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(result.Steps))
	}

	for _, s := range result.Steps {
		if !s.Success {
			t.Errorf("step %s should succeed, got error: %s", s.Name, s.Error)
		}
	}

	close(execOrder)
	var agents []string
	for a := range execOrder {
		agents = append(agents, a)
	}
	if len(agents) != 3 {
		t.Errorf("expected 3 agents called, got %d: %v", len(agents), agents)
	}
}

// ─── Tests: Dependencies ─────────────────────────────────────────────────────

func TestChainExecutor_Dependencies(t *testing.T) {
	mock := newMockOrchestrator()

	var orderMu sync.Mutex
	var execOrder []string

	mock.executeFn = func(_ context.Context, req *Request) (*Result, error) {
		agent := "CEO Agent"
		if req.Context != nil {
			if a, ok := req.Context["agent"].(string); ok {
				agent = a
			}
		}
		orderMu.Lock()
		execOrder = append(execOrder, agent)
		orderMu.Unlock()

		return &Result{
			ID:       req.ID,
			Response: fmt.Sprintf("[%s] output for %s", agent, req.Prompt),
			Agent:    agent,
			Duration: 10 * time.Millisecond,
		}, nil
	}

	exec := newTestChainExecutor(mock)

	def := ChainDefinition{
		Name: "test-deps",
		Steps: []ChainStep{
			{Name: "design", Agent: "Backend Chief", Prompt: "design API"},
			{Name: "schema", Agent: "Database Chief", Prompt: "create schema", DependsOn: []string{"design"}},
			{Name: "ui", Agent: "Frontend Chief", Prompt: "build UI", DependsOn: []string{"design"}},
			{Name: "test", Agent: "Testing Chief", Prompt: "test everything", DependsOn: []string{"schema", "ui"}},
		},
		Parallel: true,
	}

	result, err := exec.Execute(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(result.Steps))
	}

	for _, s := range result.Steps {
		if !s.Success {
			t.Errorf("step %s should succeed, got error: %s", s.Name, s.Error)
		}
	}

	orderMu.Lock()
	order := execOrder
	orderMu.Unlock()

	idx := func(name string) int {
		for i, a := range order {
			if a == name {
				return i
			}
		}
		return -1
	}

	if idx("Backend Chief") >= idx("Database Chief") {
		t.Error("design must execute before schema")
	}
	if idx("Backend Chief") >= idx("Frontend Chief") {
		t.Error("design must execute before ui")
	}
	if idx("Database Chief") >= idx("Testing Chief") {
		t.Error("schema must execute before test")
	}
	if idx("Frontend Chief") >= idx("Testing Chief") {
		t.Error("ui must execute before test")
	}
}

// ─── Tests: Context Injection ────────────────────────────────────────────────

func TestChainExecutor_ContextInjection(t *testing.T) {
	mock := newMockOrchestrator()

	var capturedContext map[string]interface{}
	mock.executeFn = func(_ context.Context, req *Request) (*Result, error) {
		capturedContext = req.Context
		return &Result{
			ID:       req.ID,
			Response: fmt.Sprintf("processed with context: %v", req.Context),
			Agent:    "Test",
			Duration: 10 * time.Millisecond,
		}, nil
	}

	exec := newTestChainExecutor(mock)
	mock.results["Backend Chief"] = &Result{
		Response: "API design output",
		Agent:    "Backend Chief",
	}

	def := ChainDefinition{
		Name: "test-context",
		Steps: []ChainStep{
			{Name: "design", Agent: "Backend Chief", Prompt: "design API"},
			{Name: "implement", Agent: "Database Chief", Prompt: "implement based on design", DependsOn: []string{"design"}},
		},
		Parallel: false,
	}

	_, err := exec.Execute(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedContext == nil {
		t.Fatal("expected context to be captured for dependent step")
	}

	_, ok := capturedContext["design_output"]
	if !ok {
		t.Error("expected design_output in context for dependent step")
	}
}

// ─── Tests: Topological Sort ─────────────────────────────────────────────────

func TestChainExecutor_TopologicalSort(t *testing.T) {
	tests := []struct {
		name  string
		steps []ChainStep
		want  int
		err   bool
	}{
		{name: "empty", steps: []ChainStep{}, want: 0},
		{name: "single", steps: []ChainStep{{Name: "a", Agent: "X", Prompt: "x"}}, want: 1},
		{
			name: "independent",
			steps: []ChainStep{
				{Name: "a", Agent: "X", Prompt: "x"},
				{Name: "b", Agent: "Y", Prompt: "y"},
			},
			want: 1,
		},
		{
			name: "linear chain",
			steps: []ChainStep{
				{Name: "a", Agent: "X", Prompt: "x"},
				{Name: "b", Agent: "Y", Prompt: "y", DependsOn: []string{"a"}},
				{Name: "c", Agent: "Z", Prompt: "z", DependsOn: []string{"b"}},
			},
			want: 3,
		},
		{
			name: "diamond",
			steps: []ChainStep{
				{Name: "a", Agent: "X", Prompt: "x"},
				{Name: "b", Agent: "Y", Prompt: "y", DependsOn: []string{"a"}},
				{Name: "c", Agent: "Z", Prompt: "z", DependsOn: []string{"a"}},
				{Name: "d", Agent: "W", Prompt: "w", DependsOn: []string{"b", "c"}},
			},
			want: 3,
		},
		{
			name: "cycle",
			steps: []ChainStep{
				{Name: "a", Agent: "X", Prompt: "x", DependsOn: []string{"b"}},
				{Name: "b", Agent: "Y", Prompt: "y", DependsOn: []string{"a"}},
			},
			err: true,
		},
		{
			name:  "self dependency",
			steps: []ChainStep{{Name: "a", Agent: "X", Prompt: "x", DependsOn: []string{"a"}}},
			err:   true,
		},
		{
			name:  "unknown dependency",
			steps: []ChainStep{{Name: "a", Agent: "X", Prompt: "x", DependsOn: []string{"nonexistent"}}},
			err:   true,
		},
		{
			name: "duplicate names",
			steps: []ChainStep{
				{Name: "a", Agent: "X", Prompt: "x"},
				{Name: "a", Agent: "Y", Prompt: "y"},
			},
			err: true,
		},
		{
			name:  "empty name",
			steps: []ChainStep{{Name: "", Agent: "X", Prompt: "x"}},
			err:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			levels, err := resolveDependencies(tt.steps)
			if tt.err {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(levels) != tt.want {
				t.Errorf("expected %d levels, got %d", tt.want, len(levels))
			}
		})
	}
}

// ─── Tests: Step Timeout ─────────────────────────────────────────────────────

func TestChainExecutor_StepTimeout(t *testing.T) {
	mock := newMockOrchestrator()

	mock.executeFn = func(ctx context.Context, _ *Request) (*Result, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return &Result{Response: "slow response", Agent: "Test"}, nil
		}
	}

	exec := newTestChainExecutor(mock)

	def := ChainDefinition{
		Name: "test-timeout",
		Steps: []ChainStep{
			{Name: "fast", Agent: "Backend Chief", Prompt: "fast task", Timeout: 500 * time.Millisecond},
			{Name: "slow", Agent: "Database Chief", Prompt: "slow task", Timeout: 50 * time.Millisecond},
			{Name: "after-slow", Agent: "Frontend Chief", Prompt: "still runs", Timeout: 500 * time.Millisecond},
		},
		Parallel: false,
	}

	result, err := exec.Execute(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(result.Steps))
	}

	if !result.Steps[0].Success {
		t.Errorf("fast step should succeed, got: %s", result.Steps[0].Error)
	}
	if result.Steps[1].Success {
		t.Error("slow step should fail due to timeout")
	}
	if !result.Steps[2].Success {
		t.Errorf("after-slow step should succeed, got: %s", result.Steps[2].Error)
	}
}

// ─── Tests: Step Error ───────────────────────────────────────────────────────

func TestChainExecutor_StepError(t *testing.T) {
	mock := newMockOrchestrator()

	mock.executeFn = func(_ context.Context, req *Request) (*Result, error) {
		agent := "CEO Agent"
		if req.Context != nil {
			if a, ok := req.Context["agent"].(string); ok {
				agent = a
			}
		}
		if agent == "Backend Chief" {
			return nil, errors.New("backend failure")
		}
		return &Result{Response: fmt.Sprintf("[%s] ok", agent), Agent: agent}, nil
	}

	exec := newTestChainExecutor(mock)

	def := ChainDefinition{
		Name: "test-error",
		Steps: []ChainStep{
			{Name: "good1", Agent: "Frontend Chief", Prompt: "task1"},
			{Name: "bad", Agent: "Backend Chief", Prompt: "failing task"},
			{Name: "good2", Agent: "Database Chief", Prompt: "task3"},
		},
		Parallel: false,
	}

	result, err := exec.Execute(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(result.Steps))
	}

	if !result.Steps[0].Success {
		t.Errorf("good1 should succeed")
	}
	if result.Steps[1].Success {
		t.Error("bad should fail")
	}
	if result.Steps[1].ErrorCode != "chain_step_failed" || strings.Contains(result.Steps[1].Error, "backend failure") {
		t.Errorf("expected safe chain error, got: %+v", result.Steps[1])
	}
	if !result.Steps[2].Success {
		t.Errorf("good2 should succeed despite previous failure")
	}
	if !strings.Contains(result.Response, "Database Chief") {
		t.Errorf("expected final response from Database Chief, got: %s", result.Response)
	}
}

// ─── Tests: All Steps Fail ───────────────────────────────────────────────────

func TestChainExecutor_AllStepsFail(t *testing.T) {
	mock := newMockOrchestrator()

	mock.executeFn = func(_ context.Context, _ *Request) (*Result, error) {
		return nil, errors.New("everything is broken")
	}

	exec := newTestChainExecutor(mock)

	def := ChainDefinition{
		Name: "all-fail",
		Steps: []ChainStep{
			{Name: "a", Agent: "Backend Chief", Prompt: "task"},
			{Name: "b", Agent: "Database Chief", Prompt: "task"},
		},
	}

	result, err := exec.Execute(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, s := range result.Steps {
		if s.Success {
			t.Errorf("step %s should have failed", s.Name)
		}
	}

	if !strings.Contains(result.Response, "all steps failed") {
		t.Errorf("expected all-steps-failed message, got: %s", result.Response)
	}
}

// ─── Tests: Synthesis ────────────────────────────────────────────────────────

func TestChainExecutor_Synthesis(t *testing.T) {
	mock := newMockOrchestrator()

	var synthesisPrompt string

	mock.executeFn = func(_ context.Context, req *Request) (*Result, error) {
		agent := "CEO Agent"
		if req.Context != nil {
			if a, ok := req.Context["agent"].(string); ok {
				agent = a
			}
		}
		if strings.Contains(req.Prompt, "Synthesize") || strings.Contains(req.Prompt, "You are the CEO Agent") {
			synthesisPrompt = req.Prompt
		}
		return &Result{Response: fmt.Sprintf("[%s] synthesized response", agent), Agent: agent}, nil
	}

	exec := newTestChainExecutor(mock)

	stepResults := []ChainStepResult{
		{Name: "api", Agent: "Backend Chief", Success: true, Output: "REST API designed"},
		{Name: "db", Agent: "Database Chief", Success: true, Output: "Schema created"},
	}

	synthesis, err := exec.synthesizeResults(context.Background(), "build a system", stepResults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if synthesis == "" {
		t.Error("synthesis returned empty string")
	}
	if synthesisPrompt == "" {
		t.Error("synthesis prompt was not captured")
	}
	if !strings.Contains(synthesisPrompt, "REST API designed") {
		t.Error("synthesis prompt should contain step1 output")
	}
	if !strings.Contains(synthesisPrompt, "Schema created") {
		t.Error("synthesis prompt should contain step2 output")
	}
	if !strings.Contains(synthesisPrompt, "build a system") {
		t.Error("synthesis prompt should contain original request")
	}
}

// ─── Tests: formatStepResults ────────────────────────────────────────────────

func TestChainExecutor_FormatStepResults(t *testing.T) {
	mock := newMockOrchestrator()
	exec := newTestChainExecutor(mock)

	results := []ChainStepResult{
		{Name: "step1", Agent: "Agent A", Success: true, Output: "Output A"},
		{Name: "step2", Agent: "Agent B", Success: false, Error: "something broke"},
	}

	formatted := exec.formatStepResults(results)

	if !strings.Contains(formatted, "Step 1") {
		t.Error("should contain Step 1 header")
	}
	if !strings.Contains(formatted, "Agent A") {
		t.Error("should contain agent name A")
	}
	if !strings.Contains(formatted, "Output A") {
		t.Error("should contain output A")
	}
	if !strings.Contains(formatted, "Step 2") {
		t.Error("should contain Step 2 header")
	}
	if !strings.Contains(formatted, "Agent B") {
		t.Error("should contain agent name B")
	}
	if !strings.Contains(formatted, "[ERROR]") {
		t.Error("should contain error marker for failed step")
	}
	if strings.Contains(formatted, "something broke") {
		t.Error("formatted results should not contain raw error text")
	}
}

// ─── Tests: Empty Steps ──────────────────────────────────────────────────────

func TestChainExecutor_EmptySteps(t *testing.T) {
	mock := newMockOrchestrator()
	exec := newTestChainExecutor(mock)

	result, err := exec.Execute(context.Background(), ChainDefinition{
		Name:  "empty",
		Steps: []ChainStep{},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Response == "" {
		t.Error("should return a response message")
	}
	if len(result.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(result.Steps))
	}
}

// ─── Tests: Max Steps ────────────────────────────────────────────────────────

func TestChainExecutor_MaxSteps(t *testing.T) {
	mock := newMockOrchestrator()
	exec := newTestChainExecutor(mock)

	steps := make([]ChainStep, 10)
	for i := range steps {
		steps[i] = ChainStep{
			Name:   fmt.Sprintf("s%d", i+1),
			Agent:  "Backend Chief",
			Prompt: fmt.Sprintf("task %d", i+1),
		}
	}

	def := ChainDefinition{Name: "too-many", Steps: steps, Parallel: false}

	result, err := exec.Execute(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Steps) != 10 {
		t.Errorf("expected 10 steps, got %d — MaxSteps only applies to dynamic decomposition", len(result.Steps))
	}
}

// ─── Tests: Max Steps in Dynamic ─────────────────────────────────────────────

func TestChainExecutor_DynamicMaxSteps(t *testing.T) {
	mock := newMockOrchestrator()

	var callCount int64
	mock.executeFn = func(_ context.Context, _ *Request) (*Result, error) {
		c := atomic.AddInt64(&callCount, 1)
		if c == 1 {
			steps := make([]string, 0, 10)
			for i := range 10 {
				steps = append(steps, fmt.Sprintf(`{"agent": "Backend Chief", "prompt": "task %d"}`, i+1))
			}
			return &Result{Response: "[" + strings.Join(steps, ",") + "]", Agent: "CEO Agent"}, nil
		}
		return &Result{Response: fmt.Sprintf("done call %d", c), Agent: "Backend Chief"}, nil
	}

	agents := newMockAgentResolver()
	agents.add("CEO Agent", "CEO", "ceo", "Leads")
	agents.add("Backend Chief", "Backend Lead", "backend", "Backend dev")

	exec := NewChainExecutor(mock, agents, newMockSkillResolver(), ChainConfig{
		Enabled:        true,
		MaxSteps:       3,
		MaxParallel:    5,
		SynthesisAgent: "CEO Agent",
	})

	result, err := exec.ExecuteDynamic(context.Background(), "do many things")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stepExecCount := 0
	for _, s := range result.Steps {
		if strings.HasPrefix(s.Name, "step-") {
			stepExecCount++
		}
	}
	if stepExecCount > 3 {
		t.Errorf("expected at most 3 steps, got %d", stepExecCount)
	}
}

// ─── Tests: Context Cancellation ─────────────────────────────────────────────

func TestChainExecutor_ContextCancellation(t *testing.T) {
	mock := newMockOrchestrator()

	mock.executeFn = func(ctx context.Context, _ *Request) (*Result, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return &Result{Response: "done", Agent: "Test"}, nil
		}
	}

	exec := newTestChainExecutor(mock)

	ctx, cancel := context.WithCancel(context.Background())

	def := ChainDefinition{
		Name: "cancel-test",
		Steps: []ChainStep{
			{Name: "s1", Agent: "Backend Chief", Prompt: "task1"},
			{Name: "s2", Agent: "Backend Chief", Prompt: "task2"},
			{Name: "s3", Agent: "Backend Chief", Prompt: "task3"},
		},
		Parallel: false,
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	result, err := exec.Execute(ctx, def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cancelledCount := 0
	for _, s := range result.Steps {
		if !s.Success && strings.Contains(s.Error, "cancelled") {
			cancelledCount++
		}
	}
	if cancelledCount == 0 {
		t.Error("expected at least one step to be cancelled")
	}
}

// ─── Tests: Parallel Context Cancellation ────────────────────────────────────

func TestChainExecutor_ParallelContextCancellation(t *testing.T) {
	mock := newMockOrchestrator()

	mock.executeFn = func(ctx context.Context, _ *Request) (*Result, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
			return &Result{Response: "done", Agent: "Test"}, nil
		}
	}

	exec := newTestChainExecutor(mock)

	ctx, cancel := context.WithCancel(context.Background())

	def := ChainDefinition{
		Name: "parallel-cancel",
		Steps: []ChainStep{
			{Name: "a", Agent: "Backend Chief", Prompt: "a"},
			{Name: "b", Agent: "Database Chief", Prompt: "b"},
			{Name: "c", Agent: "Frontend Chief", Prompt: "c"},
		},
		Parallel: true,
	}

	go func() {
		cancel()
	}()

	result, err := exec.Execute(ctx, def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, s := range result.Steps {
		if s.Success {
			t.Logf("step %s succeeded before cancellation", s.Name)
		}
	}
	_ = result
}

// ─── Tests: ExecuteParallel ──────────────────────────────────────────────────

func TestChainExecutor_ExecuteParallelMethod(t *testing.T) {
	mock := newMockOrchestrator()

	var mu sync.Mutex
	var calledAgents []string

	mock.executeFn = func(_ context.Context, req *Request) (*Result, error) {
		agent := "CEO Agent"
		if req.Context != nil {
			if a, ok := req.Context["agent"].(string); ok {
				agent = a
			}
		}
		mu.Lock()
		calledAgents = append(calledAgents, agent)
		mu.Unlock()

		time.Sleep(20 * time.Millisecond)
		return &Result{Response: fmt.Sprintf("[%s] answer", agent), Agent: agent}, nil
	}

	exec := newTestChainExecutor(mock)

	result, err := exec.ExecuteParallel(context.Background(), "solve this",
		[]string{"Backend Chief", "Database Chief", "Frontend Chief"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result.Steps))
	}

	for _, s := range result.Steps {
		if !s.Success {
			t.Errorf("step %s should succeed", s.Name)
		}
	}

	mu.Lock()
	if len(calledAgents) != 3 {
		t.Errorf("expected 3 agents called, got %d", len(calledAgents))
	}
	mu.Unlock()

	if !strings.Contains(result.Response, "Backend Chief") {
		t.Error("response should contain output from Backend Chief")
	}
	if !strings.Contains(result.Response, "Database Chief") {
		t.Error("response should contain output from Database Chief")
	}
}

func TestChainExecutor_ExecuteParallelEmpty(t *testing.T) {
	mock := newMockOrchestrator()
	exec := newTestChainExecutor(mock)

	result, err := exec.ExecuteParallel(context.Background(), "test", []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Response == "" {
		t.Error("should return a message for empty agent list")
	}
}

// ─── Tests: No Orchestrator ──────────────────────────────────────────────────

func TestChainExecutor_NoOrchestrator(t *testing.T) {
	exec := &ChainExecutor{
		engine: nil,
		config: DefaultChainConfig(),
	}

	_, err := exec.Execute(context.Background(), ChainDefinition{
		Name:  "test",
		Steps: []ChainStep{{Name: "s1", Agent: "X", Prompt: "y"}},
	})
	if err == nil {
		t.Fatal("expected error when no orchestrator configured")
	}
}

// ─── Tests: Parse Decomposition ──────────────────────────────────────────────

func TestChainExecutor_ParseDecomposition(t *testing.T) {
	mock := newMockOrchestrator()
	exec := newTestChainExecutor(mock)

	tests := []struct {
		name      string
		response  string
		wantSteps int
		err       bool
	}{
		{name: "valid json", response: `[{"agent": "Backend Chief", "prompt": "design API"}]`, wantSteps: 1},
		{name: "multiple steps", response: `[{"agent": "Backend Chief", "prompt": "design"}, {"agent": "Database Chief", "prompt": "schema"}]`, wantSteps: 2},
		{name: "with markdown fences", response: "```json\n[{\"agent\": \"Backend Chief\", \"prompt\": \"design API\"}]\n```", wantSteps: 1},
		{name: "with surrounding text", response: "Here is the decomposition:\n\n[{\"agent\": \"Backend Chief\", \"prompt\": \"design\"}]\n\nHope this helps.", wantSteps: 1},
		{name: "empty response", response: "", err: true},
		{name: "no json", response: "just some text without array", err: true},
		{name: "malformed json", response: `[{"agent": "Backend Chief", "prompt": "design"`, err: true},
		{name: "skip empty entries", response: `[{"agent": "", "prompt": ""}, {"agent": "Backend Chief", "prompt": "design"}]`, wantSteps: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps, err := exec.parseDecomposition(tt.response)
			if tt.err {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(steps) != tt.wantSteps {
				t.Errorf("expected %d steps, got %d", tt.wantSteps, len(steps))
			}
		})
	}
}

// ─── Tests: Decompose Prompt ─────────────────────────────────────────────────

func TestChainExecutor_DecomposePrompt(t *testing.T) {
	mock := newMockOrchestrator()
	exec := newTestChainExecutor(mock)

	prompt := exec.decomposePrompt("build a user management system")

	if !strings.Contains(prompt, "build a user management system") {
		t.Error("decompose prompt should contain original request")
	}
	if !strings.Contains(prompt, "Backend Chief") {
		t.Error("decompose prompt should list available agents")
	}
	if !strings.Contains(prompt, "Database Chief") {
		t.Error("decompose prompt should list Database Chief")
	}
	if !strings.Contains(prompt, "JSON") {
		t.Error("decompose prompt should request JSON format")
	}
}

func TestChainExecutor_DecomposePrompt_NoAgents(t *testing.T) {
	exec := &ChainExecutor{
		agents: nil,
		config: DefaultChainConfig(),
	}

	prompt := exec.decomposePrompt("test request")

	if !strings.Contains(prompt, "test request") {
		t.Error("should contain original request")
	}
	if !strings.Contains(prompt, "No agent resolver configured") {
		t.Error("should indicate no agent resolver")
	}
}

func TestChainExecutor_DecomposePrompt_Skills(t *testing.T) {
	mock := newMockOrchestrator()
	exec := newTestChainExecutor(mock)

	skills := newMockSkillResolver()
	skills.add("code-review", "Reviews Go code for correctness, style, and security issues.", "review")
	skills.add("sql-tuning", "Tunes slow SQLite queries using EXPLAIN QUERY PLAN and index analysis.", "database")
	exec.skills = skills

	prompt := exec.decomposePrompt("review the new API code")

	if !strings.Contains(prompt, "=== AVAILABLE SKILLS ===") {
		t.Error("decompose prompt should include the AVAILABLE SKILLS section")
	}
	if !strings.Contains(prompt, "- code-review: Reviews Go code for correctness, style, and security issues.") {
		t.Error("decompose prompt should list the code-review skill with its description")
	}
	if !strings.Contains(prompt, "- sql-tuning: Tunes slow SQLite queries using EXPLAIN QUERY PLAN and index analysis.") {
		t.Error("decompose prompt should list the sql-tuning skill with its description")
	}
}

func TestChainExecutor_DecomposePrompt_SkillsLongDescription(t *testing.T) {
	mock := newMockOrchestrator()
	exec := newTestChainExecutor(mock)

	longDesc := strings.Repeat("x", 500) + "\nsecond line"
	skills := newMockSkillResolver()
	skills.add("verbose", longDesc, "misc")
	exec.skills = skills

	prompt := exec.decomposePrompt("test request")

	if !strings.Contains(prompt, "- verbose: "+truncateString(strings.Repeat("x", 500)+" second line", 140)) {
		t.Error("skill description should be flattened to one line and truncated to 140 chars")
	}
	if strings.Contains(prompt, "second line\n") {
		t.Error("skill description should not leak a raw newline")
	}
}

func TestChainExecutor_DecomposePrompt_NoSkills(t *testing.T) {
	exec := &ChainExecutor{
		agents: nil,
		skills: newMockSkillResolver(),
		config: DefaultChainConfig(),
	}

	prompt := exec.decomposePrompt("test request")

	if strings.Contains(prompt, "=== AVAILABLE SKILLS ===") {
		t.Error("empty skill resolver should not add the AVAILABLE SKILLS section")
	}
	if !strings.Contains(prompt, "test request") {
		t.Error("should contain original request")
	}
}

// ─── Tests: Build Step Context ───────────────────────────────────────────────

func TestChainExecutor_BuildStepContext(t *testing.T) {
	mock := newMockOrchestrator()
	exec := newTestChainExecutor(mock)

	completed := []ChainStepResult{
		{Name: "step1", Agent: "Backend Chief", Success: true, Output: "output1"},
		{Name: "step2", Agent: "Database Chief", Success: true, Output: "output2"},
		{Name: "step3", Agent: "Frontend Chief", Success: false, Error: "failed"},
	}

	step := ChainStep{
		Name:      "step4",
		Agent:     "Testing Chief",
		Prompt:    "test",
		DependsOn: []string{"step1", "step3"},
	}

	ctxMap := exec.buildStepContext(step, completed)

	if agent, ok := ctxMap["agent"].(string); !ok || agent != "Testing Chief" {
		t.Errorf("expected agent 'Testing Chief', got %v", ctxMap["agent"])
	}

	if out, ok := ctxMap["step1_output"]; !ok {
		t.Error("expected step1_output in context")
	} else if out != "output1" {
		t.Errorf("expected 'output1', got %v", out)
	}

	if _, ok := ctxMap["step3_output"]; ok {
		t.Error("step3 failed, so its output should NOT be in context")
	}
}

// ─── Tests: Duration Tracking ────────────────────────────────────────────────

func TestChainResult_Duration(t *testing.T) {
	mock := newMockOrchestrator()
	// Give the step measurable wall-clock time: on coarse clocks (Windows
	// time.Now() granularity ~0.5ms) an instant step yields a 0s chain duration.
	mock.executeFn = func(_ context.Context, req *Request) (*Result, error) {
		time.Sleep(2 * time.Millisecond)
		return &Result{ID: req.ID, Response: "ok", Agent: "Backend Chief", Duration: 2 * time.Millisecond}, nil
	}
	exec := newTestChainExecutor(mock)

	result, err := exec.Execute(context.Background(), ChainDefinition{
		Name: "duration-test",
		Steps: []ChainStep{
			{Name: "s1", Agent: "Backend Chief", Prompt: "task"},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Duration <= 0 {
		t.Errorf("expected positive duration, got %v", result.Duration)
	}
	if result.RequestID == "" {
		t.Error("expected non-empty request ID")
	}
}

// ─── Tests: Dynamic Fallback on Bad Decomposition ────────────────────────────

func TestChainExecutor_DynamicFallback(t *testing.T) {
	mock := newMockOrchestrator()

	var callCount int64
	mock.executeFn = func(_ context.Context, _ *Request) (*Result, error) {
		c := atomic.AddInt64(&callCount, 1)
		if c == 1 {
			return &Result{Response: "I think you should use the Backend Chief for this.", Agent: "CEO Agent"}, nil
		}
		return &Result{Response: "CEO handled it directly.", Agent: "CEO Agent"}, nil
	}

	agents := newMockAgentResolver()
	agents.add("CEO Agent", "CEO", "ceo", "Handles everything")
	agents.add("Backend Chief", "Backend Lead", "backend", "Backend dev")

	exec := NewChainExecutor(mock, agents, newMockSkillResolver(), ChainConfig{
		Enabled:        true,
		MaxSteps:       5,
		MaxParallel:    3,
		SynthesisAgent: "CEO Agent",
	})

	result, err := exec.ExecuteDynamic(context.Background(), "build something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Response == "" {
		t.Error("expected non-empty response from fallback")
	}
	if !strings.Contains(result.Response, "CEO") {
		t.Errorf("expected CEO fallback response, got: %s", result.Response)
	}
}

// ─── Tests: extractJSONArray ─────────────────────────────────────────────────

func TestExtractJSONArray(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "simple", input: `[{"a": 1}]`, expect: `[{"a": 1}]`},
		{name: "with text before", input: `here is the plan: [{"a": 1}]`, expect: `[{"a": 1}]`},
		{name: "with text after", input: `[{"a": 1}] and that's it`, expect: `[{"a": 1}]`},
		{name: "markdown fence", input: "```json\n[{\"a\": 1}]\n```", expect: "[{\"a\": 1}]"},
		{name: "nested arrays", input: `[{"prompt": "use [1,2,3] for the design"}]`, expect: `[{"prompt": "use [1,2,3] for the design"}]`},
		{name: "no brackets", input: `just text`, expect: ""},
		{name: "unbalanced brackets", input: `[{"a": 1}`, expect: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSONArray(tt.input)
			if result != tt.expect {
				t.Errorf("expected %q, got %q", tt.expect, result)
			}
		})
	}
}

// ─── Tests: Default Chain Config ─────────────────────────────────────────────

func TestDefaultChainConfig(t *testing.T) {
	cfg := DefaultChainConfig()

	if cfg.Enabled {
		t.Error("expected Enabled=false by default")
	}
	if cfg.MaxSteps != 5 {
		t.Errorf("expected MaxSteps=5, got %d", cfg.MaxSteps)
	}
	if cfg.MaxParallel != runtime.NumCPU() {
		t.Errorf("expected MaxParallel=%d, got %d", runtime.NumCPU(), cfg.MaxParallel)
	}
	if cfg.SynthesisAgent != "COSCA KERNEL" {
		t.Errorf("expected SynthesisAgent='COSCA KERNEL', got %q", cfg.SynthesisAgent)
	}
}

// ─── Tests: NewChainExecutor Defaults ────────────────────────────────────────

func TestNewChainExecutor_DefaultsApplied(t *testing.T) {
	exec := NewChainExecutor(nil, nil, nil, ChainConfig{})
	if exec.config.MaxSteps != 5 {
		t.Errorf("expected MaxSteps=5, got %d", exec.config.MaxSteps)
	}
	if exec.config.MaxParallel != 3 {
		t.Errorf("expected MaxParallel=3 (floor), got %d", exec.config.MaxParallel)
	}
	if exec.config.SynthesisAgent != "CEO Agent" {
		t.Errorf("expected SynthesisAgent='CEO Agent', got %q", exec.config.SynthesisAgent)
	}
}

// ─── Tests: Parallelism Limiting ─────────────────────────────────────────────

func TestChainExecutor_ParallelismLimit(t *testing.T) {
	mock := newMockOrchestrator()

	var maxConcurrent int32
	var current int32

	mock.executeFn = func(_ context.Context, _ *Request) (*Result, error) {
		cur := atomic.AddInt32(&current, 1)
		for {
			mc := atomic.LoadInt32(&maxConcurrent)
			if cur > mc {
				if atomic.CompareAndSwapInt32(&maxConcurrent, mc, cur) {
					break
				}
			} else {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		return &Result{Response: "ok", Agent: "Test"}, nil
	}

	exec := NewChainExecutor(mock, newMockAgentResolver(), newMockSkillResolver(), ChainConfig{
		Enabled:        true,
		MaxSteps:       10,
		MaxParallel:    2,
		SynthesisAgent: "CEO Agent",
	})

	def := ChainDefinition{
		Name: "parallel-limit",
		Steps: []ChainStep{
			{Name: "a", Agent: "Backend Chief", Prompt: "a"},
			{Name: "b", Agent: "Backend Chief", Prompt: "b"},
			{Name: "c", Agent: "Backend Chief", Prompt: "c"},
			{Name: "d", Agent: "Backend Chief", Prompt: "d"},
		},
		Parallel: true,
	}

	_, err := exec.Execute(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&maxConcurrent) > 2 {
		t.Errorf("expected max %d concurrent, got %d", 2, maxConcurrent)
	}
}

// ─── Tests: ExecuteChain on Engine ───────────────────────────────────────────

func TestEngine_ExecuteChain_NotConfigured(t *testing.T) {
	eng := &Engine{chainExecutor: nil}

	_, err := eng.ExecuteChain(context.Background(), ChainDefinition{Name: "test"})
	if err == nil {
		t.Fatal("expected error when chain executor not configured")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestEngine_ExecuteChainDynamic_NotConfigured(t *testing.T) {
	eng := &Engine{chainExecutor: nil}

	_, err := eng.ExecuteChainDynamic(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("expected error when chain executor not configured")
	}
}

func TestEngine_ExecuteChainParallel_NotConfigured(t *testing.T) {
	eng := &Engine{chainExecutor: nil}

	_, err := eng.ExecuteChainParallel(context.Background(), "test", []string{"X"})
	if err == nil {
		t.Fatal("expected error when chain executor not configured")
	}
}
