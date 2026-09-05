# MEMORY MODEL — Canonical Memory Taxonomy

> **Version**: 4.0.0 | **Status**: active | **Owner**: Memory Chief

## Purpose
Single source of truth for all memory types, schemas, storage locations, and lifecycle policies. Referenced by KERNEL.md, Memory Engine, Memory Chief, Context Engine, and Learning Engine.

---

## Memory Architecture

```
Layer 1: Session    (Transient)      → .cosca/memory/short/
Layer 2: Project    (Project-scoped) → .cosca/memory/project/, .cosca/memory/long/
Layer 3: System     (System-scoped)  → .cosca/memory/architecture/, .cosca/memory/decision/
Layer 4: Wisdom     (Cross-project)  → ${MEMORY_GLOBAL}/pattern/, ${MEMORY_GLOBAL}/bug/
Layer 5: Agent      (Cross-project)  → ${MEMORY_GLOBAL}/agent/
```

> **PRINCÍPIO DO CÉREBRO LEVE (ordem do Don, 2026-08-27):** a memória é **armazenada em bulk** (indexada), mas **nunca carregada em bulk** no contexto do agente. O load padrão entrega **índices/referências** (caminho, tipo, tags, resumo). O **conteúdo completo** é recuperado **sob demanda**, via busca semântica por significado (`cosca knowledge search`), apenas quando o domínio da tarefa exige. Isso mantém o cérebro enxuto e saudável, evita poluição de tokens e impede ação baseada em informação irrelevante ou obsoleta.

---

## Memory Types

### 1. Short Memory — Session Transient
| Attribute | Value |
|-----------|-------|
| Purpose | Active task context, current decisions, pending actions |
| Scope | Current session only |
| Lifetime | Session duration |
| Location | `.cosca/memory/short/` |
| Cleanup | Cleared at session end; important entries promoted to long memory |
| Access Pattern | Read/write heavy during session |
| Index | `memory/short/INDEX.md` |

### 2. Long Memory — Cross-Session Project Knowledge
| Attribute | Value |
|-----------|-------|
| Purpose | Information persisting across sessions |
| Scope | Project lifetime |
| Lifetime | Permanent (project duration) |
| Location | `.cosca/memory/long/` |
| Cleanup | Manual or via Evolution Engine pruning |
| Access Pattern | Write on session end, read on session start |
| Index | `memory/long/INDEX.md` |

### 3. Project Memory — Project-Specific Knowledge
| Attribute | Value |
|-----------|-------|
| Purpose | Features, modules, releases, metrics, issues |
| Scope | Project lifetime |
| Lifetime | Project duration |
| Location | `.cosca/memory/project/` |
| Sub-stores | `features/`, `modules/`, `releases/`, `metrics/`, `issues/` |
| Access Pattern | Continuous read/write |
| Index | `memory/project/INDEX.md` |

### 4. Architecture Memory — System Architecture
| Attribute | Value |
|-----------|-------|
| Purpose | ADRs, design patterns, module contracts, integration points |
| Scope | System lifetime |
| Lifetime | Permanent (system duration) |
| Location | `.cosca/memory/architecture/` |
| Sub-stores | `adr/`, `patterns/`, `contracts/`, `integrations/`, `decisions/` |
| Access Pattern | Write on decisions, read on planning |
| Index | `memory/architecture/INDEX.md` |

### 5. Decision Memory — Decision Record
| Attribute | Value |
|-----------|-------|
| Purpose | All significant decisions and their rationale |
| Scope | Permanent |
| Lifetime | Forever |
| Location | `.cosca/memory/decision/` |
| Access Pattern | Write after every decision, read on context loading |
| Index | `memory/decision/INDEX.md` |

### 6. Pattern Memory — Cross-Project Wisdom
| Attribute | Value |
|-----------|-------|
| Purpose | Patterns that work, anti-patterns to avoid |
| Scope | Global (cross-project) |
| Lifetime | Permanent |
| Location | `${MEMORY_GLOBAL}/pattern/` |
| Sub-stores | `architecture/`, `design/`, `code/`, `testing/`, `performance/`, `security/`, `anti-patterns/`, `frameworks/` |
| Access Pattern | Write by Evolution/Learning engines, read globally |
| Index | `memory/pattern/INDEX.md` |

### 7. Bug Memory — Bug Catalog
| Attribute | Value |
|-----------|-------|
| Purpose | Bugs encountered and their fixes |
| Scope | Global (cross-project) |
| Lifetime | Permanent |
| Location | `${MEMORY_GLOBAL}/bug/` |
| Access Pattern | Write after bug fixes, read during diagnosis |
| Index | `memory/bug/INDEX.md` |

### 8. Agent Memory — Agent Performance
| Attribute | Value |
|-----------|-------|
| Purpose | Agent performance metrics, preferences, learning history |
| Scope | Global (cross-project) |
| Lifetime | Permanent |
| Location | `${MEMORY_GLOBAL}/agent/` |
| Access Pattern | Write by Learning Engine, read by Kernel |
| Index | `memory/agent/INDEX.md` |

---

## Memory Record Schema

All memory records follow this YAML frontmatter schema:

