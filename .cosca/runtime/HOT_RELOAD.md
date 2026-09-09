# Hot Reload — Extracted from KERNEL.md §14

> **Source**: KERNEL.md v3.0.1 §14 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 14. HOT RELOAD

The Runtime MUST support hot-reloading all configuration, capability, workflow, and memory Markdown files **without restarting**. Hot reload is the mechanism by which the Cosca ecosystem evolves in real-time — every edit to the `cosca/` directory is reflected in the Runtime within seconds.

---

### 14.1 Hot Reload Architecture

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              HOT RELOAD ENGINE                               │
│                                                                              │
│  ┌──────────────┐     ┌──────────────────┐     ┌─────────────────────────┐  │
│  │ FILE WATCHER │────▶│   CHANGE QUEUE   │────▶│    PIPELINE EXECUTOR    │  │
│  │              │     │                  │     │                         │  │
│  │ • inotify    │     │ • Debounce (1s)  │     │ • Per-file pipeline     │  │
│  │ • fsnotify   │     │ • Batch changes  │     │ • Dependency ordering   │  │
│  │ • Polling    │     │ • Prioritize     │     │ • Rollback on failure   │  │
│  │ (fallback)   │     │ • Deduplicate    │     │ • Event publishing      │  │
│  └──────────────┘     └──────────────────┘     └─────────────────────────┘  │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                         PIPELINE STAGES                                │   │
│  │                                                                        │   │
│  │  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐          │   │
│  │  │DETECT  │─▶│ QUEUE  │─▶│ PARSE  │─▶│VALIDATE│─▶│TRANSFORM          │   │
│  │  │ change │  │ change │  │ file   │  │ schema │  │ AST→Model          │   │
│  │  └────────┘  └────────┘  └────────┘  └────────┘  └────────┘          │   │
│  │                                                                        │   │
│  │  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐          │   │
│  │  │UPDATE  │─▶│UPDATE  │─▶│UPDATE  │─▶│PUBLISH │─▶│REFRESH │          │   │
│  │  │ DB     │  │ CACHE  │  │RUNTIME │  │ EVENT  │  │DASHBOARD           │   │
│  │  └────────┘  └────────┘  └────────┘  └────────┘  └────────┘          │   │
│  │                                                                        │   │
│  │  ┌────────────────────────────────────────────────────────────────┐   │   │
│  │  │                    ROLLBACK MANAGER                              │   │   │
│  │  │  On failure: restore previous state at each stage               │   │   │
│  │  │  Snapshot: pre-reload state saved before pipeline starts        │   │   │
│  │  └────────────────────────────────────────────────────────────────┘   │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

### 14.2 File Watcher Specification

#### Watch Configuration

```yaml
file_watcher:
  enabled: true
  
  watch_paths:
    - "${COSCA_DIR}/"           # Main Cosca directory
    - "${COSCA_DIR}/capabilities/"
    - "${COSCA_DIR}/workflows/"
    - "${COSCA_DIR}/memory/"
    - "${COSCA_DIR}/knowledge/"
    - "${COSCA_DIR}/company/"
    - "${COSCA_DIR}/engines/"
    - "${COSCA_DIR}/departments/"
    
  watch_patterns:
    - "*.md"
    - "*.yaml"
    - "*.yml"
    
  exclude_patterns:
    - "**/archive/**"
    - "**/node_modules/**"
    - "**/.git/**"
    
  detection:
    primary: "inotify (Linux) | FSEvents (macOS) | ReadDirectoryChanges (Windows)"
    fallback: "Polling (every 5s)"
    
  debounce:
    window_ms: 1000
    max_batch: 50
```

#### Detection Mechanism

