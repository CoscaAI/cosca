# Blueprint de Arquitetura — Plataforma Zernio (gestão de redes sociais via API)

> **Tipo:** Relatório de mineração (pesquisa)
> **Autor:** Architecture Chief (Cosca)
> **Data:** 2026-09-01
> **Objetivo:** Extrair o blueprint de arquitetura da plataforma Zernio (org GitHub `zernio-dev`) a partir de fontes públicas, para **construir a NOSSA plataforma** com base nos aprendizados — **não** para usar a Zernio.
> **Restrições:** Apenas leitura via `cosca.web` (API GitHub pública + raw + docs). Nenhuma alteração no workspace além do relatório.
> **Rastreabilidade:** Todos os achados têm fonte citada (repo + arquivo). Dados marcados como `[inferido]` são dedução de arquitetura a partir de evidências parciais, não afirmações confirmadas.

---

## 0. Resumo executivo

A Zernio (antiga **"Late"** — ver §11) é uma plataforma **single-tenant-primária, multitenant via Profiles**, que expõe uma **API REST versionada em URL (`/api/v1`)** para agendar e publicar conteúdo em **16+ redes sociais** com um único `POST /v1/posts`. O núcleo de valor é a **camada de conexão OAuth + fila de publicação multi-plataforma com status por plataforma** (`post.scheduled → post.published|post.failed|post.partial`), mais **analytics agregados** e **inbox/mensagens** para agentes.

A stack é distribuída publicamente em ~27 repos: 8 SDKs (Python/Node hand-written; Rust/.NET/PHP/Java/Ruby/Go gerados por OpenAPI Generator a partir de um **`openapi.yaml` de 2,4 MB**), um node n8n, CLI, plugins de IA (Cursor, Claude/MCP), 3 apps open-source (zernflow = chatbot builder, latewiz = scheduler, ads-dashboard), e um "drop-in replacement" do SDK da Ayrshare (`social-media-api`).

Arquiteturas-chave descobertas (todas com fonte):
- **Fila de publicação** com estados por plataforma e webhooks `post.*` (fonte: `n8n-nodes-zernio/late/resources/webhooks.ts` e `posts.ts`).
- **Upload presign** para arquivos até **5 GB** (fonte: SDKs Node/Python + README n8n).
- **OAuth centralizado** com `GET /v1/connect/{platform}?profileId=...` retornando `authUrl` (fonte: `docs.zernio.com/llms-full.txt`, Glossary).
- **Webhooks assinados HMAC-SHA256**, com **event id** para dedup/retry, timeout de entrega de **5 s** (fonte: `zernflow/app/api/webhooks/late/route.ts`).
- **Idempotência em duas camadas**: `Idempotency-Key` (guias de idempotência) + **content-hash dedup 24 h** devolvendo `409` + `existingPostId` (fonte: `llms-full.txt`, Glossary).
- **Billing metered por "account-day"** (Metronome + Stripe), rate limits que escalam com o nº de contas conectadas (fonte: `llms-full.txt`, Billing).
- **Backend MongoDB** (`_id` em webhooks; placeholders ObjectId) `[inferido]`.

---

## 1. Inventário da organização `zernio-dev` (27 repos públicos)

Metadados da org: `api.github.com/orgs/zernio-dev` — "Social & messaging for developers and AI agents", 717 followers, criada 2026-01-23 (mas repos datam de 2025-08 — reorg/rebrand de "Late").

| Categoria | Repos | Notas |
|---|---|---|
| SDKs **hand-written** | `zernio-python` (branch `develop`), `zernio-node`, `late-node` (legado) | Client `Zernio()` com resources: `posts, accounts, profiles, analytics, media`; async nativo no Python |
| SDKs **gerados (OpenAPI)** | `zernio-rust`, `zernio-dotnet`, `zernio-php`, `zernio-java` (`dev.zernio:zernio-sdk`), `zernio-ruby` (`zernio-sdk` gem), `zernio-go` | Generator 7.19.0, API version 1.0.4; cada repo carrega o **`openapi.yaml` (2,4 MB em `zernio-python`)** |
| Integração low-code | `n8n-nodes-zernio` (branch `master`) | Node n8n "Zernio/Late" com 24 resources e ~100 operações; fonte mais completa do contrato REST |
| Apps open-source | `zernflow` (chatbot builder/ManyChat alt), `latewiz` (scheduler/calendar), `ads-dashboard` (Next.js) | Consomem a API; revelam padrões de webhook/OAuth |
| Cliente CLI | `zernio-cli` | TypeScript |
| Plugins de IA | `cursor-plugin`, `zernio-claude-plugin` | MCP/hosted MCP server; "run sequences, manage ads" |
| Compat | `social-media-api` | "Drop-in replacement for the Ayrshare social-media-api SDK" — estratégia de migração de clientes |
| Conector | `zernio-shopify` (ARQUIVADO/discontinued) | Substituído por app na Shopify App Store |
| Outros | `.github`, `TikTok-Api` (fork de wrapper não-oficial do TikTok) | O fork sugere uso de APIs não-oficiais p/ algumas capacidades `[inferido]` |
| Não existem | `zernio-openapi`, `zernio-docs`, `zernio-mcp`, `zernio-go`-outros | Probes 404 |

