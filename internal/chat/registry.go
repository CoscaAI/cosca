package chat

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CoscaAI/cosca/internal/router"
	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog/log"
)

// globalRegistryMu protects globalChatRegistry from concurrent access.
var globalRegistryMu sync.RWMutex

// ─── Factory ────────────────────────────────────────────────────────────────

// ChatProviderFactory is a function that creates a ChatProvider instance.
//
//nolint:revive // Stutter name preserved for API compatibility — used as chat.ChatProviderFactory externally.
type ChatProviderFactory func(ctx context.Context, cfg map[string]interface{}) (ChatProvider, error)

// ─── Registered Provider ────────────────────────────────────────────────────

// chatProviderHolder wraps a ChatProvider for atomic storage.
// atomic.Pointer cannot hold interface values directly, so we wrap it.
type chatProviderHolder struct {
	provider ChatProvider
}

// registeredChatProvider holds a chat provider factory and its metadata.
type registeredChatProvider struct {
	name        string
	factory     ChatProviderFactory
	description string
	priority    int // lower = higher priority
	mu          sync.Mutex
	instance    atomic.Pointer[chatProviderHolder] // cached instance holder
}

// ─── Stats ──────────────────────────────────────────────────────────────────

// ChatStats tracks chat usage statistics.
//
//nolint:revive // Stutter name preserved for API compatibility — used as chat.ChatStats externally.
type ChatStats struct {
	TotalRequests int64 `json:"total_requests"`
	TotalTokens   int64 `json:"total_tokens"`
	Errors        int64 `json:"errors"`
}

// RecordRequest records a successful chat request with token usage.
func (s *ChatStats) RecordRequest(tokens int) {
	atomic.AddInt64(&s.TotalRequests, 1)
	atomic.AddInt64(&s.TotalTokens, int64(tokens))
}

// RecordError records a failed chat request.
func (s *ChatStats) RecordError() {
	atomic.AddInt64(&s.Errors, 1)
}

// Snapshot returns a copy of the current stats.
func (s *ChatStats) Snapshot() ChatStats {
	return ChatStats{
		TotalRequests: atomic.LoadInt64(&s.TotalRequests),
		TotalTokens:   atomic.LoadInt64(&s.TotalTokens),
		Errors:        atomic.LoadInt64(&s.Errors),
	}
}

// ─── Registry ───────────────────────────────────────────────────────────────

// ChatRegistryConfig defines configuration for the chat registry.
//
//nolint:revive // Stutter name preserved for API compatibility — used as chat.ChatRegistryConfig externally.
type ChatRegistryConfig struct {
	// Primary is the name of the primary provider to use.
	Primary string

	// Fallbacks is an ordered list of fallback provider names.
	Fallbacks []string

	// AutoDetect enables automatic detection of available providers.
	AutoDetect bool

	// Mode enables weighted provider routing (see router.Mode). An empty
	// value (the default) keeps the historical fixed-order chain exactly as
	// before. Valid values are case-insensitive: "balanced", "fast", "cheap",
	// "quality", "offline". Routing is fail-open: an invalid mode or a ranking
	// failure logs a warning and preserves the current order.
	Mode string
}

// DefaultChatRegistryConfig returns sensible defaults for the chat registry.
func DefaultChatRegistryConfig() ChatRegistryConfig {
	return ChatRegistryConfig{
		AutoDetect: true,
	}
}

// ChatRegistry manages available chat providers, their selection, fallback
// chains, and usage statistics. The registry itself implements ChatProvider
// so consumers interact with a single unified interface.
//
//nolint:revive // Stutter name preserved for API compatibility — used as chat.ChatRegistry externally.
type ChatRegistry struct {
	mu         sync.RWMutex
	providers  []registeredChatProvider
	selected   ChatProvider
	fallbacks  []ChatProvider
	stats      *ChatStats
	autoDetect bool
	metrics    map[string]*providerHealth
}

// ─── Global Singleton ───────────────────────────────────────────────────────

var globalChatRegistry *ChatRegistry

// NewChatRegistry creates a new, independent chat registry with its own
// provider factories, selection state, and usage statistics. Prefer this over
// the global GetRegistry() singleton when a registry is needed per request or
// per test so that registries never leak state into each other.
func NewChatRegistry() *ChatRegistry {
	return &ChatRegistry{
		stats:   &ChatStats{},
		metrics: make(map[string]*providerHealth),
	}
}