| Mechanism | Platform | Latency | CPU Impact | Recommended |
|-----------|----------|---------|------------|-------------|
| **inotify** | Linux | < 10ms | Low | ✅ Primary |
| **FSEvents** | macOS | < 100ms | Low | ✅ Primary |
| **ReadDirectoryChanges** | Windows | < 100ms | Low | ✅ Primary |
| **Polling** | All (fallback) | 5s | Medium | ⚠️ Fallback |
| **Git hook** | All (git) | On commit | None | ✅ Supplemental |

#### File Event Types

| Event | Trigger | Action |
|-------|---------|--------|
| **Create** | New file in watched path | Full parse, validate, add to all stores |
| **Modify** | Existing file changed | Re-parse, validate, update all stores |
| **Delete** | File removed | Remove from all stores, update indexes |
| **Rename** | File moved | Handle as delete + create |
| **Permission change** | chmod/chown | Ignore (not a content change) |

---

### 14.3 Hot Reload Pipeline (Detailed)

Each stage has a formal contract with input, output, error handling, and rollback.

#### Stage 1 — Detect Change

```yaml
hr_stage_detect:
  name: "Detect Change"
  position: 1
  
  input:
    file_path: "string"
    event_type: "create | modify | delete"
    file_hash: "SHA-256"
    
  output:
    change_record:
      file: "path/to/file.md"
      type: "create | modify | delete"
      hash: "abc123..."
      timestamp: "ISO8601"
      
  error_handling:
    - "File disappeared before read → log, skip"
    - "Permission denied → log, skip"
    
  rollback: "Not applicable (detection only)"
```

#### Stage 2 — Queue Change

```yaml
hr_stage_queue:
  name: "Queue Change"
  position: 2
  
  input:
    change_record: {}
    
  output:
    queued: true | false
    batch_id: "uuid"
    
  debounce:
    window_ms: 1000
    logic: "If same file queued within window, coalesce (last change wins)"
    
  batch:
    max_size: 50
    flush_interval_ms: 2000
    
  priority:
    high: ["KERNEL.md", "QUALITY_GATES.md", "MEMORY_MODEL.md"]
    normal: ["capabilities/**", "workflows/**"]
    low: ["memory/**", "knowledge/**"]
    
  error_handling:
    - "Queue full → flush immediately, process batch"
    - "Duplicate detected → deduplicate, keep latest"
    
  rollback: "Not applicable (queuing only)"
```

#### Stage 3 — Parse File

```yaml
hr_stage_parse:
  name: "Parse File"
  position: 3
  
  input:
    file_path: "string"
    raw_content: "string"
    file_type: "markdown | yaml | yml"
    
  output:
    ast: {}
    frontmatter: {}
    sections: []
    parse_duration_ms: 0
    
  parsers:
    markdown:
      frontmatter: "YAML between --- markers"
      body: "CommonMark-compliant markdown"
    yaml:
      parser: "Strict YAML 1.2"
      
  error_handling:
    - "Parse error → fail, log error, skip to rollback"
    - "Empty file → warn, skip"
    - "Encoding error → fail, skip to rollback"
    
  rollback: "Discard parsed AST (no side effects yet)"
```

#### Stage 4 — Validate Changes

```yaml
hr_stage_validate:
  name: "Validate Changes"
  position: 4
  
  input:
    ast: {}
    file_path: "string"
    previous_ast: {}  # For comparison
    
  output:
    valid: true | false
    violations: []
    warnings: []
    
  checks:
    - "Schema compliance (all required fields)"
    - "Reference integrity (links resolve)"
    - "Cross-file consistency"
    - "No breaking changes to contracts"
    
  severity:
    block: "Block hot reload, trigger rollback"
    warn: "Allow hot reload, log warning"
    
  error_handling:
    - "Breaking contract change → BLOCK, rollback, notify"
    - "Broken reference → WARN, allow, log"
    - "Schema violation → BLOCK, rollback"
    
  rollback: "Discard validation results (no side effects)"
```

#### Stage 5 — Transform AST → Models

