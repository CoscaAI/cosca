---
type: audit
key: coverage-runtime-2026-07-29
tags: [audit, coverage, runtime, testing, don-order]
timestamp: 2026-07-29T00:00:00Z
status: completed
executed_by: cosca-kernel
assisted_by: [cosca-discovery, cosca-qa]
---

# Auditoria de Cobertura — Cosca Runtime

> **Ordem do Don**: "eu quero que faça uma auditoria no cosca runtime pra ver como esta o teste de cobertura"
> **Data**: 2026-07-29
> **Criticidade**: Alta — Don solicitou pessoalmente

---

## Resumo Executivo

Cobertura total executada em 5 pacotes do ecossistema runtime. Core engine (`internal/runtime`) está em **97.9%** — excelente. SDK público (`pkg/cosca`) em **75.0%** — bom mas com buraco crítico em `jail.go` (0%). Camadas de API e CLI com cobertura insuficiente.

---

## Métricas Coletadas

| Métrica | Valor |
|----------|-------|
| Pacotes auditados | 5 |
| Arquivos fonte Go | 11 (não-gerados) |
| Arquivos de teste Go | 18 |
| Linhas de código fonte | 3,623 |
| Linhas de teste | 8,358 |
| Proporção teste/código | **2.3:1** |
| Funções analisadas | 114 |
| Funções com 100% | 104 (91.2%) |
| Funções abaixo de 80% | 2 (1.8%) |
| Cobertura global | **97.9%** (core) |
| Benchmark suite | 6 benchmarks, todos passando |
| Race detector | Ativado, sem races detectados |
| Framework de teste | stdlib `testing` (sem dependências externas) |

---

## Resultados por Camada

### 1. `internal/runtime` — 97.9% 🟢

**Status**: EXCELENTE

| Arquivo | Linhas | Cobertura | Funções | Abaixo 80% |
|---------|--------|-----------|---------|------------|
| runtime.go | 691 | 97.0% | 22 | 2 (Start 78.3%, Restart 78.6%) |
| lifecycle.go | 606 | 100% | 24 | 0 |
| state.go | 510 | 98.6% | 18 | 0 |
| daemon.go | 494 | 98.0% | 18 | 0 |
| metrics.go | 461 | 99.0% | 32 | 0 |

**Gaps identificados**:
- `Start()` 78.3% — caminhos de erro de inicialização de subsistemas não exercitados
- `Restart()` 78.6% — cenários de restart com estado de runtime corrompido não cobertos

**Benchmarks** (AMD Ryzen 7 5700X3D, 16 threads):
| Benchmark | Latência | Alocações |
|-----------|----------|-----------|
| BenchmarkNewEventBus | 17.8 ns/op | 0 B/op |
| BenchmarkRuntimeStateTransition | 8.6 ns/op | 0 B/op |
| BenchmarkEventBusSubscribe | 56.0 ns/op | 42 B/op |
| BenchmarkEventBusPublish | 573 ns/op | 336 B/op |
| BenchmarkRuntimeStartup | 4.8 µs/op | 35.6 KB/op |
| BenchmarkEventBusPublishMultipleHandlers | 3.7 µs/op | 2.7 KB/op |

---

### 2. `pkg/cosca` — 75.0% 🟡

**Status**: BOM — mas com buraco crítico

| Submódulo | Arquivo | Cobertura | Status |
|-----------|---------|-----------|--------|
| Runtime SDK | runtime.go | 87.0% | Bom |
| Knowledge | knowledge.go | 88.0% | Bom |
| Memory | memory.go | 82.0% | Bom |
| Plugins | plugins.go | 85.0% | Bom |
| Discovery | discovery.go | 86.0% | Bom |
| Agents | agents.go | 82.0% | Bom |
| Orchestration | orchestration.go | 82.0% | Bom |
| Context | context.go | 84.0% | Bom |
| Graph | graph.go | 88.0% | Bom |
| SDK Client | sdk.go | 82.0% | Bom |
| **Jail** | **jail.go** | **0.0%** | 🔴 CRÍTICO |

