# Mineração dos SDKs Zernio — Padrões de Integração com Redes Sociais

> **Missão**: extrair padrões de integração (OAuth, publicação, mídia, erros) dos SDKs oficiais da organização GitHub `zernio-dev` para embasar a NOSSA plataforma de gestão multi-rede.
> **Método**: pesquisa read-only via API pública do GitHub + `raw.githubusercontent.com` (nenhum clone, nenhuma alteração de workspace de código).
> **Data**: 2026-09-01 | **Agente**: cosca-integrations | **Nível de confiança**: alto (fontes primárias = READMEs, regras oficiais `zernio-api`/`social-media-api-best-practices` e código-fonte dos SDKs).

**Proveniência**: trace IDs do flight recorder: `TRACE-20260902-F45479AB`, `TRACE-20260902-FC1A89B4`, `TRACE-20260902-3F56772F`, `TRACE-20260902-6D27DF85`, `TRACE-20260902-6D1E288A`, `TRACE-20260902-10FDEA15`, `TRACE-20260902-32BFE493`, `TRACE-20260902-27CDD4C6`, `TRACE-20260902-D0270DE8`, `TRACE-20260902-B9316098`, `TRACE-20260902-DA7714FD`, `TRACE-20260902-84136958`, `TRACE-20260902-B70AE38E`, `TRACE-20260902-0956C803`, `TRACE-20260902-B55335CC`, `TRACE-20260902-D009AEB2`, `TRACE-20260902-E30A8136`, `TRACE-20260902-AC5351D7`, `TRACE-20260902-43803885`, `TRACE-20260902-2FF317A9` + chamadas read-only à API GitHub (árvores git).

---

## 0. Inventário minerado (27 repos)

| Repo | Papel | Destaque |
|---|---|---|
| `zernio-api` | Skill Claude Code com a referência oficial da API | `SKILL.md` + `rules/*.md` (authentication, errors, media, platforms, connect, webhooks, posts…) |
| `social-media-api-best-practices` | Skill com padrões "battle-tested" de 13 APIs sociais | OAuth por plataforma, rate limits, mídia, erros, quirk do Instagram 2207051 |
| `zernio-python` | SDK Python (pkg `zernio-sdk`) | client + resources + upload smart + MCP server + pipelines |
| `zernio-node` | SDK Node/TS (`@zernio/node`) | cliente manual sobre SDK gerado via openapi (hey-api) |
| `zernio-rust`, `zernio-dotnet`, `zernio-java`, `zernio-php`, `zernio-ruby`, `zernio-go` | SDKs gerados por OpenAPI Generator | mesma API, 6 linguagens |
| `n8n-nodes-zernio` | Node comunitário n8n | operações por plataforma, planos, erros (é a melhor "camada de UX" da API) |
| `openapi-specs` | Specs OpenAPI das plataformas | 761 endpoints / 13 plataformas (twitter.yaml, linkedin.yaml, telegram.yaml…) |
| `zernio-cli`, `latewiz`, `zernflow`, `unified-inbox`, `ads-dashboard`, `convex-zernio`, `social-media-api` (drop-in Ayrshare), `cursor-plugin`, `zernio-claude-plugin` | Aplicações exemplos | mostram casos de uso reais |
| `TikTok-Api` | Wrapper não-oficial TikTok | gap de API oficial |
| `zernio-shopify` | **DISCONTINUED** | superseded por app Shopify — lição de ciclo de vida |

Observação histórica importante: a empresa se chamava **Late** (`late`, `LateApiError`, `LATE_API_KEY`, `n8n-nodes-late`) e foi rebatizada para **Zernio** mantendo 100% de compatibilidade (aliases `Zernio* = Late*`, env fallback). Isso aparece em todos os SDKs — lição de branding/churn.

---

## 1. Padrão de autenticação da API

### 1.1 A API Zernio em si — API Key (não é OAuth)

- **Formato**: chave com prefixo `sk_`, **67 caracteres**, criada no dashboard (`zernio.com/dashboard/api-keys`).
- **Header**: `Authorization: Bearer sk_...` em **toda** chamada (nunca via query string).
- **Env var**: `ZERNIO_API_KEY` (fallback legado `LATE_API_KEY`).
- **Base URL**: `https://zernio.com/api`; **todos os endpoints versionados no path** `/v1`. Quebras de compatibilidade só acontecem em versão de path nova (ex.: `/v2`); operações deprecadas marcadas `deprecated: true` e anunciadas no changelog antes da remoção.
- **Verificação de credencial**: `GET /v1/auth/verify` → `VerifyCredential200Response` (usado no handshake/onboarding).
- **Ciclo de vida**: `GET/POST /v1/api-keys`, `DELETE /v1/api-keys/{keyId}`, sem expiração própria (revogação manual).

**O ponto-chave**: o cliente NUNCA faz OAuth com as plataformas. O OAuth é **centralizado no servidor Zernio** — o cliente só troca uma API key por operações. Os SDKs são "clientes finos" de REST.

### 1.2 OAuth (lado servidor) — o que a plataforma expõe para o cliente conectar contas

Fluxo padrão (`rules/connect.md`):

