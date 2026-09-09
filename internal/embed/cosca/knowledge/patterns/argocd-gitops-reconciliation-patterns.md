# Argo CD GitOps & Reconciliation Patterns

> **Source**: https://github.com/argoproj/argo-cd — Apache 2.0, Go 1.26, CNCF Graduated
> **Analyzed**: 2026-08-09 — Análise profunda cross-agent (3 agentes em paralelo)
> **Confidence**: 0.94 (validado por leitura direta: appcontroller.go 3.081 linhas, sync_context.go 1.876 linhas, diff.go 1.272 linhas)

## Intent

Extrair padrões reutilizáveis da plataforma GitOps de referência — reconciliação declarativa, diff three-way, sync engine com phases/waves, health assessment, geração por templates, notificações trigger-based. Aplicação direta na esteira Cosca (L153) e no sistema de deploy.

## Context

Argo CD é a ferramenta GitOps CNCF Graduated que implementa deploy declarativo contínuo para Kubernetes:
- **Go 1.26** — monorepo com ~52 diretórios de domínio
- **Controller**: reconciliation loop duplo (status + operations) com informers e workqueues rate-limited
- **GitOps Engine** (módulo separado): diff (3 estratégias), sync (phases/waves), health (built-in + Lua)
- **Repo Server**: geração de manifests (Helm, Kustomize, YAML, CMP plugins) via gRPC
- **ApplicationSet**: geração de Applications por templates com 8 tipos de generators
- **Multi-cluster**: cache por cluster, sharding por cluster hash, credenciais em Secrets
- **Notifications**: modelo trigger/template/service com 8 triggers built-in

## Patterns Extraídos (Top 12 para Cosca)

### 1. Double-Loop Reconciliation (Status + Operations)

**O que é**: Dois loops de reconciliação independentes rodando em goroutine pools separadas. O **status loop** (`processAppRefreshQueueItem`) compara estado desejado vs live continuamente — detecta drift, avalia health, dispara auto-sync. O **operation loop** (`processAppOperationQueueItem`) executa a sincronização propriamente dita — fases, waves, hooks, retry.

**Por que dois loops?**: O status loop é rápido e mantém o estado sempre atualizado. O operation loop pode ser lento (sync de centenas de recursos) sem bloquear a detecção de drift. Se o controller cair durante um sync, o operation state é checkpointed e reassumido.

**5 workqueues** rate-limited com batching de 5s para absorver bursts:
- `appRefreshQueue` — status reconciliation triggers
- `appComparisonTypeRefreshQueue` — deferred refresh
- `appOperationQueue` — sync operation processing
- `projectRefreshQueue` — projeto lifecycle
- `appHydrateQueue` — manifest generation

**Por que importa para Cosca**: Nossa esteira (L153) tem um loop só. Separar status (comparar estado desejado vs atual) de operations (executar passos) permite: status sempre fresco, operações longas sem bloquear detecção de drift, checkpoint de operação para retomar após crash.

**Arquivos-chave**: `controller/appcontroller.go` (3.081 linhas), `controller/state.go` (1.396 linhas), `controller/sync.go` (774 linhas)

---

### 2. Three-Tier Diff Strategy

**O que é**: Três estratégias de diff tentadas em ordem de precisão: (1) **Server-Side Diff** — `kubectl apply --dry-run=server`, mais preciso porque considera defaulting e webhooks; (2) **Structured Merge Diff** — emulação client-side do SSA usando `structured-merge-diff` library; (3) **Three-Way Diff** — strategic merge patch com `last-applied-configuration`; fallback para two-way.

**Normalização pré-diff**: Normaliza creationTimestamp, Secrets (stringData→data), RBAC (rules vazias→nil), Endpoints (ordena subsets). User-configured `ignoreDifferences` (JSON pointers) também aplicado.

**Resultado**: `DiffResult{Modified bool, NormalizedLive []byte, PredictedLive []byte}` — comparação é `bytes.Equal()` simples após normalização.

**Por que importa para Cosca**: Nosso workflow engine não tem diff — compara estado desejado vs atual de forma ad-hoc. O padrão de três estratégias em cascata (mais precisa → menos precisa) garante que sempre há um diff, mesmo sem acesso ao servidor. A normalização pré-diff é crucial para evitar falsos positivos.

**Arquivos-chave**: `gitops-engine/pkg/diff/diff.go` (1.272 linhas), `gitops-engine/pkg/diff/diff_options.go`

---

### 3. Phased + Waved Sync Execution com Dry-Run Gate

