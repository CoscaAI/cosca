# WORKFLOW: performance-audit

> **Version**: 1.0.0 | **Status**: active | **Category**: performance | **Last Updated**: 2026-07-10

## OBJECTIVE
Comprehensive performance analysis including response times, database queries, bundle sizes, memory usage, and scaling bottlenecks.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| scope | string | No | full or module-specific |
| profile_type | string | No | api, database, frontend, full-stack |
| baseline_compare | boolean | No | Compare against previous audit baseline |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| performance_report | object | Overall performance assessment |
| bottlenecks | array | Identified performance bottlenecks |
| metrics | object | Key metrics with before/after comparison |
| recommendations | array | Optimization recommendations by impact |

## PRECONDITIONS
1. Application running in test/staging environment
2. Monitoring tools configured
3. Previous audit baseline available (if comparing)

## POSTCONDITIONS
1. Performance audit complete
2. Bottlenecks identified and prioritized
3. Optimization recommendations provided
4. Audit stored for trend analysis

## DEPENDENCIES
None

## STEPS

### Step 1: API Performance Profiling
- **Chief**: Backend
- **Specialists**: Performance Engineer
- **Task**: Profile API endpoints — response times (p50, p95, p99), throughput, error rates
- **Output**: API performance profile

### Step 2: Database Query Analysis
- **Chief**: Database
- **Specialists**: DBA
- **Task**: Analyze slow queries, missing indexes, N+1 patterns, connection pool usage
- **Output**: Database performance report

### Step 3: Frontend Performance Audit
- **Chief**: Frontend
- **Specialists**: Frontend Performance Engineer
- **Task**: Measure Core Web Vitals (LCP, FID, CLS), bundle size, render performance
- **Output**: Frontend performance report

### Step 4: Memory & Resource Analysis
- **Chief**: Runtime
- **Specialists**: Runtime Engineer
- **Task**: Analyze memory usage, event loop lag, garbage collection patterns
- **Output**: Resource utilization report

### Step 5: Scalability Assessment
- **Chief**: Architecture
- **Specialists**: Solutions Architect
- **Task**: Evaluate current architecture for scaling bottlenecks, identify limits
- **Output**: Scalability assessment

### Step 6: Bottleneck Prioritization
- **Chief**: Architecture
- **Specialists**: Performance Engineer
- **Task**: Prioritize bottlenecks by impact (cost, latency, user experience)
- **Output**: Prioritized bottleneck list

### Step 7: Recommendations
- **Chief**: Architecture
- **Specialists**: Solutions Architect, Performance Engineer
- **Task**: Generate optimization recommendations with effort/impact estimates
- **Output**: Optimization recommendations

### Step 8: Baseline Update
- **Chief**: QA
- **Specialists**: Performance Tester
- **Task**: Store audit results as new baseline for future comparison
- **Output**: Updated performance baseline

## VALIDATION
1. All scoped areas profiled
2. Metrics collected for all endpoints/queries/components
3. Bottlenecks classified by impact
4. Recommendations are actionable
5. Baseline updated for trend tracking

## SUCCESS CRITERIA
- [ ] Complete performance report generated
- [ ] API, database, frontend, resource profiles complete
- [ ] Bottlenecks identified and prioritized
- [ ] Optimization recommendations provided with effort/impact estimates
- [ ] Baseline updated

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Profiling tool unavailable | Use alternative measurement approach |
| Incomplete data | Report partial results, retry affected areas |
| No bottlenecks found | Verify profiling methodology, not just absence |

## RELATED
- [Backend Chief](../departments/backend/SKILL.md)
- [Database Chief](../departments/database/SKILL.md)
- [Frontend Chief](../departments/frontend/SKILL.md)
- [Architecture Chief](../departments/architecture/SKILL.md)
- [QUALITY_GATES.md](../QUALITY_GATES.md)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Initial performance-audit workflow |