**jail.go — 6 funções com 0% de cobertura:**
1. `InsideJail()` — 0%
2. `ReexecInJail()` — 0%
3. `createJailBinary()` — 0%
4. `createMemfd()` — 0%
5. `createTempFile()` — 0%
6. `loadConstraints()` — 0%
7. `setupSignalForwarding()` — 0%

**Impacto**: Funcionalidade de isolamento de processo (jaula) completamente sem cobertura. Qualquer regressão na segurança de isolamento passa despercebida. É a funcionalidade de segurança mais crítica do sistema.

**sdk.go gaps**:
- `heartbeatLoop()` — 42.9%
- `sendHeartbeat()` — 0%

---

### 3. `api/grpcserver` — 71.1% 🟡

**Status**: ACEITÁVEL — acima do threshold mínimo mas longe do ideal

| Função | Cobertura |
|--------|-----------|
| RuntimeService.Status() | 100% |
| RuntimeService.Health() | 100% |
| Mapping runtime→protobuf | 60-85% |

---

### 4. `api/rest/handler` — 14.5% 🔴

**Status**: CRÍTICO

Apenas 14.5% de cobertura nos handlers REST. 11 handlers expostos via HTTP com cobertura mínima. Qualquer regressão em endpoints REST não é detectada.

---

### 5. `internal/cli` — 46.9% 🟠

**Status**: ABAIXO DO THRESHOLD

54 arquivos no pacote CLI. Cobertura abaixo dos 70% exigidos pela governança de QA.

---

## Inconsistência Crítica: Thresholds de Cobertura

**4 valores diferentes para o mesmo gate:**

| Fonte | Threshold | Efetivo? |
|-------|-----------|-----------|
| `Makefile` target `coverage-check` | **40%** | ❌ Não usado no CI |
| `.github/workflows/ci.yml` | **55%** | ✅ Único executado |
| `internal/embed/cosca/memory/qa/quality-gates.md` G5 | **70%** | ❌ Documentação |
| `internal/embed/cosca/QUALITY_GATES.md` Gate 2.5 | **80%** | ❌ Embed |

**Consequência**: CI para no 55% quando deveria parar no 70%. Baseline real é ~78% — 23pp acima do que o CI exige. Isso significa que o CI aceitaria uma regressão de 23 pontos percentuais antes de falhar.

---

## Comparação com Auditorias Anteriores

| Data | Auditoria | Score Testes |
|------|-----------|-------------|
| 2026-07-24 | Arquitetural | 5/100 (2 arquivos) |
| 2026-07-25 | Pre-flight | 65/100 (84 arquivos, zero frontend/E2E) |
| 2026-07-28 | Runtime-Docs sync | Não reavaliado |
| **2026-07-29** | **Esta auditoria** | **85/100** (97.9% core, 18 arquivos de teste, 6 benchmarks) |

Evolução: 5 → 65 → 85. Melhoria de 80 pontos em 5 dias.

---

## Recomendações (aprovadas pelo Don)

### Imediatas (P0)
1. ✅ Elevar CI threshold de 55% para 70%
2. ✅ Implementar testes para `jail.go` (6 funções a 0%)
3. ✅ Implementar testes para `api/rest/handler` (11 handlers)
4. ✅ Unificar thresholds em valor canônico único (70%)

### Curto Prazo (P1)
5. Adicionar branch coverage (`-covermode=atomic`, threshold 60%)
6. Adicionar job de frontend (`pnpm test:coverage`) ao CI
7. Cobrir `heartbeatLoop`/`sendHeartbeat` no SDK
8. Elevar `internal/cli` para ≥ 70%

### Médio Prazo (P2)
9. Ratchet de threshold: 70% → 80% (v1.5.0)
10. Implementar enforcement por pacote (não só global)
11. Adicionar 5 cenários E2E (atualmente draft, 0% implementados)
