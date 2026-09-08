# ADR-046: Database Self-Management — Auto-estira de manutenção contínua

> **Status:** EXPERIMENTO — Fase Protótipo | **Owner:** cosca-kernel | **Last Updated:** 2026-09-08
> **Natureza:** ADR de decisão. Cria o sistema de **auto-gerenciamento de banco de dados** — migração de 40+ bancos fragmentados para 5 bancos domain-driven, com **loop infinito de manutenção** integrado à esteira do Cosca.
> **Abordagem:** PROTOTIPO ISOLADO — construído em `internal/dsms/` como módulo independente. NÃO afeta o sistema atual. Só integra após validação completa.
> **Relação:** complementa **ADR-044** (Learning Vaults), consolida a infraestrutura de dados para suportar **ADR-047** (Intelligence Engine) e o sistema de auto-evolução.

---

## 1. Contexto

### 1.1 Problema Atual

O Cosca acumula **40+ bancos SQLite** em `.cosca/`:

```
BANCOS VETORIAIS (7 arquivos):
  vector-code.db          275 MB
  vector-docs.db           37 MB
  vector-embed-core.db     48 MB
  vector-embed-engines.db  42 MB
  vector-embed-memory.db   46 MB
  vector-fallback.db       12 KB
  vector-opencode.db       14 MB
  vector-other.db          56 MB

BANCOS DE LEARNING (17 arquivos):
  learning/ai.db, architecture.db, backend.db, cache.db,
  cli.db, database.db, devops.db, docs.db, frontend.db,
  kernel.db (2MB), mobile.db, product.db, qa.db, release.db,
  runtime.db, security.db, strategy.db, workflow.db

BANCOS CORE:
  knowledge.db            840 MB  ⚠️ MONSTRO
  graph.db                 56 MB
  projects.db              71 MB
  core.db                   3 MB
  durable.db              400 KB
  tasks.db                  4 KB
  trace.db                 20 KB
  audit.db                  4 KB
  auth_tokens.db            4 KB
  secrets.db                4 KB
  department.db             4 KB
  memory/index.db          49 KB
```

**Total: ~1.5 GB, 40+ arquivos SQLite**

### 1.2 Consequências

1. **Fragmentação**: 7 bancos vetoriais = 7 conexões, 7 WALs, 7 checkpoints
2. **Escalabilidade**: knowledge.db a 840 MB sem política de retenção
3. **Manutenção**: backup = copiar 40+ arquivos, restauração = pesadelo
4. **Performance**: overhead de conexão multiplicado
5. **Evolução**: adicionar feature = novo banco? Schema sem versionamento
6. **Observabilidade**: impossível ter visão unificada do estado dos dados

### 1.3 Direção do Don (2026-09-08)

> "como estruturar bem o banco de dados, nao virar uma bola de neve"
> "integrada direta na esteira pra ter loop infinito"

A solução NÃO é migração pontual. É **sistema de auto-gerenciamento** que roda continuamente na esteira.

### 1.4 Direção do Don — Protótipo Isolado (2026-09-08)

> "isso nao vai afetar sistema agora ne, vai ser criado como experimento, ao se cria numa pasta particular dentro do projeto"

**Abordagem:** Construir o DSMS como **módulo isolado** em `internal/dsms/` antes de integrar ao sistema principal.

```
cosca/
├── internal/
│   ├── dsms/                    ← NOVO: protótipo isolado
│   │   ├── schema/              ← Schemas dos 5 bancos
│   │   ├── migrator/            ← Script de migração
│   │   ├── health/              ← Health check
│   │   ├── compact/             ← Compactação
│   │   ├── archive/             ← Lifecycle de dados
│   │   ├── metrics/             ← Observabilidade
│   │   ├── scheduler/           ← Loop infinito
│   │   └── loop.go              ← Orquestrador
│   ├── search/                  ← EXISTENTE (não modificado)
│   ├── knowledge/               ← EXISTENTE (não modificado)
│   └── ...
├── .cosca/                      ← DADOS ATUAIS (não modificados)
│   ├── knowledge.db             ← MANTIDO
│   ├── vector-*.db              ← MANTIDO
│   └── ...
└── dsms-test/                   ← DADOS DE TESTE (isolados)
    ├── test-knowledge.db        ← Dados de teste
    ├── test-memory.db
    └── test-operations.db
```