```yaml
hr_stage_transform:
  name: "Transform AST → Models"
  position: 5
  
  input:
    valid_ast: {}
    file_type: "string"
    
  output:
    runtime_models: []
    model_types: []
    
  transforms:
    capability_file: "Generate CapabilityModel"
    workflow_file: "Generate WorkflowModel"
    memory_file: "Generate MemoryRecord"
    knowledge_file: "Generate KnowledgeEntry"
    governance_file: "Generate GovernanceDocument"
    
  error_handling:
    - "Transform error → fail, rollback"
    - "Type mismatch → fail, rollback"
    
  rollback: "Discard generated models"
```

#### Stage 6 — Update Database

```yaml
hr_stage_db:
  name: "Update Database"
  position: 6
  
  input:
    runtime_models: []
    change_type: "create | update | delete"
    
  output:
    db_updated: true | false
    records_affected: 0
    
  operations:
    create: "INSERT"
    update: "UPDATE (diff-based, only changed fields)"
    delete: "DELETE (or soft-delete)"
    
  transaction:
    begin: "Before first write"
    commit: "After all writes"
    rollback: "On any error"
    
  error_handling:
    - "DB error → rollback transaction, fail"
    - "Constraint violation → rollback, log"
    
  rollback: "ROLLBACK transaction → DB restored to pre-reload state"
```

#### Stage 7 — Update Cache

```yaml
hr_stage_cache:
  name: "Update Cache"
  position: 7
  
  input:
    changed_keys: []
    change_type: "create | update | delete"
    
  output:
    cache_updated: true | false
    keys_invalidated: 0
    keys_written: 0
    
  operations:
    create: "SET key value WITH TTL"
    update: "DELETE key (lazy re-cache) or SET new value"
    delete: "DELETE key AND remove from indexes"
    
  error_handling:
    - "Redis error → log, continue (cache will be rebuilt on next read)"
    
  rollback: "Re-write previous values (from pre-reload snapshot)"
```

#### Stage 8 — Update Runtime In-Memory

```yaml
hr_stage_runtime:
  name: "Update Runtime In-Memory"
  position: 8
  
  input:
    runtime_models: []
    change_type: "create | update | delete"
    
  output:
    runtime_updated: true | false
    registries_updated: []
    
  updates:
    - "capability_registry"
    - "workflow_registry"
    - "memory_context"
    - "knowledge_context"
    
  atomicity:
    mechansim: "Copy-on-write for registries"
    effect: "In-flight queries see old version, new queries see new version"
    
  error_handling:
    - "Memory error → rollback, log critical"
    
  rollback: "Restore previous registry snapshots from pre-reload backup"
```

#### Stage 9 — Publish Event

```yaml
hr_stage_event:
  name: "Publish Event"
  position: 9
  
  input:
    change_record: {}
    success: true | false
    duration_ms: 0
    errors: []
    
  output:
    event_published: true | false
    
  events:
    success:
      name: "HotReloadCompleted"
      payload:
        file: "path"
        duration_ms: 1500
        changes: ["capability_registry", "workflow_registry"] 
    failure:
      name: "HotReloadFailed"
      payload:
        file: "path"
        duration_ms: 3000
        error: "Validation failed"
        rolled_back: true
        
  error_handling:
    - "Event bus unavailable → log, continue (dashboard refresh skipped)"
    
  rollback: "Not applicable (event is final)"
```

#### Stage 10 — Refresh Dashboard

```yaml
hr_stage_dashboard:
  name: "Refresh Dashboard"
  position: 10
  
  input:
    change_summary: {}
    
  output:
    dashboard_refreshed: true | false
    
  mechanism: "SSE push with change payload"
  
  payload:
    event: "hot_reload"
    file: "path/to/file.md"
    timestamp: "ISO8601"
    changes_summary: "Updated capability registry (CAP-ENG-001)"
    
  error_handling:
    - "SSE client disconnected → will pick up on next poll"
    
  rollback: "Not applicable (dashboard refresh is final)"
```

