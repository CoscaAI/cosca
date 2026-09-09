# Evolution Activation Report — cosca-evolution

> **Date**: 2026-07-28 | **Agent**: cosca-evolution | **Level**: 2 | **Status**: ATIVADO

---

## 1. Codebase Scan — Maturity by Package

### Mature (interfaces + tests + reasonable size)
| Package | Files | Tests | Interfaces | Notes |
|---------|-------|-------|------------|-------|
| `orchestration/` | 14 | 8 | 11 | Best-architected package; ports pattern, clear boundaries |
| `runtime/` | 5 | 16 | 1 | Well-tested; state machine, lifecycle management |
| `chat/` | 3 | 2 | 3 | Provider abstraction, stream interface |
| `context/` | 4 | 2 | 3 | Searcher, Retriever, MemoryStore interfaces |
| `memory/` | 5 | 10 | 1 | Store interface, thorough tests |
| `plugins/` | 8 | 4 | 4 | Plugin, RuntimeAPI, Logger interfaces |

### Immature (no tests, no interfaces, or both missing)
| Package | Files | Tests | Interfaces | LOC | Risk |
|---------|-------|-------|------------|-----|------|
| `api/rest/handler/` | 19 | 0 | 0 | ~4,000 | 🔴 User-facing, 0% coverage |
| `cli/` | 52 | 10 | 0 | 19,550 | 🔴 Largest monolith, 13.7% coverage |
| `cache/` | 1 | 1 | 0 | 583 | 🟡 Multi-level cache, no mockable contract |
| `knowledge/` | 1 | 2 | 0 | 1,168 | 🟡 1 file, 0 interfaces, heavy coupling |
| `embeddings/` | 2 | 0 | 1 | 694 | 🟡 Critical path, zero tests |
| `diagnostics/` | 3 | 0 | 0 | 1,439 | 🟡 No tests despite being diagnostic tooling |

### Key Statistic
**63% of packages (27/43) define zero interfaces**. This is the dominant structural pattern.

---

## 2. Evolution Patterns Detected

### Pattern A: Missing Interface Contracts (High Severity)
27 of 43 top-level packages define no interfaces. Key examples:
- `internal/cache/` — `*Cache` is a concrete struct; no `Cache` interface exists
- `internal/knowledge/` — Direct concrete type; no `KnowledgeBase` interface
- `internal/config/` — 899-line config.go, 0 interfaces
- `internal/sqlite/` — 4 files, 0 interfaces, tight coupling to SQLite

**Effect**: Mocking requires real implementations. Integration tests are the only viable strategy. Unit testability severely limited.

### Pattern B: God Files (Medium Severity)
Files exceeding 500 lines that should likely be split:
| File | Lines | Issue |
|------|-------|-------|
| `internal/knowledge/knowledge.go` | 1,168 | Single file, 0 interfaces |
| `internal/plugins/loader.go` | 1,149 | Plugin loading + lifecycle in one file |
| `internal/config/config.go` | 899 | Config + parsing + validation |
| `internal/orchestration/chain.go` | 842 | Chain executor and all step logic |
| `internal/orchestration/pipeline.go` | 819 | Pipeline, builtin processors, service layer |
| `internal/cli/doctor.go` | 635 | Diagnostic CLI command |

### Pattern C: Concrete Handler Coupling (High Severity)
All 19 REST handlers in `api/rest/handler/` take concrete types (e.g., `*agents.Manager`, `*cache.Cache`) in constructors. No handler defines an interface for its dependencies. This makes unit testing impossible without full integration setup.

### Pattern D: Large Monolith (CLI) (High Severity)
`internal/cli/` has 52 files, 19,550 LOC, 0 interfaces, and only 13.7% test coverage. It has the highest fan-out of any package — importing nearly every internal service. Refactoring risk is extreme.

### Positive Pattern: Orchestration's Ports & Adapters
`internal/orchestration/` defines 8 interface types across 14 files, uses clear ports (KnowledgeSearcher, MemoryRetriever, MemoryStorer, AgentResolver, SkillResolver, Embedder). This is the reference architecture for the rest of the codebase.

