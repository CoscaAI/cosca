# LEARNING PROTOCOL — Auto-Evolution Memory System

> **Version**: 2.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-28

## Purpose
Every agent in the Cosca ecosystem auto-evolves. This protocol defines how agents learn from experience, store knowledge semantically, learn from failures, track confidence, and apply increasingly advanced techniques over time. v2.0.0 adds **Negative Memory** and **Confidence Scoring** as part of the Metacognition Layer.

## The Metacognition Loop

```
TASK
 ↓
SELF-ASSESS (load capability profile, check confidence)
 ↓
RETRIEVE MEMORY (search learnings + failures + patterns)
 ↓
PLAN STRATEGY (select technique, avoid known pitfalls)
 ↓
EXECUTE (with full instrumentation)
 ↓
VERIFY RESULT (quality gates, security, regression)
 ↓
CRITIQUE OWN WORK (honest self-evaluation)
 ↓
EXTRACT PATTERN (success → pattern, failure → negative memory)
 ↓
UPDATE CAPABILITY MODEL (recalc confidence, check level-up)
```

Full pipeline specification: [workflows/metacognition-pipeline.md](../workflows/metacognition-pipeline.md)

---

## Memory Structure Per Agent

Each agent has its own memory directory: `.opencode/cosca/memory/agent/{agent-name}/`

| File | Purpose |
|------|---------|
| `learnings.md` | Semantic learning journal — each entry is a discrete technique or discovery |
| `failures.md` | **NEW v2.0**: Negative memory — catalog of failed approaches and root causes |
| `patterns.md` | Reusable solution patterns discovered by this agent |
| `evolution.md` | Agent capability evolution timeline — tracks level + confidence progression |
| `capability-profile.md` | **NEW v2.0**: Self-model with strengths, weaknesses, confidence scores, evolution goal |
| `INDEX.md` | Cross-reference index of all learnings (for fast retrieval) |

## Learning Entry Format

Every learning is recorded as a semantic block:

```markdown
### {timestamp} — {technique-name}

| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | What was being done (context) |
| **Technique** | The specific technique applied |
| **Level** | 1-5 (1=basic, 5=expert) |
| **Outcome** | success / partial / failure |
| **Tags** | #security #xss #input-validation |
| **Related** | OWASP Top 10, CSP headers, Content Security Policy |
| **Learned** | What was discovered or confirmed |
| **Next** | What to try next time (progressive difficulty) |

### Technique Evolution

- **Level 1**: Basic patterns (standard checks, common vulnerabilities)
- **Level 2**: Intermediate (multi-layered checks, tool integration)
- **Level 3**: Advanced (threat modeling, attack chain analysis)
- **Level 4**: Expert (zero-day patterns, novel attack vectors, research-level)
- **Level 5**: Master (contributing new techniques back to the framework)
```

### Semantic Retrieval

Before starting any task, the agent MUST:
1. Search `learnings.md` for tags matching the current task domain
2. Load the highest-level techniques matching the task
3. Apply the best known approach (not repeating basic checks when advanced ones exist)

### Cross-Agent Learning

Learnings are indexed globally via the knowledge engine (FTS5 full-text search + vector embeddings). When agent A discovers a pattern, agent B can find it via semantic search.

### Evolution Tracking

`evolution.md` records capability milestones with confidence scores:
```markdown
## Evolution Timeline

| Date | Level | Confidence | Capability | Trigger |
|------|-------|-----------|------------|---------|
| 2026-07-27 | 2 | 0.78 | XSS detection via CSP header analysis | Security audit finding #47 |
| 2026-08-01 | 3 | 0.85 | Automated threat modeling with STRIDE | Architecture review feedback |
```

---

## Negative Memory Format (NEW v2.0)

### Purpose
Failures are the most valuable learning resource. Every approach that failed must be recorded so the agent — and all other agents — can avoid repeating the same mistake.

### Entry Format

