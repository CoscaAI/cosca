# Knowledge Integration — Extracted from KERNEL.md §16

> **Source**: KERNEL.md v1.4.0-dev §16 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 16. KNOWLEDGE INTEGRATION

The Knowledge Integration subsystem transforms raw data (sessions, decisions, patterns, incidents) into **actionable intelligence**. It maintains a knowledge graph, enables semantic search, generates embeddings, captures snapshots, replays timelines, correlates events, compresses memory, and ranks results by relevance.

---

### 16.1 Knowledge Integration Architecture

```
┌───────────────────────────────────────────────────────────────────────────────────┐
│                           KNOWLEDGE INTEGRATION                                    │
│                                                                                   │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐    │
│  │   KNOWLEDGE  │    │   DECISION   │    │   PATTERN    │    │  CORRELATION │    │
│  │    GRAPH     │───▶│    GRAPH     │───▶│    GRAPH     │───▶│   ENGINE     │    │
│  │              │    │              │    │              │    │              │    │
│  │ Entities:    │    │ Decisions:   │    │ Patterns:    │    │ Connects     │    │
│  • capabilities│    │ • rationale   │    │ • arch       │    │ related      │    │
│  • workflows   │    │ • alternatives│    │ • design     │    │ knowledge    │    │
│  • decisions   │    │ • outcome     │    │ • code       │    │ across       │    │
│  • patterns    │    │ • confidence  │    │ • testing    │    │ graphs       │    │
│  • agents      │    │ • authority   │    │ • security   │    │              │    │
│  • sessions    │    └──────────────┘    │ • anti       │    └──────────────┘    │
│  • incidents   │                        └──────────────┘                        │
│  └──────────────┘                                                                │
│                                                                                   │
│  ┌───────────────────────────────────────────────────────────────────────────┐   │
│  │                          EMBEDDINGS PIPELINE                               │   │
│  │                                                                           │   │
│  │  Content → Chunk → Embed → Index → Store → Search → Rank                 │   │
│  │                                                                           │   │
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐      │   │
│  │  │ Chunk  │─▶│Embed   │─▶│ Index  │─▶│ Store  │─▶│ Search │─▶│ Rank   │      │   │
│  │  │ text   │  │ vector │  │IVF     │  │ pgvec  │  │ cosine │  │ score  │      │   │
│  │  └────────┘  └────────┘  └────────┘  └────────┘  └────────┘  └────────┘      │   │
│  └───────────────────────────────────────────────────────────────────────────┘   │
│                                                                                   │
│  ┌───────────────────────────────────────────────────────────────────────────┐   │
│  │                       SNAPSHOT & REPLAY SYSTEM                              │   │
│  │                                                                           │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │   │
│  │  │SNAPSHOT  │─▶│  STORE   │─▶│  REPLAY  │─▶│TIMELINE  │─▶│ ANALYZE │   │   │
│  │  │create    │  │ persist  │  │          │  │ display  │  │         │   │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │   │
│  └───────────────────────────────────────────────────────────────────────────┘   │
│                                                                                   │
│  ┌───────────────────────────────────────────────────────────────────────────┐   │
│  │                       MEMORY COMPRESSION                                    │   │
│  │  Compress → Deduplicate → Summarize → Rank → Archive                       │   │
│  └───────────────────────────────────────────────────────────────────────────┘   │
│                                                                                   │
└───────────────────────────────────────────────────────────────────────────────────┘
```

---

### 16.2 Knowledge Graph Specification

The Knowledge Graph is a **labeled property graph** that connects all knowledge entities in the Cosca ecosystem.

#### Graph Schema

