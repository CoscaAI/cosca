# Runtime Synchronization Pipeline — Extracted from KERNEL.md §13

> **Source**: KERNEL.md v1.4.0-dev §13 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 13. RUNTIME SYNCHRONIZATION PIPELINE

All data flows through this official pipeline. **No implementation may bypass it.** Every artifact — Markdown files, capability definitions, workflow definitions, memory records, knowledge entries — MUST pass through the complete pipeline before being consumed by any Runtime component.

---

### 13.1 Sync Pipeline Architecture

```
┌────────────────────────────────────────────────────────────────────────────────────┐
│                         SYNC PIPELINE                                               │
│                                                                                    │
│  Source → Parse → Validate → Transform → Store → Index → Cache → Serve            │
│                                                                                    │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────────┐   │
│  │  SOURCE  │──▶  PARSE   │──▶ VALIDATE  │──▶TRANSFORM │──▶     STORE        │   │
│  │          │  │          │  │          │  │          │  │                   │   │
│  │ .md      │  │ YAML FM  │  │ Schema   │  │ AST →    │  │ ┌───────────────┐ │   │
│  │ .yaml    │  │ Markdown │  │ Contract │  │ Models   │  │ │  Database     │ │   │
│  │ .json    │  │ Sections │  │ Ref      │  │          │  │ └───────────────┘ │   │
│  │ memory/  │  │ Links    │  │          │  │          │  │ ┌───────────────┐ │   │
│  │          │  │          │  │          │  │          │  │ │   Redis       │ │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  │ └───────────────┘ │   │
│                                                           │ ┌───────────────┐ │   │
│                                                           │ │  Vector Store │ │   │
│                                                           │ └───────────────┘ │   │
│                                                           └───────────────────┘   │
│                                                                    │              │
│                                                                    ▼              │
│                                                           ┌───────────────────┐   │
│                                                           │   SERVE & SYNC    │   │
│                                                           │                   │   │
│                                                           │ ┌───────────────┐ │   │
│                                                           │ │  Runtime      │ │   │
│                                                           │ │  Models       │ │   │
│                                                           │ └───────────────┘ │   │
│                                                           │ ┌───────────────┐ │   │
│                                                           │ │  Dashboard    │ │   │
│                                                           │ └───────────────┘ │   │
│                                                           │ ┌───────────────┐ │   │
│                                                           │ │  API/CLI     │ │   │
│                                                           │ └───────────────┘ │   │
│                                                           └───────────────────┘   │
│                                                                                    │
│  ┌────────────────────────────────────────────────────────────────────────────┐   │
│  │                         SYNC ORCHESTRATOR                                   │   │
│  │  Triggers: Startup | Hot Reload | Periodic (60s) | Manual                  │   │
│  │  Modes: Full Sync | Delta Sync (changed files only)                         │   │
│  │  Health: Sync Status | Last Sync | Sync Errors | Pending Changes           │   │
│  └────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────┘
```

---

### 13.2 Pipeline Stage Contracts

Each pipeline stage has a formal contract defining inputs, outputs, validations, and error handling.

#### Stage 1 — Source Discovery

```yaml
sync_stage_source:
  name: "Source Discovery"
  position: 1
  
  input:
    - "Workspace path"
    - "File patterns (globs)"
    - "Last sync timestamp (for delta)"
    
  output:
    files_to_process: []
    total_files: 0
    changed_files: 0
    deleted_files: 0
    total_size_bytes: 0
    
  discovery_patterns:
    - "**/*.md"
    - "cosca/**/*.yaml"
    - "cosca/**/*.yml"
    - "cosca/memory/**/*"
    - "cosca/knowledge/**/*"
    
  delta_detection:
    method: "file_hash (SHA-256) + mtime"
    cache: "Redis (file_hashes key)"
    
  error_handling:
    - "Permission denied → skip file, log warning"
    - "File too large (>10MB) → skip, log warning"
    - "Binary file detected → skip, log debug"
    
  timeout_ms: 30000
```

#### Stage 2 — Parser

