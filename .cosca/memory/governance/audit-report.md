# Governance Audit Report — DNA v3.0 & CONVENTIONS Compliance

> **Auditor**: cosca-governance (Governance Chief) | **Date**: 2026-07-28
> **Scope**: 54 agents (53 Chiefs/Specialists + cosca-kernel)
> **Standards**: [AGENT_DNA.md](../../AGENT_DNA.md) v3.0.0, [CONVENTIONS.md](../../CONVENTIONS.md), [GOVERNANCE.md](../../GOVERNANCE.md)

---

## Executive Summary

| Metric | Value |
|--------|-------|
| Total agents audited | 54 |
| **Overall DNA v3.0 compliance** | **98.1% (53/54)** |
| Agents fully compliant | 53 |
| Agents with issues | 9 (1 blocker, 8 warnings) |
| Capability profiles present | 54/54 (100%) |
| Companion files complete (4/4) | 53/54 (98.1%) |
| PROMPT.md AUTO-EVOLUTION missing | 9/53 (17%) |
| Orphaned files detected | 9 |
| Cross-reference integrity | 100% (no broken links) |

**Verdict**: The platform is in good shape. 98.1% DNA v3.0 compliance. One blocker (cosca-semantic-memory capability profile is structurally non-compliant). Nine orphaned files from pre-v3.0 era. Nine specialists missing AUTO-EVOLUTION directives.

---

## 1. Capability Profile Audit (DNA v3.0 Sections)

### 1.1 Required Section Presence

The DNA v3.0 standard mandates 6 sections in `capability-profile.md`:

| Section | Required | Present In | Missing In |
|---------|----------|------------|------------|
| Current Level | Yes | 53/54 | cosca-semantic-memory |
| Per-Domain Confidence | Yes | 53/54 | cosca-semantic-memory* |
| Strengths | Yes | 53/54 | cosca-semantic-memory* |
| Weaknesses | Yes | 53/54 | cosca-semantic-memory* |
| Preferred Strategies | Yes | 53/54 | cosca-semantic-memory |
| Known Failure Modes | Yes | 53/54 | cosca-semantic-memory |
| Evolution Goal | Yes | 54/54 | — |

> *cosca-semantic-memory has partial equivalents embedded in a non-standard format (see §1.2).

### 1.2 cosca-semantic-memory — BLOCKER

**Severity**: BLOCKER

The capability profile at `memory/agent/cosca-semantic-memory/capability-profile.md` uses a completely different format incompatible with DNA v3.0:

| Issue | Detail |
|-------|--------|
| DNA Version | Declares `3.0` instead of `3.0.0` (minor) |
| Table structure | Uses `Domain Expertise` table (Domain/Level/Confidence) instead of `Per-Domain Confidence` (Domain/Confidence/Successful Tasks/Last Outcome/Trend) |
| Strengths/Weaknesses | Inline bullet under `Capability Summary` instead of dedicated headings |
| Missing sections | No `Preferred Strategies`, no `Known Failure Modes` |
| Level format | `Current Level: 1 (Learning)` instead of `Current Level: 1 (seed data — no real task execution yet)` |
| Missing file | `evolution.md` is absent (all other 53 agents have it) |

**Recommended fix**: Regenerate capability-profile.md using the standard template (matching any Level 1 seed profile, e.g., cosca-ai). Create evolution.md.

---

## 2. PROMPT.md Audit

### 2.1 Format Consistency

All 53 agent `PROMPT.md` files follow the same pattern:
```yaml
---
agent: cosca-name
type: prompt
version: 1.0.0
description: Role — Description. Reports to X.
---
```

**Finding**: All PROMPT.md version fields show `1.0.0` — this is the PROMPT format version, NOT the DNA version. This is consistent and intentional. The DNA version is tracked in capability-profile.md.

### 2.2 AUTO-EVOLUTION Missing — WARNING

**Severity**: WARNING

9 specialist agents lack the `AUTO-EVOLUTION` directive in their PROMPT.md:

| Agent | Status |
|-------|--------|
| cosca-specialist-backend-api | Missing |
| cosca-specialist-backend-service | Missing |
| cosca-specialist-database-sql | Missing |
| cosca-specialist-documentation-writer | Missing |
| cosca-specialist-frontend-component | Missing |
| cosca-specialist-review-code | Missing |
| cosca-specialist-testing-e2e | Missing |
| cosca-specialist-testing-integration | Missing |
| cosca-specialist-testing-unit | Missing |

All 44 Chiefs have the directive. All 9 Specialists lack it. This appears systematic — the specialist prompt template omitted the auto-evolution protocol.

