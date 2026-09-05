---
type: pattern
key: pattern-enterprise-prompt-governance
tags: [enterprise, prompt, governance, bootstrap, quality, constitution]
category: architecture
confidence: 0.95
times_used: 1
times_succeeded: 1
---

# Enterprise Prompt Governance Pattern

## Intent
Establish a binding governance framework that transforms any opened workspace into a fully structured, auditable, reproducible, and sustainable Enterprise Workspace using Cosca as the AI Operating System.

## Context (When to use)
- Every time a workspace is opened in OpenCode
- When bootstrapping a new project
- When enforcing quality gates before implementation
- When maintaining memory and knowledge across sessions
- When auditing code before any modification

## Solution
The Enterprise Prompt defines a comprehensive governance model:

### 1. Discover → Load Context → Route → Never Implement
Chain of command: User → Kernel → CEO → CTO → Department Chiefs → Specialists

### 2. Automatic Bootstrap on every workspace open
- Detect language, framework, monorepo, package manager, database, docker, cloud, CI/CD
- Create `.cosca/` if missing with full structure (33+ subdirectories)
- Execute comprehensive audit before any code change

### 3. Quality Constitution (Prohibitions)
- No inventing implementations, behaviors, architecture, APIs, responses, results, tests
- No ignoring errors, warnings, lint, vet, tests, types, build, broken deps
- No bypassing quality (no `@ts-ignore`, `// eslint-disable`, `// nolint`, permanent mocks)
- Every exception: temporary, documented, justified, with removal plan

### 4. Error Treatment Protocol
Discover → Reproduce → Understand → Find Root Cause → Document → Plan → Implement → Validate → Test → Audit Again → Finalize

### 5. Continuous Learning
After each task: update memory, blueprint, knowledge graph, documentation, ADRs, workflows, lessons learned, reusable patterns

## Consequences
- Zero tolerance for quality bypass
- Every modification has documented rationale
- Persistent memory across sessions
- Audit trail for all decisions
- Self-improving system

## Known Uses
- Cosca Global context
- Project bootstrapping
- Quality gate enforcement
- Memory management

## Related Patterns
- Cosca Bootstrap Engine (bootstrap/BOOTSTRAP.md)
- Cosca Quality Gates (QUALITY_GATES.md)
- Cosca Memory Model (MEMORY_MODEL.md)
- Cosca Kernel (KERNEL.md)