```yaml
knowledge_graph:
  name: "Cosca Knowledge Graph"
  storage: "pgvector + adjacency table"
  
  node_types:
    Capability:
      properties: [id, name, category, version, status, quality_score]
      indexes: [id, category, status]
      
    Workflow:
      properties: [id, name, steps, category, owner]
      indexes: [id, category]
      
    Decision:
      properties: [id, title, rationale, alternatives, outcome, confidence, authority, timestamp]
      indexes: [id, authority, timestamp]
      
    Pattern:
      properties: [id, name, category, content, confidence, times_used, times_succeeded]
      indexes: [id, category, confidence]
      
    Agent:
      properties: [id, name, type, department, performance_score]
      indexes: [id, type, department]
      
    Session:
      properties: [id, timestamp, duration_ms, quality_score, runtime_type]
      indexes: [id, timestamp]
      
    Incident:
      properties: [id, title, severity, status, resolved_at, root_cause]
      indexes: [id, severity, status]
      
  edge_types:
    DEPENDS_ON:
      description: "Capability A depends on Capability B"
      properties: [strength, type]
      
    IMPLEMENTS:
      description: "Workflow implements Capability"
      properties: [completeness]
      
    RESOLVES:
      description: "Decision resolves a problem"
      properties: [efficacy]
      
    TRIGGERS:
      description: "Session triggered a Learning event"
      properties: [timestamp]
      
    RELATES_TO:
      description: "Pattern relates to Capability"
      properties: [relevance_score]
      
    CAUSED_BY:
      description: "Incident caused by root cause"
      properties: [confidence]
```

#### Graph Operations

| Operation | Description | Query Language | Performance |
|-----------|-------------|----------------|-------------|
| **Create node** | Add entity to graph | Cypher | O(1) |
| **Create edge** | Add relationship | Cypher | O(1) |
| **Find path** | Shortest path between entities | Cypher (BFS) | O(V+E) |
| **Find neighbors** | Direct connections | Cypher | O(degree) |
| **Community detection** | Find clusters | Leiden algorithm | O(V log V) |
| **Centrality** | Find most connected nodes | PageRank | O(V+E) |
| **Graph query** | Complex pattern match | Cypher | O(V+E) |

---

### 16.3 Decision Graph Specification

The Decision Graph is a **specialized subgraph** of the Knowledge Graph that tracks every decision made in the Cosca ecosystem.

```yaml
decision_graph:
  description: "Complete decision lineage — who decided what, when, and why"
  
  node_schema:
    Decision:
      id: "uuid"
      title: "string"
      description: "string"
      rationale: "string"
      alternatives: ["{name, description, pros, cons, rejected_reason}"]
      outcome: "accepted | rejected | superseded"
      confidence: 0.0-1.0
      authority: "CEO | CTO | Chief | Council | User"
      session_id: "uuid"
      timestamp: "ISO8601"
      tags: []
      
  edge_schema:
    SUPERSEDES:
      description: "Decision B supersedes Decision A"
      direction: "B → A"
      
    ALTERNATIVE_TO:
      description: "Decision B was an alternative to Decision A"
      direction: "bidirectional"
      
    IMPLEMENTED_BY:
      description: "Decision implemented by Workflow/Session"
      direction: "Decision → Workflow"
      
    REQUIRES:
      description: "Decision requires another decision as prerequisite"
      direction: "Decision → Decision"
      
  queries:
    - "Show full decision lineage for a given topic"
    - "Find all decisions made by a specific authority"
    - "Calculate decision confidence trends over time"
    - "Find superseded decisions that need review"
    - "Identify decisions with high confidence but poor outcomes"
```

---

### 16.4 Pattern Graph Specification

The Pattern Graph catalogs **what works, what doesn't, and why**.

```yaml
pattern_graph:
  description: "Patterns, anti-patterns, and their relationships"
  
  node_schema:
    Pattern:
      id: "uuid"
      name: "string"
      category: "architecture | design | code | testing | security | performance | anti-pattern"
      content: "string"
      context: "string"
      problem: "string"
      solution: "string"
      consequences: "string"
      confidence: 0.0-1.0
      times_used: 0
      times_succeeded: 0
      success_rate: 0.0  # computed
      
  edge_schema:
    SOLVES:
      description: "Pattern solves a specific problem type"
      direction: "Pattern → Problem"
      
    CONFLICTS_WITH:
      description: "Pattern conflicts with another pattern"
      direction: "bidirectional"
      
    COMPOSES_WITH:
      description: "Patterns compose well together"
      direction: "bidirectional"
      
    PRECEDES:
      description: "Pattern A should be applied before Pattern B"
      direction: "A → B"
      
  scoring:
    success_rate: "times_succeeded / times_used"
    confidence_boost: "success_rate × log2(times_used + 1)"
    final_confidence: "min(base_confidence + confidence_boost, 1.0)"
    
  queries:
    - "Find highest-confidence pattern for a problem"
    - "Find anti-patterns related to a technology"
    - "Show pattern composition chains"
    - "Identify patterns with declining success rates"
```

