# ADR-0001: AI Architecture & Capability Framework

> **Status**: Accepted | **Date**: 2026-07-28 | **Author**: cosca-ai (AI Chief)
>
> Supersedes: None | Superseded by: None

## Context

CoscaAI is a multi-agent enterprise platform with 53+ AI agents. The platform needs a robust AI subsystem to power: semantic search, agent routing, knowledge retrieval, embeddings generation, and future RAG pipelines. A comprehensive audit was conducted as part of the Onda 5 activation of the cosca-ai agent.

## Decision

The AI subsystem architecture follows a **layered pipeline pattern** with 6 integrated layers:

```
Document Ingestion → Embeddings Generation → Vector Storage → Hybrid Search → Re-Ranking → Results
                         ↓
                   Knowledge Graph (parallel index with cross-references)
```

### Layer 1: Embeddings Framework (`internal/embeddings/`)
- **Provider interface** with single + batch embedding support
- **Provider Registry** with auto-detection, fallback chains, and SHA256-based caching
- **Provider implementations** via OpenAIC Compat API pattern (Mistral, Groq, DeepSeek, Anthropic registered)
- **Decision**: Use OpenAIC Compat API pattern as the universal embedding adapter (allows swapping any provider)
- **Provider Priority Order**: Configured primary → auto-detected → explicit fallbacks → local (always last)

### Layer 2: Vector Store (`internal/vector/`)
- **Store interface** with CRUD + Search + Filter + Document/Entity deletion + Rebuild
- **Default Implementation**: SQLiteVec — BLOB-based brute-force cosine similarity
- **Decision**: Start with SQLite brute-force (good to ~100K vectors), plan HNSW/FAISS migration for scale
- **Vector encoding**: Little-endian float64 binary (8 bytes per dimension)

### Layer 3: Knowledge Graph (`internal/graph/`)
- **In-memory directed graph** with 22 entity types and 16 relationship types
- **Traversal**: BFS, DFS, ShortestPath, Dijkstra (weighted)
- **Builder**: From entities, documents, chunks, cross-references, dependencies, Go imports
- **Persistence**: JSON serialization to SQLite cache table (graph_state key)
- **Decision**: In-memory for speed, SQLite for persistence, JSON for export/import

### Layer 4: Hybrid Search (`internal/search/`)
- **3-phase pipeline**: FTS5 full-text → Vector similarity → Graph traversal
- **Source tracking**: Each result tagged with origin (fts/vector/graph)
- **Deduplication**: Across sources via ID-based `seen` map
- **Decision**: Hybrid search with equal weight on FTS + Vector, graph as optional boost

### Layer 5: Re-Ranking (`internal/ranking/`)
- **Multi-factor scoring**: BM25 (0.25) + Vector (0.35) + Graph (0.20) + Freshness (0.10) + Popularity (0.10)
- **Normalization**: Min-max per factor before weighted combination
- **Fusion**: Reciprocal Rank Fusion (RRF) for combining ranked lists
- **Decision**: Default weights favor vector + graph for semantic relevance

### Layer 6: Knowledge Engine (`internal/knowledge/`)
- **Orchestrator** that wires all layers together
- **Caching**: 3-tier (memory → SQLite → filesystem) with 5-min TTL for search results
- **Lifecycle**: Init → Index → Search → Verify → Snapshot → Sync → Close
- **Decision**: Monolithic engine within the Go binary (no external service dependencies for core AI)

### Cross-Cutting: Semantic Router (`internal/orchestration/semantic_router.go`)
- **Agent selection via embedding similarity** between user prompt and agent descriptions
- **Cascade**: Semantic(≥0.80) → Semantic(≥0.60) → Keyword → Full-text → CEO
- **Batch embedding** for agent descriptions with fallback to individual calls (semaphore at 5)
- **Decision**: Embedding-based routing is language-agnostic (same model for all languages)

## Consequences

### Positive
- **Self-contained**: All AI runs within the Go binary (no external vector DB, no embedding service required)
- **Pluggable**: Provider registry allows swapping embedding providers without code changes
- **Graceful degradation**: Falls back from primary → fallback → local at every layer
- **Cache everywhere**: Embedding cache, search cache, graph state cache — reduces API costs
- **Observable**: Stats at every layer (embedding stats, vector count, graph stats, cache hit rate)

### Negative
- **Brute-force ceiling**: SQLiteVec degrades linearly beyond ~100K vectors — need ANN migration plan
- **Provider gaps**: Only 4/9 chat providers register embeddings (missing: OpenAI native, Google, Azure native, Ollama, Bedrock)
- **No RAG pipeline**: Pieces exist but no explicit retrieval-augmented generation flow
- **No AI safety**: No prompt injection detection, no output validation, no content filtering
- **No model management**: No model versioning, no A/B testing, no deployment tracking
- **Go-only imports**: Graph code import extraction only handles Go syntax

### Risks
- **Embedding quality variance**: Different providers produce different quality embeddings at different dimensions
- **Cache poisoning**: SHA256 cache keys are content-based — malicious content could pollute cache
- **Silent failures**: When embedding provider fails, the system continues without embeddings (no alert)

## See Also
- [system-architecture-overview.md](../system-architecture-overview.md)
- [database-architecture.md](../database-architecture.md)
- [ADR-2958 Provider Interface](adr-2958-provider.md)
- [ADR-3159 Caching Strategy](adr-3159-cache.md)
