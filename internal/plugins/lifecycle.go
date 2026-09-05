//
// Lifecycle Management: coordinates the initialization, startup,
// shutdown, and health checking of all plugins in the correct order.

package plugins

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// =============================================================================
// LifecycleManager
// =============================================================================

// LifecycleManager orchestrates the plugin lifecycle across all loaded plugins.
// It ensures proper ordering (dependencies before dependents) and
// graceful shutdown (reverse order).
type LifecycleManager struct {
	mu sync.RWMutex

	// manager is the underlying plugin manager.
	manager *Manager

	// hookRegistry is used for hook management.
	hookRegistry *HookRegistry

	// eventBus is used for event publishing.
	eventBus *EventBus

	// initialized tracks which plugins have been initialized.
	initialized map[string]bool

	// started tracks which plugins have been started.
	started map[string]bool

	// healthResults caches the latest health check results.
	healthResults map[string]PluginHealth

	// startedAt is when the lifecycle manager was started.
	startedAt time.Time
}

// NewLifecycleManager creates a new lifecycle manager.
func NewLifecycleManager(manager *Manager, hookRegistry *HookRegistry, eventBus *EventBus) *LifecycleManager {
	if hookRegistry == nil {
		hookRegistry = DefaultHookRegistry()
	}
	if eventBus == nil {
		eventBus = DefaultEventBus()
	}

	return &LifecycleManager{
		manager:       manager,
		hookRegistry:  hookRegistry,
		eventBus:      eventBus,
		initialized:   make(map[string]bool),
		started:       make(map[string]bool),
		healthResults: make(map[string]PluginHealth),
	}
}

// =============================================================================
// Initialization
// =============================================================================

// InitPlugins initializes all enabled plugins in dependency order.
func (lm *LifecycleManager) InitPlugins() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	plugins := lm.manager.List()
	if len(plugins) == 0 {
		log.Debug().Msg("no plugins to initialize")
		return nil
	}

	// Filter to only enabled plugins
	enabled := make([]PluginInfo, 0)
	for _, p := range plugins {
		if p.Enabled {
			enabled = append(enabled, p)
		}
	}

	if len(enabled) == 0 {
		log.Debug().Msg("no enabled plugins to initialize")
		return nil
	}

	// Sort by dependency order (resolve dependencies first)
	ordered, err := lm.orderByDependencies(enabled)
	if err != nil {
		return fmt.Errorf("dependency ordering: %w", err)
	}

	log.Info().Int("count", len(ordered)).Msg("initializing plugins")

	var errs []error

	for _, info := range ordered {
		pluginID := info.Manifest.ID

		// Get the plugin instance from the manager
		entry, exists := lm.getPlugin(pluginID)
		if !exists {
			errs = append(errs, fmt.Errorf("plugin %q: not loaded", pluginID))
			continue
		}

		// Create plugin context
		ctx := NewPluginContext(
			make(map[string]interface{}),
			&pluginLogger{pluginID: pluginID},
			lm.getPluginDataDir(pluginID),
			&pluginRuntimeAPI{
				pluginID:     pluginID,
				hookRegistry: lm.hookRegistry,
				eventBus:     lm.eventBus,
			},
		)

		// Initialize the plugin
		if initErr := entry.plugin.Init(ctx); initErr != nil {
			errs = append(errs, fmt.Errorf("plugin %q: init: %w", pluginID, initErr))
			entry.info.State = PluginStateError

			// Publish error event
			lm.eventBus.Publish(NewEvent(EventPluginError, "lifecycle", map[string]interface{}{
				"plugin": pluginID,
				"error":  initErr.Error(),
				"phase":  "init",
			}))

			continue
		}

		entry.info.State = PluginStateInitialized
		lm.initialized[pluginID] = true
		lm.updatePlugin(pluginID, entry)

		log.Info().Str("plugin", pluginID).Msg("plugin initialized")
	}

	if len(errs) > 0 {
		return fmt.Errorf("plugin initialization: %d error(s): %v", len(errs), errs[0])
	}

	return nil
}

// =============================================================================
// Startup
// =============================================================================

// StartPlugins starts all initialized plugins in dependency order.
func (lm *LifecycleManager) StartPlugins() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Get initialized plugins in dependency order
	ordered, err := lm.getInitializedInOrder()
	if err != nil {
		return err
	}

	if len(ordered) == 0 {
		log.Debug().Msg("no plugins to start")
		return nil
	}

	lm.startedAt = time.Now()
	log.Info().Int("count", len(ordered)).Msg("starting plugins")

	var errs []error

	for _, pluginID := range ordered {
		entry, exists := lm.getPlugin(pluginID)
		if !exists {
			errs = append(errs, fmt.Errorf("plugin %q: not found during start", pluginID))
			continue
		}

		if entry.info.State != PluginStateInitialized {
			log.Debug().Str("plugin", pluginID).
				Str("state", entry.info.State.String()).
				Msg("skipping plugin start (not initialized)")
			continue
		}

		// Start the plugin
		if startErr := entry.plugin.Start(); startErr != nil {
			errs = append(errs, fmt.Errorf("plugin %q: start: %w", pluginID, startErr))
			entry.info.State = PluginStateError

			lm.eventBus.Publish(NewEvent(EventPluginError, "lifecycle", map[string]interface{}{
				"plugin": pluginID,
				"error":  startErr.Error(),
				"phase":  "start",
			}))

			continue
		}

		entry.info.State = PluginStateStarted
		entry.startedAt = time.Now()
		lm.started[pluginID] = true
		lm.updatePlugin(pluginID, entry)

		// Publish started event
		lm.eventBus.Publish(NewEvent(EventPluginStarted, "lifecycle", map[string]interface{}{
			"plugin": pluginID,
		}))

		log.Info().Str("plugin", pluginID).Msg("plugin started")
	}

	if len(errs) > 0 {
		return fmt.Errorf("plugin startup: %d error(s): %v", len(errs), errs[0])
	}

	return nil
}

// =============================================================================
// Graceful Shutdown
// =============================================================================

// StopPlugins gracefully stops all running plugins in reverse dependency order.
func (lm *LifecycleManager) StopPlugins() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Get started plugins in reverse dependency order
	ordered, err := lm.getStartedInReverseOrder()
	if err != nil {
		return err
	}

	if len(ordered) == 0 {
		log.Debug().Msg("no plugins to stop")
		return nil
	}

	log.Info().Int("count", len(ordered)).Msg("stopping plugins")

	var errs []error

	for _, pluginID := range ordered {
		entry, exists := lm.getPlugin(pluginID)
		if !exists {
			continue
		}

		if entry.info.State != PluginStateStarted {
			continue
		}

		// Stop with timeout (soft grace period)
		stopDone := make(chan error, 1)
		go func() {
			stopDone <- entry.plugin.Stop()
		}()

		timer := time.NewTimer(10 * time.Second)
		select {
		case stopErr := <-stopDone:
			if !timer.Stop() {
				<-timer.C
			}
			if stopErr != nil {
				errs = append(errs, fmt.Errorf("plugin %q: stop: %w", pluginID, stopErr))
				entry.info.State = PluginStateError
			} else {
				entry.info.State = PluginStateStopped
				log.Info().Str("plugin", pluginID).Msg("plugin stopped")
			}
		case <-timer.C:
			errs = append(errs, fmt.Errorf("plugin %q: stop timed out", pluginID))
			entry.info.State = PluginStateError
		}

		delete(lm.started, pluginID)

		// Clear plugin hooks and subscribers
		lm.hookRegistry.ClearPluginHooks(pluginID)

		// Publish stopped event
		lm.eventBus.Publish(NewEvent(EventPluginStopped, "lifecycle", map[string]interface{}{
			"plugin": pluginID,
		}))

		lm.updatePlugin(pluginID, entry)
	}

	// Clear all tracking
	lm.initialized = make(map[string]bool)
	lm.started = make(map[string]bool)
	lm.healthResults = make(map[string]PluginHealth)

	if len(errs) > 0 {
		return fmt.Errorf("plugin shutdown: %d error(s): %v", len(errs), errs[0])
	}

	return nil
}

// =============================================================================
// Health Checking
// =============================================================================

