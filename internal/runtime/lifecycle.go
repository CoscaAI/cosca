//
// Package runtime provides lifecycle management for the runtime engine,
// including ordered initialization, start, stop, and restart sequences.

package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// =============================================================================
// Lifecycle Phase
// =============================================================================

// Phase represents a lifecycle phase.
type Phase int

const (
	// PhaseInit represents the initialization phase.
	PhaseInit Phase = iota
	// PhaseStart represents the startup phase.
	PhaseStart
	// PhaseStop represents the shutdown phase.
	PhaseStop
	// PhaseRestart represents the restart phase.
	PhaseRestart
)

func (p Phase) String() string {
	switch p {
	case PhaseInit:
		return "init"
	case PhaseStart:
		return "start"
	case PhaseStop:
		return "stop"
	case PhaseRestart:
		return "restart"
	default:
		return "unknown"
	}
}

// =============================================================================
// Lifecycle Hook
// =============================================================================

// HookFunc is a function that is called during a lifecycle phase.
type HookFunc func(ctx context.Context, r *Runtime) error

// Hook represents a single lifecycle hook with a name and function.
type Hook struct {
	Name     string
	Phase    Phase
	Fn       HookFunc
	Timeout  time.Duration
	Required bool
}

// =============================================================================
// Lifecycle
// =============================================================================

// Lifecycle manages ordered initialization, start, and stop sequences.
// Each phase emits events and enforces timeouts on individual components.
type Lifecycle struct {
	mu          sync.RWMutex
	logger      zerolog.Logger
	initHooks   []Hook
	startHooks  []Hook
	stopHooks   []Hook
	phaseTiming map[Phase]time.Duration
}

// NewLifecycle creates a new lifecycle manager.
func NewLifecycle() *Lifecycle {
	return &Lifecycle{
		logger:      zerolog.Nop(),
		initHooks:   make([]Hook, 0),
		startHooks:  make([]Hook, 0),
		stopHooks:   make([]Hook, 0),
		phaseTiming: make(map[Phase]time.Duration),
	}
}

// SetLogger sets the logger for the lifecycle manager.
func (l *Lifecycle) SetLogger(logger zerolog.Logger) {
	l.logger = logger.With().Str("component", "lifecycle").Logger()
}

// AddInitHook adds a hook to the init phase.
func (l *Lifecycle) AddInitHook(name string, fn HookFunc, timeout time.Duration, required bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	hook := Hook{
		Name:     name,
		Phase:    PhaseInit,
		Fn:       fn,
		Timeout:  timeout,
		Required: required,
	}
	for i := range l.initHooks {
		if l.initHooks[i].Name == name {
			l.initHooks[i] = hook
			return
		}
	}
	l.initHooks = append(l.initHooks, hook)
}

// AddStartHook adds a hook to the start phase.
func (l *Lifecycle) AddStartHook(name string, fn HookFunc, timeout time.Duration, required bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	hook := Hook{
		Name:     name,
		Phase:    PhaseStart,
		Fn:       fn,
		Timeout:  timeout,
		Required: required,
	}
	for i := range l.startHooks {
		if l.startHooks[i].Name == name {
			l.startHooks[i] = hook
			return
		}
	}
	l.startHooks = append(l.startHooks, hook)
}

// AddStopHook adds a hook to the stop phase.
func (l *Lifecycle) AddStopHook(name string, fn HookFunc, timeout time.Duration, required bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	hook := Hook{
		Name:     name,
		Phase:    PhaseStop,
		Fn:       fn,
		Timeout:  timeout,
		Required: required,
	}
	for i := range l.stopHooks {
		if l.stopHooks[i].Name == name {
			l.stopHooks[i] = hook
			return
		}
	}
	l.stopHooks = append(l.stopHooks, hook)
}

// =============================================================================
// Execute Init
// =============================================================================