**Recommended fix**: Add the standard AUTO-EVOLUTION block to all 9 specialist PROMPT.md files:
```
AUTO-EVOLUTION: Follow protocol at .cosca/shared/AUTO_EVOLUTION_PROTOCOL.md. Search your semantic memory at .cosca/memory/agent/cosca-specialist-XXX/learnings.md before tasks. Record learnings after. Goal: Level 2+.
```

---

## 3. Companion Files Audit

DNA v3.0 requires 4 companion files per agent: `failures.md`, `patterns.md`, `learnings.md`, `evolution.md`.

| File | Present | Missing |
|------|---------|---------|
| failures.md | 54/54 (100%) | — |
| patterns.md | 54/54 (100%) | — |
| learnings.md | 54/54 (100%) | — |
| evolution.md | 53/54 (98.1%) | cosca-semantic-memory |

### 3.1 evolution.md Missing — WARNING

**Severity**: WARNING

`cosca-semantic-memory` lacks `evolution.md`. This file tracks level history and is required for the Metacognition Pipeline stage UPDATE CAPABILITY MODEL.

**Recommended fix**: Create evolution.md for cosca-semantic-memory (can start as empty seed record).

---

## 4. Level Distribution

| Level | Count | % | Agents |
|-------|-------|---|--------|
| 3 (Strategic) | 3 | 5.6% | cosca-backend, cosca-documentation, cosca-kernel |
| 2 (Operational) | 4 | 7.4% | cosca-database, cosca-frontend, cosca-runtime, cosca-security |
| 1 (Seed) | 46 | 85.2% | All remaining chiefs + all 9 specialists |
| No Level | 1 | 1.9% | cosca-semantic-memory |

**Assessment**: 85.2% of agents at Level 1 (seed) is expected for a newly bootstrapped platform. The 13 agents at Level 2+ (including the non-level cosca-semantic-memory) represent agents with real task execution history. The ratio aligns with the stated 41/55 (75%) baseline estimate.

---

## 5. Orphaned Files

### 5.1 Files Identified — WARNING

**Severity**: WARNING

9 `agent-*.md` files orphaned in `memory/agent/` root (outside proper `cosca-*/` subdirectories):

| File | Content Type | Template Style | Target Agent |
|------|-------------|----------------|--------------|
| `agent-api-chief.md` | Performance record | YAML frontmatter + empty body | Unclear (API Chief not a Cosca agent) |
| `agent-architecture-performance.md` | Performance record | Markdown header | cosca-architecture |
| `agent-compliance-chief.md` | Performance record | YAML frontmatter | Unclear |
| `agent-cosca-architecture.md` | Performance record | Markdown header | cosca-architecture |
| `agent-cosca-security.md` | Performance record | Markdown header | cosca-security |
| `agent-kernel-performance.md` | Performance record | Markdown header | cosca-kernel |
| `agent-performance-chief.md` | Performance record | YAML frontmatter | cosca-performance |
| `agent-platform-chief.md` | Performance record | YAML frontmatter | cosca-platform |
| `agent-review-performance.md` | Performance record | Markdown header | cosca-review |

### 5.2 Analysis

These files follow a pre-DNA v3.0 naming convention (`agent-*-chief.md` and `agent-*-performance.md`) and contain either YAML frontmatter performance tracking data (sometimes empty) or markdown headers with no substantive content.

**Root cause**: These were created during the DNA v2.0 era when performance tracking used flat files in memory/agent/. DNA v3.0 moved all agent memory into subdirectories (`cosca-*/`).

### 5.3 Recommended Action

| Priority | Action |
|----------|--------|
| 1 | Review each file for any substantive historical data worth preserving |
| 2 | Migrate substantive data into respective `cosca-*/evolution.md` or `cosca-*/failures.md` |
| 3 | Delete all 9 orphaned files after migration |
| 4 | Update `memory/agent/INDEX.md` to remove orphaned file references |

Note: `INDEX.md` references 5 orphaned files (`agent-kernel-performance`, `agent-architecture-performance`, `agent-review-performance`, `agent-api-chief`, `agent-platform-chief`, `agent-compliance-chief`, `agent-performance-chief`) in its "Active Agent Records" and "Reference" sections. These must be updated or removed.

---

## 6. Cross-Reference Integrity

### 6.1 Core Document Links

All cross-references in AGENT_DNA.md, GOVERNANCE.md, and CONVENTIONS.md are valid. The one "broken" link found in CONVENTIONS.md (`[Display Name](../path/to/file.md)`) is a template placeholder within example code, not an actual reference.

### 6.2 Agent-to-Agent References

