---
name: adr-creation
description: Use when the user asks to create an Architecture Decision Record (ADR) capturing decision context, drivers, considered alternatives, and the chosen outcome.
---

> **Version**: 1.0.0 | **Status**: active (canonical — supersedes architecture/ADR_GENERATION) | **Owner**: Documentation Chief | **Last Updated**: 2026-07-23

# ADR CREATION SKILL

## Description
Create Architecture Decision Records (ADRs) following the standard template. Captures context, decision drivers, considered alternatives, outcomes, and consequences for every significant architecture decision.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| decision_title | Yes | Concise title of the decision |
| context | Yes | Why this decision is needed (background, problem, constraints) |
| alternatives | Yes | At least 2 alternatives considered with pros/cons |
| decision | Yes | The chosen option with rationale |
| consequences | Yes | Expected positive and negative consequences |
| deciders | No | List of people involved in the decision (default: team) |
| status | No | `proposed`, `accepted`, `deprecated`, `superseded` (default: proposed) |

## Outputs
| Output | Description |
|--------|-------------|
| ADR document | Complete ADR in standard format |
| ADR metadata | Status, date, deciders, tags for discovery |

## ADR Standard Format
```markdown
# ADR-NNN: [Decision Title]

**Status**: [proposed | accepted | deprecated | superseded]
**Deciders**: [list of decision-makers]
**Date**: [YYYY-MM-DD]
**Tags**: [domain-tags, e.g., architecture, security, database]

## Context
[Problem description, background, and forces at play]

## Decision Drivers
- [Driver 1: e.g., "Scalability to 10M users"]
- [Driver 2: e.g., "Team expertise in TypeScript"]

## Considered Options
- Option A: [Brief description]
- Option B: [Brief description]
- Option C: [Brief description]

## Decision Outcome
**Chosen option**: Option A because [rationale]

### Positive Consequences
- [Benefit 1]
- [Benefit 2]

### Negative Consequences
- [Trade-off 1]
- [Trade-off 2]

## Pros and Cons of Options

### Option A
- Good: [argument]
- Good: [argument]
- Bad: [argument]

### Option B
- Good: [argument]
- Bad: [argument]
- Bad: [argument]

## Compliance
[How to verify ADR is followed in implementation]
```

## Process
1. Identify that a decision needs to be recorded
2. Determine next ADR number from sequence
3. Draft context and decision drivers
4. Document all considered alternatives
5. Record decision outcome with rationale
6. Capture positive and negative consequences
7. Define compliance verification approach
8. Review with stakeholders
9. Set initial status (proposed) → final status (accepted)
10. Store in memory/architecture/adr/ with index update

## Success Criteria
- [ ] Context clearly describes the problem
- [ ] Decision drivers listed (at least 2)
- [ ] At least 2 alternatives considered
- [ ] Decision rationale documented
- [ ] Consequences captured (positive and negative)
- [ ] Compliance verification defined
- [ ] ADR stored in architecture memory
- [ ] Index updated for discovery

## Related
- [Documentation Chief](../../departments/documentation/SKILL.md)
- [ADR Generation](../../skills/architecture/ADR_GENERATION.md)
- [Architecture Chief](../../departments/architecture/SKILL.md)
- [memory/architecture/](../../memory/architecture/INDEX.md)
