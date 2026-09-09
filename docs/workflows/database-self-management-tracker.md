# TRACKER: Database Self-Management

> **ADR:** ADR-046 | **Workflow:** database-self-management-workflow.md
> **Início:** 2026-09-08 | **Fim Estimado:** 2026-09-29 (3 semanas)
> **⚠️ MODO EXPERIMENTO:** Protótipo isolado em `internal/dsms/`. NÃO afeta sistema atual.

---

## Progresso Geral

```
FASE 1: Schema Design      [██████████] 100%  (feito: 6 schemas SQL + storage + migrate_all)
FASE 2: Migration Script   [░░░░░░░░░░] 0%    (NÃO IMPLEMENTADO: migrator ausente)
FASE 3: DSMS Loop          [██████████] ~70%  (health/compact/archive/metrics/scheduler/loop existem)
FASE 4: Tests + Deploy     [░░░░░░░░░░] ~10%  (intelligence testado; stubs do loop sem teste)

TOTAL:                      [█████░░░░░] ~45%  (loop funcional; migrador = buraco central)
```

---

## ⚠️ Status do Protótipo (ESTADO REAL medido 2026-09-09)

| Item | Status | Observação |
|------|--------|------------|
| Diretório criado | ✅ FEITO | `internal/dsms/` (módulo Go próprio, isolado) |
| Schemas SQL | ✅ FEITO | 6 schemas (`schema/001_core.sql` … `006_cache.sql` + `migrate_all.sql`) |
| Código Go (loop) | ✅ FEITO | `dsms.go`, `storage`, `health`, `compact`, `archive`, `metrics`, `scheduler`, `loop`, `oracle`, `cli`, `cmd/dsms` |
| Subsistema intelligence | ✅ FEITO + testado | `scanner`, `trainer`, `expander`, `distill`, `expert`, `pipeline`, `codeanalyzer`, `csnparser`, `rules` |
| Migrator (Fase 2) | ❌ **NÃO EXISTE** | BURACO CENTRAL: nenhum `migrator.go`; 40+ bancos → 5 NÃO foi implementado |
| Testes unitários | ⚠️ PARCIAL | intelligence 72-95% cobertura; stubs (archive/compact/health/metrics/loop) = **0%** |
| Dados de teste | ❌ NÃO EXISTE | `dsms-test/` previsto no ADR, ausente |
| Benchmark | ❌ PENDENTE | vs sistema atual |
| Validação Don | ❌ PENDENTE | Gate de qualidade |

**Regra:** Nada é integrado ao sistema atual até validação completa.
**Estado:** build ✅ · vet ✅ · `go test ./...` ✅ (14 pacotes ok) · **`trainer.go` tinha BOM que quebrava `-cover` → CORRIGIDO.**

---

## FASE 1: Schema Design

### 1.1 Preparação
- [ ] Revisar ADR-046 com o time
- [ ] Validar schemas com dados reais (1000 registros)
- [ ] Definir tipos de dados exatos
- [ ] Definir política de NULLs e defaults
- [ ] Criar migration scripts SQL

**Owner:** cosca-database
**Status:** ⏳ PENDENTE
**Bloqueado por:** Nada
**Bloqueando:** 1.2

### 1.2 Validação de Schema
- [ ] Criar bancos de teste com schemas
- [ ] Inserir dados de teste (10K registros)
- [ ] Validar queries de busca existentes
- [ ] Testar performance (queries < 10ms)
- [ ] Validar integridade referencial
- [ ] Testar com SQLite WAL mode

**Owner:** cosca-database
**Status:** ⏳ PENDENTE
**Bloqueado por:** 1.1
**Bloqueando:** 2.1

---

## FASE 2: Migration Script

> ⚠️ **BURACO CENTRAL:** Nenhum arquivo de migração existe em `internal/dsms/`.
> Este é o núcleo do DSMS (consolidar 40+ bancos → 5). **Precisa ser construído.**
> Owner: cosca-backend | Status: ❌ NÃO INICIADO

### 2.0 Criar o pacote migrator (PRÉ-REQUISITO)
- [ ] Criar `internal/dsms/migrator/` (pacote) + `migrator.go`
- [ ] Criar `internal/dsms/migrations/` (migrações por domínio)
- [ ] Criar `internal/dsms/migrator/migrator_test.go`

### 2.1 Migrator Core
- [ ] Criar `internal/dsms/migrator.go`
- [ ] Implementar detecção de bancos legados
- [ ] Implementar leitura de schemas antigos
- [ ] Implementar escrita em schemas novos
- [ ] Implementar progress tracking
- [ ] Implementar rollback automático

**Owner:** cosca-backend
**Status:** ⏳ PENDENTE
**Bloqueado por:** 1.2
**Bloqueando:** 2.2

### 2.2 Migrations Específicas
- [ ] `migrateCore()` — config, auth, secrets
- [ ] `migrateKnowledge()` — entries, chunks, embeddings, graph
- [ ] `migrateMemory()` — sessions, learnings, patterns, failures
- [ ] `migrateOperations()` — projects, tasks, agents
- [ ] `migrateCache()` — search_cache, llm_cache
- [ ] `migrateIntelligence()` — criar vazio

**Owner:** cosca-backend
**Status:** ⏳ PENDENTE
**Bloqueado por:** 2.1
**Bloqueando:** 2.3