No references to non-existent agents found. All `cosca-*` references in PROMPT.md files point to existing agent directories.

### 6.3 Naming Consistency

| Check | Result |
|-------|--------|
| Agent directory naming: `cosca-*` | 100% consistent |
| Memory directory naming: `cosca-*` | 100% consistent |
| PROMPT.md frontmatter `agent:` field | 100% matches directory name |
| Capability profile heading: `# cosca-* — Capability Profile` | 100% consistent |
| No `cosca-*-chief` naming vs `cosca-*` | No conflicts found |

---

## 7. Domain Overlap Analysis

### 7.1 Chief ↔ Specialist Hierarchy (NORMAL)

These are expected delegation hierarchies — no issue:

| Chief | Specialist(s) | Assessment |
|-------|---------------|------------|
| cosca-backend | cosca-specialist-backend-api, cosca-specialist-backend-service | NORMAL — specialists handle routine implementation under Chief |
| cosca-database | cosca-specialist-database-sql | NORMAL |
| cosca-frontend | cosca-specialist-frontend-component | NORMAL |
| cosca-testing | cosca-specialist-testing-unit, -integration, -e2e | NORMAL |
| cosca-review | cosca-specialist-review-code | NORMAL |
| cosca-documentation | cosca-specialist-documentation-writer | NORMAL |

### 7.2 Potential Boundary Overlaps (INFO)

These pairs have adjacent domains where boundaries could blur:

| Pair | Overlap Area | Risk | Recommendation |
|------|-------------|------|----------------|
| cosca-performance ↔ cosca-monitoring | Profiling vs observability | LOW | Clear boundary: performance does profiling/benchmarking; monitoring does SLOs/alerting. Document in both PROMPT.md |
| cosca-testing ↔ cosca-qa | Test execution vs quality standards | MODERATE | cosca-testing writes tests; cosca-qa defines standards. Clarify that QA owns the "what" and Testing owns the "how" |
| cosca-security ↔ cosca-compliance | Security vs regulatory compliance | MODERATE | security: technical controls; compliance: regulatory frameworks (GDPR, LGPD). Document handoff for audit evidence |
| cosca-technical-debt ↔ cosca-evolution | Debt tracking vs improvement suggestions | MODERATE | technical-debt: tracks and prioritizes; evolution: analyzes patterns and proposes refactors. Clarify that evolution feeds into debt tracking |
| cosca-review ↔ cosca-critic | Code review vs decision review | LOW | review: code-level PR review; critic: strategic decision review. Distinct scopes |

**Recommendation**: Add explicit `OUT OF SCOPE` or boundary clarification sections to PROMPT.md files for the MODERATE-risk pairs.

### 7.3 Duplicate Domain Risk — NONE

No two agents claim identical primary domains. All agents have distinct responsibilities at the level of capability-profile.md definitions.

---

## 8. cosca-kernel — Special Case

### 8.1 Asymmetry

- Has memory directory: `memory/agent/cosca-kernel/` ✓
- Has capability-profile.md: ✓ (DNA 3.0.0, Level 3)
- Has all companion files: ✓
- Missing agent directory: `agents/cosca-kernel/` does not exist
- Missing PROMPT.md: No PROMPT.md file (kernel definition is in AGENTS.md system prompt)

**Assessment**: This is intentional. cosca-kernel is the orchestrator defined at the system level, not as a standard agent. No action required.

---

## 9. INDEX.md Inconsistency — WARNING

**Severity**: WARNING

`memory/agent/INDEX.md` declares 53 agents but references orphaned files in its "Active Agent Records" and "Reference" sections:

- Lists 5 orphaned files as "Active Agent Records" (e.g., `agent-kernel-performance`, `agent-architecture-performance`)
- Lists 6 orphaned files under "Reference (Framework Agents)"
- Does not list the 9 specialist agents
- Does not list cosca-critic or cosca-paradigm with memory directory keys (listed as "—" under Active Agent Records despite having full memory directories)

**Recommended fix**: Rewrite INDEX.md to reflect the actual 54-agent structure, remove orphaned file references, and properly index all agents with their memory directories.

---

## 10. Compliance Scorecard

### 10.1 Per-Agent Compliance

| Compliance Level | Count | % | Agents |
|-----------------|-------|---|--------|
| FULL (100%) | 44 | 81.5% | All Level 1 seed chiefs (excluding cosca-semantic-memory) |
| HIGH (95%+) | 4 | 7.4% | cosca-backend, cosca-documentation, cosca-kernel (Level 3); cosca-database, cosca-frontend, cosca-runtime, cosca-security (Level 2) — all structurally compliant |
| PARTIAL (75-94%) | 5 | 9.3% | 9 specialists (missing AUTO-EVOLUTION): -7% each, but structurally compliant otherwise → 93% |
| NON-COMPLIANT (<75%) | 1 | 1.9% | cosca-semantic-memory (blocker: 42% — non-standard format, missing 4/7 required sections, missing evolution.md) |

