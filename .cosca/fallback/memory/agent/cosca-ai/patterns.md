# cosca-ai — Reusable Patterns

> Discovered patterns that can be reapplied.

## Patterns Discovered

### 2026-07-28 — Full-Stack AI Audit Pattern
**Context**: When activating an agent for the first time on a mature codebase, a comprehensive multi-subsystem audit is essential.
**Pattern**:
1. Map all relevant packages via glob + grep (not subagents for depth-limited envs)
2. For each package: read interface definitions, implementations, tests, TODOs
3. Build a dependency/integration graph in mind — how do subsystems wire together?
4. Identify: (a) what's complete and functional, (b) what's partially implemented, (c) what's missing entirely, (d) what's broken/hacky
5. Classify gaps by severity: P0 (blocks core functionality), P1 (significant limitation), P2 (nice-to-have)
6. Propose 3 concrete next steps with file paths
7. Register learnings per LEARNING_PROTOCOL.md format
8. Update capability profile with expanded domains and confidence scores
**Tags**: #audit #pattern #activation #gap-analysis #multi-subsystem

### 2026-07-28 — Provider Registration Completeness Check
**Context**: Embedding/capability providers often have unequal implementation status. Some register chat but not embeddings.
**Pattern**:
1. Grep for `RegisterEmbedding` across all provider packages
2. Cross-reference with provider catalog (providers.List()) to find gaps
3. Check init() functions for each provider directory
4. Identify which providers implement embeddings.Provider vs chat.ChatProvider vs neither
5. Prioritize: P0 = providers the project already depends on for chat, P1 = commonly used, P2 = niche
**Tags**: #providers #registration #completeness #pattern
