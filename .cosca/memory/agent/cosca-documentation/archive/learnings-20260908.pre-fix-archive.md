# cosca-documentation - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-documentation — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Session: 2026-07-28 — Documentation Sync (Fase 1+2)

### 2026-07-28 — Full Documentation Audit + Sync
| Field | Value |
|-------|-------|
| **Agent** | cosca-documentation |
| **Task** | Audit all 887 documentation assets and sync with codebase reality |
| **Technique** | Level 3 — Multi-source doc audit: cross-referenced README claims (52 agents, 34 CLI commands, 336 Go files) against ground-truth codebase scan (51 agents, 39 CLI commands, 357 Go files). Compared docs/ structure against actual internal/ packages. Flagged 3 critically stale files (PostgreSQL fantasy, Go SDK fiction, compliance fabrication). Verified memory health (309 files, zero broken links, 95% YAML frontmatter coverage). |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #documentation #audit #readme #sync #versioning #memory |
| **Related** | README.md, docs/README.md, CHANGELOG.md, .cosca/memory/ |
| **Learned** | Documentation drift pattern: README badges and stats table become stale without automated verification. Key corrections made: agent count 52→51, CLI commands 34→39, Go files 336→357, workflows 25→26. Version discrepancy: docs/README.md said v1.3.0 but CHANGELOG was at v1.4.0-dev — no single source of version truth. docs/sdk/go.md described a Go client library that doesn't exist (only pkg/cosca/ types + REST API). MEMORY_MODEL.md was 44 lines out of sync between .opencode/ and internal/embed/ (missing Auto-Evolution v4.0 section). Pattern: always audit badges against actual counts. Establish single version source. Verify SDK docs against actual go.mod imports. |
| **Next** | Level 4: Create automated doc-health CI check that validates README numbers, version consistency, and SDK doc accuracy against codebase |
### 2026-07-28 — New Documentation Creation

| Field | Value |
|-------|-------|
| **Agent** | cosca-documentation |
| **Task** | Fill documentation gaps: frontend architecture, auth reference, middleware reference, runtime update |
| **Technique** | Level 2 — Topical documentation from codebase analysis: extracted architecture from 240+ .tsx files, 27 feature modules, 14 UI primitives. Documented auth from JWT source (custom HMAC-SHA256), bcrypt cost, API key format. Mapped middleware chain from server.go buildHandler(). Updated runtime docs with real metrics (8 counters, 4 histograms), daemon watchdog, known limitations. |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #documentation #frontend #auth #middleware #runtime #architecture |
| **Related** | docs/frontend/architecture.md, docs/api-reference/auth.md, docs/api-reference/middleware.md, docs/runtime/overview.md |
| **Learned** | Effective pattern: delegate codebase scanning to specialized agents (Discovery, Memory Chief), aggregate findings, write docs from structured data. Docs follow consistent header format (Status, Owner, Last Updated, Version). Cross-reference links essential for navigability. Runtime docs revealed discrepancies: Restart() broken (requires Uninitialized but Stop() leaves Stopped), EventStartupComplete fires too early (before init hooks), hot reload documented but not implemented. Document limitations honestly — don't paper over gaps. |
| **Next** | Level 3: Create documentation contribution guide, automated link checker, expand examples/ directory with real-world usage |

### 2026-07-28 — Master Index Update for Semantic Memory Infrastructure

| Field | Value |
|-------|-------|
| **Agent** | cosca-documentation |
| **Task** | Update COSCA_INDEX.md to reflect new Semantic Memory department and engine additions |
| **Technique** | Level 1 — Multi-section index update: updated 6 distinct locations in a single file (file count header, CONSTITUTION description, department count + table row, engine count + table row, new SHARED REFERENCES section). Used sequential top-down edits to avoid line-shift conflicts. Verified all changes via targeted reads of each affected section. Adapted 4-column user template to 3-column table convention (dropped "depends on" column for engine entry) for format consistency. |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #documentation #index #semantic-memory #cosca-index #master-index |
| **Related** | .cosca/COSCA_INDEX.md, .cosca/departments/semantic-memory/, .cosca/engines/semantic-memory/ |
| **Learned** | COSCA_INDEX.md serves as the single source of truth for ecosystem enumeration — every new department/engine must be registered here. The DEPARTMENTS section doubles as the agent registry (no separate AGENTS table). Edit sequencing matters: always top-down to avoid cascading line shifts. When user-provided row format differs from existing table columns, adapt for consistency rather than blindly copying — document the adaptation. The file ended at 390 lines, gained exactly 10 (department +1, engine +1, separator +2, shared +4, blank +1). Key insight: this file tracks aggregate counts that must stay in sync with actual content — future automation should validate count headers against table row counts. |
| **Next** | Level 2: Build automated index validator that checks count headers ("DEPARTMENTS (N)") against actual table row counts, detects stale entries, and flags missing cross-references |

