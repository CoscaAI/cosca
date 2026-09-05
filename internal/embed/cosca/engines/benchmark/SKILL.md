---
name: benchmark
description: Provides standardized, reproducible measurement of agent performance across all task types.
level: 2
---

# AGENT BENCHMARK ENGINE

> **Version**: 1.0.0 | **Status**: active | **Owner**: Benchmark Engine | **Last Updated**: 2026-07-12

## PURPOSE
The Agent Benchmark Engine provides standardized, reproducible measurement of agent performance across all task types. It generates the data that feeds the Learning Engine for agent selection optimization and the Evolution Engine for quality improvement. Without benchmarks, "agent performance" is subjective opinion — with benchmarks, it's measurable fact.

## ACTIVATION
- **Scheduled**: Weekly per agent type (Monday: Chiefs, Wednesday: Specialists, Friday: Review)
- **On new agent**: First-time calibration benchmark
- **On agent update**: Regression benchmark after skill changes
- **On demand**: `/benchmark [agent-type]` command
- **Before major release**: Full suite benchmark

## SCOPE
- Standardized benchmark tasks per agent type
- Multi-dimensional scoring (correctness, completeness, efficiency, consistency)
- Historical tracking and trend analysis
- Agent ranking and recommendation
- Regression detection (did this agent get worse?)
- Provider comparison (same task, different AI models)

## OUT OF SCOPE
- Agent selection (delegate to Kernel/Learning Engine)
- Agent skill modification (delegate to Evolution Engine)
- Production task execution (benchmarks are isolated)
- Real user impact measurement (delegate to Observability Engine)

## PROCESS

### 1. Benchmark Design
```
For each agent type:
  → Define N benchmark tasks (varied difficulty)
  → Define expected output format and quality criteria
  → Define scoring rubric per dimension
  → Store in benchmark registry
```

### 2. Benchmark Execution
```
Schedule: Weekly rotation
  → Spin up isolated agent instance
  → Execute each benchmark task sequentially
  → Measure: output, tokens, duration, tool calls
  → Score each dimension (0.0 - 10.0)
  → Store results in Agent Memory
```

### 3. Trend Analysis
```
Compare: current vs previous vs baseline
  → Detect regression (score dropped > 1.0 in any dimension)
  → Detect improvement (score increased > 0.5)
  → Track token efficiency over time
  → Generate trend report for Learning Engine
```

## BENCHMARK CATALOG

### Architecture Chief Benchmarks
| ID | Task | Difficulty | Expected Output |
|----|------|-----------|-----------------|
| ARCH-001 | Design REST API for e-commerce | Medium | Module boundaries, endpoint design, ADR |
| ARCH-002 | Evaluate monolith vs microservices | Hard | Tradeoff analysis with decision rationale |
| ARCH-003 | Design database schema for blog | Easy | ERD, normalization, indexing strategy |
| ARCH-004 | Design auth flow for multi-tenant SaaS | Hard | OAuth2/OIDC, RBAC, session management |
| ARCH-005 | Critique existing architecture | Medium | Issues found, improvement suggestions |

### Backend Chief Benchmarks
| ID | Task | Difficulty | Expected Output |
|----|------|-----------|-----------------|
| BACK-001 | Implement CRUD API for users | Easy | REST endpoints, validation, error handling |
| BACK-002 | Implement JWT auth middleware | Medium | Token generation, validation, refresh |
| BACK-003 | Design rate limiter with Redis | Medium | Algorithm, configuration, tests |
| BACK-004 | Implement file upload with S3 | Medium | Presigned URLs, multipart, progress |
| BACK-005 | Refactor payment service | Hard | Strategy pattern, test coverage, no regressions |

### Code Review Benchmarks
| ID | Task | Difficulty | Expected Output |
|----|------|-----------|-----------------|
| REV-001 | Review code with SQL injection | Easy | Detect, explain, suggest fix |
| REV-002 | Review code with N+1 queries | Easy | Detect, suggest eager loading |
| REV-003 | Review code with race condition | Hard | Detect, suggest locking strategy |
| REV-004 | Review code with memory leak | Medium | Detect, suggest cleanup |
| REV-005 | Review full PR with mixed issues | Hard | Categorized issues with severity |