1. **Iniciar**: `GET /v1/connect/{platform}?profileId=...&callbackUrl=...` → retorna `{ "url": "https://twitter.com/oauth/..." }` (redirect do usuário).
2. **Callback**: `POST /v1/connect/{platform}` (troca `code` + `state`).
3. **Seleção de página/org/localização/board/perfil** (Facebook, LinkedIn, GBP, Pinterest, Snapchat): endpoints `select-*` — `GET` lista entidades, `POST` conclui com `pendingDataToken` (preferido) ou `tempToken` (legado).
4. **Headless**: `GET /v1/connect/{platform}?headless=true` → o usuário volta ao seu `redirect_url` com `pendingDataToken=...&step=select_page`. Depois `GET /v1/connect/pending-data?token=...` **entrega os dados brutos OAuth; uso único, expira em 10 min, sem auth**. Em fluxos via API key, o token vai no header **`X-Connect-Token`**.

Fluxos especiais por plataforma:

| Plataforma | Método de conexão | Detalhe |
|---|---|---|
| Twitter/X | OAuth 2.0 **PKCE** | S256 obrigatório; verifier embutido no state: `${state}-cv_${codeVerifier}` |
| Instagram/Facebook | OAuth 2.0 | Instagram: troca em 2 passos (short-lived 1h → long-lived 60d) |
| LinkedIn | OAuth 2.0 | seleção de org; headers `X-RestLi-Protocol-Version: 2.0.0` e `LinkedIn-Version: 202511` |
| TikTok | OAuth 2.0 | exige UX compliance + auditoria da app |
| YouTube | Google OAuth | requer `access_type=offline` + `prompt=consent` |
| Reddit | OAuth 2.0 | `duration=permanent`; **User-Agent descritivo obrigatório** |
| Bluesky | **App password** (sem OAuth) | `POST /v1/connect/bluesky/credentials` `{identifier, appPassword}` |
| Telegram | **Bot token** (sem OAuth) | fluxo por código + polling (ver abaixo) |
| WhatsApp | Meta Cloud API | `POST /v1/connect/whatsapp/credentials` ou Embedded Signup + seleção de número |
| Snapchat | OAuth 2.0 **allowlist-only** | exige aprovação da equipe Snapchat |
| Discord | OAuth 2.0 | webhook do bot por canal |

**Fluxo Telegram (exemplo didático — o único "OAuth" por código)**:
1. `GET /v1/connect/telegram?profileId=...` → `{ code: "LATE-ABC123", botUsername: "LateScheduleBot" }`.
2. Usuário adiciona o bot ao canal e envia o código para ele.
3. `PATCH /v1/connect/telegram?code=LATE-ABC123` (polling) → `{status:"pending"}` ou `{status:"connected", account:{...}}`.
4. Alternativa power-user: `POST /v1/connect/telegram` `{profileId, chatId}` direto (bot já deve ser admin do canal).

**Conexão de Ads** (`GET /v1/connect/{platform}/ads`):
- **Same-token** (facebook, instagram, linkedin, pinterest): copia o token OAuth da conta de postagem e cria conta de ads (`metaads`, `linkedinads`, `pinterestads`) — sem OAuth extra.
- **Separate-token** (tiktok, twitter): inicia OAuth do marketing API da plataforma; **requer `accountId`** da conta de postagem; cria `tiktokads`/`xads` com token próprio.
- **Standalone** (googleads): OAuth Google Ads independente.
- Sem o add-on de Ads: `403`.

### 1.3 Detalhes OAuth por plataforma (do `social-media-api-best-practices/SKILL.md`)

| Plataforma | Grant/Fluxo | Tokens | Scopes essenciais |
|---|---|---|---|
| Instagram | `ig_exchange_token` (2 passos) | 1h → 60d | `instagram_business_basic`, `_content_publish`, `_manage_messages`, `_manage_comments`, `_manage_insights` |
| Twitter/X | PKCE S256 | access 2h, refresh longo | `tweet.read`, `tweet.write`, `users.read`, `offline.access`, `media.write`, `dm.read`, `dm.write` |
| TikTok | standard + auditoria | long-lived | `user.info.basic/profile/stats`, `video.publish`, `video.upload`, `video.list` |
| LinkedIn | standard | access 60d, refresh 365d | pessoal: `w_member_social`, `w_member_social_feed`, `r_member_postAnalytics`; org: `w_organization_social`, `r_organization_admin`, `r_organization_social_feed` |
| YouTube | Google, offline+consent | ~1y | `youtube.upload`, `youtube`, `youtube.force-ssl`, `yt-analytics.readonly` |
| Facebook | page token via `/me/accounts` | 60d+ | `pages_manage_posts`, `pages_show_list`, `pages_read_engagement`, `business_management`, `pages_messaging` |
| Threads | `th_exchange_token` | 60d | `threads_basic`, `threads_content_publish`, `threads_manage_replies`, `threads_read_replies`, `threads_manage_insights`, `threads_delete` |
| Pinterest | standard + refresh | — | `pins:read`, `pins:write`, `boards:read`, `boards:write`, `user_accounts:read` |
| Reddit | `duration=permanent` | permanente | `identity`, `submit`, `read`, `mysubreddits`, `history`, `edit`, `vote` |
| Bluesky | AT Protocol (login app-password) | `accessJwt`+`refreshJwt`, auto-refresh em `ExpiredToken` | — (DIDs, PDS) |
| Snapchat | basic, allowlist | — | `snapchat-profile-api` |

---

## 2. Plataformas × operações × requisitos

Zernio publica em **15 plataformas** + ads em **7 redes**. Modelo de dados: `Profile (marca/projeto) → Account (conta conectada) → Post (publicação multi-plataforma)`.