```yaml
sync_stage_parser:
  name: "Parser"
  position: 2
  
  input:
    file_path: "string"
    raw_content: "string"
    file_type: "markdown | yaml | json"
    
  output:
    ast: {}
    frontmatter: {}
    sections: []
    metadata:
      parsed_at: "ISO8601"
      parse_duration_ms: 12
      line_count: 150
      
  parsers:
    markdown:
      frontmatter: "YAML between --- markers"
      sections: "## and ### headings"
      tables: "GitHub-flavored markdown tables"
      code_blocks: "Fenced code blocks with language"
      links: "Internal and external references"
      lists: "Ordered and unordered"
      
    yaml:
      parser: "Strict YAML 1.2"
      schema: "JSON Schema validation"
      
    json:
      parser: "Strict JSON"
      schema: "JSON Schema validation"
      
  validation:
    - "Valid YAML frontmatter"
    - "No syntax errors"
    - "Character encoding: UTF-8"
    - "Line endings: LF (Unix)"
    
  error_handling:
    - "YAML parse error → fail file, log error"
    - "Markdown syntax error → warn, continue"
    - "Encoding error → fail file, log error"
    
  timeout_ms: 5000
```

#### Stage 3 — Validation

```yaml
sync_stage_validation:
  name: "Validation"
  position: 3
  
  input:
    ast: {}
    file_type: "string"
    
  output:
    valid: true | false
    violations: []
    warnings: []
    
  validation_rules:
    metadata:
      - "Required fields present (version, status, owner)"
      - "Version is valid SemVer"
      - "Status is valid (active, deprecated, retired)"
      
    structure:
      - "Required sections present (per file type)"
      - "No duplicate section IDs"
      - "Section hierarchy valid"
      
    references:
      - "All internal links resolve to existing files"
      - "All capability references exist in registry"
      - "All provider references exist in ORGCHART"
      
    contracts:
      - "Input/output schemas match contract requirements"
      - "Quality criteria defined (if applicable)"
      - "Dependencies reference valid entities"
      
  severity:
    error: "Block file from sync"
    warn: "Allow sync, log warning"
    
  timeout_ms: 10000
```

#### Stage 4 — Transform (AST → Runtime Models)

```yaml
sync_stage_transform:
  name: "Transform"
  position: 4
  
  input:
    valid_ast: {}
    file_type: "string"
    
  output:
    runtime_models: []
    model_types: []
    
  transforms:
    capability:
      source: "capabilities/CAPABILITY_CATALOG.md"
      target: "CapabilityModel"
      fields:
        - "id → capability_id"
        - "provider → provider_ref"
        - "dependencies → dependency_ids[]"
        
    workflow:
      source: "workflows/*.md"
      target: "WorkflowModel"
      fields:
        - "steps → workflow_steps[]"
        - "inputs → input_schema"
        - "outputs → output_schema"
        
    memory:
      source: "memory/**/*.md"
      target: "MemoryRecord"
      fields:
        - "type → memory_type"
        - "key → record_key"
        - "tags → tag_array"
        
    knowledge:
      source: "knowledge/**/*.md"
      target: "KnowledgeEntry"
      fields:
        - "category → knowledge_category"
        - "content → text_content"
        
    governance:
      source: "*.md (root level)"
      target: "GovernanceDocument"
      fields:
        - "version → doc_version"
        - "status → lifecycle_status"
        
  cross_references:
    - "Resolve file links to model references"
    - "Build dependency graph of all models"
    - "Detect circular references"
    
  timeout_ms: 30000
```

#### Stage 5 — Database Store

```yaml
sync_stage_database:
  name: "Database Store"
  position: 5
  
  input:
    runtime_models: []
    
  output:
    stored_count: 0
    updated_count: 0
    deleted_count: 0
    errors: []
    
  operations:
    upsert:
      - "INSERT ON CONFLICT UPDATE for each model"
      - "Batch size: 100 records"
    delete:
      - "Remove models for deleted source files"
      - "Soft delete (archive flag)"
      
  schema:
    tables:
      capabilities:
        columns: [id, name, version, status, provider, inputs, outputs, quality_criteria, dependencies, created_at, updated_at]
        indexes: [id, status, provider]
        
      workflows:
        columns: [id, name, steps, category, owner, input_schema, output_schema, created_at, updated_at]
        indexes: [id, category]
        
      memory_records:
        columns: [id, type, key, tags, content, status, session_id, agent, timestamp]
        indexes: [type, key, tags, timestamp]
        
      knowledge_entries:
        columns: [id, category, title, content, tags, confidence, embedding_id, created_at]
        indexes: [category, tags, confidence]
        
      governance_documents:
        columns: [id, name, version, status, owner, content_hash, updated_at]
        indexes: [name, status]
        
      sync_audit:
        columns: [id, sync_id, type, files_total, files_changed, files_deleted, duration_ms, status, error_message]
        indexes: [sync_id, timestamp]
        
  timeout_ms: 60000
```

#### Stage 6 — Redis Cache