### Testing Benchmarks
| ID | Task | Difficulty | Expected Output |
|----|------|-----------|-----------------|
| TEST-001 | Write unit tests for utility function | Easy | AAA pattern, edge cases |
| TEST-002 | Write integration test for API endpoint | Medium | Setup, teardown, assertions |
| TEST-003 | Write E2E test for login flow | Medium | User simulation, assertions |
| TEST-004 | Identify missing test cases in suite | Medium | Gap analysis, test suggestions |
| TEST-005 | Fix flaky test | Hard | Root cause, fix, verification |

### Security Benchmarks
| ID | Task | Difficulty | Expected Output |
|----|------|-----------|-----------------|
| SEC-001 | Audit code for OWASP Top 10 | Medium | Categorized findings |
| SEC-002 | Review JWT implementation | Easy | Issues found, recommendations |
| SEC-003 | Audit dependency CVEs | Easy | CVE list, upgrade path |
| SEC-004 | Design RBAC system | Hard | Roles, permissions, enforcement |
| SEC-005 | Review cloud IAM policies | Medium | Least privilege analysis |

## SCORING RUBRIC

### Dimension 1: Correctness (0-10)
```
Does the output solve the problem correctly?
  10: Perfect solution, all edge cases handled
   7: Good solution, minor issues
   5: Partial solution, significant gaps
   3: Attempted but mostly incorrect
   0: Completely wrong or unresponsive
```

### Dimension 2: Completeness (0-10)
```
Does the output address all requirements?
  10: All requirements met, bonus considerations
   7: All core requirements met
   5: Most requirements, some missing
   3: Few requirements met
   0: Requirements ignored
```

### Dimension 3: Efficiency (0-10)
```
How efficiently was the task completed?
  10: Minimal tokens, direct solution, no wasted steps
   7: Efficient but some redundancy
   5: Verbose, unnecessary tool calls
   3: Very inefficient, circular reasoning
   0: Token limit exceeded without result
```

### Dimension 4: Consistency (0-10)
```
Same task, repeated 3 times — how consistent?
  10: Identical quality across all 3 runs
   7: Minor variation, same outcome
   5: Moderate variation, mixed outcomes
   3: High variation, unpredictable
   0: Different result every run
```

### Overall Benchmark Score
```
OVERALL = (Correctness × 0.40) + (Completeness × 0.25) + (Efficiency × 0.20) + (Consistency × 0.15)
```

## BENCHMARK REPORT

```markdown
# BENCHMARK REPORT — Architecture Chief
Date: 2026-07-12 | Benchmark Suite: v1.0 | Provider: Claude 3.5 Sonnet

## SUMMARY
- Tasks executed: 5
- Overall Score: 8.4 / 10 (A−)
- Previous Score: 8.1 / 10 (B+)
- Trend: ↑ Improving (+0.3)

## PER-TASK RESULTS
| Task | Difficulty | Correctness | Completeness | Efficiency | Consistency | Score |
|------|-----------|-------------|-------------|------------|-------------|-------|
| ARCH-001 | Medium | 9.0 | 9.0 | 7.5 | 9.0 | 8.6 |
| ARCH-002 | Hard | 8.0 | 8.5 | 6.5 | 8.0 | 7.8 |
| ARCH-003 | Easy | 9.5 | 9.0 | 9.0 | 9.5 | 9.3 |
| ARCH-004 | Hard | 7.5 | 8.0 | 6.0 | 7.5 | 7.3 |
| ARCH-005 | Medium | 8.5 | 8.5 | 8.0 | 8.5 | 8.4 |

## EFFICIENCY METRICS
- Avg tokens per task: 2,450
- Avg duration: 28s
- Avg tool calls: 3.2
- Token efficiency score: 7.2/10

## REGRESSION DETECTION
- No regressions detected (all tasks within 0.5 of previous)

## PROVIDER COMPARISON
| Provider | Overall | Cost/Task |
|----------|---------|-----------|
| Claude 3.5 Sonnet | 8.4 | $0.012 |
| GPT-4o | 8.6 | $0.035 |
| Llama 3 70B | 6.8 | $0.000 |

## RECOMMENDATIONS
1. ARCH-004 (multi-tenant auth) is consistently the weak point — consider skill update
2. Claude 3.5 offers best cost/quality ratio for architecture tasks
3. Efficiency could improve: reduce redundant context loading
```

