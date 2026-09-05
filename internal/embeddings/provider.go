// Package embeddings provides the embedding provider registry for the Cosca Knowledge Engine.
// It manages provider registration, selection, fallback chains, and embedding caching.
package embeddings

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog/log"
)

// Config holds common embedding provider configuration.
// Providers interpret only the fields relevant to them.
type Config struct {
	APIKey          string
	Model           string
	Dimensions      int
	BaseURL         string
	Endpoint        string
	Deployment      string
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	KeepAlive       string
	Digest          string // digest esperado do modelo (pinagem — L376)
}

// ProviderFactory is a function that creates a Provider instance.
type ProviderFactory func(ctx context.Context, cfg *Config) (Provider, error)

// providerHolder wraps a Provider for atomic storage.
// atomic.Pointer cannot hold interface values directly, so we wrap it.
type providerHolder struct {
	provider Provider
}

// registeredProvider holds a provider factory and its metadata.
type registeredProvider struct {
	name        string
	factory     ProviderFactory
	description string
	priority    int // lower = higher priority
	mu          sync.Mutex
	instance    atomic.Pointer[providerHolder] // cached instance holder
}

// ProviderRegistry manages available embedding providers and their selection.
type ProviderRegistry struct {
	mu         sync.RWMutex
	providers  []registeredProvider
	selected   Provider
	fallbacks  []Provider
	cache      *embeddingCache
	stats      *EmbeddingStats
	autoDetect bool

	// Registry-level overrides carried from ProviderRegistryConfig so
	// createProvider can pass them into provider factories. They are set under
	// mu in Select and read under the same lock by createProvider.
	baseURLOverride string
	modelOverride   string
	apiKeyOverride  string
	dimsOverride    int
	digestOverride  string
}

// ErrEmbeddingIdentityMismatch marca falhas de IDENTIDADE do modelo de
// embeddings (L376): o digest esperado não bate com o carregado. Diferente
// de indisponibilidade, identidade é FALHA DE INTEGRIDADE — deve parar,
// nunca cair em fallback.
var ErrEmbeddingIdentityMismatch = errors.New("embedding identity mismatch")

// ProviderRegistryConfig defines configuration for the registry.
type ProviderRegistryConfig struct {
	// Primary is the name of the primary provider to use.
	Primary string

	// Fallbacks is an ordered list of fallback provider names.
	Fallbacks []string

	// AutoDetect enables automatic detection of available providers.
	AutoDetect bool

	// CacheEnabled enables embedding result caching.
	CacheEnabled bool

	// Digest é o digest esperado do modelo de embeddings (L376): quando
	// definido, o provider precisa provar a identidade (mesmo digest) antes
	// de ser aceito — fail-closed contra mudança silenciosa do modelo.
	Digest string

	// CacheTTL is the duration to cache embedding results.
	CacheTTL time.Duration

	// CacheMaxSize is the maximum number of cached embeddings.
	CacheMaxSize int

	// BaseURL optionally overrides the selected provider's base URL (e.g. a
	// local OpenAI-compatible embeddings server). Empty means the provider
	// default endpoint.
	BaseURL string

	// Model optionally overrides the selected provider's model name. Empty
	// means the provider default.
	Model string

	// APIKey optionally overrides the selected provider's API key. Empty means
	// the provider default (usually from environment).
	APIKey string

	// Dimensions optionally overrides the selected provider's output
	// dimensions. 0 means the provider default.
	Dimensions int
}

// DefaultProviderRegistryConfig returns sensible defaults.
func DefaultProviderRegistryConfig() ProviderRegistryConfig {
	return ProviderRegistryConfig{
		AutoDetect:   true,
		CacheEnabled: true,
		CacheTTL:     24 * time.Hour,
		CacheMaxSize: 10000,
	}
}

// Global registry singleton.
var (
	globalRegistry   *ProviderRegistry
	globalRegistryMu sync.RWMutex
)

// GetRegistry returns the global ProviderRegistry singleton.
func GetRegistry() *ProviderRegistry {
	globalRegistryMu.RLock()
	if globalRegistry != nil {
		globalRegistryMu.RUnlock()
		return globalRegistry
	}
	globalRegistryMu.RUnlock()

	globalRegistryMu.Lock()
	defer globalRegistryMu.Unlock()
	if globalRegistry == nil {
		globalRegistry = &ProviderRegistry{
			cache: newEmbeddingCache(DefaultProviderRegistryConfig()),
			stats: NewEmbeddingStats(),
		}
	}
	return globalRegistry
}

