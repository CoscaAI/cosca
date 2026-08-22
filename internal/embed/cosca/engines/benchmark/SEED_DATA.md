# Benchmark Engine — Seed Data

> **Version**: 1.0.0 | **Status**: active | **Last Updated**: 2026-07-23

## Agent Benchmarks

### AI Provider Comparison (2026-Q3)
| Provider | Model | Latency p50 | Latency p95 | Cost/1K tokens | Throughput |
|----------|-------|:---------:|:---------:|:------------:|:----------:|
| OpenAI | GPT-4o | 480ms | 1,200ms | $0.005 | 45 req/s |
| Anthropic | Claude 3.5 Sonnet | 520ms | 1,450ms | $0.003 | 38 req/s |
| Google | Gemini 1.5 Pro | 610ms | 1,800ms | $0.0035 | 32 req/s |
| Local | Llama 3.1 70B | 1,200ms | 3,500ms | $0.002 | 15 req/s |

### Workflow Execution Benchmarks
| Workflow | Avg Duration | p95 Duration | Success Rate | Executions |
|----------|:-----------:|:-----------:|:----------:|:---------:|
| code-review | 4.2 min | 8.1 min | 97% | 342 |
| security-audit | 12.5 min | 22.3 min | 94% | 156 |
| dependency-update | 3.1 hours | 8.5 hours | 89% | 67 |
| migration-execution | 2.3 hours | 5.1 hours | 95% | 23 |
| incident-response | 18 min | 45 min | 99% | 89 |

### Agent Performance by Department
| Department | Tasks Completed | Avg Quality Score | Avg Duration |
|------------|:--------------:|:----------------:|:-----------:|
| Backend | 1,245 | 8.7/10 | 3.2h |
| Frontend | 892 | 8.5/10 | 2.8h |
| Security | 445 | 9.1/10 | 1.5h |
| QA | 678 | 8.8/10 | 4.1h |
| DevOps | 334 | 8.9/10 | 1.8h |

## Related
- [Benchmark Engine](../tools/SKILL.md)
- [Performance Chief](../../departments/performance/SKILL.md)
- [Provider Chief](../../departments/provider/SKILL.md)