**O que é**: Sync executado em 4 fases ordenadas (PreSync→Sync→PostSync, com SyncFail em falha) e dentro de cada fase, recursos ordenados por: wave (annotation do usuário) → kind (ordenação built-in: Namespace primeiro, CRDs antes de CRs) → nome (lexicográfico). **Dry-run gate**: antes de qualquer mutação, todos os manifests são validados via `kubectl apply --dry-run=client`.

**Execução**: Tasks dentro do mesmo kind group executam em paralelo via goroutines + `sync.WaitGroup`. Delay configurável entre waves (default 2s). Pruning em ordem reversa de wave.

**SyncContext.Sync()**: Máquina de estados iterativa — cada chamada avança um passo. Não é uma função bloqueante. O controller chama repetidamente até `Succeeded`/`Failed`.

**Por que importa para Cosca**: Nosso StepRunner executa steps sequencialmente, sem fases, sem waves, sem dry-run. O padrão de fases + waves com ordenação por tipo + dry-run gate garante: (a) hooks antes/depois da sincronização principal, (b) ordem controlada de deploy, (c) validação pré-execução sem efeitos colaterais. A máquina de estados iterativa (não bloqueante) permite checkpoint entre steps.

**Arquivos-chave**: `gitops-engine/pkg/sync/sync_context.go` (1.876 linhas), `gitops-engine/pkg/sync/sync_tasks.go`, `gitops-engine/pkg/sync/common/types.go`

---

### 4. Event-Driven + Polling Hybrid com Informer Pattern

**O que é**: Mudanças são detectadas via dois mecanismos: (1) **Kubernetes informers** — watch em Application CRDs e recursos dos clusters gerenciados, com handlers que enfileiram no workqueue; (2) **Polling periódico** — `statusRefreshTimeout` (~3min) força reavaliação completa. O cache de recursos usa watch do K8S API para manter estado live em memória, populado via list inicial + mantido por watch events.

**Batching**: Mudanças durante sync são coalescidas com delay de 5s (`appOperationRequeueDelay`) para evitar thundering herd.

**Cache Invalidation**: Config changes → `invalidate()`, cluster credential changes → `clusterCache.Invalidate()`, cluster delete → remove cache entry.

**Por que importa para Cosca**: Nossa detecção de mudanças é reativa (Don dá ordem) ou via file watcher. O padrão informer + workqueue + polling híbrido permitiria: agentes reagirem a eventos de conhecimento (novo pattern registrado, learning atualizado), com fallback de polling para garantir que nada é perdido. O batching de 5s absorve bursts.

**Arquivos-chave**: `controller/appcontroller.go` (informer setup), `controller/cache/cache.go` (931 linhas), `gitops-engine/pkg/cache/cluster.go`

---

### 5. Health Assessment Model (Built-in + Lua Custom)

**O que é**: Sistema de health assessment em duas camadas: (1) **Built-in checks** para tipos conhecidos de Kubernetes (Deployment, StatefulSet, Pod, Job, Service, PVC, etc.) — cada um com lógica específica de condições; (2) **Lua scripts customizados** em `resource_customizations/{apiGroup}/{kind}/health.lua` — 81+ scripts para CRDs de terceiros (cert-manager, KEDA, Crossplane, Flux, Confluent, Grafana).

**Status codes ordenados**: `Healthy < Suspended < Progressing < Missing < Degraded < Unknown`. Agregação: pior status entre todos os recursos filhos vence.

**Health-aware sync**: Um sync não é considerado concluído até que todos os recursos atinjam Healthy. O sync loop faz polling de health entre waves.

**Por que importa para Cosca**: Nosso sistema de health é binário (agente funciona/não funciona). O modelo de health com múltiplos status + agregação + scripts customizados permitiria: health de agentes (Healthy/Progressing/Degraded), health de skills, health de workflows. Lua scripts dariam extensibilidade sem recompilar.

**Arquivos-chave**: `gitops-engine/pkg/health/health.go`, `controller/health.go`, `resource_customizations/*/health.lua` (81+ scripts)

---

### 6. Generator-Based Template Generation (ApplicationSet)

**O que é**: Sistema de geração de Applications a partir de templates com 8 tipos de **generators** plugáveis. Cada generator implementa `GenerateParams(appSetGenerator) → []map[string]any` — produz conjuntos de parâmetros que são injetados no template via `{{ param }}`.

