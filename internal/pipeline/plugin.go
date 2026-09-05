package pipeline

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// ExtensionPoint names the hook points in the pipeline where plugins can attach.
type ExtensionPoint string

const (
	// Step hooks
	ExtStepPreExecute  ExtensionPoint = "step.pre_execute"  // before a step runs
	ExtStepPostExecute ExtensionPoint = "step.post_execute" // after a step completes
	ExtStepOnFailure   ExtensionPoint = "step.on_failure"   // when a step fails

	// Plan hooks
	ExtPlanPreCreate  ExtensionPoint = "plan.pre_create"  // before planning
	ExtPlanPostCreate ExtensionPoint = "plan.post_create" // after planning

	// Pipeline hooks
	ExtPipelineStart ExtensionPoint = "pipeline.start" // pipeline execution begins
	ExtPipelineEnd   ExtensionPoint = "pipeline.end"   // pipeline execution ends
)

// PluginContext carries the execution context for a plugin invocation.
type PluginContext struct {
	Plan     *Plan
	Task     *TaskNode
	StepName string
	Result   *TaskResult
	Error    error
}

// Plugin is a pipeline extension that hooks into one or more ExtensionPoints.
// Implementations are registered with the PluginRegistry and invoked
// automatically by the StepRunner during pipeline execution.
type Plugin interface {
	// Name returns the unique plugin identifier.
	Name() string

	// ExtensionPoints returns the hook points this plugin subscribes to.
	ExtensionPoints() []ExtensionPoint

	// Priority determines execution order (lower = earlier). Default: 100.
	Priority() int

	// Execute runs the plugin's logic. Return an error to abort the pipeline.
	Execute(ctx context.Context, ext ExtensionPoint, pc *PluginContext) error
}

// PluginRegistry manages pipeline plugins.
type PluginRegistry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	order   map[ExtensionPoint][]string // ordered plugin names per extension point
}

// NewPluginRegistry creates an empty plugin registry.
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]Plugin),
		order:   make(map[ExtensionPoint][]string),
	}
}

// Register adds a plugin to the registry.
func (r *PluginRegistry) Register(p Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	r.plugins[name] = p

	for _, ext := range p.ExtensionPoints() {
		r.order[ext] = append(r.order[ext], name)
	}

	// Sort by priority
	for ext := range r.order {
		sort.SliceStable(r.order[ext], func(i, j int) bool {
			pi := r.plugins[r.order[ext][i]]
			pj := r.plugins[r.order[ext][j]]
			return pi.Priority() < pj.Priority()
		})
	}
}

// Unregister removes a plugin from the registry.
func (r *PluginRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.plugins, name)
	for ext := range r.order {
		filtered := make([]string, 0, len(r.order[ext]))
		for _, n := range r.order[ext] {
			if n != name {
				filtered = append(filtered, n)
			}
		}
		r.order[ext] = filtered
	}
}

// Execute runs all plugins registered for the given extension point.
// Execution stops on the first error (fail-fast).
func (r *PluginRegistry) Execute(ctx context.Context, ext ExtensionPoint, pc *PluginContext) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names, ok := r.order[ext]
	if !ok {
		return nil
	}

	for _, name := range names {
		p, exists := r.plugins[name]
		if !exists {
			continue
		}
		if err := p.Execute(ctx, ext, pc); err != nil {
			return fmt.Errorf("plugin %q at %s: %w", name, ext, err)
		}
	}

	return nil
}

// List returns the names of all registered plugins.
func (r *PluginRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// PluginsFor returns plugin names registered for an extension point.
func (r *PluginRegistry) PluginsFor(ext ExtensionPoint) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.order[ext]
}
