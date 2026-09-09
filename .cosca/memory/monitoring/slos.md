# Service Level Objectives — Cosca v1.4.0-dev

> **Owner**: cosca-monitoring (Monitoring Chief)
> **Version**: 1.0.0 | **Status**: active | **Date**: 2026-07-28
> **Quality Gate**: G7 (Performance Baseline — benchmarks sem degradacao >10%)
> **Dependencies**: cosca-performance baseline-report.md (baselines de latencia)

---

## Preambulo

Este documento define os 5 Service Level Objectives (SLOs) para as critical user journeys da plataforma Cosca. Cada SLO e composto por um SLI (metrica concreta), um target (percentual de conformidade), e um error budget (janela mensal de violacoes toleradas).

Os thresholds de latencia sao calibrados com base no [Performance Baseline Report](../performance/baseline-report.md) do cosca-performance, que estabeleceu as primeiras medicoes empiricas da plataforma em 2026-07-28.

---

## Infraestrutura de Metricas Existente

O endpoint `/metrics` (Prometheus) ja esta implementado e exposto na porta `14121` (configuravel via `--metrics-port`). A autenticacao e feita via Bearer token (`COSCA_METRICS_SECRET`).

### Metricas existentes (60+ linhas no formato exposition)

| Categoria | Metricas | Tipo |
|-----------|----------|------|
| **Process** | `cosca_process_goroutines`, `cosca_process_memory_alloc_bytes`, `cosca_process_memory_sys_bytes`, `cosca_process_memory_total_alloc_bytes`, `cosca_process_gc_pause_ns`, `cosca_process_gc_total` | gauge / counter |
| **Runtime** | `cosca_runtime_uptime_seconds`, `cosca_runtime_health`, `cosca_runtime_state_info`, `cosca_runtime_operations_total{operation}`, `cosca_runtime_duration_seconds{operation,quantile}`, `cosca_runtime_goroutines`, `cosca_runtime_component_health{component}` | gauge / counter |
| **Knowledge** | `cosca_knowledge_documents_total`, `cosca_knowledge_chunks_total`, `cosca_knowledge_entities_total`, `cosca_knowledge_vectors_total`, `cosca_knowledge_graph_nodes`, `cosca_knowledge_db_size_bytes`, `cosca_knowledge_embedding_requests_total`, `cosca_knowledge_embedding_tokens_total`, `cosca_knowledge_cache_entries{tier}`, `cosca_knowledge_cache_hits_total{tier}`, `cosca_knowledge_cache_misses_total` | gauge / counter |
| **Memory** | `cosca_memory_records_total`, `cosca_memory_records_size_bytes`, `cosca_memory_layer_records{layer}`, `cosca_memory_layer_size_bytes{layer}` | gauge |
| **HTTP** | `cosca_http_requests_total{method,path,status_code}`, `cosca_http_request_duration_seconds_sum`, `cosca_http_request_duration_seconds_count` | counter |
| **gRPC** | `cosca_grpc_requests_total{method,code}`, `cosca_grpc_request_duration_seconds_sum`, `cosca_grpc_request_duration_seconds_count` | counter |

### Metricas de negocio sugeridas (a implementar)

Estas metricas complementam as existentes para suportar todos os SLOs:

| Metrica | Tipo | Labels | Descricao |
|---------|------|--------|-----------|
| `cosca_tasks_total` | counter | `agent`, `status` | Task routing pelo Kernel para agentes Chief |
| `cosca_task_duration_seconds` | histogram | `agent`, `status` | Latencia de execucao de task por agente |
| `cosca_agent_confidence` | gauge | `agent`, `domain` | Confidence score do modelo de confianca por agente e dominio |
| `cosca_memory_files_total` | gauge | `type` | Distribuicao de arquivos de memoria por tipo (markdown, json, yaml, etc.) |
| `cosca_memory_decay_rate` | gauge | `layer` | Taxa de decay por layer de memoria (curation) |
| `cosca_session_duration_seconds` | histogram | `phase` | Duracao do bootstrap de sessao (Phases 0-4) |
| `cosca_session_bootstrap_total` | counter | `status` | Contagem de bootstraps de sessao (success/fail) |

---

## SLO 1 — Agent Task Routing

### Critical User Journey

Quando o Kernel recebe uma task do usuario, ele roteia para o agente Chief apropriado. A task deve ser despachada e reconhecida pelo agente destino com baixa latencia, garantindo que o sistema de orquestracao nao seja o gargalo.

### SLI