```markdown
### {timestamp} — {failure-name}

| Field | Value |
|-------|-------|
| **Agent** | cosca-{name} |
| **Task** | What was being attempted (context) |
| **Failed Approach** | The specific approach that did NOT work |
| **Root Cause** | Why it failed (technical reason, not blame) |
| **Consequence** | What broke, what was impacted |
| **Lesson** | What should be done instead |
| **Confidence Impact** | How much confidence dropped (-0.10, -0.15, -0.20) |
| **Tags** | #failure #learned #{domain} #{failure-type} |
| **Related Success** | Link to the learnings.md entry that eventually solved this |
| **Avoidance Pattern** | How to recognize this situation and avoid the same failure |
```

### Cross-Agent Avoidance

Failures are indexed globally with `#failure #learned` tags. Before executing in a domain, agents MUST:
1. Search failures.md across ALL agents for `#tags` matching the task
2. Check if any known failure mode matches the current approach
3. If a match is found, explicitly document why the approach differs or abort

### Failure → Pattern Conversion

When a failure leads to a successful alternative approach, the cycle is:
```
FAILURE (failures.md) → LEARNING (learnings.md) → PATTERN (patterns.md)
```

---

## Confidence Scoring Model (NEW v2.0)

### Purpose
Numerical self-assessment of agent reliability. Drives the SELF-ASSESS stage — agents must know when they're out of their depth.

### Calculation

```
Per-Domain Confidence = (SuccessCount × 0.6 + LevelFactor × 0.3 + RecencyFactor × 0.1) / MaxScore

Where:
  SuccessCount  = min(successes in domain, 20) / 20
  LevelFactor   = current_level / 5  (maps 1-5 to 0.2-1.0)
  RecencyFactor = 1.0 (last task succeeded), 0.3 (last task failed), 0.0 (no recent tasks)
```

### Confidence Adjustment Rules

| Event | Delta | Cap |
|-------|-------|-----|
| Successful task | +0.05 | max 1.00 |
| Failed task | -0.10 | min 0.10 |
| Novel technique discovered | +0.08 | max 1.00 |
| Repeated same failure mode | -0.15 | compounding per repeat |
| Pattern contributed to framework | +0.10 | max 1.00 |
| Regression (broke existing) | -0.20 | min 0.10 |
| First success after failure | +0.08 | recovery bonus |
| Collaboration success | +0.03 | team bonus |

### Decision Thresholds

| Confidence | Action |
|-----------|--------|
| ≥ 0.85 | Proceed autonomously |
| 0.70–0.84 | Proceed, flag for post-execution review |
| 0.50–0.69 | Proceed, request second opinion on plan |
| < 0.50 | Escalate — task exceeds current capability |

### Level-Up Thresholds

| From | To | Successful Tasks Required | Min Confidence |
|------|----|--------------------------|----------------|
| L1 | L2 | 5 at L1 | ≥ 0.80 |
| L2 | L3 | 10 at L2 | ≥ 0.80 |
| L3 | L4 | 15 at L3 + 1 novel contribution | ≥ 0.85 |
| L4 | L5 | 20 at L4 + 3 novel contributions | ≥ 0.90 |

---

## Capability Profile Format (NEW v2.0)

Stored in `capability-profile.md`:

```markdown
# {agent-name} — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: YYYY-MM-DD

## Current Level: N

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| {domain} | 0.XX | N | success/failure | ↑ ↓ → |

## Strengths
- [Capability the agent excels at — specific, not vague]
- [Capability the agent excels at]

## Weaknesses
- [Known gap — honest admission]
- [Known gap]

## Preferred Strategies
- [Go-to approach for common scenarios]
- [Go-to approach for common scenarios]

## Known Failure Modes
- [Pattern: what goes wrong, why, how to detect]
- [Pattern: what goes wrong, why, how to detect]

## Evolution Goal
Reach Level {N+1}:
"{What capability would this unlock?}"
```

