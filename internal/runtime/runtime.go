//
// Package runtime provides the central runtime engine for the Cosca platform.
// It manages the application lifecycle, subsystem coordination, health reporting,
// and event propagation.
//
// The runtime follows a state machine: uninitialized -> initializing -> ready ->
// running -> stopping -> stopped. Error states can interrupt this flow, and
// recovery mechanisms can return the runtime to a ready state.

package runtime

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// =============================================================================
// Subsystem Interfaces
// =============================================================================

// Subsystem represents a runtime component that can be started and stopped.
type Subsystem interface {
	// Name returns the subsystem name.
	Name() string
	// Start initializes and starts the subsystem.
	Start(ctx context.Context) error
	// Stop gracefully stops the subsystem.
	Stop(ctx context.Context) error
	// Health returns the current health status of the subsystem.
	Health() ComponentStatus
}

// =============================================================================
// Event Bus
// =============================================================================

// EventType represents a type of runtime event.
type EventType string

const (
	// EventStateChange is emitted when the runtime state changes.
	EventStateChange EventType = "state_change"
	// EventSubsystemStarted is emitted when a subsystem starts.
	EventSubsystemStarted EventType = "subsystem_started"
	// EventSubsystemStopped is emitted when a subsystem stops.
	EventSubsystemStopped EventType = "subsystem_stopped"
	// EventSubsystemError is emitted when a subsystem encounters an error.
	EventSubsystemError EventType = "subsystem_error"
	// EventHealthChange is emitted when runtime health changes.
	EventHealthChange EventType = "health_change"
	// EventStartupComplete is emitted when initialization completes.
	EventStartupComplete EventType = "startup_complete"
	// EventShutdownInitiated is emitted when shutdown begins.
	EventShutdownInitiated EventType = "shutdown_initiated"
	// EventShutdownComplete is emitted when shutdown finishes.
	EventShutdownComplete EventType = "shutdown_complete"
	// EventConfigReload is emitted when configuration is reloaded.
	EventConfigReload EventType = "config_reload"
)

// Event represents a runtime event with associated data.
type Event struct {
	ID        string      `json:"id" yaml:"id"`
	Type      EventType   `json:"type" yaml:"type"`
	Timestamp time.Time   `json:"timestamp" yaml:"timestamp"`
	Source    string      `json:"source,omitempty" yaml:"source,omitempty"`
	Data      interface{} `json:"data,omitempty" yaml:"data,omitempty"`
}

// EventHandler is a function that processes runtime events.
type EventHandler func(ctx context.Context, event Event) error

// publishDeadline bounds how long a single Publish may block the caller
// waiting for handlers. Fast handlers finish in microseconds; this only
// engages when a handler misbehaves. 3s is a safe upper bound for the
// health/watchdog loops that publish regularly.
const publishDeadline = 3 * time.Second

// EventBus provides a publish-subscribe mechanism for runtime events.
type EventBus struct {
	mu       sync.RWMutex
	handlers map[EventType][]EventHandler
	logger   zerolog.Logger
}

// NewEventBus creates a new event bus.
func NewEventBus(logger zerolog.Logger) *EventBus {
	return &EventBus{
		handlers: make(map[EventType][]EventHandler),
		logger:   logger,
	}
}

// Subscribe registers a handler for a specific event type.
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
	eb.logger.Debug().
		Str("event_type", string(eventType)).
		Msg("event handler subscribed")
}

// Unsubscribe removes all handlers for a specific event type.
func (eb *EventBus) Unsubscribe(eventType EventType) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	delete(eb.handlers, eventType)
}