**Regras do Protótipo:**
1. **ZERO impacto** no sistema atual
2. **Dados de teste** em diretório separado (`dsms-test/`)
3. **Schemas testados** com dados sintéticos primeiro
4. **Migrator testado** com cópia dos dados reais (backup)
5. **Loop testado** isoladamente antes de integrar
6. **Integração só após** validação completa (gate de qualidade)

---

## 2. Decisão

Criar o **Database Self-Management System (DSMS)** — um subsistema que:

1. **Consolida** os 40+ bancos em 5 bancos domain-driven
2. **Mantém** a saúde dos dados via loop infinito na esteira
3. **Auto-otimiza** compactação, limpeza, indexação
4. **Auto-gerencia** lifecycle de dados (hot → warm → cold → archive)
5. **Auto-repara** corrupção, inconsistências, gaps

### 2.1 Arquitetura Target

```
.cosca/
├── cosca-core.db           (CONFIG + AUTH + SECRETS + SCHEMA_VERSION)
├── cosca-knowledge.db      (CONHECIMENTO CURADO: entries + chunks + embeddings + graph + entities)
├── cosca-memory.db         (MEMÓRIA: sessions + learnings + patterns + failures + decisions + traces)
├── cosca-intelligence.db   (INTELIGÊNCIA: rules + patterns + expert_systems + decision_trees + metrics)
├── cosca-operations.db     (OPERAÇÕES: projects + tasks + agents + skills + plugins)
└── cosca-cache.db          (CACHE: search + llm + computed, TTL-based, max 100MB)
```

### 2.2 Princípios de Design

| Princípio | Implementação |
|-----------|---------------|
| **Um banco por domínio** | 5 bancos, não 40+ |
| **Separação por responsabilidade** | Cada banco = 1 owner claro |
| **Política de retenção** | Cada domínio = regra de lifecycle |
| **Schema versionado** | Tabela `schema_version` em cada banco |
| **Append-only knowledge** | Nunca deletar knowledge, só compactar |
| **Rotacionamento de memória** | 90 dias hot → archive |
| **Cache com TTL** | Entradas expiram, max 100MB |

### 2.3 Schemas Detalhados

#### cosca-core.db

```sql
-- Configuração chave-valor
CREATE TABLE config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tokens de autenticação
CREATE TABLE auth_tokens (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL,
    provider TEXT NOT NULL,
    expires_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Segredos criptografados
CREATE TABLE secrets (
    id TEXT PRIMARY KEY,
    encrypted_value BLOB NOT NULL,
    provider TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Versionamento de schema
CREATE TABLE schema_version (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);
```

#### cosca-knowledge.db

```sql
-- Entradas de conhecimento (118K+)
CREATE TABLE entries (
    id INTEGER PRIMARY KEY,
    content TEXT NOT NULL,
    category TEXT,
    source TEXT,
    confidence REAL DEFAULT 0.5,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Chunks de texto para busca
CREATE TABLE chunks (
    id INTEGER PRIMARY KEY,
    entry_id INTEGER REFERENCES entries(id),
    content TEXT NOT NULL,
    token_count INTEGER,
    chunk_index INTEGER
);

-- Embeddings vetoriais (122K+)
CREATE TABLE embeddings (
    id INTEGER PRIMARY KEY,
    chunk_id INTEGER REFERENCES chunks(id),
    vector BLOB NOT NULL,
    model TEXT,
    dimension INTEGER
);

-- Grafo de conhecimento (122K+ nós)
CREATE TABLE graph_nodes (
    id INTEGER PRIMARY KEY,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    properties JSON,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE graph_edges (
    id INTEGER PRIMARY KEY,
    source_id INTEGER REFERENCES graph_nodes(id),
    target_id INTEGER REFERENCES graph_nodes(id),
    relationship TEXT NOT NULL,
    weight REAL DEFAULT 1.0,
    properties JSON
);

-- Entidades extraídas
CREATE TABLE entities (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    properties JSON,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Índices
CREATE INDEX idx_entries_category ON entries(category);
CREATE INDEX idx_entries_confidence ON entries(confidence);
CREATE INDEX idx_chunks_entry ON chunks(entry_id);
CREATE INDEX idx_embeddings_chunk ON embeddings(chunk_id);
CREATE INDEX idx_graph_edges_source ON graph_edges(source_id);
CREATE INDEX idx_graph_edges_target ON graph_edges(target_id);
```