// ResetRegistry recreates the global registry for testing.
func ResetRegistry() {
	globalRegistryMu.Lock()
	globalRegistry = &ProviderRegistry{
		cache: newEmbeddingCache(DefaultProviderRegistryConfig()),
		stats: NewEmbeddingStats(),
	}
	globalRegistryMu.Unlock()
}

// Register adds a provider factory to the registry.
func (r *ProviderRegistry) Register(name string, factory ProviderFactory, description string, priority int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Replace existing entry if present
	for i := range r.providers {
		p := &r.providers[i]
		if p.name == name {
			r.providers[i] = registeredProvider{
				name: name, factory: factory, description: description, priority: priority,
			}
			log.Debug().Str("provider", name).Int("priority", priority).Msg("embedding provider re-registered")
			return
		}
	}

	r.providers = append(r.providers, registeredProvider{
		name: name, factory: factory, description: description, priority: priority,
	})

	sort.Slice(r.providers, func(i, j int) bool {
		return r.providers[i].priority < r.providers[j].priority
	})

	log.Debug().Str("provider", name).Int("priority", priority).Msg("embedding provider registered")
}

// Get returns the provider with the given name.
func (r *ProviderRegistry) Get(name string) (Provider, bool) {
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
func (p *registeredProvider) getOrCreate() Provider {
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
		log.Warn().Err(err).Str("provider", p.name).Msg("failed to create provider instance")
		return nil
	}
	p.instance.Store(&providerHolder{provider: provider})
	return provider
}

// List returns all registered provider names.
func (r *ProviderRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, len(r.providers))
	for i := range r.providers {
		p := &r.providers[i]
		names[i] = p.name
	}
	return names
}

// Select initializes the provider chain based on configuration.
// It creates the primary provider and fallbacks, returning an error if none
// can be initialized.
func (r *ProviderRegistry) Select(ctx context.Context, cfg ProviderRegistryConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.autoDetect = cfg.AutoDetect
	r.cache = newEmbeddingCache(cfg)
	r.baseURLOverride = cfg.BaseURL
	r.modelOverride = cfg.Model
	r.apiKeyOverride = cfg.APIKey
	r.dimsOverride = cfg.Dimensions
	r.digestOverride = cfg.Digest

	// Build selection order
	var candidates []string

	if cfg.Primary != "" {
		candidates = append(candidates, cfg.Primary)
	}
	if cfg.AutoDetect {
		// Add auto-detected providers in priority order
		autoCandidates := r.autoDetectProviders()
		for _, name := range autoCandidates {
			if name != cfg.Primary {
				candidates = append(candidates, name)
			}
		}
	}
	candidates = append(candidates, cfg.Fallbacks...)

	// Fail-closed (L376): o "local" só é fallback quando NÃO há provider
	// explícito. Com um Primary configurado, a falha do provider é um ERRO —
	// nunca um fallback silencioso para o local (identidade incompatível).
	if cfg.Primary == "" {
		candidates = append(candidates, "local")
	}

	// Try each candidate until one succeeds
	var primary Provider
	var fallbacks []Provider

	for _, name := range candidates {
		provider, err := r.createProvider(ctx, name)
		if err != nil {
			// Fail-closed (L376): erro de IDENTIDADE (digest do modelo) é
			// fatal — NUNCA pula para o próximo candidato/fallback.
			if errors.Is(err, ErrEmbeddingIdentityMismatch) {
				return fmt.Errorf("embedding identity mismatch: %w", err)
			}
			log.Debug().Err(err).Str("provider", name).Msg("provider unavailable, skipping")
			continue
		}

		if primary == nil {
			primary = provider
			log.Info().Str("provider", name).Str("model", provider.Model()).Int("dims", provider.Dimensions()).Msg("primary embedding provider selected")
		} else {
			fallbacks = append(fallbacks, provider)
			log.Debug().Str("provider", name).Msg("fallback embedding provider registered")
		}
	}

	if primary == nil {
		return fmt.Errorf("no embedding provider could be initialized")
	}

	r.selected = primary
	r.fallbacks = fallbacks
	return nil
}