// Publish emits an event to all registered handlers.
func (eb *EventBus) Publish(ctx context.Context, eventType EventType, source string, data interface{}) {
	event := Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Timestamp: time.Now(),
		Source:    source,
		Data:      data,
	}

	eb.mu.RLock()
	handlers := eb.handlers[eventType]
	eb.mu.RUnlock()

	if len(handlers) == 0 {
		return
	}

	eb.logger.Debug().
		Str("event_id", event.ID).
		Str("type", string(eventType)).
		Str("source", source).
		Int("handlers", len(handlers)).
		Msg("publishing event")

	// CRITICAL FIX (audit 2026-08-01): handlers were invoked synchronously
	// with a 10s PER-HANDLER timeout, so a blocked handler froze the
	// PUBLISHER for 10s or more (health-check loop, watchdog, Stop) — and a
	// handler that never observes its context blocked it forever.
	//
	// Handlers now run concurrently, but Publish waits for all of them with
	// a SINGLE global deadline. Fast handlers (the normal case) complete
	// within microseconds, so the synchronous side-effect contract callers
	// rely on is preserved; a misbehaving handler can never stall Publish
	// beyond `publishDeadline`. Each handler goroutine is still bounded by
	// its own 10s timeout and reaped on process exit.
	// The result channel is buffered so a handler that ignores cancellation can
	// finish without leaving a waiter goroutine behind. Publish itself owns the
	// single deadline; there is no separate wg.Wait goroutine to leak.
	done := make(chan error, len(handlers))
	for _, handler := range handlers {
		go func(h EventHandler) {
			handlerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			done <- h(handlerCtx, event)
		}(handler)
	}
	deadline := time.NewTimer(publishDeadline)
	defer deadline.Stop()
	for remaining := len(handlers); remaining > 0; remaining-- {
		select {
		case err := <-done:
			if err != nil {
				eb.logger.Warn().Err(err).Str("event_id", event.ID).Str("type", string(eventType)).Msg("event handler error")
			}
		case <-deadline.C:
			eb.logger.Warn().
				Str("event_id", event.ID).
				Str("type", string(eventType)).
				Dur("deadline", publishDeadline).
				Msg("publish deadline exceeded; returning without all handlers")
			return
		}
	}
}

// =============================================================================
// Runtime
// =============================================================================

// Runtime is the central orchestrator for the Cosca platform.
// It manages the lifecycle of all subsystems, coordinates startup and shutdown,
// provides health reporting, and collects metrics.
type Runtime struct {
	mu sync.RWMutex
	// lifecycleMu serializes the complete Start/Stop/Restart operation.  The
	// state check alone is not sufficient: two callers can otherwise both pass
	// the check before either one performs the transition.
	lifecycleMu sync.Mutex
	logger      zerolog.Logger
	state       *RuntimeState
	events      *EventBus
	metrics     *Metrics
	lifecycle   *Lifecycle

	// Subsystems
	knowledge Subsystem
	discovery Subsystem
	memory    Subsystem
	cache     Subsystem
	compute   Subsystem
	plugins   Subsystem
	editors   Subsystem
	watcher   Subsystem

	// Daemon
	daemon *Daemon

	// Config
	config RuntimeConfig

	// Cancellation
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Shutdown
	shutdownCh chan struct{}
}

// RuntimeConfig configures the runtime engine.
//
//nolint:revive // Stutter name preserved for API compatibility — used as runtime.RuntimeConfig externally.
type RuntimeConfig struct {
	// Name is the application name.
	Name string `json:"name" yaml:"name"`
	// Version is the application version.
	Version string `json:"version" yaml:"version"`
	// DataDir is the persistent data directory.
	DataDir string `json:"data_dir" yaml:"data_dir"`
	// RuntimeDir is the runtime directory for ephemeral state.
	RuntimeDir string `json:"runtime_dir" yaml:"runtime_dir"`
	// ComponentTimeout is the default timeout for component operations.
	ComponentTimeout time.Duration `json:"component_timeout" yaml:"component_timeout"`
	// ShutdownTimeout is the timeout for graceful shutdown.
	ShutdownTimeout time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout"`
	// ShutdownWaitTimeout is the maximum time to wait for runtime goroutines
	// after cancellation and lifecycle shutdown have completed.
	ShutdownWaitTimeout time.Duration `json:"shutdown_wait_timeout" yaml:"shutdown_wait_timeout"`
	// HealthCheckInterval is how often to check component health.
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
	// EnableMetrics enables metrics collection.
	EnableMetrics bool `json:"enable_metrics" yaml:"enable_metrics"`
	// EnableDaemon enables the background daemon.
	EnableDaemon bool `json:"enable_daemon" yaml:"enable_daemon"`
	// LogLevel is the log level for the runtime.
	LogLevel string `json:"log_level" yaml:"log_level"`
	// PidFile is the path to the PID file.
	PidFile string `json:"pid_file" yaml:"pid_file"`
}