**Plataformas suportadas (16+):** X/Twitter, Instagram, Facebook, LinkedIn, TikTok, YouTube, Threads, Bluesky, Pinterest, Reddit, Telegram, WhatsApp, Snapchat, Google Business Profile, Discord, Slack + Shopify blogs + SMS (`10DLC`) + ads (TikTok Ads, Pinterest Ads, X Ads, OpenAI/ChatGPT Ads, Meta Ads).

---

## 2. Blueprint da arquitetura

Diagrama inferido da composição de fontes públicas (n8n node baseURL, SDKs, apps open-source, docs):

```
┌────────────────────────────────────────────────────────────────────────────┐
│                           CLIENTES DA API                                  │
│  Web app (zernio.com) · latewiz · zernflow · ads-dashboard · CLI · n8n     │
│  Plugins IA (Cursor, Claude/MCP) · seus sistemas (SDKs de 8 linguagens)    │
└───────────────┬──────────────────────────────┬─────────────────────────────┘
                │ Bearer API key (sk_...)      │ OAuth redirect
                ▼                              ▼
┌──────────────────────────────┐   ┌──────────────────────────────┐
│       GATEWAY / API          │   │   CONNECT (OAuth broker)     │
│  https://zernio.com/api/v1   │   │  GET /v1/connect/{platform}  │
│  - API keys + scopes         │   │  ?profileId= → authUrl       │
│  (disabledResourceGroups)    │   │  callback → account criada   │
│  - rate limits (escalam c/   │   │  no profile                  │
│    nº contas conectadas)     │   │  Clone: POST /profiles/{id}/  │
│  - content-hash dedup 24h    │   │  clone-connection            │
└──────────────┬───────────────┘   └──────────────┬───────────────┘
               │                                   │
┌──────────────▼───────────────────────────────────▼───────────────┐
│                    CORE / DOMAIN SERVICES                         │
│  Profiles · Accounts (health, tokens, refresh) · Account Groups  │
│  Posts (estado por plataforma) · Queue (slots) · Media (presign) │
│  Analytics (agregados + por plataforma) · Inbox/Messages         │
│  Webhooks (settings + delivery) · API Keys · Users · Invites     │
│  Logs (publicação + entrega) · Tools (download/transcript)       │
│  Billing (Metronome + Stripe; account-day metering)              │
├──────────────────────────────────────────────────────────────────┤
│  Persistência: MongoDB (_id/ObjectId)  [inferido]                │
│  Jobs/agendamento: fila interna (status scheduled → due)         │
│  Sync externo: background sync de posts nativos (~90 min)        │
└───────┬───────────────────────────────┬──────────────────────────┘
        │ publica                        │ emite eventos
        ▼                               ▼
┌──────────────────────────┐   ┌──────────────────────────────┐
│  PUBLISHING FAN-OUT      │   │  WEBHOOK DELIVERY            │
│  por conta (1..N)        │   │  POST https://seu-endpoint   │
│  retry por plataforma    │   │  headers: x-late-signature   │
│  status: published /     │   │  (HMAC-SHA256), x-late-      │
│  failed / partial        │   │  event-id; timeout 5s;       │
│  logs por post+plataforma│   │  retry c/ mesmo event id     │
└───────────┬──────────────┘   └──────────────┬───────────────┘
            │                                  │
┌───────────▼──────────────────────────────────▼───────────────┐
│  REDES SOCIAIS: X, IG, FB, LI, TikTok, YT, Threads, Bluesky, │
│  Pinterest, Reddit, Telegram, WhatsApp, Snapchat, GMB,       │
│  Discord, Slack, Shopify, SMS, Ads (TikTok/Pinterest/X/OpenAI)│
└───────────────────────────────────────────────────────────────┘
```

**Modelo mental (fonte: Glossary, `llms-full.txt`):** `team` (workspace) → `profile` (pasta/tenant por cliente) → `account` (conta social conectada, `accountId`). Webhooks apontam eventos com `accountId`; quem é multitenant mantém mapeamento `accountId → customer`.

---

## 3. Domínio e modelo de dados (entidades + relacionamentos)