### Positive Pattern: Zero TODO/FIXME Density
Only 1 file in the entire codebase (a test file) contains TODO markers. This indicates disciplined code hygiene.

---

## 3. Debt Cross-Reference Validation

### Finding 1: COV-01 — api/rest/handler/ at 0% coverage ✅ CONFIRMED (Worse)
- Scorecard claims: "11 handlers at 0%" 
- Actual: **19 handler files**, still at **0% test coverage**
- Scorecard count is stale by ~8 files; severity is actually higher than reported

### Finding 2: COV-02 — api/middleware/ at 0% coverage ❓ PARTIALLY INACCURATE
- Scorecard claims: "CORS, CSRF, logging, rate limit at 0%"
- Actual: `cors.go` + `cors_test.go`, `csrf.go` + `csrf_test.go`, `security.go` + `security_test.go` exist
- However: `logging.go`, `metrics.go`, `ratelimit.go` have no tests
- Scorecard should reflect partial (3/6 files have tests), not "0%"

### Finding 3: BUG-U01 — Restart() broken ❓ POTENTIALLY STALE
- Scorecard claims: "Restart() broken — Stopped→Uninitialized transition defined but never executed"
- Actual code: `Restart()` (runtime.go:411-440) calls `TransitionTo(StateUninitialized, "restart")` explicitly at line 424, then calls `Start()` at line 434
- `validTransitions` (state.go:100-107) includes `StateStopped: {StateUninitialized}` at line 106
- Either the bug was fixed between audit and now, or the scorecard describes a different code path (daemon's `checkAndRestart()` at daemon.go:335 works at subsystem level, not Runtime level)
- **Recommendation**: Re-audit this finding; update scorecard

### Finding 4: RSK-05 — gRPC server incomplete ✅ CONFIRMED
- Scorecard claims: "Proto defined but handler pending"
- Actual: `api/grpc/pb/` has 6 generated proto files (knowledge, memory, runtime with gRPC stubs)
- `api/grpcserver/` exists but handler implementation is pending
- Confirmed: gRPC surface is defined but not wired

---

## 4. Improvement Candidates — Top 3 Refactoring Areas

### Candidate A: API Handler Interfaces + Tests (Highest ROI)
- **What**: Extract interfaces from REST handler dependencies; add handler tests
- **Files**: 19 files in `api/rest/handler/`, ~4,000 LOC
- **Impact**: User-facing API currently has zero test coverage. Every regression is customer-visible.
- **Effort**: Medium (16-24h for interface extraction + top 5 handlers)
- **Scorecard alignment**: COV-01, aligns with Priority #9
- **Approach**: Define `AgentService`, `CacheService`, `KnowledgeService` interfaces; inject into handlers; write table-driven tests for top 5 handlers (run, auth, agents, knowledge, memory)

### Candidate B: Cache Interface Extraction
- **What**: Define `Cache` interface; split 583-line cache.go into leveled modules (memory/sqlite/filesystem)
- **Files**: `internal/cache/cache.go`
- **Impact**: Cache is used by nearly every subsystem. A mockable contract unlocks unit tests across the stack.
- **Effort**: Low (4-6h)
- **Scorecard alignment**: Partially addresses COV-11 (cache coverage 45.4%); improves testability of api/rest/handler/

### Candidate C: CLI Modularization (Long-term)
- **What**: Extract backend services behind interfaces; split `cli/` into sub-packages by command domain
- **Files**: 52 files in `internal/cli/`, 19,550 LOC
- **Impact**: Highest fan-out package; testing at 13.7%; every new command adds to the monolith
- **Effort**: High (40-60h, should be phased)
- **Scorecard alignment**: COV-07 (cli coverage 13.7%)
- **Approach**: Phase 1 — Extract 3 most-coupled service types behind interfaces. Phase 2 — Split doctor.go (635 lines) and serve.go (582 lines) into sub-packages. Phase 3 — Add command-level tests incrementally.

---

## 5. Recommendations for Codebase Evolution

### Recommendation 1: Establish Interface-First Policy
**Problem**: 63% of packages have zero interfaces. Concrete coupling prevents unit testing and creates hidden dependency chains.

**Action**: Adopt a convention that every package that provides a service (cache, knowledge, config, etc.) must export at least one interface. Add this to `.cosca/shared/CODING_STANDARDS.md` or equivalent.

**Expected Impact**: Unlocks unit testing across 27 packages currently without interfaces. Reduces coupling. Enables mock-based testing in `api/rest/handler/`.

**Timeline**: 3-month rollout. Prioritize cache, knowledge, config, sqlite as first wave.

### Recommendation 2: Reconcile Scorecard with Ground Truth
**Problem**: Scorecard has stale data (handler count 11→19, middleware coverage 0%→partial, BUG-U01 may be fixed).

**Action**: Before next monthly review (2026-08-28), run a fresh automated scan to validate all scorecard items. Automate the data pipeline — manual counts drift within days.

**Expected Impact**: Eliminates decision error from stale data. Prevents wasted effort on already-fixed bugs.

**Timeline**: 1 week to add automated validation to the scorecard generation process.

### Recommendation 3: Split Top 5 God Files
**Problem**: 5 files exceed 800 lines (knowledge.go, loader.go, config.go, chain.go, pipeline.go). Single-file packages create cognitive load and merge conflicts.

**Action**: Split each file by concern:
- `knowledge/knowledge.go` (1,168 lines) → `search.go`, `store.go`, `query.go`
- `plugins/loader.go` (1,149 lines) → `loader.go`, `compiler.go`, `validator.go`
- `config/config.go` (899 lines) → `config.go`, `defaults.go`, `validation.go`
- `orchestration/chain.go` (842 lines) → `chain.go`, `step.go`, `synthesis.go`
- `orchestration/pipeline.go` (819 lines) → `pipeline.go`, `builtins.go`, `service.go`

**Expected Impact**: Improved navigability, reduced merge conflicts, clearer separation of concerns. Enables targeted testing of sub-components.

**Timeline**: 2-4 hours per file. Priority: knowledge.go, config.go, then the orchestration files.

---

## 6. Trend Indicators

| Metric | Current | Direction | Notes |
|--------|---------|-----------|-------|
| Packages with interfaces | 16/43 (37%) | → Stable | No new interfaces being added |
| God files (>500 lines) | 12 | ↑ Growing | New features add lines to existing files |
| Handler test coverage | 0/19 (0%) | → Static | No handler tests being written |
| TODO/FIXME density | Near zero | ✅ Excellent | Disciplined code hygiene |
| CLI size | 19,550 LOC | ↑ Growing | New commands added regularly |
| Agent ecosystem activation | 16/54 (30%) | ↑ Improving | Onda 6 activating 8 more agents |
| Composite debt score | 1,375/3,200 (43%) | ⬆️ Rising | Per scorecard, +7.6% projected monthly |

---

## 7. Scorecard Health Assessment

| Dimension | Grade | Rationale |
|-----------|-------|-----------|
| Completeness | B+ | Covers 8 categories, 75 items, includes structural multiplier |
| Accuracy | B- | Handler count stale (11→19), middleware coverage partially wrong, BUG-U01 may be stale |
| Actionability | A | Clear ROI rankings, effort estimates, ownership assignments |
| Timeliness | B | Baseline dated same-day but some source data pre-dates by days |

**Recommendation**: Add automated data extraction to prevent scorecard rot between monthly reviews.

---

## 8. Related Documents

| Document | Path |
|----------|------|
| Technical Debt Scorecard | `.cosca/memory/technical-debt/scorecard.md` |
| Evolution Learnings | `.cosca/memory/agent/cosca-evolution/learnings.md` |
| Evolution Timeline | `.cosca/memory/agent/cosca-evolution/evolution.md` |
| Coverage Report | `.cosca/memory/testing/coverage.md` |
| Bug Registry | `.cosca/memory/bug/INDEX.md` |
| Codebase Overview | `.cosca/memory/codebase/overview.md` |

---

*Report generated by cosca-evolution (Evolution Agent). Level 2 analysis complete. Next activation recommended after 3 new interface additions or 30-day interval.*
