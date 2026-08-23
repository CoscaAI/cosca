package worldmodel

import (
	"context"
	"fmt"
	"sync"
)

// ──────────────────────────────────────────────────────────────
// Adapter lifecycle
// ──────────────────────────────────────────────────────────────

// AdapterState represents the lifecycle state of an adapter.
type AdapterState string

const (
	AdapterCreated  AdapterState = "created"
	AdapterStarting AdapterState = "starting"
	AdapterRunning  AdapterState = "running"
	AdapterStopping AdapterState = "stopping"
	AdapterStopped  AdapterState = "stopped"
	AdapterError    AdapterState = "error"
)

// Adapter is the base interface for all adapters.
// Each adapter wraps an external tool (CLIP, SAM, Whisper, etc.)
// and exposes it through a provider interface.
type Adapter interface {
	// Name returns the adapter name (e.g., "clip", "sam2", "whisper").
	Name() string

	// State returns the current lifecycle state.
	State() AdapterState

	// Start initializes the adapter (load model, start subprocess).
	Start(ctx context.Context) error

	// Stop gracefully shuts down the adapter.
	Stop(ctx context.Context) error

	// Health returns nil if the adapter is healthy.
	Health(ctx context.Context) error
}

// ──────────────────────────────────────────────────────────────
// Adapter registry
// ──────────────────────────────────────────────────────────────

// AdapterConfig holds configuration for an adapter.
type AdapterConfig struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`     // "vision", "spatial", "audio", "vfx", "destruction", "simulation"
	Command  string            `json:"command"`  // subprocess command (e.g., "python", "clip_server")
	Args     []string          `json:"args"`     // command arguments
	WorkDir  string            `json:"work_dir"` // working directory
	Env      map[string]string `json:"env"`      // environment variables
	Timeout  int               `json:"timeout"`  // seconds (0 = no timeout)
	MaxRAM   int               `json:"max_ram"`  // MB (0 = unlimited)
	MaxGPU   int               `json:"max_gpu"`  // MB VRAM (0 = no GPU)
}

// Registry manages all registered adapters.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
	configs  map[string]AdapterConfig
}

// NewRegistry creates a new adapter registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]Adapter),
		configs:  make(map[string]AdapterConfig),
	}
}

// Register adds an adapter to the registry.
func (r *Registry) Register(a Adapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := a.Name()
	if _, exists := r.adapters[name]; exists {
		return fmt.Errorf("adapter %q already registered", name)
	}

	r.adapters[name] = a
	return nil
}

// Get returns an adapter by name.
func (r *Registry) Get(name string) (Adapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.adapters[name]
	if !ok {
		return nil, fmt.Errorf("adapter %q not found", name)
	}
	return a, nil
}

// List returns all registered adapter names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}

// StartAll starts all registered adapters.
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, a := range r.adapters {
		if err := a.Start(ctx); err != nil {
			return fmt.Errorf("failed to start adapter %q: %w", name, err)
		}
	}
	return nil
}

// StopAll stops all registered adapters.
func (r *Registry) StopAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var firstErr error
	for name, a := range r.adapters {
		if err := a.Stop(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to stop adapter %q: %w", name, err)
		}
	}
	return firstErr
}

// HealthAll checks health of all adapters.
func (r *Registry) HealthAll(ctx context.Context) map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]error, len(r.adapters))
	for name, a := range r.adapters {
		results[name] = a.Health(ctx)
	}
	return results
}
