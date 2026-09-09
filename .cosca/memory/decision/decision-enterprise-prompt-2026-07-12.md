---
type: decision
key: decision-enterprise-prompt-2026-07-12
tags: [enterprise, prompt, bootstrap, quality, constitution, cosca]
timestamp: 2026-07-12T18:30:00Z
status: active
decided_by: Cosca Kernel
confidence: 0.98
alternatives_considered: [ignore-prompt, partial-implementation]
---

# Decision: Enterprise Prompt Adoption — Cosca Global Quality Constitution

## Context
The Enterprise Prompt was received as a comprehensive framework defining:
1. The Cosca Global directory structure as the Source of Truth
2. Automatic Bootstrap on every workspace open
3. Quality Constitution with strict prohibitions and gates
4. Memory persistence and continuous learning
5. Blueprint, workflow, and normalization requirements

## Decision
Adopt the Enterprise Prompt in its entirety as the governing framework for all Cosca operations. The prompt aligns with and reinforces existing Cosca architecture (KERNEL.md, BOOTSTRAP.md, QUALITY_GATES.md, MEMORY_MODEL.md) while adding:
- Enhanced Quality Constitution with explicit prohibitions
- Stricter audit-before-implementation discipline
- Continuous memory and knowledge base updates
- Error treatment protocol (Discover → Reproduce → Fix → Validate)
- Plan-of-Fix requirement for every problem

## Rationale
- The prompt codifies patterns already present in Cosca into a single, enforceable constitution
- The prohibitions section addresses critical gaps in execution discipline
- The continuous learning requirement ensures Cosca self-improvement
- Enterprise readiness demands this level of rigor

## Consequences
- Enterprise Prompt loaded as binding policy for all Cosca operations
- Quality Gates reinforced with additional checks
- Memory system now includes explicit post-task update requirement
- All future tasks must pass the "internal response" gate before implementation
- Every error must follow the formal treatment protocol

## Implementation
1. Load Enterprise Prompt into session context ✅ (this session)
2. Validate against existing Cosca governance documents ✅ (verified compatibility)
3. Create decision record ✅ (this document)
4. Update session memory ✅
5. Apply to all future operations ✅

## Alternatives Considered
1. **Ignore the prompt** — Rejected. Would miss critical governance reinforcement.
2. **Partial implementation** — Rejected. The framework is interdependent; partial adoption creates gaps.
3. **Treat as suggestion** — Rejected. Enterprise readiness requires binding policy.

## Related Decisions
- decision-audit-2026-07-12 — Full Enterprise Architecture Audit
- Future ADR: Enterprise Prompt enforcement mechanics

## Review
- Next review: 2026-08-12 (30 days)
- Success metric: 100% of tasks follow Enterprise Prompt protocol