// ExecuteInit runs all initialization hooks in order.
// The knowledge subsystem is initialized first, followed by discovery,
// memory, cache, plugins, editors, and watcher.
func (l *Lifecycle) ExecuteInit(ctx context.Context, r *Runtime) error {
	l.logger.Info().Msg("initialization phase starting")

	// Register default init hooks in order
	defaultHooks := []struct {
		name     string
		fn       func(ctx context.Context, r *Runtime) error
		required bool
	}{
		{"knowledge", l.initKnowledge, true},
		{"discovery", l.initDiscovery, false},
		{"memory", l.initMemory, false},
		{"cache", l.initCache, true},
		{"compute-fabric", l.initCompute, false},
		{"plugins", l.initPlugins, false},
		{"editors", l.initEditors, false},
		{"watcher", l.initWatcher, false},
	}

	for _, dh := range defaultHooks {
		l.AddInitHook(dh.name, dh.fn, 30*time.Second, dh.required)
	}

	return l.executeHooks(ctx, r, l.initHooks, PhaseInit)
}

// ExecuteStart runs all start hooks in order.
func (l *Lifecycle) ExecuteStart(ctx context.Context, r *Runtime) error {
	l.logger.Info().Msg("start phase beginning")

	// Register default start hooks
	defaultHooks := []struct {
		name     string
		fn       func(ctx context.Context, r *Runtime) error
		required bool
	}{
		{"knowledge", l.startKnowledge, true},
		{"discovery", l.startDiscovery, false},
		{"memory", l.startMemory, false},
		{"cache", l.startCache, true},
		{"compute-fabric", l.startCompute, false},
		{"plugins", l.startPlugins, false},
		{"editors", l.startEditors, false},
		{"watcher", l.startWatcher, false},
	}

	for _, dh := range defaultHooks {
		l.AddStartHook(dh.name, dh.fn, 30*time.Second, dh.required)
	}

	return l.executeHooks(ctx, r, l.startHooks, PhaseStart)
}

// ExecuteStop runs all stop hooks in reverse order (first in, last out).
func (l *Lifecycle) ExecuteStop(ctx context.Context, r *Runtime) error {
	l.logger.Info().Msg("stop phase beginning")

	// Register default stop hooks in reverse order
	defaultHooks := []struct {
		name     string
		fn       func(ctx context.Context, r *Runtime) error
		required bool
	}{
		{"watcher", l.stopWatcher, false},
		{"editors", l.stopEditors, false},
		{"plugins", l.stopPlugins, false},
		{"cache", l.stopCache, true},
		{"compute-fabric", l.stopCompute, false},
		{"memory", l.stopMemory, false},
		{"discovery", l.stopDiscovery, false},
		{"knowledge", l.stopKnowledge, true},
	}

	for _, dh := range defaultHooks {
		l.AddStopHook(dh.name, dh.fn, 30*time.Second, dh.required)
	}

	return l.executeHooks(ctx, r, l.stopHooks, PhaseStop)
}

// =============================================================================
// Hook Execution
// =============================================================================