**8 generators**:
- **List**: parâmetros estáticos do CRD
- **Cluster**: descobre clusters do Argo CD, gera por cluster
- **Git**: descobre diretórios/arquivos em repo Git
- **SCM Provider**: descobre repositórios (GitHub, GitLab, Bitbucket, etc.)
- **Pull Request**: descobre PRs para preview environments
- **Matrix**: produto cartesiano de 2 generators (composição)
- **Merge**: merge por chave de 2+ generators
- **Plugin**: chama plugin externo gRPC/REST

**Pipeline**: Generator → `[]map[string]any` → `RenderTemplateParams()` → Application CRD → validação → criação.

**Por que importa para Cosca**: Nosso TaskPlanner (L153) tem 5 templates determinísticos. O padrão de Generator produzindo `[]map[string]any` que alimenta templates é superior: separa a lógica de descoberta (generator) da lógica de renderização (template). Matrix/Merge permitem composição de generators. Aplicável a: geração de workflows, geração de agentes, geração de configurações.

**Arquivos-chave**: `applicationset/generators/interface.go`, `applicationset/controllers/template/template.go`, `applicationset/generators/matrix.go`, `applicationset/generators/merge.go`

---

### 7. Progressive Sync State Machine (Rolling Updates)

**O que é**: Estratégia de rollout progressivo com state machine `Waiting → Pending → Progressing → Healthy`. Applications são agrupadas em steps por `MatchExpressions`. Cada step tem `maxUpdate` (contagem ou percentual) limitando concorrência. Um step só avança quando todos os apps do step atual atingem `Healthy`.

**Reverse deletion**: Quando `DeletionOrder: Reverse`, apps são deletados em ordem reversa com exponential backoff.

**Por que importa para Cosca**: Nossos workflows executam steps sequencialmente sem controle de concorrência entre múltiplas instâncias. O padrão de progressive sync com `maxUpdate` + health gate permitiria: rollout gradual de mudanças, canary deployments de workflows, pause automático em Degraded.

**Arquivos-chave**: `applicationset/progressivesync/progressive_sync.go` (865 linhas)

---

### 8. Resource Ownership via Annotation Stamping

**O que é**: Antes de comparar, objetos target são **carimbados** com metadados de tracking: label `app.kubernetes.io/instance` ou annotation `argocd.argoproj.io/tracking-id` com formato `appName:group/kind:namespace/name`. Isso cria cadeia de ownership imutável. Recursos live sem o carimbo são orphans.

**Três métodos**: label-only, annotation-only, annotation+label (configurável). Normalização lida com migração entre métodos sem gerar diff espúrio.

**Por que importa para Cosca**: Nossos artefatos (learnings, patterns, agent outputs) não têm tracking de ownership. Quem criou? De qual workflow? O padrão de stamping com ID composto permitiria rastrear cada artefato até o agente + workflow que o gerou.

**Arquivos-chave**: `util/argo/resource_tracking.go` (323 linhas)

---

### 9. Notification Trigger/Template/Service Model

**O que é**: Sistema de notificações com 3 componentes: **triggers** (condições avaliadas contra estado da aplicação — ex: `on-deployed`, `on-sync-failed`, `on-health-degraded`), **templates** (Go-template com blocos por serviço — slack, email, teams, webhook), **services** (destinos descobertos de annotations).

**Dedup**: `oncePer` key (ex: revision) previne notification storms.

**8 triggers built-in**: on-created, on-deleted, on-deployed, on-sync-failed, on-sync-running, on-sync-succeeded, on-sync-status-unknown, on-health-degraded.

**Self-service**: Apps podem definir seus próprios destinos via annotations.

**Por que importa para Cosca**: Nosso sistema de notificação é o console do kernel reportando ao Don. O modelo trigger/template/service permitiria: agentes notificarem eventos (task concluída, erro, health degraded), roteamento para canais (Telegram, webhook), templates por tipo de evento, dedup para evitar spam.

**Arquivos-chave**: `notification_controller/controller/controller.go`, `notifications_catalog/triggers/`, `notifications_catalog/templates/`

---

### 10. Cluster-Per-Shard Caching com Horizontal Scaling

**O que é**: Cache em memória por cluster gerenciado, isolado por shard. Três algoritmos de sharding: legacy (FNV-32a hash), round-robin, consistent-hashing com bounded loads. Cada controller pod gerencia um subconjunto de clusters. Heartbeat via ConfigMap previne split-brain.

**Sem leader election tradicional**: Todos os controllers são ativos simultaneamente, cada um com seu subconjunto. Sem single point of failure.

**Por que importa para Cosca**: Single-node hoje. Para scale-out, o padrão de shard por cluster/namespace com cache em memória isolado é o caminho: cada runtime instância gerencia um subconjunto de workflows, sem contenção de cache.