// GetRegistry returns the global ChatRegistry singleton.
func GetRegistry() *ChatRegistry {
	globalRegistryMu.RLock()
	if globalChatRegistry != nil {
		globalRegistryMu.RUnlock()
		return globalChatRegistry
	}
	globalRegistryMu.RUnlock()

	globalRegistryMu.Lock()
	defer globalRegistryMu.Unlock()
	if globalChatRegistry == nil {
		globalChatRegistry = &ChatRegistry{
			stats:   &ChatStats{},
			metrics: make(map[string]*providerHealth),
		}
	}
	return globalChatRegistry
}

// ResetRegistry recreates the global registry for testing.
func ResetRegistry() {
	globalRegistryMu.Lock()
	globalChatRegistry = &ChatRegistry{
		stats:   &ChatStats{},
		metrics: make(map[string]*providerHealth),
	}
	globalRegistryMu.Unlock()
}

// ─── Registration ───────────────────────────────────────────────────────────

// Register adds a chat provider factory to the registry. If a provider with
// the same name already exists, it is replaced. Providers are sorted by
// priority (lower = higher) on each registration.
func (r *ChatRegistry) Register(name string, factory ChatProviderFactory, description string, priority int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Replace existing entry if present
	for i := range r.providers {
		p := &r.providers[i]
		if p.name == name {
			r.providers[i] = registeredChatProvider{
				name: name, factory: factory, description: description, priority: priority,
			}
			log.Debug().Str("provider", name).Int("priority", priority).Msg("chat provider re-registered")
			return
		}
	}

	r.providers = append(r.providers, registeredChatProvider{
		name: name, factory: factory, description: description, priority: priority,
	})

	sort.Slice(r.providers, func(i, j int) bool {
		return r.providers[i].priority < r.providers[j].priority
	})

	log.Debug().Str("provider", name).Int("priority", priority).Msg("chat provider registered")
}

// ─── Lookup ─────────────────────────────────────────────────────────────────

// Get returns the provider with the given name by instantiating it through its
// registered factory. Returns the provider and true if found, nil and false otherwise.
func (r *ChatRegistry) Get(name string) (ChatProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.providers {
		p := &r.providers[i]
		if p.name == name {
			return p.getOrCreate(), true
		}
	}
	return nil, false
}

// getOrCreate returns the cached provider instance, creating it on first call.
// Thread-safe via atomic load + mutex double-check locking.
func (p *registeredChatProvider) getOrCreate() ChatProvider {
	// Fast path: instance already cached (atomic load, no lock)
	if h := p.instance.Load(); h != nil {
		return h.provider
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring lock
	if h := p.instance.Load(); h != nil {
		return h.provider
	}

	provider, err := p.factory(context.Background(), nil)
	if err != nil {
		log.Warn().Err(err).Str("provider", p.name).Msg("failed to create chat provider instance")
		return nil
	}
	p.instance.Store(&chatProviderHolder{provider: provider})
	return provider
}

// Invalidate clears the cached instance, forcing recreation on next Get().
// Useful when provider configuration changes at runtime (e.g., API key rotation).
func (p *registeredChatProvider) Invalidate() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.instance.Store(nil) // nil holder signals "needs recreation"
}

// List returns the names of all registered providers in priority order.
func (r *ChatRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, len(r.providers))
	for i := range r.providers {
		p := &r.providers[i]
		names[i] = p.name
	}
	return names
}

// ─── Selection ──────────────────────────────────────────────────────────────