| Entidade | Campos-chave observados | Relacionamentos |
|---|---|---|
| **Team** (workspace) | — | dono de profiles, users, api keys, billing |
| **Profile** | `name`, `description`, `color`; delete exige profile **vazio**; criação **sujeita a limites do plano** | 1 profile → N accounts; 1 profile → N queue slots; 1 profile → 1 API-key scope |
| **Account** (conta social conectada) | `accountId`, `platform`, `username`, `displayName`, token status, permissões, settings por plataforma (bluesky-settings, slack-settings, tiktok creator-info, ig ice-breakers, messenger-menu, telegram-commands) | N accounts → 1 profile; pode ser **movida** (`PATCH /v1/accounts/{id}`) entre profiles; tem **health** (`GET /v1/accounts/health`, `/accounts/{id}/health`) |
| **Account Group** | `name`, lista de `accountId`s (pode cruzar profiles); nome único por usuário | N accounts → M groups; escopável por API key |
| **Post** | `content`, `platforms[]` (`{platform, accountId, platformSpecificData?}`, `platformSpecificContent?`), `mediaUrls[]`/`mediaItems[]`, `scheduledFor`, `timezone`, `publishNow`, `status`, `createdBy`; `platformSpecificData` por plataforma (threadItems, contentType:story, tiktokSettings{privacy_level, allow_comment, allow_duet, allow_stitch}, youtubeTitle) | 1 post → N targets de publicação; 1 post → N logs; 1 post → analytics |
| **Media** | `filename`, `contentType`, `uploadUrl` (presign), `publicUrl`; upload direto tb. Limite **5 GB** | 1 media → 1..N posts |
| **Queue** (slots) | horários de publicação recorrentes | N slots → 1 profile; "Smart Queue" no latewiz |
| **Webhook** | `name`, `url` (HTTPS), `secret`, `events[]`, `isActive`, `customHeaders`; id tipo **ObjectId MongoDB** | N webhooks → 1 team; 1 webhook → N delivery logs |
| **API Key** | `name`, `disabledResourceGroups` (`["messages","contacts","webhooks"]`) → **escopo por grupo de recurso**; começa com `sk_` | 1 key → scopes; 1 key → limits |
| **User / Invite** | invite com "profile access" | M users → 1 team |
| **Log** | publicação por post (`/posts/{id}/logs`) e entrega de webhook (`/webhooks/logs` c/ status success/failed, event) | 1 log → 1 post ou 1 webhook delivery |
| **Message / Conversation / Contact** (inbox) | `message.id`, `conversationId`, `platformMessageId`, `direction`, `sender{id,name,username,picture}`, `attachments[]`, `isRead`; `metadata{quickReplyPayload, postbackPayload,...}` | message → conversation → account; usados por zernflow |
| **External Post** | post nativo detectado por sync de background **~90 min** | synced → vira post rastreável |

**Relacionamentos centrais:** `Team 1—N Profile 1—N Account`; `Post 1—N AccountTarget`; `Account 1—N Post`; `Post 1—N PublishLog`; `Team 1—N Webhook`; `APIKey N—M ResourceGroup (disabled)`.

---

## 4. API surface

**Convenções gerais (fonte: READMEs dos SDKs gerados — bloco idêntico em rust/dotnet/php/java/ruby):**
- Base URL: `https://zernio.com/api`; **versionamento no path** (`/v1`); breaking changes só em nova versão de path; endpoints antigos continuam vivos.
- Deprecação: `deprecated: true` no spec + changelog (`https://zernio.com/changelog`) antes de remover.
- Erros: todo 4xx/5xx é `application/json` com `code` (machine-readable) + `error` (humano) — schema `ErrorResponse`.
- Auth: `Authorization: Bearer <api_key>` (chave `sk_...`); `GET /v1/auth/verify` para validar.
- Paginação/filtros (ex. `GET /v1/posts`): `page`, `limit`, `status`, `platform`, `profileId`, `createdBy`, `dateFrom`, `dateTo`, `includeHidden`, `search`, `sortBy`.

### Recursos e operações (compilado de `n8n-nodes-zernio/late/resources/*.ts` + SDKs + docs)