// DefaultRuntimeConfig returns a default runtime configuration.
func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Name:                "cosca",
		Version:             "0.0.0",
		ComponentTimeout:    30 * time.Second,
		ShutdownTimeout:     time.Minute,
		ShutdownWaitTimeout: 10 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		EnableMetrics:       true,
		// EnableDaemon defaults to true (decision by Don, 2026-07-31,
		// approved after measurement: backup cost 49ms/hour on a 24MB DB;
		// the memory loss risk without backup outweighs the cost).
		EnableDaemon: true,
		LogLevel:     "info",
	}
}

// waitForWaitGroup waits for workers without allowing a stuck worker to block
// process shutdown forever.
func waitForWaitGroup(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Option configures the runtime engine.
type Option func(*Runtime)

// WithLogger sets the logger for the runtime.
func WithLogger(logger zerolog.Logger) Option {
	return func(r *Runtime) {
		r.logger = logger
	}
}

// WithConfig sets the runtime configuration.
func WithConfig(cfg RuntimeConfig) Option {
	return func(r *Runtime) {
		r.config = cfg
	}
}

// WithSubsystem registers a subsystem with the runtime.
func WithSubsystem(s Subsystem) Option {
	return func(r *Runtime) {
		switch s.Name() {
		case "knowledge":
			r.knowledge = s
		case "discovery":
			r.discovery = s
		case "memory":
			r.memory = s
		case "cache":
			r.cache = s
		case "plugins":
			r.plugins = s
		case "editors":
			r.editors = s
		case "watcher":
			r.watcher = s
		default:
			r.logger.Warn().Str("subsystem", s.Name()).Msg("unknown subsystem type, ignoring")
		}
	}
}

// New creates a new Runtime instance.
func New(opts ...Option) *Runtime {
	ctx, cancel := context.WithCancel(context.Background())

	r := &Runtime{
		logger:     zerolog.Nop(),
		state:      NewRuntimeState(),
		events:     NewEventBus(zerolog.Nop()),
		metrics:    NewMetrics(),
		lifecycle:  NewLifecycle(),
		config:     DefaultRuntimeConfig(),
		ctx:        ctx,
		cancel:     cancel,
		shutdownCh: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(r)
	}

	// Wire up the logger to sub-components
	r.state.OnStateChange(func(event StateChangeEvent) {
		r.logger.Info().
			Str("from", event.From.String()).
			Str("to", event.To.String()).
			Str("reason", event.Reason).
			Msg("runtime state change")
		r.events.Publish(ctx, EventStateChange, "runtime", event)
	})

	r.lifecycle.SetLogger(r.logger)

	r.logger = r.logger.With().
		Str("component", "runtime").
		Logger()

	return r
}

// =============================================================================
// Lifecycle
// =============================================================================

// Start initializes all subsystems and starts the runtime.
func (r *Runtime) Start(ctx context.Context) error {
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	return r.start(ctx)
}

func (r *Runtime) start(ctx context.Context) error {
	r.mu.Lock()
	if r.state.Current() != StateUninitialized {
		r.mu.Unlock()
		return fmt.Errorf("runtime already started (state: %s)", r.state.Current())
	}
	r.mu.Unlock()

	log.Ctx(ctx).Info().Msg("runtime starting")

	// Initialize — state invariants guarantee these transitions succeed:
	// Uninitialized → Initializing → Ready → Running are all valid transitions.
	if err := r.state.TransitionTo(StateInitializing, "runtime startup"); err != nil {
		return fmt.Errorf("transition to initializing: %w", err)
	}

	// Initialize lifecycle phases
	if err := r.lifecycle.ExecuteInit(ctx, r); err != nil {
		r.state.SetError(err)
		return fmt.Errorf("runtime initialization: %w", err)
	}

	// Transition to ready
	if err := r.state.TransitionTo(StateReady, "initialization complete"); err != nil {
		return fmt.Errorf("transition to ready: %w", err)
	}

	// Start subsystems
	if err := r.lifecycle.ExecuteStart(ctx, r); err != nil {
		r.state.SetError(err)
		return fmt.Errorf("runtime start: %w", err)
	}

	// Transition to running
	if err := r.state.TransitionTo(StateRunning, "startup complete"); err != nil {
		return fmt.Errorf("transition to running: %w", err)
	}

	// Publish startup complete AFTER all init and start hooks have executed.
	// Subscribers can safely access initialized subsystems at this point.
	r.events.Publish(ctx, EventStartupComplete, "runtime", nil)

	// Start health check loop
	r.wg.Add(1)
	go r.healthCheckLoop()

	r.logger.Info().
		Str("version", r.config.Version).
		Msg("runtime started successfully")

	return nil
}

// Stop gracefully shuts down all subsystems.
func (r *Runtime) Stop(ctx context.Context) error {
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	return r.stop(ctx)
}

func (r *Runtime) stop(ctx context.Context) error {
	r.mu.Lock()
	current := r.state.Current()
	if current == StateStopped || current == StateStopping {
		r.mu.Unlock()
		return nil // Already stopped or stopping
	}
	r.mu.Unlock()

	if current == StateUninitialized {
		return fmt.Errorf("cannot stop runtime in state %s", current)
	}
	r.events.Publish(ctx, EventShutdownInitiated, "runtime", nil)

	if err := r.state.TransitionTo(StateStopping, "shutdown requested"); err != nil {
		return fmt.Errorf("transition to stopping: %w", err)
	}

	// Create a timeout context for the shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, r.config.ShutdownTimeout)
	defer cancel()

	// Execute shutdown lifecycle
	if err := r.lifecycle.ExecuteStop(shutdownCtx, r); err != nil {
		r.logger.Warn().Err(err).Msg("shutdown completed with errors")
	}

	// Cancel the runtime context
	r.cancel()

	// Wait for all goroutines to finish, but do not let a goroutine that ignores
	// cancellation hold shutdown indefinitely.
	waitTimeout := r.config.ShutdownWaitTimeout
	if waitTimeout <= 0 {
		waitTimeout = 10 * time.Second
	}
	if !waitForWaitGroup(&r.wg, waitTimeout) {
		r.logger.Warn().
			Dur("timeout", waitTimeout).
			Msg("runtime shutdown wait timed out; returning with goroutines still running")
	}

	if err := r.state.TransitionTo(StateStopped, "shutdown complete"); err != nil {
		return fmt.Errorf("transition to stopped: %w", err)
	}
	close(r.shutdownCh)

	r.events.Publish(ctx, EventShutdownComplete, "runtime", nil)

	r.logger.Info().Msg("runtime stopped")
	return nil
}

// Restart restarts the runtime by stopping and starting again.
func (r *Runtime) Restart(ctx context.Context) error {
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	r.logger.Info().Msg("runtime restarting")

	stopCtx, cancel := context.WithTimeout(ctx, r.config.ShutdownTimeout)
	defer cancel()

	if err := r.stop(stopCtx); err != nil {
		return fmt.Errorf("stop during restart: %w", err)
	}

	// Transition back to Uninitialized so Start() can proceed.
	// After Stop(), the state is Stopped, but Start() requires Uninitialized.
	// The Stopped→Uninitialized transition is valid per validTransitions.
	if err := r.state.TransitionTo(StateUninitialized, "restart"); err != nil {
		return fmt.Errorf("transition to uninitialized: %w", err)
	}

	// Create a new context for the new runtime
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.shutdownCh = make(chan struct{})

	startCtx, startCancel := context.WithTimeout(ctx, r.config.ComponentTimeout)
	defer startCancel()

	if err := r.start(startCtx); err != nil {
		return fmt.Errorf("start during restart: %w", err)
	}

	r.logger.Info().Msg("runtime restarted successfully")
	return nil
}

// =============================================================================
// Signal Handling
// =============================================================================

// HandleSignals registers signal handlers for graceful shutdown.
// SIGINT triggers graceful stop, SIGTERM triggers force stop.
func (r *Runtime) HandleSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			select {
			case sig := <-sigCh:
				r.logger.Info().Str("signal", sig.String()).Msg("received signal")
				switch sig {
				case syscall.SIGINT:
					r.logger.Info().Msg("initiating graceful shutdown (SIGINT)")
					// Launch Stop in a separate goroutine to avoid deadlock:
					// r.Stop() calls r.wg.Wait(), which would block forever if
					// called from within an r.wg goroutine.
					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), r.config.ShutdownTimeout)
						defer cancel()
						if err := r.Stop(ctx); err != nil {
							r.logger.Error().Err(err).Msg("error during graceful shutdown")
						}
					}()
				case syscall.SIGTERM:
					r.logger.Warn().Msg("initiating force shutdown (SIGTERM)")
					r.cancel()
				case syscall.SIGHUP:
					r.logger.Info().Msg("reloading configuration (SIGHUP)")
					r.events.Publish(context.Background(), EventConfigReload, "runtime", nil)
				}
			case <-r.ctx.Done():
				return
			}
		}
	}()
}