// Select initializes the provider chain based on the supplied configuration.
// It instantiates the primary provider and all fallbacks, returning an error
// if no provider can be initialized.
func (r *ChatRegistry) Select(ctx context.Context, cfg ChatRegistryConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.autoDetect = cfg.AutoDetect

	// Build selection order
	var candidates []string

	if cfg.Primary != "" {
		candidates = append(candidates, cfg.Primary)
	}
	if cfg.AutoDetect {
		autoCandidates := r.autoDetectProviders()
		for _, name := range autoCandidates {
			if name != cfg.Primary {
				candidates = append(candidates, name)
			}
		}
	}
	candidates = append(candidates, cfg.Fallbacks...)

	// Try each candidate until one succeeds
	var primary ChatProvider
	var fallbacks []ChatProvider

	for _, name := range candidates {
		provider, err := r.createProvider(ctx, name)
		if err != nil {
			log.Debug().Err(err).Str("provider", name).Msg("chat provider unavailable, skipping")
			continue
		}

		if primary == nil {
			primary = provider
			log.Info().Str("provider", name).Str("model", provider.Model()).Msg("primary chat provider selected")
		} else {
			fallbacks = append(fallbacks, provider)
			log.Debug().Str("provider", name).Msg("fallback chat provider registered")
		}
	}

	if primary == nil {
		return fmt.Errorf("no chat provider could be initialized")
	}

	// Opt-in weighted routing. When cfg.Mode is empty the chain stays in its
	// historical fixed order. Routing is fail-open: an invalid mode or a
	// ranking error logs a warning and keeps the current order untouched.
	if cfg.Mode != "" {
		mode, err := router.ModeFromString(cfg.Mode)
		if err != nil {
			log.Warn().Err(err).Str("mode", cfg.Mode).
				Msg("router mode invalid, keeping provider order")
		} else if newPrimary, newFallbacks, ok := r.reorderForMode(mode, primary, fallbacks); ok {
			primary, fallbacks = newPrimary, newFallbacks
			log.Info().Str("mode", mode.String()).
				Strs("order", providerNames(append([]ChatProvider{primary}, fallbacks...))).
				Msg("chat providers routed by mode")
		}
	}

	r.selected = primary
	r.fallbacks = fallbacks
	return nil
}

// ─── ChatProvider Interface Implementation ──────────────────────────────────

// Chat sends a synchronous chat completion request. It tries the primary
// provider first, then falls back to each configured fallback on error.
func (r *ChatRegistry) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error) {
	r.mu.RLock()
	primary := r.selected
	fallbacks := r.fallbacks
	r.mu.RUnlock()

	if primary == nil {
		return nil, fmt.Errorf("no chat provider selected")
	}

	// Try primary
	start := time.Now()
	response, err := primary.Chat(ctx, messages, opts)
	if err == nil {
		r.providerHealth(primary.Name()).recordSuccess(time.Since(start))
		r.stats.RecordRequest(response.Usage.TotalTokens)
		return response, nil
	}
	r.providerHealth(primary.Name()).recordFailure()

	log.Warn().Err(err).Str("provider", primary.Name()).Msg("primary chat provider failed, trying fallbacks")

	// Try fallbacks
	for _, fb := range fallbacks {
		start := time.Now()
		response, err := fb.Chat(ctx, messages, opts)
		if err == nil {
			r.providerHealth(fb.Name()).recordSuccess(time.Since(start))
			r.stats.RecordRequest(response.Usage.TotalTokens)
			return response, nil
		}
		r.providerHealth(fb.Name()).recordFailure()
		log.Warn().Err(err).Str("provider", fb.Name()).Msg("fallback chat provider failed")
	}

	r.stats.RecordError()
	return nil, fmt.Errorf("all chat providers failed")
}

// ChatStream initiates a streaming chat completion request. It tries the
// primary provider first, then falls back to each configured fallback on
// error. Every successfully opened stream is wrapped in a failoverStream so a
// provider that dies mid-flight (Recv error after content has been delivered)
// transparently reconnects to the next provider in the chain instead of
// killing the stream.
func (r *ChatRegistry) ChatStream(ctx context.Context, messages []Message, opts ChatOptions) (ChatStream, error) {
	// Force stream mode in options so downstream providers behave correctly
	opts.Stream = true

	r.mu.RLock()
	primary := r.selected
	fallbacks := r.fallbacks
	r.mu.RUnlock()

	if primary == nil {
		return nil, fmt.Errorf("no chat provider selected")
	}

	// Full provider chain in order: [primary, fallbacks...]. The winning
	// stream is wrapped with providerIdx pointing at the provider that opened
	// it, so mid-stream failover starts from the next provider in the chain.
	providers := append([]ChatProvider{primary}, fallbacks...)

	for i, p := range providers {
		start := time.Now()
		stream, err := p.ChatStream(ctx, messages, opts)
		if err == nil {
			r.providerHealth(p.Name()).recordSuccess(time.Since(start))
			return newFailoverStream(ctx, messages, opts, providers, i, stream), nil
		}
		r.providerHealth(p.Name()).recordFailure()
		log.Warn().Err(err).Str("provider", p.Name()).Msg("chat stream provider failed, trying fallbacks")
	}

	r.stats.RecordError()
	return nil, fmt.Errorf("all chat stream providers failed")
}