### 2.1 Operações por plataforma (`rules/platforms.md` + README n8n)

| Plataforma | Operações suportadas | `platformSpecificData` relevante | Requisitos / riscos |
|---|---|---|---|
| **Twitter/X** | Posts, threads, imagens, vídeos, DM, retweet/bookmark/follow/search | `threadItems[]` (threads), poll, sensitiveMedia | Conta standard; PKCE; free tier não edita/unpublish; 3 níveis de rate limit |
| **Instagram** | Feed, Stories, Reels, carrossel, DMs, insights, ice-breakers, follow-status | `contentType:"story"`, `firstComment`, `collaborators[]`, `userTags[]`, `shareToFeed`, `trialParams` (Trial Reels), `audioName`, `thumbOffset` | **Business account OBRIGATÓRIA** (Personal/Creator não suportadas); 100 posts/dia |
| **Facebook** | Pages, Stories, Reels, menu persistente, DMs | `contentType:"story"`, `pageId`, `firstComment` | **Admin da page**; page token separado; 10 imagens |
| **LinkedIn** | Posts, imagens, vídeos, PDF, mentions, analytics | `firstComment`, `disableLinkPreview`; 20 imagens; PDF 100MB | Org exige admin; headers de versão; URN `urn:li:person:`/`urn:li:organization:` |
| **TikTok** | Vídeos/photo carousel com privacy, draft (Creator Inbox) | `privacyLevel`, `allowComment/Duet/Stitch`, `contentPreviewConfirmed` (req.), `expressConsentGiven` (req.), `commercialContentType`, `videoMadeWithAi`, `videoCoverTimestampMs`, `photoCoverIndex`, `autoAddMusic`, `draft`, `description` (4000) | **Auditoria da app (UX compliance)**; app não auditada só posta privado; 5 uploads pendentes/24h |
| **YouTube** | Vídeos, Shorts (≤3min auto-detect), playlist, caption, analytics | `title`, `visibility`, `firstComment` (≤10k), `containsSyntheticMedia`; `thumbnail` na mediaItem; `tags` top-level (≤500 chars) | Upload resumable; quota diária |
| **Pinterest** | Pins, boards, board selection | `title`, `boardId`, `link`, `coverImageUrl`, `coverImageKeyFrameTime` | Business recomendada; cover image p/ vídeo |
| **Reddit** | Posts, search, feed, subreddit rules, flairs | `subreddit`, `title`, `flairId`, `url`, `forceSelf`, `nativeVideo`, `videogif`, `videoPosterUrl` | **User-Agent descritivo obrigatório**; flair às vezes obrigatório; `NO_LINKS` em alguns subs |
| **Bluesky** | Posts, threads, imagens, vídeos | `threadItems[]` (threads) | App password; AT Protocol facets byte-offset |
| **Threads** | Posts, threads (reply chains), carrossel 10 | `threadItems[]` | Rate limit agressivo; igual Instagram no OAuth |
| **Google Business** | Updates, fotos, reviews, food menus, atributos, place actions, verificação | `callToAction {type,url}` (BOOK/ORDER/SHOP/LEARN_MORE/SIGN_UP/CALL) | Business Profile owner/manager; verificação de localização |
| **Telegram** | Mensagens, albums (10), comandos do bot | `parseMode` (HTML/Markdown/MarkdownV2), `disableWebPagePreview`, `disableNotification`, `protectContent` | Bot token `BOT_ID:SECRET`; bot admin do canal; 4096 chars texto / 1024 caption; HTML = subconjunto |
| **WhatsApp** | Templates, Flows, phone numbers, group chats, broadcasts, DMs | — | Meta Cloud API; KYC de números; perfis Business |
| **Snapchat** | Stories, Spotlight, perfil público | `contentType` (story/saved_story/spotlight; saved_story title ≤45, spotlight desc ≤160) | **Allowlist da Snapchat**; mídia criptografada AES-256-CBC |
| **Discord** | Webhook posts, embeds (10), polls, forum threads, announcements, crosspost | `channelId` (req.), `embeds[]`, `poll`, `tts`, `webhookUsername/AvatarUrl`, `forumThreadName`, `forumAppliedTags`, `threadFromMessage` | Bot por canal; 2000 chars texto; 10 files × 25MB |

### 2.2 Requisitos de conta/review por plataforma (gaps que Zernio tem que contornar)

- **Instagram**: só Business → o app da Zernio não atende contas pessoais (gap de mercado = oportunidade se resolvermos via Graph API pessoal? não existe — é limitação da Meta).
- **Snapchat**: allowlist-only (aprovação manual da Snapchat, meses) + criptografia AES — integração cara de manter.
- **TikTok**: auditoria de UX compliance (privacy selector, toggles comment/duet/stitch, disclosure de conteúdo comercial, preview, express consent) — app em modo dev só posta privado.
- **Facebook/LinkedIn/GBP**: exigem admin/owner + seleção de página/org/localização.
- **Google Business**: verificação de localização (fluxo próprio: `StartGoogleBusinessVerification`).
- **Reddit**: UA descritivo + regras de subreddit (flairs).
- **YouTube**: quota + resumable.
- **Telegram/Bluesky/WhatsApp**: sem OAuth clássico — fluxos por token/app-password/credentials.
- **Twitter/X**: planos de API pagos (usage tiers 280/4000/25000 chars; endpoints de pricing `getXApiPricing`).

