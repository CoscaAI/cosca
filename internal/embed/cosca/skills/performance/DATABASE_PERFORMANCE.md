---
name: database-performance
description: Use when the user asks to optimize database performance at the system level (connection pooling, throughput, index health, and vacuum).
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Performance Chief | **Last Updated**: 2026-07-23

# DATABASE PERFORMANCE SKILL

## Description
Analyze and optimize database performance at the system level. Covers connection pooling, query throughput, index health, vacuum/maintenance, replication lag, and capacity planning.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| database_type | Yes | `postgresql`, `mysql`, `mongodb`, `sqlserver`, `oracle` |
| connection_string | No | Read-only connection for live analysis |
| slow_query_log | No | Path to slow query log file |
| monitoring_period | No | Analysis period in hours (default: 24) |

## Outputs
| Output | Description |
|--------|-------------|
| Performance report | Database health and performance analysis |
| Bottleneck identification | Specific bottlenecks with root causes |
| Optimization recommendations | Prioritized actions with expected impact |
| Capacity forecast | Growth trends and scaling recommendations |

## Analysis Dimensions

### Connection Management
- Active vs idle connection ratio
- Connection pool utilization
- Connection wait events
- Max connections vs current usage
- Connection leak detection

### Query Throughput
- Queries per second (QPS)
- Read/write ratio
- Cache hit ratio (buffer pool, query cache)
- Temporary file creation (disk spill)
- Wait events breakdown

### Index Health
- Index usage statistics (used vs unused)
- Index scan vs table scan ratio
- Index bloat estimation
- Missing index detection
- Duplicate/redundant indexes

### Maintenance
- Vacuum/analyze frequency (PostgreSQL)
- Index fragmentation level (MSSQL)
- Table statistics freshness
- Replication lag
- WAL generation rate

### Storage Performance
- IOPS and throughput
- Storage latency
- Disk utilization
- TempDB growth (MSSQL)
- Undo tablespace usage (MySQL)

## Process
1. Connect to database (read-only) or load slow query log
2. Analyze connection pool and active connections
3. Review query throughput and cache hit ratios
4. Evaluate index health and usage
5. Check maintenance operations and schedules
6. Analyze storage performance metrics
7. Identify bottlenecks with root cause analysis
8. Generate prioritized recommendations
9. Create capacity forecast based on growth trends

## Success Criteria
- [ ] Connection pool analysis complete
- [ ] Query throughput baselines established
- [ ] Index health assessed (used/unused/bloat)
- [ ] Maintenance schedule evaluated
- [ ] Storage performance analyzed
- [ ] Bottlenecks identified with root causes
- [ ] Recommendations prioritized by impact

## Related
- [Performance Chief](../../departments/performance/SKILL.md)
- [Database Chief](../../departments/database/SKILL.md)
- [Query Optimization](../../skills/data/QUERY_OPTIMIZATION.md)
- [Database Audit](../../skills/data/DATABASE_AUDIT.md)
- [Performance Audit](./PERFORMANCE_AUDIT.md)
- [Load Testing](./LOAD_TESTING.md)