// HealthCheckAll performs health checks on all started plugins.
func (lm *LifecycleManager) HealthCheckAll() []PluginHealth {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	var results []PluginHealth

	for pluginID, entry := range lm.getPluginMap() {
		if entry.info.State != PluginStateStarted {
			continue
		}

		health, err := entry.plugin.Health()
		if err != nil {
			health = PluginHealth{
				PluginID:  pluginID,
				Status:    "unhealthy",
				Message:   err.Error(),
				LastCheck: time.Now(),
				State:     entry.info.State,
			}
		}

		if health.LastCheck.IsZero() {
			health.LastCheck = time.Now()
		}

		if health.PluginID == "" {
			health.PluginID = pluginID
		}

		if health.State == 0 {
			health.State = entry.info.State
		}

		lm.healthResults[pluginID] = health
		results = append(results, health)
	}

	// Sort for deterministic output
	sort.Slice(results, func(i, j int) bool {
		return results[i].PluginID < results[j].PluginID
	})

	return results
}

// GetPluginHealth returns the latest health check result for a plugin.
func (lm *LifecycleManager) GetPluginHealth(pluginID string) (PluginHealth, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	health, exists := lm.healthResults[pluginID]
	if !exists {
		return PluginHealth{}, fmt.Errorf("no health data for plugin %q", pluginID)
	}
	return health, nil
}

// =============================================================================
// Status
// =============================================================================

// LifecycleStatus provides a summary of the current plugin lifecycle status.
type LifecycleStatus struct {
	TotalPlugins   int            `json:"total_plugins"`
	Initialized    int            `json:"initialized"`
	Started        int            `json:"started"`
	Stopped        int            `json:"stopped"`
	Errors         int            `json:"errors"`
	HealthyCount   int            `json:"healthy_count"`
	UnhealthyCount int            `json:"unhealthy_count"`
	Uptime         time.Duration  `json:"uptime"`
	HealthResults  []PluginHealth `json:"health_results,omitempty"`
}

// Status returns the current lifecycle status summary.
func (lm *LifecycleManager) Status() LifecycleStatus {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	status := LifecycleStatus{}
	for _, entry := range lm.getPluginMap() {
		status.TotalPlugins++
		switch entry.info.State {
		case PluginStateInitialized:
			status.Initialized++
		case PluginStateStarted:
			status.Started++
		case PluginStateStopped:
			status.Stopped++
		case PluginStateError:
			status.Errors++
		}
	}

	for _, health := range lm.healthResults {
		if health.IsHealthy() {
			status.HealthyCount++
		} else {
			status.UnhealthyCount++
		}
	}

	if !lm.startedAt.IsZero() {
		status.Uptime = time.Since(lm.startedAt)
	}

	return status
}

// =============================================================================
// Internal Helpers
// =============================================================================

// orderByDependencies sorts plugins by their dependencies
// (topological order: dependencies first).
func (lm *LifecycleManager) orderByDependencies(plugins []PluginInfo) ([]PluginInfo, error) {
	// Kahn's algorithm expects graph[node] = DEPENDENTS (quem depende de
	// node): nodes com in-degree 0 (sem dependências) saem primeiro. O
	// esquema anterior montava graph[node] = dependências de node, o que
	// invertia a ordem — dependentes eram inicializados ANTES das suas
	// dependências (bug de lifecycle: um plugin podia iniciar sem o core).
	graph := make(map[string][]string)
	for _, p := range plugins {
		graph[p.Manifest.ID] = []string{}
	}
	for _, p := range plugins {
		for _, dep := range p.Manifest.Dependencies {
			graph[dep.PluginID] = append(graph[dep.PluginID], p.Manifest.ID)
		}
	}

	order, err := topologicalSort(graph)
	if err != nil {
		return nil, err
	}

	// Map order back to PluginInfo
	pluginMap := make(map[string]PluginInfo)
	for _, p := range plugins {
		pluginMap[p.Manifest.ID] = p
	}

	result := make([]PluginInfo, 0, len(order))
	for _, id := range order {
		if p, ok := pluginMap[id]; ok {
			result = append(result, p)
		}
	}

	return result, nil
}

// getInitializedInOrder returns initialized plugin IDs in dependency order.
func (lm *LifecycleManager) getInitializedInOrder() ([]string, error) {
	var initialized []PluginInfo
	for id := range lm.initialized {
		if entry, exists := lm.getPlugin(id); exists {
			initialized = append(initialized, entry.info)
		}
	}

	ordered, err := lm.orderByDependencies(initialized)
	if err != nil {
		return nil, err
	}

	result := make([]string, len(ordered))
	for i, p := range ordered {
		result[i] = p.Manifest.ID
	}
	return result, nil
}

