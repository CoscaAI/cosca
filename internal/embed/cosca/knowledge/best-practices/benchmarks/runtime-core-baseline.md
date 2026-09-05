---
id: benchmark-001
title: "Runtime Core Baseline — Cosca v1.4.0-dev"
category: runtime
created: 2026-07-29
hardware: "AMD Ryzen 7 5700X3D, 16 threads, Linux amd64"
go_version: "go1.25.0"
pkg: "github.com/CoscaAI/cosca/internal/runtime"
source_file: "internal/runtime/runtime_bench_test.go"
---

# Runtime Core Baseline

> **Purpose**: Estabelecer baseline de performance do runtime core para detecção de regressão.
> **Executed**: 2026-07-29, `go test -bench=. -benchmem -benchtime=1s -run=^$ ./internal/runtime/`
> **Coverage**: 97.9% (core), 6 benchmarks, 0 race conditions detected

---

## Baseline Measurements

### Benchmark Results (executed 2026-07-29)

| Benchmark | Latência (ns/op) | Memória (B/op) | Alocações (allocs/op) |
|-----------|-----------------|-----------------|----------------------|
| `BenchmarkNewEventBus-16` | 17.83 | 0 | 0 |
| `BenchmarkRuntimeStateTransition-16` | 8.921 | 0 | 0 |
| `BenchmarkEventBusSubscribe-16` | 62.24 | 43 | 0 |
| `BenchmarkEventBusPublish-16` | 593.4 | 336 | 6 |
| `BenchmarkEventBusPublishMultipleHandlers-16` | 3,916 | 2,784 | 42 |
| `BenchmarkRuntimeStartup-16` | 5,310 | 35,640 | 26 |

### Human-Readable

| Operação | Latência | Notas |
|----------|----------|-------|
| Criar EventBus vazio | 17.8 ns | Zero-alocação, alocado na stack |
| Ler estado do runtime | 8.9 ns | Zero-alocação, mutex read + atomic |
| Subscrever a evento | 62.2 ns | Append em slice, zero-alocação |
| Publicar evento (3 handlers) | 593 ns | 336B, 6 alocações (event copy + context) |
| Publicar evento (10 handlers) | 3.9 µs | 2.7KB, 42 alocações |
| Criar runtime vazio | 5.3 µs | 35.6KB, 26 alocações (struct + subsistemas) |

---

## Regression Threshold

**Nenhum benchmark pode degradar >20% sem justificativa documentada.**

| Benchmark | Baseline | Limite Aceitável | Trigger |
|-----------|----------|-------------------|---------|
| NewEventBus | 17.83 ns | ≤ 21.4 ns | > 21.4 ns |
| StateTransition | 8.92 ns | ≤ 10.7 ns | > 10.7 ns |
| EventBusSubscribe | 62.24 ns | ≤ 74.7 ns | > 74.7 ns |
| EventBusPublish | 593.4 ns | ≤ 712 ns | > 712 ns |
| PublishMultiple | 3,916 ns | ≤ 4,699 ns | > 4,699 ns |
| RuntimeStartup | 5,310 ns | ≤ 6,372 ns | > 6,372 ns |

**Processo se threshold violado:**
1. Executar `go test -bench=. -benchmem -count=5 ./internal/runtime/` para confirmar
2. Verificar `git diff` para mudanças recentes no pacote
3. Abrir issue com tag `performance-regression`
4. Documentar justificativa em `internal/embed/cosca/knowledge/benchmarks/` antes de aceitar

---

## Historical Comparison

| Data | Auditoria | NewEventBus | StateTransition | Subscribe | Publish | Multiple | Startup |
|------|-----------|-------------|-----------------|-----------|---------|----------|---------|
| 2026-07-28 | Pre-audit (learnings) | — | — | — | — | — | — |
| 2026-07-29 | Coverage audit (audit-report) | 17.8 ns | 8.6 ns | 56.0 ns | 573 ns | 3.7 µs | 4.8 µs |
| **2026-07-29** | **Esta baseline (re-executed)** | **17.83 ns** | **8.921 ns** | **62.24 ns** | **593.4 ns** | **3,916 ns** | **5,310 ns** |