// =============================================================================
// Health
// =============================================================================

// healthCheckLoop periodically checks component health.
func (r *Runtime) healthCheckLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.checkComponentHealth()
		case <-r.ctx.Done():
			return
		}
	}
}

// checkComponentHealth checks all registered subsystems for health.
func (r *Runtime) checkComponentHealth() {
	subsystems := r.getSubsystems()
	for _, sub := range subsystems {
		if sub == nil {
			continue
		}
		health := sub.Health()
		r.state.SetComponentStatus(sub.Name(), health, "")
		r.metrics.SetComponentHealth(sub.Name(), health)
	}

	// Publish health change event
	health := r.state.HealthStatus()
	r.events.Publish(r.ctx, EventHealthChange, "runtime", health)
}

// Health returns the current runtime health.
func (r *Runtime) Health() ComponentStatus {
	return r.state.HealthStatus()
}

// HealthReport returns a detailed health report.
func (r *Runtime) HealthReport() map[string]interface{} {
	state := r.state.Get()
	return map[string]interface{}{
		"state":      state.CurrentState.String(),
		"health":     state.Health,
		"uptime":     state.Uptime.String(),
		"started_at": state.StartedAt,
		"version":    r.config.Version,
		"components": state.Components,
	}
}

// =============================================================================
// State and Metrics Accessors
// =============================================================================

