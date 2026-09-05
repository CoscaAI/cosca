//
// Hook System: extension points throughout the Cosca lifecycle.
// Plugins register hooks to intercept and augment Cosca operations
// such as indexing, search, context building, and command execution.

package plugins

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// =============================================================================
// HookManager (interface for the engine)
// =============================================================================

// HookManager defines the hook execution interface that the AgentEngine
// depends on. Plugins use HookRegistry to register hooks; the engine
// only needs to trigger them.
type HookManager interface {
	ExecuteHooks(point HookPoint, args interface{}) error
}

// =============================================================================
// HookPoint
// =============================================================================

// HookPoint defines specific lifecycle points where plugins can attach hooks.
type HookPoint string

const (
	// HookBeforeIndex runs before indexing begins.
	HookBeforeIndex HookPoint = "before_index"
	// HookAfterIndex runs after indexing completes.
	HookAfterIndex HookPoint = "after_index"
	// HookBeforeSearch runs before a search operation.
	HookBeforeSearch HookPoint = "before_search"
	// HookAfterSearch runs after a search operation completes.
	HookAfterSearch HookPoint = "after_search"
	// HookBeforeContext runs before context building.
	HookBeforeContext HookPoint = "before_context"
	// HookAfterContext runs after context building.
	HookAfterContext HookPoint = "after_context"
	// HookBeforeExecute runs before command/task execution.
	HookBeforeExecute HookPoint = "before_execute"
	// HookAfterExecute runs after command/task execution.
	HookAfterExecute HookPoint = "after_execute"
	// HookPreToolUse runs before a tool is invoked by the agent engine.
	HookPreToolUse HookPoint = "pre_tool_use"
	// HookPostToolUse runs after a tool has been executed by the agent engine.
	HookPostToolUse HookPoint = "post_tool_use"
)

// ValidHookPoints returns all valid hook points.
func ValidHookPoints() []HookPoint {
	return []HookPoint{
		HookBeforeIndex, HookAfterIndex,
		HookBeforeSearch, HookAfterSearch,
		HookBeforeContext, HookAfterContext,
		HookBeforeExecute, HookAfterExecute,
		HookPreToolUse, HookPostToolUse,
	}
}

// IsValidHookPoint checks if the given hook point is valid.
func IsValidHookPoint(p HookPoint) bool {
	switch p {
	case HookBeforeIndex, HookAfterIndex,
		HookBeforeSearch, HookAfterSearch,
		HookBeforeContext, HookAfterContext,
		HookBeforeExecute, HookAfterExecute,
		HookPreToolUse, HookPostToolUse:
		return true
	default:
		return false
	}
}

// =============================================================================
// HookHandler
// =============================================================================

// HookHandler is a function that processes a hook invocation.
// The args parameter contains hook-specific data.
type HookHandler func(args interface{}) error

// registeredHook represents a registered hook handler with its metadata.
type registeredHook struct {
	// id is a unique identifier for this hook registration.
	id string
	// pluginID is the ID of the plugin that registered this hook.
	pluginID string
	// handler is the hook handler function.
	handler HookHandler
	// priority determines execution order (lower = earlier).
	priority int
	// createdAt is when the hook was registered.
	createdAt time.Time
}

// hookEntry is the internal storage for hooks at a given hook point.
type hookEntry struct {
	mu    sync.RWMutex
	hooks []registeredHook
}

// =============================================================================
// HookRegistry
// =============================================================================

// HookRegistry manages the registration and execution of hooks.
// It is thread-safe and supports priority ordering and timeouts.
type HookRegistry struct {
	mu    sync.RWMutex
	hooks map[HookPoint]*hookEntry

	// defaultTimeout is the default timeout for hook execution.
	defaultTimeout time.Duration

	// nextID is a counter for generating unique hook IDs.
	nextID int
}

// NewHookRegistry creates a new hook registry with the given default timeout.
func NewHookRegistry(defaultTimeout time.Duration) *HookRegistry {
	if defaultTimeout <= 0 {
		defaultTimeout = 30 * time.Second
	}

	return &HookRegistry{
		hooks:          make(map[HookPoint]*hookEntry),
		defaultTimeout: defaultTimeout,
	}
}

