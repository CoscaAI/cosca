---
name: memory
description: Manages all persistent knowledge across sessions - stores, retrieves, and organizes memories.
level: 3
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Memory Engine | **Last Updated**: 2026-07-10

# MEMORY ENGINE

## PURPOSE
The Memory Engine manages all persistent knowledge across sessions. It stores, retrieves, indexes, and organizes memories of all types. It enables the Cosca to learn and improve over time.

For canonical memory types, schemas, and storage locations, see [MEMORY_MODEL.md](../../MEMORY_MODEL.md).

## MEMORY OPERATIONS

### STORE
```
Input: { type, key, data, tags, timestamp }
Action: Write to appropriate memory store
Output: { stored, location, id }
```

### RETRIEVE
```
Input: { type, key?, query?, tags?, timeRange? }
Action: Read from appropriate memory store
Output: { results, count }
```

### SEARCH
```
Input: { query, types?, limit? }
Action: Full-text search across memory stores
Output: { results, relevance }
```

### INDEX
```
Input: { type }
Action: Rebuild search index for memory type
Output: { indexed, count }
```

### PRUNE
```
Input: { type, olderThan? }
Action: Archive or delete old memories
Output: { pruned, remaining }
```

## MEMORY FILE FORMAT

Each memory is stored as a markdown file:

```markdown
---
type: [short|long|project|architecture|decision|pattern|bug|agent]
key: [unique-key]
tags: [tag1, tag2]
timestamp: [ISO 8601]
status: [active|archived|superseded]
related: [key1, key2]
---

# [Title]

[Content]

## Context
[Why this memory exists]

## Decisions
[What was decided]

## Consequences
[What followed]

## Related
- [Link to related memories]
```

## AUTO-CAPTURE RULES
The Memory Engine should automatically capture:
1. Every architecture decision → .cosca/memory/decision/
2. Every bug fix → ${MEMORY_GLOBAL}/bug/
3. Every workflow completion → .cosca/memory/project/
4. Every deployment → .cosca/memory/project/
5. Every significant refactor → .cosca/memory/architecture/
6. Every session summary → .cosca/memory/short/

## INTEGRATION
- Called by Kernel at session start (retrieve)
- Called by Kernel at session end (store)
- Called by agents after decisions (store)
- Called by Review Chief after reviews (store)
- Called by Documentation Chief (link)

## DEPENDENCIES

| File | Purpose |
|------|---------|
| MEMORY_MODEL.md | Canonical memory taxonomy |

## RELATED
- [MEMORY_MODEL.md](../../MEMORY_MODEL.md) — Canonical memory types, schemas, and storage locations
- [Memory Chief](../../departments/memory/SKILL.md) — Orchestrates memory operations
- [Context Engine](../context/SKILL.md) — Provides session context from memory
- [Learning Engine](../learning/SKILL.md) — Derives patterns from stored memories

## HISTORY

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-07-10 | Initial version. Extracted memory hierarchy to MEMORY_MODEL.md. |