// GenerateEmbedding generates an embedding with caching and fallback support.
func (r *ProviderRegistry) GenerateEmbedding(ctx context.Context, text string) (*EmbeddingResult, error) {
	// Check cache under read lock to avoid race with Select()
	r.mu.RLock()
	cache := r.cache
	r.mu.RUnlock()

	if cache != nil {
		if cached, ok := cache.Get(text); ok {
			r.stats.RecordRequest("cache", cached.TokensUsed, cached.Dimensions)
			return cached, nil
		}
	}

	r.mu.RLock()
	primary := r.selected
	fallbacks := r.fallbacks
	r.mu.RUnlock()

	if primary == nil {
		return nil, fmt.Errorf("no embedding provider selected")
	}

	// Try primary
	result, err := primary.GenerateEmbedding(ctx, text)
	if err == nil {
		if validationErr := ValidateEmbedding(result); validationErr != nil {
			err = fmt.Errorf("invalid embedding from %s: %w", primary.Name(), validationErr)
		} else {
			r.stats.RecordRequest(primary.Name(), result.TokensUsed, result.Dimensions)
			if cache != nil {
				cache.Set(text, result)
			}
			return result, nil
		}
	}

	log.Warn().Err(err).Str("provider", primary.Name()).Msg("primary embedding provider failed, trying fallbacks")

	// Try fallbacks
	for _, fb := range fallbacks {
		result, err := fb.GenerateEmbedding(ctx, text)
		if err == nil {
			if validationErr := ValidateEmbedding(result); validationErr != nil {
				err = fmt.Errorf("invalid embedding from %s: %w", fb.Name(), validationErr)
			} else {
				r.stats.RecordRequest(fb.Name(), result.TokensUsed, result.Dimensions)
				if cache != nil {
					cache.Set(text, result)
				}
				return result, nil
			}
		}
		log.Warn().Err(err).Str("provider", fb.Name()).Msg("fallback embedding provider failed")
	}

	r.stats.RecordError("all")
	return nil, fmt.Errorf("all embedding providers failed")
}

// GenerateEmbeddings generates embeddings for a batch of texts with fallback support.
func (r *ProviderRegistry) GenerateEmbeddings(ctx context.Context, texts []string) ([]*EmbeddingResult, error) {
	r.mu.RLock()
	primary := r.selected
	fallbacks := r.fallbacks
	cache := r.cache
	r.mu.RUnlock()

	if primary == nil {
		return nil, fmt.Errorf("no embedding provider selected")
	}

	// Try primary
	results, err := primary.GenerateEmbeddings(ctx, texts)
	if err == nil {
		if err = validateEmbeddingBatch(results, len(texts)); err != nil {
			log.Warn().Err(err).Str("provider", primary.Name()).Msg("primary batch returned invalid embeddings")
		} else {
			for i, res := range results {
				if res != nil && cache != nil {
					cache.Set(texts[i], res)
				}
				if res != nil {
					r.stats.RecordRequest(primary.Name(), res.TokensUsed, res.Dimensions)
				}
			}
			return results, nil
		}
	}

	log.Warn().Err(err).Str("provider", primary.Name()).Msg("primary batch embedding failed, trying fallbacks")

	// Try fallbacks
	for _, fb := range fallbacks {
		results, err := fb.GenerateEmbeddings(ctx, texts)
		if err == nil {
			if validationErr := validateEmbeddingBatch(results, len(texts)); validationErr != nil {
				err = validationErr
			} else {
				for i, res := range results {
					if res != nil && cache != nil {
						cache.Set(texts[i], res)
					}
					if res != nil {
						r.stats.RecordRequest(fb.Name(), res.TokensUsed, res.Dimensions)
					}
				}
				return results, nil
			}
		}
		log.Warn().Err(err).Str("provider", fb.Name()).Msg("fallback batch embedding failed")
	}

	r.stats.RecordError("all")
	return nil, fmt.Errorf("all embedding providers failed for batch")
}

func validateEmbeddingBatch(results []*EmbeddingResult, expected int) error {
	if len(results) != expected {
		return fmt.Errorf("embedding result count %d does not match input count %d", len(results), expected)
	}
	for i, result := range results {
		if err := ValidateEmbedding(result); err != nil {
			return fmt.Errorf("embedding %d: %w", i, err)
		}
	}
	return nil
}

// Stats returns the embedding statistics.
func (r *ProviderRegistry) Stats() *EmbeddingStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats.Snapshot()
}

// Dimensions returns the dimensionality of the selected provider's
// embeddings. It returns 0 when no provider is selected, so callers can
// apply their own fallback default.
func (r *ProviderRegistry) Dimensions() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.selected == nil {
		return 0
	}
	return r.selected.Dimensions()
}

// CacheStats returns cache statistics.
func (r *ProviderRegistry) CacheStats() map[string]int {
	if r.cache == nil {
		return map[string]int{"size": 0}
	}
	return r.cache.Stats()
}

// Close closes all providers in the registry.
func (r *ProviderRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.selected != nil {
		_ = r.selected.Close()
	}
	for _, fb := range r.fallbacks {
		safe.Close(fb)
	}
	r.cache = nil
	return nil
}

// autoDetectProviders checks which registered providers are available via environment
// variables or local endpoints.
func (r *ProviderRegistry) autoDetectProviders() []string {
	available := make([]string, 0, len(r.providers))

	for i := range r.providers {
		p := &r.providers[i]
		available = append(available, p.name)
	}

	log.Debug().Strs("available", available).Msg("auto-detected embedding providers")
	return available
}

// createProvider attempts to instantiate a provider by name. When the registry
// carries override configuration (base URL, model, API key or dimensions) a
// non-nil *Config is passed to the factory; otherwise nil is passed, preserving
// the historical behavior where providers fall back to their own environment /
// default configuration.
func (r *ProviderRegistry) createProvider(ctx context.Context, name string) (Provider, error) {
	for i := range r.providers {
		p := &r.providers[i]
		if p.name == name {
			var cfg *Config
			if r.baseURLOverride != "" || r.modelOverride != "" || r.apiKeyOverride != "" || r.dimsOverride > 0 || r.digestOverride != "" {
				cfg = &Config{
					BaseURL:    r.baseURLOverride,
					Model:      r.modelOverride,
					APIKey:     r.apiKeyOverride,
					Dimensions: r.dimsOverride,
					Digest:     r.digestOverride,
				}
			}
			return p.factory(ctx, cfg)
		}
	}
	return nil, fmt.Errorf("unknown provider: %s", name)
}

// ─── Embedding Cache ───────────────────────────────────────────────────────

type cacheEntry struct {
	result    *EmbeddingResult
	expiresAt time.Time
	// seq é um contador monotônico de ordem de inserção. Serve como
	// desempate determinístico na evicção: quando duas entradas empatam em
	// expiresAt (possível com relógio de resolução grossa, ex. Windows), a de
	// menor seq (mais antiga) é evictada. Nunca depender da ordem aleatória
	// de iteração do map para desempatar.
	seq uint64
}

type embeddingCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	maxSize int
	ttl     time.Duration
	enabled bool
	// nextSeq é a próxima ordem de inserção a atribuir (monotônica).
	nextSeq uint64
}

func newEmbeddingCache(cfg ProviderRegistryConfig) *embeddingCache {
	return &embeddingCache{
		entries: make(map[string]*cacheEntry),
		maxSize: cfg.CacheMaxSize,
		ttl:     cfg.CacheTTL,
		enabled: cfg.CacheEnabled,
	}
}

func (c *embeddingCache) key(text string) string {
	h := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", h[:16])
}

func (c *embeddingCache) Get(text string) (*EmbeddingResult, bool) {
	if !c.enabled {
		return nil, false
	}

	c.mu.RLock()
	entry, ok := c.entries[c.key(text)]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.entries, c.key(text))
		c.mu.Unlock()
		return nil, false
	}

	return entry.result, true
}

func (c *embeddingCache) Set(text string, result *EmbeddingResult) {
	if !c.enabled || result == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity
	if len(c.entries) >= c.maxSize {
		// Eviction: oldest by expiresAt; tie-break by insertion order (seq).
		// O relógio de resolução grossa (ex. Windows) pode gerar expiresAt
		// idênticos para entradas inseridas em sequência rápida; sem o seq o
		// desempate cairia na ordem aleatória de iteração do map, tornando a
		// evicção não-determinística.
		var oldestKey string
		var oldestEntry *cacheEntry
		for k, v := range c.entries {
			if oldestEntry == nil ||
				v.expiresAt.Before(oldestEntry.expiresAt) ||
				(v.expiresAt.Equal(oldestEntry.expiresAt) && v.seq < oldestEntry.seq) {
				oldestKey = k
				oldestEntry = v
			}
		}
		delete(c.entries, oldestKey)
	}

	seq := c.nextSeq
	c.nextSeq++
	c.entries[c.key(text)] = &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(c.ttl),
		seq:       seq,
	}
}

func (c *embeddingCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
}

func (c *embeddingCache) Stats() map[string]int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]int{
		"size":     len(c.entries),
		"max_size": c.maxSize,
	}
}