#### cosca-memory.db

```sql
-- Sessões de trabalho
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    agent TEXT NOT NULL,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    ended_at DATETIME,
    summary TEXT
);

-- Aprendizados por agente
CREATE TABLE learnings (
    id TEXT PRIMARY KEY,
    agent TEXT NOT NULL,
    session_id TEXT REFERENCES sessions(id),
    content TEXT NOT NULL,
    type TEXT NOT NULL,
    confidence REAL DEFAULT 0.5,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Padrões detectados
CREATE TABLE patterns (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    frequency INTEGER DEFAULT 1,
    last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Lições de erro
CREATE TABLE failures (
    id TEXT PRIMARY KEY,
    agent TEXT NOT NULL,
    error TEXT NOT NULL,
    resolution TEXT,
    severity TEXT DEFAULT 'medium',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Decisões tomadas
CREATE TABLE decisions (
    id TEXT PRIMARY KEY,
    context TEXT NOT NULL,
    decision TEXT NOT NULL,
    rationale TEXT,
    outcome TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Trilhas de auditoria
CREATE TABLE traces (
    id TEXT PRIMARY KEY,
    operation TEXT NOT NULL,
    agent TEXT,
    input JSON,
    output JSON,
    duration_ms INTEGER,
    status TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Índices
CREATE INDEX idx_learnings_agent ON learnings(agent);
CREATE INDEX idx_learnings_type ON learnings(type);
CREATE INDEX idx_sessions_agent ON sessions(agent);
CREATE INDEX idx_traces_agent ON traces(agent);
CREATE INDEX idx_traces_operation ON traces(operation);
```

#### cosca-intelligence.db (NOVO — para futuro ADR-047)

```sql
-- Regras determinísticas
CREATE TABLE rules (
    id TEXT PRIMARY KEY,
    domain TEXT NOT NULL,
    name TEXT NOT NULL,
    condition TEXT NOT NULL,  -- JSON logic
    action TEXT NOT NULL,     -- JSON action
    priority INTEGER DEFAULT 0,
    confidence REAL DEFAULT 0.5,
    version INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Padrões de código
CREATE TABLE code_patterns (
    id TEXT PRIMARY KEY,
    language TEXT NOT NULL,
    pattern TEXT NOT NULL,
    description TEXT,
    severity TEXT DEFAULT 'info',
    auto_fix TEXT,  -- JSON fix template
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Sistemas especializados
CREATE TABLE expert_systems (
    id TEXT PRIMARY KEY,
    domain TEXT NOT NULL,
    name TEXT NOT NULL,
    rules JSON NOT NULL,  -- Array of rule IDs
    version INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Árvores de decisão
CREATE TABLE decision_trees (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    tree JSON NOT NULL,  -- Tree structure
    accuracy REAL,
    last_trained DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Métricas de código
CREATE TABLE code_metrics (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    metric_type TEXT NOT NULL,
    value REAL NOT NULL,
    measured_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Índices
CREATE INDEX idx_rules_domain ON rules(domain);
CREATE INDEX idx_code_patterns_language ON code_patterns(language);
CREATE INDEX idx_expert_systems_domain ON expert_systems(domain);
CREATE INDEX idx_code_metrics_file ON code_metrics(file_path);
```

#### cosca-operations.db