| Recurso | Operações (método + rota confirmada) |
|---|---|
| **Profiles** | `GET /profiles`, `POST /profiles`, `GET/PUT/DELETE /profiles/{id}` (delete só vazio; create sujeito a plano) |
| **Posts** | `GET /posts` (filtros acima), `GET /posts/{id}`, `POST /posts` (agendar/publicar), `PUT /posts/{id}` (draft/scheduled), `DELETE /posts/{id}` (publicado não deleta), `POST /posts/{id}/retry`, `POST /posts/{id}/unpublish` (body: `platform`), `POST /posts/bulk-upload` (CSV multipart, `dryRun`), `GET /posts/{id}/logs`; SDKs ainda listam `update_post_metadata` e `edit_post` (pós-publicado) |
| **Media** | `POST /media/presign` (`{filename, contentType}` → `uploadUrl`+`publicUrl`), `POST /media/upload` (direto); até **5 GB** |
| **Accounts** | `GET /accounts`, `GET /accounts/{id}`, `PUT /accounts/{id}`, `DELETE /accounts/{id}` (disconnect), `PATCH /accounts/{id}` (move to profile), `GET /accounts/health` (all), `GET /accounts/{id}/health` (token status, permissões, recomendações; WhatsApp inclui `platformConnection` probe), `GET /accounts/follower-stats`, `GET /accounts/{id}/posts`; settings por plataforma (`/bluesky-settings`, `/slack-settings`, `/tiktok/creator-info`, `/instagram-ice-breakers`, `/messenger-menu`, `/telegram-commands`) |
| **Account Groups** | `GET/POST /account-groups`, `PUT/DELETE /account-groups/{id}` |
| **Connect (OAuth)** | `GET /connect/{platform}?profileId=` → `{authUrl}`; callbacks por plataforma; Telegram: status+initiate; Bluesky: connect com credenciais (App Password); Facebook: list/select page; LinkedIn: update org (personal/company); Pinterest: boards; Reddit: subreddits/feed; Snapchat: profiles; Google Business: locations/reviews/reply |
| **Clone Connection** | `POST /profiles/{profileId}/clone-connection` (body: `sourceAccountId` + targets FB `targetPage*` ou LI `targetOrganization*`) — reusa OAuth entre profiles |
| **Queue** | `GET/POST /queue`, `PUT/DELETE /queue/{slotId}`, `GET /queue/preview`, `GET /queue/next-slot` |
| **Webhooks** | `GET/POST/PUT/DELETE /webhooks/settings`, `POST /webhooks/test`, `GET /webhooks/logs` (`limit/status/event/webhookId`), SDK Go: `RedeliverWebhookEvent` |
| **Analytics** | `GET /analytics` (por postId, period), best-time-to-post, content-decay, daily-metrics, posting-frequency, post-timeline; por plataforma: FB page insights/post earnings/post reactions, IG insights/demographics/follower-history, LI aggregate/org/post/reactions, TikTok account insights, YouTube channel insights/daily views/demographics/retention; `SyncExternalPosts` |
| **API Keys** | `GET/POST /api-keys`, `DELETE /api-keys/{keyId}`, `GET /auth/verify`; scopes via `disabledResourceGroups` |
| **Users / Team** | `GET /users`, `GET /users/{id}`, invites (create com profile access) |
| **Usage / Billing** | `GET /usage` (stats vs plano), SDK Go: `GetBilling` |
| **Tools** (utilitários) | `GET /tools/{platform}/download|transcript`, `POST /tools/instagram/hashtag-checker` (yt-dlp-style backend) |
| **Messages / Inbox** | eventos `message.received`/`comment.received` + respostas via API (zernflow); endpoints de mensagens referenciados como grupo de recurso escopável |

**Padrões de contrato:** REST-ish (GET/POST/PUT/PATCH/DELETE), actions como sub-rotas verbais (`/retry`, `/unpublish`, `/health`, `/presign`, `/bulk-upload`, `/test`, `/logs`, `/clone-connection`), query filters ricos, respostas embrulhadas (`{data: {posts: [...]}}`, `{post: {...}}` — SDKs desembrulham `{data}`). Webhook config usa `_id` no body de update (vaza convenção Mongo).

---

## 5. Fluxo de publicação (agendamento → fila → multi-plataforma → retry → logs → webhooks)

Fluxo reconstruído das fontes (contrato REST + eventos + apps):

```
1. CRIAR POST
   POST /v1/posts  { content, platforms: [{platform, accountId,
                     platformSpecificData?}], mediaUrls?, scheduledFor?,
                     timezone?, publishNow? }
   ├─ Validação plan/limites
   ├─ Dedup content-hash 24h → se duplicado: 409 + existingPostId
   └─ Resposta: post com status inicial
        ├─ publishNow=true  → "scheduled agora" (processamento imediato)
        └─ scheduledFor set → status=scheduled; entra na fila
                │
2. FILA / AGENDADOR  (jobs internos; slots de Queue p/ recorrência)
   Quando due: dispara fan-out de publicação para cada target
                │
3. PUBLICAÇÃO POR PLATAFORMA (1..N em paralelo)
   Para cada {platform, accountId}:
   ├─ Pega token válido (refresh automático se expirou)
   ├─ Publica no SDK da rede
   ├─ Sucesso → post.platform.status = published
   ├─ Falha transitória → retry (backoff) por plataforma
   └─ Falha final → status = failed (por plataforma)
                │
4. RESULTADO AGREGADO POR POST
   ├─ Todos publicados → post.status = published
   ├─ Alguns falharam → post.status = partial   (!!)
   └─ Todos falharam → post.status = failed
                │
5. LOGS (append por tentativa)
   GET /v1/posts/{id}/logs  → histórico por plataforma/tentativa/erro
   Retry manual: POST /v1/posts/{id}/retry (re-processa falhas)
   Unpublish: POST /v1/posts/{id}/unpublish {platform}
                │
6. WEBHOOKS (eventos emitidos ao longo do ciclo)
   post.scheduled (ao agendar)
   post.published (sucesso total)
   post.failed    (falha total)
   post.partial   (parcial — ALGUMAS plataformas falharam)
   post.recycled  (re-ciclo de conteúdo)
   account.connected / account.disconnected
   message.received / comment.received / webhook.test
```