// getStartedInReverseOrder returns started plugin IDs in reverse dependency order.
func (lm *LifecycleManager) getStartedInReverseOrder() ([]string, error) {
	var started []PluginInfo
	for id := range lm.started {
		if entry, exists := lm.getPlugin(id); exists {
			started = append(started, entry.info)
		}
	}

	ordered, err := lm.orderByDependencies(started)
	if err != nil {
		return nil, err
	}

	// Reverse the order
	result := make([]string, len(ordered))
	for i, p := range ordered {
		result[len(ordered)-1-i] = p.Manifest.ID
	}
	return result, nil
}

// getPlugin retrieves a plugin entry from the manager.
func (lm *LifecycleManager) getPlugin(id string) (pluginEntry, bool) {
	p, err := lm.manager.GetPlugin(id)
	if err != nil {
		return pluginEntry{}, false
	}
	info, err := lm.manager.Get(id)
	if err != nil {
		return pluginEntry{}, false
	}
	return pluginEntry{
		plugin: p,
		info:   info,
	}, true
}

// getPluginMap returns all plugins as a map.
func (lm *LifecycleManager) getPluginMap() map[string]pluginEntry {
	result := make(map[string]pluginEntry)
	plugins := lm.manager.List()
	for _, info := range plugins {
		result[info.Manifest.ID] = pluginEntry{
			info: info,
		}
	}
	return result
}

// updatePlugin updates a plugin's state in the manager.
func (lm *LifecycleManager) updatePlugin(id string, entry pluginEntry) {
	// In a real implementation, this would update the manager's internal state.
	// The manager's List() would reflect the updated state.
	_ = id
	_ = entry
}

// getPluginDataDir returns the data directory for a plugin.
func (lm *LifecycleManager) getPluginDataDir(pluginID string) string {
	return fmt.Sprintf("%s/%s/data", lm.manager.pluginsDir, pluginID)
}

// =============================================================================
// Plugin Logger
// =============================================================================

// pluginLogger implements the PluginLogger interface using zerolog.
type pluginLogger struct {
	pluginID string
}

func (l *pluginLogger) Debug(msg string, keysAndValues ...interface{}) {
	log.Debug().Str("plugin", l.pluginID).Fields(keysAndValues).Msg(msg)
}

func (l *pluginLogger) Info(msg string, keysAndValues ...interface{}) {
	log.Info().Str("plugin", l.pluginID).Fields(keysAndValues).Msg(msg)
}

func (l *pluginLogger) Warn(msg string, keysAndValues ...interface{}) {
	log.Warn().Str("plugin", l.pluginID).Fields(keysAndValues).Msg(msg)
}

func (l *pluginLogger) Error(msg string, keysAndValues ...interface{}) {
	log.Error().Str("plugin", l.pluginID).Fields(keysAndValues).Msg(msg)
}

// =============================================================================
// Plugin Runtime API
// =============================================================================

// pluginRuntimeAPI implements RuntimeAPI for plugins.
type pluginRuntimeAPI struct {
	pluginID     string
	hookRegistry *HookRegistry
	eventBus     *EventBus
}

func (a *pluginRuntimeAPI) GetConfig(_ string) (interface{}, error) {
	return nil, fmt.Errorf("runtime config not available")
}

func (a *pluginRuntimeAPI) SetConfig(_ string, _ interface{}) error {
	return fmt.Errorf("runtime config not available")
}

func (a *pluginRuntimeAPI) EmitEvent(eventType string, data interface{}) error {
	event := NewEvent(eventType, a.pluginID, data)
	a.eventBus.Publish(event)
	return nil
}

func (a *pluginRuntimeAPI) RegisterHook(hookPoint string, handler func(args interface{}) error) (string, error) {
	return a.hookRegistry.RegisterHook(HookPoint(hookPoint), a.pluginID, handler, 100)
}

func (a *pluginRuntimeAPI) UnregisterHook(hookID string) error {
	return a.hookRegistry.UnregisterHook(hookID)
}
