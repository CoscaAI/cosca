> **Version**: 1.0.0 | **Status**: active | **Owner**: Performance Chief | **Last Updated**: 2026-07-23
> 
> # PERFORMANCE AUDIT SKILL
> 
> ## Description
> Use this skill to perform comprehensive performance audits. Analyzes API latency, database queries, frontend rendering, memory usage, CPU profiles, and identifies performance bottlenecks.
> 
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | system_path | Yes | Path or endpoint to audit |
> | audit_type | Yes | `api`, `database`, `frontend`, `memory`, `full` |
> | load_profile | No | Expected traffic profile for capacity planning |
> 
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Performance report | Complete performance analysis |
> | Bottleneck inventory | Prioritized bottlenecks |
> | Optimization plan | Specific improvements with expected impact |
> | Benchmark data | Current performance baselines |
> 
> ## Audit Dimensions
> 
> ### API Performance
> - Response times (p50, p95, p99)
> - Throughput (requests/second)
> - Error rates
> - Serialization overhead
> 
> ### Database Performance
> - Query execution times
> - Missing indexes
> - N+1 queries
> - Connection pool usage
> 
> ### Frontend Performance
> - Core Web Vitals (LCP, FID, CLS)
> - Bundle size analysis
> - First meaningful paint
> - Time to interactive
> 
> ### Memory & CPU
> - Memory allocation patterns
> - CPU hot spots
> - Garbage collection impact
> - Thread contention
> 
> ## Success Criteria
> - [ ] All performance dimensions analyzed
> - [ ] Bottlenecks identified with locations
> - [ ] Baseline benchmarks established
> - [ ] Optimization plan with estimated impact
> - [ ] Report with clear pass/fail per dimension
> 
> ## Related
> - [Performance Chief](../../departments/performance/SKILL.md)
> - [QUALITY_GATES.md](../../QUALITY_GATES.md) — Gate 2.4 Performance
> - [workflows/performance-audit.md](../../workflows/performance-audit.md)
> - [Load Testing](./LOAD_TESTING.md)

## Process
1. **Baseline Collection**: Run existing benchmarks (`make benchmark`) and record current performance metrics.
2. **Hot Path Identification**: Use profiling (`go test -cpuprofile`) to identify top CPU-consuming functions.
3. **Memory Analysis**: Run `go test -memprofile` to detect excessive allocations and memory leaks.
4. **Database Query Audit**: Run EXPLAIN QUERY PLAN on all frequent SQL queries. Flag full table scans.
5. **Concurrency Check**: Review goroutine usage — verify all have exit conditions, no leaks, appropriate use of channels vs mutexes.
6. **Network I/O Review**: Check HTTP client timeouts, connection pooling, retry logic with exponential backoff.
7. **Generate Report**: Document findings with before/after metrics, ranked by performance impact.
8. **Recommendations**: Provide concrete optimization suggestions with expected improvement estimates.
