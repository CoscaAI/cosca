# Search Guide

> **Status**: active | **Owner**: AI Chief | **Last Updated**: 2026-07-23

## Hybrid Search Explained

The Cosca Knowledge Engine uses a **hybrid search** approach that combines three search strategies to deliver the most relevant results:

1. **FTS5 Full-Text Search** — Finds exact word/phrase matches using SQLite FTS5 with Porter stemming
2. **Vector Similarity Search** — Finds semantically similar content using embeddings and cosine similarity
3. **Knowledge Graph Search** — Finds related entities through graph traversal and centrality scoring

### Why Hybrid Search?

| Strategy | Strengths | Weaknesses |
|----------|-----------|------------|
| **FTS5** | Exact matches, fast, well-understood | Misses synonyms, variations |
| **Vector** | Semantic understanding, handles synonyms | Requires embedding provider, slower |
| **Graph** | Relation-aware, discovers connections | Limited to known entities |

By combining all three, the engine provides results that are both **precise** (FTS5 captures exact terms) and **comprehensive** (vector + graph find related content the user might not have thought to search for).

---

## Query Syntax

### Basic Queries

```
# Simple search
cosca knowledge search "authentication"

# Multiple terms
cosca knowledge search "user authentication flow"

# Phrase search (exact match)
cosca knowledge search "\"JWT token refresh\""

# Boolean terms (implied AND)
cosca knowledge search "database migration postgresql"
```

### FTS5 Query Syntax

The underlying FTS5 engine supports the following query operators:

| Operator | Syntax | Example |
|----------|--------|---------|
| AND (implied) | `term1 term2` | `auth login` |
| OR | `term1 OR term2` | `auth OR login` |
| NOT | `-term` | `auth -oauth` |
| Phrase | `"exact phrase"` | `"authentication flow"` |
| Prefix | `term*` | `auth*` (matches auth, auth, authenticate) |
| NEAR | `term1 NEAR term2` | `user NEAR permission` |

### Vector Search

Vector search is automatic when a query is provided. The query is embedded using the configured provider, and similar vectors are found in the vector store.

```
# Vector search is always active when EnableVector is true
cosca knowledge search "how does authentication work?"  → uses FTS5 + Vector
```

### Graph Search

Graph search finds entities matching the query by name and returns them with their relationships.

```
# Enable graph search explicitly
cosca knowledge search "authentication" --graph
```

---

## Filters and Facets

### Type Filters

Restrict results to specific content types:

```bash
# Search only documents
cosca knowledge search "query" --type document

# Search only code blocks
cosca knowledge search "query" --type code

# Search multiple types
cosca knowledge search "query" --type chunk --type entity
```

| Type | Description |
|------|-------------|
| `document` | Full documents |
| `chunk` | Document sections/chunks |
| `entity` | Extracted entities |
| `code` | Code blocks |

### Path Filters

Restrict results to a specific directory:

```bash
# Search within architecture docs
cosca knowledge search "query" --path docs/architecture

# Search within source code
cosca knowledge search "query" --path src/
```

### Tag/Metadata Filters

Filter by metadata tags (when using vector search):

```bash
cosca knowledge search "query" --tag language=go
cosca knowledge search "query" --tag framework=react
```

### Time Filters

Restrict to recent content:

```bash
# Search content updated in the last 7 days
cosca knowledge search "query" --since 7d
```

### Threshold Filters

Set minimum score threshold:

```bash
# Only show high-confidence results
cosca knowledge search "query" --min-score 0.8
```

### Limit and Offset

```bash
# Show 50 results
cosca knowledge search "query" --limit 50

# Skip first 20 results (pagination)
cosca knowledge search "query" --limit 10 --offset 20
```

### Facets

Enable faceted results to see result breakdowns:

```bash
cosca knowledge search "query" --facets
```

Facets returned:
- `type` — Counts by result type (document, chunk, entity, code)
- `entity_type` — Counts by entity type
- `language` — Counts by programming language
- `source` — Counts by search source (fts, vector, graph)

---

## Type-Specific Search

### Document Search

```bash
# Search documents only
cosca knowledge search "authentication" --type document --limit 10
```

### Code Search

```bash
# Search code blocks
cosca knowledge search "database query" --type code

# Search by language
cosca knowledge search "middleware" --type code --tag language=go
```

### Entity Search

```bash
# Search entities
cosca knowledge search "UserService" --type entity --graph
```

---

## Search Examples

### Example 1: Finding Documentation

```bash
$ cosca knowledge search "how to configure the knowledge engine"

Results (15 found, 1.2ms):
  Rank  Score  Type      Title
     1  0.92   document  Knowledge Engine Overview — docs/knowledge/overview.md
     2  0.85   chunk     Search Pipeline — docs/knowledge/overview.md
     3  0.78   chunk     Configuration — docs/runtime/configuration.md
     4  0.72   document  Configuration Reference — docs/runtime/configuration.md
     5  0.65   entity    KnowledgeEngine — entity defined in internal/knowledge/

Facets:
  type:     document=2, chunk=2, entity=1
  source:   fts=4, vector=1
```

### Example 2: Code Search

```bash
$ cosca knowledge search "gRPC client connection" --type code --tag language=go

Results (8 found, 0.8ms):
  Rank  Score  Language  File
     1  0.94   go        internal/editors/claude/claude.go
     2  0.88   go        internal/editors/opencode/opencode.go
     3  0.76   go        internal/runtime/runtime.go
     4  0.70   go        api/grpcserver/server.go
```

### Example 3: Entity-Focused Search

```bash
$ cosca knowledge search "Runtime" --graph --type entity

Results (5 found):
  Rank  Score  Entity          Type          Connections
     1  0.95   Runtime         Component     8 connections
     2  0.80   RuntimeState    StateMachine  3 connections
     3  0.75   RuntimeConfig    Config        5 connections
     4  0.70   EventBus        EventSystem   4 connections
     5  0.65   Lifecycle       Component     3 connections
```

### Example 4: Explaining Results

```bash
$ cosca knowledge explain "abc123"

Explanation for result abc123:
  Type: chunk
  Name: 4a2b8c9d
  Factors:
    content_match:     0.80
    vector_similarity: 0.60
    has_embedding:     1.00
  Connections:
    - connected to KnowledgeEngine (component)
    - connected to SearchPipeline (component)
```

---

## Search Parameters Reference

```go
type SearchParams struct {
    Query       string              // Search query string
    Limit       int                 // Max results (default: 20)
    Offset      int                 // Results to skip
    Types       []string            // Filter by type
    Path        string              // Filter by path prefix
    Tags        map[string]string   // Filter by tags
    Since       time.Time           // Filter by recency
    EnableFTS   bool                // Enable FTS5 (default: true)
    EnableVector bool               // Enable vector (default: true)
    EnableGraph bool                // Enable graph (default: false)
    EnableFacets bool               // Enable facets (default: false)
    MinScore    float64             // Minimum score threshold
}
```

---

## Best Practices

1. **Be specific** — Use exact terms for precise results
2. **Use phrases** — `"exact phrase"` for targeted searches
3. **Combine filters** — `--type code --tag language=go` narrows results
4. **Use graph for entities** — Add `--graph` when searching for components
5. **Check explanations** — Use `cosca knowledge explain` to understand results
6. **Enable facets** — Use `--facets` when exploring unfamiliar topics
7. **Adjust limits** — Increase `--limit` for comprehensive results

---

> **Related**: [Knowledge Engine Overview](overview.md) | [Knowledge Graph](graph.md) | [ADR-002](../adr/ADR-002-knowledge-engine.md)