```yaml
sync_stage_redis:
  name: "Redis Cache"
  position: 6
  
  input:
    runtime_models: []
    
  output:
    cached_keys: 0
    invalidated_keys: 0
    
  cache_strategy:
    pattern: "cache:write-through"
    
  key_schema:
    capabilities: "cosca:capability:{id}"
    workflows: "cosca:workflow:{id}"
    memory: "cosca:memory:{type}:{key}"
    knowledge: "cosca:knowledge:{category}:{id}"
    governance: "cosca:gov:{name}"
    indexes:
      - "cosca:index:capabilities"  # Set of all capability IDs
      - "cosca:index:workflows"     # Set of all workflow IDs
      - "cosca:index:memory_types"  # Set of memory types
      
  ttl:
    capabilities: "1 hour"
    workflows: "1 hour"
    memory: "30 minutes"
    knowledge: "2 hours"
    governance: "1 hour"
    indexes: "no expiry"
    
  invalidation:
    on_update: "Delete key, re-cache on next read"
    on_delete: "Delete key and remove from indexes"
    on_full_sync: "Flush all cosca:* keys"
    
  timeout_ms: 15000
```

#### Stage 7 — Vector Store Index

```yaml
sync_stage_vector:
  name: "Vector Store Index"
  position: 7
  
  input:
    text_content: []
    model_type: "text-embedding-ada-002 | bert | custom"
    
  output:
    indexed_count: 0
    embedding_dimensions: 1536
    index_name: "aos_knowledge"
    
  embedding:
    provider: "AI Provider (via Provider Interface)"
    model: "text-embedding-ada-002"
    batch_size: 20
    dimensions: 1536
    
  index_types:
    knowledge_entries:
      source: "knowledge/**/*.md"
      index: "aos_knowledge_ivfflat"
      distance: "cosine"
      
    capabilities:
      source: "capabilities/CAPABILITY_CATALOG.md"
      index: "aos_capabilities_ivfflat"
      distance: "cosine"
      
    memory:
      source: "memory/decision/*.md"
      index: "aos_decisions_ivfflat"
      distance: "cosine"
      
  search_config:
    probes: 10
    max_results: 20
    min_score: 0.7
    
  timeout_ms: 240000
```

#### Stage 8 — Runtime Model Loading

```yaml
sync_stage_runtime:
  name: "Runtime Model Loading"
  position: 8
  
  input:
    database_models: []
    cache_models: []
    
  output:
    loaded_models: {}
    memory_usage_bytes: 0
    
  loading_strategy:
    primary: "Load from Redis cache"
    fallback: "Load from Database"
    
  model_registry:
    capability_registry: "All active capabilities"
    workflow_registry: "All active workflows"
    memory_context: "Session-relevant memories"
    knowledge_context: "Relevant knowledge entries"
    
  cache_warming:
    on_startup: "Load all active capabilities and workflows"
    on_session: "Load session-specific memories"
    on_request: "Load request-relevant knowledge"
    
  timeout_ms: 30000
```

#### Stage 9 — Dashboard/API/CLI Sync

```yaml
sync_stage_serve:
  name: "Dashboard/API/CLI Sync"
  position: 9
  
  input:
    runtime_state: {}
    
  output:
    dashboard_updated: true
    api_indexes_updated: true
    
  dashboard:
    mechanism: "SSE push + REST polling"
    events:
      - "SyncCompleted → refresh dashboard"
      - "FileChanged → update live preview"
    endpoints:
      - "GET /api/v1/sync/status"
      - "GET /api/v1/sync/history"
      
  api:
    mechanism: "Rebuild in-memory indexes"
    indexes:
      - "capability_index"
      - "workflow_index"
      - "memory_index"
      - "knowledge_index"
      
  cli:
    mechanism: "Cache invalidation on next command"
    cache_location: "~/.cosca/cache/"
    
  timeout_ms: 15000
```

---

### 13.3 Sync Triggers & Modes

#### Sync Triggers

| Trigger | Description | Mode | Priority | Max Frequency |
|---------|-------------|------|----------|---------------|
| **Runtime Startup** | First initialization | Full | Critical | Once per startup |
| **Hot Reload** | File change detected | Delta | High | 1s debounce |
| **Periodic** | Scheduled sync | Delta | Low | Every 60s |
| **Manual** | User-initiated (`/sync`) | Full or Delta | User-defined | On demand |
| **Post-Session** | After session completion | Delta | Medium | Per session |
| **Post-Evolution** | Evolution Engine updates | Delta | Low | Per evolution cycle |

#### Sync Modes