| Atributo | Valor |
|----------|-------|
| **Metrica** | `cosca_task_duration_seconds{status="dispatched"}` — p95 via `histogram_quantile(0.95, ...)` |
| **Unidade** | Segundos |
| **Fonte** | `internal/runtime/metrics.go` — estender com novo histogram `taskDurations` |
| **Endpoint Prometheus** | `/metrics` (porta 14121) |

### SLO Target

| Parametro | Valor |
|-----------|-------|
| **Target** | 99.9% das tasks roteadas em < 500ms (p95) |
| **SLI Window** | 30 dias rolantes |
| **Error Budget** | 0.1% = ~43 minutos/mes de violacao permitida |

### Baseline de Performance

| Operacao | p50 | p95 | p99 | Fonte |
|----------|-----|-----|-----|-------|
| Task routing (estimado) | ~50ms | ~200ms | ~400ms | Estimativa baseada em arquitetura de channels + goroutines |

> **Nota**: Este SLO usa estimativas iniciais. A instrumentacao real (`cosca_task_duration_seconds`) deve ser implementada para validacao. O kernel de orquestracao usa channels Go para dispatch — latencia esperada e sub-milissegundo para o dispatch puro, mas o tempo de ack do agente pode variar.

### Metodo de Medicao

1. **Instrumentacao sugerida**: Adicionar `taskDurations *durationHistogram` ao `runtime.Metrics`, similar aos 4 histogramas existentes (index, search, context, memory).
2. **Coleta**: Prometheus scrape do endpoint `/metrics` a cada 15s.
3. **Consulta PromQL**: `histogram_quantile(0.95, rate(cosca_task_duration_seconds_bucket{status="dispatched"}[30d]))`
4. **Alerting**: Ver [alerting-rules.yml](alerting-rules.yml) — regra `TaskRoutingLatencyHigh`.

### Runbook — SLO Violation

1. **Verificar estado do runtime**: `GET /v1/status` — o runtime esta saudavel?
2. **Verificar fila de tasks**: Ha tasks acumuladas? Verificar `cosca_tasks_total` por agente.
3. **Verificar agentes ativos**: `GET /v1/agents` — todos os agentes Chief estao respondendo?
4. **Verificar goroutines**: `cosca_runtime_goroutines` — ha leaks de goroutines?
5. **Escalar**: Se persistente, notificar cosca-runtime (owner do pipeline de execucao) e cosca-architecture.

---

## SLO 2 — Knowledge Search (FTS5 + Vector)

### Critical User Journey

O usuario submete uma query de busca que combina FTS5 (full-text search) e busca vetorial (similaridade de embeddings). O resultado e retornado com deduplicacao e faceting.

### SLI

| Atributo | Valor |
|----------|-------|
| **Metrica** | `cosca_runtime_duration_seconds{operation="search",quantile="0.95"}` |
| **Unidade** | Segundos |
| **Fonte** | `internal/runtime/metrics.go` — searchDurations histogram (existente) |
| **Endpoint Prometheus** | `/metrics` (porta 14121) |

### SLO Target

| Parametro | Valor |
|-----------|-------|
| **Target** | 99.5% das buscas em < 200ms (p95) |
| **SLI Window** | 30 dias rolantes |
| **Error Budget** | 0.5% = ~216 minutos/mes de violacao permitida |

### Baseline de Performance

Baseado no [Performance Baseline Report](../performance/baseline-report.md):

| Operacao | Dataset | p50 | p95 (est.) | Degradacao +10% |
|----------|---------|-----|------------|-----------------|
| Vector search | 100 vecs | 542 µs | ~600 µs | > 660 µs |
| Vector search | 1,000 vecs | 4.67 ms | ~5.1 ms | > 5.6 ms |
| Vector search | 5,000 vecs | 30.4 ms | ~33.4 ms | > 36.7 ms |
| Vector search | 10,000 vecs | 64.1 ms | ~70.5 ms | > 77.6 ms |
| Graph search | 50 nodes | 33 µs | ~36 µs | > 40 µs |
| Hybrid pipeline | 10 nodes + 10 vecs | 11 µs | ~12 µs | > 13 µs |
| Result dedup | 100 results | 67 µs | ~74 µs | > 81 µs |

> **Nota**: Estes benchmarks medem a search pipeline interna (overhead de codigo Go). A latencia end-to-end inclui tambem embedding generation (chamada externa ao provider LLM) que pode adicionar 50-200ms. O SLO de 200ms cobre o pipeline completo.

### Metodo de Medicao

