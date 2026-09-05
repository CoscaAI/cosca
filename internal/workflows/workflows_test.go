package workflows

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/durable"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lifecycleLedger struct {
	mu               sync.Mutex
	begin            int
	claim            int
	heartbeat        int
	final            []durable.RunState
	run              durable.Run
	finalCtx         context.Context
	finalCtxErr      error
	heartbeatDone    chan struct{}
	heartbeatStarted chan struct{}
}

func (l *lifecycleLedger) BeginRun(_ context.Context, in durable.BeginInput) (*durable.Run, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.begin++
	l.run = durable.Run{RunID: in.RunID, WorkflowRef: in.WorkflowRef, Lease: in.Lease, Fencing: durable.Fencing{Generation: 1, Token: "token"}}
	return &l.run, nil
}
func (l *lifecycleLedger) Claim(_ context.Context, id, _ string, f durable.Fencing) (*durable.Run, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.claim++
	l.run.RunID, l.run.Fencing = id, durable.Fencing{Generation: f.Generation + 1, Token: "claimed"}
	return &l.run, nil
}
func (l *lifecycleLedger) Heartbeat(ctx context.Context, _ string, _ durable.Fencing) error {
	l.mu.Lock()
	l.heartbeat++
	call := l.heartbeat
	l.mu.Unlock()
	if l.heartbeatStarted != nil && call > 1 {
		select {
		case <-l.heartbeatStarted:
		default:
			close(l.heartbeatStarted)
		}
	}
	if l.heartbeatDone != nil && call > 1 {
		<-ctx.Done()
		select {
		case <-l.heartbeatDone:
		default:
			close(l.heartbeatDone)
		}
		return ctx.Err()
	}
	return nil
}
func (l *lifecycleLedger) Finalize(ctx context.Context, _ string, _ durable.Fencing, s durable.RunState) error {
	l.mu.Lock()
	l.final = append(l.final, s)
	l.finalCtx = ctx
	l.finalCtxErr = ctx.Err()
	l.mu.Unlock()
	return nil
}

func TestDurableRunCancellationStopsHeartbeatAndFinalizesWithCleanupContext(t *testing.T) {
	ledger := &lifecycleLedger{heartbeatDone: make(chan struct{}), heartbeatStarted: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	m := NewManager("", WithDurableRun(DurableRunConfig{Enabled: true, Lease: 30 * time.Millisecond}, ledger))

	done := make(chan error, 1)
	go func() {
		_, err := m.withDurableRun(ctx, "cancelled", func() (*Result, error) {
			<-release // Deliberately ignore ctx until the executor has finished.
			return &Result{Status: "completed"}, nil
		})
		done <- err
	}()

	select {
	case <-ledger.heartbeatStarted:
		// A heartbeat is blocked in the executor-independent worker.
	case <-time.After(time.Second):
		t.Fatal("heartbeat worker did not start")
	}
	cancel()
	select {
	case <-ledger.heartbeatDone:
		// The heartbeat worker observed cancellation while execute was still blocked.
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not observe context cancellation")
	}
	close(release)
	err := <-done
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []durable.RunState{durable.RunFailed}, ledger.final)
	assert.NotNil(t, ledger.finalCtx)
	assert.NoError(t, ledger.finalCtxErr)
}

func TestDurableRunOptInLifecycleAndLegacyCompatibility(t *testing.T) {
	workflow := Workflow{Name: "durable-test", StepList: []Step{{Name: "one"}}, Steps: 1}

	legacy := NewManager("")
	legacy.Add(workflow)
	legacy.SetStepRunner(func(context.Context, Step) (string, error) { return "ok", nil })
	legacyResult, err := legacy.Run(context.Background(), workflow.Name)
	require.NoError(t, err)
	require.Equal(t, "completed", legacyResult.Status)

	disabledLedger := &lifecycleLedger{}
	disabled := NewManager("", WithDurableRun(DurableRunConfig{Enabled: false}, disabledLedger))
	disabled.Add(workflow)
	disabled.SetStepRunner(func(context.Context, Step) (string, error) { return "ok", nil })
	_, err = disabled.Run(context.Background(), workflow.Name)
	require.NoError(t, err)
	assert.Zero(t, disabledLedger.begin)
	assert.Zero(t, disabledLedger.claim)
	assert.Empty(t, disabledLedger.final)

	ledger := &lifecycleLedger{}
	durableManager := NewManager("", WithDurableRun(DurableRunConfig{Enabled: true, WorkerID: "test"}, ledger))
	durableManager.Add(workflow)
	durableManager.SetStepRunner(func(context.Context, Step) (string, error) { return "ok", nil })
	result, err := durableManager.Run(context.Background(), workflow.Name)
	require.NoError(t, err)
	require.Equal(t, legacyResult.Status, result.Status)
	assert.Equal(t, 1, ledger.begin)
	assert.Equal(t, 1, ledger.claim)
	assert.Len(t, ledger.final, 1)
	assert.Equal(t, durable.RunCompleted, ledger.final[0])
}

func TestDurableRunFinalizesFailureWithoutChangingResultContract(t *testing.T) {
	ledger := &lifecycleLedger{}
	m := NewManager("", WithDurableRun(DurableRunConfig{Enabled: true}, ledger))
	m.Add(Workflow{Name: "fails", StepList: []Step{{Name: "one"}}, Steps: 1})
	m.SetStepRunner(func(context.Context, Step) (string, error) { return "", errors.New("step failed") })

	result, err := m.Run(context.Background(), "fails")
	require.NoError(t, err) // existing fallback contract reports status in Result
	require.Equal(t, "failed", result.Status)
	require.Equal(t, []durable.RunState{durable.RunFailed}, ledger.final)
}

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

// mockPipeline captures Execute calls and allows test-controlled behaviour.
type mockPipeline struct {
	mu       sync.Mutex
	executed []orchestration.PipelineDefinition
	// If set, Execute returns this error.
	err error
	// If set, Execute adds these output keys to pc.Data.Extra before returning.
	outputKeys []string
}

func (m *mockPipeline) Execute(ctx context.Context, pc orchestration.PipelineContext, def orchestration.PipelineDefinition) (orchestration.PipelineContext, error) {
	m.mu.Lock()
	m.executed = append(m.executed, def)
	m.mu.Unlock()

	for _, key := range m.outputKeys {
		pc = pc.WithContextData(key, "ok")
	}
	return pc, m.err
}

// mockExecutionStore captures Store calls.
type mockExecutionStore struct {
	mu     sync.Mutex
	stored []*orchestration.Execution
}

