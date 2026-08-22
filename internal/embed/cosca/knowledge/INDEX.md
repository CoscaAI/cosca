# Cosca Knowledge Repository

> **Category**: Root Index | **Version**: 2.0.0 | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-29

## Purpose

The Cosca Knowledge Repository is the structured, curated, versioned source of truth for all Cosca platform knowledge. It feeds the Knowledge Compiler pipeline to populate the SQLite knowledge database and vector embeddings used at runtime by all agents.

## Pipeline

```
┌─────────┐    ┌────────────────────┐    ┌───────────────────┐    ┌──────────────────────┐
│ GitHub  │───▶│ Knowledge          │───▶│ Knowledge          │───▶│ SQLite DB +          │
│  Repo   │    │ Repository (here)  │    │ Compiler           │    │ Vector Embeddings    │
└─────────┘    └────────────────────┘    └───────────────────┘    └──────────────────────┘
     │                  │                          │                         │
     ▼                  ▼                          ▼                         ▼
  Source of        Structured                Transforms                  Loaded at
  truth for       markdown/yaml              markdown/yaml               runtime by
  all content     with frontmatter           into DB rows +              all agents
                                             vector chunks
```

## Categories

| # | Category | Directory | Pipeline Role | Description |
|---|----------|-----------|---------------|-------------|
| 1 | **Patterns** | [`patterns/`](patterns/INDEX.md) | Reference architecture | Reusable solution templates: architecture, design, Go-specific |
| 2 | **Heuristics** | [`heuristics/`](heuristics/INDEX.md) | Decision rules | 20 extracted heuristics from agent learnings (5 patterns, 15 anti-patterns) |
| 3 | **Architecture** | [`architecture/`](architecture/INDEX.md) | System design source of truth | System architecture docs, ADRs, plans, audits |
| 4 | **Failures** | [`failures/`](failures/INDEX.md) | Negative memory | Bugs with causality trees, incidents, audits, reviews, risk registry |
| 5 | **Best Practices** | [`best-practices/`](best-practices/INDEX.md) | Operational guidance | Playbooks, runbooks, compliance assessments, benchmarks |
| 6 | **Cognitive** | [`cognitive/`](cognitive/INDEX.md) | Agent self-model | UCSS spec, cognitive state, meta-learning framework |

## Statistics

| Category | Files | Key Contents |
|----------|------:|--------------|
| Patterns | 8 | 3 architecture patterns, 2 design patterns, 3 Go-specific patterns |
| Heuristics | 20 | 5 patterns + 15 anti-patterns, confidence 0.85–0.95 |
| Architecture | 23 | 3 system docs, 3 plans, 2 audits, 15 ADRs |
| Failures | 13+ | 8 bugs + template, 2 reviews, 1 audit, 1 risk registry (14 risks) |
| Best Practices | 7+ | 6 playbooks, 1 compliance assessment, 3 benchmarks |
| Cognitive | 2 | UCSS spec + cognitive state snapshot |
| **Total** | **73+** | |

## File Format Standards

| Category | Primary Format | Frontmatter Required | Key Fields |
|----------|---------------|---------------------|------------|
| Patterns | Markdown | No (title block) | Intent, Context, Solution, Consequences |
| Heuristics | YAML (`H-NNN.yaml`) | Yes | id, title, domain, type, severity, confidence |
| Architecture | Markdown | Varies | N/A |
| Failures — Bugs | Markdown | Yes (YAML) | type, key, severity, fixed_in, causality tree N1–N4 |
| Failures — Risks | Markdown | No (table-driven) | Risk #, Probability, Impact, Mitigation |
| Best Practices — Playbooks | Markdown | No (title block) | Steps, Prerequisites, Duration |
| Best Practices — Benchmarks | Markdown | No (title block) | Metric, Value, Source |
| Cognitive | YAML (UCSS) | No | identity, emotion, energy, confidence |

## Memory System vs Knowledge Repository

| Dimension | Memory System (`memory/`) | Knowledge Repository (`knowledge/`) |
|-----------|--------------------------|-------------------------------------|
| Audience | Agents (internal) | Humans + Compiler |
| Format | Agent learnings, session context | Structured patterns, ADRs, playbooks |
| Update | Automatic (agents write) | Manual + review (curated) |
| Versioning | Implicit (timestamp) | Explicit (semver) |
| Search | File-based + semantic | SQLite FTS5 + vector |
| Decay | Yes (180d/90d/60d half-life) | No (immutable, versioned) |
| Location | `internal/embed/cosca/memory/` | `internal/embed/cosca/knowledge/` |

## Notes

- **`schema/`** at `knowledge/schema/` is an implementation detail for the SQLite database migrations. It defines the physical schema for `cosca.db` and is not part of the knowledge content taxonomy. See [`schema/INDEX.md`](schema/INDEX.md) for migration instructions.

---

*This INDEX.md replaces the legacy v1.0.0 index. Legacy directories (`incidents/`, `reference-architectures/`, `runbooks/`, `benchmarks/`, `playbooks/`) should be removed after content migration is complete.*