1. **Instrumentacao**: O `runtime.Metrics.RecordSearchDuration()` ja e chamado pelo knowledge engine durante a busca.
2. **Coleta**: Prometheus scrape a cada 15s.
3. **Consulta PromQL**: `cosca_runtime_duration_seconds{operation="search",quantile="0.95"}`
4. **Alerting**: Ver [alerting-rules.yml](alerting-rules.yml) — regra `SearchLatencyHigh`.

### Runbook — SLO Violation

1. **Verificar volume de vetores**: `cosca_knowledge_vectors_total` — o volume cresceu alem do esperado? Se > 10K vectors, a busca brute-force esta no limite (64ms sem embedding).
2. **Verificar cache**: `cosca_knowledge_cache_hits_total` / `cosca_knowledge_cache_misses_total` — cache hit rate caiu?
3. **Verificar embedding provider**: `cosca_knowledge_embedding_requests_total` — o provider externo esta lento? Timeout?
4. **Verificar schema FTS5**: Bug conhecido em `documents_fts` (schema mismatch) — se nao corrigido, FTS5 document-level search retorna 0 resultados, forcando fallback mais lento.
5. **Escalar**: Se > 5K vectors consistentemente, priorizar P1: ANN index (recomendacao #1 do cosca-performance).

---

## SLO 3 — Session Bootstrap (Phase 0-4)

### Critical User Journey

Quando um usuario inicia uma sessao no Cosca CLI (`cosca chat`), o sistema executa as Phases 0-4 de bootstrap: carregamento de contexto, inicializacao de engines, warm-up de caches, e conexao com providers. O usuario espera que a sessao esteja pronta para interagir rapidamente.

### SLI

| Atributo | Valor |
|----------|-------|
| **Metrica** | `cosca_session_duration_seconds` — p95 via `histogram_quantile(0.95, ...)` |
| **Unidade** | Segundos |
| **Fonte** | Nova metrica a implementar — histogram de bootstrap |
| **Endpoint Prometheus** | `/metrics` (porta 14121) |

### SLO Target

| Parametro | Valor |
|-----------|-------|
| **Target** | 99.0% das sessoes inicializadas em < 5s (p95) |
| **SLI Window** | 7 dias rolantes |
| **Error Budget** | 1.0% = ~100 minutos/semana de violacao permitida |

### Baseline de Performance

| Fase | Operacao | Latencia Estimada | Fonte |
|------|----------|-------------------|-------|
| Phase 0 | Config load + CLI init | ~100ms | Estimativa |
| Phase 1 | SQLite open + WAL init + migration check | ~50ms | SQLite overhead tipico |
| Phase 2 | Knowledge engine init (embedding provider conn) | ~500ms-2s | Depende de rede (provider externo) |
| Phase 3 | Memory engine init + layer load | ~200ms | File-based layers |
| Phase 4 | Context build (session history + project context) | ~500ms | Depende do tamanho do contexto |
| **Total** | | **~1.5-3s** | |

> **Nota**: A latencia da Phase 2 (conexao com provider) e o fator dominante e variavel. Em ambientes offline ou com provider lento, pode exceder 5s. O SLO cobre o caso comum (provider disponivel e rapido).

### Metodo de Medicao

1. **Instrumentacao sugerida**: Adicionar `sessionDurations *durationHistogram` ao `runtime.Metrics`. Marcar inicio no `main()` e fim apos `lifecycle.ExecuteStart()` completar.
2. **Coleta**: Prometheus scrape a cada 15s.
3. **Consulta PromQL**: `histogram_quantile(0.95, rate(cosca_session_duration_seconds_bucket[7d]))`
4. **Alerting**: Ver [alerting-rules.yml](alerting-rules.yml) — regra `SessionBootstrapSlow`.

### Runbook — SLO Violation

1. **Verificar Phase 2 (provider)**: A conexao com o LLM provider esta funcionando? Timeout de rede?
2. **Verificar SQLite**: O banco esta corrompido? WAL file muito grande? Rodar `PRAGMA integrity_check`.
3. **Verificar conhecimento indexado**: `cosca_knowledge_documents_total` — volume excessivo de documentos carregados no bootstrap?
4. **Verificar memoria**: `cosca_memory_records_total` — muitas camadas carregadas no startup?
5. **Escalar**: Se persistente, notificar cosca-runtime (owner do lifecycle) e cosca-provider (owner de conexao com provider).

---

## SLO 4 — API Response (REST)

### Critical User Journey

Toda chamada a API REST da Cosca (endpoints `/v1/*`) deve responder rapidamente. Este SLO cobre a experiencia do usuario/cliente da API, independente da operacao especifica.

### SLI

| Atributo | Valor |
|----------|-------|
| **Metrica** | `cosca_http_request_duration_seconds_sum / cosca_http_request_duration_seconds_count` — p95 agregado por `path` |
| **Unidade** | Segundos |
| **Fonte** | `api/middleware/metrics.go` — MetricsMiddleware (existente) |
| **Endpoint Prometheus** | `/metrics` (porta 14121) |

### SLO Target

| Parametro | Valor |
|-----------|-------|
| **Target** | 99.9% das requisicoes API em < 100ms (p95) |
| **SLI Window** | 30 dias rolantes |
| **Error Budget** | 0.1% = ~43 minutos/mes de violacao permitida |

### Baseline de Performance

| Endpoint | Operacao | Latencia Esperada |
|----------|----------|-------------------|
| `GET /v1/status` | Status check | < 5ms |
| `GET /v1/health` | Health check | < 5ms |
| `POST /v1/knowledge/search` | Busca com embedding | < 200ms (SLO proprio, ver SLO #2) |
| `GET /v1/knowledge/stats` | Dashboard stats (6 queries) | < 15ms |
| `POST /v1/memory/store` | Store memory | < 50ms |
| `GET /v1/memory/search` | PATH-based search | < 50ms (SLO proprio, ver SLO #5) |
| `GET /v1/agents` | List agents | < 10ms |
| `POST /v1/run` | Task execution (dispatch) | < 100ms |

> **Nota**: Endpoints de busca (`/v1/knowledge/search`) e execucao (`/v1/run`) tem SLOs especificos mais relaxados. Para fins do SLO #4, estes endpoints sao excluidos (usar label `path` para filtrar no PromQL).

### Metodo de Medicao

1. **Instrumentacao**: `MetricsMiddleware` ja registra toda request HTTP com method, path, status, e duration.
2. **Coleta**: Prometheus scrape a cada 15s.
3. **Consulta PromQL**: `histogram_quantile(0.95, rate(cosca_http_request_duration_seconds_bucket{path!~"/v1/knowledge/search|/v1/run"}[30d]))`
4. **Alerting**: Ver [alerting-rules.yml](alerting-rules.yml) — regra `APILatencyHigh`.

### Runbook — SLO Violation

1. **Identificar endpoint lento**: `topk(5, cosca_http_request_duration_seconds_sum / cosca_http_request_duration_seconds_count)` — qual path esta lento?
2. **Verificar middleware chain**: Auth (JWT validation), CSRF, rate limiter — algum middleware adicionando latencia?
3. **Verificar GC pauses**: `cosca_process_gc_pause_ns` — GC esta causando pausas longas?
4. **Verificar memoria**: `cosca_process_memory_alloc_bytes` — heap pressure causando GC frequente?
5. **Verificar concorrencia**: `cosca_http_requests_total` — spike de trafego sobrecarregando o server?
6. **Escalar**: Se endpoint especifico, notificar domain Chief. Se sistemico, notificar cosca-backend.

---

## SLO 5 — Memory Retrieval (PATH-based)

### Critical User Journey

Os agentes Cosca usam o sistema de memoria PATH-based (`MemoryEngine`) para armazenar e recuperar aprendizados, contexto, e historico. A recuperacao deve ser rapida para nao bloquear o raciocinio do agente.

### SLI

| Atributo | Valor |
|----------|-------|
| **Metrica** | `cosca_runtime_duration_seconds{operation="memory",quantile="0.95"}` |
| **Unidade** | Segundos |
| **Fonte** | `internal/runtime/metrics.go` — memoryDurations histogram (existente) |
| **Endpoint Prometheus** | `/metrics` (porta 14121) |

### SLO Target

| Parametro | Valor |
|-----------|-------|
| **Target** | 99.9% das recuperacoes de memoria em < 50ms (p95) |
| **SLI Window** | 30 dias rolantes |
| **Error Budget** | 0.1% = ~43 minutos/mes de violacao permitida |

### Baseline de Performance

| Operacao | Latencia Estimada | Notas |
|----------|-------------------|-------|
| PATH read (file-based layer) | < 1ms | Leitura de arquivo em disco |
| PATH search (multiple layers) | < 10ms | Walk + glob + read |
| Memory promote (layer migration) | < 50ms | Write + delete entre layers |
| Curation cycle (decay check) | < 100ms | Batch — nao medido por request |

> **Nota**: O sistema de memoria e baseado em arquivos (filesystem), sem banco de dados. A latencia e dominada por I/O de disco. NVMe/SSD deve manter operacoes em < 1ms.

### Metodo de Medicao

1. **Instrumentacao**: O `runtime.Metrics.RecordMemoryDuration()` ja e chamado pelo MemoryEngine durante operacoes de store/search/get.
2. **Coleta**: Prometheus scrape a cada 15s.
3. **Consulta PromQL**: `cosca_runtime_duration_seconds{operation="memory",quantile="0.95"}`
4. **Alerting**: Ver [alerting-rules.yml](alerting-rules.yml) — regra `MemoryRetrievalLatencyHigh`.

### Runbook — SLO Violation

1. **Verificar volume de records**: `cosca_memory_records_total` — numero excessivo de records em uma layer?
2. **Verificar tamanho por layer**: `cosca_memory_layer_size_bytes` — alguma layer com tamanho desproporcional?
3. **Verificar decay rate**: `cosca_memory_decay_rate` — decay executando durante reads?
4. **Verificar filesystem**: Disco cheio? I/O saturado? Filesystem com high latency?
5. **Verificar camadas**: `cosca_memory_layer_records` — records concentrados em layer lenta (ex: long-term em HDD)?
6. **Escalar**: Se persistente, considerar sharding de layers ou cache em memoria (cosca-cache).

---

## Error Budget Summary

| SLO | Target | Error Budget (mensal) | Severidade da Violacao |
|-----|--------|----------------------|----------------------|
| SLO #1 — Task Routing | 99.9% @ p95 < 500ms | ~43 min | High — afeta orquestracao |
| SLO #2 — Knowledge Search | 99.5% @ p95 < 200ms | ~216 min | High — afeta UX de busca |
| SLO #3 — Session Bootstrap | 99.0% @ p95 < 5s | ~100 min/semana | Medium — afeta onboarding |
| SLO #4 — API Response | 99.9% @ p95 < 100ms | ~43 min | Critical — afeta toda API |
| SLO #5 — Memory Retrieval | 99.9% @ p95 < 50ms | ~43 min | Medium — afeta agentes internos |

### Politica de Error Budget

1. **Burn rate > 1x**: Alerting informativo (warning). Nao requer acao imediata.
2. **Burn rate > 5x**: Alerting critico (critical). Investigation requerida em 24h.
3. **Burn rate > 10x**: Incident response. Freeze de deployments nao-criticos ate resolucao.
4. **Error budget esgotado (>100%)**: Bloqueio de todos os deployments ate que o SLO volte a conformidade (politica de "stop the line").

---

## Metricas de Negocio — Plano de Implementacao

As metricas de negocio sugeridas na tabela da secao de infraestrutura devem ser implementadas em ordem de prioridade:

| Prioridade | Metrica | Bloqueia SLO? | Esforco Estimado | Owner Sugerido |
|------------|---------|---------------|-----------------|----------------|
| **P0** | `cosca_task_duration_seconds` | Sim (SLO #1) | 4h | cosca-runtime |
| **P0** | `cosca_session_duration_seconds` | Sim (SLO #3) | 2h | cosca-runtime |
| **P1** | `cosca_tasks_total` | Parcial (SLO #1) | 2h | cosca-runtime |
| **P1** | `cosca_agent_confidence` | Nao (alerta) | 4h | cosca-ai |
| **P2** | `cosca_memory_files_total` | Nao (dashboard) | 1h | cosca-memory |
| **P2** | `cosca_memory_decay_rate` | Nao (alerta) | 2h | cosca-memory |

---

## Referencias

| Documento | Relacao |
|-----------|---------|
| [Performance Baseline Report](../performance/baseline-report.md) | Baselines de latencia para calibracao dos SLOs |
| [Quality Gates](../qa/quality-gates.md) | G7 (Performance Baseline) |
| [Alerting Rules](alerting-rules.yml) | Regras de alerta Prometheus derivadas dos SLOs |
| [Grafana Dashboard](grafana-dashboard.json) | Dashboard com visualizacao dos SLIs |
| `internal/runtime/metrics.go` | Implementacao dos histogramas e contadores |
| `internal/metrics/prometheus.go` | Formatador Prometheus exposition |
| `internal/cli/serve.go:340` | Handler do endpoint `/metrics` |
| `api/middleware/metrics.go` | Middleware HTTP de coleta de metricas |

---

## Historico

| Versao | Data | Autor | Alteracoes |
|--------|------|-------|-----------|
| 1.0.0 | 2026-07-28 | cosca-monitoring | Criacao inicial: 5 SLOs, SLIs, error budgets, runbooks |

---

> **"You can't improve what you don't measure."** — Peter Drucker
> **Maintainer**: cosca-monitoring (Monitoring Chief)
> **Next review**: Apos implementacao das metricas P0 ou 30 dias, o que ocorrer primeiro.