// Model returns the model name of the currently selected primary provider.
func (r *ChatRegistry) Model() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.selected == nil {
		return ""
	}
	return r.selected.Model()
}

// Name returns the name of the currently selected primary provider.
func (r *ChatRegistry) Name() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.selected == nil {
		return "chat-registry"
	}
	return r.selected.Name()
}

// Close shuts down all active providers in the registry.
func (r *ChatRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.selected != nil {
		_ = r.selected.Close()
	}
	for _, fb := range r.fallbacks {
		safe.Close(fb)
	}
	return nil
}

// ─── Stats ──────────────────────────────────────────────────────────────────

// Stats returns a snapshot of the chat usage statistics.
func (r *ChatRegistry) Stats() ChatStats {
	return r.stats.Snapshot()
}

// ─── Internal Helpers ───────────────────────────────────────────────────────

// autoDetectProviders returns all registered provider names sorted by priority.
// This is a simplified detection: any registered provider is considered available.
func (r *ChatRegistry) autoDetectProviders() []string {
	available := make([]string, 0, len(r.providers))

	for i := range r.providers {
		p := &r.providers[i]
		available = append(available, p.name)
	}

	log.Debug().Strs("available", available).Msg("auto-detected chat providers")
	return available
}

// createProvider attempts to instantiate a provider by name using its factory.
func (r *ChatRegistry) createProvider(ctx context.Context, name string) (ChatProvider, error) {
	for i := range r.providers {
		p := &r.providers[i]
		if p.name == name {
			return p.factory(ctx, nil)
		}
	}
	return nil, fmt.Errorf("unknown chat provider: %s", name)
}

// providerHealth returns the tracked health metrics for the given provider,
// creating a fresh tracker on first use.
func (r *ChatRegistry) providerHealth(name string) *providerHealth {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.providerHealthLocked(name)
}

// providerHealthLocked is providerHealth with the caller holding r.mu.
func (r *ChatRegistry) providerHealthLocked(name string) *providerHealth {
	if r.metrics == nil {
		r.metrics = make(map[string]*providerHealth)
	}
	h, ok := r.metrics[name]
	if !ok {
		h = newProviderHealth()
		r.metrics[name] = h
	}
	return h
}

// reorderForMode ranks the instantiated chain [primary, fallbacks...] using the
// weighted router and returns the new primary + fallbacks plus whether ranking
// succeeded. On any failure (parse/rank) the caller keeps the current order.
// The caller must hold r.mu, which is why providerHealthLocked is used.
func (r *ChatRegistry) reorderForMode(mode router.Mode, primary ChatProvider, fallbacks []ChatProvider) (ChatProvider, []ChatProvider, bool) {
	chain := append([]ChatProvider{primary}, fallbacks...)
	candidates := make([]router.Candidate, 0, len(chain))
	byName := make(map[string]ChatProvider, len(chain))
	for _, p := range chain {
		name := p.Name()
		byName[name] = p
		h := r.providerHealthLocked(name)
		candidates = append(candidates, router.Candidate{
			Name:      name,
			Health:    h.health(),
			Cost:      costScore(name, p.Model()),
			Latency:   h.latency(),
			TaskFit:   defaultTaskFit,
			Available: true,
			Offline:   isOfflineProvider(name),
		})
	}

	ranked, err := router.Rank(candidates, mode)
	if err != nil {
		log.Warn().Err(err).Str("mode", mode.String()).
			Msg("router ranking failed, keeping provider order")
		return nil, nil, false
	}

	newPrimary, ok := byName[ranked[0].Name]
	if !ok || newPrimary == nil {
		return nil, nil, false
	}
	newFallbacks := make([]ChatProvider, 0, len(ranked)-1)
	for _, c := range ranked[1:] {
		if p, ok := byName[c.Name]; ok {
			newFallbacks = append(newFallbacks, p)
		}
	}
	return newPrimary, newFallbacks, true
}

// providerNames returns the names of the providers in chain, preserving order.
func providerNames(chain []ChatProvider) []string {
	names := make([]string, len(chain))
	for i, p := range chain {
		names[i] = p.Name()
	}
	return names
}