---

## 3. Fluxo de upload de mídia (normal vs presign)

Dois endpoints oficiais (`rules/media.md`):

| Fluxo | Endpoint | Limite | Retenção | Uso típico |
|---|---|---|---|---|
| **Presign** | `POST /v1/media/presign` `{filename, contentType}` | **5 GB** | permanente (storage de mídia do post) | vídeos e arquivos grandes |
| **Direto** | `POST /v1/media/upload-direct` (multipart `-F file=`) | **25 MB** | **7 dias** (auto-delete) | inbox/DMs, arquivos pequenos |

**Fluxo presign** (documentado no README do Node SDK e media.md):

```
1. POST /v1/media/presign  { filename, contentType }   → { uploadUrl, fileUrl }
2. PUT uploadUrl  (body = arquivo, header Content-Type) → 200
3. Usar fileUrl/publicUrl em mediaUrls[] ou mediaItems[]
   no createPost  (ex.: { type:"video", url: fileUrl, thumbnail:"..." })
```

- `uploadUrl` e `fileUrl` vêm juntos; a plataforma (Zernio) guarda e serve o `publicUrl`.
- O Python SDK tem um **"smart uploader"** que decide por tamanho (`upload/smart.py`): **< 4 MB → direct multipart** na API; **≥ 4 MB → presign/Vercel Blob** (requer token `vercel_blob_rw_...`, teto 5 GB). Erro `LargeFileError` orienta o usuário quando não há token. No SDK Node atual o caminho é só `getMediaPresignedUrl` — ou seja, houve evolução: Vercel Blob → presign próprio. **Para nós: fazer presign próprio (S3/R2) desde o início.**
- Callbacks de progresso (`on_progress`) e versões sync/async (`upload`/`aupload`, `upload_multiple`/`aupload_multiple`).

**Formatos aceitos (API Zernio)**: imagens JPG/PNG/WebP/GIF (5MB), vídeos MP4/MOV/WebM (**5GB**), documentos PDF (100MB, só LinkedIn).

**Limites por plataforma (crítico para o validador de mídia)** — tabela consolidada (SKILL best-practices + media.md):

| Plataforma | Imagem máx | Vídeo máx | Aspecto | Especial |
|---|---|---|---|---|
| Instagram | 8MB | 100MB stories / 300MB reels | feed 4:5–1.91:1; story 9:16 | 10 itens carrossel |
| TikTok | 20MB | 4GB, 3s–10min | 9:16 estrito | 35 foto carrossel |
| Twitter/X | 5MB | 512MB, 2min20s | flexível | 1–4 imagens |
| LinkedIn | 8MB | 5GB | flexível | 20 carrossel; PDF 100MB |
| YouTube | 2MB (thumbnail) | 256GB | 16:9 | resumable |
| Facebook | 10MB | 4GB | flexível | 10 multi-imagem |
| Threads | 8MB | 1GB, 5min | 9:16 | 10 carrossel |
| Pinterest | 32MB | 2GB | flexível | cover obrigatória p/ vídeo |
| Bluesky | 1MB | 50MB, 3min | flexível | máx 4 imagens |
| Snapchat | 20MB | 500MB | 9:16 | **AES-256-CBC** |
| Google Business | 5MB | — | flexível | só imagens |
| Reddit | 20MB | — | flexível | via URL / native video |
| Telegram | 10MB | 50MB | flexível | album 10; 4096 chars |

**Regras de ouro de mídia** (SKILL best-practices):
- **Stream, nunca carregar arquivo inteiro em memória** (`fetch(url).body` direto).
- **Hosts problemáticos** (Google Drive, Docs, Dropbox, OneDrive/1drv.ms) retornam HTML em vez de mídia → **re-host para o próprio storage antes de publicar**. Fix do Dropbox: `dl=0` → `dl=1`.
- **Upload chunked do Twitter**: `INIT → APPEND (4MB chunks, base64) → FINALIZE → poll STATUS` com `check_after_secs`.
- **Upload resumable do YouTube** para arquivos grandes.
- **Polling de processamento**: estado `succeeded`/`failed`, timeout ~5min.

---

## 4. Tratamento de erros e códigos

### 4.1 Envelope de erro da API (unificado)

```json
{ "error": "Invalid API key", "code": "UNAUTHORIZED", "details": {} }
```

- **`code`** = machine-readable (UPPER_SNAKE), **`error`** = humano, **`details`** = contexto (ex.: `{fields: {email: ["must be valid"]}}` na validação; `{param, code}` nos erros de validação Python).

### 4.2 Status codes (rules/errors.md + errors.ts do Node SDK)

| Status | Code | Significado | Helper no SDK |
|---|---|---|---|
| 400 | `BAD_REQUEST` | parâmetros inválidos / validação de request | `ValidationError` com `details.fields` |
| 401 | `UNAUTHORIZED` | API key inválida/ausente | `isAuthError()` |
| 402 | `PAYMENT_REQUIRED` | — | `isPaymentRequired()` (existe no SDK Node) |
| 403 | `FORBIDDEN` | sem permissão; **plan limits excedidos** (n8n); sem add-on Ads | `isForbidden()` |
| 404 | `NOT_FOUND` | recurso inexistente | `isNotFound()` |
| 422 | `VALIDATION_ERROR` | validação de negócio falhou | — |
| 429 | `RATE_LIMITED` | muitos requests | `RateLimitError` com `limit/remaining/resetAt` |
| 500 | `INTERNAL_ERROR` | erro servidor | — |

