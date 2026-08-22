---
id: benchmark-002
title: "Coverage Evolution — Cosca v1.4.0-dev"
category: coverage
created: 2026-07-29
hardware: "AMD Ryzen 7 5700X3D, 16 threads, Linux amd64"
go_version: "go1.25.0"
source: "memory/audit/coverage-audit-2026-07-29.md, go test -cover"
---

# Coverage Evolution

> **Purpose**: Track testing coverage evolution across the project lifecycle.
> **Metric**: `go test -cover` + function-level analysis
> **Governance Gate**: CI threshold 55% (documentado 70%, target 80% para v1.5.0)

---

## Executive Summary

**Evolução**: 5/100 → 65/100 → 85/100 em 5 dias (2026-07-24 a 2026-07-29). Melhoria de **80 pontos** no score de testes.

---

## Historical Timeline

| Data | Auditoria | Score | Arquivos Teste | Cobertura Core | Contexto |
|------|-----------|-------|----------------|----------------|----------|
| 2026-07-24 | Arquitetural | **5/100** | 2 | ~0% | Projeto recém-iniciado, testes inexistentes |
| 2026-07-25 | Pre-flight | **65/100** | 84 | ~60% | Zero frontend, zero E2E |
| 2026-07-28 | Runtime-Docs sync | **65/100** | 84 | ~60% | Sem reavaliação formal |
| **2026-07-29** | **Coverage Audit** | **85/100** | 18+ | **97.9%** | 6 benchmarks, race detector ativo |

---

## Per-Package Coverage Evolution

### 2026-07-29 Audit Baseline → Post-Audit Improvements

| Pacote | Audit (Jul 29) | Pós-Audit (Jul 29) | Delta | Status |
|--------|---------------|---------------------|-------|--------|
| **internal/runtime** | 97.9% | 97.9% | — | 🟢 Excelente |
| **pkg/cosca** | 75.0% | **80.0%** | +5.0pp | 🟡 Bom (jail.go 0%) |
| **internal/cli** | 46.9% | **71.5%** | +24.6pp | 🟡 Acima do threshold CI |
| **api/rest/handler** | 14.5% | **18.8%** | +4.3pp | 🔴 Crítico (11 handlers expostos) |
| **api/grpcserver** | 71.1% | 71.1% | — | 🟡 Aceitável |

### CLI Coverage Breakthrough (Jul 29)

| Técnica | Descrição | Impacto |
|---------|-----------|---------|
| Extrair-para-testar | `runServe` (699 linhas) → 3 funções extraíveis identificadas | +23pp |
| Funções extraídas | `loadDotEnv` (~15 linhas), `resolveDataDir` (~10 linhas), `configureCORSFromEnv` (~3 linhas) | Cada função = 100% cobertura trivial |
| Refactor runServe | Substitui inline code por chamadas às funções extraídas | 0.5% → 61.8% |
| Teste de integração | Servidor real com temp dir + SIGINT + graceful shutdown | Cobertura de shutdown |
| **Resultado CLI** | **46.9% → 71.5%** | +24.6pp |

### pkg/cosca Coverage Improvements (Jul 29)

| Submódulo | Audit | Pós-Audit | Delta |
|-----------|-------|-----------|-------|
| runtime.go | 87.0% | 87.0% | — |
| knowledge.go | 88.0% | 88.0% | — |
| memory.go | 82.0% | 82.0% | — |
| plugins.go | 85.0% | 85.0% | — |
| discovery.go | 86.0% | 86.0% | — |
| agents.go | 82.0% | 82.0% | — |
| graph.go | 88.0% | 88.0% | — |
| sdk.go | 82.0% | 82.0% | — |
| others | 70-82% | 78-84% | +3-8pp |
| **jail.go** | **0.0%** | **0.0%** 🔴 | **—** |
| **Total pkg/cosca** | **75.0%** | **80.0%** | **+5.0pp** |

### api/rest/handler Coverage (Jul 29)

| Função | Audit | Pós-Audit | Status |
|--------|-------|-----------|--------|
| `NewRuntimeHandler` | 0% | **100%** | ✅ |
| `SetHub` | 0% | **100%** | ✅ |
| `Status` | 0% | **100%** | ✅ |
| `Health` | 0% | **100%** | ✅ |
| `StatusStream` | 0% | **77.8%** | 🟡 (heartbeat/ticker paths uncovered) |
| Outros handlers | 14.5% | 14.5% | 🔴 Ainda não testados |
| **Total handler** | **14.5%** | **18.8%** | +4.3pp |

---

## Runtime Core Detail (97.9%)

### Arquivos e Cobertura Individual

| Arquivo | Linhas | Cobertura | Funções | Abaixo 80% |
|---------|--------|-----------|---------|------------|
| lifecycle.go | 606 | **100.0%** | 24 | 0 |
| metrics.go | 461 | **99.0%** | 32 | 0 |
| state.go | 510 | **98.6%** | 18 | 0 |
| daemon.go | 494 | **98.0%** | 18 | 0 |
| runtime.go | 691 | **97.0%** | 22 | 2 |

### Gaps Identificados (Runtime)

| Função | Cobertura | Causa |
|--------|-----------|-------|
| `Start()` | 78.3% | Caminhos de erro de inicialização de subsistemas não exercitados |
| `Restart()` | 78.6% | Cenários de restart com estado de runtime corrompido não cobertos |

### Critical Gaps Fora do Runtime

| Arquivo | Cobertura | Risco |
|---------|-----------|-------|
| `pkg/cosca/jail.go` | **0.0%** | 🔴 6 funções de isolamento sem cobertura — funcionalidade de segurança mais crítica do sistema |
| `api/rest/handler/*.go` (outros) | **<15%** | 🔴 11 handlers REST expostos com cobertura mínima |
| `pkg/cosca/sdk.go` heartbeat | **42.9%** | 🟡 `heartbeatLoop()` parcialmente coberta, `sendHeartbeat()` a 0% |

---

## Threshold Crisis (Documented vs Reality)

**4 valores diferentes para o mesmo gate de cobertura:**

| Fonte | Threshold | Efetivo? |
|-------|-----------|-----------|
| `Makefile` target `coverage-check` | **40%** | ❌ Não usado no CI |
| `.github/workflows/ci.yml` | **55%** | ✅ Único executado |
| `internal/embed/cosca/memory/qa/quality-gates.md` G5 | **70%** | ❌ Apenas documentação |
| `internal/embed/cosca/QUALITY_GATES.md` Gate 2.5 | **80%** | ❌ Embed |

**Resolução**: Don aprovou elevar CI threshold para 70% e unificar documentação.

---

## Target: v1.5.0

| Meta | Atual | Target |
|------|-------|--------|
| Coverage global | ~78% | ≥ 80% |
| CI gate | 55% | 70% (imediato) → 80% (v1.5.0) |
| Branch coverage | Não medido | ≥ 60% (`-covermode=atomic`) |
| Frontend coverage | 0% (sem CI job) | `pnpm test:coverage` no CI |
| E2E scenarios | 0% (draft) | 5 cenários implementados |
| jail.go coverage | 0% | ≥ 80% |

---

## How to Verify Current Coverage

```bash
# Global
go test -cover ./...

# Per-package (core)
go test -cover ./internal/runtime/
go test -cover ./pkg/cosca/
go test -cover ./internal/cli/
go test -cover ./api/rest/handler/
go test -cover ./api/grpcserver/

# With race detection
go test -race -cover ./internal/runtime/
```

---

*Coverage evolution tracked by cosca-performance. Sources: coverage audit 2026-07-29, go test -cover, kernel learnings L18-L19.*