---

### 16.5 Embeddings Pipeline

#### Pipeline Stages

```
Content → Chunk → Embed → Index → Store → Search → Rank

1. Content: Raw text from knowledge stores, decisions, patterns
2. Chunk: Split into optimal-size segments (512 tokens)
3. Embed: Generate vector embedding via AI provider
4. Index: Build IVF index for fast approximate search
5. Store: Persist vectors in pgvector
6. Search: Cosine similarity search with filtering
7. Rank: Re-rank results by relevance + freshness + confidence
```

#### Chunking Strategy

```yaml
chunking:
  strategy: "Recursive text splitter"
  chunk_size: 512  # tokens
  chunk_overlap: 64
  separators: ["\n## ", "\n### ", "\n\n", "\n", ". ", " "]
  
  document_types:
    markdown: "Split by section (## heading boundaries)"
    yaml: "Split by top-level keys"
    plain_text: "Split by paragraph"
```

#### Embedding Configuration

```yaml
embedding:
  provider: "AI Provider (via Provider Interface)"
  model: "text-embedding-ada-002"
  dimensions: 1536
  batch_size: 20
  max_retries: 3
  timeout_ms: 30000
  
  supported_models:
    - name: "text-embedding-ada-002"
      dimensions: 1536
      provider: "openai"
    - name: "embed-english-v3.0"
      dimensions: 1024
      provider: "cohere"
    - name: "multilingual-e5-large"
      dimensions: 1024
      provider: "local"
      
  index_types:
    ivfflat:
      description: "Inverted file with flat quantization"
      probes: 10
      lists: 1000
      recall: 0.99
    hnsw:
      description: "Hierarchical navigable small world"
      ef_search: 40
      ef_construction: 200
      M: 16
      recall: 0.995
```

---

### 16.6 Semantic Search Protocol

```yaml
semantic_search:
  endpoint: "/api/v1/knowledge/search"
  method: "POST"
  
  request:
    query: "string (natural language)"
    filters:
      stores: ["patterns", "playbooks", "incidents", "architectures"]
      categories: []
      confidence_min: 0.0
      date_from: "ISO8601"
      date_to: "ISO8601"
    top_k: 20
    min_score: 0.7
    
  response:
    results:
      - id: "uuid"
        store: "patterns"
        title: "string"
        snippet: "string (with highlighted match)"
        score: 0.95
        confidence: 0.85
        freshness_hours: 72
        url: "knowledge/patterns/..."
        
  search_strategy:
    primary: "Vector similarity (cosine)"
    hybrid: "Vector + keyword (BM25 fusion)"
    fallback: "Keyword-only (when vector unavailable)"
    
  fusion:
    method: "Reciprocal rank fusion (RRF)"
    formula: "score = Σ(1 / (k + rank_vector(i))) + Σ(1 / (k + rank_keyword(i)))"
    k: 60  # RRF constant
```

---

### 16.7 Knowledge Snapshots

```yaml
knowledge_snapshots:
  description: "Point-in-time captures of knowledge state"
  
  creation:
    automatic:
      - "Pre-release (before Gate 3)"
      - "Post-release (after Gate 4)"
      - "Weekly (Sunday 2am)"
    manual:
      - "User-initiated via API"
      - "Pre/post major refactoring"
      
  schema:
    snapshot:
      id: "uuid"
      timestamp: "ISO8601"
      type: "pre_release | post_release | weekly | manual"
      size_bytes: 0
      stores_included: []
      graph_nodes: 0
      graph_edges: 0
      embedding_count: 0
      
  storage:
    format: "JSON + compressed vectors"
    retention: "90 days"
    max_snapshots: 52  # One per week
    
  restoration:
    - "Select snapshot by ID"
    - "Validate integrity (hash check)"
    - "Restore graph nodes"
    - "Restore graph edges"
    - "Restore vector indexes"
    - "Verify post-restore consistency"
    - "Publish SnapshotRestored event"
```

---

### 16.8 Knowledge Replay