```sql
-- Projetos registrados
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    stack JSON,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tarefas/execuções
CREATE TABLE tasks (
    id TEXT PRIMARY KEY,
    project_id TEXT REFERENCES projects(id),
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    input JSON,
    output JSON,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

-- Estado dos agentes
CREATE TABLE agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    status TEXT DEFAULT 'idle',
    last_active DATETIME,
    tasks_completed INTEGER DEFAULT 0
);

-- Skills instalados
CREATE TABLE skills (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT,
    installed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    config JSON
);

-- Plugins ativos
CREATE TABLE plugins (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT DEFAULT 'active',
    config JSON,
    installed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

#### cosca-cache.db

```sql
-- Cache de busca
CREATE TABLE search_cache (
    query_hash TEXT PRIMARY KEY,
    results JSON NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL
);

-- Cache de LLM
CREATE TABLE llm_cache (
    prompt_hash TEXT PRIMARY KEY,
    response TEXT NOT NULL,
    model TEXT,
    tokens_used INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL
);

-- Cache de cálculos
CREATE TABLE computed_cache (
    key TEXT PRIMARY KEY,
    value JSON NOT NULL,
    computation_time_ms INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL
);
```

---

## 3. Loop Infinito de Manutenção (Esteira)

### 3.1 Conceito

O Database Self-Management System (DSMS) roda como **plugin da esteira** em loop infinito:

```
┌─────────────────────────────────────────────────────────────┐
│                    COSCA PIPELINE (ESTEIRA)                  │
│                                                             │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐ │
│  │  Tasks   │──▶│ Search   │──▶│ Context  │──▶│ Execute  │ │
│  └──────────┘   └──────────┘   └──────────┘   └──────────┘ │
│       │                                              │      │
│       │         ┌──────────────────────────────────┐ │      │
│       │         │      DSMS LOOP (INFINITO)        │ │      │
│       │         │                                  │ │      │
│       └────────▶│  1. HEALTH CHECK (a cada 5min)   │◀┘      │
│                 │  2. CONSOLIDATE (a cada 1h)       │        │
│                 │  3. COMPACT (a cada 24h)          │        │
│                 │  4. ARCHIVE (a cada 7d)           │        │
│                 │  5. REPAIR (se anomal detectada)  │        │
│                 │  6. OPTIMIZE (a cada 7d)          │        │
│                 │  7. METRICS (a cada 5min)         │        │
│                 │                                  │        │
│                 └──────────────────────────────────┘        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 Tarefas do Loop

#### Fase 1: HEALTH CHECK (a cada 5 minutos)

```go
// Verifica integridade de todos os bancos
func (d *DSMS) HealthCheck() error {
    for _, db := range d.databases {
        // 1. PRAGMA integrity_check
        // 2. Verificar WAL size
        // 3. Verificar fragmentação
        // 4. Verificar space usage
        // 5. Alertar se anomalia
    }
    return nil
}
```

**Métricas coletadas:**
- Integridade (integrity_check)
- Tamanho do WAL
- Fragmentação (page_count * page_size vs actual_size)
- Espaço livre
- Número de tabelas/índices

#### Fase 2: CONSOLIDATE (a cada 1 hora)

```go
// Consolida dados temporários
func (d *DSMS) Consolidate() error {
    // 1. Migrar dados de bancos legados (se existirem)
    // 2. Merge de chunks duplicados
    // 3. Deduplicação de embeddings
    // 4. Atualizar grafo (merge entidades)
    return nil
}
```

#### Fase 3: COMPACT (a cada 24 horas)

```go
// Compacta bancos
func (d *DSMS) Compact() error {
    for _, db := range d.databases {
        // 1. VACUUM (se fragmentação > 20%)
        // 2. REINDEX (se índices degradados)
        // 3. ANALYZE (atualizar estatísticas)
        // 4. checkpoint WAL
    }
    return nil
}
```

#### Fase 4: ARCHIVE (a cada 7 dias)

```go
// Arquiva dados antigos
func (d *DSMS) Archive() error {
    // 1. Mover sessões > 90 dias para cold storage
    // 2. Mover traces > 30 dias para archive
    // 3. Mover cache expirado para delete
    // 4. Comprimir knowledge entries antigas
    return nil
}
```

#### Fase 5: REPAIR (condicional)