**Hierarquia de erros nos SDKs** (cópia direta para o nosso):

- Node (`src/errors.ts`): `ZernioApiError` (base: `statusCode`, `code`, `details`, métodos `isRateLimited()`, `isAuthError()`, `isForbidden()`, `isNotFound()`, `isValidationError()`, `isPaymentRequired()`) → `RateLimitError` (com `getSecondsUntilReset()`) e `ValidationError` (com `fields`); `parseApiError()` lê os headers `X-RateLimit-*` e vira `RateLimitError`.
- Python (`client/exceptions.py`): `LateError` → `LateAPIError` (com `__str__` que inclui `[status] mensagem (field: X; code: Y)`) → `LateAuthenticationError` (401), `LateRateLimitError` (429, com `reset_time`), `LateNotFoundError` (404), `LateForbiddenError` (403); mais `LateValidationError`, `LateConnectionError`, `LateTimeoutError`. Aliases `Zernio*`.

### 4.3 Rate limits (da própria API e das plataformas)

**API Zernio** — por plano (rpm): Free 60, Build 120, Accelerate 600, Unlimited 1.200. Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` (unix). Retry: esperar `max(reset - now, 1000ms)` e refazer.

**Rate limiter client-side** (Python `rate_limiter.py`): classe que lê os 3 headers a cada resposta, `should_wait()` quando `remaining <= 0`, `get_wait_time()` = segundos até reset + 1s buffer (default 60s).

**Backoff exponencial** (SKILL best-practices): retryable = `429, 500, 502, 503`; `base 5000ms × 2^attempt`, cap 30s, máx 3 tentativas.

**Limites por plataforma**:

| Plataforma | Limite-chave | Estratégia |
|---|---|---|
| Instagram | 100 posts/dia/conta | fila + espaçar |
| TikTok | 5 uploads pendentes/24h | esperar processamento |
| Twitter/X | **3 níveis**: app (24h) + user (24h) + endpoint | checar os 3 (headers `x-app-limit-24hour-*`, `x-user-limit-24hour-*`) |
| LinkedIn | geral generoso, bulk estrito | batch cuidadoso |
| YouTube | quota diária por canal | monitorar |
| Reddit | 60 req/min | respeitar headers |
| Threads | agressivo | fail fast |

### 4.4 Erros específicos de plataforma (códigos nativos que vazam no `details`/logs)

- **Instagram (Meta)** `2207xxx`: `2207001` spam, `2207003` media download timeout (retry), `2207004` imagem >8MB (comprimir), `2207006` media não encontrada, `2207026` formato não suportado, `2207042` 100 posts/dia, `2207050` usuário restrito, **`2207051` "bloqueado mas pode ter publicado!"** (anti-spam: verificar mídia recente após 5s antes de marcar falha), `2207052` media fetch falhou (usar URL direta).
- **TikTok**: `access_token_invalid`, `scope_not_authorized`, `rate_limit_exceeded`, `spam_risk_*`, `file_format_check_failed`, `unaudited_client_can_only_post_to_private`.
- **Twitter/X**: `invalid_grant` (token revogado → reconectar), `usage-capped`, `duplicate`, `186` (tweet longo).
- **Bluesky**: `ExpiredToken` (auto-refresh JWT), `InvalidToken`, `XRPCNotSupported` (app password sem DM).
- **Reddit**: `invalid token`, `not allowed to submit`, `RATELIMIT`, `NO_LINKS`.
- **Telegram**: `message too long` (>4096), `chat not found`, `bot was blocked`.

### 4.5 Falhas parciais, logs e saúde

- Post tem **status por plataforma**: `{platform, status: published|failed, publishedUrl, error}` — cross-posting gera `post.partial` quando algumas plataformas falham (webhook).
- **Logs de publicação retidos 7 dias**: `GET /v1/posts/{postId}/logs`, `GET /v1/logs`, `GET /v1/logs/{logId}`.
- **Health de contas**: `GET /v1/accounts/health` e `GET /v1/accounts/{accountId}/health` — detecta token expirado/desconectado proativamente; webhook `account.disconnected` com `disconnectionType` e `reason` (ex.: "Token expired").
- **Retry**: `POST /v1/posts/{postId}/retry` para post falho. **Unpublish**: `POST /v1/posts/{postId}/unpublish`. **Edit**: `POST /v1/posts/{postId}/edit` (suporte varia; Twitter free tier não permite). **Update-metadata**: `POST /v1/posts/{postId}/update-metadata`.

### 4.6 Quotas/planos (rules/errors.md + README n8n)

- **Planos Zernio**: Free (10 posts/mês, 2 Social Sets, $0), Build (120 posts, 10 sets, $13/mo), Accelerate (posts ilimitados, 50 sets, $33/mo), Unlimited (ilimitado, $667/mo). Add-ons: Analytics, Comments+DMs, Ads.
- **Endpoint de uso**: `GET /v1/usage-stats` → `{planName, billingPeriod, signupDate, billingAnchorDay, limits:{uploads, profiles,...}, usage:{uploads, profiles, lastReset}}`. **Use `usage.lastReset` + `billingAnchorDay` para saber quando o ciclo vira** — pre-flight de quota na UI ("X de Y usados neste ciclo").
- Node SDK expõe billing detalhado: `usage.getBilling()` (plan, cycle, balance, caps, status), `getCallsUsage()`, `getSmsUsage()`, `getUsage()` (metering), `getUsageStats()`, `getXApiPricing()`.

### 4.7 Webhooks

- Endpoints: `GET/POST/PUT/DELETE /v1/webhooks/settings`, `POST /v1/webhooks/test`, `GET /v1/webhooks/logs` (+ redelivery).
- Config: `{name, isActive, url, secret, events[]}`.
- **Eventos**: `post.scheduled`, `post.published`, `post.failed`, `post.partial`, `account.connected`, `account.disconnected` (token expirado), `message.received` (DMs Instagram/Facebook/Telegram).
- **Verificação de assinatura**: header `x-zernio-signature`, HMAC-SHA256 do body bruto com o `secret`, comparar com `timingSafeEqual`.

---

## 5. Estrutura/organização dos SDKs (o que copiar no nosso SDK)

### 5.1 Padrão geral — "OpenAPI é a fonte de verdade"

Todos os SDKs (Node, Python, Rust, .NET, Java, PHP, Ruby, Go) são **gerados a partir de uma spec OpenAPI da API** (`openapi.yaml` no repo Python; `openapi-specs/` para as specs das plataformas). Os SDKs não-Node (Rust/.NET/etc.) são 100% OpenAPI Generator. Node e Python são **gerados + camada manual por cima**.

### 5.2 Node SDK (`zernio-node`) — o mais limpo

```
src/
  client.ts          # Zernio class: apiKey, baseURL, timeout (60s default), defaultHeaders
  errors.ts          # ZernioApiError + RateLimitError + ValidationError + parseApiError
  generated/
    sdk.gen.ts       # ~800 funções geradas (createPost, listAccounts, getMediaPresignedUrl…)
    types.gen.ts     # tipos TS gerados (OpenAPI → TypeScript)
  index.ts           # re-export
