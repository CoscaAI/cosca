// Package orchestration defines the core types and consumer-side port interfaces
// for the Cosca AI Orchestration Engine. The engine coordinates the Knowledge,
// Memory, Agents, Skills, Chat, and Embeddings subsystems to fulfill user
// requests through a configurable pipeline.
//
// All interfaces defined here are ports — the orchestration engine depends on
// these abstractions. Implementations live in their respective packages and are
// wired by dependency injection at startup.
package orchestration

import (
	"context"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/embeddings"
)

// ─── Knowledge Searcher ──────────────────────────────────────────────────────

// KnowledgeSearchParams carries the parameters for a knowledge-base search.
// It is a simplified, orchestration-level projection of the richer
// search.SearchParams used internally by the knowledge engine.
type KnowledgeSearchParams struct {
	// Query is the search query string.
	Query string `json:"query"`

	// Limit is the maximum number of results to return.
	Limit int `json:"limit,omitempty"`

	// Types restricts results to specific entity/document types
	// (e.g. "document", "chunk", "entity", "code_block").
	Types []string `json:"types,omitempty"`

	// Path restricts results to a specific path prefix.
	Path string `json:"path,omitempty"`

	// MinScore filters results below this score threshold.
	MinScore float64 `json:"min_score,omitempty"`
}

// KnowledgeSearchResult is a lightweight projection of a single knowledge-base
// search hit, suitable for consumption by the orchestration engine.
type KnowledgeSearchResult struct {
	ID           string  `json:"id"`
	Title        string  `json:"title,omitempty"`
	Content      string  `json:"content,omitempty"`
	Snippet      string  `json:"snippet,omitempty"`
	Score        float64 `json:"score"`
	DocumentPath string  `json:"document_path,omitempty"`
	// PolicyState is optional for legacy providers; explicit quarantine/block is
	// excluded before the result can reach model context.
	PolicyState string `json:"policy_state,omitempty"`
}

// KnowledgeSearchResults bundles search hits with query metadata.
type KnowledgeSearchResults struct {
	Results    []KnowledgeSearchResult `json:"results"`
	TotalCount int                     `json:"total_count"`
	Query      string                  `json:"query"`
}

// KnowledgeSearcher searches the knowledge engine for information relevant
// to the current orchestration request.
type KnowledgeSearcher interface {
	Search(ctx context.Context, params KnowledgeSearchParams) (*KnowledgeSearchResults, error)
}

// ─── Memory Ports ────────────────────────────────────────────────────────────

// MemoryRecord is an orchestration-level projection of the full
// memory.MemoryRecord, carrying only the fields needed by the engine.
type MemoryRecord struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Layer    string            `json:"layer"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Priority int               `json:"priority"`
}

// MemorySearchOptions filters memory searches within the orchestration layer.
type MemorySearchOptions struct {
	Types    []string `json:"types,omitempty"`
	Layers   []string `json:"layers,omitempty"`
	Limit    int      `json:"limit"`
	MinScore float64  `json:"min_score,omitempty"`

	// ReaderAgent is the agent requesting the search. The memory adapter uses it
	// to scope results per the roles policy (A7): specialists see their own
	// memory; trusted roles (kernel/chiefs/executives/critic) see everything.
	ReaderAgent string `json:"reader_agent,omitempty"`
}

// MemoryRetriever searches and retrieves records from the memory engine.
// Matches the MemoryEngine.Search and MemoryEngine.Retrieve methods.
type MemoryRetriever interface {
	// Search searches across memory layers for records matching the query.
	Search(ctx context.Context, query string, opts MemorySearchOptions) ([]MemoryRecord, error)

	// Retrieve fetches a specific memory record by ID and layer.
	Retrieve(ctx context.Context, id, layer string) (*MemoryRecord, error)
}

// MemoryStorer persists results into the memory engine.
// Matches the MemoryEngine.Store method.
type MemoryStorer interface {
	// Store saves a memory record and returns the persisted record.
	Store(ctx context.Context, record MemoryRecord) (*MemoryRecord, error)
}

// ─── Agent Resolver ──────────────────────────────────────────────────────────

// AgentInfo is an orchestration-level projection of agents.Agent, exposing
// only the fields the orchestration engine requires.
type AgentInfo struct {
	Name            string   `json:"name"`
	Role            string   `json:"role"`
	Department      string   `json:"department"`
	Description     string   `json:"description"`
	Capabilities    []string `json:"capabilities,omitempty"`
	Responsibilities []string `json:"responsibilities,omitempty"`
}

// AgentResolver resolves agents from the agent registry.
// Matches the agents.Manager.Get and agents.Manager.Search methods.
type AgentResolver interface {
	// Get returns a specific agent by name (case-insensitive).
	Get(name string) (*AgentInfo, error)

	// Search finds agents whose name, role, department, or description
	// match the given query.
	Search(query string) ([]AgentInfo, error)
}

// ─── Skill Resolver ──────────────────────────────────────────────────────────

// SkillInfo is an orchestration-level projection of skills.Skill, exposing
// only the fields the orchestration engine requires.
type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// SkillResolver resolves skills from the skill registry.
// Matches the skills.Manager.Get, skills.Manager.Search, and
// skills.Manager.List methods.
type SkillResolver interface {
	// Get returns a specific skill by name (case-insensitive).
	Get(name string) (*SkillInfo, error)

	// Search finds skills whose name, description, or category match
	// the given query.
	Search(query string) ([]SkillInfo, error)

	// List returns all available skills.
	List() ([]SkillInfo, error)
}

// ─── Chat Provider ───────────────────────────────────────────────────────────

// ChatProvider is imported directly from internal/chat and re-exported as a
// port type for convenience. The chat.ChatProvider interface supports both
// synchronous and streaming completions, making it the central abstraction
// for LLM communication within the orchestration engine.
//
// See internal/chat.ChatProvider for the full method set:
//   - Chat(ctx, messages, opts) (*ChatResponse, error)
//   - ChatStream(ctx, messages, opts) (ChatStream, error)
//   - Model() string
//   - Name() string
//   - Close() error
type ChatProvider = chat.ChatProvider

// ─── Embedder ────────────────────────────────────────────────────────────────

// Embedder is a simplified port for generating vector embeddings from text.
// It matches the embeddings.Provider.GenerateEmbedding streaming method,
// exposing only the single-text embedding generation that the orchestration
// engine needs.
type Embedder interface {
	// GenerateEmbedding produces a vector embedding for a single text input.
	GenerateEmbedding(ctx context.Context, text string) (*embeddings.EmbeddingResult, error)

	// GenerateEmbeddings produces vector embeddings for a batch of text inputs.
	// Providers that support batching can process all texts in a single API call,
	// dramatically reducing latency and cost compared to N individual calls.
	GenerateEmbeddings(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error)
}
