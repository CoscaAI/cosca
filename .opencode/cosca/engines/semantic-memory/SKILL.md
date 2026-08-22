# SEMANTIC MEMORY ENGINE

> **Version**: 1.0.0 | **Status**: active | **Owner**: Semantic Memory Engine | **Last Updated**: 2026-07-28

## PURPOSE
The Semantic Memory Engine provides meaning-based search and retrieval across all Cosca memory. Unlike the hierarchical Memory Engine (path/name-based), this engine uses vector embeddings to find relevant knowledge by semantic similarity. This enables cross-agent knowledge transfer — Agent B can discover patterns learned by Agent A even without explicit keyword matching.

## ACTIVATION
- On Kernel startup (verify index freshness)
- When any agent requests semantic search
- When memory files change (auto-reindex)
- When new agent learnings are recorded

## ARCHITECTURE
```
Source Files (.opencode/cosca/memory/*.md)
    │
    ▼
[Parser] → Chunks (title + tags + content)
    │
    ▼
[Embedder] → Vector embeddings (768-dim)
    │
    ▼
[Vector Store] → FTS5 + vector index (SQLite)
    │
    ▼
[Search API] ← Query → [Embed query] → [Similarity search] → Ranked results
```

## OPERATIONS

### INDEX
Input: { path: ".opencode/cosca/memory/", recursive: true, force: false }
Action: Walk all .md files, parse frontmatter (tags, type, agent), chunk content, generate embeddings, store in vector index
Output: { indexed: 421, failed: 0, duration: "2.3s" }

### SEARCH
Input: { query: "API design patterns", topK: 5, minRelevance: 0.30, types: ["learning", "pattern", "decision"] }
Action: Embed query, cosine similarity search against vector index, filter by type/tags, rank by relevance
Output: { results: [{file, relevance, snippet, tags}, ...], count: 5 }

### REINDEX
Input: { files: ["learnings.md", "pattern.md"], incremental: true }
Action: Re-embed only changed files, update vector index entries
Output: { reindexed: 2, duration: "0.4s" }

### HEALTH
Input: {}
Action: Check index freshness (last reindex timestamp), verify file count matches (421 expected), validate index integrity
Output: { status: "healthy", files_indexed: 421, last_index: "2026-07-28T12:00:00Z", stale_files: 0 }

## SEARCH ALGORITHM
1. Embed query using configured embedding model (via Provider Chief)
2. Cosine similarity against all vectors in index
3. Filter by memory type (learning, pattern, decision, bug, architecture)
4. Apply confidence decay: older memories get 0.95^days multiplier
5. Return top-K above minimum relevance threshold
6. Include source file path, relevance score, content snippet, and tags

## RELEVANCE SCORING
Score = Similarity × Freshness × Authority
- Similarity: cosine(0-1) between query and memory vectors
- Freshness: 0.95^(days since last update)
- Authority: 1.0 for learning entries with Level 4+, 0.8 for Level 3, 0.6 for Level 2 or below

## INTEGRATION WITH GO RUNTIME
The engine delegates heavy lifting to the Go binary's existing infrastructure:
- `internal/embeddings/` — ProviderRegistry.GenerateEmbedding()
- `internal/search/` — Search engine with FTS5 + vector
- `internal/indexer/` — Document parsing, chunking, embedding pipeline
- `cosca memory index` — CLI command (if available) for batch indexing
- `cosca memory search --query "..."` — CLI command for search

## CACHE STRATEGY
- Query results cached for 300s (5 minutes)
- Index metadata cached for 60s
- Vector index stored in `.cosca/memory/vectors.db` (SQLite, same DB as FTS5)
- Embedding cache: LRU with 1000 entries

## DEPENDENCIES
| Dependency | Purpose |
|------------|---------|
| Memory Engine | Storage and retrieval of indexed data |
| Provider Chief | Embedding model selection |
| Context Chief | File change detection |
| Knowledge Engine | Cross-references with patterns and playbooks |

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-28 | Cosca Kernel (por ordem do Don) | Initial Semantic Memory Engine |

## RELATED
- [MEMORY_MODEL.md](../../MEMORY_MODEL.md)
- [Memory Engine](../memory/SKILL.md)
- [Knowledge Engine](../knowledge/SKILL.md)
- [Learning Engine](../learning/SKILL.md)
- [Semantic Memory Chief](../../departments/semantic-memory/SKILL.md)
