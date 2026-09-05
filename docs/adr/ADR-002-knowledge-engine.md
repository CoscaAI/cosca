# ADR-002: Knowledge Engine with Hybrid Search

> **Status**: Accepted | **Owner**: AI Chief | **Last Updated**: 2026-07-23

## Context

The Cosca platform requires intelligent knowledge management over Cosca Markdown files, source code, configuration files, and other project artifacts. Users need to:

- Search project documentation using natural language
- Find code snippets by semantic meaning, not just keywords
- Discover relationships between entities (functions, types, modules)
- Get relevant results even when exact terms are not in the query
- Work entirely offline without external API dependencies

The search must be fast (<20ms p50), work on large codebases (100K+ documents), and provide high-quality results.

## Decision

We adopt a **hybrid search architecture** combining three complementary strategies:

1. **SQLite FTS5** for exact keyword matching
2. **Vector embeddings** (sqlite-vec) for semantic similarity
3. **Knowledge Graph** for entity-relationship traversal

### Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                         HYBRID SEARCH ENGINE                           │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                       QUERY PROCESSOR                          │    │
│  │  Classifies query → Determines strategy → Routes to phases    │    │
│  └──────────┬─────────────────────────────────────────┬──────────┘    │
│             │                                         │               │
│     ┌───────▼───────┐                    ┌────────────▼──────────┐   │
│     │  Phase 1      │                    │  Phase 2              │   │
│     │  FTS5 Search  │                    │  Vector Search        │   │
│     │  (exact)      │                    │  (semantic)           │   │
│     └───────┬───────┘                    └────────────┬──────────┘   │
│             │                                         │               │
│     ┌───────▼───────┐                    ┌────────────▼──────────┐   │
│     │  Phase 3      │◄───────────────────│  Phase 4              │   │
│     │  Graph        │    Enrich results  │  Re-ranking           │   │
│     │  Traversal    │                    │  Dedup + Scoring      │   │
│     └───────┬───────┘                    └────────────┬──────────┘   │
│             │                                         │               │
│     ┌───────▼─────────────────────────────────────────▼──────────┐   │
│     │                     RESULTS                                 │   │
│     │  Ranked list + Facets + Suggestions + Explanations         │   │
│     └────────────────────────────────────────────────────────────┘   │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

### Storage Design

| Store | Technology | Purpose |
|-------|-----------|---------|
| Full-text index | SQLite FTS5 (Porter tokenizer) | Exact keyword matching with BM25 scoring |
| Vector store | sqlite-vec extension | 128-dimensional embeddings for semantic search |
| Knowledge graph | SQLite adjacency tables | Entity nodes, typed relationships, graph traversal |
| Document store | SQLite relational tables | Document metadata, chunks, code blocks |
| Cache | In-memory LRU + SQLite | Multi-level caching of frequent queries |

### Indexing Pipeline

```
Raw file
  → Parser (Markdown, Code, YAML/JSON frontmatter)
  → Chunker (heading-based, paragraph-based, code-block, size-based)
  → Indexer:
      ├── Store document/chunk metadata in SQLite
      ├── Write to FTS5 virtual tables (documents_fts, chunks_fts, entities_fts)
      ├── Generate embeddings via configured provider
      ├── Store vectors in sqlite-vec
      └── Build knowledge graph entities and relationships
  → Cache
```

## Rationale

### Why Hybrid over Pure Vector Search?

| Strategy | Strengths | Weaknesses |
|----------|-----------|------------|
| Pure FTS5 | Exact match, fast, simple | Misses synonyms, typos, semantic meaning |
| Pure Vector | Semantic understanding, language-agnostic | Misses exact identifiers, slow on large sets |
| Elasticsearch | Powerful, feature-rich | Heavy dependency, requires Java, external service |
| **Hybrid (chosen)** | **Best of FTS5 + Vector + Graph** | **More complex indexing** |

### Why SQLite over Elasticsearch?

1. **Zero external dependencies** — No server to install, configure, or maintain
2. **Local-first** — All data lives on the user's machine, works offline
3. **Performance** — FTS5 is extremely fast for full-text search on moderate datasets
4. **Portability** — Single database file can be copied, backed up, or transferred
5. **Embedded** — Runs in-process with no network latency
6. **sqlite-vec extension** — Provides vector search capabilities without a separate vector database

### Why FTS5 over alternative SQLite FTS?

| FTS Version | Features | Verdict |
|-------------|----------|---------|
| FTS3 | Basic full-text search, no ranking | Too limited |
| FTS4 | Column-level search, ranking | Viable but deprecated |
| **FTS5 (chosen)** | **BM25 ranking, Porter tokenizer, prefix queries, snippet generation** | **Best option** |

## Alternatives Considered

### Pure Vector Search (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Excellent semantic understanding, works across languages |
| Cons | Misses exact identifier matches, higher latency, requires embedding model |
| Verdict | Rejected — exact match is critical for code search |

### Elasticsearch (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Battle-tested, rich query DSL, distributed, scalable |
| Cons | Heavy dependency (Java), external service, no offline mode, operational overhead |
| Verdict | Rejected — violates local-first and self-contained requirements |

### FTS5 Only (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Simple, fast, well-understood |
| Cons | No semantic understanding, misses synonyms and related concepts |
| Verdict | Rejected — modern AI-assisted workflows require semantic search |

## Consequences

### Positive

- **Local first, always available** — No network connection required for search
- **Fast hybrid search** — <20ms p50, <100ms p95 for most queries
- **High relevance** — Combined FTS5 + Vector + Graph yields best-in-class search quality
- **No external services** — Everything runs locally in SQLite
- **Cache layer** — Repeated queries served from memory in microseconds

### Negative

- **Indexing complexity** — Three parallel indexing pipelines increase code complexity
- **Storage footprint** — Vectors and FTS indexes increase database size (~2x raw content)
- **Embedding model dependency** — Quality depends on embedding model; default is lightweight local model

### Neutral

- **sqlite-vec is an extension** — Requires CGo or pre-compiled SQLite; managed via Go build tags
- **Embedding providers are pluggable** — Users can configure local or remote embedding models

---

**Related**: [Knowledge Engine Overview](../knowledge/overview.md) | [Search Guide](../knowledge/search.md) | [Knowledge Graph](../knowledge/graph.md) | [ADR-001](ADR-001-cosca-cli-architecture.md)