**Idempotência (2 camadas, fonte Glossary/`guides/idempotency`):** (a) `Idempotency-Key` do cliente por requisição; (b) **content-hash dedup**: mesma `(platform, accountId, content+media)` em 24 h → `409` + `existingPostId`.

**Detalhes de contrato (fonte `posts.ts`):**
- `GET /posts` aceita `status`, `platform`, `profileId`, `createdBy`, `dateFrom/dateTo`, `includeHidden`, `search`, `sortBy`.
- `POST /posts/bulk-upload` aceita CSV multipart com **`dryRun`** (validação sem publicar).
- Twitter threads: `platformSpecificData.threadItems[]` (280 chars/tweet, mídia só no 1º). Instagram: `contentType: "story"`. TikTok: `tiktokSettings{privacy_level, allow_comment, allow_duet, allow_stitch}` + carrossel de fotos + AIGC disclosure.

---

## 6. Auth

| Camada | Mecanismo | Fontes |
|---|---|---|
| **Clientes (devs)** | **API keys** `Bearer` (`sk_...`), criadas em `POST /v1/api-keys`, **escopáveis por grupo de recurso** (`disabledResourceGroups: ["messages","contacts","webhooks"]`), verificáveis via `GET /v1/auth/verify`; revogáveis (`DELETE`) | SDKs (ex. PHP quickstart), docs |
| **Conexão de contas sociais** | **OAuth broker centralizado**: `GET /v1/connect/{platform}?profileId=…` → `authUrl`; usuário autoriza; conta cai no profile; a Zernio gerencia **token refresh + rate limiting** (zernflow README: "Powered by Zernio for OAuth, token refresh, rate limiting"). Bluesky usa App Password (sem OAuth). Telegram tem flow próprio (status/initiate) | Glossary + zernflow README + n8n `connect.ts` |
| **Clonagem de conexão** | `POST /v1/profiles/{profileId}/clone-connection` com `sourceAccountId` e, p/ FB: `targetPageId/Name/AccessToken`; p/ LinkedIn: `targetOrganizationId/Urn/Name/Type` — reusa credenciais OAuth entre profiles/páginas/orgs sem re-autenticar | `n8n-nodes-zernio/late/resources/clone.ts` |
| **Saúde do token** | `GET /v1/accounts/{id}/health` retorna `tokenStatus`, permissões, recomendações; para WhatsApp inclui probe vivo `platformConnection` (o token pode estar OK e o link Meta morto) | `llms-full.txt` (accounts/get-account-health) |

---

## 7. Eventos / Webhooks

**Configuração (fonte `webhooks.ts`):** `POST/PUT /v1/webhooks/settings` com `{name, url (HTTPS), secret, events[], isActive, customHeaders}`. Default: `["post.published","post.failed"]`. Teste: `POST /v1/webhooks/test`. Entrega auditável: `GET /v1/webhooks/logs?status=success|failed&event=...`.

**Eventos suportados (nomes exatos):**
- Publicação: `post.scheduled`, `post.published`, `post.failed`, `post.partial`, `post.recycled`
- Contas: `account.connected`, `account.disconnected`
- Inbox: `message.received`, `comment.received`
- Infra: `webhook.test`

**Formato de entrega (fonte: `zernflow/app/api/webhooks/late/route.ts` — receptor real):**
- Headers: `x-late-signature` (HMAC-SHA256 do body com o `secret`), `x-late-event-id`.
- Payload (evento de mensagem): `{id (event id), event, message{id, conversationId, platform, platformMessageId, direction, text, attachments[{type,url,payload}], sender{id,name,username,picture}, sentAt, isRead}, conversation{id, platformConversationId, participantId, participantName, participantUsername, participantPicture, status}, account{id, platform, username, displayName}, metadata{quickReplyPayload, callbackData, postbackPayload, postbackTitle}, timestamp}`.
- Evento de comentário: `{id, event, comment{id, postId|null, platformPostId, platform, text, author{...}, createdAt, isReply, parentCommentId}, post{id, platformPostId}, account{...}, timestamp}`.

**Política de entrega (fonte: comentários no route.ts do zernflow):**
- **Timeout de 5 s**: "Zernio aborts deliveries at 5s and retries".
- **Retries com o mesmo event id** → consumidor faz **dedup por event id** (claim em tabela com unique constraint; `23505` = já processado).
- **Reentrega manual**: `RedeliverWebhookEvent` no SDK Go.
- Falha na validação de assinatura → `401`; o consumidor deve verificar antes de processar.

---

## 8. Estrutura dos SDKs

**Dois padrões distintos:**

1. **Hand-written (Python, Node)** — client `Zernio(apiKey, baseURL, timeout)`; **resources como namespaces** (`client.posts.create(...)`, `client.accounts.list()`, `client.media.upload(...)`, `client.analytics.get(...)`); tipagem forte de erros (`ZernioAPIError`, `ZernioRateLimitError` com `getSecondsUntilReset()`, `ZernioValidationError` com `fields`); suporte async (`await client.posts.alist(status="scheduled")`); lê env `ZERNIO_API_KEY` (fallback `LATE_API_KEY`); mantém **compatibilidade de import legado** (`from late import Late` e `from zernio import Zernio` idênticos). O repo Python ainda vira **servidor HTTP** (Dockerfile + `docs/HTTP_DEPLOYMENT.md`) e **servidor MCP** (`docs/MCP.md`, `claude-desktop-config.json`, `scripts/generate_mcp_docs.py`).

2. **Gerados por OpenAPI Generator (Rust, .NET, PHP, Java, Ruby, Go)** — 1 classe por recurso (`AccountsApi`, `PostsApi`, `QueueAPI`, `WebhooksAPI`, `GMBReviewsAPI`, `LinkedInMentionsApi`, `AdAccountsApi`, `APIKeysApi`, `MediaAPI`, `UsageAPI`, `UsersAPI`, `AccountGroupsApi`, `AccountSettingsApi`, `AnalyticsAPI`, `ProfilesAPI`...), modelos tipados, package legado `Late`/`late-sdk` publicado junto para backward-compat. Pipelines `generate.yml`/`release.yml` por repo (SDKs são re-gerados do `openapi.yaml` 2,4 MB).

**Convenção de nomes:** snake_case (Python/Ruby/PHP/Rust), camelCase (Node/.NET/Java/Go), `WithHttpInfo` (Java), `Execute` (Go). Respuestas padronizadas `{data: ...}`.

**Padrão de operação:** os SDKs e o node n8n espelham **1 recurso = 1 módulo de arquivo** (`late/resources/{posts,accounts,webhooks,queue,clone,...}.ts`), com campos reutilizáveis via builders (`buildCommonPostFields`, `buildAccountSelectors`, `buildPlatformSelector`, `buildMediaItemsField`).

---

## 9. Escalabilidade / operação

- **Uploads grandes:** presigned URL (`POST /v1/media/presign` → `uploadUrl` + `publicUrl`) p/ arquivos até **5 GB**; o cliente faz `PUT` direto no storage (S3-style); `publicUrl` é o que vai no post. Há também upload direto p/ arquivos menores.
- **Health check de contas:** `GET /v1/accounts/health` (todas) e `GET /v1/accounts/{id}/health` (detalhado: token status, permissões, recomendações; WhatsApp com probe `platformConnection` ao vivo). Usado p/ detectar contas mortas e acionar `account.disconnected`.
- **Rate limits:** "rate limits scale with your connected accounts, not your key count" (Glossary) — quota por conta conectada, não por key.
- **Billing metered:** account-days (1 unidade por conta por dia conectado), prorata diário, tiers graduados ($6/$3/$1), crédito free-tier $12/mês; invoicing via **Metronome + Stripe**, itemizado por conta/telefone/mensagem/operação X API.
- **Quotas por plano:** criação de profile "subject to plan limits"; `GET /v1/usage` monitora uso vs plano; n8n expõe "Usage Statistics".
- **Sync de posts externos:** background sync ~90 min por conta (`GET /analytics` inclui `SyncExternalPosts`) — eventual consistency p/ posts feitos nativamente.
- **Equipe:** invites com perfil de acesso; multi-usuário por team; API keys por usuário.

---

## 10. Lições / boas práticas para A NOSSA plataforma (priorizadas)

1. **[P0] Contrato de publicação "um post → N targets com status por plataforma" como recurso de 1ª classe.** O `post.partial` (agregado por post + status por plataforma + logs por tentativa) é a joia: modelar `Post → PostTarget(status) → PublishAttempt(log)` desde o dia 1, com retry por target e `unpublish` por plataforma. Sem isso, o cliente não consegue saber "o que exatamente falhou onde".
2. **[P0] Webhooks com event id + HMAC-SHA256 + timeout curto + retry c/ mesmo id.** Copiar: header de assinatura (`x-*-signature`), header/body `event-id` para dedup no consumidor, timeout de entrega (~5 s), `test` endpoint, `logs` de entrega com filtro por status/evento, e reentrega manual. Isto é o que permite construir apps (zernflow) sem perder evento.
3. **[P0] Idempotência em duas camadas: `Idempotency-Key` do cliente + content-hash dedup com janela (24 h) devolvendo `409` + id existente.** Previne duplicação real em retries e em "cliques duplos", e é auditável.
4. **[P0] `openapi.yaml` como fonte única de verdade, com SDKs gerados por pipeline.** Ter um spec canônico (2,4 MB na Zernio = spec gigante; o nosso deve nascer estruturado) e regenerar SDKs em CI (`generate.yml`) elimina drift entre linguagens. A Zernio convive com hand-written vs generated — **nós devemos escolher UM caminho desde o início** (spec-first).
5. **[P1] API versionada em path (`/v1`) com política explícita de deprecação** (`deprecated: true` + changelog + janela) e erros padronizados `{code, error}` machine-readable em todo 4xx/5xx. Contrato estável é o que permite SDKs multi-linguagem.
6. **[P1] Escopo de API key por "resource group"** (`disabledResourceGroups`) + `auth/verify`. Permite chaves mínimas (ex.: só posts, sem inbox) e chaves de cliente em multi-tenancy sem vazar mensagens.
7. **[P1] OAuth broker centralizado com `connect/{platform}?profileId=` + clone de conexão.** O cliente nunca guarda token; o clone (`clone-connection` com targets FB/LI) reusa autorização entre profiles/páginas/orgs — enorme redutor de fricção em agências. Nosso equivalente: conexão de conta desacoplada do profile, com `move` e `clone` como operações de 1ª classe.
8. **[P1] Health check de contas como recurso + health em todas as contas** (`/accounts/health`) com granularidade por plataforma (token vs permissão vs link vivo). É o alarme precoce de `account.disconnected` e a base para billing preciso (account-day).
9. **[P1] Presign para uploads grandes (5 GB) + direct upload pequeno; media referenciada por `publicUrl`.** Separar upload (storage) de publicação (rede social) com contrato `presign → PUT → publicUrl → post`. Nunca trafegar binário pelo gateway.
10. **[P2] Estratégia de compat com rebrand/legado:** a Zernio migrou "Late"→"Zernio" mantendo namespaces, env vars e headers antigos (`late-node`, `late-sdk`, `x-late-signature`, `LATE_API_KEY`, path `/webhooks/late`). Lição dupla: (a) é possível rebrandar sem quebrar clientes; (b) **não repetir o acúmulo**: definir plano de sunset do alias legado. Para nós: pensar o nome/prefixo da plataforma como estável desde o início.
11. **[P2] "Drop-in replacement" como ferramenta de migração de clientes** (`social-media-api` = Ayrshare-compat). Se quisermos capturar mercado de concorrente existente, um adaptador de contrato é mais barato que convencer a migrar.
12. **[P2] Utilitários de conteúdo como parte da API** (`/tools/{platform}/download|transcript`, hashtag-checker) — o "repurpose content" vira feature vendável sem cliente implementar yt-dlp. Apenas se for compatível com licença/ToS (ver risco R5).

---

## 11. Riscos / armadilhas da Zernio que devemos evitar

| # | Risco | Evidência | O que fazer na NOSSA plataforma |
|---|---|---|---|
| **R1** | **Drift entre SDKs hand-written e gerados** (Python/Node vs 6 gerados) — superfície diferente entre linguagens (ex.: o n8n tem Queue/Clone/Tools que o README do Python/Node não lista; Go tem `RedeliverWebhookEvent` e `GetBilling` que outros não mostram) | Comparação READMEs | **Spec-first desde o dia 1**: 1 spec → N SDKs gerados; nenhum SDK manual; diff contratual em CI |
| **R2** | **Spec gigante (2,4 MB)** sinaliza endpoint-sprawl: ~100 operações, endpoints por plataforma e por variação de settings. Custo: geração lenta, docs massiva, API difícil de aprender | `zernio-python/openapi.yaml` = 2.470.230 bytes | Manter superfície enxuta: recursos genéricos + `platformSpecificData` tipado em vez de 1 endpoint por plataforma; gates de crescimento do spec |
| **R3** | **Vazamento de schema interno no contrato** — webhook update manda `_id` no body; placeholders de ID são `ObjectId` Mongo | `webhooks.ts` | Contratos desacoplados do storage: IDs próprios (ex.: `wh_...`), nunca `_id`; abstração de persistência |
| **R4** | **Herança de rebrand mal-saneada** — headers `x-late-*`, paths `/webhooks/late`, env `LATE_API_KEY`, package `late-sdk` em produção; confusão suportada "para sempre" | `zernflow/.../route.ts`, READMEs | Prefixos estáveis + plano de sunset documentado do alias; nunca dois nomes convivendo sem data de fim |
| **R5** | **Dependência de integrações não-oficiais / risco ToS** — fork de `TikTok-Api` (wrapper não-oficial) na org + tools de download de mídia (yt-dlp-style) podem violar ToS das plataformas e quebrar a qualquer momento | repo `TikTok-Api` (fork), `tools.ts` | Para cada plataforma, registrar em ADR se é integração oficial (API/OAuth) ou "unofficial/scraping"; política explícita de risco e fallback; evitar scraping como caminho crítico |
| **R6** | **Timeout de webhook de 5 s + retry** — consumidores lentos geram retry-storm; processamento síncrono antes do ack causa perda (zernflow teve que usar padrão "ack antes, processa depois" com `after()`) | `zernflow/route.ts` | Entrega assíncrona: ack imediato + fila de processamento no consumidor; documentar o timeout e dar "besteira-teste" (`/webhooks/test`) |
| **R7** | **Billing metered por account-day com tiers graduados** — modelo poderoso mas complexo (Metronome + Stripe, prorata diário, crédito free-tier, "billable units" não-intuitivos); risco de suporte/billing disputes | `llms-full.txt` (Billing) | Se for meter por uso, começar por metering simples (conta-mês) e adicionar prorata/tiers só quando o volume justificar; instrumentar desde o dia 1 (account-day events) |
| **R8** | **Rate limit escalando com nº de contas** — incomum e pode ser explorado (abrir contas = mais cota) ou confuso | Glossary | Definir modelo de quota explícito (por key + por tenant + por conta) com headers `X-RateLimit-*` e `Retry-After` padronizados |
| **R9** | **Sync de posts externos eventual (~90 min)** — janela de inconsistência: post feito no app nativo demora a aparecer; analytics/estado divergem | Glossary | Se formos sincronizar posts nativos, expor a latência no contrato (`status: pending_sync`) e permitir sync on-demand (`SyncExternalPosts`) |
| **R10** | **Contrato "verboso" e inconsistente**: respostas ora `{data: [...]}`, ora `{posts: [...]}`, ora `{post: {...}}`; `PATCH` para move + `PUT` para update sem convenção clara | SDKs/READMEs | **Envelope de resposta único** (`{data}`) + convenção REST estrita (PUT=full, PATCH=partial) + OpenAPI como teste de contrato |
| **R11** | **Estratégia de "16 plataformas" sem abstração visível** — cada plataforma tem settings específicos no mesmo objeto `platformSpecificData`; sem tipagem forte, erros de contrato migram para runtime | `posts.ts` (n8n), SDKs | Definir `platformSpecificData` como union tipada por plataforma no spec OpenAPI (discriminator), com validação server-side por plataforma |

---

## 12. Metodologia e fontes

**Fontes mineradas (todas via `cosca.web`):**
- `api.github.com/orgs/zernio-dev` e `/repos` (páginas 1–7; listagem truncada a 8k por chamada → inventário parcial, complementado por probes de README)
- READMEs raw: `n8n-nodes-zernio`, `zernio-python`, `zernio-node`, `zernio-rust`, `zernio-dotnet`, `zernio-php`, `zernio-java`, `zernio-ruby`, `zernio-go`, `late-node`, `zernflow`, `latewiz`, `social-media-api` (via org listing)
- Código raw: `n8n-nodes-zernio/master/late/{Late.node.ts, resources/posts.ts, resources/webhooks.ts, resources/clone.ts, resources/tools.ts}`; `zernflow/main/app/api/webhooks/late/route.ts`
- Docs: `docs.zernio.com/llms.txt` (índice), `docs.zernio.com/llms-full.txt` (primeiros 8k — Billing, Changelog, início do Glossary)
- Árvores git: `zernio-python` (develop), `n8n-nodes-zernio` (master)

**Limitações (honestidade epistêmica):**
- `cosca.web` limita o conteúdo a ~8k chars/requisição; arquivos grandes (README do Python 50 KB, `openapi.yaml` 2,4 MB, `llms-full.txt` completo) foram lidos **parcialmente** (sempre o início). Detalhes além do corte não foram verificados.
- `docs.zernio.com` é SPA Next.js/Mintlify: páginas `.mdx` devolvem o shell (sem conteúdo). Só `llms.txt`/`llms-full.txt` renderizam markdown.
- Lista de 27 repos: 20 identificados diretamente; ~7 não enumerados (provavelmente SDKs/ferramentas menores) — não afeta as conclusões de arquitetura.
- O `openapi.yaml` de 2,4 MB é a fonte definitiva do contrato e **não foi integralmente lido**; recomenda-se baixá-lo localmente numa fase futura para gerar o mapa completo de rotas/schemas.
- Marcações `[inferido]` (ex.: MongoDB) são deduções de evidências parciais.

---

*Relatório gerado por cosca-architecture — mineração externa para blueprint da nossa plataforma. Nenhum código produzido, nenhum repositório clonado, nenhuma alteração além deste arquivo.*
