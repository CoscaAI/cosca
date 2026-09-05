package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/CoscaAI/cosca/internal/trace"
)

// projectDir returns the project directory to run build/test/lint in.
// Inside the jail (COSCA_JAILED=1) the workspace is mounted at "/", so the
// plugins must run go in "/" to find the Go module (COSCA_PROJECT_DIR is the
// host path and does not exist inside the bubble).
func projectDir() string {
	if os.Getenv("COSCA_JAILED") == "1" {
		return "/"
	}
	if d := os.Getenv("COSCA_PROJECT_DIR"); d != "" {
		return d
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// runInProject runs a Go command in the project directory.
func runInProject(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = projectDir()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ── Build Plugin ──────────────────────────────────────────────────────

// BuildPlugin runs `go build` after each step.
type BuildPlugin struct{}

func (p *BuildPlugin) Name() string                         { return "build" }
func (p *BuildPlugin) Priority() int                        { return 10 }
func (p *BuildPlugin) ExtensionPoints() []ExtensionPoint    { return []ExtensionPoint{ExtStepPostExecute} }
func (p *BuildPlugin) Execute(ctx context.Context, ext ExtensionPoint, pc *PluginContext) error {
	out, err := runInProject(ctx, "build", "./...")
	if err != nil {
		return fmt.Errorf("build failed: %s", strings.TrimSpace(out))
	}
	return nil
}

// ── Test Plugin ───────────────────────────────────────────────────────

// TestPlugin runs `go test` after each step.
type TestPlugin struct{}

func (p *TestPlugin) Name() string                         { return "test" }
func (p *TestPlugin) Priority() int                        { return 20 }
func (p *TestPlugin) ExtensionPoints() []ExtensionPoint    { return []ExtensionPoint{ExtStepPostExecute} }
func (p *TestPlugin) Execute(ctx context.Context, ext ExtensionPoint, pc *PluginContext) error {
	out, err := runInProject(ctx, "test", "./...", "-count=1", "-timeout=120s")
	if err != nil {
		return fmt.Errorf("tests failed: %s", strings.TrimSpace(out))
	}
	return nil
}

// ── Lint Plugin ───────────────────────────────────────────────────────

// LintPlugin runs `go vet` after each step.
type LintPlugin struct{}

func (p *LintPlugin) Name() string                         { return "lint" }
func (p *LintPlugin) Priority() int                        { return 30 }
func (p *LintPlugin) ExtensionPoints() []ExtensionPoint    { return []ExtensionPoint{ExtStepPostExecute} }
func (p *LintPlugin) Execute(ctx context.Context, ext ExtensionPoint, pc *PluginContext) error {
	out, err := runInProject(ctx, "vet", "./...")
	if err != nil && strings.TrimSpace(out) != "" {
		return fmt.Errorf("lint errors: %s", strings.TrimSpace(out))
	}
	return nil
}

// ── Checkpoint Plugin ─────────────────────────────────────────────────

// CheckpointPlugin saves a checkpoint after each step.
type CheckpointPlugin struct {
	store *CheckpointStore
}

func NewCheckpointPlugin(store *CheckpointStore) *CheckpointPlugin {
	return &CheckpointPlugin{store: store}
}

func (p *CheckpointPlugin) Name() string                      { return "checkpoint" }
func (p *CheckpointPlugin) Priority() int                     { return 100 }
func (p *CheckpointPlugin) ExtensionPoints() []ExtensionPoint { return []ExtensionPoint{ExtStepPostExecute} }
func (p *CheckpointPlugin) Execute(ctx context.Context, ext ExtensionPoint, pc *PluginContext) error {
	if p.store == nil || pc.Plan == nil {
		return nil
	}
	cp := Checkpoint{
		PlanID:       pc.Plan.ID,
		LastSequence: 0,
		TaskStatuses: make(map[string]string),
	}
	for _, t := range pc.Plan.Tasks {
		cp.TaskStatuses[t.ID] = string(t.Status)
	}
	return p.store.Save(cp)
}

// ── Trace Propagation Plugin ────────────────────────────────────────────

// TracePlugin propagates W3C trace context for each step.
type TracePlugin struct {
	planTrace *trace.Traceparent
}

func NewTracePlugin() *TracePlugin {
	return &TracePlugin{}
}

func (p *TracePlugin) Name() string                      { return "trace" }
func (p *TracePlugin) Priority() int                     { return 5 } // runs first
func (p *TracePlugin) ExtensionPoints() []ExtensionPoint { return []ExtensionPoint{ExtStepPreExecute} }
func (p *TracePlugin) Execute(ctx context.Context, ext ExtensionPoint, pc *PluginContext) error {
	if pc.Task == nil {
		return nil
	}
	// Generate W3C trace context for this step
	coscaID := trace.NewID()
	tp := trace.NewTraceparent(coscaID)

	// Store traceparent in task metadata for downstream propagation
	if pc.Task.Result == nil {
		pc.Task.Result = &TaskResult{}
	}
	pc.Task.Result.TraceID = string(coscaID)

	// The traceparent can be injected into HTTP calls by downstream agents.
	_ = tp
	return nil
}

// ── Register Built-in Plugins ─────────────────────────────────────────

// RegisterBuiltinPlugins adds the standard plugins to the registry.
func RegisterBuiltinPlugins(reg *PluginRegistry, checkpoint *CheckpointStore) {
	reg.Register(&TracePlugin{})
	reg.Register(&BuildPlugin{})
	reg.Register(&TestPlugin{})
	reg.Register(&LintPlugin{})
	if checkpoint != nil {
		reg.Register(NewCheckpointPlugin(checkpoint))
	}
}