```yaml
knowledge_replay:
  description: "Replay knowledge timeline for analysis, debugging, audit"
  
  replay_modes:
    full:
      description: "Replay all knowledge events from start time to end time"
      rate: "1x, 10x, 100x"
      use_case: "Audit trail analysis"
      
    filtered:
      description: "Replay events matching filter criteria"
      filters: ["store", "entity_type", "event_type"]
      use_case: "Debug specific knowledge change"
      
    session:
      description: "Replay knowledge changes that occurred during a session"
      session_id: "uuid"
      use_case: "Session impact analysis"
      
  replay_engine:
    source: "Knowledge changelog (append-only log)"
    event_types:
      - "knowledge.created"
      - "knowledge.updated"
      - "knowledge.deleted"
      - "knowledge.embedded"
      - "knowledge.searched"
    output: "Timeline visualization + metrics"
    
  use_cases:
    - "Audit: What knowledge changed and when?"
    - "Debug: Why did a search return different results?"
    - "Compliance: Show knowledge state at specific date"
    - "Learning: Replay knowledge evolution to understand trends"
```

---

### 16.9 Knowledge Timeline

```yaml
knowledge_timeline:
  description: "Temporal visualization of knowledge evolution"
  
  timeline_events:
    - "Decision created"
    - "Pattern discovered"
    - "Incident resolved"
    - "Capability updated"
    - "Workflow modified"
    - "Snapshot created"
    - "Knowledge pruned"
    
  visualization:
    format: "Interactive timeline (JSON for dashboard)"
    grouping: "By store, by day, by week"
    markers:
      - name: "Decision milestones"
        color: "blue"
      - name: "Pattern discoveries"
        color: "green"
      - name: "Incidents"
        color: "red"
      - name: "Snapshots"
        color: "purple"
        
  queries:
    - "Show all knowledge events in date range"
    - "Show knowledge velocity (events per day)"
    - "Correlate knowledge events with session activity"
    - "Identify periods of high knowledge creation"
```

---

### 16.10 Correlation Engine

```yaml
correlation_engine:
  description: "Connects related knowledge across stores, sessions, and time"
  
  correlation_types:
    semantic:
      description: "Content-based similarity (embedding cosine)"
      threshold: 0.85
      
    temporal:
      description: "Events occurring within same time window"
      window_ms: 3600000  # 1 hour
      
    causal:
      description: "Event A caused Event B (from causal analysis)"
      confidence: 0.0-1.0
      
    structural:
      description: "Entities connected by graph edges"
      max_hops: 3
      
    cooccurrence:
      description: "Entities appearing together frequently"
      min_cooccurrences: 3
      
  correlation_graph:
    description: "Meta-graph of knowledge correlations"
    edges:
      - type: "semantically_similar_to"
      - type: "temporally_related_to"
      - type: "causally_connected_to"
      - type: "structurally_dependent_on"
      - type: "cooccurs_with"
      
  use_cases:
    - "Find all knowledge related to a specific decision"
    - "Identify patterns that correlate with successful outcomes"
    - "Detect early warning signals from incident correlations"
    - "Recommend related knowledge when viewing an entry"
```

---

### 16.11 Memory Compression

```yaml
memory_compression:
  description: "Reduce memory footprint while preserving intelligence"
  
  compression_strategies:
    deduplication:
      method: "MinHash + LSH for near-duplicate detection"
      threshold: 0.90  # Jaccard similarity
      action: "Keep highest-confidence version, link duplicates"
      
    summarization:
      method: "Extractive summarization (LLM)"
      ratio: "4:1 compression"
      trigger: "Memory size > 10KB per record"
      
    pruning:
      method: "Remove low-confidence, low-utility entries"
      criteria:
        - "confidence < 0.3"
        - "not accessed in 90 days"
        - "superseded by newer entry"
        
    archiving:
      method: "Move cold entries to compressed archive"
      trigger: "Not accessed in 180 days"
      storage: "JSON Lines (gzip, ~10:1 compression)"
      
    aggregation:
      method: "Roll up similar entries into summary"
      trigger: "> 10 related entries on same topic"
      action: "Create aggregate entry, archive individuals"
      
  compression_pipeline:
    schedule: "Weekly (Sunday 3am)"
    steps:
      1. "Scan memory stores for compression candidates"
      2. "Run deduplication (MinHash + LSH)"
      3. "Run summarization (for large records)"
      4. "Apply pruning rules"
      5. "Archive cold entries"
      6. "Run aggregation"
      7. "Rebuild indexes"
      8. "Publish CompressionCompleted event"
      
  metrics:
    - "compression.ratio: pre/post size ratio"
    - "compression.entries_removed: dedup + pruned count"
    - "compression.entries_archived: archived count"
    - "compression.entries_summarized: summarized count"
    - "compression.duration_ms: total compression time"
```

