# Knowledge Engine Overview

> **Status**: active | **Owner**: AI Chief | **Last Updated**: 2026-07-23

## Architecture

The Knowledge Engine is the central intelligence system of the Cosca platform. It combines full-text search (FTS5), vector similarity search, and a knowledge graph into a unified pipeline with re-ranking, faceting, and caching.

```
┌──────────────────────────────────────────────────────────────────────┐
│                       KNOWLEDGE ENGINE                                │
│                                                                       │
│  ┌─────────────┐    ┌─────────────┐    ┌────────────────────────┐    │
│  │  INDEXING   │───▶│   SEARCH    │───▶│    CACHING             │    │
│  │  PIPELINE   │    │   PIPELINE  │    │  (Memory + SQLite)     │    │
│  └─────────────┘    └─────────────┘    └────────────────────────┘    │
│         │                  │                                           │
│         ▼                  ▼                                           │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                    CORE SUBSYSTEMS                            │    │
│  │                                                               │    │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────┐ ┌──────────┐ │    │
│  │  │  FTS5   │ │ Vector  │ │ Graph   │ │Ranker│ │Explainer │ │    │
│  │  │ Search  │ │ Search  │ │ Search  │ │      │ │          │ │    │
│  │  └─────────┘ └─────────┘ └─────────┘ └──────┘ └──────────┘ │    │
│  │                                                               │    │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────┐ ┌──────────┐ │    │
│  │  │ Indexer │ │ Chunker │ │  Parser │ │Entity│ │Embeddings│ │    │
│  │  │         │ │         │ │(MD+Code)│ │Extr. │ │ Provider │ │    │
│  │  └─────────┘ └─────────┘ └─────────┘ └──────┘ └──────────┘ │    │
│  └──────────────────────────────────────────────────────────────┘    │
│         │                                                           │
│         ▼                                                           │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                    STORAGE LAYER                              │    │
│  │                                                               │    │
│  │  ┌────────────────────┐  ┌────────────────────────────────┐  │    │
│  │  │  SQLite Database   │  │  Filesystem (documents, conf)  │  │    │
│  │  │  • FTS5 virtual    │  │  • Markdown files             │  │    │
│  │  │  • sqlite-vec vec  │  │  • Source code files          │  │    │
│  │  │  • Metadata tables │  │  • Configuration files        │  │    │
│  │  └────────────────────┘  └────────────────────────────────┘  │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Indexing Pipeline

The indexing pipeline converts raw files into searchable knowledge:

```
Raw File
   │
   ▼
┌──────────────┐
│   Parser     │  → Markdown parser (headings, code blocks, frontmatter)
│              │  → Entity parser (extract entities and relationships)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   Chunker    │  → Split into semantic chunks (by heading, paragraph)
│              │  → Preserve metadata (source, heading, section type)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   Indexer    │  → Write to SQLite (documents, chunks tables)
│              │  → Write to FTS5 (documents_fts, chunks_fts)
│              │  → Generate embeddings via provider
│              │  → Store vectors in sqlite-vec
│              │  → Build knowledge graph entities + relationships
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   Cache      │  → Store in multi-level cache (memory + SQLite)
└──────────────┘
```

### Supported File Types

| Type | Parser | Example |
|------|--------|---------|
| Markdown | `markdown.Parser` | `docs/**/*.md`, `README.md` |
| Code | `parser.CodeParser` | `.go`, `.ts`, `.py`, `.js`, etc. |
| YAML/JSON | `parser.FrontmatterParser` | Config files with frontmatter |

### Chunking Strategy

| Strategy | Description | Use Case |
|----------|-------------|----------|
| Heading-based | Split on markdown headings | Documentation files |
| Paragraph-based | Split on paragraph breaks | Prose content |
| Code block | Each code block is a chunk | Source files |
| Size-based | Max chunk size with overlap | Large files |

---

## Search Pipeline

The search pipeline combines three search strategies:

```
Query
  │
  ▼