## BENCHMARK SCHEDULE

| Day | Agent Types | Suite Size |
|-----|------------|------------|
| Monday | Architecture, CTO, Product | 5 tasks each |
| Tuesday | Backend, Frontend, Mobile | 5 tasks each |
| Wednesday | Database, DevOps, Infrastructure | 4 tasks each |
| Thursday | Security, Review, QA | 5 tasks each |
| Friday | Documentation, Testing, AI | 3 tasks each |
| Weekend | Provider comparison (all agents × all providers) | 5 per agent |

## REGRESSION ALERT RULES

| Condition | Action |
|-----------|--------|
| Score drops > 1.0 in any dimension | Alert agent owner + CTO |
| Score drops > 2.0 overall | Block agent from production, investigate |
| Consistency drops > 2.0 | Flag as flaky, increase benchmark frequency |
| Token efficiency decreases > 30% | Alert AI Chief, investigate cost impact |
| New agent baseline < 5.0 | Do not activate for production tasks |

## INPUTS

| Input | From | Format |
|-------|------|--------|
| Benchmark tasks | Benchmark registry | Task definitions |
| Agent configuration | Skills Engine | Agent type, tools, model |
| Previous results | Agent Memory | Historical benchmark data |
| Provider configuration | PROVIDER_INTERFACE.md | Provider registry |

## OUTPUTS

| Output | To | Format |
|--------|-----|--------|
| Benchmark report | Agent Memory, Learning Engine | Structured report |
| Regression alerts | Agent owner, CTO | Alert notification |
| Provider comparison | AI Chief | Comparison matrix |
| Trend analysis | Evolution Engine | Trend data |
| Agent ranking | Kernel | Ranked agent list per task type |

## DEPENDENCIES

| Depends On | Why |
|-----------|-----|
| Learning Engine | Consumer of benchmark data for optimization |
| Evolution Engine | Consumer of regression data for skill improvements |
| Agent Memory | Storage of historical benchmark results |
| PROVIDER_INTERFACE.md | Provider comparison benchmark |
| Skills Engine | Agent type registry and configuration |
| Observability Engine | Trend data cross-reference |

## CONSTRAINTS

- Benchmarks must be isolated (no side effects on production)
- Same task, same agent type, same provider = comparable
- Benchmark suite must be versioned (tasks may evolve)
- Results stored for minimum 90 days (trend window)
- Regression alerts must fire within 1h of benchmark completion
- Provider comparison uses identical tasks for fairness

## QUALITY CRITERIA

- [ ] All agent types have at least 3 benchmark tasks
- [ ] Benchmark suite executed on schedule (0 missed weeks)
- [ ] Results stored in Agent Memory with full metadata
- [ ] Regression detection Sensitivity ≥ 90% (catches real regressions)
- [ ] Provider comparison updated weekly
- [ ] Trend reports generated after every benchmark run
- [ ] New agents benchmarked before production activation

## RELATED

- [Learning Engine](../learning/SKILL.md) — Consumes benchmark data for agent optimization
- [Evolution Engine](../evolution/SKILL.md) — Consumes regression data for skill improvements
- [AI Chief](../../departments/ai/SKILL.md) — Provider selection strategy
- [PROVIDER_INTERFACE.md](../../PROVIDER_INTERFACE.md) — Provider registry for comparison
- [MEMORY_MODEL.md](../../MEMORY_MODEL.md) — Agent Memory schema
- [Observability Engine](../observability/SKILL.md) — Cross-reference with production metrics

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Initial Agent Benchmark Engine — standardized scoring, multi-agent catalog, regression detection, provider comparison |