// executeHooks runs a sequence of hooks with individual timeouts.
func (l *Lifecycle) executeHooks(ctx context.Context, r *Runtime, hooks []Hook, phase Phase) error {
	phaseStart := time.Now()
	l.logger.Info().
		Str("phase", phase.String()).
		Int("hooks", len(hooks)).
		Msg("executing phase")

	var errs []error

	for i, hook := range hooks {
		hookStart := time.Now()

		select {
		case <-ctx.Done():
			l.logger.Warn().
				Str("phase", phase.String()).
				Str("hook", hook.Name).
				Msg("phase cancelled")
			return fmt.Errorf("phase %s cancelled: %w", phase, ctx.Err())
		default:
		}

		// Create hook-specific timeout context
		timeout := hook.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		hookCtx, hookCancel := context.WithTimeout(ctx, timeout)

		// CRITICAL FIX (audit 2026-08-01): the hook ran synchronously, so a
		// hook that ignores its context (e.g. Stop() doing wg.Wait() without
		// observing cancellation) blocked the whole shutdown past the
		// timeout. The hook now runs in a goroutine and the phase proceeds
		// after the timeout even if the hook is stuck; the stuck goroutine
		// is eventually reaped by the process exit or the runtime context.
		hookResult := make(chan error, 1)
		go func() {
			hookResult <- hook.Fn(hookCtx, r)
		}()
		var hookErr error
		select {
		case hookErr = <-hookResult:
		case <-hookCtx.Done():
			hookErr = hookCtx.Err()
			l.logger.Warn().
				Str("phase", phase.String()).
				Str("hook", hook.Name).
				Dur("timeout", timeout).
				Msg("hook timed out; proceeding")
		}
		hookCancel()

		elapsed := time.Since(hookStart)

		if hookErr != nil {
			l.logger.Error().
				Err(hookErr).
				Str("phase", phase.String()).
				Str("hook", hook.Name).
				Int("step", i+1).
				Int("total", len(hooks)).
				Dur("elapsed", elapsed).
				Bool("required", hook.Required).
				Msg("hook failed")

			r.state.SetComponentStatus(hook.Name, StatusUnhealthy, hookErr.Error())

			if hook.Required {
				errs = append(errs, fmt.Errorf("%s hook %q failed: %w", phase, hook.Name, hookErr))
			} else {
				r.logger.Warn().Err(hookErr).Str("hook", hook.Name).Msg("non-required hook failed, continuing")
			}
		} else {
			l.logger.Info().
				Str("phase", phase.String()).
				Str("hook", hook.Name).
				Int("step", i+1).
				Int("total", len(hooks)).
				Dur("elapsed", elapsed).
				Msg("hook completed")

			r.state.SetComponentStatus(hook.Name, StatusHealthy, "running")
		}
	}

	phaseElapsed := time.Since(phaseStart)
	l.phaseTiming[phase] = phaseElapsed

	l.logger.Info().
		Str("phase", phase.String()).
		Dur("elapsed", phaseElapsed).
		Int("errors", len(errs)).
		Msg("phase completed")

	if len(errs) > 0 {
		return fmt.Errorf("phase %s completed with %d errors", phase, len(errs))
	}
	return nil
}

// =============================================================================
// Default Init Hooks
// =============================================================================

func (l *Lifecycle) initKnowledge(ctx context.Context, r *Runtime) error {
	if r.knowledge == nil {
		return nil
	}
	r.state.SetComponentStatus("knowledge", StatusStarting, "initializing")
	r.logger.Debug().Msg("initializing knowledge subsystem")
	err := r.knowledge.Start(ctx)
	if err != nil {
		r.state.SetComponentStatus("knowledge", StatusUnhealthy, err.Error())
		return fmt.Errorf("knowledge init: %w", err)
	}
	r.state.SetComponentStatus("knowledge", StatusHealthy, "initialized")
	r.events.Publish(ctx, EventSubsystemStarted, "knowledge", nil)
	return nil
}

func (l *Lifecycle) initDiscovery(ctx context.Context, r *Runtime) error {
	if r.discovery == nil {
		return nil
	}
	r.state.SetComponentStatus("discovery", StatusStarting, "initializing")
	r.logger.Debug().Msg("initializing discovery subsystem")
	err := r.discovery.Start(ctx)
	if err != nil {
		r.state.SetComponentStatus("discovery", StatusUnhealthy, err.Error())
		return fmt.Errorf("discovery init: %w", err)
	}
	r.state.SetComponentStatus("discovery", StatusHealthy, "initialized")
	r.events.Publish(ctx, EventSubsystemStarted, "discovery", nil)
	return nil
}

