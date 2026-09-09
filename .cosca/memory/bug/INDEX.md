# Bug Memory — Cosca

> **Version**: 3.0.0 | **Status**: active | **Last Updated**: 2026-07-28
>
> **Upgrade v3.0.0:** Causality Tree (4 níveis) aplicado a todos os bugs existentes. Template: [CAUSALITY_TREE_TEMPLATE.md](CAUSALITY_TREE_TEMPLATE.md).

## Causality Summary

| Bug | N1 — Causa Direta | N2 — Arquitetural | N3 — Processo | N4 — Prevenção |
|-----|-------------------|-------------------|---------------|----------------|
| [001](bug-001-tmp-path.md) | Paths hardcoded `/tmp/` | Sem abstração de project temp dir | CI só Linux, sem lint rule | Matrix OS no CI, forbidigo rule |
| [002](bug-002-lint-issues.md) | 959 style issues acumuladas | CI sem gate de qualidade estática | Sem pre-commit hooks | golangci-lint required check |
| [003](bug-003-race-conditions.md) | Maps sem sync em goroutines | Sem contratos de thread-safety | `-race` ausente no CI | `go test -race` obrigatório |
| [004](bug-004-provider-caching.md) | Cache sem TTL/invalidação | Sem separação config vs instance | Sem soak test (>24h) | TTL obrigatório + soak test no CI |
| [005](bug-005-sqlite-first-run.md) | `panic()` em first-run | Sem conceito de zero state | Sem teste de fresh install | Clean state test + forbidigo `panic()` |

## Active Bugs
| Key | Severity | Description | Fixed |
|-----|----------|-------------|-------|
| [bug-001-tmp-path](bug-001-tmp-path.md) | medium | Hardcoded /tmp paths in daemon PID file | ✅ c30fac3 |
| [bug-002-lint-issues](bug-002-lint-issues.md) | low | 959 golangci-lint warnings | ✅ 03860c2 |
| [bug-003-race-conditions](bug-003-race-conditions.md) | high | Race conditions in parallel execution | ✅ 9a950ff |
| [bug-004-provider-caching](bug-004-provider-caching.md) | medium | LLM provider instance cache leak | ✅ f3dbdc2 |
| [bug-005-sqlite-first-run](bug-005-sqlite-first-run.md) | high | SQLite migration panic on fresh install | ✅ c30fac3 |

## Usage
Bug memory records bugs found IN the Cosca project. All entries reference real commits and actual code paths. Template/foreign bugs removed 2026-07-28 (limpeza de memória).