// State returns the current runtime state.
func (r *Runtime) State() *RuntimeState {
	return r.state
}

// Metrics returns the runtime metrics collector.
func (r *Runtime) Metrics() *Metrics {
	return r.metrics
}

// Events returns the runtime event bus.
func (r *Runtime) Events() *EventBus {
	return r.events
}

// Lifecycle returns the lifecycle manager.
func (r *Runtime) Lifecycle() *Lifecycle {
	return r.lifecycle
}

// Config returns the runtime configuration.
func (r *Runtime) Config() RuntimeConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// ShutdownCh returns a channel that is closed when the runtime has shut down.
func (r *Runtime) ShutdownCh() <-chan struct{} {
	return r.shutdownCh
}

// Context returns the runtime context.
func (r *Runtime) Context() context.Context {
	return r.ctx
}

// =============================================================================
// Subsystem Access
// =============================================================================

// Subsystem returns a specific subsystem by name.
func (r *Runtime) Subsystem(name string) Subsystem {
	switch name {
	case "knowledge":
		return r.knowledge
	case "discovery":
		return r.discovery
	case "memory":
		return r.memory
	case "cache":
		return r.cache
	case "plugins":
		return r.plugins
	case "editors":
		return r.editors
	case "watcher":
		return r.watcher
	default:
		return nil
	}
}

// getSubsystems returns all registered subsystems.
func (r *Runtime) getSubsystems() []Subsystem {
	return []Subsystem{
		r.knowledge,
		r.discovery,
		r.memory,
		r.cache,
		r.compute,
		r.plugins,
		r.editors,
		r.watcher,
	}
}

// =============================================================================
// Subsystem Registration
// =============================================================================

// RegisterKnowledge sets the knowledge subsystem.
func (r *Runtime) RegisterKnowledge(s Subsystem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.knowledge = s
}

// RegisterDiscovery sets the discovery subsystem.
func (r *Runtime) RegisterDiscovery(s Subsystem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.discovery = s
}

// RegisterMemory sets the memory subsystem.
func (r *Runtime) RegisterMemory(s Subsystem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memory = s
}

// RegisterCache sets the cache subsystem.
func (r *Runtime) RegisterCache(s Subsystem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = s
}

// RegisterCompute sets the compute fabric subsystem.
func (r *Runtime) RegisterCompute(s Subsystem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.compute = s
}

// RegisterPlugins sets the plugins subsystem.
func (r *Runtime) RegisterPlugins(s Subsystem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins = s
}

// RegisterEditors sets the editors subsystem.
func (r *Runtime) RegisterEditors(s Subsystem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.editors = s
}

// RegisterWatcher sets the watcher subsystem.
func (r *Runtime) RegisterWatcher(s Subsystem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.watcher = s
}

// RegisterDaemon sets the daemon subsystem.
func (r *Runtime) RegisterDaemon(d *Daemon) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.daemon = d
}

// =============================================================================
// Daemon Access
// =============================================================================

// Daemon returns the daemon instance.
func (r *Runtime) Daemon() *Daemon {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.daemon
}