scripts/
  generate-client.ts         # regenera a partir da spec
  generate-endpoint-tests.ts # gera testes por endpoint
  generate-readme-reference.ts
examples/create-post.ts
tests/  (client, endpoints, errors, isolation, request-init, deno smoke)
```

- Cliente HTTP: **`@hey-api/client-fetch`** (config com `auth` no header). API chama `{ body, query }` e devolve `{ data, error }`.
- **Convenção de nomes**: recurso em camelCase (`posts.createPost`, `analytics.getAnalytics`, `connect.getConnectUrl`) espelhando o REST (`POST /v1/posts`, `GET /v1/analytics`).
- "Resource" = agrupador de operações sobre um domínio (posts, accounts, profiles, analytics, queue, webhooks, media, connect, usage, logs, apiKeys, users, adAccounts…). É só agrupamento de funções, sem classe por recurso.

### 5.3 Python SDK (`zernio-python`) — o mais rico em padrões

```
src/late/
  client/          base.py, late_client.py, exceptions.py, rate_limiter.py
  resources/       _generated/*.py (1 por recurso, gerado) + overrides manuais (accounts, posts, media, profiles, queue, tools, users, ads, analytics)
  models/          _generated/models.py + responses.py
  upload/          config.py, direct.py, smart.py, protocols.py, utils.py, vercel/{client,uploader}.py
  pipelines/       cross_poster.py, csv_scheduler.py
  ai/              content_generator.py, protocols.py, providers/openai.py
  mcp/             server.py, http_server.py, auth.py, generated_tools.py, tool_definitions.py (SDK vira MCP server!)
  enums.py
src/zernio/__init__.py   # namespace novo, re-exporta de late (compat)
scripts/          generate_*.py (models, resources, mcp_tools, mcp_docs, readme_reference, examples)
examples/         01_basic_usage.py … 05_download_tools.py, publish_post.py, data/sample_posts.csv
tests/            test_client, test_integration, test_generated_request_bodies, test_mcp_*…
```

- **Resource base** (`resources/base.py`) com sync + async (`alist`, `acreate`…) — paridade total sync/async.
- **Rate limiter** embutido no client.
- **Pipelines prontos** (cross-poster, CSV scheduler) — boa ideia para nossa plataforma (ex.: importar CSV e distribuir).
- **MCP server embutido** — o SDK também é um servidor MCP (stdio + streamable HTTP). Padrão a considerar para nossos conectores (AI-first).

### 5.4 n8n node (`n8n-nodes-zernio`) — a camada de UX da API

```
credentials/LateNode.credentials.ts   # credencial "Zernio API" (só API key)
late/Late.node.ts                     # nó raiz (resource/operation dispatch)
late/resources/*.ts                   # 1 módulo por resource (accounts, analytics, connect, facebook,
                                      #   googlebusiness, linkedin, media, pinterest, posts, profiles,
                                      #   queue, reddit, snapchat, telegram, usage, users, webhooks…)
late/utils/                           # commonFields, expressionProcessor, fieldBuilders, nodeBuilder,
                                      #   platformHelpers, routingHooks
late/types.ts
```

- Padrão **resource × operation** = espelho direto do REST — nosso SDK de conectores deve seguir o mesmo desenho (facilita gerar UIs de automação depois).
- Requisitos de plataforma, planos e erros documentados no README = documentação executável.

### 5.5 Specs por plataforma (`openapi-specs`) — 761 endpoints

Pinterest 236, Twitter/X 147, Reddit 50, YouTube 42, Instagram 36, Telegram 34, Bluesky 34, Threads 29, Google Business 48, Facebook 28, Snapchat 27, LinkedIn 27, TikTok 23. **Fonte direta para montar nossos conectores** (não reimplementar da documentação solta).

### 5.6 O que copiar no NOSSO SDK (checklist)

1. **Spec OpenAPI como fonte de verdade** + geração de client (Node: hey-api; Python: requests/httpx gerado) + testes gerados por endpoint.
2. **Estrutura de recursos espelhando REST**: `posts`, `accounts`, `connect`, `media`, `queue`, `webhooks`, `usage`, `logs`.
3. **Erros tipados por status** com `code` machine-readable + `details`; helpers `isAuthError()/isForbidden()/isRateLimited()`; `RateLimitError` com `getSecondsUntilReset()`.
4. **Envelope de resposta uniforme** (`{data, error}`) e **input `{body, query}`**.
5. **Paridade sync/async** (Python `alist`/`aupload`; TS top-level await).
6. **Rate limiter client-side** lendo `X-RateLimit-*` e bloqueando antes de 429.
7. **Uploader inteligente**: <4MB direto, ≥4MB presign (5GB), streaming, `on_progress`, `upload_multiple`.
8. **Aliases de compatibilidade** para rebranding (Late→Zernio) — útil se renomearmos produtos.
9. **Documentação-as-código** (SKILL.md + rules/*.md) versionada junto.
10. **Endpoints de uso/quotas** (`usage-stats` com `lastReset`+`billingAnchorDay`) para pre-flight de planos.

---

## 6. Lições práticas para integrar NA NOSSA plataforma

### 6.1 Twitter/X

1. **PKCE S256 obrigatório**; guarde o `code_verifier` no `state` (`${state}-cv_${verifier}`) para o callback — não dá para confiar em sessão.
2. Scopes mínimos: `tweet.write`, `media.write`, `offline.access`, `users.read`; token de acesso dura **2h** → arquitetura de refresh obrigatória (job de refresh + webhook `account.disconnected`).
3. **Threads**: `platformSpecificData.threadItems[]` com `{content, mediaItems[]}` — cada item é um tweet; nosso modelo de post deve aceitar lista.
4. **Rate limit de 3 níveis** (app 24h, user 24h, endpoint) — ler `x-app-limit-24hour-remaining/reset` e `x-user-limit-24hour-*`, não só `x-rate-limit-*`.
5. **Vídeo**: chunked upload (INIT/APPEND 4MB/FINALIZE) + polling de `processing_info` (`check_after_secs`) — não assumir sucesso imediato; limites 512MB/2min20s.
6. **Contagem de caracteres**: URLs = 23 chars sempre; emoji/CJK = 2; limites por tier (free 280 / premium 4000 / premium+ 25000).
7. Erros: `invalid_grant` → reconectar conta; `duplicate` → não retry automático; `186` → truncar. **Editar/unpublish não existe no free tier.**
8. **Custo**: API do X é paga por uso (endpoint de pricing `getXApiPricing`) — decidir repasse/limite antes.

### 6.2 LinkedIn

1. **Headers obrigatórios**: `X-RestLi-Protocol-Version: 2.0.0` e **`LinkedIn-Version` por endpoint** (202511, algumas 202505/202401) — o versionamento do LinkedIn é por header, não por path; manter tabela de versões.
2. **Personal vs Organization**: `urn:li:person:` vs `urn:li:organization:` — scopes e endpoints diferentes; fluxo de seleção de org (`connect.listLinkedInOrganizations` + `selectLinkedInOrganization`); analisar org exige admin.
3. **Token**: access 60d + refresh 365d.
4. **Escape de texto**: reservados `| { } [ ] ( ) < > # \ * _ ~` — **não escapar `@`** (senão vira `\@` no post); preservar menções URN `@[Nome](urn:li:person:ID)` e hashtags.
5. **Mídia**: até 20 imagens; PDF único 100MB; `firstComment`; `disableLinkPreview`.
6. **Analytics ricos**: aggregate, org aggregate, post reactions, mentions (resolver `getLinkedInMentions`).
7. Erros: bulk posting estrito — batch com cuidado; `LinkedIn-Version` errada = 400 genérico.

### 6.3 Telegram

1. **Sem OAuth**: fluxo por **código + bot** (gera `code` tipo `APP-ABC123`, usuário manda pro bot, polling `PATCH`) **ou chatId direto** (power user, bot já admin). Escolher o UX de conexão: código é mais amigável para não-técnicos.
2. **Bot token** `BOT_ID:SECRET` — nunca expor em client; guardar criptografado no servidor.
3. **Parse mode**: só subconjunto HTML (`<b> <i> <u> <s> <code> <pre> <a> <tg-spoiler> <blockquote>`) — sanitizar HTML antes de enviar; `MarkdownV2` tem escaping próprio agressivo.
4. **Limites**: 4096 chars texto, 1024 caption, albums de até 10 mídias (enviar como grupo), 50MB vídeo.
5. **Comandos do bot**: set/get `telegram-commands` por conta.
6. Erros: `message too long` (truncar), `chat not found` (validar chatId), `bot was blocked` (não retry).
7. **Webhook de DMs**: `message.received` cobre Telegram → usar para inbox unificado.

### 6.4 Instagram / YouTube / TikTok (fase 2)

- **Instagram**: exigir **Business account** no onboarding (validação antecipada = menos falha na hora de publicar); 2-step token exchange (`ig_exchange_token`); tipar contentType `story` explícito, feed/reel auto por mídia; `userTags` com coordenadas normalizadas; **IDs 17+ dígitos → tratar como string** (`safeJsonParse`); erro **2207051 = verificar se publicou mesmo** (anti-spam); 100 posts/dia.
- **YouTube**: `access_type=offline` + `prompt=consent` no OAuth (senão não vem refresh token); Shorts auto-detect ≤3min; `visibility` (public/private/unlisted); thumbnail separada (2MB, JPG/PNG); `firstComment` até 10k; AI disclosure `containsSyntheticMedia`; upload resumable; quota diária.
- **TikTok**: **UX compliance completa antes da auditoria** (privacy selector sem default, toggles comment/duet/stitch, disclosure comercial, preview, express consent) — se a app não for auditada, posts só privados; `contentPreviewConfirmed` + `expressConsentGiven` = true obrigatórios; `draft:true` → Creator Inbox (bom para aprovação humana); `commercialContentType` (none/brand_organic/brand_content) + `videoMadeWithAi` por política; foto carrossel 35 itens com `photoCoverIndex` e `autoAddMusic`.

### 6.5 Arquitetura recomendada para os NOSSOS conectores (síntese)

```
nossa-plataforma/
  connect/            # orquestra OAuth de cada plataforma (server-side)
    twitter.ts        # PKCE + state-cv_ + refresh job + 3-tier rate limits
    linkedin.ts       # X-RestLi + LinkedIn-Version + urn org/person
    telegram.ts       # bot code flow + chatId + parseMode
  publish/            # posts.create com platforms[] + platformSpecificData
  media/              # presign próprio (S3/R2): <25MB direto, >=4MB presign; validador por plataforma
  errors/             # envelope {error, code, details} + erros tipados + retry 429/5xx
  webhooks/           # HMAC-SHA256 + eventos post.*/account.* + redelivery
  usage/              # quota por plano (lastReset + billingAnchorDay) + pre-flight
  inbox/              # DMs/comentários unificados (message.received)