---

### 14.4 Rollback Mechanism

When any pipeline stage fails after side effects have been applied, the Hot Reload Engine MUST roll back all changes.

```yaml
hot_reload_rollback:
  triggers:
    - "Parse failure"
    - "Validation failure (block severity)"
    - "Database write failure"
    - "Runtime model update failure"
    
  mechanism:
    pre_reload:
      action: "Snapshot current state before starting pipeline"
      artifacts:
        - "DB: BEGIN TRANSACTION"
        - "Cache: Backup current values for affected keys"
        - "Runtime: Snapshot current registries"
        
    on_failure:
      action: "Restore snapshot in reverse order"
      sequence:
        - "1. Runtime: Restore registry snapshots"
        - "2. Cache: Restore previous values"
        - "3. DB: ROLLBACK TRANSACTION"
        
    on_success:
      action: "Commit changes"
      sequence:
        - "1. DB: COMMIT TRANSACTION"
        - "2. Cache: Confirm new values"
        - "3. Runtime: Release old snapshots"
        
  state_after_rollback: "Runtime returns to exact state before hot reload began"
  notification: "Publish HotReloadFailed event with rollback confirmation"
```

---

### 14.5 Concurrent Hot Reload Handling

Multiple file changes can arrive in rapid succession. The Hot Reload Engine handles concurrency through a formal protocol.

#### Concurrency Rules

```
HR-CON-01: Changes to the same file within debounce window (1s) are coalesced
HR-CON-02: Changes to different files within debounce window are batched
HR-CON-03: Dependent files are reloaded in dependency order
HR-CON-04: A hot reload in progress blocks new hot reloads (queue, not drop)
HR-CON-05: Maximum queue depth: 100 pending changes
HR-CON-06: If queue exceeds 100, oldest changes are dropped (last-write-wins)
HR-CON-07: KERNEL.md changes have highest priority (processed first)
HR-CON-08: Memory file changes have lowest priority (processed last)
```

#### Dependency-Aware Reload

When file A changes, the engine determines which other files might be affected:

```yaml
dependency_reload:
  enabled: true
  
  rules:
    - "CAPABILITY_CATALOG.md change → Reload all capability references"
    - "ORGCHART.md change → Reload all department references"
    - "QUALITY_GATES.md change → Reload quality engine rules"
    - "MEMORY_MODEL.md change → Reload memory store configuration"
    - "Workflow file change → Reload workflow registry"
    - "Engine SKILL.md change → Reload engine registry"
    
  cross_reference_index:
    purpose: "Pre-computed map of file → dependent files"
    update: "On every hot reload"
    storage: "Redis (cosca:hotreload:dependencies)"
```

---

### 14.6 Hot Reload Scope by File Type

| File Type | Hot Reload Supported? | Effect | Restart Required? |
|-----------|----------------------|--------|-------------------|
| `capabilities/**/*.md` | ✅ Full | Update capability registry, re-index | No |
| `workflows/*.md` | ✅ Full | Update workflow registry | No |
| `memory/**/*.md` | ✅ Full | Update memory context | No |
| `knowledge/**/*.md` | ✅ Full | Update knowledge base | No |
| `company/ORGCHART.md` | ✅ Full | Update organizational model | No |
| `QUALITY_GATES.md` | ✅ Full | Update quality gate rules | No |
| `MEMORY_MODEL.md` | ✅ Full | Update memory configuration | No |
| `GOVERNANCE.md` | ✅ Full | Update governance rules | No |
| `CONVENTIONS.md` | ✅ Full | Update convention rules | No |
| `engines/**/SKILL.md` | ✅ Full | Update engine registry | No |
| `departments/**/SKILL.md` | ✅ Full | Update department registry | No |
| `KERNEL.md` | ⚠️ Partial | Update non-structural sections only | Structural changes require restart |
| `RUNTIME_CONTRACT.md` | ✅ Full | Update runtime contract | No |
| `*.yaml` / `*.yml` | ✅ Full | Update configuration | No |