```yaml
sync_modes:
  full:
    description: "Complete rebuild of all stores"
    when: "Runtime startup, manual full sync"
    actions:
      - "Full source discovery"
      - "Parse all files"
      - "Validate all files"
      - "Transform all models"
      - "Truncate database tables"
      - "Flush Redis cache"
      - "Rebuild vector indexes"
      - "Reload Runtime models"
      - "Refresh Dashboard"
    duration_estimate: "30s-120s"
    
  delta:
    description: "Only changed files"
    when: "Hot reload, periodic, post-session"
    actions:
      - "Hash-based change detection"
      - "Parse changed files only"
      - "Validate changed files only"
      - "Transform changed models"
      - "Upsert database records"
      - "Invalidate Redis keys"
      - "Update vector indexes (changed only)"
      - "Reload changed Runtime models"
      - "Push delta to Dashboard"
    duration_estimate: "1s-10s"
```

---

### 13.4 Conflict Resolution

When sync conflicts occur (e.g., concurrent edits), the pipeline follows formal resolution rules.

| Conflict Type | Detection | Resolution | Data Loss? |
|--------------|-----------|------------|------------|
| **File modified during sync** | Hash mismatch | Re-scan file, re-sync | No |
| **Concurrent hot reloads** | Debounce (1s) | Last write wins | No (coalesced) |
| **Database constraint violation** | DB error | Rollback batch, retry | No |
| **Vector index conflict** | Duplicate ID | Upsert (overwrite) | No |
| **Memory record conflict** | Key collision | Last timestamp wins | Yes (old data) |
| **Cross-reference broken** | Validation error | Block sync, notify | No |

#### Resolution Priority

```
1. Last-write-wins (timestamp-based)
2. Manual override (user decides)
3. Rollback to previous consistent state
4. Block sync, escalate to administrator
```

---

### 13.5 Sync Health & Monitoring

#### Sync Status Model

```yaml
sync_status:
  last_full_sync:
    timestamp: "ISO8601"
    duration_ms: 45000
    files_processed: 156
    status: "completed | failed"
    
  last_delta_sync:
    timestamp: "ISO8601"
    duration_ms: 3200
    files_changed: 3
    status: "completed | failed"
    
  pending_changes:
    count: 0
    files: []
    
  sync_health:
    status: "healthy | degraded | stalled"
    consecutive_failures: 0
    last_error: "string | null"
```

#### Sync Metrics

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `sync.duration_ms` | Histogram | mode | Sync duration |
| `sync.files.processed` | Histogram | mode | Files processed |
| `sync.files.changed` | Gauge | mode | Changed files |
| `sync.files.failed` | Counter | stage | File failures |
| `sync.validation.violations` | Counter | severity | Validation violations |
| `sync.db.upserted` | Counter | table | Database upserts |
| `sync.db.deleted` | Counter | table | Database deletes |
| `sync.redis.keys_cached` | Gauge | — | Redis keys cached |
| `sync.vector.indexed` | Counter | index | Vector embeddings indexed |
| `sync.errors` | Counter | stage | Sync errors |
| `sync.conflicts` | Counter | type | Sync conflicts |

#### Sync Health Dashboard

```yaml
sync_dashboard:
  sections:
    - name: "Sync Status"
      widgets:
        - "Last full sync: timestamp + duration"
        - "Last delta sync: timestamp + duration"
        - "Pending changes: count"
        - "Sync health: healthy/degraded/stalled"
        
    - name: "Sync Performance"
      widgets:
        - "Sync duration by mode (graph)"
        - "Files processed per sync (graph)"
        - "Stage breakdown (table)"
        
    - name: "Sync Errors"
      widgets:
        - "Error count by stage (table)"
        - "Failed files list"
        - "Validation violations timeline"
```

---

### 13.6 Sync Configuration

```yaml
sync_config:
  triggers:
    startup:
      enabled: true
      mode: "full"
      
    hot_reload:
      enabled: true
      mode: "delta"
      debounce_ms: 1000
      
    periodic:
      enabled: true
      mode: "delta"
      interval_ms: 60000
      
    post_session:
      enabled: true
      mode: "delta"
      
  storage:
    database:
      enabled: true
      connection_string: "${DATABASE_URL}"
      max_batch_size: 100
      
    redis:
      enabled: true
      connection_string: "${REDIS_URL}"
      key_prefix: "cosca:"
      
    vector:
      enabled: true
      model: "text-embedding-ada-002"
      index_type: "ivfflat"
      
  validation:
    strict: true
    fail_on_error: true
    max_warnings: 100
    
  performance:
    max_parallel_files: 10
    max_batch_size: 100
    compression: true
```