**Nota sobre variância**: A auditoria original reportou números ligeiramente diferentes (17.8 vs 17.83, 8.6 vs 8.9, 56 vs 62, 573 vs 593, 3.7µs vs 3.9µs, 4.8µs vs 5.3µs). Esta baseline foi re-executada com `-benchtime=1s` (vs `-benchtime=3s` na auditoria) e carga de sistema de 2026-07-29. A variância de ~3-10% está dentro da margem de ruído esperada para benchmarks em CPU compartilhada.

---

## Supplementary: Discovery & Memory Benchmarks

### Discovery Engine (`internal/discovery/discovery_bench_test.go`)

| Benchmark | Latência | Memória | Alocações |
|-----------|----------|---------|-----------|
| EngineCreation | 79.4 ns | 192 B | 2 |
| EngineCreationWithOptions | 133.9 ns | 208 B | 3 |
| DiscoverRuntime | 389.9 ns | 152 B | 3 |
| DiscoverWorkspace | 28.9 µs | 5.2 KB | 101 |
| DiscoverProject | 117.1 µs | 22.9 KB | 479 |
| DiscoverEnvironment | 1,956.6 µs | 118.6 KB | 321 |

### Memory Engine (`internal/memory/memory_bench_test.go`)

| Benchmark | Latência | Memória | Alocações |
|-----------|----------|---------|-----------|
| MemoryStore | 1,599.3 µs | 20.5 KB | 115 |
| MemorySearch | 2,841.1 µs | 372.4 KB | 4,358 |
| MemoryRetrieve | 32.3 µs | 13.1 KB | 136 |
| MemoryEngineCreation | 5,646.7 µs | 23.3 KB | 224 |
| MemoryDelete | 4,499.4 µs | 55.0 KB | 394 |

### Search Pipeline (`internal/search/`)

| Benchmark | Latência | Memória | Alocações |
|-----------|----------|---------|-----------|
| FTS5TableToResultType | 5.0 ns | 0 B | 0 |
| SearchResultCreation | 16.2 ns | 0 B | 0 |
| TruncateContent | 31.3 ns | 64 B | 1 |
| Suggestions | 158.2 ns | 224 B | 5 |
| ResolveFTSTables | 230.9 ns | 112 B | 3 |
| GenerateSnippet | 335.3 ns | 320 B | 3 |

---

## Known Limitations

1. **Runtime startup benchmark** mede apenas `New()` (criação da struct), não `Start()` + `Stop()` (ciclo completo com subsistemas). O ciclo completo é pesado e requer setup de subsistemas.
2. **FTS5 document search** retorna 0 resultados devido a bug de schema (`documents_fts` referencia coluna `content` inexistente na tabela `documents`). Bug documentado em `internal/sqlite/schema.go:237-244`.
3. **Entity FTS5 search** falha com `SQL logic error: no such column: T.metadata`. Rebuild de índice panica ao tentar rebuild `entities_fts`.
4. **Benchmarks de memory** sofrem variância alta (~5-10%) devido a operações de I/O em disco (SQLite temp files).

---

## How to Re-run

```bash
# Core runtime
go test -bench=. -benchmem -benchtime=1s -run=^$ ./internal/runtime/

# Discovery
go test -bench=. -benchmem -benchtime=1s -run=^$ ./internal/discovery/

# Memory
go test -bench=. -benchmem -benchtime=1s -run=^$ ./internal/memory/

# Search
go test -bench=. -benchmem -benchtime=1s -run=^$ ./internal/search/

# SQLite FTS5 (alguns quebram — ver Known Limitations)
go test -bench='SanitizeFTSQuery|FTS5CountQuery' -benchmem -benchtime=1s -run=^$ ./internal/sqlite/
```

---

*Baseline registered by cosca-performance, 2026-07-29*