#### What CANNOT be Hot-Reloaded

```yaml
cannot_hot_reload:
  - "Changes to section structure of KERNEL.md (new/removed sections)"
  - "Changes to state machine definitions"
  - "Changes to event catalog structure"
  - "Changes to metrics schema"
  - "Removal of required contracts"
  - "Changes to the hot reload configuration itself"
  
  action: "Require explicit Runtime restart"
  detection: "Automatically detected during validation stage"
  notification: "Publish HotReloadRequiresRestart event"
```

---

### 14.7 Hot Reload Events

| Event | Trigger | Payload | Consumers |
|-------|---------|---------|-----------|
| `HotReloadTriggered` | File change detected | file, change_type, hash | Kernel, Dashboard |
| `HotReloadStarted` | Pipeline begins | file, batch_id | Dashboard |
| `HotReloadStageCompleted` | Each stage completes | stage_name, duration_ms | Dashboard |
| `HotReloadCompleted` | Pipeline succeeds | file, duration_ms, changes[] | Dashboard, All |
| `HotReloadFailed` | Pipeline fails | file, error, rolled_back | Dashboard, Kernel |
| `HotReloadPartial` | Some changes applied, some failed | succeeded[], failed[] | Dashboard |
| `HotReloadRequiresRestart` | Change requires restart | reason, sections[] | Kernel, User |
| `HotReloadRolledBack` | Rollback executed | file, restored_state | Dashboard, Audit |

---

### 14.8 Hot Reload Metrics

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `hotreload.triggered.count` | Counter | file_type | Hot reload triggers |
| `hotreload.duration_ms` | Histogram | file_type, status | Total hot reload duration |
| `hotreload.stage.duration_ms` | Histogram | stage_name | Per-stage duration |
| `hotreload.success.count` | Counter | file_type | Successful hot reloads |
| `hotreload.failure.count` | Counter | file_type, error_type | Failed hot reloads |
| `hotreload.rollback.count` | Counter | stage | Rollbacks executed |
| `hotreload.files_batched` | Histogram | — | Files per batch |
| `hotreload.queue.depth` | Gauge | — | Current queue depth |
| `hotreload.queue.wait_ms` | Histogram | — | Time in queue |
| `hotreload.bytes_processed` | Histogram | file_type | Bytes processed |

---

### 14.9 Hot Reload Security

```yaml
hot_reload_security:
  restrictions:
    - "Only files under COSCA_DIR can be hot-reloaded"
    - "Symbolic links to outside COSCA_DIR are NOT followed"
    - "Executable files (*.sh, *.py, *.js) are NEVER hot-reloaded"
    - "Binary files are NEVER hot-reloaded"
    - "Files > 10MB are skipped (log warning)"
    
  audit:
    - "All hot reloads are logged to sync_audit table"
    - "All failed hot reloads are logged as incidents"
    - "All rollbacks are logged with pre/post state"
    
  rate_limiting:
    max_per_minute: 60
    max_per_hour: 1000
    action_on_exceed: "Throttle to 1 change per 5 seconds"
```

---

### 14.10 Hot Reload Configuration

```yaml
hot_reload_config:
  enabled: true
  
  file_watcher:
    enabled: true
    mechanism: "auto-detect"
    polling_interval_ms: 5000
    
  pipeline:
    timeout_ms: 30000
    fail_on_error: true
    
  debounce:
    window_ms: 1000
    max_batch: 50
    
  rollback:
    enabled: true
    snapshot_runtime: true
    snapshot_cache: true
    
  dashboard:
    refresh_on_complete: true
    push_via_sse: true
    
  security:
    restrict_to_aos_dir: true
    max_file_size_bytes: 10485760  # 10MB
    rate_limit_per_minute: 60
```