// RegisterHook registers a hook handler for the given hook point.
// Returns a unique hook ID that can be used to unregister the hook.
func (r *HookRegistry) RegisterHook(point HookPoint, pluginID string, handler HookHandler, priority int) (string, error) {
	if !IsValidHookPoint(point) {
		return "", fmt.Errorf("invalid hook point: %s", point)
	}
	if handler == nil {
		return "", fmt.Errorf("hook handler cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Get or create the hook entry for this point
	entry, exists := r.hooks[point]
	if !exists {
		entry = &hookEntry{}
		r.hooks[point] = entry
	}

	r.nextID++
	hookID := fmt.Sprintf("%s_%s_%d", point, pluginID, r.nextID)

	entry.mu.Lock()
	entry.hooks = append(entry.hooks, registeredHook{
		id:        hookID,
		pluginID:  pluginID,
		handler:   handler,
		priority:  priority,
		createdAt: time.Now(),
	})
	entry.mu.Unlock()

	// Sort hooks by priority (ascending)
	r.sortHooks(point)

	log.Debug().Str("hook_point", string(point)).
		Str("plugin", pluginID).
		Str("hook_id", hookID).
		Int("priority", priority).
		Msg("hook registered")

	return hookID, nil
}

// UnregisterHook removes a previously registered hook by its ID.
func (r *HookRegistry) UnregisterHook(hookID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for point, entry := range r.hooks {
		entry.mu.Lock()
		for i, h := range entry.hooks {
			if h.id == hookID {
				entry.hooks = append(entry.hooks[:i], entry.hooks[i+1:]...)
				entry.mu.Unlock()

				log.Debug().Str("hook_point", string(point)).
					Str("hook_id", hookID).
					Msg("hook unregistered")
				return nil
			}
		}
		entry.mu.Unlock()
	}

	return fmt.Errorf("hook %q not found", hookID)
}

// ExecuteHooks executes all hooks registered for the given hook point.
// Hooks are executed in priority order. If a hook times out or returns
// an error, remaining hooks are still executed (fire-and-forget for errors,
// but the error is logged and collected).
func (r *HookRegistry) ExecuteHooks(point HookPoint, args interface{}) error {
	r.mu.RLock()
	entry, exists := r.hooks[point]
	r.mu.RUnlock()

	if !exists {
		return nil
	}

	entry.mu.RLock()
	hooks := make([]registeredHook, len(entry.hooks))
	copy(hooks, entry.hooks)
	entry.mu.RUnlock()

	if len(hooks) == 0 {
		return nil
	}

	log.Debug().Str("hook_point", string(point)).
		Int("count", len(hooks)).
		Msg("executing hooks")

	var errs []error

	for _, h := range hooks {
		if err := r.executeHookWithTimeout(h, args); err != nil {
			errs = append(errs, fmt.Errorf("hook %s (%s): %w", h.id, h.pluginID, err))
			log.Error().Err(err).
				Str("hook_id", h.id).
				Str("plugin", h.pluginID).
				Str("hook_point", string(point)).
				Msg("hook execution failed")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("hook execution completed with %d error(s): %v", len(errs), errs[0])
	}

	return nil
}

// executeHookWithTimeout runs a single hook with a timeout.
func (r *HookRegistry) executeHookWithTimeout(h registeredHook, args interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.defaultTimeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				done <- fmt.Errorf("hook panicked: %v", rec)
			}
		}()
		done <- h.handler(args)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("hook timed out after %v", r.defaultTimeout)
	}
}

// ListHooks returns all registered hooks grouped by hook point.
func (r *HookRegistry) ListHooks() map[HookPoint][]struct {
	ID        string
	PluginID  string
	Priority  int
	CreatedAt time.Time
} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[HookPoint][]struct {
		ID        string
		PluginID  string
		Priority  int
		CreatedAt time.Time
	})

	for point, entry := range r.hooks {
		entry.mu.RLock()
		list := make([]struct {
			ID        string
			PluginID  string
			Priority  int
			CreatedAt time.Time
		}, 0, len(entry.hooks))
		for _, h := range entry.hooks {
			list = append(list, struct {
				ID        string
				PluginID  string
				Priority  int
				CreatedAt time.Time
			}{
				ID:        h.id,
				PluginID:  h.pluginID,
				Priority:  h.priority,
				CreatedAt: h.createdAt,
			})
		}
		entry.mu.RUnlock()
		result[point] = list
	}

	return result
}

// ClearPluginHooks removes all hooks registered by a specific plugin.
func (r *HookRegistry) ClearPluginHooks(pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for point, entry := range r.hooks {
		entry.mu.Lock()
		filtered := make([]registeredHook, 0, len(entry.hooks))
		for _, h := range entry.hooks {
			if h.pluginID != pluginID {
				filtered = append(filtered, h)
			}
		}
		entry.hooks = filtered
		entry.mu.Unlock()

		_ = point // unused but clear for clarity
	}

	log.Debug().Str("plugin", pluginID).Msg("all hooks cleared for plugin")
}

// sortHooks sorts hooks for a given hook point by priority (ascending).
func (r *HookRegistry) sortHooks(point HookPoint) {
	entry, exists := r.hooks[point]
	if !exists {
		return
	}

	entry.mu.Lock()
	sort.Slice(entry.hooks, func(i, j int) bool {
		if entry.hooks[i].priority != entry.hooks[j].priority {
			return entry.hooks[i].priority < entry.hooks[j].priority
		}
		return entry.hooks[i].createdAt.Before(entry.hooks[j].createdAt)
	})
	entry.mu.Unlock()
}

// =============================================================================
// Default Hook Registry
// =============================================================================

// defaultRegistry is the package-level hook registry used by the lifecycle manager.
var (
	defaultRegistry     *HookRegistry
	defaultRegistryOnce sync.Once
)

// DefaultHookRegistry returns the package-level default hook registry.
func DefaultHookRegistry() *HookRegistry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewHookRegistry(30 * time.Second)
	})
	return defaultRegistry
}

// RegisterGlobalHook registers a hook in the default registry.
func RegisterGlobalHook(point HookPoint, pluginID string, handler HookHandler, priority int) (string, error) {
	return DefaultHookRegistry().RegisterHook(point, pluginID, handler, priority)
}

// ExecuteGlobalHooks executes hooks in the default registry.
func ExecuteGlobalHooks(point HookPoint, args interface{}) error {
	return DefaultHookRegistry().ExecuteHooks(point, args)
}