func (m *mockExecutionStore) Store(exec *orchestration.Execution) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stored = append(m.stored, exec)
}

// ---------------------------------------------------------------------------
// parseTimeout
// ---------------------------------------------------------------------------

func TestParseTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    string
		want time.Duration
	}{
		{"empty string", "", 0},
		{"seconds", "30s", 30 * time.Second},
		{"minutes", "5m", 5 * time.Minute},
		{"milliseconds", "100ms", 100 * time.Millisecond},
		{"hours", "2h", 2 * time.Hour},
		{"malformed", "not-a-duration", 0},
		{"negative", "-1s", -1 * time.Second}, // ParseDuration accepts negative; parseTimeout passes it through
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseTimeout(tt.s)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// containsFold
// ---------------------------------------------------------------------------

func TestContainsFold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		s, sub string
		want   bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "xyz", false},
		{"", "anything", false},
		{"anything", "", true},
		{"Café", "café", true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("%q in %q", tt.sub, tt.s), func(t *testing.T) {
			t.Parallel()
			got := containsFold(tt.s, strings.ToLower(tt.sub))
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// Manager construction and setters
// ---------------------------------------------------------------------------

func TestManager_Options(t *testing.T) {
	t.Parallel()

	p := &mockPipeline{}
	es := &mockExecutionStore{}

	m := NewManager("", WithPipeline(p), WithExecutionStore(es))

	assert.NotNil(t, m.pipeline, "pipeline should be set")
	assert.NotNil(t, m.executionStore, "executionStore should be set")
}

func TestManager_SetPipeline(t *testing.T) {
	t.Parallel()

	p := &mockPipeline{}
	m := NewManager("")
	require.Nil(t, m.pipeline)

	m.SetPipeline(p)
	assert.NotNil(t, m.pipeline)
}

func TestManager_SetExecutionStore(t *testing.T) {
	t.Parallel()

	es := &mockExecutionStore{}
	m := NewManager("")
	require.Nil(t, m.executionStore)

	m.SetExecutionStore(es)
	assert.NotNil(t, m.executionStore)
}

func TestManager_SetStepRunner(t *testing.T) {
	t.Parallel()

	m := NewManager("")
	require.Nil(t, m.stepRunner)

	runner := func(ctx context.Context, step Step) (string, error) {
		return "ok", nil
	}
	m.SetStepRunner(runner)
	assert.NotNil(t, m.stepRunner)
}

// ---------------------------------------------------------------------------
// Run — fallback path (no pipeline)
// ---------------------------------------------------------------------------

func TestRun_Fallback_WithStepList(t *testing.T) {
	t.Parallel()

	m := emptyManager()
	m.Add(Workflow{
		Name: "test-wf",
		StepList: []Step{
			{Name: "step1", Timeout: "100ms"},
			{Name: "step2", Timeout: "100ms"},
		},
	})

	r, err := m.Run(context.Background(), "test-wf")
	require.NoError(t, err)
	assert.Equal(t, "completed", r.Status)
	assert.Equal(t, 2, r.StepsCompleted)
	assert.Equal(t, 2, r.TotalSteps)
	assert.NotEmpty(t, r.Duration)
}

func TestRun_Fallback_StepRunner(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var executed []Step

	m := emptyManager()
	m.SetStepRunner(func(ctx context.Context, step Step) (string, error) {
		mu.Lock()
		executed = append(executed, step)
		mu.Unlock()
		return "result", nil
	})

	m.Add(Workflow{
		Name: "wf",
		StepList: []Step{
			{Name: "step1"},
			{Name: "step2"},
		},
	})

	r, err := m.Run(context.Background(), "wf")
	require.NoError(t, err)
	assert.Equal(t, "completed", r.Status)
	assert.Equal(t, 2, r.StepsCompleted)
	assert.Len(t, executed, 2)
}

func TestRun_Fallback_StepRunnerError(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var executed []Step

	m := emptyManager()
	m.SetStepRunner(func(ctx context.Context, step Step) (string, error) {
		mu.Lock()
		executed = append(executed, step)
		mu.Unlock()
		if step.Name == "step2" {
			return "", errors.New("step2 failed")
		}
		return "result", nil
	})

	m.Add(Workflow{
		Name: "wf",
		StepList: []Step{
			{Name: "step1"},
			{Name: "step2"},
			{Name: "step3"},
		},
	})

	r, err := m.Run(context.Background(), "wf")
	require.NoError(t, err, "fallback path never returns an error")
	assert.Equal(t, "failed", r.Status)
	assert.Equal(t, 1, r.StepsCompleted)
	assert.Equal(t, 3, r.TotalSteps)
	assert.Len(t, executed, 2)
}

func TestRun_Fallback_ContextCancellation(t *testing.T) {
	t.Parallel()

	m := emptyManager()
	m.Add(Workflow{
		Name: "wf",
		StepList: []Step{
			{Name: "step1", Timeout: "10s"}, // long timeout
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	r, err := m.Run(ctx, "wf")
	require.NoError(t, err)
	assert.Equal(t, "failed", r.Status)
	assert.Equal(t, 0, r.StepsCompleted)
}

func TestRun_Fallback_ContextDeadline(t *testing.T) {
	t.Parallel()

	m := emptyManager()
	m.Add(Workflow{
		Name: "wf",
		StepList: []Step{
			{Name: "step1", Timeout: "10s"}, // long timeout
		},
	})

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	r, err := m.Run(ctx, "wf")
	require.NoError(t, err)
	assert.Equal(t, "failed", r.Status)
	assert.Equal(t, 0, r.StepsCompleted)
}

func TestRun_Fallback_StepTimeout(t *testing.T) {
	t.Parallel()

	start := time.Now()
	m := emptyManager()
	m.Add(Workflow{
		Name: "wf",
		StepList: []Step{
			{Name: "step1", Timeout: "50ms"},
		},
	})

	// The step runner blocks for 200ms, but step timeout is 50ms → should fail.
	m.SetStepRunner(func(ctx context.Context, step Step) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return "result", nil
		}
	})

	r, err := m.Run(context.Background(), "wf")
	require.NoError(t, err)
	assert.Equal(t, "failed", r.Status)
	assert.Equal(t, 0, r.StepsCompleted)
	assert.Less(t, time.Since(start), 300*time.Millisecond, "should not wait for 200ms step")
}

func TestRun_Fallback_NoStepList(t *testing.T) {
	t.Parallel()

	m := emptyManager()
	m.Add(Workflow{Name: "trivial", Steps: 0})

	// A workflow with zero steps is a stub and must NOT report success.
	r, err := m.Run(context.Background(), "trivial")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stub")
	assert.Nil(t, r)
}

func TestRun_Fallback_NoStepListButStepsField(t *testing.T) {
	t.Parallel()

	m := emptyManager()
	// Steps field says 5 but no StepList — nothing executable, so it is a stub.
	m.Add(Workflow{Name: "trivial", Steps: 5})

	_, err := m.Run(context.Background(), "trivial")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stub")
}

// ---------------------------------------------------------------------------
// Run — pipeline path
// ---------------------------------------------------------------------------

func TestRun_Pipeline_Success(t *testing.T) {
	t.Parallel()

	p := &mockPipeline{
		outputKeys: []string{"step_0_output", "step_1_output"},
	}

	m := emptyManager()
	m.SetPipeline(p)
	m.Add(Workflow{
		Name:  "wf",
		Steps: 2,
		StepList: []Step{
			{Name: "step1", Agent: "agent1", Description: "desc1", Timeout: "30s"},
			{Name: "step2", Agent: "agent2", Description: "desc2", Timeout: "1m"},
		},
	})

	r, err := m.Run(context.Background(), "wf")
	require.NoError(t, err)
	assert.Equal(t, "completed", r.Status)
	assert.Equal(t, 2, r.StepsCompleted)
	assert.Equal(t, 2, r.TotalSteps)

	// Verify pipeline was called with correct definition.
	assert.Len(t, p.executed, 1)
	def := p.executed[0]
	assert.Equal(t, "wf", def.Name)
	assert.Len(t, def.Steps, 2)
	assert.Equal(t, "step1", def.Steps[0].Name)
	assert.Equal(t, "agent1", def.Steps[0].Skill)
	assert.Equal(t, "step_0_input", def.Steps[0].Input)
	assert.Equal(t, "step_0_output", def.Steps[0].Output)
	assert.Equal(t, 30*time.Second, def.Steps[0].Timeout)
}

func TestRun_Pipeline_Error(t *testing.T) {
	t.Parallel()

	p := &mockPipeline{
		err:        errors.New("pipeline exploded"),
		outputKeys: []string{"step_0_output"}, // only first step completed
	}

	m := emptyManager()
	m.SetPipeline(p)
	m.Add(Workflow{
		Name:  "wf",
		Steps: 3,
		StepList: []Step{
			{Name: "step1", Agent: "a1"},
			{Name: "step2", Agent: "a2"},
			{Name: "step3", Agent: "a3"},
		},
	})

	r, err := m.Run(context.Background(), "wf")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline exploded")
	assert.Equal(t, "failed", r.Status)
	assert.Equal(t, 3, r.TotalSteps)
	assert.Equal(t, 1, r.StepsCompleted) // only step_0_output was produced
}

func TestRun_Pipeline_ExecutionStore(t *testing.T) {
	t.Parallel()

	p := &mockPipeline{
		outputKeys: []string{"step_0_output"},
	}
	es := &mockExecutionStore{}

	m := emptyManager()
	m.SetPipeline(p)
	m.SetExecutionStore(es)
	m.Add(Workflow{
		Name: "wf",
		StepList: []Step{
			{Name: "step1", Agent: "a1"},
		},
	})

	_, err := m.Run(context.Background(), "wf")
	require.NoError(t, err)

	assert.Len(t, es.stored, 1)
	assert.Equal(t, "wf", es.stored[0].Prompt)
	assert.Equal(t, "workflow", es.stored[0].Agent)
	assert.Equal(t, "completed", es.stored[0].Status)
	// DurationMs could be 0 in very fast tests; just verify it's non-negative.
	assert.GreaterOrEqual(t, es.stored[0].DurationMs, int64(0))
}

func TestRun_Pipeline_ErrorWithStore(t *testing.T) {
	t.Parallel()

	p := &mockPipeline{
		err: errors.New("boom"),
	}
	es := &mockExecutionStore{}

	m := emptyManager()
	m.SetPipeline(p)
	m.SetExecutionStore(es)
	m.Add(Workflow{
		Name: "wf",
		StepList: []Step{
			{Name: "step1", Agent: "a1"},
		},
	})

	_, err := m.Run(context.Background(), "wf")
	require.Error(t, err)

	assert.Len(t, es.stored, 1)
	assert.Equal(t, "failed", es.stored[0].Status)
}

func TestRun_Pipeline_NoExplicitSteps(t *testing.T) {
	t.Parallel()

	// When StepList is empty the workflow is a stub: even with a pipeline
	// configured, Run must refuse to fabricate success.
	p := &mockPipeline{}

	m := emptyManager()
	m.SetPipeline(p)
	m.Add(Workflow{Name: "no-steps", Steps: 3})

	_, err := m.Run(context.Background(), "no-steps")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stub")

	// Pipeline should NOT have been called (empty StepList).
	assert.Empty(t, p.executed)
}

func TestRun_Pipeline_OnlyPipelineNoStore(t *testing.T) {
	t.Parallel()

	// Pipeline set, but no execution store — should not panic.
	p := &mockPipeline{
		outputKeys: []string{"step_0_output"},
	}

	m := emptyManager()
	m.SetPipeline(p)
	// intentionally not setting executionStore
	m.Add(Workflow{
		Name: "wf",
		StepList: []Step{
			{Name: "step1", Agent: "a1"},
		},
	})

	r, err := m.Run(context.Background(), "wf")
	require.NoError(t, err)
	assert.Equal(t, "completed", r.Status)
}

// ---------------------------------------------------------------------------
// countPipelineCompleted
// ---------------------------------------------------------------------------

func TestCountPipelineCompleted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		presentKeys []string // keys in pc.Data.Extra
		totalSteps  int
		want        int
	}{
		{"all completed", []string{"step_0_output", "step_1_output", "step_2_output"}, 3, 3},
		{"first only", []string{"step_0_output"}, 3, 1},
		{"none", []string{}, 3, 0},
		{"first two", []string{"step_0_output", "step_1_output"}, 3, 2},
		{"zero steps", []string{}, 0, 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pc := orchestration.NewPipelineContext("", "")
			for _, key := range tt.presentKeys {
				pc = pc.WithContextData(key, "value")
			}

			got := countPipelineCompleted(pc, tt.totalSteps)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// Run — edge case: Run with pipeline but stepList derived from Steps
// ---------------------------------------------------------------------------

func TestRun_SingleStepPipeline(t *testing.T) {
	t.Parallel()

	p := &mockPipeline{
		outputKeys: []string{"step_0_output"},
	}

	m := emptyManager()
	m.SetPipeline(p)
	m.Add(Workflow{
		Name: "single",
		StepList: []Step{
			{Name: "only-step", Agent: "chief", Timeout: "10s"},
		},
	})

	r, err := m.Run(context.Background(), "single")
	require.NoError(t, err)
	assert.Equal(t, "completed", r.Status)
	assert.Equal(t, 1, r.TotalSteps)
}

// ---------------------------------------------------------------------------
// parseWorkflowContent — YAML frontmatter
// ---------------------------------------------------------------------------

func TestParseWorkflowContent_YAMLFrontmatter(t *testing.T) {
	t.Parallel()

	content := `---
name: my-workflow
description: A workflow defined in YAML frontmatter
version: "2.0"
status: active
enabled: true
inputs:
  - name: repo
    type: string
    required: true
  - name: branch
    type: string
    required: false
outputs:
  - name: result
    type: string
---
# WORKFLOW: my-workflow
> **Version**: 2.0 | **Status**: active
`

	wf, err := parseWorkflowContent(content, "my-workflow.md")
	require.NoError(t, err)
	assert.Equal(t, "my-workflow", wf.Name)
	assert.Equal(t, "A workflow defined in YAML frontmatter", wf.Description)
	assert.Equal(t, "2.0", wf.Version)
	assert.Equal(t, "active", wf.Status)
	assert.True(t, wf.Enabled)
	require.Len(t, wf.Inputs, 2)
	assert.Equal(t, "repo", wf.Inputs[0].Name)
	assert.True(t, wf.Inputs[0].Required)
	assert.Equal(t, "branch", wf.Inputs[1].Name)
	assert.False(t, wf.Inputs[1].Required)
	require.Len(t, wf.Outputs, 1)
	assert.Equal(t, "result", wf.Outputs[0].Name)
}

func TestParseWorkflowContent_YAMLFrontmatter_Inactive(t *testing.T) {
	t.Parallel()

	content := `---
name: disabled-wf
status: inactive
---
nothing
`

	wf, err := parseWorkflowContent(content, "disabled-wf.md")
	require.NoError(t, err)
	assert.Equal(t, "disabled-wf", wf.Name)
	assert.Equal(t, "inactive", wf.Status)
	assert.False(t, wf.Enabled, "inactive status should set Enabled=false")
}

func TestParseWorkflowContent_YAMLFrontmatter_Disabled(t *testing.T) {
	t.Parallel()

	content := `---
name: disabled2-wf
status: disabled
---
`

	wf, err := parseWorkflowContent(content, "disabled2-wf.md")
	require.NoError(t, err)
	assert.Equal(t, "disabled2-wf", wf.Name)
	assert.False(t, wf.Enabled)
}

func TestParseWorkflowContent_YAMLFrontmatter_Malformed(t *testing.T) {
	t.Parallel()

	// Missing closing ---, should fall through to inline parsing.
	content := `---
name: bad-yaml
status: active
# No closing delimiter
`

	wf, err := parseWorkflowContent(content, "fallback-name.md")
	require.NoError(t, err)
	// Should have used inline parsing and derived name from hint.
	assert.Equal(t, "fallback-name", wf.Name)
}

func TestParseWorkflowContent_YAMLFrontmatter_EmptyFile(t *testing.T) {
	t.Parallel()

	wf, err := parseWorkflowContent("", "empty.md")
	require.NoError(t, err)
	assert.Equal(t, "empty", wf.Name)
	assert.True(t, wf.Enabled)
}

func TestParseWorkflowContent_YAMLFrontmatter_MalformedYAML(t *testing.T) {
	t.Parallel()

	content := `---
name: [this is not valid yaml
---
`

	// The YAML unmarshal fails; should fall through to inline.
	wf, err := parseWorkflowContent(content, "bad.md")
	require.NoError(t, err)
	assert.Equal(t, "bad", wf.Name) // derived from nameHint
}

func TestParseWorkflowContent_OnlyInline_NoFrontmatter(t *testing.T) {
	t.Parallel()

	// Note: the table header row is also parsed as data because the parser
	// treats any non-separator table row as a data row. We include only
	// separator + data rows, no header, to keep the test clean.
	content := `# WORKFLOW: Deploy App
> **Version**: 1.0.0 | **Status**: active

## OBJECTIVE
Deploy the latest application version to production

## INPUTS
|------------|--------|----------|----------------|
| env        | string | yes      | Target env     |
| version    | string | no       | Version to use |

## OUTPUTS
|--------------|--------|---------------------|
| deploy_url   | string | Deployment URL      |

## STEPS
### Step 1: Build
- **Chief**: backend-chief
- **Task**: Build the application
- **Timeout**: 30s

### Step 2: Deploy
- **Chief**: devops-chief
- **Task**: Deploy artifacts
- **Timeout**: 2m
`

	wf, err := parseWorkflowContent(content, "deploy-app.md")
	require.NoError(t, err)
	assert.Equal(t, "Deploy App", wf.Name)
	assert.Equal(t, "1.0.0", wf.Version)
	assert.Equal(t, "active", wf.Status)
	assert.Contains(t, wf.Description, "Deploy the latest application version to production")

	// The separator row (|---|---|...) has 0 cells after splitting (all cells are
	// "------" etc.), and isTableSeparator skips it.
	assert.Len(t, wf.Inputs, 2)
	assert.Len(t, wf.Outputs, 1)
	assert.Len(t, wf.StepList, 2)
	assert.Equal(t, 2, wf.Steps)

	// Check first step
	assert.Equal(t, "Build", wf.StepList[0].Name)
	assert.Equal(t, "backend-chief", wf.StepList[0].Agent)
	assert.Equal(t, "Build the application", wf.StepList[0].Description)
	assert.Equal(t, "30s", wf.StepList[0].Timeout)

	// Check second step
	assert.Equal(t, "Deploy", wf.StepList[1].Name)
	assert.Equal(t, "devops-chief", wf.StepList[1].Agent)
}

func TestParseWorkflowContent_Inline_NoWorkflowTitle(t *testing.T) {
	t.Parallel()

	// No # WORKFLOW: title, should derive from nameHint.
	content := `> **Version**: 1.0.0 | **Status**: active

## OBJECTIVE
Some description
`

	wf, err := parseWorkflowContent(content, "derived-name.md")
	require.NoError(t, err)
	assert.Equal(t, "derived-name", wf.Name)
	assert.Equal(t, "1.0.0", wf.Version)
}

func TestParseWorkflowContent_Inline_InfoLineWithoutSpace(t *testing.T) {
	t.Parallel()

	content := `>**Version**: 3.0.0 | **Status**: beta
`

	wf, err := parseWorkflowContent(content, "info.md")
	require.NoError(t, err)
	assert.Equal(t, "3.0.0", wf.Version)
	assert.Equal(t, "beta", wf.Status)
}

func TestParseWorkflowContent_Inline_NameFromWorkflowTitle(t *testing.T) {
	t.Parallel()

	content := `# WORKFLOW: Named Explicitly
## OBJECTIVE
Do things.
`

	wf, err := parseWorkflowContent(content, "fallback.md")
	require.NoError(t, err)
	assert.Equal(t, "Named Explicitly", wf.Name)
}

// ---------------------------------------------------------------------------
// parseInfoLine edge cases
// ---------------------------------------------------------------------------

func TestParseInfoLine_WithoutLeadingSpace(t *testing.T) {
	t.Parallel()

	wf := &Workflow{}
	parseInfoLine(">**Version**: 2.3.4 | **Status**: stable", wf)
	assert.Equal(t, "2.3.4", wf.Version)
	assert.Equal(t, "stable", wf.Status)
}

func TestParseInfoLine_NoVersion(t *testing.T) {
	t.Parallel()

	wf := &Workflow{}
	parseInfoLine("> **Status**: beta", wf)
	assert.Equal(t, "", wf.Version)
	assert.Equal(t, "beta", wf.Status)
}

func TestParseInfoLine_EmptyStatusValue(t *testing.T) {
	t.Parallel()

	wf := &Workflow{Status: "active"}
	parseInfoLine("> **Version**: 1.0 | **Status**: ", wf)
	assert.Equal(t, "1.0", wf.Version)
	assert.Equal(t, "active", wf.Status) // unchanged since status value was empty
}

// ---------------------------------------------------------------------------
// extractStepName edge cases
// ---------------------------------------------------------------------------

func TestExtractStepName_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line string
		want string
	}{
		{"### Step 1: Setup Environment", "Setup Environment"},
		{"### Step 2: Run", "Run"},
		{"#### Step 1.1: Sub Step", "Sub Step"},
		// No colon in step heading
		{"### Step 1 Build", "Build"},
		// No space after "Step N" and no colon
		{"### Step 1", ""},
		// Not a step heading
		{"### Not a step", ""},
		// Empty
		{"", ""},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.line, func(t *testing.T) {
			t.Parallel()
			got := extractStepName(tt.line)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// parseStepDetailLine — multiple calls with same field
// ---------------------------------------------------------------------------

func TestParseStepDetailLine_DuplicateFields(t *testing.T) {
	t.Parallel()

	step := &Step{}
	parseStepDetailLine("- **Chief**: chief1", step)
	assert.Equal(t, "chief1", step.Agent)

	// Second Chief should NOT overwrite the first.
	parseStepDetailLine("- **Chief**: chief2", step)
	assert.Equal(t, "chief1", step.Agent)

	// Second Task should NOT overwrite the first.
	parseStepDetailLine("- **Task**: task1", step)
	assert.Equal(t, "task1", step.Description)
	parseStepDetailLine("- **Task**: task2", step)
	assert.Equal(t, "task1", step.Description)

	// Second Timeout should NOT overwrite the first.
	parseStepDetailLine("- **Timeout**: 10s", step)
	assert.Equal(t, "10s", step.Timeout)
	parseStepDetailLine("- **Timeout**: 60s", step)
	assert.Equal(t, "10s", step.Timeout)
}

// ---------------------------------------------------------------------------
// parseInputRow edge cases
// ---------------------------------------------------------------------------

func TestParseInputRow_EdgeCases(t *testing.T) {
	t.Parallel()

	// Too few cells
	assert.Nil(t, parseInputRow("| a |"))
	assert.Nil(t, parseInputRow("| a | b |"))

	// Required = false (explicit "no")
	input := parseInputRow("| name | string | no | desc |")
	require.NotNil(t, input)
	assert.False(t, input.Required)

	// Required = "yes" case-insensitive
	input = parseInputRow("| name | string | YES | desc |")
	require.NotNil(t, input)
	assert.True(t, input.Required)
}

// ---------------------------------------------------------------------------
// parseOutputRow edge cases
// ---------------------------------------------------------------------------

func TestParseOutputRow_EdgeCases(t *testing.T) {
	t.Parallel()

	// Too few cells
	assert.Nil(t, parseOutputRow("| a |"))

	// Normal case
	output := parseOutputRow("| url | string |")
	require.NotNil(t, output)
	assert.Equal(t, "url", output.Name)
	assert.Equal(t, "string", output.Type)
	assert.False(t, output.Required) // outputs are never required
}

// ---------------------------------------------------------------------------
// Workflow struct validation
// ---------------------------------------------------------------------------

func TestWorkflowStruct_DefaultValues(t *testing.T) {
	t.Parallel()

	wf := Workflow{}
	assert.False(t, wf.Enabled)
	assert.Empty(t, wf.Status)
	assert.Zero(t, wf.Steps)
	assert.Nil(t, wf.StepList)
}

func TestWorkflowStruct_StatusInactive(t *testing.T) {
	t.Parallel()

	wf := Workflow{Name: "w", Enabled: true, Status: "inactive"}
	// The Enabled flag reflects user intent; parseWorkflowContent handles
	// setting Enabled=false when Status is "inactive" or "disabled".
	// Direct struct creation should keep whatever was set.
	assert.True(t, wf.Enabled)
	assert.Equal(t, "inactive", wf.Status)
}

// ---------------------------------------------------------------------------
// parseWorkflowContent — name derivation
// ---------------------------------------------------------------------------

func TestParseWorkflowContent_NameDerivation(t *testing.T) {
	t.Parallel()

	// Name hint is everything before .md
	wf, err := parseWorkflowContent("some content", "cool-workflow.md")
	require.NoError(t, err)
	assert.Equal(t, "cool-workflow", wf.Name)

	// No .md suffix
	wf, err = parseWorkflowContent("content", "workflow-name.txt")
	require.NoError(t, err)
	assert.Equal(t, "workflow-name.txt", wf.Name)
}

func TestParseWorkflowContent_NoNameAtAll(t *testing.T) {
	t.Parallel()

	// Empty nameHint + no content → error
	_, err := parseWorkflowContent("", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has no name")
}

// ---------------------------------------------------------------------------
// parseYAMLFrontmatter edge cases
// ---------------------------------------------------------------------------

func TestParseYAMLFrontmatter_TooFewLines(t *testing.T) {
	t.Parallel()

	wf := &Workflow{Name: "before"}
	parseYAMLFrontmatter([]string{"---"}, wf)
	assert.Equal(t, "before", wf.Name, "should not change when too few lines")

	wf2 := &Workflow{Name: "before2"}
	parseYAMLFrontmatter([]string{}, wf2)
	assert.Equal(t, "before2", wf2.Name)
}

func TestParseYAMLFrontmatter_NoClosingDelimiter(t *testing.T) {
	t.Parallel()

	wf := &Workflow{Name: "before"}
	parseYAMLFrontmatter([]string{"---", "name: no-close"}, wf)
	assert.Equal(t, "before", wf.Name, "no closing delimiter, should not parse")
}

func TestParseYAMLFrontmatter_FirstLineNotDelimiter(t *testing.T) {
	t.Parallel()

	wf := &Workflow{Name: "before"}
	parseYAMLFrontmatter([]string{"name: value", "---", "---"}, wf)
	assert.Equal(t, "before", wf.Name, "first line not ---, should skip")
}

// ---------------------------------------------------------------------------
// splitTableRow edge cases
// ---------------------------------------------------------------------------

func TestSplitTableRow_EdgeCases(t *testing.T) {
	t.Parallel()

	// Empty string with pipe returns slice with empty cell
	cells := splitTableRow("|")
	assert.Len(t, cells, 1)
	assert.Equal(t, "", cells[0])

	// Just pipes: splitTableRow trims both pipes, leaving empty string → [""]
	cells = splitTableRow("||")
	assert.Len(t, cells, 1)
	assert.Equal(t, "", cells[0])

	// Single cell
	cells = splitTableRow("| only |")
	assert.Len(t, cells, 1)
	assert.Equal(t, "only", cells[0])

	// No trailing pipe
	cells = splitTableRow("| a | b")
	assert.Len(t, cells, 2)
	assert.Equal(t, "a", cells[0])
	assert.Equal(t, "b", cells[1])

	// With extra whitespace
	cells = splitTableRow(" |  a  |  b  | ")
	assert.Len(t, cells, 2)
	assert.Equal(t, "a", cells[0])
	assert.Equal(t, "b", cells[1])
}

// ---------------------------------------------------------------------------
// isTableSeparator edge cases
// ---------------------------------------------------------------------------

func TestIsTableSeparator_EdgeCases(t *testing.T) {
	t.Parallel()

	assert.True(t, isTableSeparator("| :--- | ---: | :---: |"))
	assert.True(t, isTableSeparator("|---|"))
	assert.False(t, isTableSeparator(""))        // no pipe
	assert.False(t, isTableSeparator("| |"))     // no dash
	assert.False(t, isTableSeparator("| -a- |")) // contains letter
}

// ---------------------------------------------------------------------------
// isTableRow edge cases
// ---------------------------------------------------------------------------

func TestIsTableRow_EdgeCases(t *testing.T) {
	t.Parallel()

	assert.True(t, isTableRow("|"))
	assert.False(t, isTableRow(""))
	assert.False(t, isTableRow(" leading | pipe"))
}

// ---------------------------------------------------------------------------
// cleanDescription edge cases
// ---------------------------------------------------------------------------

func TestCleanDescription_EdgeCases(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "plain", cleanDescription("**plain**"))
	assert.Equal(t, "bold", cleanDescription("**bold"))
	assert.Equal(t, "bold", cleanDescription("bold**"))
	assert.Equal(t, "", cleanDescription("**"))
	assert.Equal(t, "", cleanDescription(""))
	// TrimSpace applies before stripping bold markers, so inner spaces survive.
	assert.Equal(t, "spaces", cleanDescription("**spaces**"))
	assert.Equal(t, "spaces", cleanDescription("  **spaces**  "))
}

// ---------------------------------------------------------------------------
// isStepHeading additional cases
// ---------------------------------------------------------------------------

func TestIsStepHeading_Additional(t *testing.T) {
	t.Parallel()

	assert.False(t, isStepHeading(""))
	assert.False(t, isStepHeading("Step 1: something"))
	assert.False(t, isStepHeading("# Step 1: something"))
	assert.False(t, isStepHeading("## Step 1: something"))
	assert.True(t, isStepHeading("  ### Step 42: Hello World  "))
}

// ---------------------------------------------------------------------------
// parseInlineMarkdown — table handling edge cases
// ---------------------------------------------------------------------------

func TestParseInlineMarkdown_SkipsTableSeparator(t *testing.T) {
	t.Parallel()

	// No header row — just separator (skipped) + data (parsed).
	content := `# WORKFLOW: test

## INPUTS
|------|------|----------|
| foo  | str  | yes      |
`

	wf, err := parseWorkflowContent(content, "test.md")
	require.NoError(t, err)
	assert.Len(t, wf.Inputs, 1)
	assert.Equal(t, "foo", wf.Inputs[0].Name)
}

func TestParseInlineMarkdown_StepWithSubDetails(t *testing.T) {
	t.Parallel()

	content := `# WORKFLOW: steps-test

## STEPS
### Step 1: First
- **Chief**: cto
- **Task**: Do first task
- **Timeout**: 30s

### Step 2: Second
- **Chief**: qa-chief
`

	wf, err := parseWorkflowContent(content, "steps.md")
	require.NoError(t, err)
	assert.Len(t, wf.StepList, 2)
	assert.Equal(t, "First", wf.StepList[0].Name)
	assert.Equal(t, "cto", wf.StepList[0].Agent)
	assert.Equal(t, "Do first task", wf.StepList[0].Description)
	assert.Equal(t, "30s", wf.StepList[0].Timeout)

	assert.Equal(t, "Second", wf.StepList[1].Name)
	assert.Equal(t, "qa-chief", wf.StepList[1].Agent)
}

// ---------------------------------------------------------------------------
// parseInlineMarkdown — multiple sections
// ---------------------------------------------------------------------------

func TestParseInlineMarkdown_MultipleSections(t *testing.T) {
	t.Parallel()

	content := `# WORKFLOW: multi-section

## OBJECTIVE
First sentence.
Second sentence with **bold**.

## INPUTS
| name | string | yes |
| desc | string | no  |

## OUTPUTS
| result | json |

## STEPS
### Step 1: Analyze
- **Chief**: analytics-chief

### Step 2: Report
- **Chief**: documentation-chief
`

	wf, err := parseWorkflowContent(content, "multi.md")
	require.NoError(t, err)
	assert.Equal(t, "multi-section", wf.Name)
	assert.Contains(t, wf.Description, "First sentence")
	assert.Contains(t, wf.Description, "Second sentence")
	assert.Len(t, wf.Inputs, 2)
	assert.Len(t, wf.Outputs, 1)
	assert.Len(t, wf.StepList, 2)
	assert.Equal(t, 2, wf.Steps)
}

// ---------------------------------------------------------------------------
// Manager Search — additional cases
// ---------------------------------------------------------------------------

func TestSearch_CaseInsensitiveDescription(t *testing.T) {
	t.Parallel()

	m := emptyManager()
	m.Add(Workflow{Name: "wf1", Description: "DEPLOYMENT of app"})
	m.Add(Workflow{Name: "wf2", Description: "Testing pipeline"})

	results, err := m.Search("deployment")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "wf1", results[0].Name)
}

func TestSearch_NoResults(t *testing.T) {
	t.Parallel()

	m := emptyManager()
	m.Add(Workflow{Name: "wf1"})

	results, err := m.Search("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearch_EmptyQuery(t *testing.T) {
	t.Parallel()

	m := emptyManager()
	m.Add(Workflow{Name: "wf1"})

	results, err := m.Search("")
	require.NoError(t, err)
	assert.Len(t, results, 1) // empty string is substring of everything
}

// ---------------------------------------------------------------------------
// Manager Add — case-insensitive key
// ---------------------------------------------------------------------------

func TestAdd_CaseInsensitiveKey(t *testing.T) {
	t.Parallel()

	m := emptyManager()
	m.Add(Workflow{Name: "MyWorkflow", Description: "first"})
	m.Add(Workflow{Name: "myworkflow", Description: "second"}) // overwrites

	wf, err := m.Get("MYWORKFLOW")
	require.NoError(t, err)
	assert.Equal(t, "second", wf.Description, "second Add should overwrite first")
}

// ---------------------------------------------------------------------------
// Result struct
// ---------------------------------------------------------------------------

func TestResult_Fields(t *testing.T) {
	t.Parallel()

	r := Result{
		Status:         "completed",
		Duration:       "100ms",
		StepsCompleted: 3,
		TotalSteps:     5,
		Outputs:        []IO{{Name: "out1", Type: "string"}},
	}

	assert.Equal(t, "completed", r.Status)
	assert.Equal(t, "100ms", r.Duration)
	assert.Equal(t, 3, r.StepsCompleted)
	assert.Equal(t, 5, r.TotalSteps)
	assert.Len(t, r.Outputs, 1)
}

// ---------------------------------------------------------------------------
// Step struct
// ---------------------------------------------------------------------------

func TestStep_Fields(t *testing.T) {
	t.Parallel()

	s := Step{
		Name:        "Build",
		Description: "Build the app",
		Agent:       "backend-chief",
		Timeout:     "2m",
	}

	assert.Equal(t, "Build", s.Name)
	assert.Equal(t, "Build the app", s.Description)
	assert.Equal(t, "backend-chief", s.Agent)
	assert.Equal(t, "2m", s.Timeout)
}

// ---------------------------------------------------------------------------
// IO struct
// ---------------------------------------------------------------------------

func TestIO_Fields(t *testing.T) {
	t.Parallel()

	io := IO{
		Name:     "version",
		Type:     "string",
		Required: true,
	}

	assert.Equal(t, "version", io.Name)
	assert.Equal(t, "string", io.Type)
	assert.True(t, io.Required)
}

// ---------------------------------------------------------------------------
// Run - pipeline: verify pipeline context has step inputs
// ---------------------------------------------------------------------------

func TestRun_Pipeline_PassesStepInputs(t *testing.T) {
	t.Parallel()

	p := &mockPipeline{
		outputKeys: []string{"step_0_output"},
	}

	m := emptyManager()
	m.SetPipeline(p)
	m.Add(Workflow{
		Name: "wf",
		StepList: []Step{
			{Name: "step1", Agent: "a1", Description: "input description here"},
		},
	})

	_, err := m.Run(context.Background(), "wf")
	require.NoError(t, err)

	// The pipeline was called; we can't inspect PipelineContext from here,
	// but we can verify the definition was correct.
	assert.Len(t, p.executed, 1)
	assert.Equal(t, "step_0_input", p.executed[0].Steps[0].Input)
}

// ---------------------------------------------------------------------------
// NewManager — loadFromEmbed (coverage for the path)
// ---------------------------------------------------------------------------

func TestNewManager_LoadsEmbed(t *testing.T) {
	// Not parallel — may access embed.FS.
	// We just verify construction does not panic.
	m := NewManager("")
	assert.NotNil(t, m)
	assert.NotNil(t, m.workflows)
}

// ---------------------------------------------------------------------------
// Run — nil workflow pointer safety
// ---------------------------------------------------------------------------

func TestRun_WorkflowMapIntegrity(t *testing.T) {
	t.Parallel()

	// Directly test that Run with a nil map entry is unreachable
	// (the map always returns the zero-value and ok=false for missing keys).
	m := emptyManager()
	_, err := m.Run(context.Background(), "not-in-map")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList_ReturnsAllWorkflows(t *testing.T) {
	t.Parallel()

	m := emptyManager()
	m.Add(Workflow{Name: "wf-a", Description: "first"})
	m.Add(Workflow{Name: "wf-b", Description: "second"})

	list := m.List()
	assert.Len(t, list, 2)

	names := make(map[string]bool)
	for _, w := range list {
		names[w.Name] = true
	}
	assert.True(t, names["wf-a"])
	assert.True(t, names["wf-b"])
}

func TestList_EmptyManager(t *testing.T) {
	t.Parallel()

	m := emptyManager()
	list := m.List()
	assert.Empty(t, list)
}

// ---------------------------------------------------------------------------
// parseWorkflowFile — real filesystem
// ---------------------------------------------------------------------------

func TestParseWorkflowFile_ValidFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test-wf.md")

	err := os.WriteFile(path, []byte(`# WORKFLOW: DiskWorkflow

## OBJECTIVE
Loaded from disk.
`), 0o644)
	require.NoError(t, err)

	wf, err := parseWorkflowFile(path)
	require.NoError(t, err)
	assert.Equal(t, "DiskWorkflow", wf.Name)
	assert.Contains(t, wf.Description, "Loaded from disk")
}

func TestParseWorkflowFile_WithFrontmatter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "frontmatter.md")

	err := os.WriteFile(path, []byte(`---
name: FM Workflow
version: "3.0"
status: active
---
`), 0o644)
	require.NoError(t, err)

	wf, err := parseWorkflowFile(path)
	require.NoError(t, err)
	assert.Equal(t, "FM Workflow", wf.Name)
	assert.Equal(t, "3.0", wf.Version)
}

func TestParseWorkflowFile_NonExistent(t *testing.T) {
	t.Parallel()

	_, err := parseWorkflowFile("/nonexistent/path/workflow.md")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// loadFilesFromDir — real filesystem
// ---------------------------------------------------------------------------

func TestLoadFilesFromDir(t *testing.T) {
	// Not parallel — mutates Manager.
	dir := t.TempDir()

	// Create a valid .md file and an invalid one (no name).
	err := os.WriteFile(filepath.Join(dir, "valid.md"), []byte(`# WORKFLOW: Valid
## OBJECTIVE
ok
`), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "invalid.md"), []byte(`no name here`), 0o644)
	require.NoError(t, err)

	// Also create a non-.md file; it should be skipped.
	err = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte(`some text`), 0o644)
	require.NoError(t, err)

	// Create a subdirectory; it should be skipped.
	err = os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	require.NoError(t, err)

	m := emptyManager()
	m.loadFilesFromDir(dir)

	// Only the valid .md should be loaded.
	wf, err := m.Get("Valid")
	require.NoError(t, err)
	assert.Equal(t, "Valid", wf.Name)
	assert.Contains(t, wf.Description, "ok")

	// "invalid" has no name in content and nameHint "invalid" → should be loaded.
	wf2, err := m.Get("invalid")
	require.NoError(t, err)
	assert.Equal(t, "invalid", wf2.Name)
}

func TestLoadFilesFromDir_NonExistent(t *testing.T) {
	// Not parallel — mutates Manager.
	m := emptyManager()
	m.loadFilesFromDir("/nonexistent/dir")
	// Should not panic; just no workflows added.
	assert.Empty(t, m.List())
}

func TestLoadFilesFromDir_UnreadableFile(t *testing.T) {
	// Not parallel — mutates Manager.
	// Permissões POSIX: chmod 0o000 não torna um arquivo ilegível no Windows
	// (o NTFS não implementa permissões de leitura por modo), então o cenário
	// "arquivo ilegível" não existe nesta plataforma.
	if runtime.GOOS == "windows" {
		t.Skip("permissões POSIX (chmod 0o000) não se aplicam no Windows")
	}
	dir := t.TempDir()

	// Create an unreadable .md file. ReadFile will fail, hitting the
	// `if err != nil { continue }` path in loadFilesFromDir.
	badPath := filepath.Join(dir, "unreadable.md")
	err := os.WriteFile(badPath, []byte("content"), 0o000)
	require.NoError(t, err)
	defer os.Chmod(badPath, 0o644) // restore for cleanup

	m := emptyManager()
	m.loadFilesFromDir(dir)
	assert.Empty(t, m.List())
}

// ---------------------------------------------------------------------------
// loadFromDir via NewManager
// ---------------------------------------------------------------------------

func TestNewManager_WithLocalWorkflowsDir(t *testing.T) {
	// Not parallel — uses embed and file I/O.
	coscaDir := t.TempDir()

	// Create a workflows subdirectory.
	wfDir := filepath.Join(coscaDir, "workflows")
	err := os.MkdirAll(wfDir, 0o755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(wfDir, "local.md"), []byte(`# WORKFLOW: LocalWF
## OBJECTIVE
Local workflow.
`), 0o644)
	require.NoError(t, err)

	m := NewManager(coscaDir)

	wf, err := m.Get("LocalWF")
	require.NoError(t, err)
	assert.Contains(t, wf.Description, "Local workflow")
}

func TestNewManager_WithCoscaDirAsWorkflowsDir(t *testing.T) {
	// Not parallel — uses embed and file I/O.
	// When the coscaDir itself has basename "workflows", loadFromDir
	// scans it directly.
	coscaDir := t.TempDir()
	// We can't rename TempDir, but we can create a subdir called "workflows"
	// and pass that as the coscaDir.
	wfDir := filepath.Join(coscaDir, "workflows")
	err := os.MkdirAll(wfDir, 0o755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(wfDir, "direct.md"), []byte(`# WORKFLOW: DirectWF
## OBJECTIVE
Direct scan.
`), 0o644)
	require.NoError(t, err)

	// Pass the workflows dir itself as the coscaDir.
	m := NewManager(wfDir)

	wf, err := m.Get("DirectWF")
	require.NoError(t, err)
	assert.Contains(t, wf.Description, "Direct scan")
}

// ---------------------------------------------------------------------------
// parseInfoLine — leading ">" without space
// ---------------------------------------------------------------------------

func TestParseInfoLine_LeadingGreaterWithoutSpace(t *testing.T) {
	t.Parallel()

	// When the line starts with ">" (no trailing space), the first TrimPrefix
	// (" > ") does not match, but the second (" >") does.
	wf := &Workflow{}
	parseInfoLine(">**Version**: 5.0.0 | **Status**: prod", wf)
	assert.Equal(t, "5.0.0", wf.Version)
	assert.Equal(t, "prod", wf.Status)
}

func TestParseInfoLine_NoGreaterPrefix(t *testing.T) {
	t.Parallel()

	// Line without any ">" prefix still works.
	wf := &Workflow{}
	parseInfoLine("**Version**: 6.0.0 | **Status**: released", wf)
	assert.Equal(t, "6.0.0", wf.Version)
	assert.Equal(t, "released", wf.Status)
}

func TestParseInfoLine_EmptyFieldBetweenPipes(t *testing.T) {
	t.Parallel()

	// Double pipe creates an empty field that should be skipped.
	wf := &Workflow{}
	parseInfoLine("> **Version**: 7.0.0 || **Status**: skipped-field", wf)
	assert.Equal(t, "7.0.0", wf.Version)
	// The empty field is skipped; Status still gets set from the last non-empty field.
	assert.Equal(t, "skipped-field", wf.Status)
}

func emptyManager() *Manager {
	return &Manager{workflows: make(map[string]*Workflow)}
}