**Arquivos-chave**: `controller/sharding/sharding.go` (514 linhas), `controller/sharding/cache.go`

---

### 11. Repo Server como Serviço Separado (Manifest Generation)

**O que é**: A geração de manifests (Git clone, Helm template, Kustomize build, CMP plugins) é feita por um **serviço gRPC separado** (`argocd-repo-server`), não pelo controller. O controller nunca gera manifests — só compara e sincroniza. Separação limpa entre "o que é desejado" (repo server) e "como aplicar" (controller).

**CMP (Config Management Plugins)**: Sidecar gRPC que implementa plugin interface. Repo server descobre plugins e delega geração. Suporte a init, generate, parameters discovery.

**Por que importa para Cosca**: Nosso sistema não tem separação entre "gerar plano" e "executar plano". O padrão de serviço separado para geração permitiria: múltiplos geradores de workflow (YAML, Go templates, Lua), plugins de terceiros, cache de geração por revisão.

**Arquivos-chave**: `reposerver/repository/repository.go` (3.643 linhas), `cmpserver/server.go`

---

### 12. Project-Based Multi-Tenancy com Casbin RBAC

**O que é**: `AppProject` CRD isola: source repos, target clusters/namespaces, recursos permitidos, RBAC policies (Casbin), sync windows. Project tokens no formato `proj:projectName:roleName`. SSO groups do OIDC provider mapeados para Casbin policies.

**Anti-timing attack**: Quando acesso é negado para app inexistente, ainda faz o GET para evitar enumeração por timing.

**Por que importa para Cosca**: Single-tenant hoje. Para SaaS multi-usuário, o padrão Project + Casbin permitiria isolar: namespaces de conhecimento, permissões de agente, quotas de execução. Tokens por projeto + role para API access.

**Arquivos-chave**: `server/rbacpolicy/rbacpolicy.go` (162 linhas), `server/project/project.go`

---

## Patterns Não Copiados

| Pattern | Razão |
|---------|-------|
| **Kubernetes CRDs como storage** | Cosca usa SQLite — não precisamos de CRDs e etcd |
| **Kustomize/Helm manifest generation** | Não geramos Kubernetes YAML |
| **SCM Provider integrations** (GitHub, GitLab, etc.) | Útil futuramente, não prioritário |
| **Dex SSO/OIDC** | Single-user (Don) hoje, sem necessidade imediata |
| **Webhook handling com signed payloads** | Não temos webhooks de Git |

---

## Confidence Tracking

| # | Pattern | Confidence | Validated By |
|---|---------|:----------:|--------------|
| 1 | Double-Loop Reconciliation | 0.95 | Controller principal, Netflix-scale production |
| 2 | Three-Tier Diff Strategy | 0.94 | 3 estratégias em diff.go, 1.272 linhas |
| 3 | Phased+Waved Sync com Dry-Run Gate | 0.94 | Sync engine em produção, sync_context.go 1.876 linhas |
| 4 | Event-Driven+Polling Hybrid | 0.93 | Informers + cache + polling timeout |
| 5 | Health Assessment (Built-in+Lua) | 0.92 | 81+ scripts Lua + built-in checks |
| 6 | Generator Template Generation | 0.91 | ApplicationSet com 8 generators |
| 7 | Progressive Sync State Machine | 0.89 | Rolling update strategy |
| 8 | Resource Ownership Stamping | 0.90 | Tracking label/annotation em toda plataforma |
| 9 | Notification Trigger/Template/Service | 0.88 | 8 triggers, engine separado |
| 10 | Cluster-Per-Shard Caching | 0.92 | 3 algoritmos de sharding |
| 11 | Repo Server Separado | 0.90 | gRPC service isolado |
| 12 | Project Multi-Tenancy Casbin | 0.88 | RBAC + tokens + SSO groups |

**Average confidence**: ~0.91

---

## Top 5 Para Implementação Imediata na Cosca

1. **Double-Loop Reconciliation** (Pattern 1) — Separar status loop de operation loop no workflow engine
2. **Three-Tier Diff** (Pattern 2) — Diff declarativo com normalização no estado do conhecimento
3. **Phased+Waved Sync com Dry-Run** (Pattern 3) — Substituir StepRunner linear por fases/waves com validação pré-execução
4. **Health Assessment** (Pattern 5) — Substituir health binário por multi-status com scripts customizados
5. **Generator Template Generation** (Pattern 6) — Separar descoberta (generator) de renderização (template) no TaskPlanner

---