```go
// Repara anomalias detectadas
func (d *DSMS) Repair() error {
    // 1. Reconstruir embeddings corrompidos
    // 2. Reconstruir grafo (nós órfãos)
    // 3. Preencher gaps em schema_version
    // 4. Corrigir foreign keys quebradas
    return nil
}
```

#### Fase 6: OPTIMIZE (a cada 7 dias)

```go
// Otimiza performance
func (d *DSMS) Optimize() error {
    // 1. Criar índices faltantes
    // 2. Remover índices não utilizados
    // 3. Atualizar estatísticas de query planner
    // 4. Otimizar queries lentas (EXPLAIN)
    return nil
}
```

#### Fase 7: METRICS (a cada 5 minutos)

```go
// Coleta métricas para observabilidade
func (d *DSMS) CollectMetrics() error {
    // 1. Tamanho de cada banco
    // 2. Número de registros por tabela
    // 3. Hit rate de cache
    // 4. Latência de queries
    // 5. Exportar para Prometheus/trace
    return nil
}
```

### 3.3 Scheduler

```go
// DSMS Loop — roda infinitamente na esteira
type DSMSLoop struct {
    databases map[string]*sql.DB
    scheduler *Scheduler
    metrics   *MetricsCollector
}

func (l *DSMSLoop) Run(ctx context.Context) error {
    // Cron schedule
    l.scheduler.Every(5).Minutes().Do(l.HealthCheck)
    l.scheduler.Every(1).Hours().Do(l.Consolidate)
    l.scheduler.Every(24).Hours().Do(l.Compact)
    l.scheduler.Every(7).Days().Do(l.Archive)
    l.scheduler.Every(7).Days().Do(l.Optimize)
    l.scheduler.Every(5).Minutes().Do(l.CollectMetrics)

    // Repair sob demanda (quando anomalia detectada)
    l.OnAnomaly(l.Repair)

    return l.scheduler.Run(ctx)
}
```

---

## 4. Políticas de Dados

### 4.1 Lifecycle por Domínio

| Domínio | Hot (acesso freq.) | Warm (acesso raro) | Cold (archive) | Delete |
|---------|--------------------|--------------------|----------------|--------|
| **knowledge** | Atual + 30d | 30d-180d | >180d | Nunca |
| **memory** | 7d | 7d-90d | >90d | >1 ano |
| **intelligence** | Versão atual | Versões anteriores | — | Nunca |
| **operations** | 30d | 30d-90d | >90d | >1 ano |
| **cache** | TTL 1h | — | Expirado | Automático |

### 4.2 Compactação

| Domínio | Trigger | Política |
|---------|---------|----------|
| **knowledge** | Fragmentação > 30% | VACUUM + REINDEX |
| **memory** | Diariamente | DELETE + VACUUM |
| **intelligence** | Por release | VACUUM |
| **operations** | Semanalmente | DELETE archiving + VACUUM |
| **cache** | A cada hora | DELETE expirados |

### 4.3 Backup

| Domínio | Frequência | Retenção |
|---------|------------|----------|
| **core** | Diário | 30 dias |
| **knowledge** | Semanal | 4 semanas |
| **memory** | Semanal | 4 semanas |
| **intelligence** | Por release | Todas |
| **operations** | Semanal | 4 semanas |
| **cache** | Nunca | — |

---

## 5. Migração

### 5.1 Script de Migração

```go
// Migrator — move dados dos 40+ bancos legados para 5 bancos novos
type Migrator struct {
    legacy   map[string]*sql.DB  // Bancos antigos
    target   map[string]*sql.DB  // 5 bancos novos
    progress *MigrationProgress
}

func (m *Migrator) Migrate(ctx context.Context) error {
    // Fase 1: Criar schemas novos
    // Fase 2: Migrar core (config, auth, secrets)
    // Fase 3: Migrar knowledge (entries, chunks, embeddings, graph)
    // Fase 4: Migrar memory (sessions, learnings, patterns)
    // Fase 5: Migrar operations (projects, tasks, agents)
    // Fase 6: Validar integridade
    // Fase 7: Backup dos legados
    // Fase 8: Toggle para novos bancos
    return nil
}
```

