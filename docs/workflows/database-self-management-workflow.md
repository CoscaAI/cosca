# WORKFLOW: Database Self-Management — Implementação

> **ADR:** ADR-046 | **Owner:** cosca-kernel | **Início:** 2026-09-08
> **Objetivo:** Migrar 40+ bancos SQLite para 5 bancos domain-driven + DSMS Loop infinito
> **⚠️ MODO EXPERIMENTO:** Protótipo isolado em `internal/dsms/`. NÃO afeta sistema atual.

---

## Visão Geral

```
┌─────────────────────────────────────────────────────────────────┐
│                    FASES DE IMPLEMENTAÇÃO                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  FASE 1          FASE 2          FASE 3          FASE 4        │
│  ┌─────┐         ┌─────┐         ┌─────┐         ┌─────┐       │
│  │SCHEMA│────────▶│MIGRA│────────▶│DSMS │────────▶│TEST │       │
│  │DESIGN│         │TION │         │LOOP │         │+DEPL│       │
│  └─────┘         └─────┘         └─────┘         └─────┘       │
│                                                                 │
│  3-5 dias        5-7 dias        7-10 dias       3-5 dias      │
│                                                                 │
│  Total estimado: 18-27 dias (3-4 semanas)                      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## ⚠️ Regras do Protótipo Isolado

**IMPORTANTE:** Este workflow é um EXPERIMENTO. O sistema atual NÃO será modificado.

### Diretório de Trabalho

```
cosca/
├── internal/dsms/           ← CÓDIGO do protótipo
│   ├── schema/              ← Schemas SQL
│   ├── migrator/            ← Lógica de migração
│   ├── health/              ← Health check
│   ├── compact/             ← Compactação
│   ├── archive/             ← Lifecycle
│   ├── metrics/             ← Observabilidade
│   ├── scheduler/           ← Loop infinito
│   └── loop.go              ← Orquestrador
│
└── dsms-test/               ← DADOS DE TESTE (isolados)
    ├── test-core.db         ← Banco de teste
    ├── test-knowledge.db    ← Dados sintéticos
    ├── test-memory.db       ← Dados sintéticos
    └── test-operations.db   ← Dados sintéticos
```

### Regras

1. **ZERO impacto** — nada no `.cosca/` ou sistema atual é modificado
2. **Dados sintéticos** — testar com dados gerados, não cópia dos reais
3. **Backup primeiro** — só copiar dados reais após backup completo
4. **Validação rigorosa** — todos os testes passam antes de integrar
5. **Gate de qualidade** — Don aprova antes de qualquer integração
6. **Rollback pronto** — sempre ter caminho de volta

### Ciclo de Vida do Protótipo

```
DESENVOLVIMENTO → TESTE → VALIDAÇÃO → APROVAÇÃO → INTEGRAÇÃO
      │               │          │            │           │
      ▼               ▼          ▼            ▼           ▼
   Código isolado  Dados     Benchmark    Don aprova  Sistema
   em internal/dsms/ sintéticos vs real   integra     atualizado
