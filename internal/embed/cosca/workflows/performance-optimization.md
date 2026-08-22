# WORKFLOW: performance-optimization

> **Version**: 1.0.0 | **Category**: refactor | **Estimated Duration**: 2-5 days | **Status**: active | **Owner**: Performance Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Systematically identify, prioritize, and resolve performance bottlenecks. Uses data-driven approach with before/after benchmarking to validate improvements and prevent regressions.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| target_system | String | Yes | System or service to optimize |
| slo_targets | Document | Yes | Target SLOs (latency, throughput, error rate) |
| known_bottlenecks | String[] | No | Suspected bottlenecks to investigate first |
| optimization_budget | String | No | Effort estimate: XS/S/M/L/XL |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| Optimization report | Document | Changes made and measured impact |
| Benchmark comparison | Report | Before/after metrics with variances |
| New baselines | Document | Updated performance baselines for future comparison |
| Regression tests | Config | Automated performance regression checks |

## PRECONDITIONS
1. Performance baselines established for target system
2. Monitoring data available for current performance
3. Optimization budget allocated and approved

## POSTCONDITIONS
1. Performance SLOs met after optimization
2. Benchmarks updated with new baselines
3. Regression tests deployed in CI/CD
4. Documentation updated with optimization results

## STEPS
### Step 1: Baseline Measurement
- **Chief**: Performance Chief
- **Specialists**: Performance Test Engineer
- **Task**: Establish current performance baselines (latency p50/p95/p99, throughput, error rate)
- **Output**: Baseline benchmark report

### Step 2: Bottleneck Analysis
- **Chief**: Performance Chief
- **Specialists**: Performance Test Engineer, Database Performance Engineer
- **Task**: Profile system, identify bottlenecks using profiling tools and monitoring data
- **Output**: Bottleneck inventory with impact estimates

### Step 3: Optimization Design
- **Chief**: Performance Chief
- **Specialists**: Backend/Frontend Chief (depending on target)
- **Task**: Design optimization approach for each bottleneck, estimate improvement
- **Output**: Optimization plan with expected impact

### Step 4: Implementation
- **Chief**: Backend/Frontend Chief
- **Specialists**: Service Developer
- **Task**: Implement optimizations following the plan
- **Output**: Optimized code

### Step 5: Validation
- **Chief**: Testing Chief
- **Specialists**: Performance Test Engineer
- **Task**: Run benchmarks, compare against baselines, verify no regressions
- **Output**: Before/after comparison report

### Step 6: Deployment
- **Chief**: DevOps Chief
- **Specialists**: Release Engineer
- **Task**: Deploy optimized code to production with monitoring
- **Output**: Production deployment

### Step 7: Monitoring
- **Chief**: Monitoring Chief
- **Specialists**: SRE
- **Task**: Monitor production performance, verify improvements hold
- **Output**: Production performance report

## VALIDATION
1. Performance metrics improved as expected
2. No regressions in other dimensions
3. All existing tests pass
4. Production monitoring shows stable improvement

## SUCCESS CRITERIA
- [ ] Performance SLOs met or exceeded
- [ ] Bottlenecks resolved with measured improvement
- [ ] No regressions introduced
- [ ] Benchmarks updated in performance database
- [ ] Regression tests added to CI/CD

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Optimization degrades performance | Rollback changes, analyze root cause |
| Bottleneck not found after profiling | Escalate to Performance Chief for deep dive |
| Optimization conflicts with architecture | Escalate to Architecture Chief for ADR |
| Production degradation after deploy | Rollback, investigate, re-optimize |

## RELATED
- [Performance Chief](../departments/performance/SKILL.md)
- [Performance Audit skill](../skills/performance/PERFORMANCE_AUDIT.md)
- [Load Testing skill](../skills/performance/LOAD_TESTING.md)
- [QUALITY_GATES.md](../QUALITY_GATES.md)

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |
