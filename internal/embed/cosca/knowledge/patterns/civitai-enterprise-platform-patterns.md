# CivitAI Enterprise Platform Patterns

> **Source**: https://github.com/civitai/civitai — Apache 2.0, ~7.700 arquivos, monorepo pnpm
> **Analyzed**: 2026-08-09 — Análise profunda cross-agent (3 agentes em paralelo)
> **Confidence**: 0.92 (validado por leitura direta de código fonte)

## Intent

Extrair padrões reutilizáveis de uma plataforma enterprise de IA generativa que serve milhões de usuários — arquitetura, API design, model management, economia virtual, moderação, observabilidade.

## Context

O CivitAI é o maior hub de modelos de IA generativa do mundo (Stable Diffusion, LoRA, fluxos ComfyUI). É uma plataforma completa com:
- **Monorepo pnpm** com Next.js 16 + 7 apps standalone + 14 packages compartilhados
- **PostgreSQL + ClickHouse + Meilisearch + Redis** como stack de dados
- **tRPC** como camada de API (100+ routers)
- **Economia virtual** (Buzz) com múltiplas contas e creator program
- **Moderação em camadas** com scanning automático + revisão humana
- **Observabilidade completa**: OpenTelemetry, Prometheus, Grafana Faro, Pyroscope

## Patterns Extraídos (Top 15 para Cosca)

### 1. Hub-and-Spoke Auth Architecture

**O que é**: Um serviço central de autenticação (`apps/auth`, SvelteKit) emite tokens finos (apenas `sub` + `jti` com ES256/JWKS). Todos os outros serviços verificam LOCALMENTE (sem round-trip ao hub a cada request). JWKS é cacheado, refetch só no `kid` desconhecido.

**Por que importa para Cosca**: Nosso `cosca-runtime.service` como hub de auth, `cosca-serve` e outros serviços como spokes. Token fino evita estado stale. A verificação local evita latência e ponto único de falha.

**Código-chave**: `packages/civitai-auth/src/index.ts` — `createAuthVerifier`, `createSpokeGuard`

### 2. Transactional Outbox + CDC Dual-Path

**O que é**: Eventos de ciclo de vida de entidades são escritos numa tabela `Outbox` no PostgreSQL (na mesma transação). Duas vias de consumo: (a) Debezium CDC → Kafka (caminho rápido), (b) `OutboxPoller` reconcilia gaps periodicamente. Entrega at-least-once com dedup no consumidor.

**Por que importa para Cosca**: Nossa esteira autônoma e workflow engine precisam de garantia de entrega de eventos entre agentes. O outbox na mesma transação garante atomicidade. O poller de reconciliação cobre falhas do CDC.

**Código-chave**: `apps/event-engine/src/index.ts`, schema `Outbox`

### 3. tRPC com Layered Procedure Guards

**O que é**: Middleware chain por tier de procedimento: `publicProcedure → protectedProcedure → guardedProcedure → moderatorProcedure`. Cada tier adiciona guards. `heavyProcedure` adiciona bulkhead (cap de concorrência por pod, 429 quando cheio). `cacheIt` com Redis + msgpackr, `rateLimit` sliding-window, `edgeCacheIt` com CDN tags.

**Por que importa para Cosca**: Nossos endpoints REST podem adotar tieragem similar. O bulkhead previne que um endpoint pesado derrube o pod inteiro. Rate limiting fail-open (Redis down = request passa sem limite) é a régua de disponibilidade.

**Código-chave**: `src/server/trpc.ts`, `src/server/middleware.trpc.ts`

### 4. Bitwise Flags para Permissões e Estados

**O que é**: `browsingLevel` (NSFW), `tokenScope` (API keys), `onboarding` state, `flags` em modelos — todos usam bitwise integers. Combina múltiplos booleanos em um campo indexável. Consulta SQL: `WHERE (flags & 4) != 0`.

**Por que importa para Cosca**: Nossos agentes, skills, capabilities podem usar bitwise para features flags, permissões, estados. Compacto, extensível sem migração, queryable.

**Código-chave**: `packages/civitai-shared/` — browsing levels

### 5. Denormalized Metrics per Entity per Timeframe

**O que é**: Cada entidade (Model, Image, User, Post, etc.) tem uma tabela de métricas separada com PK composta `(entityId, timeframe)`. Timeframes: Day, Week, Month, Year, AllTime. Atualizadas async pelo event-engine via CDC. Leitura é um SELECT indexado, nunca COUNT(*).

**Por que importa para Cosca**: Nossos dashboards (Command Center, agent metrics) podem usar o mesmo padrão. Métricas de agente (tasks completadas, confiança, erros) agregadas por timeframe.

**Código-chave**: schema `ModelMetric`, `ModelVersionMetric`, `UserMetric`

### 6. Fail-Open em Serviços Auxiliares

**O que é**: Redis, Meilisearch, rate limiting — NUNCA bloqueiam o request principal. Todo write/read em serviço auxiliar é try/catch. Falha → log + métrica + continua sem cache/limite. "A Redis blip never turns a successful DB response into a 500."

**Por que importa para Cosca**: Nossa régua sem regredir. O decision trace, o knowledge engine, o embedding — todos devem ser best-effort. Se o vector store falhar, a busca FTS5 ainda funciona.

**Código-chave**: `src/server/meilisearch/client.ts` — circuit breaker, timeout, fail-open

### 7. Event Engine com Poison Message Handling

**O que é**: Consumidor Kafka classifica erros em 3 categorias: (a) Poison (TypeError, ReferenceError) → skip + advance offset, (b) Transient (DB/network) → bail batch para redelivery, (c) Backpressure → sleep + retry. ClickHouse ReplacingMergeTree absorve re-inserts.

**Por que importa para Cosca**: Nosso Recovery Loop (L153) pode adotar classificação tripla. Erro de compilação = poison, timeout de rede = transient. Dedup no knowledge.db absorve retries.

**Código-chave**: `apps/event-engine/src/handlers/index.ts`

### 8. Health Check com Concurrent Racing

**O que é**: `GET /health` dispara checks concorrentes (DB, Redis, ClickHouse, Meilisearch). Cada check tem timeout individual. Resultado parcial reportado conforme checks completam. Checks podem ser desabilitados/demovidos via env vars.

**Por que importa para Cosca**: Nosso `/health` e `/ready` já existem mas são sequenciais. Concorrente com timeout dá diagnóstico mais rápido e granular.

**Código-chave**: `src/pages/api/health.ts`

### 9. Lint Rules as Tests

**O que é**: Convenções de código são testadas, não só lintadas. Regras customizadas ESLint escritas como funções AST + testadas com `RuleTester` da ESLint via Vitest. CI: `pnpm test:lint-rules`. Se uma regra falha = teste vermelho = bloqueia merge.

**Por que importa para Cosca**: Podemos aplicar ao Go: `go vet` rules customizadas + testes para convenções de arquitetura (ex: "nunca importar `internal/knowledge` de `internal/cli`").

**Código-chave**: `eslint-local-rules.js`, `src/server/services/__tests__/no-io-in-transaction.test.ts`

### 10. importOriginal Mock Pattern

**O que é**: Testes que mockam módulos SENSÍVEIS (DB, Redis, etc.) devem ESPALHAR o módulo real e sobrescrever só partes. `vi.mock('~/server/db', async (importOriginal) => ({ ...(await importOriginal()), ...myOverrides }))`. Isso evita que um mock esconda uma nova dependência que quebraria em produção.

**Por que importa para Cosca**: Nosso `go test` com interfaces já faz isso por construção. Mas o princípio de "nunca mockar o módulo inteiro, só as bordas" se aplica aos nossos testes de handler e service.

**Código-chave**: `no-wholesale-module-mock` rule (771 linhas de teste!)

### 11. Schema Migrations Manuais (Nunca Auto-Apply)

**O que é**: `prisma migrate deploy` NUNCA é usado. SQL é escrito e aplicado por humanos em cada ambiente. `schema.full.prisma` é o source of truth; `schema.prisma` é gitignored e gerado. CI: `db:check-generated` falha se o cliente gerado está stale.

**Por que importa para Cosca**: Nosso `knowledge.db` com SQLite. Migrations devem ser manuais e versionadas, nunca automáticas. Schema canônico → geração → verificação de drift.

**Código-chave**: `packages/civitai-db-schema/`

### 12. Polymorphic Reports Pattern

**O que é**: Uma tabela central `Report` com 17+ tabelas entity-specific (`ModelReport`, `ImageReport`, etc.) ligadas por FK nullable. Separa o workflow de denúncia (genérico) dos dados específicos de cada entidade.

**Por que importa para Cosca**: Nosso sistema de `ConflictRecord` e `BugFingerprint` pode adotar padrão similar — uma tabela central de "incident" com subtabelas por tipo de entidade afetada.

**Código-chave**: schema `Report` + `ModelReport`, `ImageReport`, ...