┌──────────────────────────────────────────────────┐
│              HYBRID SEARCH ENGINE                  │
│                                                    │
│  Phase 1: FTS5 Full-Text                          │
│  ┌─────────────────────────────────────────────┐  │
│  │ • SQLite FTS5 with Porter tokenizer          │  │
│  │ • Table-specific search (docs, chunks, etc.) │  │
│  │ • BM25 scoring with snippet generation       │  │
│  │ • Type-based table resolution                │  │
│  └─────────────────────────────────────────────┘  │
│                                                    │
│  Phase 2: Vector Similarity                        │
│  ┌─────────────────────────────────────────────┐  │
│  │ • Embed query via provider                   │  │
│  │ • Search sqlite-vec for similar vectors      │  │
│  │ • Cosine similarity scoring                  │  │
│  │ • Filtered search (tags, path)              │  │
│  └─────────────────────────────────────────────┘  │
│                                                    │
│  Phase 3: Graph Traversal                          │
│  ┌─────────────────────────────────────────────┐  │
│  │ • Match nodes by name/type                   │  │
│  │ • Boost score by centrality (connections)    │  │
│  │ • Include connected entities                 │  │
│  └─────────────────────────────────────────────┘  │
│                                                    │
│  Phase 4: Re-ranking                               │
│  ┌─────────────────────────────────────────────┐  │
│  │ • Merge results from all phases              │  │
│  │ • Deduplicate by ID                          │  │
│  │ • Rank by combined score                     │  │
│  │ • Apply offset/limit                         │  │
│  └─────────────────────────────────────────────┘  │
│                                                    │
│  Phase 5: Facets + Suggestions                     │
│  ┌─────────────────────────────────────────────┐  │
│  │ • Compute type, language, source facets     │  │
│  │ • Generate query suggestions                │  │
│  └─────────────────────────────────────────────┘  │
│                                                    │
└──────────────────────────────────────────────────┘
                    │
                    ▼
            Structured Results
```

---

## Supported Features

| Feature | Status | Description |
|---------|--------|-------------|
| Full-text search | ✅ | SQLite FTS5 with Porter tokenizer |
| Vector search | ✅ | sqlite-vec with 128d embeddings |
| Graph search | ✅ | Entity relationship traversal |
| Hybrid search | ✅ | Combined FTS5 + Vector + Graph |
| Re-ranking | ✅ | Score-based result ordering |
| Faceted search | ✅ | Type, language, entity, source facets |
| Search explanations | ✅ | Why a result was returned |
| Query suggestions | ✅ | Auto-generated suggestion terms |
| Document indexing | ✅ | Full pipeline: parse → chunk → index |
| Directory indexing | ✅ | Recursive directory processing |
| File watching | ✅ | Auto-reindex on file changes |
| Synchronization | ✅ | Detect added/removed/modified files |
| Integrity verification | ✅ | Check consistency of all stores |
| Snapshots | ✅ | Point-in-time exports with graph |
| Rebuild | ✅ | Full rebuild of all indexes |
| Vacuum | ✅ | Clean stale data, reclaim space |
| Multi-level cache | ✅ | Memory + SQLite caching |

---

## Performance Characteristics

| Operation | Performance |
|-----------|-------------|
| FTS5 search (100K docs) | ~5ms p50, ~20ms p95 |
| Vector search (100K vectors) | ~15ms p50, ~50ms p95 |
| Document indexing | ~10ms/doc |
| Full re-index (10K files) | ~30s |
| Sync (incremental) | ~2s for 1000 files |
| Snapshot | ~500ms |
| Vacuum | ~1s (varies with DB size) |

---

## Database Schema

The knowledge engine uses a SQLite database with these core tables:

| Table | Purpose |
|-------|---------|
| `documents` | Indexed document metadata |
| `documents_fts` | FTS5 virtual table for documents |
| `chunks` | Document chunks with content |
| `chunks_fts` | FTS5 virtual table for chunks |
| `entities` | Extracted entities |
| `entities_fts` | FTS5 virtual table for entities |
| `code_blocks` | Extracted code blocks |
| `code_blocks_fts` | FTS5 virtual table for code blocks |
| `vectors` | Vector embeddings (sqlite-vec) |
| `cache` | Cached query results |
| `sync_log` | Synchronization history |
| `snapshots` | Snapshot metadata |

---

## Knowledge Engine Commands

```bash
# Search the knowledge base
cosca knowledge search "query"

# Search with type filter
cosca knowledge search "query" --type document
cosca knowledge search "query" --type chunk --type entity

# Search with path filter
cosca knowledge search "query" --path docs/architecture

# Get search explanation
cosca knowledge explain <result-id>

# Show engine statistics
cosca knowledge stats

# Index a file or directory
cosca knowledge index file.md
cosca knowledge index ./docs

# Sync with filesystem
cosca knowledge sync

# Rebuild all indexes
cosca knowledge rebuild

# Create a snapshot
cosca knowledge snapshot daily-backup

# Verify integrity
cosca knowledge verify

# Vacuum database
cosca knowledge vacuum
```

---

> **Related**: [Search Guide](search.md) | [Knowledge Graph](graph.md) | [ADR-002](../adr/ADR-002-knowledge-engine.md)