```

---

## 7. Riscos e armadilhas

1. **Zernio centraliza o OAuth**: os SDKs **não revelam** como cada plataforma é integrada (só `connect` + seleção). Para NÓS, o trabalho real é o backend OAuth por plataforma — o repositório `social-media-api-best-practices` e `openapi-specs` são o atalho (scopes, headers, tokens, limites, códigos de erro).
2. **Instagram 2207051**: "bloqueado" ≠ falhou — pode ter publicado. Sem verificação pós-erro, você gera **duplicados** no retry automático.
3. **Snapchat**: allowlist-only + AES-256-CBC — onboarding longo e mídia precisa ser criptografada. Custo alto; começar por último ou não começar.
4. **TikTok**: app não auditada posta só privado; auditoria exige UX compliance real (não dá para fazer headless). `contentPreviewConfirmed`/`expressConsentGiven` são hard-required.
5. **Instagram exige Business account** — um chunk grande de usuários (contas pessoais) fica de fora; comunicar cedo no onboarding.
6. **IDs gigantes** (Instagram/Facebook) estouram `Number.MAX_SAFE_INTEGER` em JSON.parse — parse seguro com string.
7. **Reddit** bloqueia User-Agent genérico (axios/curl) — UA descritivo obrigatório; subreddits podem bloquear links (`NO_LINKS`) e exigir flair.
8. **LinkedIn muda versão por endpoint** (`LinkedIn-Version`) — atualização contínua; header errado = 400 misterioso.
9. **Twitter/X é pago por uso** (tiers) — custo variável por post/API call; `usage.getXApiPricing()` existe justamente para isso.
10. **Mídia em Google Drive/Dropbox/OneDrive** não é publicável diretamente — re-host obrigatório; e **upload direto da Zernio expira em 7 dias** (só presign é durável).
11. **Versionamento no path** (`/v1`) é a convenção que evita quebra — adotar desde o dia 1 (e nunca mudar `/v1`).
12. **Rebranding Late→Zernio** gerou aliases em todos os SDKs — churn de naming custa caro; escolher nomes estáveis.
13. **Planos freemium apertados** (Free = 10 posts/mês) geram 403 — surfaced como "plan limits exceeded"; pre-flight com `usage-stats` evita frustração.
14. **`zernio-shopify` descontinuado** → superseded por app na loja da Shopify: distribuição via marketplace muda o jogo; planejar app/connector como produto, não script.

---

## 8. Conclusão

A Zernio vende "uma API para postar em tudo" e o **valor real está no backend**: OAuth server-side por plataforma (PKCE, token exchange, refresh), fila de publicação com status por plataforma e falhas parciais, presign de mídia (5GB), retry com backoff, webhooks assinados e health de contas. Os SDKs são clientes finos gerados por OpenAPI — a arquitetura de SDK deles é um **padrão a copiar** (spec-first, resources × operations, erros tipados, rate limiter, uploader smart, aliases de compatibilidade), e o `social-media-api-best-practices` + `openapi-specs` são **fontes diretas** de conhecimento de integração por plataforma. Para a nossa plataforma multi-rede, o caminho é: modelar `Profile → Account → Post` (multi-target), centralizar OAuth com refresh/health, presign próprio, envelope de erro `{error, code, details}`, rate limits por plano com headers `X-RateLimit-*`, webhooks HMAC e quota com ciclo conhecido (`lastReset` + `billingAnchorDay`).