### 13. Multi-Account Virtual Currency (Buzz)

**O que é**: Sistema de contas coloridas (yellow=usuário, blue=geração, green/red=domínios, creatorProgramBank=pool de criadores). Blue Buzz é não-sacável (gasto só em geração). Creator program: pool USD dividido proporcionalmente ao Buzz acumulado. Hard cap $1/1000 buzz.

**Por que importa para Cosca**: Nossa economia cognitiva (budget, custo de agentes, créditos de API) pode ser modelada como multi-account. "Agente gastou X tokens" → debita da conta do projeto. Pool de créditos compartilhado entre agentes.

**Código-chave**: `packages/civitai-buzz/`

### 14. Creator Studio Analytics com ClickHouse

**O que é**: Analytics pesado vai para ClickHouse (columnar), não PostgreSQL. Eventos de page view, geração, download são ingeridos async. Dashboards de creator mostram tendências, earnings, engajamento com queries agregadas rápidas.

**Por que importa para Cosca**: Nosso CMITracker (L153) e métricas de agente podem usar tabela separada de eventos com agregação. Não poluir o knowledge.db transacional com analytics.

**Código-chave**: `apps/creator-studio/src/server/analytics.ts`

### 15. Client Disconnect Abort

**O que é**: `AbortController` ligado ao `res.close` do request HTTP. Quando o cliente fecha a conexão, o sinal propaga para queries DB e chamadas Meilisearch — economizando CPU do pod em requests abandonados.

**Por que importa para Cosca**: Nosso `cosca serve` com SSE e long-polling. Se o Don fecha o browser durante uma busca, o trabalho deve abortar.

**Código-chave**: `src/server/trpc.ts` — `createContext`

## Anti-Padrões Observados (O que NÃO copiar)

1. **Schema único de 7.642 linhas**: O `schema.full.prisma` é monolítico. Para Cosca, preferir schemas modulares por domínio.
2. **Múltiplos sistemas de comment (V1 + V2)**: Migração incompleta gera dívida. Nós já resolvemos isso com o AgentBridge unificado.
3. **Prisma + Kysely coexistindo**: Dois ORMs no mesmo projeto. Complexidade desnecessária. Nós usamos SQLite direto.
4. **Testes que não provam o revert**: O próprio CLAUDE.md deles admite que teste passando não prova que pega regressão. Nós já temos `-race` e testes de regressão com revert check (L57).

## Métricas do Repo (Contexto)

| Métrica | Valor |
|---------|-------|
| Arquivos | ~7.700 |
| Apps standalone | 7 (auth, creator-studio, event-engine, moderator, notifications, orchestrator-gateway, storage) |
| Packages compartilhados | 14 (`@civitai/*`) |
| Tabelas PostgreSQL | ~150+ |
| tRPC Routers | ~100 |
| Tipos de modelo IA | 22 (Checkpoint, LoRA, DoRA, LoCon, TextualInversion, VAE, Controlnet...) |
| Índices Meilisearch | 10 |
| Provedores de pagamento | 5 (Stripe, Paddle, PayPal, Coinbase, NowPayments) |
| Tipos de conta Buzz | 9 (yellow, blue, green, red, creatorProgramBank...) |
| Linhas do schema Prisma | 7.642 |

## Relação com Patterns Existentes na Cosca

| Pattern CivitAI | Pattern Cosca Relacionado |
|-----------------|--------------------------|
| Hub-and-spoke auth | L154 — Single Owner Model (runtime ↔ serve) |
| Transactional outbox | L153 — Recovery Loop + esteira autônoma |
| tRPC layered guards | Gate 0-4 + RBAC (L71) |
| Bitwise flags | Feature flags (28 flags, L66) |
| Denormalized metrics | CMITracker (6 dimensões, L153) |
| Fail-open | Régua sem regredir (todas as fases) |
| Event engine poison handling | Error Classifier (37 padrões, L153) |
| Lint rules as tests | convention guards pendentes |
| Polymorphic reports | ConflictRecord + BugFingerprint (L74) |
| Multi-account currency | Cognitive Budget (L75) |

## Tags

`#civitai` `#enterprise` `#monorepo` `#trpc` `#auth` `#outbox` `#bitwise` `#metrics` `#fail-open` `#virtual-currency` `#moderation` `#analytics` `#patterns` `#cross-agent-analysis` `#level-4`

---

*Análise conduzida por 3 agentes Cosca em paralelo (architecture, patterns/testing, AI/ML). Extração e síntese pelo cosca-kernel. 2026-08-09.*