func (l *Lifecycle) initMemory(ctx context.Context, r *Runtime) error {
	if r.memory == nil {
		return nil
	}
	r.state.SetComponentStatus("memory", StatusStarting, "initializing")
	r.logger.Debug().Msg("initializing memory subsystem")
	err := r.memory.Start(ctx)
	if err != nil {
		r.state.SetComponentStatus("memory", StatusUnhealthy, err.Error())
		return fmt.Errorf("memory init: %w", err)
	}
	r.state.SetComponentStatus("memory", StatusHealthy, "initialized")
	r.events.Publish(ctx, EventSubsystemStarted, "memory", nil)
	return nil
}

func (l *Lifecycle) initCache(ctx context.Context, r *Runtime) error {
	if r.cache == nil {
		return nil
	}
	r.state.SetComponentStatus("cache", StatusStarting, "initializing")
	r.logger.Debug().Msg("initializing cache subsystem")
	err := r.cache.Start(ctx)
	if err != nil {
		r.state.SetComponentStatus("cache", StatusUnhealthy, err.Error())
		return fmt.Errorf("cache init: %w", err)
	}
	r.state.SetComponentStatus("cache", StatusHealthy, "initialized")
	r.events.Publish(ctx, EventSubsystemStarted, "cache", nil)
	return nil
}

func (l *Lifecycle) initCompute(ctx context.Context, r *Runtime) error {
	if r.compute == nil {
		return nil
	}
	r.state.SetComponentStatus("compute-fabric", StatusStarting, "initializing")
	r.logger.Debug().Msg("initializing compute-fabric subsystem")
	err := r.compute.Start(ctx)
	if err != nil {
		r.state.SetComponentStatus("compute-fabric", StatusUnhealthy, err.Error())
		return fmt.Errorf("compute-fabric init: %w", err)
	}
	r.state.SetComponentStatus("compute-fabric", StatusHealthy, "initialized")
	r.events.Publish(ctx, EventSubsystemStarted, "compute-fabric", nil)
	return nil
}

func (l *Lifecycle) initPlugins(ctx context.Context, r *Runtime) error {
	if r.plugins == nil {
		return nil
	}
	r.state.SetComponentStatus("plugins", StatusStarting, "initializing")
	r.logger.Debug().Msg("initializing plugins subsystem")
	err := r.plugins.Start(ctx)
	if err != nil {
		r.state.SetComponentStatus("plugins", StatusUnhealthy, err.Error())
		return fmt.Errorf("plugins init: %w", err)
	}
	r.state.SetComponentStatus("plugins", StatusHealthy, "initialized")
	r.events.Publish(ctx, EventSubsystemStarted, "plugins", nil)
	return nil
}

func (l *Lifecycle) initEditors(ctx context.Context, r *Runtime) error {
	if r.editors == nil {
		return nil
	}
	r.state.SetComponentStatus("editors", StatusStarting, "initializing")
	r.logger.Debug().Msg("initializing editors subsystem")
	err := r.editors.Start(ctx)
	if err != nil {
		r.state.SetComponentStatus("editors", StatusUnhealthy, err.Error())
		return fmt.Errorf("editors init: %w", err)
	}
	r.state.SetComponentStatus("editors", StatusHealthy, "initialized")
	r.events.Publish(ctx, EventSubsystemStarted, "editors", nil)
	return nil
}

func (l *Lifecycle) initWatcher(ctx context.Context, r *Runtime) error {
	if r.watcher == nil {
		return nil
	}
	r.state.SetComponentStatus("watcher", StatusStarting, "initializing")
	r.logger.Debug().Msg("initializing watcher subsystem")
	err := r.watcher.Start(ctx)
	if err != nil {
		r.state.SetComponentStatus("watcher", StatusUnhealthy, err.Error())
		return fmt.Errorf("watcher init: %w", err)
	}
	r.state.SetComponentStatus("watcher", StatusHealthy, "initialized")
	r.events.Publish(ctx, EventSubsystemStarted, "watcher", nil)
	return nil
}

// =============================================================================
// Default Start Hooks
// =============================================================================

