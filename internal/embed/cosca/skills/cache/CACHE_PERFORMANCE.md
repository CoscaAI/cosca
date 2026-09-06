---
name: cache-performance
description: Use when the user asks to profile and optimize cache performance, including hit ratio, eviction, and latency.
---

# Cache Performance

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cache Chief | **Last Updated**: 2026-07-27

## Purpose
Analyze cache hit ratios, optimize eviction policies, and prevent cache degradation.

## Process
1. Collect cache metrics: hit ratio, miss ratio, eviction count, memory usage.
2. Identify cold spots: cache keys with hit ratio < 50%.
3. Analyze eviction patterns: are frequently-used keys being evicted?
4. Tune eviction policy: LRU vs LFU vs TTL-based.
5. Adjust cache size: balance memory usage vs hit ratio.
6. Implement cache warming: preload frequently-accessed data on startup.

## Success Criteria
- Cache hit ratio maintained > 80% under load
- No cache stampede or thundering herd events
- Memory usage within budget