```yaml
---
type: short | long | project | architecture | decision | pattern | bug | agent
key: unique-identifier
tags: [tag1, tag2]
timestamp: ISO8601
status: active | archived | superseded
related: [key1, key2]
agent: agent-name
session: session-id
---
```

### Type-Specific Extensions

#### Decision Records
```yaml
decided_by: agent-name
confidence: 0.0-1.0
alternatives_considered: [alt1, alt2]
```

#### Pattern Records
```yaml
category: architecture | design | code | testing | performance | security | anti-pattern
confidence: 0.0-1.0
times_used: N
times_succeeded: N
```

#### Bug Records
```yaml
severity: critical | high | medium | low
fix_commit: hash
related_patterns: [pattern-keys]
```

#### Agent Records
```yaml
agent_type: chief | specialist | engine
department: department-name
metric_type: performance | preference | learning
```

---

## Memory Operations

| Operation | Description | Trigger |
|-----------|-------------|---------|
| **Store** | Write record to appropriate store | Auto (decisions, bugs, patterns) or manual (agents) |
| **Retrieve** | Read **index/reference** (path, type, tags, summary) of records by key, tags, or time range — NOT bulk content | Session start, context loading |
| **Search** | **Content on-demand** — semantic (meaning-first) or full-text search across stores, pulled only when the task domain requires it | Agent queries, pattern matching |
| **Index** | Rebuild search metadata | After batch writes |
| **Prune** | Archive old/irrelevant records | Periodic (Evolution Engine) |
| **Promote** | Move record from short to long memory | Session end |

---

## Memory Lifecycle

```
CREATE → ACTIVE → ARCHIVE/PRUNE
  ↑                  ↓
  └── PROMOTE ←──────┘ (important memories)
```

| Phase | Action |
|-------|--------|
| Create | Write record with `status: active` |
| Active | Available for retrieval and search |
| Promote | Move from short → long memory at session end |
| Archive | Mark `status: archived`, move to archive subdirectory |
| Prune | Delete records older than retention period |

### Retention Policies
| Memory Type | Retention |
|-------------|-----------|
| Short | Current session only |
| Long | Project lifetime |
| Project | Project lifetime |
| Architecture | Forever (project lifetime) |
| Decision | Forever |
| Pattern | Forever (periodic review) |
| Bug | Forever (periodic review) |
| Agent | Last 12 months rolling window |

---

## Related

- [Memory Engine](engines/memory/SKILL.md) — Memory operations and auto-capture rules
- [Memory Chief](departments/memory/SKILL.md) — Memory management
- [Context Engine](engines/context/SKILL.md) — Context building from memory
- [Learning Engine](engines/learning/SKILL.md) — Agent memory updates
- [Evolution Engine](engines/evolution/SKILL.md) — Pattern and bug memory updates
- [KERNEL.md](KERNEL.md) — Memory loading at session start

---

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Memory Chief | Initial canonical memory taxonomy |

---

> **Enforced by**: Memory Engine | **Last reviewed**: 2026-07-10

---

## Semantic Auto-Evolution Memory (v4.0.0 — 2026-07-27)

### Concept
Each agent maintains a self-evolving semantic memory that grows with experience. Agents progress through 5 capability levels by learning from each task and applying increasingly advanced techniques.

### Architecture
```
.opencode/cosca/memory/agent/{agent-name}/
├── learnings.md    ← Semantic journal (FTS5-indexed, vector-searchable)
├── evolution.md    ← Capability level tracking
├── patterns.md     ← Reusable solution patterns
└── INDEX.md        ← Fast retrieval cross-reference
```

### The Evolution Loop
1. **RETRIEVE**: Agent searches learnings.md for #tags matching current task
2. **APPLY**: Agent uses highest-level technique found (never regress)
3. **EXECUTE**: Agent performs the task with the selected technique
4. **LEARN**: Agent records outcome, technique, level, and improvement direction
5. **EVOLVE**: Over time, agent progresses Level 1→2→3→4→5

### Level Progression
| Level | Name | Trigger | Example (Security Chief) |
|-------|------|---------|--------------------------|
| 1 | Basic | Initialization | OWASP Top 10 checklist |
| 2 | Intermediate | 5 successful L1 tasks | Automated govulncheck + manual review |
| 3 | Advanced | 10 successful L2 tasks | STRIDE threat modeling per subsystem |
| 4 | Expert | 15 successful L3 tasks | Novel attack vector discovery, zero-day patterns |
| 5 | Master | 20 successful L4 tasks | Contributing new OWASP techniques, training other agents |

### Cross-Agent Learning
All learnings are indexed in the knowledge engine (SQLite FTS5 + vector embeddings). The Knowledge Engine indexes .opencode/cosca/memory/agent/ recursively. Agent A's security pattern can be semantically retrieved by Agent B when facing a related task.

### Semantic Search
Before any task, agents execute: `cosca knowledge search "#security #xss"` to find relevant learnings. Results ranked by: level (higher = better), recency (fresher = more relevant), outcome (success > partial > failure).

### Memory Health
- Maximum learnings per agent: unlimited (append-only journal)
- Indexing: automatic via FTS5 on every write
- Cross-reference integrity: validated by Memory Chief weekly
- Expiration: learnings never expire (cumulative knowledge)