func (l *Lifecycle) startKnowledge(ctx context.Context, r *Runtime) error {
	if r.knowledge == nil {
		return nil
	}
	r.events.Publish(ctx, EventSubsystemStarted, "knowledge", nil)
	return nil
}

func (l *Lifecycle) startDiscovery(ctx context.Context, r *Runtime) error {
	if r.discovery == nil {
		return nil
	}
	r.events.Publish(ctx, EventSubsystemStarted, "discovery", nil)
	return nil
}

func (l *Lifecycle) startMemory(ctx context.Context, r *Runtime) error {
	if r.memory == nil {
		return nil
	}
	r.events.Publish(ctx, EventSubsystemStarted, "memory", nil)
	return nil
}

func (l *Lifecycle) startCache(ctx context.Context, r *Runtime) error {
	if r.cache == nil {
		return nil
	}
	r.events.Publish(ctx, EventSubsystemStarted, "cache", nil)
	return nil
}

func (l *Lifecycle) startCompute(ctx context.Context, r *Runtime) error {
	if r.compute == nil {
		return nil
	}
	r.events.Publish(ctx, EventSubsystemStarted, "compute-fabric", nil)
	return nil
}

func (l *Lifecycle) startPlugins(ctx context.Context, r *Runtime) error {
	if r.plugins == nil {
		return nil
	}
	r.events.Publish(ctx, EventSubsystemStarted, "plugins", nil)
	return nil
}

func (l *Lifecycle) startEditors(ctx context.Context, r *Runtime) error {
	if r.editors == nil {
		return nil
	}
	r.events.Publish(ctx, EventSubsystemStarted, "editors", nil)
	return nil
}

func (l *Lifecycle) startWatcher(ctx context.Context, r *Runtime) error {
	if r.watcher == nil {
		return nil
	}
	r.events.Publish(ctx, EventSubsystemStarted, "watcher", nil)
	return nil
}

// =============================================================================
// Default Stop Hooks
// =============================================================================

func (l *Lifecycle) stopWatcher(ctx context.Context, r *Runtime) error {
	if r.watcher == nil {
		return nil
	}
	r.state.SetComponentStatus("watcher", StatusStopping, "stopping")
	r.logger.Debug().Msg("stopping watcher subsystem")
	err := r.watcher.Stop(ctx)
	if err != nil {
		r.state.SetComponentStatus("watcher", StatusUnhealthy, err.Error())
		return fmt.Errorf("watcher stop: %w", err)
	}
	r.state.SetComponentStatus("watcher", StatusStoppedComponent, "stopped")
	r.events.Publish(ctx, EventSubsystemStopped, "watcher", nil)
	return nil
}

func (l *Lifecycle) stopEditors(ctx context.Context, r *Runtime) error {
	if r.editors == nil {
		return nil
	}
	r.state.SetComponentStatus("editors", StatusStopping, "stopping")
	r.logger.Debug().Msg("stopping editors subsystem")
	err := r.editors.Stop(ctx)
	if err != nil {
		r.state.SetComponentStatus("editors", StatusUnhealthy, err.Error())
		return fmt.Errorf("editors stop: %w", err)
	}
	r.state.SetComponentStatus("editors", StatusStoppedComponent, "stopped")
	r.events.Publish(ctx, EventSubsystemStopped, "editors", nil)
	return nil
}

func (l *Lifecycle) stopPlugins(ctx context.Context, r *Runtime) error {
	if r.plugins == nil {
		return nil
	}
	r.state.SetComponentStatus("plugins", StatusStopping, "stopping")
	r.logger.Debug().Msg("stopping plugins subsystem")
	err := r.plugins.Stop(ctx)
	if err != nil {
		r.state.SetComponentStatus("plugins", StatusUnhealthy, err.Error())
		return fmt.Errorf("plugins stop: %w", err)
	}
	r.state.SetComponentStatus("plugins", StatusStoppedComponent, "stopped")
	r.events.Publish(ctx, EventSubsystemStopped, "plugins", nil)
	return nil
}

