package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── Registry ────────────────────────────────────────────────────────────────

// Registry manages multiple LLM providers with primary + fallback selection.
type Registry struct {
	providers map[string]chat.Provider
	primary   string
	fallbacks []string
	mu        sync.RWMutex
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]chat.Provider),
	}
}

// Register adds or replaces a provider in the registry by name.
func (r *Registry) Register(p chat.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

// Get returns the primary provider. If the primary is not set or is unavailable,
// it falls back to the first available provider in the fallback list. If no
// provider is available, it returns an error.
func (r *Registry) Get(ctx context.Context) (chat.Provider, error) {
	r.mu.RLock()
	primary := r.primary
	fallbacks := r.fallbacks
	providers := r.providers
	r.mu.RUnlock()

	// Try primary first
	if primary != "" {
		if p, ok := providers[primary]; ok && p.IsAvailable() {
			return p, nil
		}
	}

	// Try fallbacks
	for _, name := range fallbacks {
		if name == primary {
			continue
		}
		if p, ok := providers[name]; ok && p.IsAvailable() {
			return p, nil
		}
	}

	// Last resort: return any available provider
	// Prefer the primary name even if not explicitly set as primary
	for name, p := range providers {
		if name == primary || len(fallbacks) == 0 {
			if p.IsAvailable() {
				return p, nil
			}
		}
	}

	return nil, fmt.Errorf("no available provider: checked primary=%q fallbacks=[%s]",
		primary, strings.Join(fallbacks, ","))
}

// SetPrimary sets the primary provider name.
func (r *Registry) SetPrimary(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.primary = name
}

// SetFallbacks sets the ordered fallback provider names.
func (r *Registry) SetFallbacks(names []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallbacks = names
}

// Available returns the names of all registered providers.
func (r *Registry) Available() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// Primary returns the name of the primary provider, if set.
func (r *Registry) Primary() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.primary
}

// Fallbacks returns the ordered list of fallback provider names.
func (r *Registry) Fallbacks() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, len(r.fallbacks))
	copy(result, r.fallbacks)
	return result
}

// GetProvider returns a specific provider by name, or nil if not registered.
func (r *Registry) GetProvider(name string) chat.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providers[name]
}