---

### 16.12 Knowledge Ranking

```yaml
knowledge_ranking:
  description: "Rank knowledge results by combined relevance score"
  
  ranking_formula:
    final_score = 0.35 × vector_similarity
                + 0.20 × keyword_relevance
                + 0.15 × confidence_score
                + 0.15 × freshness_score
                + 0.10 × popularity_score
                + 0.05 × authority_score
                
  score_components:
    vector_similarity:
      description: "Cosine similarity of embeddings"
      range: 0.0-1.0
      
    keyword_relevance:
      description: "BM25 keyword matching score"
      range: 0.0-1.0
      
    confidence_score:
      description: "Pattern/decision confidence"
      range: 0.0-1.0
      
    freshness_score:
      description: "Recency: 1.0 if < 7 days, decays to 0.1 after 1 year"
      formula: "max(0.1, 1.0 - days_since_update / 365)"
      
    popularity_score:
      description: "Times accessed / total accesses"
      formula: "log2(access_count + 1) / log2(max_access_count + 1)"
      
    authority_score:
      description: "Authority of the source (CEO = 1.0, Specialist = 0.3)"
      values: { CEO: 1.0, CTO: 0.9, Chief: 0.7, Specialist: 0.3, Engine: 0.5, System: 0.4 }
      
  re_ranking:
    trigger: "Initial results returned"
    action: "Apply cross-encoder model for precision re-ranking"
    top_k_for_rerank: 50
    final_k: 20
```

---

### 16.13 Knowledge Events

| Event | Trigger | Payload | Consumers |
|-------|---------|---------|-----------|
| `KnowledgeGraphUpdated` | Graph node/edge change | entity_type, entity_id, change_type | Dashboard, Sync |
| `DecisionRecorded` | Decision created | decision_id, authority, outcome | Memory, Graph |
| `PatternDiscovered` | Pattern extracted | pattern_id, category, confidence | Graph, Ranking |
| `PatternUpdated` | Pattern confidence changed | pattern_id, new_confidence | Ranking |
| `EmbeddingGenerated` | Embedding complete | content_id, dimensions, model | Search, Index |
| `EmbeddingFailed` | Embedding error | content_id, error | Alerting |
| `SemanticSearchCompleted` | Search executed | query, result_count, duration_ms | Dashboard |
| `SnapshotCreated` | Snapshot taken | snapshot_id, size_bytes, stores | Storage, Audit |
| `SnapshotRestored` | Snapshot restored | snapshot_id, stores_restored | Dashboard |
| `KnowledgeReplayed` | Replay executed | mode, start_time, end_time, events_count | Audit |
| `CorrelationFound` | Correlation detected | type, entity_a, entity_b, strength | Graph, Dashboard |
| `CompressionCompleted` | Compression cycle | entries_removed, ratio, duration_ms | Dashboard, Memory |

---

### 16.14 Knowledge Metrics

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `knowledge.graph.nodes` | Gauge | node_type | Total graph nodes |
| `knowledge.graph.edges` | Gauge | edge_type | Total graph edges |
| `knowledge.graph.path_length` | Histogram | — | Average path length |
| `knowledge.search.latency_ms` | Histogram | search_type | Search response time |
| `knowledge.search.result_count` | Histogram | — | Results per query |
| `knowledge.search.zero_results` | Counter | — | Queries with no results |
| `knowledge.embedding.duration_ms` | Histogram | model | Embedding generation time |
| `knowledge.embedding.count` | Counter | model | Embeddings generated |
| `knowledge.embedding.failures` | Counter | model | Embedding failures |
| `knowledge.snapshot.count` | Counter | type | Snapshots created |
| `knowledge.snapshot.size` | Histogram | type | Snapshot size |
| `knowledge.replay.count` | Counter | mode | Replays executed |
| `knowledge.correlation.count` | Counter | type | Correlations detected |
| `knowledge.compression.ratio` | Gauge | — | Compression ratio |
| `knowledge.compression.entries` | Counter | action | Entries processed |
| `knowledge.ranking.distribution` | Histogram | score_bucket | Score distribution |