### 5.2 Rollback

```go
// Em caso de falha na migração
func (m *Migrator) Rollback() error {
    // 1. Restaurar backup dos bancos legados
    // 2. Reverter config para bancos antigos
    // 3. Log da falha para investigação
    return nil
}
```

### 5.3 Validations

```go
// Após migração, validar:
func (m *Migrator) Validate() error {
    // 1. Contagem de registros (legado vs novo)
    // 2. Integridade referencial
    // 3. Embeddings válidos (dimensão correta)
    // 4. Grafo consistente (sem nós órfãos)
    // 5. Queries de busca retornam mesmos resultados
    return nil
}
```

---

## 6. Observabilidade

### 6.1 Métricas DSMS

```
# Tamanho dos bancos
cosca_db_size_bytes{database="knowledge"} 840085504
cosca_db_size_bytes{database="memory"} 3145728
cosca_db_size_bytes{database="intelligence"} 1048576
cosca_db_size_bytes{database="operations"} 10485760
cossa_db_size_bytes{database="cache"} 10485760

# Registros por tabela
cosca_db_records{database="knowledge", table="entries"} 118304
cosca_db_records{database="knowledge", table="embeddings"} 122786

# Health check
cosca_db_health{database="knowledge"} 1  # 1=ok, 0=degraded
cosca_db_health{database="memory"} 1

# Última compactação
cosca_db_last_compact_timestamp{database="knowledge"} 1694150400

# Hit rate de cache
cosca_db_cache_hit_rate 0.85
```

### 6.2 Alerts

| Alerta | Condição | Severidade |
|--------|----------|------------|
| DB corrupted | integrity_check != 'ok' | CRITICAL |
| WAL too large | WAL > 100MB | WARNING |
| Fragmentation high | > 40% | WARNING |
| Migration failed | migrate_status == 'failed' | CRITICAL |
| Backup missing | last_backup > 7d | WARNING |

---

## 7. Consequências

### 7.1 Trade-offs

| Ganho | Custo |
|-------|-------|
| 40+ bancos → 5 | Migração complexa (1-2 semanas) |
| Backup simples (5 arquivos) | losing granularidade por arquivo |
| Performance (menos conexões) | Refactor de queries existentes |
| Manutenção automática | Complexidade do loop |
| Lifecycle claro | Migração de dados cold |

### 7.2 Riscos

| Risco | Mitigação |
|-------|-----------|
| Dados perdidos na migração | Backup completo + validação |
| Queries quebram | Testes de integração + rollback |
| Loop infinito consome CPU | Throttling + sleep entre fases |
| Cache cresce demais | Max 100MB + TTL agressivo |

### 7.3 Decisões Futuras

Este ADR habilita:
- **ADR-047**: Intelligence Engine (usa `cosca-intelligence.db`)
- **ADR-048**: Auto-Evolution Loop (usa `cosca-memory.db`)
- **ADR-049**: Zero-Dependency Mode (usa dados locais)

---

## 8. Referências

- **ADR-044**: Learning Vaults por Departamento (consolidação de learning/*.db)
- **ADR-035**: Context Compiler (usa knowledge.db)
- **ADR-013**: Modlink Scope (usa graph.db)
- **ADR-031**: Token Efficiency (usa cache.db)
- **WORKFLOWS.md**: database-change workflow

---

## 9. Status

| Fase | Status | Owner |
|------|--------|-------|
| Design (ADR) | ✅ PROPOSTO | cosca-kernel |
| Workflow | ⏳ PENDENTE | cosca-workflow-chief |
| Schema Design | ⏳ PENDENTE | cosca-database |
| Migration Script | ⏳ PENDENTE | cosca-backend |
| DSMS Loop | ⏳ PENDENTE | cosca-devops |
| Tests | ⏳ PENDENTE | cosca-qa |
| Deploy | ⏳ PENDENTE | cosca-release |

---

**Decisão tomada por:** Cosca Kernel (consigliere)
**Aprovada por:** Don (pendente)
**Próximo passo:** Workflow de implementação + schema final + início da Fase 1