```

---

## FASE 1: Schema Design (3-5 dias)

### 1.1 Preparação

**Owner:** cosca-database
**Dependências:** Nenhuma

**Tarefas:**
- [ ] Revisar ADR-046 com o time
- [ ] Validar schemas com dados reais ( amostra de 1000 registros)
- [ ] Definir tipos de dados exatos (JSON vs BLOB vs TEXT)
- [ ] Definir política de NULLs e defaults
- [ ] Criar migration scripts SQL

**Saídas:**
- [ ] `migrations/001_create_core.sql`
- [ ] `migrations/002_create_knowledge.sql`
- [ ] `migrations/003_create_memory.sql`
- [ ] `migrations/004_create_intelligence.sql`
- [ ] `migrations/005_create_operations.sql`
- [ ] `migrations/006_create_cache.sql`

### 1.2 Validação de Schema

**Owner:** cosca-database
**Dependências:** 1.1

**Tarefas:**
- [ ] Criar bancos de teste com schemas
- [ ] Inserir dados de teste (10K registros)
- [ ] Validar queries de busca existentes
- [ ] Testar performance (queries < 10ms)
- [ ] Validar integridade referencial
- [ ] Testar com SQLite WAL mode

**Saídas:**
- [ ] `schema_validation_test.go`
- [ ] Benchmark results
- [ ] Approvação do Don

---

## FASE 2: Migration Script (5-7 dias)

### 2.1 Migrator Core

**Owner:** cosca-backend
**Dependências:** 1.2

**Tarefas:**
- [ ] Criar `internal/dsms/migrator.go`
- [ ] Implementar detecção de bancos legados
- [ ] Implementar leitura de schemas antigos
- [ ] Implementar escrita em schemas novos
- [ ] Implementar progress tracking
- [ ] Implementar rollback automático

**Saídas:**
- [ ] `internal/dsms/migrator.go`
- [ ] `internal/dsms/migrator_test.go`

### 2.2 Migrations Específicas

**Owner:** cosca-backend
**Dependências:** 2.1

**Tarefas:**
- [ ] `migrateCore()` — config, auth, secrets
- [ ] `migrateKnowledge()` — entries, chunks, embeddings, graph
- [ ] `migrateMemory()` — sessions, learnings, patterns, failures
- [ ] `migrateOperations()` — projects, tasks, agents
- [ ] `migrateCache()` — search_cache, llm_cache (ignorar legado)
- [ ] `migrateIntelligence()` — criar vazio (novo)

**Saídas:**
- [ ] `internal/dsms/migrations.go`
- [ ] `internal/dsms/migrations_test.go`

### 2.3 Validação de Migração

**Owner:** cosca-qa
**Dependências:** 2.2

**Tarefas:**
- [ ] Testar migração com dados reais (backup)
- [ ] Validar contagem de registros
- [ ] Validar integridade referencial
- [ ] Validar embeddings (dimensão, normalização)
- [ ] Validar grafo (nós, arestas, sem órfãos)
- [ ] Testar rollback completo
- [ ] Benchmark de performance pós-migração

**Saídas:**
- [ ] `internal/dsms/migration_validation_test.go`
- [ ] Relatório de validação
- [ ] Approvação do Don

---

## FASE 3: DSMS Loop (7-10 dias)

### 3.1 Health Check

**Owner:** cosca-devops
**Dependências:** 2.3

**Tarefas:**
- [ ] Criar `internal/dsms/health.go`
- [ ] Implementar `integrity_check` para cada banco
- [ ] Implementar verificação de WAL size
- [ ] Implementar verificação de fragmentação
- [ ] Implementar verificação de space usage
- [ ] Implementar alerting (log + metrics)

**Saídas:**
- [ ] `internal/dsms/health.go`
- [ ] `internal/dsms/health_test.go`

### 3.2 Compact & Optimize

**Owner:** cosca-devops
**Dependências:** 3.1

**Tarefas:**
- [ ] Criar `internal/dsms/compact.go`
- [ ] Implementar VACUUM condicional (fragmentação > 20%)
- [ ] Implementar REINDEX condicional
- [ ] Implementar ANALYZE (atualizar estatísticas)
- [ ] Implementar WAL checkpoint
- [ ] Criar `internal/dsms/optimize.go`
- [ ] Implementar criação de índices faltantes
- [ ] Implementar remoção de índices não utilizados
- [ ] Implementar otimização de queries lentas

**Saídas:**
- [ ] `internal/dsms/compact.go`
- [ ] `internal/dsms/optimize.go`
- [ ] `internal/dsms/compact_test.go`
- [ ] `internal/dsms/optimize_test.go`

### 3.3 Archive & Cleanup

**Owner:** cosca-devops
**Dependências:** 3.2

**Tarefas:**
- [ ] Criar `internal/dsms/archive.go`
- [ ] Implementar migração sessões > 90d para cold
- [ ] Implementar migração traces > 30d para archive
- [ ] Implementar delete de cache expirado
- [ ] Implementar compressão de knowledge entries antigas
- [ ] Criar `internal/dsms/cleanup.go`
- [ ] Implementar delete de dados expirados
- [ ] Implementar VACUUM pós-cleanup

**Saídas:**
- [ ] `internal/dsms/archive.go`
- [ ] `internal/dsms/cleanup.go`
- [ ] `internal/dsms/archive_test.go`
- [ ] `internal/dsms/cleanup_test.go`

### 3.4 Metrics & Observability

**Owner:** cosca-monitoring
**Dependências:** 3.3

**Tarefas:**
- [ ] Criar `internal/dsms/metrics.go`
- [ ] Implementar coleta de tamanho de bancos
- [ ] Implementar contagem de registros
- [ ] Implementar hit rate de cache
- [ ] Implementar latência de queries
- [ ] Integrar com Prometheus (ou trace interno)
- [ ] Criar dashboards de observabilidade

**Saídas:**
- [ ] `internal/dsms/metrics.go`
- [ ] `internal/dsms/metrics_test.go`
- [ ] Dashboard de database health

### 3.5 Scheduler (Loop Infinito)

**Owner:** cosca-devops
**Dependências:** 3.4

**Tarefas:**
- [ ] Criar `internal/dsms/scheduler.go`
- [ ] Implementar cron schedule (5min, 1h, 24h, 7d)
- [ ] Implementar throttling (não sobrecarregar CPU)
- [ ] Implementar graceful shutdown
- [ ] Implementar logging de todas as operações
- [ ] Integrar com plugin system da esteira
- [ ] Criar `internal/dsms/loop.go` (orchestra tudo)

**Saídas:**
- [ ] `internal/dsms/scheduler.go`
- [ ] `internal/dsms/loop.go`
- [ ] `internal/dsms/loop_test.go`

---

## FASE 4: Tests + Deploy (3-5 dias)

### 4.1 Testes de Integração

**Owner:** cosca-qa
**Dependências:** 3.5

**Tarefas:**
- [ ] Teste completo de migração (legado → novo)
- [ ] Teste de DSMS loop (health + compact + archive)
- [ ] Teste de rollback completo
- [ ] Teste de performance (antes vs depois)
- [ ] Teste de concorrência (múltiplos writers)
- [ ] Teste de crash recovery
- [ ] Teste de backup/restore

**Saídas:**
- [ ] `internal/dsms/integration_test.go`
- [ ] Relatório de testes
- [ ] Coverage > 80%

### 4.2 Deploy

**Owner:** cosca-release
**Dependências:** 4.1

**Tarefas:**
- [ ] Criar release branch
- [ ] Atualizar CHANGELOG
- [ ] Criar migration script de deploy
- [ ] Testar deploy em staging
- [ ] Criar rollback script
- [ ] Documentar procedure de deploy
- [ ] Deploy em produção

**Saídas:**
- [ ] Release v1.6.0
- [ ] Deploy script
- [ ] Rollback script
- [ ] Runbook de operações

---

## Gate de Qualidade

### Pre-Commit
- [ ] `go vet ./...` limpo
- [ ] `go test ./internal/dsms/...` passa
- [ ] `go build ./...` compila
- [ ] Schema validation OK
- [ ] Migration test OK

### Pre-Deploy
- [ ] Todos os testes de integração passam
- [ ] Performance benchmark OK (queries < 10ms)
- [ ] Backup completo feito
- [ ] Rollback testado
- [ ] Don aprova

### Post-Deploy
- [ ] Health check OK em todos os bancos
- [ ] DSMS loop rodando
- [ ] Métricas coletadas
- [ ] Sem erros nos logs
- [ ] Alertas configurados

---

## Riscos e Mitigações

| Risco | Impacto | Mitigação |
|-------|---------|-----------|
| Dados perdidos na migração | CRITICAL | Backup completo + validação + rollback |
| Queries quebram | HIGH | Testes de integração + migration test |
| Loop infinito consome CPU | MEDIUM | Throttling + sleep entre fases |
| Cache cresce demais | LOW | Max 100MB + TTL agressivo |
| Performance degrada | MEDIUM | Benchmark antes vs depois |
| Deploy falha | HIGH | Rollback script + staging test |

---

## Timeline

```
Semana 1:  FASE 1 (Schema Design) ─────────────────────────┐
Semana 2:  FASE 2 (Migration Script) ──────────────────────┤
Semana 3:  FASE 3 (DSMS Loop) ─────────────────────────────┤
Semana 4:  FASE 4 (Tests + Deploy) ────────────────────────┘
                                                          │
                                                    Deploy v1.6.0
```

---

## Aprovações

| Fase | Aprovador | Status |
|------|-----------|--------|
| FASE 1 | cosca-database + Don | ⏳ PENDENTE |
| FASE 2 | cosca-backend + cosca-qa | ⏳ PENDENTE |
| FASE 3 | cosca-devops + cosca-monitoring | ⏳ PENDENTE |
| FASE 4 | cosca-release + Don | ⏳ PENDENTE |

---

**Workflow criado por:** Cosca Kernel
**Aprovado por:** Don (pendente)
**Próximo passo:** Iniciar FASE 1 — Schema Design
