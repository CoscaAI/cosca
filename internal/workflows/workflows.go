// Package workflows provides the workflow management system for Cosca.
// Workflows define structured processes for common tasks.
package workflows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/durable"
	"github.com/CoscaAI/cosca/internal/embed"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/google/uuid"
	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v3"
)

// Workflow represents a structured process with defined steps.
type Workflow struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Version     string `json:"version" yaml:"version"`
	Status      string `json:"status" yaml:"status"`
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	Steps       int    `json:"steps" yaml:"steps"`
	StepList    []Step `json:"step_list,omitempty" yaml:"step_list,omitempty"`
	Inputs      []IO   `json:"inputs" yaml:"inputs"`
	Outputs     []IO   `json:"outputs" yaml:"outputs"`
}

// Step represents a single step in a workflow.
type Step struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Agent       string `json:"agent" yaml:"agent"`
	Timeout     string `json:"timeout" yaml:"timeout"`
}

// IO represents a workflow input or output definition.
type IO struct {
	Name     string `json:"name" yaml:"name"`
	Type     string `json:"type" yaml:"type"`
	Required bool   `json:"required" yaml:"required"`
}

// StepProgress reports the status of a single workflow step during
// streaming execution. It is sent to the caller via a StepProgressFn
// callback before and after each step.
type StepProgress struct {
	StepName   string `json:"step_name"`
	Status     string `json:"status"` // "started", "completed", "failed"
	Output     string `json:"output,omitempty"`
	StepNum    int    `json:"step_num"`
	TotalSteps int    `json:"total_steps"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// StepProgressFn is a callback invoked for each step progress event during
// workflow execution. It is called before a step starts (Status="started")
// and after it completes or fails (Status="completed"/"failed").
type StepProgressFn func(StepProgress)

// MaxProgressOutput limits the Output field in StepProgress to prevent
// excessively large SSE events. Outputs longer than this are truncated.
const MaxProgressOutput = 2048 // 2KB

// Result holds the result of a workflow execution.
type Result struct {
	Status         string `json:"status" yaml:"status"`
	Duration       string `json:"duration" yaml:"duration"`
	StepsCompleted int    `json:"steps_completed" yaml:"steps_completed"`
	TotalSteps     int    `json:"total_steps" yaml:"total_steps"`
	Outputs        []IO   `json:"outputs" yaml:"outputs"`
}

// StepRunner executes a single workflow step. It receives a context (with
// optional deadline) and the step definition and returns the step output.
// When StepRunner is nil, Run simulates step execution using timeouts.
type StepRunner func(ctx context.Context, step Step) (string, error)

// PipelineExecutor abstracts the orchestration Pipeline so the workflows
// package can delegate step execution without depending on the concrete
// Pipeline type. Matches orchestration.Pipeline.Execute.
type PipelineExecutor interface {
	Execute(ctx context.Context, pc orchestration.PipelineContext, def orchestration.PipelineDefinition) (orchestration.PipelineContext, error)
}

// ExecutionStorer abstracts the execution history store so the workflows
// package can persist run results without depending on the concrete
// ExecutionStore type. Matches orchestration.ExecutionStore.Store.
type ExecutionStorer interface {
	Store(execution *orchestration.Execution)
}

// DurableRunConfig controls the optional durable run lifecycle. It deliberately
// contains no request payload: only a hash of the workflow reference is sent to
// the ledger, so prompts and outputs cannot be persisted by this integration.
type DurableRunConfig struct {
	Enabled  bool
	WorkerID string
	Lease    time.Duration
}

// RunLedger is the narrow lifecycle surface used by workflows. The concrete
// durable.Ledger is injected by the application; no singleton is consulted.
type RunLedger interface {
	BeginRun(context.Context, durable.BeginInput) (*durable.Run, error)
	Claim(context.Context, string, string, durable.Fencing) (*durable.Run, error)
	Heartbeat(context.Context, string, durable.Fencing) error
	Finalize(context.Context, string, durable.Fencing, durable.RunState) error
}

const durableCleanupTimeout = 5 * time.Second

// Manager manages workflows in the Cosca system.
type Manager struct {
	mu         sync.RWMutex
	workflows  map[string]*Workflow
	stepRunner StepRunner
	coscaDir   string

	// Optional orchestration integration. When PipelineExecutor is set
	// Run() delegates step execution to the orchestration engine instead
	// of simulating or using the StepRunner. When nil, Run() falls back
	// to the current step-by-step simulation/runner behaviour.
	pipeline       PipelineExecutor
	executionStore ExecutionStorer
	durableConfig  DurableRunConfig
	durableLedger  RunLedger
}

// ManagerOption configures a Manager at construction time.
type ManagerOption func(*Manager)

// WithPipeline sets the orchestration pipeline executor. When set, Run()
// delegates step execution to the pipeline instead of using the StepRunner
// or simulation fallback.
func WithPipeline(p PipelineExecutor) ManagerOption {
	return func(m *Manager) { m.pipeline = p }
}

// WithExecutionStore sets the execution history store. When set, Run()
// persists execution results after every workflow run.
func WithExecutionStore(store ExecutionStorer) ManagerOption {
	return func(m *Manager) { m.executionStore = store }
}

// WithDurableRun enables the opt-in run lifecycle. Disabled (or nil ledger)
// is exactly the legacy execution path.
func WithDurableRun(config DurableRunConfig, ledger RunLedger) ManagerOption {
	return func(m *Manager) {
		m.durableConfig = config
		m.durableLedger = ledger
	}
}

// NewManager creates a new workflow manager and loads workflows from the Cosca
// directory structure. It first loads all built-in workflows from the embedded
// Cosca framework assets, then scans the local <coscaDir>/workflows/ directory
// for any user or project-specific overrides.
//
// Options (WithPipeline, WithExecutionStore) are applied after loading so
// they can override any default behaviour. When no options are supplied the
// manager behaves exactly as it did before the orchestration integration.
func NewManager(coscaDir string, opts ...ManagerOption) *Manager {
	m := &Manager{
		workflows: make(map[string]*Workflow),
		coscaDir:  coscaDir,
	}
	m.loadFromEmbed()
	if coscaDir != "" {
		m.loadFromDir(coscaDir)
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// SetPipeline configures the optional pipeline executor after construction.
// This is an alternative to the WithPipeline option for setups where the
// pipeline is not available at NewManager time.
func (m *Manager) SetPipeline(p PipelineExecutor) {
	m.pipeline = p
}

// SetExecutionStore configures the optional execution store after
// construction.
func (m *Manager) SetExecutionStore(store ExecutionStorer) {
	m.executionStore = store
}

// SetStepRunner configures an optional StepRunner that Run will invoke for
// each step. When nil (the default), Run simulates step execution using
// only the step's timeout for timing.
func (m *Manager) SetStepRunner(runner StepRunner) {
	m.stepRunner = runner
}

// Add adds a workflow to the manager.
func (m *Manager) Add(workflow Workflow) {
	key := strings.ToLower(workflow.Name)
	m.workflows[key] = &workflow
}

// List returns all available workflows.
func (m *Manager) List() []Workflow {
	result := make([]Workflow, 0, len(m.workflows))
	for _, w := range m.workflows {
		result = append(result, *w)
	}
	return result
}

// Get returns a specific workflow by name (case-insensitive).
func (m *Manager) Get(name string) (*Workflow, error) {
	w, ok := m.workflows[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("workflow %q not found", name)
	}
	return w, nil
}

// Run executes a workflow by name (case-insensitive).
//
// Two execution paths are supported:
//
//   - When a PipelineExecutor is configured and the workflow has explicit
//     steps (StepList non-empty), Run converts the steps into an
//     orchestration PipelineDefinition and delegates execution to the
//     pipeline engine. Results are persisted to the ExecutionStorer when
//     one is configured.
//
//   - When no PipelineExecutor is configured (nil), Run falls back to the
//     existing behaviour: steps are executed sequentially with an optional
//     timeout. If a StepRunner is configured it is invoked for every step;
//     otherwise execution is simulated with real duration measurements.
//
// Workflows without explicit steps (StepList empty) are treated as
// trivially successful regardless of the execution path.
func (m *Manager) Run(ctx context.Context, name string) (*Result, error) {
	w, ok := m.workflows[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("workflow %q not found", name)
	}

	return m.withDurableRun(ctx, strings.ToLower(name), func() (*Result, error) {
		return m.run(ctx, w)
	})
}

func (m *Manager) run(ctx context.Context, w *Workflow) (*Result, error) {
	if err := m.stubError(w); err != nil {
		return nil, err
	}

	start := time.Now()
	totalSteps := w.Steps
	if len(w.StepList) > 0 {
		totalSteps = len(w.StepList)
	}

	// Orchestration path: delegate to the pipeline engine.
	if m.pipeline != nil && len(w.StepList) > 0 {
		return m.runWithPipeline(ctx, w, start, totalSteps)
	}

	// Fallback path: step-by-step simulation or StepRunner.
	return m.runFallback(ctx, w, start, totalSteps)
}

// IsStub reports whether a workflow has no executable steps. Stub workflows
// are scaffolding/blueprint documents and cannot be executed. A Steps count
// without a concrete StepList is also a stub — there is nothing to run.
func (m *Manager) IsStub(w *Workflow) bool {
	return w != nil && len(w.StepList) == 0
}

// stubError returns a clear error when the workflow is a non-executable stub.
func (m *Manager) stubError(w *Workflow) error {
	if m.IsStub(w) {
		return fmt.Errorf("workflow %q is a stub (0 steps) — not executable", w.Name)
	}
	return nil
}

// RunWithProgress executes a workflow by name and reports progress via the
// callback for each step. It supports both the pipeline and fallback
// execution paths, calling fn before each step starts and after each step
// completes or fails.
//
// The input parameter is reserved for future use (passing dynamic inputs
// to workflow steps) and is currently ignored.
func (m *Manager) RunWithProgress(ctx context.Context, name string, input map[string]interface{}, fn StepProgressFn) (*Result, error) {
	if fn == nil {
		return m.Run(ctx, name)
	}

	w, ok := m.workflows[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("workflow %q not found", name)
	}

	if err := m.stubError(w); err != nil {
		return nil, err
	}

	return m.withDurableRun(ctx, strings.ToLower(name), func() (*Result, error) {
		start := time.Now()
		totalSteps := w.Steps
		if len(w.StepList) > 0 {
			totalSteps = len(w.StepList)
		}
		if m.pipeline != nil && len(w.StepList) > 0 {
			return m.runWithPipelineProgress(ctx, w, start, totalSteps, fn)
		}
		return m.runFallbackProgress(ctx, w, start, totalSteps, fn)
	})
}

func (m *Manager) withDurableRun(ctx context.Context, workflowRef string, execute func() (*Result, error)) (*Result, error) {
	if !m.durableConfig.Enabled || m.durableLedger == nil {
		return execute()
	}
	worker := m.durableConfig.WorkerID
	if worker == "" {
		worker = "workflow-manager"
	}
	h := sha256.Sum256([]byte(workflowRef))
	run, err := m.durableLedger.BeginRun(ctx, durable.BeginInput{
		RunID: uuid.NewString(), WorkflowRef: workflowRef,
		InputHash: hex.EncodeToString(h[:]), Lease: m.durableConfig.Lease,
	})
	if err != nil {
		return nil, fmt.Errorf("durable begin run: %w", err)
	}
	if err := validateDurableRun(run, "begin"); err != nil {
		return nil, err
	}
	run, err = m.durableLedger.Claim(ctx, run.RunID, worker, run.Fencing)
	if err != nil {
		return nil, fmt.Errorf("durable claim run: %w", err)
	}
	if err := validateDurableRun(run, "claim"); err != nil {
		return nil, err
	}
	// Renew once at the execution boundary. Long-running executions also get
	// periodic renewal below; heartbeat failures are intentionally left to the
	// fencing check at finalization rather than changing the legacy result path.
	var heartbeatErr error
	if err := m.durableLedger.Heartbeat(ctx, run.RunID, run.Fencing); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		heartbeatErr = fmt.Errorf("durable heartbeat run: %w", err)
	}
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	var heartbeatDone sync.WaitGroup
	var heartbeatMu sync.Mutex
	lease := run.Lease
	if lease <= 0 {
		lease = 30 * time.Second
	}
	if lease > 0 {
		heartbeatDone.Add(1)
		go func() {
			defer heartbeatDone.Done()
			interval := lease / 3
			if interval <= 0 {
				interval = time.Second
			}
			t := time.NewTicker(interval)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					if err := m.durableLedger.Heartbeat(heartbeatCtx, run.RunID, run.Fencing); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
						heartbeatMu.Lock()
						heartbeatErr = errors.Join(heartbeatErr, fmt.Errorf("durable heartbeat run: %w", err))
						heartbeatMu.Unlock()
					}
				case <-heartbeatCtx.Done():
					return
				}
			}
		}()
	}
	result, execErr := execute()
	ctxErr := ctx.Err()
	heartbeatCancel()
	heartbeatDone.Wait()
	state := durable.RunCompleted
	if execErr != nil || ctxErr != nil || result == nil || result.Status == "failed" {
		state = durable.RunFailed
	}
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), durableCleanupTimeout)
	finalizeErr := m.durableLedger.Finalize(cleanupCtx, run.RunID, run.Fencing, state)
	cleanupCancel()
	if finalizeErr != nil {
		finalizeErr = fmt.Errorf("durable finalize run: %w", finalizeErr)
	}
	heartbeatMu.Lock()
	joinedHeartbeatErr := heartbeatErr
	heartbeatMu.Unlock()
	return result, errors.Join(execErr, ctxErr, joinedHeartbeatErr, finalizeErr)
}

func validateDurableRun(run *durable.Run, phase string) error {
	if run == nil {
		return fmt.Errorf("durable %s run: ledger returned nil run", phase)
	}
	if run.RunID == "" || run.Fencing.Generation < 1 || run.Fencing.Token == "" {
		return fmt.Errorf("durable %s run: ledger returned invalid run or fencing", phase)
	}
	return nil
}

// runWithPipelineProgress executes the pipeline path with per-step progress
// reporting. Since the pipeline engine batches all steps, we report one
// started event for each step before execution, then completed/failed based
// on the pipeline result.
func (m *Manager) runWithPipelineProgress(ctx context.Context, w *Workflow, start time.Time, totalSteps int, fn StepProgressFn) (*Result, error) {
	steps := make([]orchestration.PipelineStep, 0, len(w.StepList))
	pc := orchestration.NewPipelineContext("", w.Description)

	for i, s := range w.StepList {
		inputKey := fmt.Sprintf("step_%d_input", i)
		outputKey := fmt.Sprintf("step_%d_output", i)
		pc = pc.WithContextData(inputKey, s.Description)

		steps = append(steps, orchestration.PipelineStep{
			Name:    s.Name,
			Skill:   s.Agent,
			Input:   inputKey,
			Output:  outputKey,
			Timeout: parseTimeout(s.Timeout),
		})
	}

	// Report "started" for every step before executing the pipeline.
	for i, s := range w.StepList {
		fn(StepProgress{
			StepName:   s.Name,
			Status:     "started",
			StepNum:    i + 1,
			TotalSteps: totalSteps,
		})
	}

	def := orchestration.PipelineDefinition{
		Name:  w.Name,
		Steps: steps,
	}

	pipeStart := time.Now()
	pc, pipeErr := m.pipeline.Execute(ctx, pc, def)
	pipeDuration := time.Since(pipeStart)

	duration := time.Since(start)

	status := "completed"
	completed := totalSteps
	if pipeErr != nil {
		status = "failed"
		completed = countPipelineCompleted(pc, len(w.StepList))
	}

	// Report completed/failed for each step based on pipeline result.
	for i, s := range w.StepList {
		outputKey := fmt.Sprintf("step_%d_output", i)
		stepCompleted := i < completed

		if stepCompleted {
			output := ""
			if v, ok := pc.Data.Extra[outputKey]; ok {
				output = truncateOutput(fmt.Sprintf("%v", v))
			}
			fn(StepProgress{
				StepName:   s.Name,
				Status:     "completed",
				Output:     output,
				StepNum:    i + 1,
				TotalSteps: totalSteps,
				DurationMs: pipeDuration.Milliseconds() / int64(totalSteps),
			})
		} else {
			errStr := ""
			if pipeErr != nil {
				errStr = pipeErr.Error()
			}
			fn(StepProgress{
				StepName:   s.Name,
				Status:     "failed",
				StepNum:    i + 1,
				TotalSteps: totalSteps,
				Error:      errStr,
			})
		}
	}

	// Persist execution when a store is configured.
	if m.executionStore != nil {
		exec := &orchestration.Execution{
			Prompt:     w.Name,
			Agent:      "workflow",
			Status:     status,
			DurationMs: duration.Milliseconds(),
		}
		m.executionStore.Store(exec)
	}

	return &Result{
		Status:         status,
		Duration:       duration.Round(time.Millisecond).String(),
		StepsCompleted: completed,
		TotalSteps:     totalSteps,
		Outputs:        w.Outputs,
	}, pipeErr
}

// runFallbackProgress implements the step-by-step fallback path with
// per-step progress reporting via the callback.
func (m *Manager) runFallbackProgress(ctx context.Context, w *Workflow, start time.Time, totalSteps int, fn StepProgressFn) (*Result, error) {
	completed := 0
	status := "completed"

	// Workflows without a StepList trivially succeed — report a single step.
	if len(w.StepList) == 0 {
		if totalSteps == 0 {
			totalSteps = 1
		}
		fn(StepProgress{
			StepName:   "workflow:" + w.Name,
			Status:     "started",
			StepNum:    1,
			TotalSteps: totalSteps,
		})
		fn(StepProgress{
			StepName:   "workflow:" + w.Name,
			Status:     "completed",
			StepNum:    1,
			TotalSteps: totalSteps,
			DurationMs: 0,
		})

		duration := time.Since(start)
		return &Result{
			Status:         "completed",
			Duration:       duration.Round(time.Millisecond).String(),
			StepsCompleted: totalSteps,
			TotalSteps:     totalSteps,
			Outputs:        w.Outputs,
		}, nil
	}

	for i, step := range w.StepList {
		stepNum := i + 1

		// Report step started.
		fn(StepProgress{
			StepName:   step.Name,
			Status:     "started",
			StepNum:    stepNum,
			TotalSteps: totalSteps,
		})

		stepStart := time.Now()

		timeout := parseTimeout(step.Timeout)

		stepCtx := ctx
		var cancel context.CancelFunc
		if timeout > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, timeout)
		} else {
			stepCtx, cancel = context.WithCancel(ctx)
		}

		var stepOutput string
		var stepErr error
		stepOK := true

		if m.stepRunner != nil {
			stepOutput, stepErr = m.stepRunner(stepCtx, step)
			if stepErr != nil {
				stepOK = false
			}
		} else {
			// Simulate step execution.
			select {
			case <-stepCtx.Done():
				stepOK = false
				stepErr = stepCtx.Err()
			case <-time.After(10 * time.Millisecond):
				stepOutput = "step completed (simulated)"
			}
		}
		cancel()

		stepDuration := time.Since(stepStart).Milliseconds()

		if stepOK {
			completed++
			fn(StepProgress{
				StepName:   step.Name,
				Status:     "completed",
				Output:     truncateOutput(stepOutput),
				StepNum:    stepNum,
				TotalSteps: totalSteps,
				DurationMs: stepDuration,
			})
		} else {
			status = "failed"
			errStr := ""
			if stepErr != nil {
				errStr = stepErr.Error()
			}
			fn(StepProgress{
				StepName:   step.Name,
				Status:     "failed",
				StepNum:    stepNum,
				TotalSteps: totalSteps,
				DurationMs: stepDuration,
				Error:      errStr,
			})
			break
		}
	}

	duration := time.Since(start)

	return &Result{
		Status:         status,
		Duration:       duration.Round(time.Millisecond).String(),
		StepsCompleted: completed,
		TotalSteps:     totalSteps,
		Outputs:        w.Outputs,
	}, nil
}

// truncateOutput truncates a string to MaxProgressOutput bytes.
func truncateOutput(s string) string {
	if len(s) <= MaxProgressOutput {
		return s
	}
	return s[:MaxProgressOutput] + "..."
}

// runWithPipeline converts the workflow steps into an orchestration
// PipelineDefinition and executes them through the configured pipeline.
// Each step's description is stored as a named context-data input so the
// pipeline can feed the correct input to each skill invocation.
func (m *Manager) runWithPipeline(ctx context.Context, w *Workflow, start time.Time, totalSteps int) (*Result, error) {
	steps := make([]orchestration.PipelineStep, 0, len(w.StepList))
	pc := orchestration.NewPipelineContext("", w.Description)

	for i, s := range w.StepList {
		inputKey := fmt.Sprintf("step_%d_input", i)
		outputKey := fmt.Sprintf("step_%d_output", i)
		pc = pc.WithContextData(inputKey, s.Description)

		steps = append(steps, orchestration.PipelineStep{
			Name:    s.Name,
			Skill:   s.Agent,
			Input:   inputKey,
			Output:  outputKey,
			Timeout: parseTimeout(s.Timeout),
		})
	}

	def := orchestration.PipelineDefinition{
		Name:  w.Name,
		Steps: steps,
	}

	pc, pipeErr := m.pipeline.Execute(ctx, pc, def)

	duration := time.Since(start)

	status := "completed"
	completed := totalSteps
	if pipeErr != nil {
		status = "failed"
		// Count how many steps completed before the failure.
		completed = countPipelineCompleted(pc, len(w.StepList))
	}

	// Persist execution when a store is configured.
	if m.executionStore != nil {
		exec := &orchestration.Execution{
			Prompt:     w.Name,
			Agent:      "workflow",
			Status:     status,
			DurationMs: duration.Milliseconds(),
		}
		m.executionStore.Store(exec)
	}

	return &Result{
		Status:         status,
		Duration:       duration.Round(time.Millisecond).String(),
		StepsCompleted: completed,
		TotalSteps:     totalSteps,
		Outputs:        w.Outputs,
	}, pipeErr
}

// countPipelineCompleted scans the PipelineContext for output keys to
// determine how many steps executed successfully before a pipeline failure.
func countPipelineCompleted(pc orchestration.PipelineContext, totalSteps int) int {
	for i := 0; i < totalSteps; i++ {
		key := fmt.Sprintf("step_%d_output", i)
		if _, ok := pc.Data.Extra[key]; !ok {
			return i // this step did not complete
		}
	}
	return totalSteps
}

// runFallback implements the original step-by-step simulation/StepRunner
// execution path. It is used when no orchestration pipeline is configured.
func (m *Manager) runFallback(ctx context.Context, w *Workflow, start time.Time, totalSteps int) (*Result, error) {
	completed := 0
	status := "completed"

	for _, step := range w.StepList {
		timeout := parseTimeout(step.Timeout)

		stepCtx := ctx
		var cancel context.CancelFunc
		if timeout > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, timeout)
		} else {
			stepCtx, cancel = context.WithCancel(ctx)
		}

		stepOK := true

		if m.stepRunner != nil {
			_, err := m.stepRunner(stepCtx, step)
			if err != nil {
				stepOK = false
			}
		} else {
			// Simulate step execution. When a timeout is configured we
			// honour it so the caller sees realistic step durations;
			// otherwise a minimal tick keeps the loop cheap.
			select {
			case <-stepCtx.Done():
				stepOK = false
			case <-time.After(10 * time.Millisecond):
			}
		}
		cancel()

		if stepOK {
			completed++
		} else {
			status = "failed"
			break
		}
	}

	// Workflows without a StepList trivially succeed.
	if len(w.StepList) == 0 {
		completed = totalSteps
	}

	duration := time.Since(start)

	return &Result{
		Status:         status,
		Duration:       duration.Round(time.Millisecond).String(),
		StepsCompleted: completed,
		TotalSteps:     totalSteps,
		Outputs:        w.Outputs,
	}, nil
}

// parseTimeout converts a human-readable duration string (e.g. "30s", "5m")
// to a time.Duration. Returns 0 when the string is empty or malformed.
func parseTimeout(s string) time.Duration {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

// Search searches for workflows by query string (case-insensitive).
func (m *Manager) Search(query string) ([]Workflow, error) {
	var result []Workflow
	lower := strings.ToLower(query)
	for _, w := range m.workflows {
		if containsFold(w.Name, lower) || containsFold(w.Description, lower) {
			result = append(result, *w)
		}
	}
	return result, nil
}

// containsFold is a case-insensitive substring search (ASCII).
func containsFold(s, substrLower string) bool {
	sLower := strings.ToLower(s)
	return strings.Contains(sLower, substrLower)
}

// loadFromEmbed scans the embedded Cosca framework assets for workflow files
// and registers them with the manager.
func (m *Manager) loadFromEmbed() {
	files, err := embed.ListFilesRecursive("workflows")
	if err != nil {
		return
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") {
			continue
		}
		content, err := embed.ReadString(f)
		if err != nil {
			continue
		}
		wf, err := parseWorkflowContent(content, filepath.Base(f))
		if err != nil {
			continue
		}
		if wf != nil && wf.Name != "" {
			m.Add(*wf)
		}
	}
}

// loadFromDir scans the workflows/ subdirectory for .md files and
// also loads from the global Cosca framework workflows directory.
func (m *Manager) loadFromDir(dir string) {
	// Try local <coscaDir>/workflows/
	localDir := filepath.Join(dir, "workflows")
	if info, err := os.Stat(localDir); err == nil && info.IsDir() {
		m.loadFilesFromDir(localDir)
	}

	// If coscaDir itself is a workflows directory, scan it directly
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		if base := filepath.Base(dir); base == "workflows" {
			m.loadFilesFromDir(dir)
		}
	}

	// Try global Cosca framework workflows at ~/.config/opencode/cosca/workflows/
	home, err := homedir.Dir()
	if err == nil {
		globalDir := filepath.Join(home, ".config", "opencode", "cosca", "workflows")
		if info, err := os.Stat(globalDir); err == nil && info.IsDir() {
			if globalDir != localDir && globalDir != dir {
				m.loadFilesFromDir(globalDir)
			}
		}
	}
}

// loadFilesFromDir reads all .md files from a directory and registers
// them as workflows after parsing.
func (m *Manager) loadFilesFromDir(wfDir string) {
	entries, err := os.ReadDir(wfDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(wfDir, entry.Name())
		wf, err := parseWorkflowFile(path)
		if err != nil {
			continue
		}
		if wf != nil && wf.Name != "" {
			m.Add(*wf)
		}
	}
}

// parseWorkflowFile reads a markdown workflow file and parses it into a
// Workflow struct. It supports both YAML frontmatter (between --- delimiters)
// and inline Cosca workflow metadata.
func parseWorkflowFile(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow file %s: %w", path, err)
	}
	return parseWorkflowContent(string(data), filepath.Base(path))
}

// parseWorkflowContent parses workflow markdown content into a Workflow struct.
// It supports both YAML frontmatter (between --- delimiters) and inline Cosca
// workflow metadata. The nameHint is used to derive a workflow name when neither
// frontmatter nor inline markdown provide one.
func parseWorkflowContent(content string, nameHint string) (*Workflow, error) {
	lines := strings.Split(content, "\n")

	wf := &Workflow{
		Enabled: true,
		Status:  "active",
	}

	// --- Phase 1: YAML frontmatter parsing ---
	parseYAMLFrontmatter(lines, wf)

	// --- Phase 2: Inline markdown parsing ---
	parseInlineMarkdown(lines, wf)

	// Derive name from filename if not found in content
	if wf.Name == "" {
		wf.Name = strings.TrimSuffix(nameHint, ".md")
	}

	// Count steps from step list
	wf.Steps = len(wf.StepList)

	// If status was set by frontmatter or info line but not enabled, reflect that
	if wf.Status == "inactive" || wf.Status == "disabled" {
		wf.Enabled = false
	}

	if wf.Name == "" {
		return nil, fmt.Errorf("workflow has no name")
	}

	return wf, nil
}

// parseYAMLFrontmatter checks for --- delimited YAML frontmatter at the
// beginning of the file and unmarshals it into the workflow.
func parseYAMLFrontmatter(lines []string, wf *Workflow) {
	if len(lines) < 3 {
		return
	}
	first := strings.TrimSpace(lines[0])
	if first != "---" {
		return
	}

	// Find closing ---
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}
	if endIdx < 2 {
		return
	}

	frontmatter := strings.Join(lines[1:endIdx], "\n")
	if err := yaml.Unmarshal([]byte(frontmatter), wf); err != nil {
		// Frontmatter is malformed; silently fall through to inline parsing
		return
	}
}

// parseInlineMarkdown parses the Cosca workflow markdown format, extracting
// title, info line, section content, steps, inputs, and outputs.
func parseInlineMarkdown(lines []string, wf *Workflow) {
	currentSection := ""
	var step Step
	inStep := false
	stepIndex := 0

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// --- Title: # WORKFLOW: name ---
		if strings.HasPrefix(line, "# WORKFLOW:") {
			name := strings.TrimPrefix(line, "# WORKFLOW:")
			if wf.Name == "" {
				wf.Name = strings.TrimSpace(name)
			}
			continue
		}

		// --- Info line: > **Version**: ... | **Status**: ... ---
		if strings.HasPrefix(line, "> **Version**:") || strings.HasPrefix(line, ">**Version**:") {
			parseInfoLine(line, wf)
			continue
		}

		// --- Section headings: ## SECTION ---
		if strings.HasPrefix(line, "## ") {
			// Finalize any in-progress step
			if inStep {
				wf.StepList = append(wf.StepList, step)
				step = Step{}
				inStep = false
			}

			currentSection = strings.TrimSpace(strings.TrimPrefix(line, "##"))
			currentSection = strings.ToUpper(currentSection)
			continue
		}

		// --- Step heading: ### Step N: Name or #### Step N: Name ---
		if isStepHeading(line) {
			if inStep {
				wf.StepList = append(wf.StepList, step)
				step = Step{}
			}
			stepIndex++
			step = Step{
				Name: extractStepName(line),
			}
			inStep = true
			continue
		}

		// Parse content based on current section
		switch currentSection {
		case "OBJECTIVE":
			if wf.Description == "" {
				wf.Description = cleanDescription(line)
			} else {
				wf.Description += " " + cleanDescription(line)
			}

		case "INPUTS":
			if isTableRow(line) && !isTableSeparator(line) {
				if input := parseInputRow(line); input != nil {
					wf.Inputs = append(wf.Inputs, *input)
				}
			}

		case "OUTPUTS":
			if isTableRow(line) && !isTableSeparator(line) {
				if output := parseOutputRow(line); output != nil {
					wf.Outputs = append(wf.Outputs, *output)
				}
			}

		case "STEPS":
			// Additional step details inside the STEPS section beside (or instead of)
			// ### Step headings — parse step details lines
			if inStep {
				parseStepDetailLine(line, &step)
			}
		}
	}

	// Finalize last step
	if inStep {
		wf.StepList = append(wf.StepList, step)
	}
}

// parseInfoLine extracts version, status, category, and last-updated from the
// info line format: > **Version**: 1.1.0 | **Status**: active | ...
func parseInfoLine(line string, wf *Workflow) {
	// Strip leading "> " or ">" prefix
	cleaned := strings.TrimPrefix(line, "> ")
	cleaned = strings.TrimPrefix(cleaned, ">")

	parts := strings.Split(cleaned, "|")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		switch {
		case strings.HasPrefix(part, "**Version**:"):
			wf.Version = strings.TrimSpace(strings.TrimPrefix(part, "**Version**:"))
		case strings.HasPrefix(part, "**Status**:"):
			status := strings.TrimSpace(strings.TrimPrefix(part, "**Status**:"))
			if status != "" {
				wf.Status = status
			}
		}
	}
}

// isStepHeading checks if a line is a step heading like "### Step 1: Name"
// or "#### Step 1.1: Name".
func isStepHeading(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "### Step ") || strings.HasPrefix(trimmed, "#### Step ")
}

// extractStepName extracts the step name from a heading like "### Step 1: Name".
func extractStepName(line string) string {
	trimmed := strings.TrimSpace(line)
	// Remove ### or #### prefix
	rest := strings.TrimPrefix(trimmed, "#### ")
	rest = strings.TrimPrefix(rest, "### ")

	// "Step N: Name" or "Step N.N: Name"
	stepPrefix := "Step "
	if !strings.HasPrefix(rest, stepPrefix) {
		return ""
	}
	rest = strings.TrimPrefix(rest, stepPrefix)

	// Find the colon after the number
	colonIdx := strings.Index(rest, ":")
	if colonIdx < 0 {
		// No colon, just use the rest after the number
		spaceIdx := strings.Index(rest, " ")
		if spaceIdx < 0 {
			return ""
		}
		return strings.TrimSpace(rest[spaceIdx:])
	}
	return strings.TrimSpace(rest[colonIdx+1:])
}

// parseStepDetailLine extracts Chief, Task, Output, and Timeout details from
// bullet point lines inside a step definition.
func parseStepDetailLine(line string, step *Step) {
	switch {
	case strings.HasPrefix(line, "- **Chief**:"):
		v := strings.TrimSpace(strings.TrimPrefix(line, "- **Chief**:"))
		// Set agent; use the first Chief found if not set
		if step.Agent == "" {
			step.Agent = v
		}

	case strings.HasPrefix(line, "- **Task**:"):
		v := strings.TrimSpace(strings.TrimPrefix(line, "- **Task**:"))
		if step.Description == "" {
			step.Description = v
		}

	case strings.HasPrefix(line, "- **Timeout**:"):
		v := strings.TrimSpace(strings.TrimPrefix(line, "- **Timeout**:"))
		if step.Timeout == "" {
			step.Timeout = v
		}
	}
}

// cleanDescription removes leading/trailing markers from description text.
func cleanDescription(line string) string {
	s := strings.TrimSpace(line)
	// Remove bold markers sometimes used in objectives
	s = strings.TrimPrefix(s, "**")
	s = strings.TrimSuffix(s, "**")
	return s
}

// isTableRow returns true if the line looks like a markdown table row.
func isTableRow(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

// isTableSeparator returns true if the line is a table separator row.
func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return false
	}
	// Separator rows contain only |, -, :, and spaces
	for _, ch := range trimmed {
		if ch != '|' && ch != '-' && ch != ':' && ch != ' ' {
			return false
		}
	}
	return strings.Contains(trimmed, "-")
}

// parseInputRow parses a markdown table row from the INPUTS section.
// Expected columns: Name | Type | Required | Description
func parseInputRow(line string) *IO {
	cells := splitTableRow(line)
	if len(cells) < 3 {
		return nil
	}

	input := &IO{
		Name: strings.TrimSpace(cells[0]),
		Type: strings.TrimSpace(cells[1]),
	}

	requiredStr := strings.TrimSpace(cells[2])
	input.Required = strings.EqualFold(requiredStr, "yes") || strings.EqualFold(requiredStr, "true")

	return input
}

// parseOutputRow parses a markdown table row from the OUTPUTS section.
// Expected columns: Name | Type | Description
func parseOutputRow(line string) *IO {
	cells := splitTableRow(line)
	if len(cells) < 2 {
		return nil
	}

	output := &IO{
		Name: strings.TrimSpace(cells[0]),
		Type: strings.TrimSpace(cells[1]),
	}

	// Outputs are always considered "produced" (not user-required), so Required is false
	return output
}

// splitTableRow splits a markdown table row into its cell values, trimming
// whitespace and leading/trailing pipes.
func splitTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")

	parts := strings.Split(trimmed, "|")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		result = append(result, strings.TrimSpace(p))
	}
	return result
}