func (l *Lifecycle) stopCache(ctx context.Context, r *Runtime) error {
	if r.cache == nil {
		return nil
	}
	r.state.SetComponentStatus("cache", StatusStopping, "stopping")
	r.logger.Debug().Msg("stopping cache subsystem")
	err := r.cache.Stop(ctx)
	if err != nil {
		r.state.SetComponentStatus("cache", StatusUnhealthy, err.Error())
		return fmt.Errorf("cache stop: %w", err)
	}
	r.state.SetComponentStatus("cache", StatusStoppedComponent, "stopped")
	r.events.Publish(ctx, EventSubsystemStopped, "cache", nil)
	return nil
}

func (l *Lifecycle) stopCompute(ctx context.Context, r *Runtime) error {
	if r.compute == nil {
		return nil
	}
	r.state.SetComponentStatus("compute-fabric", StatusStopping, "stopping")
	r.logger.Debug().Msg("stopping compute-fabric subsystem")
	err := r.compute.Stop(ctx)
	if err != nil {
		r.state.SetComponentStatus("compute-fabric", StatusUnhealthy, err.Error())
		return fmt.Errorf("compute-fabric stop: %w", err)
	}
	r.state.SetComponentStatus("compute-fabric", StatusStoppedComponent, "stopped")
	r.events.Publish(ctx, EventSubsystemStopped, "compute-fabric", nil)
	return nil
}

func (l *Lifecycle) stopMemory(ctx context.Context, r *Runtime) error {
	if r.memory == nil {
		return nil
	}
	r.state.SetComponentStatus("memory", StatusStopping, "stopping")
	r.logger.Debug().Msg("stopping memory subsystem")
	err := r.memory.Stop(ctx)
	if err != nil {
		r.state.SetComponentStatus("memory", StatusUnhealthy, err.Error())
		return fmt.Errorf("memory stop: %w", err)
	}
	r.state.SetComponentStatus("memory", StatusStoppedComponent, "stopped")
	r.events.Publish(ctx, EventSubsystemStopped, "memory", nil)
	return nil
}

func (l *Lifecycle) stopDiscovery(ctx context.Context, r *Runtime) error {
	if r.discovery == nil {
		return nil
	}
	r.state.SetComponentStatus("discovery", StatusStopping, "stopping")
	r.logger.Debug().Msg("stopping discovery subsystem")
	err := r.discovery.Stop(ctx)
	if err != nil {
		r.state.SetComponentStatus("discovery", StatusUnhealthy, err.Error())
		return fmt.Errorf("discovery stop: %w", err)
	}
	r.state.SetComponentStatus("discovery", StatusStoppedComponent, "stopped")
	r.events.Publish(ctx, EventSubsystemStopped, "discovery", nil)
	return nil
}

func (l *Lifecycle) stopKnowledge(ctx context.Context, r *Runtime) error {
	if r.knowledge == nil {
		return nil
	}
	r.state.SetComponentStatus("knowledge", StatusStopping, "stopping")
	r.logger.Debug().Msg("stopping knowledge subsystem")
	err := r.knowledge.Stop(ctx)
	if err != nil {
		r.state.SetComponentStatus("knowledge", StatusUnhealthy, err.Error())
		return fmt.Errorf("knowledge stop: %w", err)
	}
	r.state.SetComponentStatus("knowledge", StatusStoppedComponent, "stopped")
	r.events.Publish(ctx, EventSubsystemStopped, "knowledge", nil)
	return nil
}

// =============================================================================
// Phase Stats
// =============================================================================

// PhaseDuration returns the duration of a completed phase.
func (l *Lifecycle) PhaseDuration(phase Phase) (time.Duration, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	d, ok := l.phaseTiming[phase]
	return d, ok
}