### 2.3 Validação de Migração
- [ ] Testar migração com dados reais
- [ ] Validar contagem de registros
- [ ] Validar integridade referencial
- [ ] Validar embeddings
- [ ] Validar grafo
- [ ] Testar rollback completo
- [ ] Benchmark de performance

**Owner:** cosca-qa
**Status:** ⏳ PENDENTE
**Bloqueado por:** 2.2
**Bloqueando:** 3.1

---

## FASE 3: DSMS Loop

### 3.1 Health Check
- [ ] Criar `internal/dsms/health.go`
- [ ] Implementar `integrity_check`
- [ ] Implementar verificação de WAL size
- [ ] Implementar verificação de fragmentação
- [ ] Implementar verificação de space usage
- [ ] Implementar alerting

**Owner:** cosca-devops
**Status:** ⏳ PENDENTE
**Bloqueado por:** 2.3
**Bloqueando:** 3.2

### 3.2 Compact & Optimize
- [ ] Criar `internal/dsms/compact.go`
- [ ] Implementar VACUUM condicional
- [ ] Implementar REINDEX condicional
- [ ] Implementar ANALYZE
- [ ] Implementar WAL checkpoint
- [ ] Criar `internal/dsms/optimize.go`
- [ ] Implementar criação de índices
- [ ] Implementar remoção de índices
- [ ] Implementar otimização de queries

**Owner:** cosca-devops
**Status:** ⏳ PENDENTE
**Bloqueado por:** 3.1
**Bloqueando:** 3.3

### 3.3 Archive & Cleanup
- [ ] Criar `internal/dsms/archive.go`
- [ ] Implementar migração sessões > 90d
- [ ] Implementar migração traces > 30d
- [ ] Implementar delete de cache expirado
- [ ] Implementar compressão knowledge
- [ ] Criar `internal/dsms/cleanup.go`
- [ ] Implementar delete de dados expirados
- [ ] Implementar VACUUM pós-cleanup

**Owner:** cosca-devops
**Status:** ⏳ PENDENTE
**Bloqueado por:** 3.2
**Bloqueando:** 3.4

### 3.4 Metrics & Observability
- [ ] Criar `internal/dsms/metrics.go`
- [ ] Implementar coleta de tamanho
- [ ] Implementar contagem de registros
- [ ] Implementar hit rate de cache
- [ ] Implementar latência de queries
- [ ] Integrar com Prometheus
- [ ] Criar dashboards

**Owner:** cosca-monitoring
**Status:** ⏳ PENDENTE
**Bloqueado por:** 3.3
**Bloqueando:** 3.5

### 3.5 Scheduler (Loop Infinito)
- [ ] Criar `internal/dsms/scheduler.go`
- [ ] Implementar cron schedule
- [ ] Implementar throttling
- [ ] Implementar graceful shutdown
- [ ] Implementar logging
- [ ] Integrar com plugin system
- [ ] Criar `internal/dsms/loop.go`

**Owner:** cosca-devops
**Status:** ⏳ PENDENTE
**Bloqueado por:** 3.4
**Bloqueando:** 4.1

---

## FASE 4: Tests + Deploy

### 4.1 Testes de Integração
- [ ] Teste completo de migração
- [ ] Teste de DSMS loop
- [ ] Teste de rollback completo
- [ ] Teste de performance
- [ ] Teste de concorrência
- [ ] Teste de crash recovery
- [ ] Teste de backup/restore

**Owner:** cosca-qa
**Status:** ⏳ PENDENTE
**Bloqueado por:** 3.5
**Bloqueando:** 4.2

### 4.2 Deploy
- [ ] Criar release branch
- [ ] Atualizar CHANGELOG
- [ ] Criar migration script de deploy
- [ ] Testar deploy em staging
- [ ] Criar rollback script
- [ ] Documentar procedure
- [ ] Deploy em produção

**Owner:** cosca-release
**Status:** ⏳ PENDENTE
**Bloqueado por:** 4.1
**Bloqueando:** Nada

---

## Bloqueios Ativos

| ID | Descrição | Owner | Desde |
|----|-----------|-------|-------|
| — | Nenhum bloqueio ativo | — | — |

---

## Decisões Tomadas

| Data | Decisão | Tomada por |
|------|---------|------------|
| 2026-09-08 | 5 bancos domain-driven, não 40+ | cosca-kernel + Don |
| 2026-09-08 | DSMS loop infinito na esteira | Don |
| 2026-09-08 | cosca-intelligence.db como banco novo (futuro ADR-047) | cosca-kernel |

---

## Métricas de Progresso

| Métrica | Valor |
|---------|-------|
| Tarefas totais | 87 |
| Tarefas concluídas | ~39 (Fase 1 completa + Fase 3 parcial + intelligence) |
| Tarefas em andamento | 0 |
| Tarefas pendentes | ~48 (Fase 2 migrator ausente + testes stubs + deploy) |
| Bloqueios | 1 (migrator não iniciado — buraco central) |
| Dias decorridos | 1 (2026-09-08 → 09) |
| Dias restantes estimados | 18-27 (Fase 2 + 4 pendentes) |

---

**Tracker atualizado por:** cosca-kernel
**Última atualização:** 2026-09-09 (estado real medido: build ✅ / vet ✅ / teste ok / trainer BOM corrigido)
