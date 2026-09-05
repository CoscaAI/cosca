# cosca-semantic-memory — Discovered Patterns

> Recurring patterns identified during agent operation. Used to optimize future decisions.

## Patterns

### Pattern: Cross-Source Architecture Synthesis for Engine Specifications

| Field | Value |
|-------|-------|
| **Pattern ID** | SEM-001 |
| **Discovered** | 2026-07-30 (during Knowledge Federation Engine spec) |
| **Context** | Designing a new Cosca engine (engine specification) that integrates multiple existing systems |
| **Problem** | How to design a coherent engine specification that respects all existing architecture, reuses existing patterns, and defines clear interfaces with 5+ dependent engines |
| **Solution** | 1) Read all reference documents (architecture, workflow, protocols, related engines) before writing. 2) Identify integration points FIRST — how the new engine touches existing systems. 3) Define the signature/format that bridges systems (e.g., Knowledge Signature YAML as the bridge between projects). 4) Use the existing SKILL_TEMPLATE.md as structural foundation. 5) Validate every section against acceptance criteria from the originating workflow task. 6) Write examples that demonstrate the cross-system value. 7) Define constraints (what NOT to do) explicitly. |
| **Applied** | 2 times (Semantic Index spec + Knowledge Federation spec) |
| **Confidence** | 0.92 |
| **Related** | Cognitive Maturity architecture, AUTO_EVOLUTION_PROTOCOL.md, SKILL_TEMPLATE.md |

### Pattern: Signature-Based Cross-Domain Knowledge Matching

| Field | Value |
|-------|-------|
| **Pattern ID** | SEM-002 |
| **Discovered** | 2026-07-30 (during Knowledge Federation Engine spec) |
| **Context** | Knowledge from one domain (e.g., Go CLI refactoring) needs to be recognized as applicable in a completely different domain (e.g., React component refactoring) |
| **Problem** | Direct pattern matching fails across domains because terminology, stack, and implementation details differ. A "function extraction" in Go looks nothing like a "component decomposition" in React syntactically. |
| **Solution** | Abstract knowledge into signatures with domain-independent preconditions: (1) Classify by pattern_type (e.g., `extract_then_test`) rather than implementation. (2) Describe preconditions structurally (e.g., "unit > 100 lines", "coverage < 20%") rather than linguistically. (3) Use applicability_score to indicate how universal the pattern is (domain_coupling = 0.15 means highly transferable). (4) Match on signature embedding similarity (cosine) rather than keyword. (5) Adapt the implementation to the target stack while preserving the pattern structure. |
| **Applied** | Designed, not yet operationally validated |
| **Confidence** | 0.85 |
| **Related** | Knowledge Federation Engine §4.1 (Pattern Matching), Knowledge Signature §5.1, COGNITIVE_MATURITY.md §A4

### Pattern: Dimension-First Reindex Audit

| Field | Value |
|-------|-------|
| **Pattern ID** | SEM-003 |
| **Discovered** | 2026-08-04 |
| **Context** | Planejar reindexação sem alterar nem misturar um índice vetorial existente |
| **Problem** | A dimensão do SQLite é inferida em runtime; provider fallback, modelo remoto e dados antigos podem divergir silenciosamente |
| **Solution** | Rastrear CLI/registro/seleção até `Provider.Dimensions()`, comparar com `SQLiteVecConfig.Dimension`, confirmar tamanho dos BLOBs no banco read-only, congelar provider/modelo/dimensão explicitamente, e reindexar em snapshot/DB isolado antes do cutover. Nunca tratar padding/truncation como equivalência semântica de modelos diferentes. |
| **Applied** | 1 vez |
| **Confidence** | 0.95 |
| **Known Pitfalls** | `IndexDocument` gera UUID novo; repetir o loop duplica documentos. `RebuildAll` reconstrói vectors/FTS, mas requer validação das tabelas relacionais. Ollama é instanciado sem health-check do endpoint. |