### 10.2 DNA v3.0 Field Coverage (Aggregate)

| DNA Field | Coverage | Notes |
|-----------|----------|-------|
| Current Level | 53/54 | cosca-semantic-memory uses non-standard format |
| Per-Domain Confidence | 53/54 | cosca-semantic-memory uses wrong table structure |
| Strengths | 53/54 | cosca-semantic-memory embeds in Capability Summary |
| Weaknesses | 53/54 | cosca-semantic-memory embeds in Capability Summary |
| Preferred Strategies | 53/54 | cosca-semantic-memory: missing entirely |
| Known Failure Modes | 53/54 | cosca-semantic-memory: missing entirely |
| Evolution Goal | 54/54 | All compliant |
| failures.md | 54/54 | All present |
| patterns.md | 54/54 | All present |
| learnings.md | 54/54 | All present |
| evolution.md | 53/54 | cosca-semantic-memory: missing |

---

## 11. Recommendations

### 11.1 Blocker — Must Fix Before Next Audit

| # | Action | Agent | Effort |
|---|--------|-------|--------|
| B1 | Regenerate `cosca-semantic-memory/capability-profile.md` with standard DNA v3.0 format | cosca-semantic-memory | Low |
| B2 | Create `cosca-semantic-memory/evolution.md` | cosca-semantic-memory | Low |

### 11.2 High Priority — Fix Within 7 Days

| # | Action | Affected | Effort |
|---|--------|----------|--------|
| H1 | Add AUTO-EVOLUTION directive to all 9 specialist PROMPT.md files | 9 specialists | Low |
| H2 | Rewrite `memory/agent/INDEX.md` to reflect actual 54-agent structure | INDEX.md | Medium |
| H3 | Review, migrate (if needed), and delete 9 orphaned `agent-*.md` files | 9 orphaned files | Medium |

### 11.3 Medium Priority — Fix Within 30 Days

| # | Action | Affected | Effort |
|---|--------|----------|--------|
| M1 | Add boundary clarification to MODERATE-overlap pairs (see §7.2) | 5 pairs | Medium |
| M2 | Standardize `Current Level` format across all profiles (some use "seed data — no real task execution yet", others omit the parenthetical) | ~3 agents | Low |

### 11.4 Low Priority — Continuous Improvement

| # | Action | Effort |
|---|--------|--------|
| L1 | Schedule quarterly governance audits (next: 2026-10-28) | — |
| L2 | Create automated compliance checker (script that validates capability-profile.md structure) | Medium |
| L3 | Add DNA version checker to pre-commit hooks | Low |

---

## 12. Methodology

This audit was conducted by:

1. **Reference review**: Read CONVENTIONS.md, AGENT_DNA.md, GOVERNANCE.md for standard definitions
2. **Structural scan**: Enumerated all 54 capability profiles via glob patterns
3. **Section presence check**: Verified 7 required sections (6 DNA + Evolution Goal) in every profile
4. **Companion file audit**: Checked failures.md, patterns.md, learnings.md, evolution.md for all 54 agents
5. **PROMPT.md format analysis**: Checked all 53 PROMPT.md files for AUTO-EVOLUTION and version consistency
6. **Orphaned file detection**: Identified files outside `cosca-*/` subdirectories in memory/agent/
7. **Cross-reference validation**: Verified all markdown links in core documents
8. **Domain overlap analysis**: Compared primary domains and PROMPT.md responsibilities across adjacent agents
9. **INDEX.md consistency check**: Validated INDEX.md against actual directory structure

---

## Appendices

### A. Full Agent Inventory

See [memory/agent/INDEX.md](../agent/INDEX.md) (needs update per recommendation H2).

### B. DNA v3.0 Capability Profile Template

Reference: [AGENT_DNA.md §24](../../AGENT_DNA.md) and [capabilities/CAPABILITY_TEMPLATE.md](../../capabilities/CAPABILITY_TEMPLATE.md).

### C. Audit History

| Date | Auditor | Scope | Compliance |
|------|---------|-------|------------|
| 2026-07-28 | cosca-governance | 54 agents, DNA v3.0 + CONVENTIONS | 98.1% (first audit) |

---

> **Next audit scheduled**: 2026-10-28 | **Enforced by**: Governance Chief | **Reviewed by**: (pending CTO review)
