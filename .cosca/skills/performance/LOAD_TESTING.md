> **Version**: 1.0.0 | **Status**: active | **Owner**: Performance Chief | **Last Updated**: 2026-07-23

# LOAD TESTING SKILL

## Description
Plan, design, and execute load tests to validate system performance under expected and peak traffic conditions. Identifies throughput limits, latency degradation curves, and resource bottlenecks.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| target_endpoints | Yes | Endpoints or services to test (URLs or service names) |
| load_profile | Yes | Expected traffic pattern (RPS, concurrent users, ramp-up) |
| test_duration | No | Test duration in minutes (default: 15) |
| ramp_up_time | No | Ramp-up period in minutes (default: 5) |
| tool | No | Load testing tool: `k6`, `artillery`, `locust`, `jmeter` (default: k6) |

## Outputs
| Output | Description |
|--------|-------------|
| Load test results | Performance metrics under various load levels |
| Bottleneck analysis | Identified bottlenecks with evidence |
| Capacity report | Maximum sustainable throughput |
| Scaling recommendations | Vertical and horizontal scaling guidance |

## Test Types

### Baseline Test
- Single user, no load
- Establishes baseline latency and throughput
- Validates test script correctness

### Smoke Test
- Low load (10-20% of expected peak)
- Validates system handles basic load
- Quick feedback on configuration issues

### Load Test
- Expected peak traffic levels
- Measures latency at target throughput
- Validates SLO compliance

### Stress Test
- 150-200% of expected peak
- Identifies breaking points
- Tests auto-scaling behavior

### Endurance Test
- Sustained load for extended period (1-12 hours)
- Detects memory leaks and degradation
- Validates long-running stability

### Spike Test
- Sudden traffic surge (2-5x normal)
- Tests burst handling and recovery
- Validates auto-scaling responsiveness

## Process
1. Define test scenarios based on traffic patterns
2. Configure load testing tool with target endpoints
3. Implement test scripts with realistic user flows
4. Execute baseline test to validate setup
5. Execute load test at expected peak traffic
6. Execute stress test to find breaking point
7. Execute endurance test for stability
8. Analyze results: latency, throughput, errors, resources
9. Identify bottlenecks with root causes
10. Generate report with recommendations

## Success Criteria
- [ ] All test types executed as planned
- [ ] Latency metrics captured (p50, p95, p99)
- [ ] Throughput limits identified
- [ ] Bottlenecks documented with evidence
- [ ] Scaling recommendations provided
- [ ] Results archived in memory

## Related
- [Performance Chief](../../departments/performance/SKILL.md)
- [Performance Audit](./PERFORMANCE_AUDIT.md)
- [Database Performance](./DATABASE_PERFORMANCE.md)
- [workflows/performance-optimization.md](../../workflows/performance-optimization.md)
