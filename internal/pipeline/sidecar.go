package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ── Dapr Sidecar Pattern — AgentRuntime Building Blocks ─────────────────
//
// Agents call the sidecar via standardized APIs.
// Backends (SQLite, Redis, HTTP) are pluggable — agents never know
// about infrastructure details.
//
// Dapr Pattern #1: Sidecar Architecture
//   Building blocks: State, Pub/Sub, Secrets, LLM
//   Pluggable components via YAML
//   Declarative resiliency: retries, timeouts, circuit breakers

// ── State Store ──────────────────────────────────────────────────────────

// StateStore is the interface for key-value state storage.
// Implementations: SQLite (default), Redis, in-memory.
type StateStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// FileStateStore stores state as JSON files on disk (zero dependencies).
type FileStateStore struct {
	dir string
	mu  sync.RWMutex
}

func NewFileStateStore(dir string) (*FileStateStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("state store: %w", err)
	}
	return &FileStateStore{dir: dir}, nil
}

func (s *FileStateStore) path(key string) string {
	return filepath.Join(s.dir, key+".json")
}

func (s *FileStateStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return os.ReadFile(s.path(key))
}

func (s *FileStateStore) Set(_ context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.WriteFile(s.path(key), value, 0o644)
}

func (s *FileStateStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.Remove(s.path(key))
}

func (s *FileStateStore) List(_ context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			name := e.Name()[:len(e.Name())-5] // strip .json
			if prefix == "" || len(name) >= len(prefix) && name[:len(prefix)] == prefix {
				keys = append(keys, name)
			}
		}
	}
	return keys, nil
}

// ── Pub/Sub ──────────────────────────────────────────────────────────────

// PubSub is the interface for publish-subscribe messaging.
type PubSub interface {
	Publish(ctx context.Context, topic string, data []byte) error
	Subscribe(ctx context.Context, topic string) (<-chan []byte, error)
}

// ChannelPubSub is an in-memory pub/sub (single process).
type ChannelPubSub struct {
	mu    sync.RWMutex
	subs  map[string][]chan []byte
}

func NewChannelPubSub() *ChannelPubSub {
	return &ChannelPubSub{subs: make(map[string][]chan []byte)}
}

func (ps *ChannelPubSub) Publish(_ context.Context, topic string, data []byte) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	for _, ch := range ps.subs[topic] {
		select {
		case ch <- data:
		default:
			// drop if subscriber is slow
		}
	}
	return nil
}

func (ps *ChannelPubSub) Subscribe(_ context.Context, topic string) (<-chan []byte, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ch := make(chan []byte, 10)
	ps.subs[topic] = append(ps.subs[topic], ch)
	return ch, nil
}

// ── AgentRuntime (Sidecar) ────────────────────────────────────────────────

// AgentRuntime is the sidecar that agents interact with.
// It exposes building blocks via standardized interfaces.
type AgentRuntime struct {
	State  StateStore
	PubSub PubSub
	Runner Runner // LLM provider

	mu     sync.RWMutex
	config map[string]interface{}
}

// NewAgentRuntime creates a runtime with the given building blocks.
func NewAgentRuntime(state StateStore, pubsub PubSub, runner Runner) *AgentRuntime {
	return &AgentRuntime{
		State:  state,
		PubSub: pubsub,
		Runner: runner,
		config: make(map[string]interface{}),
	}
}

// SetConfig stores runtime configuration (declarative, from YAML).
func (rt *AgentRuntime) SetConfig(key string, value interface{}) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.config[key] = value
}

// GetConfig retrieves runtime configuration.
func (rt *AgentRuntime) GetConfig(key string) (interface{}, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	v, ok := rt.config[key]
	return v, ok
}

// ── State helpers (typed) ────────────────────────────────────────────────

// GetJSON unmarshals a state value as JSON into the provided target.
func GetJSON[T any](store StateStore, ctx context.Context, key string) (T, error) {
	var zero T
	data, err := store.Get(ctx, key)
	if err != nil {
		return zero, err
	}
	var val T
	if err := json.Unmarshal(data, &val); err != nil {
		return zero, fmt.Errorf("state: unmarshal %q: %w", key, err)
	}
	return val, nil
}

// SetJSON marshals a value as JSON and stores it.
func SetJSON[T any](store StateStore, ctx context.Context, key string, val T) error {
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("state: marshal %q: %w", key, err)
	}
	return store.Set(ctx, key, data)
}

// ── Resiliency Config ────────────────────────────────────────────────────

// ResiliencyConfig defines retry, timeout, and circuit breaker settings.
type ResiliencyConfig struct {
	MaxRetries    int    `json:"max_retries"`
	RetryDelayMs  int    `json:"retry_delay_ms"`
	TimeoutMs     int    `json:"timeout_ms"`
	CircuitBreaker bool  `json:"circuit_breaker"`
	MaxFailures   int    `json:"max_failures"`
}

// DefaultResiliency returns a sensible default configuration.
func DefaultResiliency() ResiliencyConfig {
	return ResiliencyConfig{
		MaxRetries:     3,
		RetryDelayMs:   2000,
		TimeoutMs:      30000,
		CircuitBreaker: true,
		MaxFailures:    5,
	}
}
